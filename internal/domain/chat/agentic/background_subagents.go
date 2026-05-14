package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SUB-008 — Background subagents.
//
// PDF arXiv:2604.14228v1 §8.5. Subordinate to a parent run, but the
// parent does NOT block on the result. The parent enqueues the job,
// continues its own loop, and may later poll, await, or cancel. Lets
// long-running research/coder/etc subagents work in parallel with the
// foreground reasoning.
//
// Distinct from neighbouring features:
//   - SUB-001 (Agent tool) is the synchronous spawn — parent awaits.
//   - SUB-002 (Built-in) / SUB-003 (Custom) supply the DEFINITION; this
//     feature is the EXECUTION mode (async vs sync).
//   - SUB-011 (Multi-agent coordination) orchestrates batches; a single
//     SUB-011 task may execute as SUB-008 background job.
//   - FUTURE-003 (BackgroundAgentRun) is a TOP-LEVEL agent loop the
//     platform initiates on its own (cron, threshold, event). SUB-008
//     is always subordinate to a parent run.

// BackgroundSubagentState bounded enum for the job lifecycle.
type BackgroundSubagentState string

const (
	BackgroundSubagentStateQueued     BackgroundSubagentState = "queued"
	BackgroundSubagentStateRunning    BackgroundSubagentState = "running"
	BackgroundSubagentStateSucceeded  BackgroundSubagentState = "succeeded"
	BackgroundSubagentStateFailed     BackgroundSubagentState = "failed"
	BackgroundSubagentStateCancelled  BackgroundSubagentState = "cancelled"
	BackgroundSubagentStateTimedOut   BackgroundSubagentState = "timed_out"
)

var allBackgroundSubagentStates = []BackgroundSubagentState{
	BackgroundSubagentStateQueued,
	BackgroundSubagentStateRunning,
	BackgroundSubagentStateSucceeded,
	BackgroundSubagentStateFailed,
	BackgroundSubagentStateCancelled,
	BackgroundSubagentStateTimedOut,
}

// IsValidBackgroundSubagentState returns true for the bounded set.
func IsValidBackgroundSubagentState(s BackgroundSubagentState) bool {
	for _, v := range allBackgroundSubagentStates {
		if s == v {
			return true
		}
	}
	return false
}

// AllBackgroundSubagentStates returns a defensive copy.
func AllBackgroundSubagentStates() []BackgroundSubagentState {
	out := make([]BackgroundSubagentState, len(allBackgroundSubagentStates))
	copy(out, allBackgroundSubagentStates)
	return out
}

// IsTerminalBackgroundSubagentState classifies states that can't
// transition further.
func IsTerminalBackgroundSubagentState(s BackgroundSubagentState) bool {
	switch s {
	case BackgroundSubagentStateSucceeded,
		BackgroundSubagentStateFailed,
		BackgroundSubagentStateCancelled,
		BackgroundSubagentStateTimedOut:
		return true
	default:
		return false
	}
}

// BackgroundSubagentJob is one async subagent task. SubagentSlug
// references either a SUB-002 builtin slug or a SUB-003 custom slug;
// the registry doesn't enforce that distinction — runner does.
type BackgroundSubagentJob struct {
	JobID           uuid.UUID
	ParentRunID     uuid.UUID
	OwnerTenantSlug string
	SubagentSlug    string
	State           BackgroundSubagentState
	EnqueuedAt      time.Time
	StartedAt       time.Time // zero until State≥Running
	CompletedAt     time.Time // zero until State terminal
	ResultSummary   string    // populated on Succeeded
	ErrorMsg        string    // populated on Failed/TimedOut
}

var backgroundSubagentSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ValidateForEnqueue checks the shape required when a job is being
// enqueued. JobID and ParentRunID must be non-zero; slugs kebab-case;
// EnqueuedAt non-zero; State must be Queued.
func (j BackgroundSubagentJob) ValidateForEnqueue() error {
	if j.JobID == uuid.Nil {
		return ErrBackgroundSubagentEmptyJobID
	}
	if j.ParentRunID == uuid.Nil {
		return ErrBackgroundSubagentEmptyParent
	}
	if !backgroundSubagentSlugRE.MatchString(j.OwnerTenantSlug) {
		return fmt.Errorf("%w: %q must be kebab-case",
			ErrBackgroundSubagentBadOwner, j.OwnerTenantSlug)
	}
	if !backgroundSubagentSlugRE.MatchString(j.SubagentSlug) {
		return fmt.Errorf("%w: %q must be kebab-case",
			ErrBackgroundSubagentBadSlug, j.SubagentSlug)
	}
	if j.EnqueuedAt.IsZero() {
		return ErrBackgroundSubagentEmptyEnqueuedAt
	}
	if j.State != BackgroundSubagentStateQueued {
		return fmt.Errorf("%w: enqueue must start in queued, got %q",
			ErrBackgroundSubagentBadInitialState, j.State)
	}
	return nil
}

// canTransition encodes the state machine. queued→running. running→
// any terminal. Terminal→nothing.
func canTransitionBackgroundSubagent(from, to BackgroundSubagentState) bool {
	if IsTerminalBackgroundSubagentState(from) {
		return false
	}
	switch from {
	case BackgroundSubagentStateQueued:
		return to == BackgroundSubagentStateRunning ||
			to == BackgroundSubagentStateCancelled
	case BackgroundSubagentStateRunning:
		return to == BackgroundSubagentStateSucceeded ||
			to == BackgroundSubagentStateFailed ||
			to == BackgroundSubagentStateCancelled ||
			to == BackgroundSubagentStateTimedOut
	}
	return false
}

