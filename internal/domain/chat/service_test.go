package chat_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/evals"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockChatRepo struct {
	sessions        map[uuid.UUID]chat.ChatSession
	messages        []chat.ChatMessage
	routingAgents   []chat.AgentRoutingInfo
	routingErr      error
	getSessionCalls int
}

func uuidPtr() *uuid.UUID { id := uuid.New(); return &id }

func newMockRepo() *mockChatRepo {
	return &mockChatRepo{sessions: make(map[uuid.UUID]chat.ChatSession)}
}

func (m *mockChatRepo) FindSessions(_ context.Context, _ pagination.PageRequest) ([]chat.ChatSession, int64, error) {
	out := make([]chat.ChatSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}

func (m *mockChatRepo) GetSessionByID(_ context.Context, id uuid.UUID) (chat.ChatSession, error) {
	m.getSessionCalls++
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	return s, nil
}

func (m *mockChatRepo) CreateSession(_ context.Context, s chat.ChatSession) (chat.ChatSession, error) {
	s.ID = uuid.New()
	s.Status = chat.StatusActive
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockChatRepo) CloneSession(_ context.Context, s chat.ChatSession, messages []chat.ChatMessage) (chat.ChatSession, error) {
	s.ID = uuid.New()
	s.Status = chat.StatusActive

	copiedMessages := make([]chat.ChatMessage, 0, len(messages))
	for _, message := range messages {
		copied := message
		copied.ID = uuid.New()
		copied.SessionID = s.ID
		copied.RunID = nil
		copied.CreatedAt = time.Time{}
		copied.ToolCalls = append(json.RawMessage(nil), message.ToolCalls...)
		copied.Metadata = append(json.RawMessage(nil), message.Metadata...)
		copied.TokenUsage = append(json.RawMessage(nil), message.TokenUsage...)
		copiedMessages = append(copiedMessages, copied)
	}

	m.sessions[s.ID] = s
	m.messages = append(m.messages, copiedMessages...)
	return s, nil
}

func (m *mockChatRepo) UpdateSessionStatus(_ context.Context, id uuid.UUID, status chat.ChatStatus) (chat.ChatSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	s.Status = status
	m.sessions[id] = s
	return s, nil
}

func (m *mockChatRepo) UpdateSessionTitle(_ context.Context, id uuid.UUID, title string) (chat.ChatSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	s.Title = title
	m.sessions[id] = s
	return s, nil
}

func (m *mockChatRepo) DeleteSession(_ context.Context, id uuid.UUID) error {
	if _, ok := m.sessions[id]; !ok {
		return chat.ErrNotFound
	}
	delete(m.sessions, id)
	return nil
}

func (m *mockChatRepo) FindMessages(_ context.Context, sessionID uuid.UUID, _ pagination.PageRequest) ([]chat.ChatMessage, int64, error) {
	var out []chat.ChatMessage
	for _, msg := range m.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockChatRepo) CreateMessage(_ context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	msg.ID = uuid.New()
	m.messages = append(m.messages, msg)
	return msg, nil
}

func (m *mockChatRepo) GetLatestAssistantMessage(_ context.Context, sessionID uuid.UUID, after time.Time) (chat.ChatMessage, bool, error) {
	for _, msg := range m.messages {
		if msg.SessionID == sessionID && msg.Role == "assistant" && msg.CreatedAt.After(after) {
			return msg, true, nil
		}
	}
	return chat.ChatMessage{}, false, nil
}

func (m *mockChatRepo) FindAllMessages(_ context.Context, sessionID uuid.UUID) ([]chat.ChatMessage, error) {
	var out []chat.ChatMessage
	for _, msg := range m.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *mockChatRepo) GetLatestCompactSummary(_ context.Context, sessionID uuid.UUID) (chat.ChatMessage, bool, error) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		if msg.SessionID == sessionID && msg.MessageType == chat.MessageTypeCompactSummary {
			return msg, true, nil
		}
	}
	return chat.ChatMessage{}, false, nil
}

func (m *mockChatRepo) GetSessionListStamp(_ context.Context) (chat.ChatSessionListStamp, error) {
	return chat.ChatSessionListStamp{}, nil
}

func (m *mockChatRepo) UpdateSessionConfigHash(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockChatRepo) UpdateSessionAgent(_ context.Context, sessionID uuid.UUID, agentID uuid.UUID) error {
	s, ok := m.sessions[sessionID]
	if !ok {
		return chat.ErrNotFound
	}
	s.AgentID = &agentID
	m.sessions[sessionID] = s
	return nil
}

