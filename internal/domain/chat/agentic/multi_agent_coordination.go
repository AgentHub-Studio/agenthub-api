package agentic

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// SUB-011 — Multi-agent coordination.
//
// PDF arXiv:2604.14228v1 §8 (Subagents) — when a parent agent spawns
// MULTIPLE subagents, it needs a strategy for how they coordinate:
// sequentially (one after the next), in parallel (fan-out), through a
// pipeline (each consumes the previous output), or via an arbitrary
// DAG with dependency tracking.
//
// Distinct from existing AgentHub plumbing:
//   - SUB-001 (Agent tool) is the SPAWN primitive — one subagent at a time.
//   - SUB-006 PermissionInheritanceMode + SUB-005 SubagentToolsetPolicy
//     control individual subagent SCOPE; SUB-011 orchestrates GROUPS.
//   - SUB-010 SubagentReturnSummary is the result of one subagent;
//     SUB-011 collects + dispatches based on those results.
//   - SUB-009 sidechain transcripts persists individual subagent
//     transcripts; SUB-011 tracks the SCHEDULER state across them.

// MultiAgentStrategy bounded enum declares how the coordinator
// schedules subagents.
type MultiAgentStrategy string

const (
	// MultiAgentSequential — run tasks one at a time in declared order.
	// Each task starts only after the previous one finishes.
	MultiAgentSequential MultiAgentStrategy = "sequential"
	// MultiAgentParallel — fan out: all eligible tasks run concurrently.
	// Coordinator returns the full set when asked for next batch.
	MultiAgentParallel MultiAgentStrategy = "parallel"
	// MultiAgentPipeline — each task feeds the next (ordered chain).
	// Like sequential but with explicit data hand-off contract.
	MultiAgentPipeline MultiAgentStrategy = "pipeline"
	// MultiAgentDAG — arbitrary directed acyclic graph of dependencies.
	// Task starts when all its declared deps have completed successfully.
	MultiAgentDAG MultiAgentStrategy = "dag"
)

var allMultiAgentStrategies = []MultiAgentStrategy{
	MultiAgentSequential, MultiAgentParallel,
	MultiAgentPipeline, MultiAgentDAG,
}

