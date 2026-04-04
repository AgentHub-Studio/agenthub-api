package agentic

import (
	"crypto/rand"
	"fmt"
	"time"
)

// TaskType classifies the kind of background task.
//
// Inspired by Claude Code's TaskType in Task.ts.
type TaskType string

const (
	TaskLocalBash          TaskType = "local_bash"
	TaskLocalAgent         TaskType = "local_agent"
	TaskRemoteAgent        TaskType = "remote_agent"
	TaskInProcessTeammate  TaskType = "in_process_teammate"
	TaskLocalWorkflow      TaskType = "local_workflow"
	TaskMonitorMCP         TaskType = "monitor_mcp"
	TaskDream              TaskType = "dream"
)

// BGTaskStatus represents the lifecycle state of a background task.
//
// Inspired by Claude Code's TaskStatus in Task.ts.
type BGTaskStatus string

const (
	BGTaskPending   BGTaskStatus = "pending"
	BGTaskRunning   BGTaskStatus = "running"
	BGTaskCompleted BGTaskStatus = "completed"
	BGTaskFailed    BGTaskStatus = "failed"
	BGTaskKilled    BGTaskStatus = "killed"
)

// IsTerminalBGTaskStatus returns true when the status is a terminal state
// (completed, failed, or killed) and will not transition further.
// Used to guard against injecting messages into dead tasks, evicting
// finished tasks, and orphan-cleanup paths.
//
// Inspired by Claude Code's isTerminalTaskStatus.
func IsTerminalBGTaskStatus(status BGTaskStatus) bool {
	return status == BGTaskCompleted || status == BGTaskFailed || status == BGTaskKilled
}

// TaskStateBase carries the common state shared by all task variants.
// Each concrete task type embeds this and adds type-specific fields.
//
// Inspired by Claude Code's TaskStateBase in Task.ts.
type TaskStateBase struct {
	// ID is the unique task identifier (prefix + 8 random chars).
	ID string `json:"id"`
	// Type classifies the task variant.
	Type TaskType `json:"type"`
	// Status is the current lifecycle state.
	Status BGTaskStatus `json:"status"`
	// Description is a human-readable summary of what this task does.
	Description string `json:"description"`
	// ToolUseID links back to the tool_use that created this task, if any.
	ToolUseID string `json:"toolUseId,omitempty"`
	// StartTime is when the task was created (Unix millis).
	StartTime int64 `json:"startTime"`
	// EndTime is when the task reached a terminal state (Unix millis). Zero if still active.
	EndTime int64 `json:"endTime,omitempty"`
	// TotalPausedMs tracks cumulative paused time in milliseconds.
	TotalPausedMs int64 `json:"totalPausedMs,omitempty"`
	// OutputFile is the path where task output is written.
	OutputFile string `json:"outputFile"`
	// OutputOffset tracks how much of the output file has been read.
	OutputOffset int64 `json:"outputOffset"`
	// Notified indicates whether the user has been notified of task completion.
	Notified bool `json:"notified"`
}

// taskIDAlphabet is the set of characters used for random task ID suffixes.
const taskIDAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// taskTypePrefix maps each task type to its single-character ID prefix.
var taskTypePrefix = map[TaskType]byte{
	TaskLocalBash:         'b',
	TaskLocalAgent:        'a',
	TaskRemoteAgent:       'r',
	TaskInProcessTeammate: 't',
	TaskLocalWorkflow:     'w',
	TaskMonitorMCP:        'm',
	TaskDream:             'd',
}

// GenerateTaskID creates a unique task ID with a type-specific prefix
// followed by 8 random alphanumeric characters (~41 bits of entropy).
//
// Inspired by Claude Code's generateTaskId in Task.ts.
func GenerateTaskID(typ TaskType) string {
	prefix, ok := taskTypePrefix[typ]
	if !ok {
		prefix = 'x' // unknown type fallback
	}

	buf := make([]byte, 8)
	_, _ = rand.Read(buf)

	id := make([]byte, 9)
	id[0] = prefix
	for i := 0; i < 8; i++ {
		id[i+1] = taskIDAlphabet[int(buf[i])%len(taskIDAlphabet)]
	}
	return string(id)
}