func (m *mockChatRepo) UpdateSessionSnapshots(_ context.Context, sessionID uuid.UUID, systemPrompt *string, modelConfig, skillBindings, agentSnapshot json.RawMessage, agentSnapshotHash *string) error {
	s, ok := m.sessions[sessionID]
	if !ok {
		return chat.ErrNotFound
	}
	s.SystemPromptSnapshot = systemPrompt
	s.ModelConfigSnapshot = modelConfig
	s.SkillBindingsSnapshot = skillBindings
	s.AgentSnapshot = agentSnapshot
	s.AgentSnapshotHash = agentSnapshotHash
	m.sessions[sessionID] = s
	return nil
}

func (m *mockChatRepo) FindAgentsForRouting(_ context.Context) ([]chat.AgentRoutingInfo, error) {
	return m.routingAgents, m.routingErr
}

func (m *mockChatRepo) CreateRun(_ context.Context, r chat.ChatRun) (chat.ChatRun, error) {
	r.ID = uuid.New()
	return r, nil
}

func (m *mockChatRepo) GetRunByID(_ context.Context, _ uuid.UUID) (chat.ChatRun, error) {
	return chat.ChatRun{}, chat.ErrNotFound
}

func (m *mockChatRepo) GetActiveRunBySession(_ context.Context, _ uuid.UUID) (chat.ChatRun, bool, error) {
	return chat.ChatRun{}, false, nil
}

func (m *mockChatRepo) UpdateRunStatus(_ context.Context, _ uuid.UUID, _ chat.ChatRunStatus, _ string) error {
	return nil
}

func (m *mockChatRepo) MarkRunCompleted(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockChatRepo) MarkRunFailed(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockChatRepo) UpdateRunMetadata(_ context.Context, _ uuid.UUID, _ json.RawMessage) error {
	return nil
}

func TestChatService_CreateSession_Success(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	agentID := uuid.New()
	s, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "Support Chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "Support Chat", s.Title)
	assert.NotEqual(t, uuid.Nil, s.ID)
	assert.Equal(t, chat.StatusActive, s.Status)
}

func TestChatService_GetSession_NotFound(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	_, err := svc.GetSession(context.Background(), uuid.New())
	require.ErrorIs(t, err, chat.ErrNotFound)
}

func TestChatService_ArchiveSession(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	created, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{AgentID: uuidPtr(), Title: "test"})
	require.NoError(t, err)
	archived, err := svc.ArchiveSession(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, chat.StatusArchived, archived.Status)
}

