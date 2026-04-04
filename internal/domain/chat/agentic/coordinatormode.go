package agentic

import (
	"fmt"
	"strings"
	"sync"
)

// Coordinator mode detection and orchestration.
//
// Inspired by Claude Code's coordinatorMode.ts — detects and manages
// coordinator vs worker execution modes. The coordinator delegates tasks
// to workers, controls tool access, and defines execution phases.

// AgentMode identifies how an agent is executing.
type AgentMode string

const (
	// ModeNormal is standard single-agent execution.
	ModeNormal AgentMode = "normal"
	// ModeCoordinator orchestrates multiple workers.
	ModeCoordinator AgentMode = "coordinator"
	// ModeWorker executes tasks delegated by a coordinator.
	ModeWorker AgentMode = "worker"
)

// WorkflowPhase represents a coordinator workflow phase.
// (TaskPhase and Phase* constants are defined in coordinator.go.)
type WorkflowPhase = TaskPhase

// ParallelismStrategy controls how tasks can run concurrently.
type ParallelismStrategy string

const (
	// ParallelFree allows full parallelism (e.g., read-only tasks).
	ParallelFree ParallelismStrategy = "free"
	// ParallelSerialize forces sequential execution (e.g., write tasks).
	ParallelSerialize ParallelismStrategy = "serialize"
)

// WorkerConfig describes a worker's capabilities and constraints.
type WorkerConfig struct {
	// Name identifies this worker.
	Name string `json:"name"`
	// AllowedTools lists tools the worker can use.
	AllowedTools []string `json:"allowedTools,omitempty"`
	// DeniedTools lists tools the worker cannot use.
	DeniedTools []string `json:"deniedTools,omitempty"`
	// Parallelism controls the worker's concurrency behavior.
	Parallelism ParallelismStrategy `json:"parallelism"`
	// MaxTurns limits the worker's execution turns.
	MaxTurns int `json:"maxTurns,omitempty"`
}

// CoordinatorModeConfig configures coordinator mode behavior.
type CoordinatorModeConfig struct {
	// Workers defines available worker configurations.
	Workers []WorkerConfig `json:"workers,omitempty"`
	// InternalTools are tools excluded from worker access.
	InternalTools []string `json:"internalTools,omitempty"`
	// MaxWorkers is the maximum number of concurrent workers.
	MaxWorkers int `json:"maxWorkers,omitempty"`
}

// CoordinatorModeState tracks the current mode and metadata.
type CoordinatorModeState struct {
	mu           sync.RWMutex
	mode         AgentMode
	phase        WorkflowPhase
	config       CoordinatorModeConfig
	workerCount  int
	sessionID    string
}

// NewCoordinatorModeState creates a coordinator mode tracker.
func NewCoordinatorModeState(mode AgentMode, config CoordinatorModeConfig) *CoordinatorModeState {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 10
	}
	return &CoordinatorModeState{
		mode:   mode,
		phase:  PhaseResearch,
		config: config,
	}
}

// Mode returns the current agent mode.
func (s *CoordinatorModeState) Mode() AgentMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// SetMode changes the execution mode.
func (s *CoordinatorModeState) SetMode(mode AgentMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
}

// IsCoordinator returns true if in coordinator mode.
func (s *CoordinatorModeState) IsCoordinator() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode == ModeCoordinator
}

// IsWorker returns true if in worker mode.
func (s *CoordinatorModeState) IsWorker() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode == ModeWorker
}

// Phase returns the current execution phase.
func (s *CoordinatorModeState) Phase() WorkflowPhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.phase
}

// SetPhase changes the execution phase.
func (s *CoordinatorModeState) SetPhase(phase WorkflowPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

// WorkerCount returns the number of active workers.
func (s *CoordinatorModeState) WorkerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workerCount
}

// IncrementWorkers increments the active worker count.
// Returns false if max workers would be exceeded.
func (s *CoordinatorModeState) IncrementWorkers() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workerCount >= s.config.MaxWorkers {
		return false
	}
	s.workerCount++
	return true
}

// DecrementWorkers decrements the active worker count.
func (s *CoordinatorModeState) DecrementWorkers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workerCount > 0 {
		s.workerCount--
	}
}

// GetWorkerTools returns the allowed tools for a named worker,
// filtering out internal tools.
func (s *CoordinatorModeState) GetWorkerTools(workerName string, allTools []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var worker *WorkerConfig
	for i := range s.config.Workers {
		if s.config.Workers[i].Name == workerName {
			worker = &s.config.Workers[i]
			break
		}
	}

	internal := make(map[string]bool)
	for _, t := range s.config.InternalTools {
		internal[t] = true
	}

	denied := make(map[string]bool)
	if worker != nil {
		for _, t := range worker.DeniedTools {
			denied[t] = true
		}
	}

	// If worker has explicit allowed list, use it (minus internal).
	if worker != nil && len(worker.AllowedTools) > 0 {
		var result []string
		for _, t := range worker.AllowedTools {
			if !internal[t] && !denied[t] {
				result = append(result, t)
			}
		}
		return result
	}

	// Otherwise, use all tools minus internal and denied.
	var result []string
	for _, t := range allTools {
		if !internal[t] && !denied[t] {
			result = append(result, t)
		}
	}
	return result
}

// BuildCoordinatorModePrompt generates the system prompt for coordinator mode.
func BuildCoordinatorModePrompt(config CoordinatorModeConfig) string {
	var b strings.Builder

	b.WriteString("You are operating in coordinator mode. You orchestrate work across multiple workers.\n\n")

	b.WriteString("## Execution Phases\n")
	b.WriteString("1. **Research** — Gather information, read files, understand codebase\n")
	b.WriteString("2. **Synthesis** — Analyze findings and create implementation plan\n")
	b.WriteString("3. **Implementation** — Execute the plan using workers\n")
	b.WriteString("4. **Verification** — Validate results and ensure correctness\n\n")

	b.WriteString("## Parallelism Rules\n")
	b.WriteString("- Read-only tasks can run in parallel freely\n")
	b.WriteString("- Write tasks must be serialized to prevent conflicts\n")
	b.WriteString("- Verification can run in parallel after implementation\n\n")

	b.WriteString("## Worker Instructions\n")
	b.WriteString("- Pass synthesized specs to workers, not lazy references\n")
	b.WriteString("- Each worker should have a clear, self-contained task\n")
	b.WriteString("- Include all context needed for the worker to succeed\n\n")

	if len(config.Workers) > 0 {
		b.WriteString("## Available Workers\n")
		for _, w := range config.Workers {
			b.WriteString(fmt.Sprintf("- **%s** (parallelism: %s", w.Name, w.Parallelism))
			if w.MaxTurns > 0 {
				b.WriteString(fmt.Sprintf(", max turns: %d", w.MaxTurns))
			}
			b.WriteString(")\n")
		}
	}

	return b.String()
}
