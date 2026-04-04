package agentic

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WorkerIdentity identifies a worker agent in a coordinator swarm.
// Inspired by Claude Code's teammate identity in utils/teammate.ts.
type WorkerIdentity struct {
	// ID is a unique identifier for this worker instance.
	ID string `json:"id"`
	// Name is a human-readable label (e.g. "researcher-1", "implementer").
	Name string `json:"name"`
	// Team is the team this worker belongs to (for multi-team orchestration).
	Team string `json:"team,omitempty"`
	// Role describes the worker's specialization.
	Role string `json:"role,omitempty"`
}

// TaskNotification is sent by workers to the coordinator to report progress.
// The coordinator uses these to decide follow-up work.
//
// Inspired by Claude Code's <task-notification> XML protocol in coordinatorMode.ts.
type TaskNotification struct {
	// WorkerID identifies who sent this notification.
	WorkerID string `json:"workerId"`
	// WorkerName is the human-readable worker label.
	WorkerName string `json:"workerName"`
	// TaskID is the sub-task being reported on.
	TaskID string `json:"taskId"`
	// Status is the task status (in_progress, completed, failed).
	Status TaskStatus `json:"status"`
	// Summary is a brief description of what was accomplished or went wrong.
	Summary string `json:"summary"`
	// Findings are structured results from research tasks.
	Findings json.RawMessage `json:"findings,omitempty"`
	// Error is set when status is "failed".
	Error *string `json:"error,omitempty"`
	// Timestamp is when the notification was created.
	Timestamp time.Time `json:"timestamp"`
}