func TestChatService_AddMessage(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{AgentID: uuidPtr(), Title: "q&a"})
	require.NoError(t, err)
	msg, err := svc.AddMessage(context.Background(), session.ID, chat.CreateMessageRequest{
		Role:    "user",
		Content: "Hello!",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello!", msg.Content)
	assert.NotEqual(t, uuid.Nil, msg.ID)
}

func TestChatService_CloneSession_CopiesMessagesUntilMessageID(t *testing.T) {
	repo := newMockRepo()
	svc := chat.NewService(repo, nil)
	ctx := context.Background()
	agentID := uuid.New()

	source, err := svc.CreateSession(ctx, chat.CreateSessionRequest{AgentID: &agentID, Title: "Original"})
	require.NoError(t, err)
	systemPrompt := "stable prompt"
	sourceEntity := repo.sessions[source.ID]
	sourceEntity.SystemPromptSnapshot = &systemPrompt
	sourceEntity.ModelConfigSnapshot = json.RawMessage(`{"model":"test"}`)
	sourceEntity.SkillBindingsSnapshot = json.RawMessage(`{"skillIds":[]}`)
	sourceEntity.AgentSnapshot = json.RawMessage(`{"systemPrompt":"stable prompt","modelConfig":{"model":"test"}}`)
	sourceHash := sha256.Sum256(sourceEntity.AgentSnapshot)
	sourceHashText := hex.EncodeToString(sourceHash[:])
	sourceEntity.AgentSnapshotHash = &sourceHashText
	configHash := "a3e561b1d7f768f844e412e23b02c5b45d3e78c82d75912d7c4e3f9b4968da12"
	sourceEntity.ConfigHash = &configHash
	repo.sessions[source.ID] = sourceEntity

	firstID := uuid.New()
	secondID := uuid.New()
	runID := uuid.New()
	repo.messages = append(repo.messages,
		chat.ChatMessage{ID: firstID, SessionID: source.ID, Role: "user", Content: "primeira", MessageType: chat.MessageTypeText, RunID: &runID},
		chat.ChatMessage{ID: secondID, SessionID: source.ID, Role: "user", Content: "segunda", MessageType: chat.MessageTypeText, RunID: &runID},
	)

	clone, err := svc.CloneSession(ctx, source.ID, chat.CloneSessionRequest{
		UntilMessageID: &firstID,
		Title:          "Variant A",
	})

	require.NoError(t, err)
	assert.NotEqual(t, source.ID, clone.ID)
	assert.Equal(t, "Variant A", clone.Title)
	assert.Equal(t, chat.StatusActive, clone.Status)
	require.NotNil(t, clone.ClonedFromSessionID)
	assert.Equal(t, source.ID, *clone.ClonedFromSessionID)
	require.NotNil(t, clone.ClonedFromSessionTitle)
	assert.Equal(t, "Original", *clone.ClonedFromSessionTitle)
	require.NotNil(t, clone.AgentID)
	assert.Equal(t, agentID, *clone.AgentID)

	clonedSession := repo.sessions[clone.ID]
	require.NotNil(t, clonedSession.SystemPromptSnapshot)
	assert.Equal(t, "stable prompt", *clonedSession.SystemPromptSnapshot)
	assert.JSONEq(t, `{"model":"test"}`, string(clonedSession.ModelConfigSnapshot))
	assert.JSONEq(t, `{"skillIds":[]}`, string(clonedSession.SkillBindingsSnapshot))
	assert.JSONEq(t, `{"systemPrompt":"stable prompt","modelConfig":{"model":"test"}}`, string(clonedSession.AgentSnapshot))
	require.NotNil(t, clonedSession.AgentSnapshotHash)
	assert.Equal(t, sourceHashText, *clonedSession.AgentSnapshotHash)
	require.NotNil(t, clonedSession.ConfigHash)
	assert.Equal(t, configHash, *clonedSession.ConfigHash)

	clonedMessages, err := repo.FindAllMessages(ctx, clone.ID)
	require.NoError(t, err)
	require.Len(t, clonedMessages, 1)
	assert.NotEqual(t, firstID, clonedMessages[0].ID)
	assert.Equal(t, clone.ID, clonedMessages[0].SessionID)
	assert.Equal(t, "primeira", clonedMessages[0].Content)
	assert.Equal(t, chat.MessageTypeText, clonedMessages[0].MessageType)
	assert.Nil(t, clonedMessages[0].RunID)

	originalMessages, err := repo.FindAllMessages(ctx, source.ID)
	require.NoError(t, err)
	assert.Len(t, originalMessages, 2)
	assert.Equal(t, secondID, originalMessages[1].ID)
}

func TestChatService_CloneSession_CloneDoesNotAffectOriginal(t *testing.T) {
	repo := newMockRepo()
	svc := chat.NewService(repo, nil)
	ctx := context.Background()

	source, err := svc.CreateSession(ctx, chat.CreateSessionRequest{Title: "Original"})
	require.NoError(t, err)
	repo.messages = append(repo.messages, chat.ChatMessage{
		ID:          uuid.New(),
		SessionID:   source.ID,
		Role:        "user",
		Content:     "mensagem original",
		MessageType: chat.MessageTypeText,
	})

	clone, err := svc.CloneSession(ctx, source.ID, chat.CloneSessionRequest{})
	require.NoError(t, err)
	_, err = svc.AddMessage(ctx, clone.ID, chat.CreateMessageRequest{Role: "user", Content: "mensagem exclusiva do clone"})
	require.NoError(t, err)

	originalMessages, err := repo.FindAllMessages(ctx, source.ID)
	require.NoError(t, err)
	clonedMessages, err := repo.FindAllMessages(ctx, clone.ID)
	require.NoError(t, err)

	require.Len(t, originalMessages, 1)
	require.Len(t, clonedMessages, 2)
	assert.Equal(t, "mensagem original", originalMessages[0].Content)
	assert.Equal(t, []string{"mensagem original", "mensagem exclusiva do clone"}, []string{clonedMessages[0].Content, clonedMessages[1].Content})
}

func TestChatService_CloneSession_UntilMessageMustBelongToSession(t *testing.T) {
	repo := newMockRepo()
	svc := chat.NewService(repo, nil)
	ctx := context.Background()

	source, err := svc.CreateSession(ctx, chat.CreateSessionRequest{Title: "Original"})
	require.NoError(t, err)
	missingBoundary := uuid.New()

	_, err = svc.CloneSession(ctx, source.ID, chat.CloneSessionRequest{UntilMessageID: &missingBoundary})

	require.ErrorIs(t, err, chat.ErrMessageNotFound)
	assert.Len(t, repo.sessions, 1)
}

// --- mock SessionRunner ---

type mockSessionRunner struct {
	events    []chat.RunEvent
	err       error
	lastInput chat.RunInput
}

type mockEvalRecorder struct {
	calls chan evals.RecordRequest
}

func (m *mockEvalRecorder) RecordRunComplete(_ context.Context, req evals.RecordRequest) (evals.EvalRun, bool, error) {
	m.calls <- req
	return evals.EvalRun{}, true, nil
}

func (m *mockSessionRunner) RunSession(_ context.Context, in chat.RunInput) (<-chan chat.RunEvent, error) {
	m.lastInput = in
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan chat.RunEvent, len(m.events))
	for _, ev := range m.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestChatService_RunSession_RecordsEvalOnRunComplete(t *testing.T) {
	repo := newMockRepo()
	agentID := uuid.New()
	runner := &mockSessionRunner{
		events: []chat.RunEvent{
			{Type: "text_delta", Data: []byte(`{"content":"ok"}`)},
			{Type: "run_complete", Data: []byte(`{"totalTurns":1}`)},
		},
	}
	recorder := &mockEvalRecorder{calls: make(chan evals.RecordRequest, 1)}
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		ID:     agentID,
		Status: "PUBLISHED",
		EvalConfig: evals.EvalConfig{
			Scorers:    []string{"exact_match"},
			SampleRate: 1,
		},
	}}
	svc := chat.NewService(repo, runner).WithAgentLoader(loader).WithEvalRecorder(recorder)

	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "eval sampling",
	})
	require.NoError(t, err)

	ch, err := svc.RunSession(context.Background(), session.ID, "Hello", "test-tenant")
	require.NoError(t, err)
	for range ch {
	}

	select {
	case req := <-recorder.calls:
		assert.Equal(t, session.ID, req.SessionID)
		assert.Equal(t, agentID, req.AgentID)
		require.Equal(t, []string{"exact_match"}, req.Config.Scorers)
		assert.Equal(t, 1.0, req.Config.SampleRate)
	case <-time.After(time.Second):
		t.Fatal("eval recorder was not called")
	}
}

