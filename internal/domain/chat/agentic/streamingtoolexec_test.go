package agentic_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ConcToolStatus values ---

func TestConcToolStatus_Values(t *testing.T) {
	assert.Equal(t, agentic.ConcToolStatus("queued"), agentic.ConcToolQueued)
	assert.Equal(t, agentic.ConcToolStatus("executing"), agentic.ConcToolExecuting)
	assert.Equal(t, agentic.ConcToolStatus("completed"), agentic.ConcToolCompleted)
	assert.Equal(t, agentic.ConcToolStatus("yielded"), agentic.ConcToolYielded)
	assert.Equal(t, agentic.ConcToolStatus("failed"), agentic.ConcToolFailed)
}

// --- NewConcToolExecutor ---

func TestNewConcToolExecutor(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	assert.NotNil(t, e)
	assert.Equal(t, 0, e.Count())
	assert.True(t, e.AllDone())
}

// --- AddTool ---

func TestConcToolExecutor_AddTool(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	tt := e.AddTool("tc-1", "Read")
	require.NotNil(t, tt)
	assert.Equal(t, "tc-1", tt.ID)
	assert.Equal(t, "Read", tt.ToolName)
	assert.Equal(t, agentic.ConcToolQueued, tt.Status)
	assert.Equal(t, 1, e.Count())
}

func TestConcToolExecutor_AddTool_ConcurrencySafe(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{
		IsConcurrencySafe: func(name string) bool {
			return name == "Read" || name == "Grep"
		},
	})

	r := e.AddTool("tc-1", "Read")
	w := e.AddTool("tc-2", "Write")

	assert.True(t, r.ConcurrencySafe)
	assert.False(t, w.ConcurrencySafe)
}

// --- GetReady ---

func TestConcToolExecutor_GetReady_ConcurrentTools(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{
		IsConcurrencySafe: func(name string) bool {
			return name == "Read" || name == "Grep"
		},
	})

	e.AddTool("tc-1", "Read")
	e.AddTool("tc-2", "Grep")

	ready := e.GetReady()
	assert.Len(t, ready, 2)
}

func TestConcToolExecutor_GetReady_ExclusiveTool(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{
		IsConcurrencySafe: func(string) bool { return false },
	})

	e.AddTool("tc-1", "Write")
	e.AddTool("tc-2", "Edit")

	ready := e.GetReady()
	assert.Len(t, ready, 1)
}

func TestConcToolExecutor_GetReady_ExclusiveBlocksConcurrent(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{
		IsConcurrencySafe: func(name string) bool { return name == "Read" },
	})

	e.AddTool("tc-1", "Write")
	e.AddTool("tc-2", "Read")

	ready := e.GetReady()
	assert.Len(t, ready, 1)
	assert.Equal(t, "tc-1", ready[0].ID)

	e.MarkExecuting("tc-1")
	ready = e.GetReady()
	assert.Empty(t, ready)

	e.MarkCompleted("tc-1", "done")
	ready = e.GetReady()
	assert.Len(t, ready, 1)
	assert.Equal(t, "tc-2", ready[0].ID)
}

// --- MarkExecuting ---

func TestConcToolExecutor_MarkExecuting(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Read")

	ok := e.MarkExecuting("tc-1")
	assert.True(t, ok)
	assert.Equal(t, 1, e.ExecutingCount())
}

func TestConcToolExecutor_MarkExecuting_NonExistent(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	ok := e.MarkExecuting("nonexistent")
	assert.False(t, ok)
}

// --- MarkCompleted ---

func TestConcToolExecutor_MarkCompleted(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Read")
	e.MarkExecuting("tc-1")

	ok := e.MarkCompleted("tc-1", "result data")
	assert.True(t, ok)
	assert.Equal(t, 0, e.ExecutingCount())
}

// --- MarkFailed ---

func TestConcToolExecutor_MarkFailed(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Bash")
	e.MarkExecuting("tc-1")

	ok := e.MarkFailed("tc-1", fmt.Errorf("command failed"))
	assert.True(t, ok)
	assert.Equal(t, 0, e.ExecutingCount())
}

// --- Progress ---

func TestConcToolExecutor_Progress(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Read")

	e.AddProgress("tc-1", "loading file...")
	e.AddProgress("tc-1", "50% done")

	progress := e.YieldProgress()
	assert.Len(t, progress, 2)

	progress = e.YieldProgress()
	assert.Empty(t, progress)
}

// --- GetResults (FIFO) ---

func TestConcToolExecutor_GetResults_FIFO(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{
		IsConcurrencySafe: func(string) bool { return true },
	})

	e.AddTool("tc-1", "Read")
	e.AddTool("tc-2", "Grep")
	e.AddTool("tc-3", "Glob")

	e.MarkExecuting("tc-1")
	e.MarkExecuting("tc-2")
	e.MarkExecuting("tc-3")

	// tc-2 completes first (out of order)
	e.MarkCompleted("tc-2", "grep result")

	results := e.GetResults()
	assert.Empty(t, results, "FIFO blocked by tc-1")

	e.MarkCompleted("tc-1", "read result")

	results = e.GetResults()
	assert.Len(t, results, 2)
	assert.Equal(t, "tc-1", results[0].ID)
	assert.Equal(t, "tc-2", results[1].ID)

	e.MarkCompleted("tc-3", "glob result")
	results = e.GetResults()
	assert.Len(t, results, 1)
	assert.Equal(t, "tc-3", results[0].ID)
}

func TestConcToolExecutor_GetResults_IncludesFailed(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Bash")
	e.MarkExecuting("tc-1")
	e.MarkFailed("tc-1", fmt.Errorf("exit 1"))

	results := e.GetResults()
	assert.Len(t, results, 1)
	assert.NotNil(t, results[0].Error, "failed tool should have error")
}

// --- Discard ---

func TestConcToolExecutor_Discard(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Read")
	e.AddTool("tc-2", "Write")
	e.MarkExecuting("tc-1")

	e.Discard()

	assert.True(t, e.IsDiscarded())
	assert.Equal(t, 0, e.ExecutingCount())

	tt := e.AddTool("tc-3", "Glob")
	assert.Nil(t, tt)
}

// --- AllDone ---

func TestConcToolExecutor_AllDone(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Read")
	assert.False(t, e.AllDone())

	e.MarkExecuting("tc-1")
	assert.False(t, e.AllDone())

	e.MarkCompleted("tc-1", "done")
	assert.True(t, e.AllDone())
}

// --- Summary ---

func TestConcToolExecutor_Summary(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{})
	e.AddTool("tc-1", "Read")
	e.AddTool("tc-2", "Write")
	e.MarkExecuting("tc-1")
	e.MarkCompleted("tc-1", "done")

	s := e.ConcToolExecSummary()
	assert.Contains(t, s, "queued=1")
	assert.Contains(t, s, "completed=1")
}

// --- MaxConcurrent ---

func TestConcToolExecutor_MaxConcurrent(t *testing.T) {
	e := agentic.NewConcToolExecutor(agentic.ConcToolExecutorConfig{
		IsConcurrencySafe: func(string) bool { return true },
		MaxConcurrent:     2,
	})

	e.AddTool("tc-1", "Read")
	e.AddTool("tc-2", "Grep")
	e.AddTool("tc-3", "Glob")

	ready := e.GetReady()
	assert.Len(t, ready, 2)

	e.MarkExecuting("tc-1")
	e.MarkExecuting("tc-2")

	ready = e.GetReady()
	assert.Empty(t, ready)

	e.MarkCompleted("tc-1", "done")
	ready = e.GetReady()
	assert.Len(t, ready, 1)
}
