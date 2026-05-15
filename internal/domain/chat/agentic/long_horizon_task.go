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

// FUTURE-004 — Long-horizon task support.
//
// PDF arXiv:2604.14228v1 §12 (tasks that span days/weeks across many
// runs — not single-session work).
//
// Distinct from:
//   - taskmodel.go BGTask = transient backgrounded subprocess (minutes).
//   - FUTURE-003 BackgroundAgentRun = single scheduled execution.
//   - workflow_templates = single end-to-end run from one trigger.
//
// LongHorizonTask is the multi-week umbrella: composed of PHASES, each
// with goals + deliverables + deadlines. A phase progresses through
// MILESTONES. The whole task can be paused, resumed, or abandoned.
//
// Examples:
//   - "Monitor this customer for 30 days, daily check-ins"
//   - "Research this market over a week, drafts every 2 days"
//   - "Multi-week implementation plan with weekly milestones"

// LongHorizonStatus bounded enum.
type LongHorizonStatus string

const (
	LongHorizonPlanning   LongHorizonStatus = "planning"
	LongHorizonInProgress LongHorizonStatus = "in_progress"
	LongHorizonPaused     LongHorizonStatus = "paused"
	LongHorizonCompleted  LongHorizonStatus = "completed"
	LongHorizonAbandoned  LongHorizonStatus = "abandoned"
)

var allLongHorizonStatuses = []LongHorizonStatus{
	LongHorizonPlanning, LongHorizonInProgress, LongHorizonPaused,
	LongHorizonCompleted, LongHorizonAbandoned,
}

