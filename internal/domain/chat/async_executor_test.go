package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type asyncExecutorRepoStub struct {
	session       ChatSession
	active        bool
	createRunErr  error
	createdRun    ChatRun
	completedID   uuid.UUID
	failedID      uuid.UUID
	failedReason  string
	statusID      uuid.UUID
	status        ChatRunStatus
	statusReason  string
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

func (r *asyncExecutorRepoStub) CloneSession(context.Context, ChatSession, []ChatMessage) (ChatSession, error) {
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

func (r *asyncExecutorRepoStub) UpdateSessionSnapshots(context.Context, uuid.UUID, *string, json.RawMessage, json.RawMessage, json.RawMessage, *string) error {
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
	if r.createRunErr != nil {
		return ChatRun{}, r.createRunErr
	}
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

func (r *asyncExecutorRepoStub) UpdateRunStatus(_ context.Context, id uuid.UUID, status ChatRunStatus, reason string) error {
	r.statusID = id
	r.status = status
	r.statusReason = reason
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

type blockingAsyncExecutorRunnerStub struct {
	started chan struct{}
	done    chan struct{}
}

func (r *blockingAsyncExecutorRunnerStub) RunSession(ctx context.Context, _ RunInput) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	close(r.started)
	go func() {
		defer close(r.done)
		defer close(ch)
		<-ctx.Done()
		payload, _ := json.Marshal(map[string]string{
			"code":    "context_cancelled",
			"message": ctx.Err().Error(),
		})
		ch <- RunEvent{Type: "error", Data: payload}
	}()
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

func TestAsyncExecutor_EnqueueRun_QueueDialFailureMarksPersistedRunFailedAndRemovesBuffer(t *testing.T) {
	repo := &asyncExecutorRepoStub{}
	reg := NewRunEventBufferRegistry()
	exec := NewAsyncExecutor(repo, nil, "amqp://guest:guest@127.0.0.1:1/").WithEventBufferRegistry(reg)

	runID, err := exec.EnqueueRun(context.Background(), uuid.New(), "test", "hello")

	require.Error(t, err)
	require.ErrorIs(t, err, ErrQueueUnavailable)
	assert.NotEqual(t, uuid.Nil, runID)
	assert.Equal(t, runID, repo.createdRun.ID)
	assert.Equal(t, runID, repo.failedID)
	assert.Equal(t, "chat queue is temporarily unavailable", repo.failedReason)
	assert.Equal(t, 0, reg.Len())
}

func TestChatHandler_RunSession_QueueUnavailableReturnsServiceUnavailable(t *testing.T) {
	repo := &asyncExecutorRepoStub{}
	exec := NewAsyncExecutor(repo, nil, "amqp://guest:guest@127.0.0.1:1/")
	handler := NewHandler(nil, exec)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	sessionID := uuid.New()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/run",
		bytes.NewBufferString(`{"message":"hello"}`),
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "chat queue is temporarily unavailable")
	assert.NotContains(t, w.Body.String(), "rabbitmq")
	assert.Equal(t, repo.createdRun.ID, repo.failedID)
}

func TestChatHandler_RunSession_EnqueueFailureDoesNotLeakInfrastructureDetail(t *testing.T) {
	repo := &asyncExecutorRepoStub{createRunErr: errors.New("dial tcp 10.42.0.19:5432: connect: connection refused")}
	exec := NewAsyncExecutor(repo, nil, "amqp://guest:guest@127.0.0.1:1/")
	handler := NewHandler(nil, exec)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chat/sessions/"+uuid.New().String()+"/run",
		bytes.NewBufferString(`{"message":"hello"}`),
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to enqueue run")
	assert.NotContains(t, w.Body.String(), "10.42.0.19")
	assert.NotContains(t, w.Body.String(), "connection refused")
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

func TestAsyncExecutor_ProcessTask_RedactsSensitiveErrorEventDataFromLogs(t *testing.T) {
	const authorizationSecret = "async-log-authorization-secret"
	const passwordSecret = "async-log-password-secret"
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{ID: sessionID, AgentID: &agentID},
	}
	runner := &asyncExecutorRunnerStub{
		events: []RunEvent{
			{Type: "error", Data: json.RawMessage(`{"code":"llm_call","authorization":"Bearer ` + authorizationSecret + `","credentials":{"password":"` + passwordSecret + `"},"message":"Authorization: Bearer ` + authorizationSecret + `\npassword=` + passwordSecret + `"}`)},
		},
	}

	NewAsyncExecutor(repo, runner, "").processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "redact diagnostic logs",
	})

	output := logs.String()
	assert.NotContains(t, output, authorizationSecret)
	assert.NotContains(t, output, passwordSecret)
	assert.NotContains(t, output, "Authorization:")
	assert.NotContains(t, output, "authorization")
	assert.NotContains(t, output, "password")
	assert.NotContains(t, output, "password=")
	assert.Contains(t, output, "[REDACTED]")
}

func TestAsyncExecutor_ProcessTask_RedactsSensitiveStartupErrorFromLogs(t *testing.T) {
	const authorizationSecret = "async-startup-authorization-secret"
	const passwordSecret = "async-startup-password-secret"
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{ID: sessionID, AgentID: &agentID},
	}
	runner := &asyncExecutorRunnerStub{
		err: errors.New("Authorization: Bearer " + authorizationSecret + "\npassword=" + passwordSecret),
	}

	NewAsyncExecutor(repo, runner, "").processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "redact startup failure logs",
	})

	output := logs.String()
	assert.NotContains(t, output, authorizationSecret)
	assert.NotContains(t, output, passwordSecret)
	assert.NotContains(t, output, "Authorization:")
	assert.NotContains(t, output, "password")
	assert.Contains(t, output, "[REDACTED]")
}

