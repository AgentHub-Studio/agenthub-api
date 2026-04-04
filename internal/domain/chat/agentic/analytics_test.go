package agentic_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- mock sink ---

type mockSink struct {
	runMetrics  []agentic.RunMetric
	toolMetrics []agentic.ToolMetric
}

func (s *mockSink) InsertRunMetrics(metrics []agentic.RunMetric) error {
	s.runMetrics = append(s.runMetrics, metrics...)
	return nil
}

func (s *mockSink) InsertToolMetrics(metrics []agentic.ToolMetric) error {
	s.toolMetrics = append(s.toolMetrics, metrics...)
	return nil
}

func newCollector(sink *mockSink) *agentic.MetricsCollector {
	return agentic.NewMetricsCollector(
		sink, "my-tenant", uuid.New(), uuid.New(), "run-1", "anthropic", "claude-sonnet-4-20250514",
	)
}

func TestMetricsCollector_TurnComplete(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 0,
		TokenUsage: agentic.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			CostUSD:          0.001,
		},
	}))

	rm := c.RunMetrics()
	assert.Equal(t, 1, rm.TotalTurns)
	assert.Equal(t, 150, rm.TotalTokens)
	assert.InDelta(t, 0.001, rm.CostUSD, 0.0001)
}

func TestMetricsCollector_MultipleTurns(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	for i := 0; i < 3; i++ {
		c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
			TurnIndex: i,
			TokenUsage: agentic.TokenUsage{
				PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
				CostUSD: 0.001,
			},
		}))
	}

	rm := c.RunMetrics()
	assert.Equal(t, 3, rm.TotalTurns)
	assert.Equal(t, 450, rm.TotalTokens)
}

func TestMetricsCollector_RunComplete(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns: 5, TotalTokens: 3200, TotalCost: 0.05,
	}))

	rm := c.RunMetrics()
	assert.Equal(t, 5, rm.TotalTurns)
	assert.Equal(t, 3200, rm.TotalTokens)
	assert.Equal(t, "stop", rm.FinishReason)
}

func TestMetricsCollector_ToolExecution(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	// Tool call start.
	c.Collect(agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc1", Name: "document_search", Input: json.RawMessage(`{"query":"test"}`),
	}))

	// Tool progress → completed.
	c.Collect(agentic.NewRunEvent(agentic.EventToolProgress, agentic.ToolProgressData{
		ID: "tc1", Name: "document_search", State: agentic.ToolStateCompleted,
	}))

	// Tool result.
	c.Collect(agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{
		ID: "tc1", Name: "document_search", DurationMs: 150,
	}))

	tools := c.ToolMetrics()
	require.Len(t, tools, 1)
	assert.Equal(t, "document_search", tools[0].ToolName)
	assert.Equal(t, int64(150), tools[0].DurationMs)
	assert.Equal(t, "completed", tools[0].State)
}

func TestMetricsCollector_ToolDenied(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventToolDenied, agentic.ToolDeniedData{
		ID: "tc2", Name: "execute_sql", Reason: "not allowed", DenialCount: 1,
	}))

	tools := c.ToolMetrics()
	require.Len(t, tools, 1)
	assert.Equal(t, "execute_sql", tools[0].ToolName)
	assert.True(t, tools[0].WasDenied)
	assert.Equal(t, "denied", tools[0].State)
}

func TestMetricsCollector_Error(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventError, agentic.ErrorData{
		Message: "budget exceeded", Code: "budget_exceeded",
	}))

	rm := c.RunMetrics()
	require.NotNil(t, rm.Error)
	assert.Equal(t, "budget exceeded", *rm.Error)
	assert.Equal(t, "error", rm.FinishReason)
}

func TestMetricsCollector_ErrorClassTracking(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	// Send a rate-limit error.
	c.Collect(agentic.NewRunEvent(agentic.EventError, agentic.ErrorData{
		Message: "status 429: rate limited", Code: "rate_limit",
	}))

	rm := c.RunMetrics()
	assert.Contains(t, rm.ErrorClasses, "rate_limit")

	// Send a connection error.
	c.Collect(agentic.NewRunEvent(agentic.EventError, agentic.ErrorData{
		Message: "ECONNRESET", Code: "connection_error",
	}))

	rm = c.RunMetrics()
	assert.Contains(t, rm.ErrorClasses, "stale_connection")
	assert.Len(t, rm.ErrorClasses, 2) // rate_limit + stale_connection
}