// BackgroundSubagentRegistry is an in-memory store. Thread-safe.
type BackgroundSubagentRegistry struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]BackgroundSubagentJob
}

// NewBackgroundSubagentRegistry creates an empty registry.
func NewBackgroundSubagentRegistry() *BackgroundSubagentRegistry {
	return &BackgroundSubagentRegistry{
		jobs: map[uuid.UUID]BackgroundSubagentJob{},
	}
}

// Enqueue adds a fresh job in queued state.
func (r *BackgroundSubagentRegistry) Enqueue(j BackgroundSubagentJob) error {
	if err := j.ValidateForEnqueue(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.jobs[j.JobID]; exists {
		return fmt.Errorf("%w: %q", ErrBackgroundSubagentDuplicate, j.JobID)
	}
	r.jobs[j.JobID] = j
	return nil
}

// Start transitions a queued job to running.
func (r *BackgroundSubagentRegistry) Start(jobID uuid.UUID, startedAt time.Time) error {
	return r.transition(jobID, BackgroundSubagentStateRunning, startedAt, "", "")
}

// Succeed transitions a running job to succeeded with a result summary.
func (r *BackgroundSubagentRegistry) Succeed(jobID uuid.UUID, completedAt time.Time, resultSummary string) error {
	return r.transition(jobID, BackgroundSubagentStateSucceeded, completedAt, resultSummary, "")
}

// Fail transitions a running job to failed with an error message.
func (r *BackgroundSubagentRegistry) Fail(jobID uuid.UUID, completedAt time.Time, errorMsg string) error {
	return r.transition(jobID, BackgroundSubagentStateFailed, completedAt, "", errorMsg)
}

// Cancel transitions a queued or running job to cancelled.
func (r *BackgroundSubagentRegistry) Cancel(jobID uuid.UUID, completedAt time.Time) error {
	return r.transition(jobID, BackgroundSubagentStateCancelled, completedAt, "", "")
}

// TimeOut transitions a running job to timed_out.
func (r *BackgroundSubagentRegistry) TimeOut(jobID uuid.UUID, completedAt time.Time) error {
	return r.transition(jobID, BackgroundSubagentStateTimedOut, completedAt, "", "timed out")
}

func (r *BackgroundSubagentRegistry) transition(
	jobID uuid.UUID, to BackgroundSubagentState,
	at time.Time, resultSummary, errorMsg string,
) error {
	if at.IsZero() {
		return ErrBackgroundSubagentEmptyTimestamp
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[jobID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrBackgroundSubagentNotFound, jobID)
	}
	if !canTransitionBackgroundSubagent(j.State, to) {
		return fmt.Errorf("%w: %q → %q", ErrBackgroundSubagentBadTransition, j.State, to)
	}
	j.State = to
	if to == BackgroundSubagentStateRunning {
		j.StartedAt = at
	}
	if IsTerminalBackgroundSubagentState(to) {
		j.CompletedAt = at
	}
	if resultSummary != "" {
		j.ResultSummary = resultSummary
	}
	if errorMsg != "" {
		j.ErrorMsg = errorMsg
	}
	r.jobs[jobID] = j
	return nil
}

// Lookup returns a job by ID.
func (r *BackgroundSubagentRegistry) Lookup(jobID uuid.UUID) (BackgroundSubagentJob, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[jobID]
	return j, ok
}

// ListByParent returns jobs spawned by a given parent run, sorted by
// EnqueuedAt ascending.
func (r *BackgroundSubagentRegistry) ListByParent(parentRunID uuid.UUID) []BackgroundSubagentJob {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []BackgroundSubagentJob{}
	for _, j := range r.jobs {
		if j.ParentRunID == parentRunID {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnqueuedAt.Before(out[j].EnqueuedAt)
	})
	return out
}

// ListByState returns jobs in a given state, sorted by EnqueuedAt
// ascending.
func (r *BackgroundSubagentRegistry) ListByState(state BackgroundSubagentState) []BackgroundSubagentJob {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []BackgroundSubagentJob{}
	for _, j := range r.jobs {
		if j.State == state {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnqueuedAt.Before(out[j].EnqueuedAt)
	})
	return out
}

// Size returns the registry count.
func (r *BackgroundSubagentRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.jobs)
}

// PendingJobsForParent counts non-terminal jobs for a parent run —
// useful for "should the parent still wait?" checks.
func (r *BackgroundSubagentRegistry) PendingJobsForParent(parentRunID uuid.UUID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, j := range r.jobs {
		if j.ParentRunID == parentRunID && !IsTerminalBackgroundSubagentState(j.State) {
			count++
		}
	}
	return count
}

// Sentinel errors.
var (
	ErrBackgroundSubagentEmptyJobID       = errors.New("background subagent: job id required")
	ErrBackgroundSubagentEmptyParent      = errors.New("background subagent: parent run id required")
	ErrBackgroundSubagentBadOwner         = errors.New("background subagent: owner tenant slug must be kebab-case")
	ErrBackgroundSubagentBadSlug          = errors.New("background subagent: subagent slug must be kebab-case")
	ErrBackgroundSubagentEmptyEnqueuedAt  = errors.New("background subagent: enqueued_at required")
	ErrBackgroundSubagentEmptyTimestamp   = errors.New("background subagent: transition timestamp required")
	ErrBackgroundSubagentBadInitialState  = errors.New("background subagent: initial state must be queued")
	ErrBackgroundSubagentDuplicate        = errors.New("background subagent: duplicate job id")
	ErrBackgroundSubagentNotFound         = errors.New("background subagent: job not found")
	ErrBackgroundSubagentBadTransition    = errors.New("background subagent: invalid state transition")
)
