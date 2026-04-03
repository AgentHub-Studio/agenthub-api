package agentic_test

import (
	"encoding/json"
	"testing"

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
