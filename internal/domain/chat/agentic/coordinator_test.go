package agentic_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// makeToolCalls creates ai.ToolCall slices for testing.
func makeToolCalls(names ...string) []ai.ToolCall {
	calls := make([]ai.ToolCall, len(names))
	for i, name := range names {
		calls[i] = ai.ToolCall{
			ID:   fmt.Sprintf("tc_%d", i),
			Type: "function",
			Function: ai.ToolFunction{
				Name:      name,
				Arguments: `{}`,
			},
		}
	}
	return calls
}

// --- CoordinatorState ---

func TestCoordinatorState_RegisterWorker(t *testing.T) {
	state := agentic.NewCoordinatorState()

	state.RegisterWorker(agentic.WorkerIdentity{
		ID:   "w1",
		Name: "researcher-1",
		Role: "research",
		Team: "alpha",
	})

	workers := state.Workers()
	assert.Len(t, workers, 1)
	assert.Equal(t, "researcher-1", workers[0].Name)
}

func TestCoordinatorState_TaskLifecycle(t *testing.T) {
	state := agentic.NewCoordinatorState()

	// Create task.
	id := state.CreateTask("Investigate auth module", agentic.PhaseResearch, nil)
	assert.NotEmpty(t, id)

	// Should be ready (no dependencies).
	ready := state.ReadyTasks()
	assert.Len(t, ready, 1)
	assert.Equal(t, id, ready[0].ID)
	assert.Equal(t, agentic.TaskStatusPending, ready[0].Status)

	// Assign to worker.
	err := state.AssignTask(id, "w1")
	assert.NoError(t, err)

	// Should no longer be in ready list (now in-progress).
	ready = state.ReadyTasks()
	assert.Len(t, ready, 0)

	// Complete the task.
	err = state.CompleteTask(id)
	assert.NoError(t, err)
}

func TestCoordinatorState_TaskDependencies(t *testing.T) {
	state := agentic.NewCoordinatorState()

	researchID := state.CreateTask("Research", agentic.PhaseResearch, nil)
	implID := state.CreateTask("Implement", agentic.PhaseImplementation, []string{researchID})

	// Only research should be ready.
	ready := state.ReadyTasks()
	assert.Len(t, ready, 1)
	assert.Equal(t, researchID, ready[0].ID)

	// Complete research.
	_ = state.CompleteTask(researchID)

	// Now implementation should be ready.
	ready = state.ReadyTasks()
	assert.Len(t, ready, 1)
	assert.Equal(t, implID, ready[0].ID)
}

func TestCoordinatorState_FailTask(t *testing.T) {
	state := agentic.NewCoordinatorState()

	id := state.CreateTask("Will fail", agentic.PhaseVerification, nil)
	err := state.FailTask(id)
	assert.NoError(t, err)
}

func TestCoordinatorState_AssignUnknownTask(t *testing.T) {
	state := agentic.NewCoordinatorState()
	err := state.AssignTask("nonexistent", "w1")
	assert.Error(t, err)
}

func TestCoordinatorState_CompleteUnknownTask(t *testing.T) {
	state := agentic.NewCoordinatorState()
	err := state.CompleteTask("nonexistent")
	assert.Error(t, err)
}

func TestCoordinatorState_Notifications(t *testing.T) {
	state := agentic.NewCoordinatorState()

	state.AddNotification(agentic.TaskNotification{
		WorkerID:   "w1",
		WorkerName: "researcher-1",
		TaskID:     "t1",
		Status:     agentic.TaskStatusCompleted,
		Summary:    "Found 3 relevant files",
	})

	pending := state.PendingNotifications()
	assert.Len(t, pending, 1)
	assert.Equal(t, "Found 3 relevant files", pending[0].Summary)

	// Second call should return empty (notifications consumed).
	pending = state.PendingNotifications()
	assert.Len(t, pending, 0)
}