func TestMetricsCollector_Flush(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns: 2, TotalTokens: 1000, TotalCost: 0.01,
	}))

	c.Collect(agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc1", Name: "search",
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventToolProgress, agentic.ToolProgressData{
		ID: "tc1", Name: "search", State: agentic.ToolStateCompleted,
	}))

	err := c.Flush()
	require.NoError(t, err)
	assert.Len(t, sink.runMetrics, 1)
	assert.Len(t, sink.toolMetrics, 1)
	assert.Equal(t, "my-tenant", sink.runMetrics[0].TenantID)
	assert.Equal(t, "anthropic", sink.runMetrics[0].Provider)
}

func TestMetricsCollector_CollectFromChannel(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	ch := make(chan agentic.RunEvent, 10)
	ch <- agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 0, TokenUsage: agentic.TokenUsage{TotalTokens: 100},
	})
	ch <- agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns: 1, TotalTokens: 100,
	})
	close(ch)

	c.CollectFromChannel(ch)

	assert.Len(t, sink.runMetrics, 1)
	assert.Equal(t, 1, sink.runMetrics[0].TotalTurns)
}

func TestMetricsCollector_FlushWithNoTools(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns: 1, TotalTokens: 500,
	}))

	err := c.Flush()
	require.NoError(t, err)
	assert.Len(t, sink.runMetrics, 1)
	assert.Empty(t, sink.toolMetrics) // no tool metrics expected
}

func TestMetricsCollector_ToolAborted(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc1", Name: "slow_tool",
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventToolProgress, agentic.ToolProgressData{
		ID: "tc1", Name: "slow_tool", State: agentic.ToolStateAborted,
	}))

	tools := c.ToolMetrics()
	require.Len(t, tools, 1)
	assert.Equal(t, "aborted", tools[0].State)
}

func TestMetricsCollector_ToolErrorClassification(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	errMsg := "connection timeout while executing tool"

	c.Collect(agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc1", Name: "http_call",
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventToolProgress, agentic.ToolProgressData{
		ID: "tc1", Name: "http_call", State: agentic.ToolStateCompleted,
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{
		ID: "tc1", Name: "http_call", DurationMs: 5000, Error: &errMsg,
	}))

	tools := c.ToolMetrics()
	require.Len(t, tools, 1)
	assert.Equal(t, agentic.ToolErrorCategory("timeout"), tools[0].ErrorCategory)
}

func TestMetricsCollector_ToolNoErrorClassification(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	c.Collect(agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc1", Name: "search",
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventToolProgress, agentic.ToolProgressData{
		ID: "tc1", Name: "search", State: agentic.ToolStateCompleted,
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{
		ID: "tc1", Name: "search", DurationMs: 100,
	}))

	tools := c.ToolMetrics()
	require.Len(t, tools, 1)
	assert.Empty(t, tools[0].ErrorCategory) // no error → no category
}

func TestMetricsCollector_PerModelUsage_SingleModel(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	// Two turns with the same model — no ModelUsage in output.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex:  0,
		TokenUsage: agentic.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CostUSD: 0.001},
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex:  1,
		TokenUsage: agentic.TokenUsage{PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280, CostUSD: 0.002},
	}))

	err := c.Flush()
	require.NoError(t, err)
	require.Len(t, sink.runMetrics, 1)
	// Single model — ModelUsage should be nil (only populated for multi-model runs).
	assert.Nil(t, sink.runMetrics[0].ModelUsage)
}