func TestChatService_RunSession_Success(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{
		events: []chat.RunEvent{
			{Type: "text_delta", Data: []byte(`{"content":"Hello"}`)},
			{Type: "run_complete", Data: []byte(`{"totalTurns":1}`)},
		},
	}
	svc := chat.NewService(repo, runner)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	ch, err := svc.RunSession(context.Background(), session.ID, "Hello", "test-tenant")
	require.NoError(t, err)

	var events []chat.RunEvent
	for ev := range ch {
		events = append(events, ev)
	}
	assert.Len(t, events, 2)
	assert.Equal(t, "text_delta", events[0].Type)
	assert.Equal(t, "run_complete", events[1].Type)
}

func TestChatService_RunSession_NoRunner(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	_, err := svc.RunSession(context.Background(), uuid.New(), "Hello", "tenant")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agentic features not configured")
}

func TestChatService_RunSession_NoAgent(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner)

	// Create session WITHOUT an agent; the mock repo has no routable agents.
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		Title: "no-agent",
	})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "Hello", "tenant")
	require.ErrorIs(t, err, chat.ErrNoAgentAvailable)
}

// TestRunSession_RoutesSingleAgent verifies that an agentless session is bound
// to the only published agent when exactly one exists (the OOB default-assistant
// case for a fresh tenant).
func TestRunSession_RoutesSingleAgent(t *testing.T) {
	repo := newMockRepo()
	agentID := uuid.New()
	repo.routingAgents = []chat.AgentRoutingInfo{{ID: agentID, Name: "Meu Assistente"}}
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner)

	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{Title: "agentless"})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "olá, tudo bem?", "tenant")
	require.NoError(t, err)
	assert.Equal(t, agentID, runner.lastInput.AgentID, "runner should receive the routed agent")

	bound, err := svc.GetSession(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, bound.AgentID)
	assert.Equal(t, agentID, *bound.AgentID, "session should be bound to the routed agent")
}

// TestRunSession_RoutesBestOfMultiple verifies keyword-based routing picks the
// most relevant published agent when several exist.
func TestRunSession_RoutesBestOfMultiple(t *testing.T) {
	repo := newMockRepo()
	general := uuid.New()
	billing := uuid.New()
	repo.routingAgents = []chat.AgentRoutingInfo{
		{ID: general, Name: "General", Description: "helpful companion"},
		{ID: billing, Name: "Billing Specialist", Description: "handles invoices"},
	}
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner)

	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{Title: "agentless"})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "I have a billing question", "tenant")
	require.NoError(t, err)
	assert.Equal(t, billing, runner.lastInput.AgentID)
}