func TestCoordinatorState_Summary(t *testing.T) {
	state := agentic.NewCoordinatorState()

	state.RegisterWorker(agentic.WorkerIdentity{ID: "w1", Name: "worker-1"})
	state.CreateTask("Task 1", agentic.PhaseResearch, nil)
	state.CreateTask("Task 2", agentic.PhaseImplementation, nil)

	summary := state.Summary()
	assert.Contains(t, summary, "Tasks: 2 total")
	assert.Contains(t, summary, "2 pending")
	assert.Contains(t, summary, "Workers: 1")
}

// --- BuildWorkerSystemPromptContext ---

func TestBuildWorkerSystemPromptContext_Basic(t *testing.T) {
	worker := agentic.WorkerIdentity{
		ID:   "w1",
		Name: "researcher-1",
		Role: "research specialist",
		Team: "alpha",
	}
	task := &agentic.CoordinatorTask{
		ID:          "t1",
		Description: "Find all authentication middleware in src/",
		Phase:       agentic.PhaseResearch,
	}

	ctx := agentic.BuildWorkerSystemPromptContext(worker, task)

	assert.Contains(t, ctx, "researcher-1")
	assert.Contains(t, ctx, "research specialist")
	assert.Contains(t, ctx, "alpha")
	assert.Contains(t, ctx, "Find all authentication middleware")
	assert.Contains(t, ctx, "research")
}

func TestBuildWorkerSystemPromptContext_NoTask(t *testing.T) {
	worker := agentic.WorkerIdentity{
		ID:   "w1",
		Name: "helper",
	}

	ctx := agentic.BuildWorkerSystemPromptContext(worker, nil)

	assert.Contains(t, ctx, "helper")
	assert.NotContains(t, ctx, "Current Task")
}

// --- FilterToolsForWorker ---

func TestFilterToolsForWorker_AllowedTools(t *testing.T) {
	available := []string{"document_search", "memory_store", "execute-sql", "http-get"}
	result := agentic.FilterToolsForWorker(available, nil)
	assert.Contains(t, result, "document_search")
	assert.Contains(t, result, "memory_store")
	assert.Contains(t, result, "http-get")
	assert.NotContains(t, result, "execute-sql")
}

func TestFilterToolsForWorker_DisallowedRemoved(t *testing.T) {
	available := []string{"ask_user", "document_search", "plan_mode"}
	result := agentic.FilterToolsForWorker(available, nil)
	assert.NotContains(t, result, "ask_user")
	assert.NotContains(t, result, "plan_mode")
	assert.Contains(t, result, "document_search")
}

func TestFilterToolsForWorker_CustomAllowed(t *testing.T) {
	available := []string{"document_search", "execute-sql", "custom-tool"}
	custom := map[string]bool{"execute-sql": true, "custom-tool": true}
	result := agentic.FilterToolsForWorker(available, custom)
	assert.Contains(t, result, "execute-sql")
	assert.Contains(t, result, "custom-tool")
	assert.NotContains(t, result, "document_search")
}

func TestFilterToolsForWorker_EmptyInput(t *testing.T) {
	result := agentic.FilterToolsForWorker(nil, nil)
	assert.Nil(t, result)
}

// --- ResolveWorkerTools ---

func TestResolveWorkerTools_NoAgentDisallow(t *testing.T) {
	available := []string{"document_search", "memory_store", "http-get"}
	result := agentic.ResolveWorkerTools(available, nil, nil)
	assert.Contains(t, result, "document_search")
	assert.Contains(t, result, "memory_store")
	assert.Contains(t, result, "http-get")
}

func TestResolveWorkerTools_WithAgentDisallow(t *testing.T) {
	available := []string{"document_search", "memory_store", "http-get"}
	result := agentic.ResolveWorkerTools(available, []string{"memory_store"}, nil)
	assert.Contains(t, result, "document_search")
	assert.NotContains(t, result, "memory_store")
	assert.Contains(t, result, "http-get")
}

