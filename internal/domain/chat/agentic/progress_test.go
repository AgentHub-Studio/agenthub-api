package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunProgressTracker_NewTracker(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	snap := tracker.Snapshot()
	assert.Equal(t, 0, snap.TurnIndex)
	assert.Equal(t, 0, snap.TotalTokens)
	assert.Equal(t, 0, snap.TotalToolCalls)
	assert.Equal(t, 0.0, snap.TotalCostUSD)
	assert.Equal(t, 0, snap.ActiveSubtasks)
	assert.Empty(t, snap.RecentActivity)
}

func TestRunProgressTracker_RecordLLMCall(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.RecordLLMCall(1000, 0.05, "claude-sonnet-4")
	tracker.RecordLLMCall(500, 0.02, "claude-sonnet-4")

	snap := tracker.Snapshot()
	assert.Equal(t, 1500, snap.TotalTokens)
	assert.InDelta(t, 0.07, snap.TotalCostUSD, 0.001)
	require.Len(t, snap.RecentActivity, 2)
	assert.Equal(t, ActivityLLMCall, snap.RecentActivity[0].Type)
	assert.Contains(t, snap.RecentActivity[0].Summary, "claude-sonnet-4")
}

func TestRunProgressTracker_RecordToolCall(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.RecordToolCall("document_search")
	tracker.RecordToolCall("execute_sql")

	snap := tracker.Snapshot()
	assert.Equal(t, 2, snap.TotalToolCalls)
	require.Len(t, snap.RecentActivity, 2)
	assert.Equal(t, ActivityToolCall, snap.RecentActivity[0].Type)
	assert.Contains(t, snap.RecentActivity[0].Summary, "document_search")
	assert.Contains(t, snap.RecentActivity[1].Summary, "execute_sql")
}

func TestRunProgressTracker_SubtaskTracking(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.RecordSubtaskStart("search docs")
	tracker.RecordSubtaskStart("query db")

	snap := tracker.Snapshot()
	assert.Equal(t, 2, snap.ActiveSubtasks)

	tracker.RecordSubtaskComplete("search docs")
	snap = tracker.Snapshot()
	assert.Equal(t, 1, snap.ActiveSubtasks)

	tracker.RecordSubtaskComplete("query db")
	snap = tracker.Snapshot()
	assert.Equal(t, 0, snap.ActiveSubtasks)
}

func TestRunProgressTracker_SubtaskCompleteNeverNegative(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.RecordSubtaskComplete("stray complete")
	snap := tracker.Snapshot()
	assert.Equal(t, 0, snap.ActiveSubtasks)
}

func TestRunProgressTracker_SetTurnIndex(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.SetTurnIndex(3)
	snap := tracker.Snapshot()
	assert.Equal(t, 3, snap.TurnIndex)
}

func TestRunProgressTracker_ActivityEviction(t *testing.T) {
	tracker := NewRunProgressTracker(3)
	tracker.RecordToolCall("tool-1")
	tracker.RecordToolCall("tool-2")
	tracker.RecordToolCall("tool-3")
	tracker.RecordToolCall("tool-4")

	snap := tracker.Snapshot()
	require.Len(t, snap.RecentActivity, 3)
	// Oldest (tool-1) should be evicted.
	assert.Contains(t, snap.RecentActivity[0].Summary, "tool-2")
	assert.Contains(t, snap.RecentActivity[1].Summary, "tool-3")
	assert.Contains(t, snap.RecentActivity[2].Summary, "tool-4")
}

func TestRunProgressTracker_DefaultMaxActivities(t *testing.T) {
	tracker := NewRunProgressTracker(0)
	assert.Equal(t, 10, tracker.maxActivities)
}

func TestRunProgressTracker_SnapshotIsCopy(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.RecordToolCall("tool-1")

	snap1 := tracker.Snapshot()
	tracker.RecordToolCall("tool-2")
	snap2 := tracker.Snapshot()

	assert.Len(t, snap1.RecentActivity, 1)
	assert.Len(t, snap2.RecentActivity, 2)
}

func TestRunProgressTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewRunProgressTracker(100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.RecordLLMCall(100, 0.01, "test")
			tracker.RecordToolCall("tool")
			tracker.RecordSubtaskStart("task")
			tracker.RecordSubtaskComplete("task")
			tracker.SetTurnIndex(1)
			_ = tracker.Snapshot()
		}()
	}

	wg.Wait()
	snap := tracker.Snapshot()
	assert.Equal(t, 5000, snap.TotalTokens) // 50 * 100
	assert.Equal(t, 50, snap.TotalToolCalls)
	assert.Equal(t, 0, snap.ActiveSubtasks) // all completed
}

func TestRunProgressTracker_RecordText(t *testing.T) {
	tracker := NewRunProgressTracker(10)
	tracker.RecordText("Generating response")
	snap := tracker.Snapshot()
	require.Len(t, snap.RecentActivity, 1)
	assert.Equal(t, ActivityText, snap.RecentActivity[0].Type)
	assert.Equal(t, "Generating response", snap.RecentActivity[0].Summary)
}

func TestRunProgressData_EventType(t *testing.T) {
	assert.Equal(t, RunEventType("run_progress"), EventRunProgress)
}