// TestRunSession_RoutingRepoError verifies a repository failure during routing
// is surfaced as an error rather than silently picking no agent.
func TestRunSession_RoutingRepoError(t *testing.T) {
	repo := newMockRepo()
	repo.routingErr = errors.New("db unavailable")
	svc := chat.NewService(repo, &mockSessionRunner{})

	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{Title: "agentless"})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "Hello", "tenant")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "route agent")
}

// TestRunSession_SnapshotOnFirstRoute verifies that when an agent is routed and
// bound to a previously agentless session, its persona/model/skills are captured
// onto the session AND forwarded to the runner (P-C115-1 consistency guarantee).
func TestRunSession_SnapshotOnFirstRoute(t *testing.T) {
	repo := newMockRepo()
	agentID := uuid.New()
	repo.routingAgents = []chat.AgentRoutingInfo{{ID: agentID, Name: "Routed Agent"}}
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		ID:           agentID,
		SystemPrompt: "You are the routed agent.",
		ModelConfig:  json.RawMessage(`{"provider":"openrouter","model":"x"}`),
		SkillIDs:     []uuid.UUID{uuid.New()},
		Status:       "PUBLISHED",
	}}
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner).WithAgentLoader(loader)

	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{Title: "agentless"})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "do something useful", "tenant")
	require.NoError(t, err)

	// Forwarded to the runner for this run.
	require.NotNil(t, runner.lastInput.SystemPromptSnapshot)
	assert.Equal(t, "You are the routed agent.", *runner.lastInput.SystemPromptSnapshot)
	assert.NotEmpty(t, runner.lastInput.SkillIDsSnapshot)

	// Persisted to the session so subsequent runs stay consistent.
	stored, err := repo.GetSessionByID(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.SystemPromptSnapshot)
	assert.Equal(t, "You are the routed agent.", *stored.SystemPromptSnapshot)
}

func TestChatService_RunSession_SessionNotFound(t *testing.T) {
	runner := &mockSessionRunner{}
	svc := chat.NewService(newMockRepo(), runner)

	_, err := svc.RunSession(context.Background(), uuid.New(), "Hello", "tenant")
	require.Error(t, err)
}

func TestChatService_ListMessages(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{AgentID: uuidPtr(), Title: "q&a"})
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = svc.AddMessage(context.Background(), session.ID, chat.CreateMessageRequest{Role: "user", Content: "msg"})
		require.NoError(t, err)
	}
	page, err := svc.ListMessages(context.Background(), session.ID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
	assert.Len(t, page.Content, 3)
}

func TestChatService_CreateAndRunSession_ReusesCreatedSnapshot(t *testing.T) {
	repo := newMockRepo()
	agentID := uuid.New()
	config := &chat.AgentRunConfig{ID: agentID, Status: "PUBLISHED"}
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner).WithAgentLoader(&mockAgentLoader{cfg: config})

	run, err := svc.CreateAndRunSession(context.Background(), chat.CreateSessionRequest{
		AgentID:     &agentID,
		Title:       "a2a invocation",
		AgentConfig: config,
	}, "hello", "target-tenant")
	require.NoError(t, err)
	require.NotNil(t, run.Events)
	assert.Equal(t, 0, repo.getSessionCalls, "new session must not be read again before its first run")
	require.NotNil(t, runner.lastInput.UserMessageID, "user message must be persisted before the runner starts")
	assert.Equal(t, run.Session.ID, runner.lastInput.SessionID)
	assert.Equal(t, agentID, runner.lastInput.AgentID)
	assert.Same(t, config, runner.lastInput.AgentConfig)
}

// --- TR-01-TASK-12: User message persisted before run (P-C178-2) ---

// TestRunSession_UserMessagePersistedBeforeRun verifies that the user message is
// written to the repository before the runner is invoked, so that it is never
// lost even if the run later fails.
func TestRunSession_UserMessagePersistedBeforeRun(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "Hello from user", "test-tenant")
	require.NoError(t, err)

	// The runner must have received a non-nil UserMessageID.
	require.NotNil(t, runner.lastInput.UserMessageID, "runner should receive a pre-persisted UserMessageID")

	// The message must exist in the repository.
	var found bool
	for _, msg := range repo.messages {
		if msg.Role == "user" && msg.Content == "Hello from user" {
			found = true
			assert.Equal(t, *runner.lastInput.UserMessageID, msg.ID)
		}
	}
	assert.True(t, found, "user message must be persisted in the repo before the runner is called")
}

