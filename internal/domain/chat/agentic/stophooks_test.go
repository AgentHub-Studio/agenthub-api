package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- StopHookResult ---

func TestStopHookResult_ZeroValue(t *testing.T) {
	var result agentic.StopHookResult
	assert.False(t, result.PreventContinuation)
	assert.Empty(t, result.BlockingErrors)
	assert.Empty(t, result.HookErrors)
	assert.Equal(t, 0, result.HookCount)
}

// --- StopHooksOrchestrator ---

func TestStopHooksOrchestrator_NilDeps(t *testing.T) {
	ch := make(chan agentic.RunEvent, 10)
	orch := agentic.NewStopHooksOrchestrator(nil, nil, nil, ch)

	result := orch.HandleStopHooks(nil, agentic.StopHookContext{
		QuerySource: agentic.SourceMainLoop,
	})

	assert.Equal(t, 0, result.HookCount)
	assert.False(t, result.PreventContinuation)
	assert.Empty(t, result.BlockingErrors)
}

func TestStopHooksOrchestrator_SavesCacheSafeParams(t *testing.T) {
	ch := make(chan agentic.RunEvent, 10)
	snap := &agentic.CacheSafeParamsSnapshot{}
	orch := agentic.NewStopHooksOrchestrator(nil, nil, snap, ch)

	params := agentic.NewCacheSafeParams("prompt", nil, "anthropic", "model", true)

	orch.HandleStopHooks(nil, agentic.StopHookContext{
		QuerySource:     agentic.SourceMainLoop,
		CurrentDepth:    0, // Root agent.
		CacheSafeParams: params,
	})

	assert.NotNil(t, snap.Get())
	assert.True(t, params.Matches(snap.Get()))
}

func TestStopHooksOrchestrator_DoesNotSaveForSubAgents(t *testing.T) {
	ch := make(chan agentic.RunEvent, 10)
	snap := &agentic.CacheSafeParamsSnapshot{}
	orch := agentic.NewStopHooksOrchestrator(nil, nil, snap, ch)

	params := agentic.NewCacheSafeParams("prompt", nil, "anthropic", "model", true)

	orch.HandleStopHooks(nil, agentic.StopHookContext{
		QuerySource:     agentic.SourceSubtask,
		CurrentDepth:    1, // Sub-agent — must NOT overwrite parent snapshot.
		CacheSafeParams: params,
	})

	assert.Nil(t, snap.Get(), "sub-agents should not overwrite parent's cache-safe params")
}

func TestStopHooksOrchestrator_SkipsBackgroundTasksForSubAgents(t *testing.T) {
	ch := make(chan agentic.RunEvent, 10)
	// Create a memory extractor that would extract if conditions are met.
	cfg := agentic.SessionMemoryConfig{
		MinimumMessageTokensToInit: 100,
		MinimumTokensBetweenUpdate: 50,
		ToolCallsBetweenUpdates:    1,
	}
	extractor := agentic.NewSessionMemoryExtractor(
		agentic.NewForkedAgentRunner(nil, agentic.DefaultRunConfig(), nil),
		cfg,
	)
	extractor.TrackToolCall()

	orch := agentic.NewStopHooksOrchestrator(nil, extractor, nil, ch)

	result := orch.HandleStopHooks(nil, agentic.StopHookContext{
		QuerySource:  agentic.SourceSubtask,
		CurrentDepth: 1, // Sub-agent.
		CurrentTokens: 10000,
	})

	// Sub-agents should not fire background tasks.
	assert.Equal(t, 0, result.HookCount)
	// Stats should show no extraction was triggered.
	stats := extractor.Stats()
	assert.Equal(t, 0, stats.TotalExtractions)
}

// --- BackgroundTaskTracker ---

func TestBackgroundTaskTracker_Lifecycle(t *testing.T) {
	tracker := agentic.NewBackgroundTaskTracker()

	assert.Equal(t, 0, tracker.Active())

	id1 := tracker.Start("memory_extraction")
	id2 := tracker.Start("auto_dream")

	assert.Equal(t, 2, tracker.Active())
	tasks := tracker.List()
	assert.Len(t, tasks, 2)

	tracker.Complete(id1)
	assert.Equal(t, 1, tracker.Active())

	tracker.Complete(id2)
	assert.Equal(t, 0, tracker.Active())
}

func TestBackgroundTaskTracker_CompleteUnknown(t *testing.T) {
	tracker := agentic.NewBackgroundTaskTracker()
	// Should not panic.
	tracker.Complete("nonexistent")
	assert.Equal(t, 0, tracker.Active())
}

// --- StopHookSummaryData ---

func TestStopHookSummaryData_Fields(t *testing.T) {
	data := agentic.StopHookSummaryData{
		TurnIndex:           3,
		HookCount:           2,
		ErrorCount:          1,
		PreventContinuation: false,
		Summary:             "2 hook(s) executed, 1 error(s)",
		DurationMs:          150,
	}

	assert.Equal(t, 3, data.TurnIndex)
	assert.Equal(t, 2, data.HookCount)
	assert.Equal(t, 1, data.ErrorCount)
	assert.Equal(t, int64(150), data.DurationMs)
}
