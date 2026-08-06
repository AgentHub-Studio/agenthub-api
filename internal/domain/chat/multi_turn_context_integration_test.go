//go:build integration

package chat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// multiTurnHistoryModel is deliberately deterministic: the integration boundary
// under test is the persisted conversation history assembled by the real HTTP,
// service, adapter, Runner, and PostgreSQL path, not an external LLM provider.
type multiTurnHistoryModel struct {
	mu    sync.Mutex
	calls [][]ai.Message
}

func (m *multiTurnHistoryModel) Chat(context.Context, []ai.Message, ai.ChatOptions) (*ai.ChatResponse, error) {
	return nil, errors.New("chat is not used by this integration model")
}

func (m *multiTurnHistoryModel) ChatStream(_ context.Context, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	m.mu.Lock()
	m.calls = append(m.calls, append([]ai.Message(nil), messages...))
	call := len(m.calls)
	m.mu.Unlock()

	responses := []string{
		"Anotado: seu nome é Cezar.",
		"Anotado: você mora em Recife.",
		"Você é Cezar e mora em Recife.",
	}
	if call > len(responses) {
		return nil, errors.New("unexpected extra model call")
	}

	ch := make(chan ai.StreamChunk, 2)
	ch <- ai.StreamChunk{Delta: responses[call-1]}
	ch <- ai.StreamChunk{FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func (m *multiTurnHistoryModel) GetProviderName() string {
	return "integration-deterministic"
}

func (m *multiTurnHistoryModel) snapshotCalls() [][]ai.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([][]ai.Message, len(m.calls))
	for i := range m.calls {
		calls[i] = append([]ai.Message(nil), m.calls[i]...)
	}
	return calls
}

type multiTurnHistoryLoader struct {
	agentID uuid.UUID
}

func (l multiTurnHistoryLoader) GetAgentForRun(_ context.Context, agentID uuid.UUID) (*chat.AgentRunConfig, error) {
	if agentID != l.agentID {
		return nil, errors.New("unexpected agent ID")
	}
	return &chat.AgentRunConfig{
		ID:           l.agentID,
		Status:       "PUBLISHED",
		SystemPrompt: "Answer from the conversation history.",
		ModelConfig:  json.RawMessage(`{"provider":"integration","model":"deterministic"}`),
	}, nil
}

func TestIntegration_MultiTurnContext_HTTPPersistsAndReusesTranscript(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	agents, err := repo.FindAgentsForRouting(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, agents)
	agentID := agents[0].ID

	model := &multiTurnHistoryModel{}
	loader := multiTurnHistoryLoader{agentID: agentID}
	skills := emptyIntegrationSkillLister{}
	kbs := emptyIntegrationKBLister{}
	adapter := agentic.NewSessionRunnerAdapter(
		model,
		nil,
		agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, emptyIntegrationToolsBySkill{}, kbs),
		nil,
		nil,
		nil,
		repo,
		loader,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	svc := chat.NewService(repo, adapter).WithAgentLoader(loader)

	session, err := repo.CreateSession(ctx, chat.ChatSession{
		AgentID: &agentID,
		Title:   "three persisted turns",
		Status:  chat.StatusActive,
	})
	require.NoError(t, err)

	handler := chat.NewHandler(svc, nil)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	turns := []string{
		"Meu nome é Cezar. Lembre disso.",
		"Também moro em Recife. Lembre disso.",
		"Qual é meu nome e onde moro?",
	}
	for _, message := range turns {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/chat/sessions/"+session.ID.String()+"/run",
			bytes.NewBufferString(`{"message":`+mustJSON(t, message)+`}`),
		).WithContext(ctx)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, "text/event-stream", response.Header().Get("Content-Type"))
		assert.Contains(t, response.Body.String(), "event: run_complete\n")
	}

	calls := model.snapshotCalls()
	require.Len(t, calls, 3)
	require.Len(t, calls[2], 5, "third turn must receive both earlier user/assistant pairs and its current user message")
	assert.Equal(t, ai.RoleUser, calls[2][0].Role)
	assert.Equal(t, turns[0], calls[2][0].Content)
	assert.Equal(t, ai.RoleAssistant, calls[2][1].Role)
	assert.Equal(t, "Anotado: seu nome é Cezar.", calls[2][1].Content)
	assert.Equal(t, ai.RoleUser, calls[2][2].Role)
	assert.Equal(t, turns[1], calls[2][2].Content)
	assert.Equal(t, ai.RoleAssistant, calls[2][3].Role)
	assert.Equal(t, "Anotado: você mora em Recife.", calls[2][3].Content)
	assert.Equal(t, ai.RoleUser, calls[2][4].Role)
	assert.Equal(t, turns[2], calls[2][4].Content)

	persisted, err := repo.FindAllMessages(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, persisted, 6)
	assert.Equal(t,
		[]string{"user", "assistant", "user", "assistant", "user", "assistant"},
		[]string{persisted[0].Role, persisted[1].Role, persisted[2].Role, persisted[3].Role, persisted[4].Role, persisted[5].Role},
	)
	assert.Equal(t, []string{
		turns[0],
		"Anotado: seu nome é Cezar.",
		turns[1],
		"Anotado: você mora em Recife.",
		turns[2],
		"Você é Cezar e mora em Recife.",
	}, []string{
		persisted[0].Content,
		persisted[1].Content,
		persisted[2].Content,
		persisted[3].Content,
		persisted[4].Content,
		persisted[5].Content,
	})
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