// GetTaskOutputPath returns the conventional output file path for a task ID.
func GetTaskOutputPath(id string) string {
	return fmt.Sprintf("/tmp/agenthub-task-%s.log", id)
}

// CreateTaskStateBase initializes a new TaskStateBase in pending status.
//
// Inspired by Claude Code's createTaskStateBase.
func CreateTaskStateBase(id string, typ TaskType, description string, toolUseID string) TaskStateBase {
	return TaskStateBase{
		ID:          id,
		Type:        typ,
		Status:      BGTaskPending,
		Description: description,
		ToolUseID:   toolUseID,
		StartTime:   time.Now().UnixMilli(),
		OutputFile:  GetTaskOutputPath(id),
		OutputOffset: 0,
		Notified:    false,
	}
}

// NewTask is a convenience that generates an ID and creates a TaskStateBase.
func NewTask(typ TaskType, description string, toolUseID string) TaskStateBase {
	id := GenerateTaskID(typ)
	return CreateTaskStateBase(id, typ, description, toolUseID)
}

// MarkRunning transitions the task to running state.
func (t *TaskStateBase) MarkRunning() {
	t.Status = BGTaskRunning
}

// MarkCompleted transitions the task to completed state.
func (t *TaskStateBase) MarkCompleted() {
	t.Status = BGTaskCompleted
	t.EndTime = time.Now().UnixMilli()
}

// MarkFailed transitions the task to failed state.
func (t *TaskStateBase) MarkFailed() {
	t.Status = BGTaskFailed
	t.EndTime = time.Now().UnixMilli()
}

// MarkKilled transitions the task to killed state.
func (t *TaskStateBase) MarkKilled() {
	t.Status = BGTaskKilled
	t.EndTime = time.Now().UnixMilli()
}

// IsTerminal returns true if this task is in a terminal state.
func (t *TaskStateBase) IsTerminal() bool {
	return IsTerminalBGTaskStatus(t.Status)
}

// Duration returns the elapsed time from start to end (or now if still active).
func (t *TaskStateBase) Duration() time.Duration {
	end := t.EndTime
	if end == 0 {
		end = time.Now().UnixMilli()
	}
	return time.Duration(end-t.StartTime) * time.Millisecond
}

// IsBackground returns true if the task is in a backgroundable state
// (pending or running). Terminal tasks are not backgrounded.
//
// Inspired by Claude Code's isBackgroundTask.
func (t *TaskStateBase) IsBackground() bool {
	return t.Status == BGTaskPending || t.Status == BGTaskRunning
}

// --- TodoTask (task management with dependencies) ---

// TodoTaskStatus represents the lifecycle of a managed todo/task item.
type TodoTaskStatus string

const (
	TodoPending    TodoTaskStatus = "pending"
	TodoInProgress TodoTaskStatus = "in_progress"
	TodoCompleted  TodoTaskStatus = "completed"
)

// TodoTask represents a managed task with dependency tracking.
//
// Inspired by Claude Code's Task in utils/tasks.ts.
type TodoTask struct {
	ID          string            `json:"id"`
	Subject     string            `json:"subject"`
	Description string            `json:"description"`
	ActiveForm  string            `json:"activeForm,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Status      TodoTaskStatus    `json:"status"`
	Blocks      []string          `json:"blocks"`
	BlockedBy   []string          `json:"blockedBy"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// IsBlocked returns true if this task has unresolved blockers.
func (t *TodoTask) IsBlocked() bool {
	return len(t.BlockedBy) > 0
}

// ClaimTaskResult holds the outcome of attempting to claim a task for execution.
//
// Inspired by Claude Code's ClaimTaskResult.
type ClaimTaskResult struct {
	Success        bool     `json:"success"`
	Reason         string   `json:"reason,omitempty"` // task_not_found, already_claimed, already_resolved, blocked, agent_busy
	Task           *TodoTask `json:"task,omitempty"`
	BusyWithTasks  []string `json:"busyWithTasks,omitempty"`
	BlockedByTasks []string `json:"blockedByTasks,omitempty"`
}