func TestResolveWorkerTools_WithSkillAllowedTools(t *testing.T) {
	available := []string{"document_search", "memory_store", "http-get", "web-scraper"}
	// Skill only allows document_search and web-scraper.
	result := agentic.ResolveWorkerTools(available, nil, []string{"document_search", "web-scraper"})
	assert.Contains(t, result, "document_search")
	assert.Contains(t, result, "web-scraper")
	assert.NotContains(t, result, "memory_store")
	assert.NotContains(t, result, "http-get")
}

func TestResolveWorkerTools_WithBothDisallowAndSkillAllowed(t *testing.T) {
	available := []string{"document_search", "memory_store", "http-get", "web-scraper"}
	// Skill allows document_search and memory_store, but agent disallows memory_store.
	result := agentic.ResolveWorkerTools(available, []string{"memory_store"}, []string{"document_search", "memory_store"})
	assert.Contains(t, result, "document_search")
	assert.NotContains(t, result, "memory_store") // disallowed by agent
	assert.NotContains(t, result, "http-get")     // not in skill allowed list
	assert.NotContains(t, result, "web-scraper")  // not in skill allowed list
}

// --- BuildCoordinatorToolContext ---

func TestBuildCoordinatorToolContext_WithTools(t *testing.T) {
	ctx := agentic.BuildCoordinatorToolContext(
		[]string{"document_search", "http-get"},
		[]string{"github-mcp", "slack-mcp"},
	)
	assert.Contains(t, ctx, "document_search")
	assert.Contains(t, ctx, "http-get")
	assert.Contains(t, ctx, "github-mcp")
	assert.Contains(t, ctx, "slack-mcp")
	assert.Contains(t, ctx, "MCP")
}

func TestBuildCoordinatorToolContext_NoMCP(t *testing.T) {
	ctx := agentic.BuildCoordinatorToolContext(
		[]string{"document_search"},
		nil,
	)
	assert.Contains(t, ctx, "document_search")
	assert.NotContains(t, ctx, "MCP")
}

func TestBuildCoordinatorToolContext_NoTools(t *testing.T) {
	ctx := agentic.BuildCoordinatorToolContext(nil, nil)
	assert.Contains(t, ctx, "no tools available")
}

// --- BuildCoordinatorSystemPrompt ---

func TestBuildCoordinatorSystemPrompt_ContainsAllSections(t *testing.T) {
	prompt := agentic.BuildCoordinatorSystemPrompt(
		"AgentHub Coordinator",
		[]string{"document_search", "http-get"},
		[]string{"github-mcp"},
	)
	assert.Contains(t, prompt, "AgentHub Coordinator")
	assert.Contains(t, prompt, "Your Role")
	assert.Contains(t, prompt, "Your Tools")
	assert.Contains(t, prompt, "Workers")
	assert.Contains(t, prompt, "document_search")
	assert.Contains(t, prompt, "http-get")
	assert.Contains(t, prompt, "github-mcp")
	assert.Contains(t, prompt, "Task Workflow")
	assert.Contains(t, prompt, "Research")
	assert.Contains(t, prompt, "Implementation")
	assert.Contains(t, prompt, "Verification")
	assert.Contains(t, prompt, "Guidelines")
}

func TestBuildCoordinatorSystemPrompt_NoWorkerTools(t *testing.T) {
	prompt := agentic.BuildCoordinatorSystemPrompt("Coord", nil, nil)
	assert.Contains(t, prompt, "Coord")
	assert.NotContains(t, prompt, "MCP")
}

// --- PartitionToolCalls (from toolexec.go) ---

func TestPartitionToolCalls_Empty(t *testing.T) {
	batches := agentic.PartitionToolCalls(nil, nil)
	assert.Nil(t, batches)
}

func TestPartitionToolCalls_AllReadOnly(t *testing.T) {
	calls := makeToolCalls("document_search", "document_search", "memory_store")
	batches := agentic.PartitionToolCalls(calls, nil)

	// document_search is read-only, memory_store is NOT → should split.
	assert.Len(t, batches, 2)
	assert.True(t, batches[0].IsConcurrencySafe)
	assert.Len(t, batches[0].Indices, 2) // Two doc searches.
	assert.False(t, batches[1].IsConcurrencySafe)
	assert.Len(t, batches[1].Indices, 1) // memory_store.
}

