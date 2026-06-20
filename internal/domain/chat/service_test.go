package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockChatRepo struct {
	sessions      map[uuid.UUID]chat.ChatSession
	messages      []chat.ChatMessage
	routingAgents []chat.AgentRoutingInfo
	routingErr    error
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

func (m *mockChatRepo) UpdateSessionSnapshots(_ context.Context, sessionID uuid.UUID, systemPrompt *string, modelConfig, skillBindings json.RawMessage, configHash string) error {
	s, ok := m.sessions[sessionID]
	if !ok {
		return chat.ErrNotFound
	}
	s.SystemPromptSnapshot = systemPrompt
	s.ModelConfigSnapshot = modelConfig
	s.SkillBindingsSnapshot = skillBindings
	if configHash != "" {
		s.ConfigHash = &configHash
	} else {
		s.ConfigHash = nil
	}
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

// --- mock SessionRunner ---

type mockSessionRunner struct {
	events    []chat.RunEvent
	err       error
	lastInput chat.RunInput
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
	cfg *chat.AgentRunConfig
	err error
}

func (m *mockAgentLoader) GetAgentForRun(_ context.Context, _ uuid.UUID) (*chat.AgentRunConfig, error) {
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

func TestCreateSession_SnapshotsRedactedModelConfigAndHash(t *testing.T) {
	repo := newMockRepo()
	rawConfig := json.RawMessage(`{"provider":"openai","model":"gpt-4o","apiKey":"sk-secret"}`)
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: "You are helpful.",
		ModelConfig:  rawConfig,
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
	assert.NotContains(t, string(stored.ModelConfigSnapshot), "apiKey")
	assert.NotContains(t, string(stored.ModelConfigSnapshot), "sk-secret")
	require.NotNil(t, stored.ConfigHash)
	assert.Equal(t, chat.HashModelConfig(rawConfig), *stored.ConfigHash)
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

func TestRunSession_CapturesMissingSnapshotOnFirstRun(t *testing.T) {
	repo := newMockRepo()
	runner := &mockSessionRunner{}
	agentID := uuid.New()
	skillID := uuid.New()
	sessionID := uuid.New()
	repo.sessions[sessionID] = chat.ChatSession{
		ID:      sessionID,
		AgentID: &agentID,
		Status:  chat.StatusActive,
	}
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: "Pinned prompt",
		ModelConfig:  json.RawMessage(`{"provider":"openai","model":"gpt-4o"}`),
		Status:       "PUBLISHED",
		SkillIDs:     []uuid.UUID{skillID},
	}}
	svc := chat.NewService(repo, runner).WithAgentLoader(loader)

	_, err := svc.RunSession(context.Background(), sessionID, "Hello", "tenant")
	require.NoError(t, err)

	stored := repo.sessions[sessionID]
	require.NotNil(t, stored.SystemPromptSnapshot)
	assert.Equal(t, "Pinned prompt", *stored.SystemPromptSnapshot)
	assert.JSONEq(t, `{"provider":"openai","model":"gpt-4o"}`, string(stored.ModelConfigSnapshot))
	require.NotNil(t, stored.ConfigHash)
	assert.Equal(t, chat.HashModelConfig(json.RawMessage(`{"provider":"openai","model":"gpt-4o"}`)), *stored.ConfigHash)
	assert.Equal(t, []uuid.UUID{skillID}, runner.lastInput.SkillIDsSnapshot)
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

func TestCreateSession_SnapshotsEmptySkillBindings(t *testing.T) {
	repo := newMockRepo()
	loader := &mockAgentLoader{cfg: &chat.AgentRunConfig{
		SystemPrompt: "You are helpful.",
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
	require.NotEmpty(t, stored.SkillBindingsSnapshot)
	assert.JSONEq(t, `{"skillIds":[]}`, string(stored.SkillBindingsSnapshot))
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