// TaskStatus represents the lifecycle state of a coordinator task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// CoordinatorTask represents a unit of work assigned by the coordinator.
type CoordinatorTask struct {
	// ID is a unique identifier.
	ID string `json:"id"`
	// Description is the task objective.
	Description string `json:"description"`
	// AssignedTo is the worker ID handling this task.
	AssignedTo string `json:"assignedTo,omitempty"`
	// Status tracks the task lifecycle.
	Status TaskStatus `json:"status"`
	// Phase categorizes the task (research, implementation, verification).
	Phase TaskPhase `json:"phase"`
	// DependsOn lists task IDs that must complete before this one can start.
	DependsOn []string `json:"dependsOn,omitempty"`
	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"createdAt"`
	// CompletedAt is when the task finished (if completed).
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// TaskPhase categorizes coordinator tasks into workflow phases.
// Inspired by Claude Code's coordinator task workflow table.
type TaskPhase string

const (
	// PhaseResearch covers investigation, codebase exploration, finding files.
	PhaseResearch TaskPhase = "research"
	// PhaseSynthesis is the coordinator's own work: reading findings, crafting specs.
	PhaseSynthesis TaskPhase = "synthesis"
	// PhaseImplementation covers making targeted changes per spec.
	PhaseImplementation TaskPhase = "implementation"
	// PhaseVerification covers testing that changes work correctly.
	PhaseVerification TaskPhase = "verification"
)

// CoordinatorState manages the distributed state of a coordinator swarm.
// Thread-safe for concurrent worker access.
type CoordinatorState struct {
	mu            sync.RWMutex
	tasks         map[string]*CoordinatorTask
	workers       map[string]*WorkerIdentity
	notifications []TaskNotification
}

// NewCoordinatorState creates an empty coordinator state.
func NewCoordinatorState() *CoordinatorState {
	return &CoordinatorState{
		tasks:   make(map[string]*CoordinatorTask),
		workers: make(map[string]*WorkerIdentity),
	}
}

// RegisterWorker adds a worker to the swarm.
func (s *CoordinatorState) RegisterWorker(worker WorkerIdentity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[worker.ID] = &worker
}

// CreateTask creates a new coordinator task and returns its ID.
func (s *CoordinatorState) CreateTask(description string, phase TaskPhase, dependsOn []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	s.tasks[id] = &CoordinatorTask{
		ID:          id,
		Description: description,
		Status:      TaskStatusPending,
		Phase:       phase,
		DependsOn:   dependsOn,
		CreatedAt:   time.Now(),
	}
	return id
}

// AssignTask assigns a task to a worker.
func (s *CoordinatorState) AssignTask(taskID, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.AssignedTo = workerID
	task.Status = TaskStatusInProgress
	return nil
}

// CompleteTask marks a task as completed.
func (s *CoordinatorState) CompleteTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	return nil
}

// FailTask marks a task as failed.
func (s *CoordinatorState) FailTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.Status = TaskStatusFailed
	now := time.Now()
	task.CompletedAt = &now
	return nil
}

// AddNotification records a task notification from a worker.
func (s *CoordinatorState) AddNotification(notification TaskNotification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = append(s.notifications, notification)
}

// PendingNotifications returns and clears unprocessed notifications.
func (s *CoordinatorState) PendingNotifications() []TaskNotification {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := s.notifications
	s.notifications = nil
	return pending
}

// ReadyTasks returns tasks whose dependencies are all completed.
func (s *CoordinatorState) ReadyTasks() []*CoordinatorTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ready []*CoordinatorTask
	for _, task := range s.tasks {
		if task.Status != TaskStatusPending {
			continue
		}
		allDepsComplete := true
		for _, dep := range task.DependsOn {
			if depTask, ok := s.tasks[dep]; ok {
				if depTask.Status != TaskStatusCompleted {
					allDepsComplete = false
					break
				}
			}
		}
		if allDepsComplete {
			ready = append(ready, task)
		}
	}
	return ready
}

// Workers returns all registered workers.
func (s *CoordinatorState) Workers() []WorkerIdentity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]WorkerIdentity, 0, len(s.workers))
	for _, w := range s.workers {
		result = append(result, *w)
	}
	return result
}

// Summary returns a human-readable summary of the coordinator state.
func (s *CoordinatorState) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	counts := map[TaskStatus]int{}
	for _, t := range s.tasks {
		counts[t.Status]++
	}

	fmt.Fprintf(&sb, "Tasks: %d total", len(s.tasks))
	if c := counts[TaskStatusPending]; c > 0 {
		fmt.Fprintf(&sb, ", %d pending", c)
	}
	if c := counts[TaskStatusInProgress]; c > 0 {
		fmt.Fprintf(&sb, ", %d in-progress", c)
	}
	if c := counts[TaskStatusCompleted]; c > 0 {
		fmt.Fprintf(&sb, ", %d completed", c)
	}
	if c := counts[TaskStatusFailed]; c > 0 {
		fmt.Fprintf(&sb, ", %d failed", c)
	}
	fmt.Fprintf(&sb, ". Workers: %d", len(s.workers))
	return sb.String()
}

// BuildWorkerSystemPromptContext generates the context section injected into
// a worker's system prompt. This gives the worker its identity and task context.
//
// Inspired by Claude Code's getCoordinatorUserContext() in coordinator/coordinatorMode.ts.
func BuildWorkerSystemPromptContext(worker WorkerIdentity, task *CoordinatorTask) string {
	var sb strings.Builder
	sb.WriteString("## Worker Context\n\n")
	fmt.Fprintf(&sb, "You are **%s**", worker.Name)
	if worker.Role != "" {
		fmt.Fprintf(&sb, " (%s)", worker.Role)
	}
	if worker.Team != "" {
		fmt.Fprintf(&sb, " on team **%s**", worker.Team)
	}
	sb.WriteString(".\n\n")

	if task != nil {
		fmt.Fprintf(&sb, "### Current Task\n\n")
		fmt.Fprintf(&sb, "**Phase:** %s\n", task.Phase)
		fmt.Fprintf(&sb, "**Objective:** %s\n\n", task.Description)
		sb.WriteString("When you complete this task, report your findings clearly.\n")
		sb.WriteString("Do not start work on unrelated tasks.\n")
	}

	return sb.String()
}

// --- Worker tool restrictions ---

// AsyncAgentAllowedTools is the set of tools that asynchronous workers can use.
// Workers spawned by the coordinator are restricted to this set to prevent
// dangerous or recursive operations.
//
// Inspired by Claude Code's ASYNC_AGENT_ALLOWED_TOOLS in constants/tools.ts.
var AsyncAgentAllowedTools = map[string]bool{
	"document_search":  true,
	"document-search":  true,
	"http-get":         true,
	"web-scraper":      true,
	"memory_store":     true,
	"memory_recall":    true,
	agentToolName:      false, // sub-agents cannot spawn further sub-agents by default
}

// AgentDisallowedTools is the set of tools that no agent (sync or async) should use.
// These tools are internal to the coordinator or require direct user interaction.
//
// Inspired by Claude Code's ALL_AGENT_DISALLOWED_TOOLS in constants/tools.ts.
var AgentDisallowedTools = map[string]bool{
	"ask_user":     true, // requires direct user interaction
	"plan_mode":    true, // coordinator-level only
	"task_output":  true, // internal coordinator tool
	"task_stop":    true, // internal coordinator tool
}

// CoordinatorModeAllowedTools is the set of tools the coordinator itself can use.
// The coordinator only orchestrates workers — it does not execute domain tools directly.
//
// Inspired by Claude Code's COORDINATOR_MODE_ALLOWED_TOOLS in constants/tools.ts:
// Agent, TaskStop, SendMessage, SyntheticOutput.
var CoordinatorModeAllowedTools = map[string]bool{
	agentToolName:    true,  // spawn workers
	"task_stop":      true,  // stop workers
	"send_message":   true,  // communicate with workers
	"tool_search":    true,  // discover tools for delegation
}

// InProcessTeammateAllowedTools extends AsyncAgentAllowedTools with task management
// tools. In-process teammates (vs external async agents) can manage tasks and
// communicate with peers.
//
// Inspired by Claude Code's IN_PROCESS_TEAMMATE_ALLOWED_TOOLS in constants/tools.ts.
var InProcessTeammateAllowedTools = map[string]bool{
	"task_create":    true,
	"task_get":       true,
	"task_list":      true,
	"task_update":    true,
	"send_message":   true,
}

// FilterToolsForWorker filters a list of tool names to only those allowed for
// an async worker agent. Tools in the disallow list are always removed.
// If allowedOverrides is provided, it replaces the default AsyncAgentAllowedTools.
//
// Inspired by Claude Code's filterToolsForAgent in agentToolUtils.ts.
func FilterToolsForWorker(availableTools []string, allowedOverrides map[string]bool) []string {
	allowed := AsyncAgentAllowedTools
	if allowedOverrides != nil {
		allowed = allowedOverrides
	}

	var filtered []string
	for _, tool := range availableTools {
		if AgentDisallowedTools[tool] {
			continue
		}
		if allowed[tool] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// ResolveWorkerTools determines the final set of tools a worker agent should receive.
// It takes the full tool set, applies the worker restriction filter, optionally
// applies agent-specific disallow rules (from agent definition), and optionally
// intersects with skill-level AllowedTools restrictions.
//
// skillAllowedTools comes from skill.AllowedTools (DB column allowed_tools) and
// restricts which tools the skill can use when invoked in worker/coordinator mode.
// Empty or nil means no additional restriction.
//
// Inspired by Claude Code's resolveAgentTools in agentToolUtils.ts and
// BundledSkillDefinition.allowedTools.
func ResolveWorkerTools(availableTools []string, agentDisallowed []string, skillAllowedTools []string) []string {
	// Start with the worker-filtered set.
	workerTools := FilterToolsForWorker(availableTools, nil)

	if len(agentDisallowed) == 0 && len(skillAllowedTools) == 0 {
		return workerTools
	}

	// Apply agent-specific disallow rules.
	disallowSet := make(map[string]bool, len(agentDisallowed))
	for _, d := range agentDisallowed {
		disallowSet[d] = true
	}

	// Apply skill-level allowed list as intersection filter.
	var skillAllowed map[string]bool
	if len(skillAllowedTools) > 0 {
		skillAllowed = make(map[string]bool, len(skillAllowedTools))
		for _, t := range skillAllowedTools {
			skillAllowed[t] = true
		}
	}

	var result []string
	for _, tool := range workerTools {
		if disallowSet[tool] {
			continue
		}
		if skillAllowed != nil && !skillAllowed[tool] {
			continue
		}
		result = append(result, tool)
	}
	return result
}

// BuildCoordinatorToolContext generates the tool context section that tells the
// coordinator what tools its workers have access to. This is injected into the
// coordinator's system prompt so it can make informed delegation decisions.
//
// Inspired by Claude Code's getCoordinatorUserContext() in coordinatorMode.ts.
func BuildCoordinatorToolContext(workerToolNames []string, mcpServerNames []string) string {
	var sb strings.Builder
	sb.WriteString("## Worker Tool Context\n\n")

	if len(workerToolNames) > 0 {
		sb.WriteString("Workers spawned via the agent tool have access to these tools: ")
		sb.WriteString(strings.Join(workerToolNames, ", "))
		sb.WriteString("\n")
	} else {
		sb.WriteString("Workers have no tools available.\n")
	}

	if len(mcpServerNames) > 0 {
		sb.WriteString("\nWorkers also have access to MCP tools from connected MCP servers: ")
		sb.WriteString(strings.Join(mcpServerNames, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// BuildCoordinatorSystemPrompt generates the complete system prompt for a
// coordinator agent that orchestrates workers.
//
// Inspired by Claude Code's getCoordinatorSystemPrompt() in coordinatorMode.ts.
func BuildCoordinatorSystemPrompt(agentName string, workerToolNames []string, mcpServerNames []string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "You are %s, an AI assistant that orchestrates software engineering tasks across multiple workers.\n\n", agentName)

	sb.WriteString("## Your Role\n\n")
	sb.WriteString("- Help the user achieve their goal\n")
	sb.WriteString("- Direct workers to research, implement and verify code changes\n")
	sb.WriteString("- Synthesize results and communicate with the user\n")
	sb.WriteString("- Answer questions directly when possible — don't delegate work you can handle without tools\n\n")

	sb.WriteString("## Your Tools\n\n")
	sb.WriteString("- **agent** — Spawn a new worker to handle a subtask\n")
	sb.WriteString("- **send_message** — Continue an existing worker with a follow-up\n")
	sb.WriteString("- **task_stop** — Stop a running worker\n\n")

	sb.WriteString("## Workers\n\n")
	sb.WriteString("Workers execute tasks autonomously. Each worker runs in isolation and reports back.\n")

	if len(workerToolNames) > 0 {
		sb.WriteString("Workers have access to: ")
		sb.WriteString(strings.Join(workerToolNames, ", "))
		sb.WriteString("\n")
	}

	if len(mcpServerNames) > 0 {
		sb.WriteString("Workers also have access to MCP tools from: ")
		sb.WriteString(strings.Join(mcpServerNames, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Task Workflow\n\n")
	sb.WriteString("Most tasks can be broken down into phases:\n")
	sb.WriteString("1. **Research** — Explore the codebase, understand the problem\n")
	sb.WriteString("2. **Synthesis** — Analyze findings, design the solution (your job)\n")
	sb.WriteString("3. **Implementation** — Make targeted changes per spec\n")
	sb.WriteString("4. **Verification** — Test that changes work correctly\n\n")

	sb.WriteString("## Guidelines\n\n")
	sb.WriteString("- Every message you send is to the user. Worker results are internal signals — never thank or acknowledge them.\n")
	sb.WriteString("- Prefer spawning focused workers over broad ones.\n")
	sb.WriteString("- When a worker fails, analyze why before retrying.\n")
	sb.WriteString("- Report progress at natural milestones.\n")

	return sb.String()
}