// IsValidMultiAgentStrategy returns true for the bounded set.
func IsValidMultiAgentStrategy(s MultiAgentStrategy) bool {
	for _, v := range allMultiAgentStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// MultiAgentTaskStatus bounded enum tracks per-task state in the
// coordinator.
type MultiAgentTaskStatus string

const (
	MultiAgentTaskPending       MultiAgentTaskStatus = "pending"
	MultiAgentTaskBlockedOnDeps MultiAgentTaskStatus = "blocked_on_deps"
	MultiAgentTaskRunning       MultiAgentTaskStatus = "running"
	MultiAgentTaskCompleted     MultiAgentTaskStatus = "completed"
	MultiAgentTaskFailed        MultiAgentTaskStatus = "failed"
	MultiAgentTaskSkipped       MultiAgentTaskStatus = "skipped"
)

var allMultiAgentTaskStatuses = []MultiAgentTaskStatus{
	MultiAgentTaskPending, MultiAgentTaskBlockedOnDeps,
	MultiAgentTaskRunning, MultiAgentTaskCompleted,
	MultiAgentTaskFailed, MultiAgentTaskSkipped,
}

// IsValidMultiAgentTaskStatus returns true for the bounded set.
func IsValidMultiAgentTaskStatus(s MultiAgentTaskStatus) bool {
	for _, v := range allMultiAgentTaskStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// MultiAgentFailurePolicy bounded enum declares how the coordinator
// reacts to a failed task.
type MultiAgentFailurePolicy string

const (
	// MultiAgentAbortOnFailure — first failure stops the whole plan.
	MultiAgentAbortOnFailure MultiAgentFailurePolicy = "abort_on_failure"
	// MultiAgentContinueOnFailure — log the failure, keep going. The
	// final report flags the failed tasks but other tasks still execute.
	MultiAgentContinueOnFailure MultiAgentFailurePolicy = "continue_on_failure"
	// MultiAgentSkipDownstreamOnFailure — DAG-aware: failed task's
	// downstream dependents are marked skipped (not eligible to run).
	// Other independent branches continue.
	MultiAgentSkipDownstreamOnFailure MultiAgentFailurePolicy = "skip_downstream_on_failure"
)

var allMultiAgentFailurePolicies = []MultiAgentFailurePolicy{
	MultiAgentAbortOnFailure, MultiAgentContinueOnFailure,
	MultiAgentSkipDownstreamOnFailure,
}

// IsValidMultiAgentFailurePolicy returns true for the bounded set.
func IsValidMultiAgentFailurePolicy(p MultiAgentFailurePolicy) bool {
	for _, v := range allMultiAgentFailurePolicies {
		if p == v {
			return true
		}
	}
	return false
}

// CoordinatedTask is one unit of work in a multi-agent plan.
type CoordinatedTask struct {
	TaskID           string
	Description      string
	DependsOn        []string // task IDs that must complete before this one
	Status           MultiAgentTaskStatus
	AssignedSubagent string // populated after the coordinator schedules it
	CreatedAt        time.Time
	StartedAt        time.Time
	FinishedAt       time.Time
}

// Validate enforces invariants.
func (t CoordinatedTask) Validate() error {
	if strings.TrimSpace(t.TaskID) == "" {
		return ErrMultiAgentEmptyTaskID
	}
	if strings.TrimSpace(t.Description) == "" {
		return ErrMultiAgentEmptyTaskDescription
	}
	if t.Status != "" && !IsValidMultiAgentTaskStatus(t.Status) {
		return fmt.Errorf("%w: %q", ErrMultiAgentBadTaskStatus, t.Status)
	}
	for _, dep := range t.DependsOn {
		if strings.TrimSpace(dep) == "" {
			return ErrMultiAgentEmptyTaskID
		}
		if dep == t.TaskID {
			return fmt.Errorf("%w: %q depends on itself", ErrMultiAgentSelfDependency, t.TaskID)
		}
	}
	return nil
}

// CoordinationPlan is the inputs to the coordinator.
type CoordinationPlan struct {
	Strategy       MultiAgentStrategy
	FailurePolicy  MultiAgentFailurePolicy
	Tasks          []CoordinatedTask
	MaxParallelism int // 0 = unbounded
}

// Validate checks plan-level invariants (strategy + DAG validity).
func (p CoordinationPlan) Validate() error {
	if !IsValidMultiAgentStrategy(p.Strategy) {
		return fmt.Errorf("%w: %q", ErrMultiAgentBadStrategy, p.Strategy)
	}
	if p.FailurePolicy != "" && !IsValidMultiAgentFailurePolicy(p.FailurePolicy) {
		return fmt.Errorf("%w: %q", ErrMultiAgentBadFailurePolicy, p.FailurePolicy)
	}
	if p.MaxParallelism < 0 {
		return ErrMultiAgentBadParallelism
	}
	if len(p.Tasks) == 0 {
		return ErrMultiAgentEmptyPlan
	}
	ids := map[string]bool{}
	for _, t := range p.Tasks {
		if err := t.Validate(); err != nil {
			return err
		}
		if ids[t.TaskID] {
			return fmt.Errorf("%w: %q", ErrMultiAgentDuplicateTaskID, t.TaskID)
		}
		ids[t.TaskID] = true
	}
	for _, t := range p.Tasks {
		for _, dep := range t.DependsOn {
			if !ids[dep] {
				return fmt.Errorf("%w: %q depends on unknown task %q",
					ErrMultiAgentUnknownDependency, t.TaskID, dep)
			}
		}
	}
	if p.Strategy == MultiAgentDAG {
		if err := detectCycle(p.Tasks); err != nil {
			return err
		}
	}
	return nil
}

// detectCycle returns an error if the dependency graph contains a cycle.
func detectCycle(tasks []CoordinatedTask) error {
	const (
		white = 0 // unvisited
		gray  = 1 // in stack
		black = 2 // fully processed
	)
	color := map[string]int{}
	deps := map[string][]string{}
	for _, t := range tasks {
		color[t.TaskID] = white
		deps[t.TaskID] = t.DependsOn
	}
	var visit func(string) error
	visit = func(id string) error {
		if color[id] == gray {
			return fmt.Errorf("%w: at %q", ErrMultiAgentCycleDetected, id)
		}
		if color[id] == black {
			return nil
		}
		color[id] = gray
		for _, d := range deps[id] {
			if err := visit(d); err != nil {
				return err
			}
		}
		color[id] = black
		return nil
	}
	for _, t := range tasks {
		if err := visit(t.TaskID); err != nil {
			return err
		}
	}
	return nil
}

// CoordinationState is the mutable per-evaluation snapshot the
// scheduler uses. Callers update Tasks' Status as subagents complete;
// scheduler reads to decide what to spawn next.
type CoordinationState struct {
	Tasks []CoordinatedTask
}

// FindTask returns the task with the given ID, or false.
func (s CoordinationState) FindTask(taskID string) (CoordinatedTask, bool) {
	for _, t := range s.Tasks {
		if t.TaskID == taskID {
			return t, true
		}
	}
	return CoordinatedTask{}, false
}

// MultiAgentCoordinator computes which tasks are eligible to spawn
// given current state + strategy.
type MultiAgentCoordinator struct {
	mu   sync.RWMutex
	plan CoordinationPlan
	now  func() time.Time
}

// NewMultiAgentCoordinator builds a coordinator. Validates the plan
// eagerly.
func NewMultiAgentCoordinator(plan CoordinationPlan) (*MultiAgentCoordinator, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return &MultiAgentCoordinator{plan: plan, now: time.Now}, nil
}

// SetClock injects a clock for deterministic test timestamps. nil is
// a defensive no-op.
func (c *MultiAgentCoordinator) SetClock(fn func() time.Time) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = fn
}

// Plan returns a defensive copy of the plan.
func (c *MultiAgentCoordinator) Plan() CoordinationPlan {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tasks := append([]CoordinatedTask(nil), c.plan.Tasks...)
	return CoordinationPlan{
		Strategy:       c.plan.Strategy,
		FailurePolicy:  c.plan.FailurePolicy,
		Tasks:          tasks,
		MaxParallelism: c.plan.MaxParallelism,
	}
}

// NextBatch returns the set of task IDs that should spawn now. Returns
// an empty slice when all tasks have terminal status. Capped at
// MaxParallelism when > 0. Pure function: never mutates state.
func (c *MultiAgentCoordinator) NextBatch(state CoordinationState) ([]string, error) {
	c.mu.RLock()
	plan := c.plan
	c.mu.RUnlock()

	// abort_on_failure: if any task is failed, return empty (no more spawns).
	if plan.FailurePolicy == MultiAgentAbortOnFailure {
		for _, t := range state.Tasks {
			if t.Status == MultiAgentTaskFailed {
				return nil, nil
			}
		}
	}

	// Count tasks currently running for parallelism cap.
	runningCount := 0
	for _, t := range state.Tasks {
		if t.Status == MultiAgentTaskRunning {
			runningCount++
		}
	}

	// Build status map for dep lookup.
	statusOf := map[string]MultiAgentTaskStatus{}
	for _, t := range state.Tasks {
		statusOf[t.TaskID] = t.Status
	}

	eligible := []string{}
	for _, t := range state.Tasks {
		if t.Status != MultiAgentTaskPending && t.Status != MultiAgentTaskBlockedOnDeps {
			continue
		}
		// Check deps.
		ready := true
		for _, dep := range t.DependsOn {
			s := statusOf[dep]
			if s != MultiAgentTaskCompleted {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		eligible = append(eligible, t.TaskID)
	}

	// Apply strategy filter.
	switch plan.Strategy {
	case MultiAgentSequential, MultiAgentPipeline:
		// Only one at a time.
		if runningCount > 0 {
			return nil, nil
		}
		if len(eligible) > 0 {
			return []string{eligible[0]}, nil
		}
		return nil, nil
	case MultiAgentParallel, MultiAgentDAG:
		// Apply MaxParallelism cap.
		capacity := len(eligible)
		if plan.MaxParallelism > 0 {
			budget := plan.MaxParallelism - runningCount
			if budget < 0 {
				budget = 0
			}
			if budget < capacity {
				capacity = budget
			}
		}
		// Deterministic order: by task id.
		sort.Strings(eligible)
		return eligible[:capacity], nil
	}
	return nil, nil
}

// IsTerminalState returns true when every task is in a terminal status
// (completed, failed, skipped) — no more progress possible.
func (c *MultiAgentCoordinator) IsTerminalState(state CoordinationState) bool {
	for _, t := range state.Tasks {
		if t.Status != MultiAgentTaskCompleted &&
			t.Status != MultiAgentTaskFailed &&
			t.Status != MultiAgentTaskSkipped {
			return false
		}
	}
	return true
}

// SummariseState returns counts per status for audit / dashboard.
func (c *MultiAgentCoordinator) SummariseState(state CoordinationState) map[MultiAgentTaskStatus]int {
	out := map[MultiAgentTaskStatus]int{}
	for _, t := range state.Tasks {
		out[t.Status]++
	}
	return out
}

// Sentinel errors.
var (
	ErrMultiAgentBadStrategy          = errors.New("multi-agent: invalid strategy")
	ErrMultiAgentBadFailurePolicy     = errors.New("multi-agent: invalid failure policy")
	ErrMultiAgentBadTaskStatus        = errors.New("multi-agent: invalid task status")
	ErrMultiAgentBadParallelism       = errors.New("multi-agent: max parallelism must be >= 0")
	ErrMultiAgentEmptyPlan            = errors.New("multi-agent: plan must have at least one task")
	ErrMultiAgentEmptyTaskID          = errors.New("multi-agent: task id required")
	ErrMultiAgentEmptyTaskDescription = errors.New("multi-agent: task description required")
	ErrMultiAgentDuplicateTaskID      = errors.New("multi-agent: duplicate task id in plan")
	ErrMultiAgentSelfDependency       = errors.New("multi-agent: task cannot depend on itself")
	ErrMultiAgentUnknownDependency    = errors.New("multi-agent: dependency references unknown task")
	ErrMultiAgentCycleDetected        = errors.New("multi-agent: dependency cycle detected")
)
