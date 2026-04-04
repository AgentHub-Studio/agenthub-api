package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestSpeculation_Constants(t *testing.T) {
	assert.Equal(t, 20, agentic.SpeculationMaxTurns)
	assert.Equal(t, 100, agentic.SpeculationMaxMessages)
}

func TestSpeculationStatus_Values(t *testing.T) {
	assert.Equal(t, agentic.SpeculationStatus("idle"), agentic.SpeculationIdle)
	assert.Equal(t, agentic.SpeculationStatus("active"), agentic.SpeculationActive)
	assert.Equal(t, agentic.SpeculationStatus("done"), agentic.SpeculationDone)
	assert.Equal(t, agentic.SpeculationStatus("aborted"), agentic.SpeculationAborted)
}

func TestCompletionBoundary_Values(t *testing.T) {
	assert.Equal(t, agentic.CompletionBoundary("complete"), agentic.BoundaryComplete)
	assert.Equal(t, agentic.CompletionBoundary("denied_tool"), agentic.BoundaryToolDenied)
	assert.Equal(t, agentic.CompletionBoundary("max_turns"), agentic.BoundaryMaxTurns)
}

// --- NewSpeculativeExecution ---

func TestNewSpeculativeExecution(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", []string{"Read", "Glob", "Grep"})
	assert.Equal(t, "spec-1", s.ID())
	assert.Equal(t, agentic.SpeculationActive, s.Status())
	assert.True(t, s.IsActive())
}

// --- RecordTurn ---

func TestSpeculativeExecution_RecordTurn(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	assert.True(t, s.RecordTurn())
	assert.True(t, s.RecordTurn())

	result := s.Result()
	assert.Equal(t, 2, result.Turns)
}

func TestSpeculativeExecution_RecordTurn_MaxReached(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	for i := 0; i < agentic.SpeculationMaxTurns-1; i++ {
		assert.True(t, s.RecordTurn())
	}
	assert.False(t, s.RecordTurn(), "should stop at max turns")
	assert.Equal(t, agentic.SpeculationDone, s.Status())
	assert.Equal(t, agentic.BoundaryMaxTurns, s.Result().Boundary)
}

// --- RecordMessage ---

func TestSpeculativeExecution_RecordMessage(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	assert.True(t, s.RecordMessage())
	assert.Equal(t, 1, s.Result().Messages)
}

func TestSpeculativeExecution_RecordMessage_MaxReached(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	for i := 0; i < agentic.SpeculationMaxMessages-1; i++ {
		s.RecordMessage()
	}
	assert.False(t, s.RecordMessage())
	assert.Equal(t, agentic.BoundaryMaxMessages, s.Result().Boundary)
}

// --- IsToolAllowed ---

func TestSpeculativeExecution_IsToolAllowed(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", []string{"Read", "Glob", "Grep"})
	assert.True(t, s.IsToolAllowed("Read"))
	assert.True(t, s.IsToolAllowed("Glob"))
	assert.False(t, s.IsToolAllowed("Write"))
	assert.False(t, s.IsToolAllowed("Edit"))
}

// --- DenyTool ---

func TestSpeculativeExecution_DenyTool(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.DenyTool("Write")
	assert.Equal(t, agentic.SpeculationDone, s.Status())
	assert.Equal(t, agentic.BoundaryToolDenied, s.Result().Boundary)
}

// --- Complete ---

func TestSpeculativeExecution_Complete(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.Complete()
	assert.Equal(t, agentic.SpeculationDone, s.Status())
	assert.Equal(t, agentic.BoundaryComplete, s.Result().Boundary)
}

// --- Abort ---

func TestSpeculativeExecution_Abort(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.Abort()
	assert.Equal(t, agentic.SpeculationAborted, s.Status())
	assert.True(t, s.AbortController().IsAborted())
}

func TestSpeculativeExecution_Abort_AlreadyDone(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.Complete()
	s.Abort() // should not change status
	assert.Equal(t, agentic.SpeculationDone, s.Status())
}

// --- RecordWrite ---

func TestSpeculativeExecution_RecordWrite(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.RecordWrite("/main.go")
	s.RecordWrite("/test.go")
	s.RecordWrite("/main.go") // duplicate

	paths := s.WrittenPaths()
	assert.Len(t, paths, 2)
}

// --- Result ---

func TestSpeculativeExecution_Result(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.RecordTurn()
	s.RecordMessage()
	s.RecordMessage()
	s.RecordWrite("/main.go")
	s.Complete()

	result := s.Result()
	assert.Equal(t, agentic.BoundaryComplete, result.Boundary)
	assert.Equal(t, 1, result.Turns)
	assert.Equal(t, 2, result.Messages)
	require.Len(t, result.WrittenPaths, 1)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}

// --- Summary ---

func TestSpeculativeExecution_Summary(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.RecordTurn()
	s.Complete()

	summary := s.Summary()
	assert.Contains(t, summary, "spec-1")
	assert.Contains(t, summary, "done")
	assert.Contains(t, summary, "1 turns")
}

// --- Not active after completion ---

func TestSpeculativeExecution_NotActiveAfterComplete(t *testing.T) {
	s := agentic.NewSpeculativeExecution("spec-1", nil)
	s.Complete()
	assert.False(t, s.IsActive())
	assert.False(t, s.RecordTurn())
	assert.False(t, s.RecordMessage())
}
