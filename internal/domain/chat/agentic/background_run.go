package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FUTURE-003 — Proactive/background agent loop.
//
// PDF arXiv:2604.14228v1 §12 (Future Directions — agents that initiate
// actions on their own, not just react to user prompts).
//
// Differs from the existing run-on-demand model (user msg → agent reply):
// background agents wake up via 4 trigger kinds:
//   - schedule_cron — recurring time-based (morning briefing, hourly check)
//   - event_arrived — external signal hits a subscription
//   - threshold_crossed — metric/budget/quality crosses a value
//   - absence_timeout — expected signal didn't arrive in window
//
// Distinct from neighbouring abstractions:
//   - taskmodel.go BGTask = transient backgrounded subprocess.
//   - GOV-003 Checkpoint = pause-for-approval mid-run.
//   - HUMAN-004 Understanding = pre-action verification.
//   - FUTURE-003 BackgroundAgentRun = AGENT-INITIATED top-level run.

// BackgroundTrigger bounded enum classifies what woke the run.
type BackgroundTrigger string

const (
	BackgroundTriggerScheduleCron     BackgroundTrigger = "schedule_cron"
	BackgroundTriggerEventArrived     BackgroundTrigger = "event_arrived"
	BackgroundTriggerThresholdCrossed BackgroundTrigger = "threshold_crossed"
	BackgroundTriggerAbsenceTimeout   BackgroundTrigger = "absence_timeout"
)

var allBackgroundTriggers = []BackgroundTrigger{
	BackgroundTriggerScheduleCron,
	BackgroundTriggerEventArrived,
	BackgroundTriggerThresholdCrossed,
	BackgroundTriggerAbsenceTimeout,
}

// IsValidBackgroundTrigger returns true for the bounded set.
func IsValidBackgroundTrigger(t BackgroundTrigger) bool {
	for _, v := range allBackgroundTriggers {
		if t == v {
			return true
		}
	}
	return false
}

// AllBackgroundTriggers returns a copy.
func AllBackgroundTriggers() []BackgroundTrigger {
	out := make([]BackgroundTrigger, len(allBackgroundTriggers))
	copy(out, allBackgroundTriggers)
	return out
}

// BackgroundRunStatus bounded enum tracks lifecycle.
type BackgroundRunStatus string

const (
	BackgroundRunScheduled BackgroundRunStatus = "scheduled"
	BackgroundRunRunning   BackgroundRunStatus = "running"
	BackgroundRunCompleted BackgroundRunStatus = "completed"
	BackgroundRunFailed    BackgroundRunStatus = "failed"
	BackgroundRunCancelled BackgroundRunStatus = "cancelled"
	BackgroundRunSkipped   BackgroundRunStatus = "skipped"
)

var allBackgroundRunStatuses = []BackgroundRunStatus{
	BackgroundRunScheduled, BackgroundRunRunning,
	BackgroundRunCompleted, BackgroundRunFailed,
	BackgroundRunCancelled, BackgroundRunSkipped,
}

