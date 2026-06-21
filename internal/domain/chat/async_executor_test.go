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
	session       ChatSession
	active        bool
	createdRun    ChatRun
	completedID   uuid.UUID
	failedID      uuid.UUID
	failedReason  string
	routingAgents []AgentRoutingInfo
	routingErr    error
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

func (r *asyncExecutorRepoStub) UpdateSessionSnapshots(context.Context, uuid.UUID, *string, json.RawMessage, json.RawMessage, string) error {
	return nil
}

func (r *asyncExecutorRepoStub) FindAgentsForRouting(context.Context) ([]AgentRoutingInfo, error) {
	return r.routingAgents, r.routingErr
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
	input  RunInput
}

func (r *asyncExecutorRunnerStub) RunSession(_ context.Context, input RunInput) (<-chan RunEvent, error) {
	r.input = input
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

type asyncExecutorVoiceStub struct {
	synthesis VoiceSynthesisInput
	calls     int
}

func (v *asyncExecutorVoiceStub) Transcribe(context.Context, VoiceTranscriptionInput) (VoiceTranscription, error) {
	return VoiceTranscription{}, nil
}

func (v *asyncExecutorVoiceStub) Synthesize(_ context.Context, in VoiceSynthesisInput) (VoiceAudio, error) {
	v.calls++
	v.synthesis = in
	return VoiceAudio{Format: "mp3", Base64: "YXVkaW8="}, nil
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

type asyncMetricsCollectorStub struct {
	events  []RunEvent
	flushed bool
}

func (c *asyncMetricsCollectorStub) Collect(event RunEvent) {
	c.events = append(c.events, event)
}

func (c *asyncMetricsCollectorStub) Flush() error {
	c.flushed = true
	return nil
}

type asyncMetricsFactoryStub struct {
	collector *asyncMetricsCollectorStub
	tenantID  string
	agentID   uuid.UUID
	sessionID uuid.UUID
	runID     string
	provider  string
	model     string
}

func (f *asyncMetricsFactoryStub) NewCollector(tenantID string, agentID, sessionID uuid.UUID, runID, provider, model string) RunMetricsCollector {
	f.tenantID = tenantID
	f.agentID = agentID
	f.sessionID = sessionID
	f.runID = runID
	f.provider = provider
	f.model = model
	return f.collector
}

func TestAsyncExecutor_ProcessTask_EmitsVoiceAudioBeforeRunComplete(t *testing.T) {
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
			{Type: "text_delta", Data: json.RawMessage(`{"content":"hello "}`)},
			{Type: "text_delta", Data: json.RawMessage(`{"content":"world"}`)},
			{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":1}`)},
		},
	}
	voice := &asyncExecutorVoiceStub{}
	reg := NewRunEventBufferRegistry()
	exec := NewAsyncExecutor(repo, runner, "").
		WithEventBufferRegistry(reg).
		WithVoiceService(voice)

	exec.processTask(ChatRunTask{
		RunID:       runID,
		SessionID:   sessionID,
		TenantID:    "test",
		Message:     "hello",
		VoiceOutput: true,
	})

	buf := reg.Get(runID.String())
	require.NotNil(t, buf)
	events, ok := buf.EventsSince(0)
	require.True(t, ok)
	require.Len(t, events, 4)
	assert.Equal(t, "text_delta", events[0].Event.Type)
	assert.Equal(t, "text_delta", events[1].Event.Type)
	assert.Equal(t, EventAudioDelta, events[2].Event.Type)
	assert.Equal(t, "run_complete", events[3].Event.Type)
	assert.Equal(t, 1, voice.calls)
	assert.Equal(t, "hello world", voice.synthesis.Text)

	var audio VoiceAudioDelta
	require.NoError(t, json.Unmarshal(events[2].Event.Data, &audio))
	assert.Equal(t, "mp3", audio.Format)
	assert.Equal(t, "YXVkaW8=", audio.Chunk)
}

func TestAsyncExecutor_ProcessTask_RecordsRunMetrics(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{
			ID:                  sessionID,
			AgentID:             &agentID,
			ModelConfigSnapshot: json.RawMessage(`{"provider":"anthropic","model":"claude-sonnet"}`),
		},
	}
	runner := &asyncExecutorRunnerStub{
		events: []RunEvent{
			{Type: "turn_complete", Data: json.RawMessage(`{"turnIndex":0}`)},
			{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":1,"totalTokens":42}`)},
		},
	}
	collector := &asyncMetricsCollectorStub{}
	factory := &asyncMetricsFactoryStub{collector: collector}
	exec := NewAsyncExecutor(repo, runner, "").
		WithRunMetricsCollectorFactory(factory)

	exec.processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "ah_test",
		Message:   "hello",
	})

	assert.Equal(t, "test", factory.tenantID)
	assert.Equal(t, agentID, factory.agentID)
	assert.Equal(t, sessionID, factory.sessionID)
	assert.Equal(t, runID.String(), factory.runID)
	assert.Equal(t, "anthropic", factory.provider)
	assert.Equal(t, "claude-sonnet", factory.model)
	require.Len(t, collector.events, 2)
	assert.Equal(t, "turn_complete", collector.events[0].Type)
	assert.Equal(t, "run_complete", collector.events[1].Type)
	assert.True(t, collector.flushed)
	assert.Equal(t, runID, repo.completedID)
}