// IsValidLongHorizonStatus returns true for the bounded set.
func IsValidLongHorizonStatus(s LongHorizonStatus) bool {
	for _, v := range allLongHorizonStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllLongHorizonStatuses returns a copy.
func AllLongHorizonStatuses() []LongHorizonStatus {
	out := make([]LongHorizonStatus, len(allLongHorizonStatuses))
	copy(out, allLongHorizonStatuses)
	return out
}

// IsTerminalLongHorizonStatus returns true for completed/abandoned.
func IsTerminalLongHorizonStatus(s LongHorizonStatus) bool {
	return s == LongHorizonCompleted || s == LongHorizonAbandoned
}

// PhaseStatus bounded enum tracks per-phase lifecycle.
type PhaseStatus string

const (
	PhaseStatusPending   PhaseStatus = "pending"
	PhaseStatusActive    PhaseStatus = "active"
	PhaseStatusCompleted PhaseStatus = "completed"
	PhaseStatusSkipped   PhaseStatus = "skipped"
	PhaseStatusBlocked   PhaseStatus = "blocked"
)

var allPhaseStatuses = []PhaseStatus{
	PhaseStatusPending, PhaseStatusActive, PhaseStatusCompleted,
	PhaseStatusSkipped, PhaseStatusBlocked,
}

// IsValidPhaseStatus returns true for the bounded set.
func IsValidPhaseStatus(s PhaseStatus) bool {
	for _, v := range allPhaseStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllPhaseStatuses returns a copy.
func AllPhaseStatuses() []PhaseStatus {
	out := make([]PhaseStatus, len(allPhaseStatuses))
	copy(out, allPhaseStatuses)
	return out
}

// LongHorizonPhase is one stage of a multi-week task.
type LongHorizonPhase struct {
	ID           string      `json:"id"`           // phase identifier (e.g. "phase-1")
	Name         string      `json:"name"`         // human-readable name
	Goal         string      `json:"goal"`         // what this phase achieves (≤ 500 chars)
	Deliverables []string    `json:"deliverables"` // concrete outputs
	Deadline     time.Time   `json:"deadline,omitempty"`
	Status       PhaseStatus `json:"status"`
	StartedAt    time.Time   `json:"startedAt,omitempty"`
	CompletedAt  time.Time   `json:"completedAt,omitempty"`
	// DependsOn lists other phase IDs that must complete first.
	DependsOn []string `json:"dependsOn,omitempty"`
}

// ProgressMilestone is an incremental marker within a task.
type ProgressMilestone struct {
	ID          uuid.UUID `json:"id"`
	PhaseID     string    `json:"phaseId,omitempty"`
	Description string    `json:"description"`
	RecordedAt  time.Time `json:"recordedAt"`
}

// LongHorizonTask is the multi-week umbrella entity.
type LongHorizonTask struct {
	ID          uuid.UUID         `json:"id"`
	TenantID    string            `json:"tenantId"`
	AgentID     string            `json:"agentId"`
	Title       string            `json:"title"`       // ≤ 200 chars
	Description string            `json:"description"` // ≤ 1000 chars
	Status      LongHorizonStatus `json:"status"`
	Phases      []LongHorizonPhase `json:"phases"`
	Milestones  []ProgressMilestone `json:"milestones,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	StartedAt   time.Time         `json:"startedAt,omitempty"`
	CompletedAt time.Time         `json:"completedAt,omitempty"`
	PausedAt    time.Time         `json:"pausedAt,omitempty"`
}

// Sentinels.
var (
	ErrLongHorizonTaskNotFound      = errors.New("long-horizon task: not found")
	ErrInvalidLongHorizonStatus     = errors.New("long-horizon task: invalid status")
	ErrInvalidPhaseStatus           = errors.New("long-horizon task: invalid phase status")
	ErrLongHorizonAlreadyTerminal   = errors.New("long-horizon task: already terminal")
	ErrLongHorizonInvalidTransition = errors.New("long-horizon task: invalid status transition")
	ErrPhaseNotFound                = errors.New("long-horizon task: phase not found")
	ErrPhaseDependenciesNotMet      = errors.New("long-horizon task: phase dependencies not met")
)

func validateLongHorizonTask(t LongHorizonTask) error {
	if t.TenantID == "" {
		return errors.New("long-horizon task: tenantId required")
	}
	if t.AgentID == "" {
		return errors.New("long-horizon task: agentId required")
	}
	if t.Title == "" {
		return errors.New("long-horizon task: title required")
	}
	if len(t.Phases) == 0 {
		return errors.New("long-horizon task: at least one phase required")
	}
	// Validate phase IDs unique.
	phaseIDs := map[string]bool{}
	for _, p := range t.Phases {
		if p.ID == "" {
			return errors.New("long-horizon task: phase ID required")
		}
		if phaseIDs[p.ID] {
			return fmt.Errorf("long-horizon task: duplicate phase ID %q", p.ID)
		}
		phaseIDs[p.ID] = true
	}
	// Validate dependsOn references exist.
	for _, p := range t.Phases {
		for _, dep := range p.DependsOn {
			if !phaseIDs[dep] {
				return fmt.Errorf("long-horizon task: phase %q depends on unknown phase %q", p.ID, dep)
			}
		}
	}
	return nil
}

// LongHorizonTaskStore is the persistence interface.
type LongHorizonTaskStore interface {
	Create(ctx context.Context, t LongHorizonTask) (LongHorizonTask, error)
	FindByID(ctx context.Context, id uuid.UUID) (LongHorizonTask, error)
	ListByAgent(ctx context.Context, tenantID, agentID string, statusFilter LongHorizonStatus) ([]LongHorizonTask, error)
	StartTask(ctx context.Context, id uuid.UUID) error
	StartPhase(ctx context.Context, id uuid.UUID, phaseID string) error
	CompletePhase(ctx context.Context, id uuid.UUID, phaseID string) error
	RecordMilestone(ctx context.Context, id uuid.UUID, m ProgressMilestone) error
	Pause(ctx context.Context, id uuid.UUID) error
	Resume(ctx context.Context, id uuid.UUID) error
	Complete(ctx context.Context, id uuid.UUID) error
	Abandon(ctx context.Context, id uuid.UUID) error
	CountByStatus(ctx context.Context, tenantID string) (map[LongHorizonStatus]int, error)
}

// --- InMemoryLongHorizonTaskStore ---

type InMemoryLongHorizonTaskStore struct {
	mu    sync.Mutex
	tasks map[uuid.UUID]LongHorizonTask
}

func NewInMemoryLongHorizonTaskStore() *InMemoryLongHorizonTaskStore {
	return &InMemoryLongHorizonTaskStore{tasks: map[uuid.UUID]LongHorizonTask{}}
}

// Create persists a new task with status=planning. Phase IDs validated.
func (s *InMemoryLongHorizonTaskStore) Create(ctx context.Context, t LongHorizonTask) (LongHorizonTask, error) {
	if err := ctx.Err(); err != nil {
		return LongHorizonTask{}, err
	}
	if err := validateLongHorizonTask(t); err != nil {
		return LongHorizonTask{}, err
	}
	if len(t.Title) > 200 {
		t.Title = t.Title[:197] + "..."
	}
	if len(t.Description) > 1000 {
		t.Description = t.Description[:997] + "..."
	}
	t.ID = uuid.New()
	t.Status = LongHorizonPlanning
	t.CreatedAt = time.Now()
	// Default all phases to pending.
	for i := range t.Phases {
		if t.Phases[i].Status == "" {
			t.Phases[i].Status = PhaseStatusPending
		} else if !IsValidPhaseStatus(t.Phases[i].Status) {
			return LongHorizonTask{}, fmt.Errorf("%w: %q", ErrInvalidPhaseStatus, t.Phases[i].Status)
		}
		if len(t.Phases[i].Goal) > 500 {
			t.Phases[i].Goal = t.Phases[i].Goal[:497] + "..."
		}
	}
	s.mu.Lock()
	s.tasks[t.ID] = t
	s.mu.Unlock()
	return t, nil
}

// FindByID returns a task by ID.
func (s *InMemoryLongHorizonTaskStore) FindByID(ctx context.Context, id uuid.UUID) (LongHorizonTask, error) {
	if err := ctx.Err(); err != nil {
		return LongHorizonTask{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return LongHorizonTask{}, ErrLongHorizonTaskNotFound
	}
	return t, nil
}

// ListByAgent returns tasks for an agent. Empty status filter = all statuses.
func (s *InMemoryLongHorizonTaskStore) ListByAgent(ctx context.Context, tenantID, agentID string, statusFilter LongHorizonStatus) ([]LongHorizonTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := []LongHorizonTask{}
	for _, t := range s.tasks {
		if t.TenantID != tenantID || t.AgentID != agentID {
			continue
		}
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		matched = append(matched, t)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	return matched, nil
}

// StartTask transitions planning → in_progress.
func (s *InMemoryLongHorizonTaskStore) StartTask(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if t.Status != LongHorizonPlanning {
		return fmt.Errorf("%w: cannot start from %q", ErrLongHorizonInvalidTransition, t.Status)
	}
	t.Status = LongHorizonInProgress
	t.StartedAt = time.Now()
	s.tasks[id] = t
	return nil
}

// StartPhase transitions a phase to active. Validates dependencies.
func (s *InMemoryLongHorizonTaskStore) StartPhase(ctx context.Context, id uuid.UUID, phaseID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if t.Status != LongHorizonInProgress {
		return fmt.Errorf("%w: task must be in_progress to start phase", ErrLongHorizonInvalidTransition)
	}
	phaseIdx := -1
	for i, p := range t.Phases {
		if p.ID == phaseID {
			phaseIdx = i
			break
		}
	}
	if phaseIdx < 0 {
		return ErrPhaseNotFound
	}
	if t.Phases[phaseIdx].Status != PhaseStatusPending {
		return fmt.Errorf("%w: phase %q must be pending to start (is %q)",
			ErrLongHorizonInvalidTransition, phaseID, t.Phases[phaseIdx].Status)
	}
	// Validate dependencies completed.
	for _, depID := range t.Phases[phaseIdx].DependsOn {
		for _, p := range t.Phases {
			if p.ID == depID && p.Status != PhaseStatusCompleted && p.Status != PhaseStatusSkipped {
				return fmt.Errorf("%w: phase %q depends on %q (status: %q)",
					ErrPhaseDependenciesNotMet, phaseID, depID, p.Status)
			}
		}
	}
	t.Phases[phaseIdx].Status = PhaseStatusActive
	t.Phases[phaseIdx].StartedAt = time.Now()
	s.tasks[id] = t
	return nil
}

// CompletePhase transitions a phase to completed.
func (s *InMemoryLongHorizonTaskStore) CompletePhase(ctx context.Context, id uuid.UUID, phaseID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	for i, p := range t.Phases {
		if p.ID != phaseID {
			continue
		}
		if p.Status != PhaseStatusActive {
			return fmt.Errorf("%w: phase %q must be active to complete (is %q)",
				ErrLongHorizonInvalidTransition, phaseID, p.Status)
		}
		t.Phases[i].Status = PhaseStatusCompleted
		t.Phases[i].CompletedAt = time.Now()
		s.tasks[id] = t
		return nil
	}
	return ErrPhaseNotFound
}

// RecordMilestone appends a milestone. Always allowed in non-terminal states.
func (s *InMemoryLongHorizonTaskStore) RecordMilestone(ctx context.Context, id uuid.UUID, m ProgressMilestone) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if IsTerminalLongHorizonStatus(t.Status) {
		return ErrLongHorizonAlreadyTerminal
	}
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.RecordedAt.IsZero() {
		m.RecordedAt = time.Now()
	}
	t.Milestones = append(t.Milestones, m)
	s.tasks[id] = t
	return nil
}

// Pause transitions in_progress → paused.
func (s *InMemoryLongHorizonTaskStore) Pause(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if t.Status != LongHorizonInProgress {
		return fmt.Errorf("%w: cannot pause from %q", ErrLongHorizonInvalidTransition, t.Status)
	}
	t.Status = LongHorizonPaused
	t.PausedAt = time.Now()
	s.tasks[id] = t
	return nil
}

// Resume transitions paused → in_progress.
func (s *InMemoryLongHorizonTaskStore) Resume(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if t.Status != LongHorizonPaused {
		return fmt.Errorf("%w: cannot resume from %q", ErrLongHorizonInvalidTransition, t.Status)
	}
	t.Status = LongHorizonInProgress
	t.PausedAt = time.Time{}
	s.tasks[id] = t
	return nil
}

// Complete transitions in_progress → completed.
func (s *InMemoryLongHorizonTaskStore) Complete(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if IsTerminalLongHorizonStatus(t.Status) {
		return ErrLongHorizonAlreadyTerminal
	}
	if t.Status != LongHorizonInProgress {
		return fmt.Errorf("%w: cannot complete from %q (must be in_progress)",
			ErrLongHorizonInvalidTransition, t.Status)
	}
	// Verify all phases terminal (completed or skipped).
	for _, p := range t.Phases {
		if p.Status != PhaseStatusCompleted && p.Status != PhaseStatusSkipped {
			return fmt.Errorf("%w: phase %q is %q (must be completed or skipped)",
				ErrLongHorizonInvalidTransition, p.ID, p.Status)
		}
	}
	t.Status = LongHorizonCompleted
	t.CompletedAt = time.Now()
	s.tasks[id] = t
	return nil
}

// Abandon transitions any non-terminal → abandoned.
func (s *InMemoryLongHorizonTaskStore) Abandon(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return ErrLongHorizonTaskNotFound
	}
	if IsTerminalLongHorizonStatus(t.Status) {
		return ErrLongHorizonAlreadyTerminal
	}
	t.Status = LongHorizonAbandoned
	t.CompletedAt = time.Now()
	s.tasks[id] = t
	return nil
}

// CountByStatus returns histogram (always 5 keys).
func (s *InMemoryLongHorizonTaskStore) CountByStatus(ctx context.Context, tenantID string) (map[LongHorizonStatus]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[LongHorizonStatus]int{}
	for _, st := range allLongHorizonStatuses {
		hist[st] = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tasks {
		if t.TenantID == tenantID {
			hist[t.Status]++
		}
	}
	return hist, nil
}