// IsValidBackgroundRunStatus returns true for the bounded set.
func IsValidBackgroundRunStatus(s BackgroundRunStatus) bool {
	for _, v := range allBackgroundRunStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllBackgroundRunStatuses returns a copy.
func AllBackgroundRunStatuses() []BackgroundRunStatus {
	out := make([]BackgroundRunStatus, len(allBackgroundRunStatuses))
	copy(out, allBackgroundRunStatuses)
	return out
}

// IsTerminalBackgroundRunStatus returns true for completed/failed/cancelled/skipped.
func IsTerminalBackgroundRunStatus(s BackgroundRunStatus) bool {
	switch s {
	case BackgroundRunCompleted, BackgroundRunFailed,
		BackgroundRunCancelled, BackgroundRunSkipped:
		return true
	}
	return false
}

// BackgroundAgentRun is one scheduled-or-fired background execution.
type BackgroundAgentRun struct {
	ID             uuid.UUID           `json:"id"`
	TenantID       string              `json:"tenantId"`
	AgentID        string              `json:"agentId"`
	Trigger        BackgroundTrigger   `json:"trigger"`
	// TriggerPayload carries the source signal — cron expression for
	// schedule, event JSON for event_arrived, etc. Free-form (≤ 1000 chars).
	TriggerPayload string              `json:"triggerPayload,omitempty"`
	// Description is a one-line summary of what this run will do (≤ 300 chars).
	Description    string              `json:"description"`
	// ScheduledAt is when the run is supposed to fire.
	ScheduledAt    time.Time           `json:"scheduledAt"`
	// ExecutedAt is when the run actually started (zero if not yet run).
	ExecutedAt     time.Time           `json:"executedAt,omitempty"`
	// CompletedAt is when the run reached a terminal state.
	CompletedAt    time.Time           `json:"completedAt,omitempty"`
	Status         BackgroundRunStatus `json:"status"`
	// Result is a one-line outcome summary (≤ 500 chars).
	Result         string              `json:"result,omitempty"`
	// ErrorMessage carries failure details if Status=failed.
	ErrorMessage   string              `json:"errorMessage,omitempty"`
}

// Sentinels.
var (
	ErrBackgroundRunNotFound      = errors.New("background run: not found")
	ErrInvalidBackgroundTrigger   = errors.New("background run: invalid trigger")
	ErrInvalidBackgroundRunStatus = errors.New("background run: invalid status")
	ErrBackgroundRunAlreadyTerminal = errors.New("background run: already terminal")
)

func validateBackgroundRun(r BackgroundAgentRun) error {
	if r.TenantID == "" {
		return errors.New("background run: tenantId required")
	}
	if r.AgentID == "" {
		return errors.New("background run: agentId required")
	}
	if !IsValidBackgroundTrigger(r.Trigger) {
		return fmt.Errorf("%w: %q", ErrInvalidBackgroundTrigger, r.Trigger)
	}
	if r.Description == "" {
		return errors.New("background run: description required")
	}
	if r.ScheduledAt.IsZero() {
		return errors.New("background run: scheduledAt required")
	}
	return nil
}

// BackgroundRunStore is the persistence + scheduling interface.
type BackgroundRunStore interface {
	// Schedule persists a new run with status=scheduled.
	Schedule(ctx context.Context, r BackgroundAgentRun) (BackgroundAgentRun, error)
	// FindByID returns a run by ID.
	FindByID(ctx context.Context, id uuid.UUID) (BackgroundAgentRun, error)
	// FetchDue returns runs whose ScheduledAt ≤ now, status=scheduled,
	// limited per call. Used by the worker to claim work.
	FetchDue(ctx context.Context, tenantID string, now time.Time, limit int) ([]BackgroundAgentRun, error)
	// MarkRunning transitions scheduled → running. Idempotent guard.
	MarkRunning(ctx context.Context, id uuid.UUID) error
	// MarkCompleted transitions running → completed with result.
	MarkCompleted(ctx context.Context, id uuid.UUID, result string) error
	// MarkFailed transitions running → failed with error.
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
	// Cancel transitions any non-terminal status → cancelled.
	Cancel(ctx context.Context, id uuid.UUID) error
	// CountByStatus returns histogram for tenant.
	CountByStatus(ctx context.Context, tenantID string) (map[BackgroundRunStatus]int, error)
}

// --- InMemoryBackgroundRunStore ---

// InMemoryBackgroundRunStore is the default in-memory impl.
type InMemoryBackgroundRunStore struct {
	mu   sync.Mutex
	runs map[uuid.UUID]BackgroundAgentRun
}

// NewInMemoryBackgroundRunStore creates an empty store.
func NewInMemoryBackgroundRunStore() *InMemoryBackgroundRunStore {
	return &InMemoryBackgroundRunStore{
		runs: map[uuid.UUID]BackgroundAgentRun{},
	}
}

// Schedule persists a run with status=scheduled.
func (s *InMemoryBackgroundRunStore) Schedule(ctx context.Context, r BackgroundAgentRun) (BackgroundAgentRun, error) {
	if err := ctx.Err(); err != nil {
		return BackgroundAgentRun{}, err
	}
	if err := validateBackgroundRun(r); err != nil {
		return BackgroundAgentRun{}, err
	}
	if len(r.Description) > 300 {
		r.Description = r.Description[:297] + "..."
	}
	if len(r.TriggerPayload) > 1000 {
		r.TriggerPayload = r.TriggerPayload[:997] + "..."
	}
	r.ID = uuid.New()
	r.Status = BackgroundRunScheduled
	s.mu.Lock()
	s.runs[r.ID] = r
	s.mu.Unlock()
	return r, nil
}

// FindByID returns a run by ID.
func (s *InMemoryBackgroundRunStore) FindByID(ctx context.Context, id uuid.UUID) (BackgroundAgentRun, error) {
	if err := ctx.Err(); err != nil {
		return BackgroundAgentRun{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return BackgroundAgentRun{}, ErrBackgroundRunNotFound
	}
	return r, nil
}

// FetchDue returns scheduled runs whose ScheduledAt ≤ now.
// Sorted by ScheduledAt ASC (earliest first), capped at limit.
func (s *InMemoryBackgroundRunStore) FetchDue(ctx context.Context, tenantID string, now time.Time, limit int) ([]BackgroundAgentRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := []BackgroundAgentRun{}
	for _, r := range s.runs {
		if r.TenantID != tenantID {
			continue
		}
		if r.Status != BackgroundRunScheduled {
			continue
		}
		if r.ScheduledAt.After(now) {
			continue
		}
		matched = append(matched, r)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ScheduledAt.Before(matched[j].ScheduledAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// MarkRunning transitions scheduled → running.
func (s *InMemoryBackgroundRunStore) MarkRunning(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return ErrBackgroundRunNotFound
	}
	if r.Status != BackgroundRunScheduled {
		return fmt.Errorf("background run: cannot transition %q → running", r.Status)
	}
	r.Status = BackgroundRunRunning
	r.ExecutedAt = time.Now()
	s.runs[id] = r
	return nil
}

// MarkCompleted transitions running → completed.
func (s *InMemoryBackgroundRunStore) MarkCompleted(ctx context.Context, id uuid.UUID, result string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return ErrBackgroundRunNotFound
	}
	if IsTerminalBackgroundRunStatus(r.Status) {
		return ErrBackgroundRunAlreadyTerminal
	}
	if r.Status != BackgroundRunRunning {
		return fmt.Errorf("background run: cannot transition %q → completed (must be running)", r.Status)
	}
	if len(result) > 500 {
		result = result[:497] + "..."
	}
	r.Status = BackgroundRunCompleted
	r.Result = result
	r.CompletedAt = time.Now()
	s.runs[id] = r
	return nil
}

// MarkFailed transitions running → failed.
func (s *InMemoryBackgroundRunStore) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return ErrBackgroundRunNotFound
	}
	if IsTerminalBackgroundRunStatus(r.Status) {
		return ErrBackgroundRunAlreadyTerminal
	}
	if r.Status != BackgroundRunRunning {
		return fmt.Errorf("background run: cannot transition %q → failed (must be running)", r.Status)
	}
	if len(errMsg) > 500 {
		errMsg = errMsg[:497] + "..."
	}
	r.Status = BackgroundRunFailed
	r.ErrorMessage = errMsg
	r.CompletedAt = time.Now()
	s.runs[id] = r
	return nil
}

// Cancel transitions non-terminal → cancelled.
func (s *InMemoryBackgroundRunStore) Cancel(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return ErrBackgroundRunNotFound
	}
	if IsTerminalBackgroundRunStatus(r.Status) {
		return ErrBackgroundRunAlreadyTerminal
	}
	r.Status = BackgroundRunCancelled
	r.CompletedAt = time.Now()
	s.runs[id] = r
	return nil
}

// CountByStatus returns histogram (always 6 keys).
func (s *InMemoryBackgroundRunStore) CountByStatus(ctx context.Context, tenantID string) (map[BackgroundRunStatus]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[BackgroundRunStatus]int{}
	for _, st := range allBackgroundRunStatuses {
		hist[st] = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.runs {
		if r.TenantID == tenantID {
			hist[r.Status]++
		}
	}
	return hist, nil
}