// TestRunSession_RunnerError_UserMessageStillPersisted verifies that even when the
// runner returns an error (e.g. agent not published), the user message is already
// in the repository and is not lost.
func TestRunSession_RunnerError_UserMessageStillPersisted(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{err: chat.ErrAgentNotPublished}
	svc := chat.NewService(repo, runner)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "message before failure", "tenant")
	require.Error(t, err)

	// Despite the runner error, the user message must be persisted.
	var found bool
	for _, msg := range repo.messages {
		if msg.Role == "user" && msg.Content == "message before failure" {
			found = true
			break
		}
	}
	assert.True(t, found, "user message must be in the repo even when runner returns an error")
}

// --- TR-01-TASK-16: session snapshot (P-C115-1, P-C330-1) ---

// mockAgentLoader is a test double for chat.AgentLoader.
type mockAgentLoader struct {
	cfg   *chat.AgentRunConfig
	err   error
	calls int
}

func (m *mockAgentLoader) GetAgentForRun(_ context.Context, _ uuid.UUID) (*chat.AgentRunConfig, error) {
	m.calls++
	return m.cfg, m.err
}

func TestCreateSession_SnapshotsSystemPromptAtCreation(t *testing.T) {
	repo := newMockRepo()
	prompt := "You are ARIA, a helpful assistant."
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: prompt,
		Status:       "PUBLISHED",
	}}
	svc := chat.NewService(repo, nil).WithAgentLoader(loader)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	// Verify snapshot was persisted through the repo by fetching the raw session.
	stored := repo.sessions[session.ID]
	require.NotNil(t, stored.SystemPromptSnapshot)
	assert.Equal(t, prompt, *stored.SystemPromptSnapshot)
}

func TestCreateSession_UsesProvidedAgentConfig(t *testing.T) {
	repo := newMockRepo()
	agentID := uuid.New()
	config := &chat.AgentRunConfig{
		ID:           agentID,
		SystemPrompt: "Use the supplied configuration.",
		Status:       "PUBLISHED",
	}
	loader := &mockAgentLoader{err: errors.New("loader must not be called")}
	svc := chat.NewService(repo, nil).WithAgentLoader(loader)

	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID:     &agentID,
		Title:       "test",
		AgentConfig: config,
	})

	require.NoError(t, err)
	assert.Zero(t, loader.calls)
	stored := repo.sessions[session.ID]
	require.NotNil(t, stored.SystemPromptSnapshot)
	assert.Equal(t, config.SystemPrompt, *stored.SystemPromptSnapshot)
}

func TestCreateSession_SnapshotsCanonicalAgentSnapshotAndHash(t *testing.T) {
	repo := newMockRepo()
	skillID1 := uuid.New()
	skillID2 := uuid.New()
	prompt := "You are ARIA, a helpful assistant."
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: prompt,
		ModelConfig:  json.RawMessage(`{"provider":"openrouter","model":"openai/gpt-4.1-mini"}`),
		Status:       "PUBLISHED",
		SkillIDs:     []uuid.UUID{skillID1, skillID2},
	}}
	svc := chat.NewService(repo, nil).WithAgentLoader(loader)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	stored := repo.sessions[session.ID]
	require.NotEmpty(t, stored.AgentSnapshot)
	require.NotNil(t, stored.AgentSnapshotHash)
	require.Len(t, *stored.AgentSnapshotHash, 64)
	_, err = hex.DecodeString(*stored.AgentSnapshotHash)
	require.NoError(t, err)
	hash := sha256.Sum256(stored.AgentSnapshot)
	assert.Equal(t, hex.EncodeToString(hash[:]), *stored.AgentSnapshotHash)

	var snapshot chat.AgentSnapshotData
	require.NoError(t, json.Unmarshal(stored.AgentSnapshot, &snapshot))
	assert.Equal(t, prompt, snapshot.SystemPrompt)
	assert.JSONEq(t, `{"provider":"openrouter","model":"openai/gpt-4.1-mini"}`, string(snapshot.ModelConfig))
	assert.Equal(t, []uuid.UUID{skillID1, skillID2}, snapshot.SkillIDs)
}

