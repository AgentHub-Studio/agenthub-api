//go:build integration

package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

type configChangeIntegrationModel struct {
	systemMsg string
}

func (m *configChangeIntegrationModel) Chat(context.Context, []ai.Message, ai.ChatOptions) (*ai.ChatResponse, error) {
	return nil, errors.New("chat not implemented in integration fake")
}

func (m *configChangeIntegrationModel) ChatStream(_ context.Context, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	m.systemMsg = opts.SystemMsg
	ch := make(chan ai.StreamChunk, 2)
	ch <- ai.StreamChunk{Delta: "done"}
	ch <- ai.StreamChunk{FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func (m *configChangeIntegrationModel) GetProviderName() string {
	return "integration-fake"
}

type configChangeIntegrationFactory struct {
	model *configChangeIntegrationModel
}

func (f *configChangeIntegrationFactory) Build(context.Context, string, string) (ai.ChatModel, error) {
	return f.model, nil
}

func (f *configChangeIntegrationFactory) ResolveModel(context.Context, string) string {
	return ""
}

func (f *configChangeIntegrationFactory) ResolveDefaultProvider(context.Context) string {
	return ""
}

type configChangeIntegrationLoader struct {
	cfg *chat.AgentRunConfig
}

func (l *configChangeIntegrationLoader) GetAgentForRun(context.Context, uuid.UUID) (*chat.AgentRunConfig, error) {
	return l.cfg, nil
}

type emptyIntegrationSkillLister struct{}

func (emptyIntegrationSkillLister) ListByAgentID(context.Context, uuid.UUID) ([]skill.Skill, error) {
	return nil, nil
}

func (emptyIntegrationSkillLister) ListByIDs(context.Context, []uuid.UUID) ([]skill.Skill, error) {
	return nil, nil
}

func (emptyIntegrationSkillLister) List(context.Context, *string, pagination.PageRequest) ([]skill.Skill, int64, error) {
	return nil, 0, nil
}

type emptyIntegrationKBLister struct{}

func (emptyIntegrationKBLister) ListByAgentID(context.Context, uuid.UUID) ([]knowledgebase.KnowledgeBase, error) {
	return nil, nil
}

func (emptyIntegrationKBLister) List(context.Context, pagination.PageRequest) ([]knowledgebase.KnowledgeBase, int64, error) {
	return nil, 0, nil
}

type emptyIntegrationToolsBySkill struct{}

func (emptyIntegrationToolsBySkill) ListBySkill(context.Context, uuid.UUID) ([]tool.SkillTool, []tool.Tool, error) {
	return nil, nil, nil
}

func TestIntegration_ConfigChangedOnPersonaDriftPersistsSystemMessage(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	agents, err := repo.FindAgentsForRouting(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, agents)
	agentID := agents[0].ID

	snapshotPrompt := "stable persona from session snapshot"
	currentPrompt := "new live persona should only apply to new sessions"
	modelConfig := json.RawMessage(`{"provider":"ollama","model":"snapshot-model"}`)
	created, err := repo.CreateSession(ctx, chat.ChatSession{
		AgentID:              &agentID,
		Title:                "persona drift integration",
		Status:               chat.StatusActive,
		SystemPromptSnapshot: &snapshotPrompt,
		ModelConfigSnapshot:  modelConfig,
	})
	require.NoError(t, err)

	model := &configChangeIntegrationModel{}
	loader := &configChangeIntegrationLoader{cfg: &chat.AgentRunConfig{
		ID:           agentID,
		Status:       "PUBLISHED",
		SystemPrompt: currentPrompt,
		ModelConfig:  modelConfig,
	}}
	skills := emptyIntegrationSkillLister{}
	kbs := emptyIntegrationKBLister{}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		&configChangeIntegrationFactory{model: model},
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

	eventsCh, err := adapter.RunSession(ctx, chat.RunInput{
		SessionID:            created.ID,
		AgentID:              agentID,
		UserMessage:          "hello",
		TenantID:             provisioningTestTenant,
		SystemPromptSnapshot: &snapshotPrompt,
		ModelConfigSnapshot:  modelConfig,
	})
	require.NoError(t, err)

	var events []chat.RunEvent
	for ev := range eventsCh {
		events = append(events, ev)
	}
	require.NotEmpty(t, events)
	assert.Equal(t, "config_changed", events[0].Type)

	var payload struct {
		OldPersona      string `json:"oldPersona"`
		NewSnapshotHash string `json:"newSnapshotHash"`
	}
	require.NoError(t, json.Unmarshal(events[0].Data, &payload))
	assert.Equal(t, "redacted", payload.OldPersona)
	assert.Len(t, payload.NewSnapshotHash, 64)
	assert.NotContains(t, string(events[0].Data), snapshotPrompt)
	assert.NotContains(t, string(events[0].Data), currentPrompt)
	assert.Contains(t, model.systemMsg, snapshotPrompt)
	assert.NotContains(t, model.systemMsg, currentPrompt)

	messages, err := repo.FindAllMessages(ctx, created.ID)
	require.NoError(t, err)
	var systemMessages []chat.ChatMessage
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		}
	}
	require.Len(t, systemMessages, 1)
	assert.Contains(t, systemMessages[0].Content, "continues with the session snapshot")
	assert.NotContains(t, systemMessages[0].Content, snapshotPrompt)
	assert.NotContains(t, systemMessages[0].Content, currentPrompt)
}
