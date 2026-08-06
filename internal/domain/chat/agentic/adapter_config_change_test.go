package agentic_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

type adapterRepoStub struct {
	session     chat.ChatSession
	messages    []chat.ChatMessage
	configHash  string
	runMetadata json.RawMessage
}

func (r *adapterRepoStub) FindSessions(context.Context, pagination.PageRequest) ([]chat.ChatSession, int64, error) {
	return []chat.ChatSession{r.session}, 1, nil
}

func (r *adapterRepoStub) GetSessionListStamp(context.Context) (chat.ChatSessionListStamp, error) {
	return chat.ChatSessionListStamp{}, nil
}

func (r *adapterRepoStub) GetSessionByID(_ context.Context, id uuid.UUID) (chat.ChatSession, error) {
	if r.session.ID != id {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	return r.session, nil
}

func (r *adapterRepoStub) CreateSession(_ context.Context, s chat.ChatSession) (chat.ChatSession, error) {
	r.session = s
	return s, nil
}

func (r *adapterRepoStub) CloneSession(_ context.Context, s chat.ChatSession, _ []chat.ChatMessage) (chat.ChatSession, error) {
	r.session = s
	return s, nil
}

func (r *adapterRepoStub) UpdateSessionStatus(_ context.Context, _ uuid.UUID, status chat.ChatStatus) (chat.ChatSession, error) {
	r.session.Status = status
	return r.session, nil
}

func (r *adapterRepoStub) UpdateSessionTitle(_ context.Context, _ uuid.UUID, title string) (chat.ChatSession, error) {
	r.session.Title = title
	return r.session, nil
}

func (r *adapterRepoStub) UpdateSessionAgent(_ context.Context, _ uuid.UUID, agentID uuid.UUID) error {
	r.session.AgentID = &agentID
	return nil
}

func (r *adapterRepoStub) UpdateSessionConfigHash(_ context.Context, _ uuid.UUID, hash string) error {
	r.configHash = hash
	r.session.ConfigHash = &hash
	return nil
}

func (r *adapterRepoStub) UpdateSessionSnapshots(_ context.Context, _ uuid.UUID, systemPrompt *string, modelConfig, skillBindings, agentSnapshot json.RawMessage, agentSnapshotHash *string) error {
	r.session.SystemPromptSnapshot = systemPrompt
	r.session.ModelConfigSnapshot = modelConfig
	r.session.SkillBindingsSnapshot = skillBindings
	r.session.AgentSnapshot = agentSnapshot
	r.session.AgentSnapshotHash = agentSnapshotHash
	return nil
}

func (r *adapterRepoStub) FindAgentsForRouting(context.Context) ([]chat.AgentRoutingInfo, error) {
	return nil, nil
}

func (r *adapterRepoStub) DeleteSession(context.Context, uuid.UUID) error {
	return nil
}

func (r *adapterRepoStub) FindMessages(_ context.Context, sessionID uuid.UUID, _ pagination.PageRequest) ([]chat.ChatMessage, int64, error) {
	out := make([]chat.ChatMessage, 0, len(r.messages))
	for _, msg := range r.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, int64(len(out)), nil
}

func (r *adapterRepoStub) CreateMessage(_ context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()
	r.messages = append(r.messages, msg)
	return msg, nil
}

func (r *adapterRepoStub) GetLatestAssistantMessage(context.Context, uuid.UUID, time.Time) (chat.ChatMessage, bool, error) {
	return chat.ChatMessage{}, false, nil
}

func (r *adapterRepoStub) FindAllMessages(_ context.Context, sessionID uuid.UUID) ([]chat.ChatMessage, error) {
	out := make([]chat.ChatMessage, 0, len(r.messages))
	for _, msg := range r.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (r *adapterRepoStub) GetLatestCompactSummary(context.Context, uuid.UUID) (chat.ChatMessage, bool, error) {
	return chat.ChatMessage{}, false, nil
}

func (r *adapterRepoStub) CreateRun(_ context.Context, run chat.ChatRun) (chat.ChatRun, error) {
	run.ID = uuid.New()
	return run, nil
}

func (r *adapterRepoStub) GetRunByID(context.Context, uuid.UUID) (chat.ChatRun, error) {
	return chat.ChatRun{}, chat.ErrNotFound
}

func (r *adapterRepoStub) GetActiveRunBySession(context.Context, uuid.UUID) (chat.ChatRun, bool, error) {
	return chat.ChatRun{}, false, nil
}

func (r *adapterRepoStub) UpdateRunStatus(context.Context, uuid.UUID, chat.ChatRunStatus, string) error {
	return nil
}

func (r *adapterRepoStub) MarkRunCompleted(context.Context, uuid.UUID) error {
	return nil
}

func (r *adapterRepoStub) MarkRunFailed(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *adapterRepoStub) UpdateRunMetadata(_ context.Context, _ uuid.UUID, metadata json.RawMessage) error {
	r.runMetadata = metadata
	return nil
}

type recordingModelFactory struct {
	model    ai.ChatModel
	provider string
	modelID  string
}

func (f *recordingModelFactory) Build(_ context.Context, provider, model string) (ai.ChatModel, error) {
	f.provider = provider
	f.modelID = model
	return f.model, nil
}

func (f *recordingModelFactory) ResolveModel(context.Context, string) string {
	return ""
}

func (f *recordingModelFactory) ResolveDefaultProvider(context.Context) string {
	return ""
}

func testConfigHash(config json.RawMessage) string {
	var value interface{}
	if err := json.Unmarshal(config, &value); err != nil {
		sum := sha256.Sum256(config)
		return hex.EncodeToString(sum[:])
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		sum := sha256.Sum256(config)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func TestRunSession_ConfigChangeEmitsEventAndKeepsSnapshotModelConfig(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	oldHash := "previous-config-hash"
	snapshotPrompt := "hidden system prompt from the session snapshot"
	snapshotConfig := json.RawMessage(`{"provider":"ollama","model":"snapshot-model"}`)
	currentConfig := json.RawMessage(`{"provider":"openai","model":"current-model"}`)

	repo := &adapterRepoStub{session: chat.ChatSession{
		ID:                   sessionID,
		AgentID:              &agentID,
		Status:               chat.StatusActive,
		SystemPromptSnapshot: &snapshotPrompt,
		ModelConfigSnapshot:  snapshotConfig,
		ConfigHash:           &oldHash,
	}}
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		ID:           agentID,
		Status:       "PUBLISHED",
		SystemPrompt: "current prompt must not be used",
		ModelConfig:  currentConfig,
	}}
	model := &mockChatModel{streamFn: func(_ int, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
		assert.Equal(t, "snapshot-model", opts.Model)
		assert.Contains(t, opts.SystemMsg, snapshotPrompt)
		assert.NotContains(t, opts.SystemMsg, "current prompt must not be used")
		return makeTextStream("done"), nil
	}}
	factory := &recordingModelFactory{model: model}
	skills := &mockSkillLister{}
	kbs := &mockKBLister{}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		factory,
		nil,
		agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs),
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := adapter.RunSession(ctx, chat.RunInput{
		SessionID:            sessionID,
		AgentID:              agentID,
		UserMessage:          "hello",
		TenantID:             "tenant",
		SystemPromptSnapshot: &snapshotPrompt,
		ModelConfigSnapshot:  snapshotConfig,
	})
	require.NoError(t, err)

	var events []chat.RunEvent
	for ev := range ch {
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

	assert.Equal(t, "ollama", factory.provider)
	assert.Equal(t, "snapshot-model", factory.modelID)
	require.NotEmpty(t, repo.configHash)
	assert.Len(t, repo.configHash, 64)

	var systemMessages []chat.ChatMessage
	for _, msg := range repo.messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		}
	}
	require.Len(t, systemMessages, 1)
	assert.Contains(t, systemMessages[0].Content, "continues with the session snapshot")
	assert.NotContains(t, systemMessages[0].Content, snapshotPrompt)
}