func TestCreateSession_CanonicalAgentSnapshotRedactsModelAPIKey(t *testing.T) {
	repo := newMockRepo()
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: "You are ARIA.",
		ModelConfig:  json.RawMessage(`{"provider":"openai","model":"gpt-4.1-mini","apiKey":"sk-secret","api_secret":"legacy-secret","temperature":0.2}`),
		Status:       "PUBLISHED",
	}}
	svc := chat.NewService(repo, nil).WithAgentLoader(loader)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	stored := repo.sessions[session.ID]
	require.NotEmpty(t, stored.AgentSnapshot)
	assert.NotContains(t, string(stored.AgentSnapshot), "sk-secret")
	assert.NotContains(t, string(stored.AgentSnapshot), "legacy-secret")
	assert.NotContains(t, string(stored.AgentSnapshot), "apiKey")
	assert.NotContains(t, string(stored.AgentSnapshot), "api_secret")

	var snapshot chat.AgentSnapshotData
	require.NoError(t, json.Unmarshal(stored.AgentSnapshot, &snapshot))
	assert.JSONEq(t, `{"provider":"openai","model":"gpt-4.1-mini","temperature":0.2}`, string(snapshot.ModelConfig))
}

func TestCreateSession_NoLoader_NoSnapshot(t *testing.T) {
	repo := newMockRepo()
	svc := chat.NewService(repo, nil) // no agent loader wired

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	stored := repo.sessions[session.ID]
	assert.Nil(t, stored.SystemPromptSnapshot, "no snapshot when loader is not wired")
}

func TestRunSession_PassesSnapshotToRunner(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}

	snapshot := "You are ARIA."
	agentID := uuid.New()
	sessionID := uuid.New()
	repo.sessions[sessionID] = chat.ChatSession{
		ID:                   sessionID,
		AgentID:              &agentID,
		Status:               chat.StatusActive,
		SystemPromptSnapshot: &snapshot,
	}

	svc := chat.NewService(repo, runner)
	_, err := svc.RunSession(context.Background(), sessionID, "Hello", "tenant")
	require.NoError(t, err)

	// The runner must have received the snapshot.
	require.NotNil(t, runner.lastInput.SystemPromptSnapshot)
	assert.Equal(t, snapshot, *runner.lastInput.SystemPromptSnapshot)
}

func TestRunSession_SystemPromptOverrideUpdatesSessionSnapshotAndRunnerInput(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}

	original := "Persisted session snapshot."
	override := "Preview prompt from Studio."
	agentID := uuid.New()
	sessionID := uuid.New()
	repo.sessions[sessionID] = chat.ChatSession{
		ID:                   sessionID,
		AgentID:              &agentID,
		Status:               chat.StatusActive,
		SystemPromptSnapshot: &original,
	}

	svc := chat.NewService(repo, runner)
	_, err := svc.RunSession(context.Background(), sessionID, "Hello", "tenant", chat.RunSessionOptions{
		SystemPromptOverride: &override,
	})
	require.NoError(t, err)

	stored := repo.sessions[sessionID]
	require.NotNil(t, stored.SystemPromptSnapshot)
	assert.Equal(t, override, *stored.SystemPromptSnapshot)
	assert.JSONEq(t, `{"systemPrompt":"Preview prompt from Studio."}`, string(stored.AgentSnapshot))
	require.NotNil(t, stored.AgentSnapshotHash)
	hash := sha256.Sum256(stored.AgentSnapshot)
	assert.Equal(t, hex.EncodeToString(hash[:]), *stored.AgentSnapshotHash)
	assert.Equal(t, override, runner.lastInput.SystemPrompt)
	require.NotNil(t, runner.lastInput.SystemPromptSnapshot)
	assert.Equal(t, override, *runner.lastInput.SystemPromptSnapshot)
}

