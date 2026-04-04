package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- TaskType constants ---

func TestTaskType_Constants(t *testing.T) {
	assert.Equal(t, agentic.TaskType("local_bash"), agentic.TaskLocalBash)
	assert.Equal(t, agentic.TaskType("local_agent"), agentic.TaskLocalAgent)
	assert.Equal(t, agentic.TaskType("remote_agent"), agentic.TaskRemoteAgent)
	assert.Equal(t, agentic.TaskType("in_process_teammate"), agentic.TaskInProcessTeammate)
	assert.Equal(t, agentic.TaskType("local_workflow"), agentic.TaskLocalWorkflow)
	assert.Equal(t, agentic.TaskType("monitor_mcp"), agentic.TaskMonitorMCP)
	assert.Equal(t, agentic.TaskType("dream"), agentic.TaskDream)
}

// --- TaskStatus constants ---

func TestTaskStatus_Constants(t *testing.T) {
	assert.Equal(t, agentic.BGTaskStatus("pending"), agentic.BGTaskPending)
	assert.Equal(t, agentic.BGTaskStatus("running"), agentic.BGTaskRunning)
	assert.Equal(t, agentic.BGTaskStatus("completed"), agentic.BGTaskCompleted)
	assert.Equal(t, agentic.BGTaskStatus("failed"), agentic.BGTaskFailed)
	assert.Equal(t, agentic.BGTaskStatus("killed"), agentic.BGTaskKilled)
}

// --- IsTerminalTaskStatus ---

func TestIsTerminalTaskStatus_Terminal(t *testing.T) {
	assert.True(t, agentic.IsTerminalBGTaskStatus(agentic.BGTaskCompleted))
	assert.True(t, agentic.IsTerminalBGTaskStatus(agentic.BGTaskFailed))
	assert.True(t, agentic.IsTerminalBGTaskStatus(agentic.BGTaskKilled))
}

func TestIsTerminalTaskStatus_NonTerminal(t *testing.T) {
	assert.False(t, agentic.IsTerminalBGTaskStatus(agentic.BGTaskPending))
	assert.False(t, agentic.IsTerminalBGTaskStatus(agentic.BGTaskRunning))
}

// --- GenerateTaskID ---

func TestGenerateTaskID_Prefix(t *testing.T) {
	tests := []struct {
		typ    agentic.TaskType
		prefix byte
	}{
		{agentic.TaskLocalBash, 'b'},
		{agentic.TaskLocalAgent, 'a'},
		{agentic.TaskRemoteAgent, 'r'},
		{agentic.TaskInProcessTeammate, 't'},
		{agentic.TaskLocalWorkflow, 'w'},
		{agentic.TaskMonitorMCP, 'm'},
		{agentic.TaskDream, 'd'},
	}
	for _, tt := range tests {
		id := agentic.GenerateTaskID(tt.typ)
		assert.Equal(t, tt.prefix, id[0], "type=%s", tt.typ)
		assert.Len(t, id, 9, "type=%s", tt.typ)
	}
}

func TestGenerateTaskID_UnknownType(t *testing.T) {
	id := agentic.GenerateTaskID("unknown_type")
	assert.Equal(t, byte('x'), id[0])
	assert.Len(t, id, 9)
}

func TestGenerateTaskID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := agentic.GenerateTaskID(agentic.TaskLocalBash)
		assert.False(t, ids[id], "duplicate ID: %s", id)
		ids[id] = true
	}
}

func TestGenerateTaskID_ValidChars(t *testing.T) {
	id := agentic.GenerateTaskID(agentic.TaskLocalBash)
	for i := 1; i < len(id); i++ {
		c := id[i]
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z'), "invalid char %c at pos %d", c, i)
	}
}

// --- GetTaskOutputPath ---

func TestGetTaskOutputPath(t *testing.T) {
	path := agentic.GetTaskOutputPath("b12345678")
	assert.Equal(t, "/tmp/agenthub-task-b12345678.log", path)
}

// --- CreateTaskStateBase ---

func TestCreateTaskStateBase(t *testing.T) {
	ts := agentic.CreateTaskStateBase("b00000000", agentic.TaskLocalBash, "Run tests", "tu_1")
	assert.Equal(t, "b00000000", ts.ID)
	assert.Equal(t, agentic.TaskLocalBash, ts.Type)
	assert.Equal(t, agentic.BGTaskPending, ts.Status)
	assert.Equal(t, "Run tests", ts.Description)
	assert.Equal(t, "tu_1", ts.ToolUseID)
	assert.Greater(t, ts.StartTime, int64(0))
	assert.Equal(t, int64(0), ts.EndTime)
	assert.False(t, ts.Notified)
	assert.Contains(t, ts.OutputFile, "b00000000")
}

// --- NewTask ---

func TestNewTask(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalAgent, "Analyze code", "")
	assert.Equal(t, byte('a'), ts.ID[0])
	assert.Equal(t, agentic.TaskLocalAgent, ts.Type)
	assert.Equal(t, agentic.BGTaskPending, ts.Status)
}

// --- State transitions ---

func TestTaskStateBase_MarkRunning(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	ts.MarkRunning()
	assert.Equal(t, agentic.BGTaskRunning, ts.Status)
	assert.Equal(t, int64(0), ts.EndTime, "running should not set end time")
}

func TestTaskStateBase_MarkCompleted(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	ts.MarkRunning()
	ts.MarkCompleted()
	assert.Equal(t, agentic.BGTaskCompleted, ts.Status)
	assert.Greater(t, ts.EndTime, int64(0))
}