func TestMetricsCollector_PerModelUsage_Fallback(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	// Turn 0: primary model.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex:  0,
		TokenUsage: agentic.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CostUSD: 0.001},
		Model:      "claude-sonnet-4-20250514",
	}))

	// Turn 1: fallback model (different model name).
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex:  1,
		TokenUsage: agentic.TokenUsage{PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280, CostUSD: 0.005},
		Model:      "claude-haiku-4-20250414",
	}))

	// Turn 2: back to primary.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex:  2,
		TokenUsage: agentic.TokenUsage{PromptTokens: 150, CompletionTokens: 60, TotalTokens: 210, CostUSD: 0.002},
		Model:      "claude-sonnet-4-20250514",
	}))

	err := c.Flush()
	require.NoError(t, err)
	require.Len(t, sink.runMetrics, 1)

	rm := sink.runMetrics[0]

	// Multi-model run → ModelUsage populated.
	require.NotNil(t, rm.ModelUsage)
	assert.Len(t, rm.ModelUsage, 2)

	// Primary model: 2 turns.
	primary := rm.ModelUsage["claude-sonnet-4-20250514"]
	require.NotNil(t, primary)
	assert.Equal(t, 2, primary.Turns)
	assert.Equal(t, 250, primary.PromptTokens)  // 100 + 150
	assert.Equal(t, 110, primary.CompletionTokens) // 50 + 60
	assert.InDelta(t, 0.003, primary.CostUSD, 0.0001)

	// Fallback model: 1 turn.
	fallback := rm.ModelUsage["claude-haiku-4-20250414"]
	require.NotNil(t, fallback)
	assert.Equal(t, 1, fallback.Turns)
	assert.Equal(t, 200, fallback.PromptTokens)
	assert.Equal(t, 80, fallback.CompletionTokens)
	assert.InDelta(t, 0.005, fallback.CostUSD, 0.0001)

	// Aggregate totals still correct.
	assert.Equal(t, 3, rm.TotalTurns)
	assert.Equal(t, 640, rm.TotalTokens) // 150 + 280 + 210
	assert.InDelta(t, 0.008, rm.CostUSD, 0.0001)
}

// --- CacheBreakDetector tests ---

func TestCacheBreakDetector_NoBreakOnFirstCall(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 5000, CacheCreationTokens: 1000,
	}
	brk := d.CheckForBreak(snap, 0)
	assert.Nil(t, brk) // no baseline to compare
}

func TestCacheBreakDetector_NoBreakOnStableCache(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	// Second call with similar cache reads — no break.
	snap.CacheReadTokens = 9800 // only 2% drop
	brk := d.CheckForBreak(snap, 30*time.Second)
	assert.Nil(t, brk)
}

func TestCacheBreakDetector_DetectsSignificantDrop(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	// Big drop: 10000 → 2000 (80% drop, >2000 absolute).
	snap.CacheReadTokens = 2000
	snap.CacheCreationTokens = 8000
	brk := d.CheckForBreak(snap, 30*time.Second)
	require.NotNil(t, brk)
	assert.Equal(t, 10000, brk.PrevCacheReadTokens)
	assert.Equal(t, 2000, brk.CacheReadTokens)
	assert.Equal(t, 8000, brk.TokenDrop)
	assert.InDelta(t, 80.0, brk.DropPercent, 0.1)
}

func TestCacheBreakDetector_SmallAbsoluteDropIgnored(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 3000,
	}
	d.CheckForBreak(snap, 0)

	// 50% drop but only 1500 tokens — below minimum threshold.
	snap.CacheReadTokens = 1500
	brk := d.CheckForBreak(snap, 30*time.Second)
	assert.Nil(t, brk)
}

func TestCacheBreakDetector_ModelChangeExplanation(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	// Model changed + cache dropped.
	snap.Model = "claude-haiku-4-20250414"
	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 30*time.Second)
	require.NotNil(t, brk)
	assert.Contains(t, brk.Reason, "model changed")
}

func TestCacheBreakDetector_SystemPromptChangeExplanation(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		SystemPromptHash: 12345, CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	snap.SystemPromptHash = 99999
	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 30*time.Second)
	require.NotNil(t, brk)
	assert.Contains(t, brk.Reason, "system prompt changed")
}

func TestCacheBreakDetector_ToolCountChangeExplanation(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		ToolCount: 5, CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	snap.ToolCount = 8
	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 30*time.Second)
	require.NotNil(t, brk)
	assert.Contains(t, brk.Reason, "tool count changed")
}