func TestRunSession_AppliesInputProcessorsBeforePersistingAndRunning(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}

	agentID := uuid.New()
	sessionID := uuid.New()
	repo.sessions[sessionID] = chat.ChatSession{
		ID:      sessionID,
		AgentID: &agentID,
		Status:  chat.StatusActive,
	}
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		ID:              agentID,
		Status:          "PUBLISHED",
		InputProcessors: []string{"upper_caser", "pii_redactor"},
	}}
	svc := chat.NewService(repo, runner).WithAgentLoader(loader)

	_, err := svc.RunSession(context.Background(), sessionID, "Meu CPF é 123.456.789-00", "tenant")

	require.NoError(t, err)
	require.Len(t, repo.messages, 1)
	msg := repo.messages[0]
	assert.Equal(t, "MEU CPF É [REDACTED:CPF]", msg.Content)
	assert.Equal(t, "MEU CPF É [REDACTED:CPF]", runner.lastInput.UserMessage)

	var metadata struct {
		OriginalContent  string   `json:"originalContent"`
		ProcessedContent string   `json:"processedContent"`
		InputProcessors  []string `json:"inputProcessors"`
	}
	require.NoError(t, json.Unmarshal(msg.Metadata, &metadata))
	assert.Equal(t, "Meu CPF é 123.456.789-00", metadata.OriginalContent)
	assert.Equal(t, "MEU CPF É [REDACTED:CPF]", metadata.ProcessedContent)
	assert.Equal(t, []string{"upper_caser", "pii_redactor"}, metadata.InputProcessors)

	resp := chat.MessageResponseFrom(msg)
	require.NotNil(t, resp.OriginalContent)
	require.NotNil(t, resp.ProcessedContent)
	assert.Equal(t, metadata.OriginalContent, *resp.OriginalContent)
	assert.Equal(t, metadata.ProcessedContent, *resp.ProcessedContent)
}

func TestRunSession_PassesOutputProcessorsToRunner(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}

	agentID := uuid.New()
	sessionID := uuid.New()
	repo.sessions[sessionID] = chat.ChatSession{
		ID:      sessionID,
		AgentID: &agentID,
		Status:  chat.StatusActive,
	}
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		ID:               agentID,
		Status:           "PUBLISHED",
		OutputProcessors: []string{"pii_redactor"},
	}}
	svc := chat.NewService(repo, runner).WithAgentLoader(loader)

	_, err := svc.RunSession(context.Background(), sessionID, "Hello", "tenant")

	require.NoError(t, err)
	assert.Equal(t, []string{"pii_redactor"}, runner.lastInput.OutputProcessors)
	assert.Same(t, loader.cfg, runner.lastInput.AgentConfig)
}

// TestCreateSession_SnapshotsSkillBindings verifies that skill IDs from the agent loader
// are captured as a JSON snapshot in the session at creation time. P-C115-1.
func TestCreateSession_SnapshotsSkillBindings(t *testing.T) {
	repo := newMockRepo()
	skillID1 := uuid.New()
	skillID2 := uuid.New()
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: "You are helpful.",
		Status:       "PUBLISHED",
		SkillIDs:     []uuid.UUID{skillID1, skillID2},
	}}
	svc := chat.NewService(repo, nil).WithAgentLoader(loader)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	stored := repo.sessions[session.ID]
	require.NotEmpty(t, stored.SkillBindingsSnapshot, "skill bindings snapshot must be stored")

	var snap chat.SkillBindingsSnapshotData
	require.NoError(t, json.Unmarshal(stored.SkillBindingsSnapshot, &snap))
	assert.ElementsMatch(t, []uuid.UUID{skillID1, skillID2}, snap.SkillIDs)
}

// TestRunSession_PassesSkillSnapshotToRunner verifies that the runner receives
// the skill IDs extracted from the session's snapshot. P-C115-1.
func TestRunSession_PassesSkillSnapshotToRunner(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}

	skillID := uuid.New()
	snapData := chat.SkillBindingsSnapshotData{SkillIDs: []uuid.UUID{skillID}}
	snapJSON, err := json.Marshal(snapData)
	require.NoError(t, err)

	agentID := uuid.New()
	sessionID := uuid.New()
	repo.sessions[sessionID] = chat.ChatSession{
		ID:                    sessionID,
		AgentID:               &agentID,
		Status:                chat.StatusActive,
		SkillBindingsSnapshot: snapJSON,
	}

	svc := chat.NewService(repo, runner)
	_, err = svc.RunSession(context.Background(), sessionID, "Hello", "tenant")
	require.NoError(t, err)

	assert.Equal(t, []uuid.UUID{skillID}, runner.lastInput.SkillIDsSnapshot,
		"runner must receive the snapshotted skill IDs")
}

// TestRunSession_EmptyMessage_NoMessagePersisted verifies that when the user
// sends an empty message no spurious record is written to the repository.
func TestRunSession_EmptyMessage_NoMessagePersisted(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}
	svc := chat.NewService(repo, runner)

	agentID := uuid.New()
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "test",
	})
	require.NoError(t, err)

	_, err = svc.RunSession(context.Background(), session.ID, "", "tenant")
	require.NoError(t, err)

	// No message should have been written for an empty user message.
	assert.Empty(t, repo.messages, "no message should be persisted when userMessage is empty")
	assert.Nil(t, runner.lastInput.UserMessageID, "UserMessageID must be nil when userMessage is empty")
}