func TestAsyncExecutor_ProcessTask_RedactsSensitiveStartupErrorFromBufferedSSE(t *testing.T) {
	const authorizationSecret = "async-startup-sse-authorization-secret"
	const passwordSecret = "async-startup-sse-password-secret"
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{ID: sessionID, AgentID: &agentID},
	}
	runner := &asyncExecutorRunnerStub{
		err: errors.New("Authorization: Bearer " + authorizationSecret + "\npassword=" + passwordSecret),
	}
	registry := NewRunEventBufferRegistry()

	NewAsyncExecutor(repo, runner, "").WithEventBufferRegistry(registry).processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "redact startup failure SSE",
	})

	buffer := registry.Get(runID.String())
	require.NotNil(t, buffer)
	events, ok := buffer.EventsSince(0)
	require.True(t, ok)
	require.Len(t, events, 1)
	payload := string(events[0].Event.Data)
	assert.Equal(t, "error", events[0].Event.Type)
	assert.NotContains(t, payload, authorizationSecret)
	assert.NotContains(t, payload, passwordSecret)
	assert.NotContains(t, payload, "Authorization:")
	assert.NotContains(t, payload, "password")
	assert.Contains(t, payload, "[REDACTED]")
}

func TestAsyncExecutor_ProcessTask_FiresCompletionHookWithPersistedUsage(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{
			ID:      sessionID,
			AgentID: &agentID,
		},
		createdRun: ChatRun{
			ID:        runID,
			SessionID: sessionID,
			Metadata: json.RawMessage(`{
				"totalTurns": 3,
				"totalInputTokens": 11,
				"totalOutputTokens": 7
			}`),
		},
	}
	runner := &asyncExecutorRunnerStub{
		events: []RunEvent{{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":3}`)}},
	}
	executor := NewAsyncExecutor(repo, runner, "")

	type completion struct {
		sessionID uuid.UUID
		runID     uuid.UUID
		status    ChatRunStatus
		turns     int
		tokens    int
		errMsg    string
	}
	completed := make(chan completion, 1)
	executor.WithCompletionHook(func(_ context.Context, gotSessionID, gotRunID uuid.UUID, status ChatRunStatus, turns, tokens int, errMsg string) {
		completed <- completion{
			sessionID: gotSessionID,
			runID:     gotRunID,
			status:    status,
			turns:     turns,
			tokens:    tokens,
			errMsg:    errMsg,
		}
	})

	executor.processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "finish the triggered run",
	})

	select {
	case got := <-completed:
		assert.Equal(t, sessionID, got.sessionID)
		assert.Equal(t, runID, got.runID)
		assert.Equal(t, ChatRunStatusCompleted, got.status)
		assert.Equal(t, 3, got.turns)
		assert.Equal(t, 18, got.tokens)
		assert.Empty(t, got.errMsg)
	case <-time.After(time.Second):
		t.Fatal("completion hook was not invoked")
	}
	assert.Equal(t, runID, repo.completedID)
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

func TestAsyncExecutor_ProcessTask_UsesSessionVoiceSettingsFromModelConfig(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{
			ID:      sessionID,
			AgentID: &agentID,
			ModelConfigSnapshot: json.RawMessage(`{
				"provider":"openai",
				"voice":{"enabled":true,"ttsModel":"gpt-4o-mini-tts","ttsVoice":"nova","language":"en"}
			}`),
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

	require.Equal(t, 1, voice.calls)
	assert.Equal(t, "hello world", voice.synthesis.Text)
	assert.Equal(t, "gpt-4o-mini-tts", voice.synthesis.Model)
	assert.Equal(t, "nova", voice.synthesis.Voice)
	assert.Equal(t, "en", voice.synthesis.Language)
}

func TestAsyncExecutor_ProcessTask_RespectsVoiceEnabledFalse(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{
			ID:                  sessionID,
			AgentID:             &agentID,
			ModelConfigSnapshot: json.RawMessage(`{"voice":{"enabled":false,"ttsModel":"gpt-4o-mini-tts"}}`),
		},
	}
	runner := &asyncExecutorRunnerStub{
		events: []RunEvent{
			{Type: "text_delta", Data: json.RawMessage(`{"content":"hello"}`)},
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
	require.Len(t, events, 2)
	assert.Equal(t, "text_delta", events[0].Event.Type)
	assert.Equal(t, "run_complete", events[1].Event.Type)
	assert.Equal(t, 0, voice.calls)
}

func TestAsyncExecutor_Shutdown_CancelsInProgressTaskAndClosesBuffer(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := &asyncExecutorRepoStub{
		session: ChatSession{
			ID:      sessionID,
			AgentID: &agentID,
		},
	}
	runner := &blockingAsyncExecutorRunnerStub{
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
	reg := NewRunEventBufferRegistry()
	exec := NewAsyncExecutor(repo, runner, "").WithEventBufferRegistry(reg)
	exec.runTimeout = time.Minute

	exec.startTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "keep working",
	})

	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, exec.Shutdown(shutdownCtx))

	select {
	case <-runner.done:
	case <-time.After(time.Second):
		t.Fatal("runner did not observe shutdown cancellation")
	}

	assert.Equal(t, runID, repo.statusID)
	assert.Equal(t, ChatRunStatusCancelled, repo.status)
	assert.Equal(t, uuid.Nil, repo.completedID)
	assert.Equal(t, uuid.Nil, repo.failedID)

	buf := reg.Get(runID.String())
	require.NotNil(t, buf)
	assert.True(t, buf.IsDone())
	events, ok := buf.EventsSince(0)
	require.True(t, ok)
	require.NotEmpty(t, events)
	assert.Equal(t, "error", events[0].Event.Type)
}