func TestPartitionToolCalls_MixedOrder(t *testing.T) {
	// [read, read, write, read] → [{read,read}, {write}, {read}]
	calls := makeToolCalls("document_search", "document_search", "execute_sql", "document_search")
	batches := agentic.PartitionToolCalls(calls, nil)

	assert.Len(t, batches, 3)
	assert.True(t, batches[0].IsConcurrencySafe)
	assert.Len(t, batches[0].Indices, 2)
	assert.False(t, batches[1].IsConcurrencySafe)
	assert.Len(t, batches[1].Indices, 1)
	assert.True(t, batches[2].IsConcurrencySafe)
	assert.Len(t, batches[2].Indices, 1)
}

func TestPartitionToolCalls_AllWrite(t *testing.T) {
	calls := makeToolCalls("execute_sql", "execute_http")
	batches := agentic.PartitionToolCalls(calls, nil)

	// Each write tool gets its own batch.
	assert.Len(t, batches, 2)
	assert.False(t, batches[0].IsConcurrencySafe)
	assert.False(t, batches[1].IsConcurrencySafe)
}

func TestPartitionToolCalls_SingleRead(t *testing.T) {
	calls := makeToolCalls("document_search")
	batches := agentic.PartitionToolCalls(calls, nil)

	assert.Len(t, batches, 1)
	assert.True(t, batches[0].IsConcurrencySafe)
}

func TestPartitionToolCalls_DBReadOnlyOverride(t *testing.T) {
	// "custom-tool" is NOT in the hardcoded readOnlySlugs map,
	// but the DB marks it as read_only via the readOnlyIndex.
	calls := makeToolCalls("custom-tool", "document_search", "execute_sql")
	dbIndex := map[string]bool{"custom-tool": true}
	batches := agentic.PartitionToolCalls(calls, dbIndex)

	// custom-tool (DB read-only) + document_search (hardcoded) should merge.
	assert.Len(t, batches, 2)
	assert.True(t, batches[0].IsConcurrencySafe)
	assert.Len(t, batches[0].Indices, 2) // custom-tool + document_search
	assert.False(t, batches[1].IsConcurrencySafe)
	assert.Len(t, batches[1].Indices, 1) // execute_sql
}

func TestBuildReadOnlyIndex(t *testing.T) {
	tools := []agentic.LLMTool{
		{Name: "read-tool", ReadOnly: true},
		{Name: "write-tool", ReadOnly: false},
		{Name: "another-read", ReadOnly: true},
	}
	idx := agentic.BuildReadOnlyIndex(tools)
	assert.True(t, idx["read-tool"])
	assert.True(t, idx["another-read"])
	assert.False(t, idx["write-tool"])
}

func TestBuildDestructiveIndex(t *testing.T) {
	tools := []agentic.LLMTool{
		{Name: "delete-agent", IsDestructive: true},
		{Name: "list-agents", IsDestructive: false},
		{Name: "drop-table", IsDestructive: true},
	}
	idx := agentic.BuildDestructiveIndex(tools)
	assert.True(t, idx["delete-agent"])
	assert.True(t, idx["drop-table"])
	assert.False(t, idx["list-agents"])
}

func TestDeferredToolThreshold(t *testing.T) {
	// Constant should be reasonable for prompt cache optimization.
	assert.GreaterOrEqual(t, agentic.DeferredToolThreshold, 10)
	assert.LessOrEqual(t, agentic.DeferredToolThreshold, 30)
}

// --- TaskPhase constants ---

func TestTaskPhaseConstants(t *testing.T) {
	assert.Equal(t, agentic.TaskPhase("research"), agentic.PhaseResearch)
	assert.Equal(t, agentic.TaskPhase("synthesis"), agentic.PhaseSynthesis)
	assert.Equal(t, agentic.TaskPhase("implementation"), agentic.PhaseImplementation)
	assert.Equal(t, agentic.TaskPhase("verification"), agentic.PhaseVerification)
}
