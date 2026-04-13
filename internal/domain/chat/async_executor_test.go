package chat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type asyncExecutorRepoStub struct {
	session      ChatSession
	active       bool
	createdRun   ChatRun
	completedID  uuid.UUID
	failedID     uuid.UUID
	failedReason string
}

func (r *asyncExecutorRepoStub) FindSessions(context.Context, pagination.PageRequest) ([]ChatSession, int64, error) {
	return nil, 0, nil
}

func (r *asyncExecutorRepoStub) GetSessionListStamp(context.Context) (ChatSessionListStamp, error) {
	return ChatSessionListStamp{}, nil
}

func (r *asyncExecutorRepoStub) GetSessionByID(_ context.Context, id uuid.UUID) (ChatSession, error) {
	if r.session.ID == id {
		return r.session, nil
	}
	return ChatSession{}, ErrNotFound
}

func (r *asyncExecutorRepoStub) CreateSession(context.Context, ChatSession) (ChatSession, error) {
	return ChatSession{}, nil
}

func (r *asyncExecutorRepoStub) UpdateSessionStatus(context.Context, uuid.UUID, ChatStatus) (ChatSession, error) {
	return ChatSession{}, nil
}

func (r *asyncExecutorRepoStub) UpdateSessionTitle(context.Context, uuid.UUID, string) (ChatSession, error) {
	return ChatSession{}, nil
}

func (r *asyncExecutorRepoStub) UpdateSessionAgent(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (r *asyncExecutorRepoStub) UpdateSessionConfigHash(context.Context, uuid.UUID, string) error {
	return nil
}

func (r *asyncExecutorRepoStub) FindDefaultAgentID(context.Context) (*uuid.UUID, error) {
	return nil, nil
}

func (r *asyncExecutorRepoStub) FindAgentsForRouting(context.Context) ([]AgentRoutingInfo, error) {
	return nil, nil
}

func (r *asyncExecutorRepoStub) DeleteSession(context.Context, uuid.UUID) error {
	return nil
}

func (r *asyncExecutorRepoStub) FindMessages(context.Context, uuid.UUID, pagination.PageRequest) ([]ChatMessage, int64, error) {
	return nil, 0, nil
}

func (r *asyncExecutorRepoStub) CreateMessage(_ context.Context, m ChatMessage) (ChatMessage, error) {
	return m, nil
}

func (r *asyncExecutorRepoStub) GetLatestAssistantMessage(context.Context, uuid.UUID, time.Time) (ChatMessage, bool, error) {
	return ChatMessage{}, false, nil
}

func (r *asyncExecutorRepoStub) FindAllMessages(context.Context, uuid.UUID) ([]ChatMessage, error) {
	return nil, nil
}

func (r *asyncExecutorRepoStub) GetLatestCompactSummary(context.Context, uuid.UUID) (ChatMessage, bool, error) {
	return ChatMessage{}, false, nil
}

func (r *asyncExecutorRepoStub) CreateRun(_ context.Context, run ChatRun) (ChatRun, error) {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	run.CreatedAt = time.Now()
	r.createdRun = run
	return run, nil
}

func (r *asyncExecutorRepoStub) GetRunByID(_ context.Context, id uuid.UUID) (ChatRun, error) {
	if r.createdRun.ID == id {
		return r.createdRun, nil
	}
	return ChatRun{}, ErrNotFound
}

func (r *asyncExecutorRepoStub) GetActiveRunBySession(_ context.Context, _ uuid.UUID) (ChatRun, bool, error) {
	return ChatRun{}, r.active, nil
}

func (r *asyncExecutorRepoStub) UpdateRunStatus(context.Context, uuid.UUID, ChatRunStatus, string) error {
	return nil
}

func (r *asyncExecutorRepoStub) MarkRunCompleted(_ context.Context, id uuid.UUID) error {
	r.completedID = id
	return nil
}

func (r *asyncExecutorRepoStub) MarkRunFailed(_ context.Context, id uuid.UUID, reason string) error {
	r.failedID = id
	r.failedReason = reason
	return nil
}

func (r *asyncExecutorRepoStub) UpdateRunMetadata(context.Context, uuid.UUID, json.RawMessage) error {
	return nil
}

type asyncExecutorRunnerStub struct {
	events []RunEvent
	err    error
}

func (r *asyncExecutorRunnerStub) RunSession(_ context.Context, _ RunInput) (<-chan RunEvent, error) {
	if r.err != nil {
		return nil, r.err
	}
	ch := make(chan RunEvent, len(r.events))
	for _, ev := range r.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestNewHandler_WiresExecutorBufferRegistry(t *testing.T) {
	exec := NewAsyncExecutor(nil, nil, "")
	handler := NewHandler(nil, exec)

	require.NotNil(t, handler.bufferRegistry)
	assert.Same(t, handler.bufferRegistry, exec.bufferRegistry)
}

func TestAsyncExecutor_EnqueueRun_CreatesBuffer(t *testing.T) {
	repo := &asyncExecutorRepoStub{}
	reg := NewRunEventBufferRegistry()
	exec := NewAsyncExecutor(repo, nil, "").WithEventBufferRegistry(reg)

	runID, err := exec.EnqueueRun(context.Background(), uuid.New(), "test", "hello")

	require.NoError(t, err)
	assert.Equal(t, 1, reg.Len())
	require.NotNil(t, reg.Get(runID.String()))
}

func TestAsyncExecutor_ProcessTask_BuffersEventsForResume(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{
			ID:      sessionID,
			AgentID: &agentID,
		},
	}
	runner := &asyncExecutorRunnerStub{
		events: []RunEvent{
			{Type: "text_delta", Data: json.RawMessage(`{"content":"hello"}`)},
			{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":1}`)},
		},
	}
	reg := NewRunEventBufferRegistry()
	exec := NewAsyncExecutor(repo, runner, "").WithEventBufferRegistry(reg)

	exec.processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "hello",
	})

	buf := reg.Get(runID.String())
	require.NotNil(t, buf)
	assert.True(t, buf.IsDone())
	assert.Equal(t, 2, buf.Len())

	events, ok := buf.EventsSince(0)
	require.True(t, ok)
	require.Len(t, events, 2)
	assert.Equal(t, "text_delta", events[0].Event.Type)
	assert.Equal(t, "run_complete", events[1].Event.Type)
	assert.Equal(t, runID, repo.completedID)
	assert.Equal(t, uuid.Nil, repo.failedID)
}
