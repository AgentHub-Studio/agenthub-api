package chat

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RunStatus represents the current state of a backgrounded run.
type RunStatus string

const (
	RunStatusActive    RunStatus = "active"
	RunStatusCompleted RunStatus = "completed"
	RunStatusCancelled RunStatus = "cancelled"
)

// BackgroundRun tracks a single backgrounded agentic run.
type BackgroundRun struct {
	RunID       string    `json:"runId"`
	SessionID   uuid.UUID `json:"sessionId"`
	Status      RunStatus `json:"status"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`

	// ctx is the independent context for this run (not tied to HTTP request).
	ctx    context.Context
	cancel context.CancelFunc

	// events is the output channel from the Runner.
	events <-chan RunEvent
}

// BackgroundRunRegistry tracks active agentic runs that continue after
// client disconnection. This allows SSE reconnection and run status queries.
type BackgroundRunRegistry struct {
	mu   sync.Mutex
	runs map[string]*BackgroundRun // keyed by runID

	// ttl is how long completed runs are kept before cleanup.
	ttl time.Duration
}

// NewBackgroundRunRegistry creates a registry with the given TTL for completed runs.
// If ttl <= 0, defaults to 5 minutes.
func NewBackgroundRunRegistry(ttl time.Duration) *BackgroundRunRegistry {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	reg := &BackgroundRunRegistry{
		runs: make(map[string]*BackgroundRun),
		ttl:  ttl,
	}
	go reg.cleanupLoop()
	return reg
}

// Register creates a new background run with an independent context.
// Returns the run and a function that starts the runner with the decoupled context.
// The returned context is NOT tied to any HTTP request — the run continues
// even if the client disconnects.
func (reg *BackgroundRunRegistry) Register(sessionID uuid.UUID) (runID string, runCtx context.Context) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	id := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	run := &BackgroundRun{
		RunID:     id,
		SessionID: sessionID,
		Status:    RunStatusActive,
		StartedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
	reg.runs[id] = run

	return id, ctx
}

// AttachEvents associates the Runner's event channel with the run.
func (reg *BackgroundRunRegistry) AttachEvents(runID string, events <-chan RunEvent) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if run, ok := reg.runs[runID]; ok {
		run.events = events
	}
}

// Get retrieves a background run by ID.
func (reg *BackgroundRunRegistry) Get(runID string) *BackgroundRun {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.runs[runID]
}

// GetBySession retrieves the active run for a session, if any.
func (reg *BackgroundRunRegistry) GetBySession(sessionID uuid.UUID) *BackgroundRun {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	for _, run := range reg.runs {
		if run.SessionID == sessionID && run.Status == RunStatusActive {
			return run
		}
	}
	return nil
}

// MarkCompleted transitions a run to completed status.
func (reg *BackgroundRunRegistry) MarkCompleted(runID string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if run, ok := reg.runs[runID]; ok {
		if run.Status != RunStatusActive {
			return
		}
		run.Status = RunStatusCompleted
		run.CompletedAt = time.Now()
	}
}

// Cancel aborts a running run.
func (reg *BackgroundRunRegistry) Cancel(runID string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if run, ok := reg.runs[runID]; ok && run.Status == RunStatusActive {
		run.cancel()
		run.Status = RunStatusCancelled
		run.CompletedAt = time.Now()
	}
}

// ActiveRuns returns the count of currently active runs.
func (reg *BackgroundRunRegistry) ActiveRuns() int {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	count := 0
	for _, run := range reg.runs {
		if run.Status == RunStatusActive {
			count++
		}
	}
	return count
}

// Events returns the event channel for a run. May be nil if not yet attached.
func (reg *BackgroundRunRegistry) Events(runID string) <-chan RunEvent {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if run, ok := reg.runs[runID]; ok {
		return run.events
	}
	return nil
}

// cleanupLoop periodically removes completed runs older than TTL.
func (reg *BackgroundRunRegistry) cleanupLoop() {
	ticker := time.NewTicker(reg.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		reg.cleanup()
	}
}

func (reg *BackgroundRunRegistry) cleanup() {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	cutoff := time.Now().Add(-reg.ttl)
	for id, run := range reg.runs {
		if isTerminalRunStatus(run.Status) && run.CompletedAt.Before(cutoff) {
			delete(reg.runs, id)
		}
	}
}

func isTerminalRunStatus(status RunStatus) bool {
	return status == RunStatusCompleted || status == RunStatusCancelled
}