func TestRunSession_SystemPromptChangeEmitsConfigChangedEvent(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	snapshotPrompt := "stable persona from the session snapshot"
	snapshotConfig := json.RawMessage(`{"provider":"ollama","model":"snapshot-model"}`)
	currentPrompt := "new live persona should only apply to new sessions"
	currentHash := testConfigHash(snapshotConfig)

	repo := &adapterRepoStub{session: chat.ChatSession{
		ID:                   sessionID,
		AgentID:              &agentID,
		Status:               chat.StatusActive,
		SystemPromptSnapshot: &snapshotPrompt,
		ModelConfigSnapshot:  snapshotConfig,
		ConfigHash:           &currentHash,
	}}
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		ID:           agentID,
		Status:       "PUBLISHED",
		SystemPrompt: currentPrompt,
		ModelConfig:  snapshotConfig,
	}}
	model := &mockChatModel{streamFn: func(_ int, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
		assert.Equal(t, "snapshot-model", opts.Model)
		assert.Contains(t, opts.SystemMsg, snapshotPrompt)
		assert.NotContains(t, opts.SystemMsg, currentPrompt)
		return makeTextStream("done"), nil
	}}
	factory := &recordingModelFactory{model: model}
	skills := &mockSkillLister{}
	kbs := &mockKBLister{}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		factory,
		nil,
		agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs),
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := adapter.RunSession(ctx, chat.RunInput{
		SessionID:            sessionID,
		AgentID:              agentID,
		UserMessage:          "hello",
		TenantID:             "tenant",
		SystemPromptSnapshot: &snapshotPrompt,
		ModelConfigSnapshot:  snapshotConfig,
	})
	require.NoError(t, err)

	var events []chat.RunEvent
	for ev := range ch {
		events = append(events, ev)
	}

	require.NotEmpty(t, events)
	assert.Equal(t, "config_changed", events[0].Type)

	var systemMessages []chat.ChatMessage
	for _, msg := range repo.messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		}
	}
	require.Len(t, systemMessages, 1)
	assert.Contains(t, systemMessages[0].Content, "continues with the session snapshot")
	assert.NotContains(t, systemMessages[0].Content, snapshotPrompt)
	assert.NotContains(t, systemMessages[0].Content, currentPrompt)
}