func TestCacheBreakDetector_TTLExpiry5Min(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	// No client-side changes, but 7-minute gap.
	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 7*time.Minute)
	require.NotNil(t, brk)
	assert.Contains(t, brk.Reason, "5min TTL expiry")
}

func TestCacheBreakDetector_TTLExpiry1Hour(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 2*time.Hour)
	require.NotNil(t, brk)
	assert.Contains(t, brk.Reason, "1h TTL expiry")
}

func TestCacheBreakDetector_ServerSideBreak(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	// No changes, short gap → likely server-side.
	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 2*time.Minute)
	require.NotNil(t, brk)
	assert.Contains(t, brk.Reason, "server-side")
}

func TestCacheBreakDetector_CompactionResetsBaseline(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	d.NotifyCompaction("main_loop")

	// Next call has lower tokens — should NOT trigger break.
	snap.CacheReadTokens = 1000
	brk := d.CheckForBreak(snap, 30*time.Second)
	assert.Nil(t, brk)
}

func TestCacheBreakDetector_IndependentSources(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	mainSnap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	subSnap := agentic.CacheBreakSnapshot{
		Source: "subtask", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 5000,
	}

	d.CheckForBreak(mainSnap, 0)
	d.CheckForBreak(subSnap, 0)

	// Drop on subtask only.
	subSnap.CacheReadTokens = 0
	brk := d.CheckForBreak(subSnap, 30*time.Second)
	require.NotNil(t, brk)
	assert.Equal(t, "subtask", brk.Source)

	// Main loop is still stable.
	mainSnap.CacheReadTokens = 9900
	brk = d.CheckForBreak(mainSnap, 30*time.Second)
	assert.Nil(t, brk)
}

func TestCacheBreakDetector_Reset(t *testing.T) {
	d := agentic.NewCacheBreakDetector()
	snap := agentic.CacheBreakSnapshot{
		Source: "main_loop", Model: "claude-sonnet-4-20250514",
		CacheReadTokens: 10000,
	}
	d.CheckForBreak(snap, 0)

	d.Reset()

	// After reset, this is treated as first call again — no break.
	snap.CacheReadTokens = 0
	brk := d.CheckForBreak(snap, 0)
	assert.Nil(t, brk)
}

func TestDjb2Hash(t *testing.T) {
	h1 := agentic.Djb2Hash("hello world")
	h2 := agentic.Djb2Hash("hello world")
	h3 := agentic.Djb2Hash("different string")
	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.NotZero(t, h1)
}

func TestMetricsCollector_CacheBreakIntegration(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	// Turn 0: establish baseline with high cache reads.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 0,
		TokenUsage: agentic.TokenUsage{
			PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
			CacheReadTokens: 10000,
		},
		Source: agentic.SourceMainLoop,
	}))

	// Turn 1: cache drops significantly.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 1,
		TokenUsage: agentic.TokenUsage{
			PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
			CacheReadTokens: 1000, CacheCreationTokens: 9000,
		},
		Source: agentic.SourceMainLoop,
	}))

	rm := c.RunMetrics()
	require.Len(t, rm.CacheBreaks, 1)
	assert.Equal(t, 10000, rm.CacheBreaks[0].PrevCacheReadTokens)
	assert.Equal(t, 1000, rm.CacheBreaks[0].CacheReadTokens)
}

func TestMetricsCollector_QuerySourceTracking(t *testing.T) {
	sink := &mockSink{}
	c := newCollector(sink)

	// Turns with different sources.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 0, TokenUsage: agentic.TokenUsage{TotalTokens: 100},
		Source: agentic.SourceMainLoop,
	}))
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 1, TokenUsage: agentic.TokenUsage{TotalTokens: 100},
		Source: agentic.SourceSubtask,
	}))
	// Duplicate source — should not appear twice.
	c.Collect(agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 2, TokenUsage: agentic.TokenUsage{TotalTokens: 100},
		Source: agentic.SourceMainLoop,
	}))

	err := c.Flush()
	require.NoError(t, err)
	require.Len(t, sink.runMetrics, 1)

	rm := sink.runMetrics[0]
	assert.Len(t, rm.QuerySources, 2)
	assert.Contains(t, rm.QuerySources, "main_loop")
	assert.Contains(t, rm.QuerySources, "subtask")
}