func TestTaskStateBase_MarkFailed(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	ts.MarkFailed()
	assert.Equal(t, agentic.BGTaskFailed, ts.Status)
	assert.Greater(t, ts.EndTime, int64(0))
}

func TestTaskStateBase_MarkKilled(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	ts.MarkKilled()
	assert.Equal(t, agentic.BGTaskKilled, ts.Status)
	assert.Greater(t, ts.EndTime, int64(0))
}

// --- IsTerminal ---

func TestTaskStateBase_IsTerminal(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	assert.False(t, ts.IsTerminal())

	ts.MarkRunning()
	assert.False(t, ts.IsTerminal())

	ts.MarkCompleted()
	assert.True(t, ts.IsTerminal())
}

// --- Duration ---

func TestTaskStateBase_Duration_Active(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	time.Sleep(10 * time.Millisecond)
	dur := ts.Duration()
	assert.GreaterOrEqual(t, dur.Milliseconds(), int64(10))
}

func TestTaskStateBase_Duration_Completed(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	time.Sleep(10 * time.Millisecond)
	ts.MarkCompleted()
	dur := ts.Duration()
	assert.GreaterOrEqual(t, dur.Milliseconds(), int64(10))

	// Duration should be fixed after completion.
	time.Sleep(10 * time.Millisecond)
	dur2 := ts.Duration()
	assert.Equal(t, dur, dur2)
}

// --- IsBackground ---

func TestTaskStateBase_IsBackground(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskLocalBash, "test", "")
	assert.True(t, ts.IsBackground(), "pending should be backgroundable")

	ts.MarkRunning()
	assert.True(t, ts.IsBackground(), "running should be backgroundable")

	ts.MarkCompleted()
	assert.False(t, ts.IsBackground(), "completed should not be backgroundable")
}

// --- TodoTaskStatus constants ---

func TestTodoTaskStatus_Constants(t *testing.T) {
	assert.Equal(t, agentic.TodoTaskStatus("pending"), agentic.TodoPending)
	assert.Equal(t, agentic.TodoTaskStatus("in_progress"), agentic.TodoInProgress)
	assert.Equal(t, agentic.TodoTaskStatus("completed"), agentic.TodoCompleted)
}

// --- TodoTask ---

func TestTodoTask_IsBlocked(t *testing.T) {
	task := agentic.TodoTask{
		ID:      "t1",
		Subject: "Task 1",
		Status:  agentic.TodoPending,
		Blocks:  []string{},
		BlockedBy: []string{},
	}
	assert.False(t, task.IsBlocked())

	task.BlockedBy = []string{"t0"}
	assert.True(t, task.IsBlocked())
}

func TestTodoTask_Fields(t *testing.T) {
	task := agentic.TodoTask{
		ID:          "t1",
		Subject:     "Implement feature",
		Description: "Full description",
		ActiveForm:  "implementing feature",
		Owner:       "agent-1",
		Status:      agentic.TodoInProgress,
		Blocks:      []string{"t2", "t3"},
		BlockedBy:   []string{},
		Metadata:    map[string]any{"priority": "high"},
	}
	assert.Equal(t, "t1", task.ID)
	assert.Equal(t, "agent-1", task.Owner)
	assert.Len(t, task.Blocks, 2)
	assert.Equal(t, "high", task.Metadata["priority"])
}

// --- ClaimTaskResult ---

func TestClaimTaskResult_Success(t *testing.T) {
	task := &agentic.TodoTask{ID: "t1", Subject: "Test"}
	result := agentic.ClaimTaskResult{
		Success: true,
		Task:    task,
	}
	assert.True(t, result.Success)
	require.NotNil(t, result.Task)
	assert.Equal(t, "t1", result.Task.ID)
}

func TestClaimTaskResult_Blocked(t *testing.T) {
	result := agentic.ClaimTaskResult{
		Success:        false,
		Reason:         "blocked",
		BlockedByTasks: []string{"t0"},
	}
	assert.False(t, result.Success)
	assert.Equal(t, "blocked", result.Reason)
	assert.Len(t, result.BlockedByTasks, 1)
}

func TestClaimTaskResult_AgentBusy(t *testing.T) {
	result := agentic.ClaimTaskResult{
		Success:       false,
		Reason:        "agent_busy",
		BusyWithTasks: []string{"t5", "t6"},
	}
	assert.False(t, result.Success)
	assert.Len(t, result.BusyWithTasks, 2)
}

// --- Full lifecycle ---

func TestTaskLifecycle_PendingToCompleted(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskRemoteAgent, "Deploy check", "tu_42")
	assert.Equal(t, agentic.BGTaskPending, ts.Status)
	assert.True(t, ts.IsBackground())
	assert.False(t, ts.IsTerminal())

	ts.MarkRunning()
	assert.Equal(t, agentic.BGTaskRunning, ts.Status)
	assert.True(t, ts.IsBackground())

	ts.MarkCompleted()
	assert.Equal(t, agentic.BGTaskCompleted, ts.Status)
	assert.False(t, ts.IsBackground())
	assert.True(t, ts.IsTerminal())
	assert.GreaterOrEqual(t, ts.EndTime, ts.StartTime)
}

func TestTaskLifecycle_PendingToKilled(t *testing.T) {
	ts := agentic.NewTask(agentic.TaskInProcessTeammate, "Help with refactor", "")
	ts.MarkRunning()
	ts.MarkKilled()
	assert.True(t, ts.IsTerminal())
	assert.Equal(t, agentic.BGTaskKilled, ts.Status)
}
