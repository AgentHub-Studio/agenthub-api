package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sharedCancellationRepo models two API/worker processes sharing the same
// persisted run state. The handler writes the cancellation through one
// executor while another executor later receives the queued task.
type sharedCancellationRepo struct {
	*asyncExecutorRepoStub

	mu       sync.Mutex
	statuses map[uuid.UUID]ChatRunStatus
	history  []ChatRunStatus
}

func newSharedCancellationRepo(session ChatSession, run ChatRun) *sharedCancellationRepo {
	return &sharedCancellationRepo{
		asyncExecutorRepoStub: &asyncExecutorRepoStub{
			session:    session,
			createdRun: run,
		},
		statuses: map[uuid.UUID]ChatRunStatus{run.ID: run.Status},
	}
}

func (r *sharedCancellationRepo) GetRunByID(_ context.Context, id uuid.UUID) (ChatRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createdRun.ID != id {
		return ChatRun{}, ErrNotFound
	}
	run := r.createdRun
	run.Status = r.statuses[id]
	return run, nil
}

func (r *sharedCancellationRepo) UpdateRunStatus(_ context.Context, id uuid.UUID, status ChatRunStatus, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createdRun.ID != id {
		return ErrNotFound
	}
	r.statuses[id] = status
	r.history = append(r.history, status)
	return nil
}

func (r *sharedCancellationRepo) statusSnapshot(id uuid.UUID) (ChatRunStatus, []ChatRunStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statuses[id], append([]ChatRunStatus(nil), r.history...)
}

type cancellationProbeRunner struct {
	calls atomic.Int32
}

func (r *cancellationProbeRunner) RunSession(context.Context, RunInput) (<-chan RunEvent, error) {
	r.calls.Add(1)
	events := make(chan RunEvent)
	close(events)
	return events, nil
}

func TestAsyncExecutor_ProcessTaskSkipsPersistentlyCancelledRun(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := newSharedCancellationRepo(
		ChatSession{ID: sessionID, AgentID: &agentID},
		ChatRun{ID: runID, SessionID: sessionID, Status: ChatRunStatusQueued},
	)

	// Simulate the API pod accepting a cancel before the RabbitMQ worker has
	// registered an in-flight cancel function for this task.
	apiExecutor := NewAsyncExecutor(repo, nil, "")
	router := chi.NewRouter()
	NewHandler(nil, apiExecutor).RegisterRoutes(router)
	cancelReq := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/run/"+runID.String()+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	router.ServeHTTP(cancelRec, cancelReq)
	require.Equal(t, http.StatusOK, cancelRec.Code, cancelRec.Body.String())

	status, history := repo.statusSnapshot(runID)
	require.Equal(t, ChatRunStatusCancelled, status)
	require.Equal(t, []ChatRunStatus{ChatRunStatusCancelled}, history)

	// A different worker process receives the already-cancelled task. It must
	// not reactivate the run or invoke the model/runner.
	runner := &cancellationProbeRunner{}
	workerExecutor := NewAsyncExecutor(repo, runner, "")
	workerExecutor.processTask(ChatRunTask{
		RunID:     runID,
		SessionID: sessionID,
		TenantID:  "test",
		Message:   "must not reach the model",
	})

	assert.Zero(t, runner.calls.Load(), "a persistently cancelled run must not invoke the runner")
	status, history = repo.statusSnapshot(runID)
	assert.Equal(t, ChatRunStatusCancelled, status)
	assert.Equal(t, []ChatRunStatus{ChatRunStatusCancelled}, history)
}

func TestHandler_CancelRunAbortsInFlightAsyncWorker(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	repo := newSharedCancellationRepo(
		ChatSession{ID: sessionID, AgentID: &agentID},
		ChatRun{ID: runID, SessionID: sessionID, Status: ChatRunStatusQueued},
	)
	runner := &blockingAsyncExecutorRunnerStub{
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
	executor := NewAsyncExecutor(repo, runner, "")
	executor.runTimeout = time.Minute
	handler := NewHandler(nil, executor)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	executor.startTask(ChatRunTask{
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

	cancelReq := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/run/"+runID.String()+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	router.ServeHTTP(cancelRec, cancelReq)
	require.Equal(t, http.StatusOK, cancelRec.Code, cancelRec.Body.String())
	assert.JSONEq(t, `{"status":"cancelled"}`, cancelRec.Body.String())

	select {
	case <-runner.done:
	case <-time.After(time.Second):
		t.Fatal("runner did not observe HTTP cancellation")
	}

	buf := handler.bufferRegistry.Get(runID.String())
	require.NotNil(t, buf)
	require.Eventually(t, buf.IsDone, time.Second, 10*time.Millisecond)
	status, history := repo.statusSnapshot(runID)
	assert.Equal(t, ChatRunStatusCancelled, status)
	assert.Equal(t, []ChatRunStatus{ChatRunStatusActive, ChatRunStatusCancelled}, history)
}
