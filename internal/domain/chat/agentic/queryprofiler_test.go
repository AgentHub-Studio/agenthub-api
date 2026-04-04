package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewQueryProfile ---

func TestNewQueryProfile(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	require.NotNil(t, qp)
	assert.Empty(t, qp.Checkpoints())
}

// --- Checkpoint ---

func TestQueryProfile_Checkpoint(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	qp.Checkpoint("step1")
	qp.Checkpoint("step2")

	cps := qp.Checkpoints()
	require.Len(t, cps, 2)
	assert.Equal(t, "step1", cps[0].Name)
	assert.Equal(t, "step2", cps[1].Name)
}

func TestQueryProfile_Checkpoint_DurationFromStart(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	time.Sleep(10 * time.Millisecond)
	qp.Checkpoint("after_10ms")

	cps := qp.Checkpoints()
	require.Len(t, cps, 1)
	assert.GreaterOrEqual(t, cps[0].DurationFromStart.Milliseconds(), int64(5))
}

func TestQueryProfile_Checkpoint_DurationFromPrev(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	qp.Checkpoint("first")
	time.Sleep(10 * time.Millisecond)
	qp.Checkpoint("second")

	cps := qp.Checkpoints()
	require.Len(t, cps, 2)
	assert.GreaterOrEqual(t, cps[1].DurationFromPrev.Milliseconds(), int64(5))
}

// --- TimeToCheckpoint ---

func TestQueryProfile_TimeToCheckpoint(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	time.Sleep(10 * time.Millisecond)
	qp.Checkpoint("target")

	d := qp.TimeToCheckpoint("target")
	assert.GreaterOrEqual(t, d.Milliseconds(), int64(5))
}

func TestQueryProfile_TimeToCheckpoint_NotFound(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	assert.Equal(t, time.Duration(0), qp.TimeToCheckpoint("nonexistent"))
}

// --- End / TotalDuration ---

func TestQueryProfile_End(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	time.Sleep(10 * time.Millisecond)
	qp.End()

	d := qp.TotalDuration()
	assert.GreaterOrEqual(t, d.Milliseconds(), int64(5))

	// Duration should be stable after End.
	time.Sleep(10 * time.Millisecond)
	d2 := qp.TotalDuration()
	assert.Equal(t, d, d2)
}

func TestQueryProfile_TotalDuration_NotEnded(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	time.Sleep(10 * time.Millisecond)
	d := qp.TotalDuration()
	assert.Greater(t, d, time.Duration(0))
}

// --- SetMeta ---

func TestQueryProfile_SetMeta(t *testing.T) {
	qp := agentic.NewQueryProfile("test-1")
	qp.SetMeta("model", "claude-opus")
	qp.SetMeta("tokens", 1500)

	report := qp.Report()
	assert.Contains(t, report, "model: claude-opus")
	assert.Contains(t, report, "tokens: 1500")
}

// --- Report ---

func TestQueryProfile_Report_Basic(t *testing.T) {
	qp := agentic.NewQueryProfile("query-42")
	qp.Checkpoint("context_loaded")
	qp.Checkpoint("tools_built")
	qp.Checkpoint("first_token")
	qp.End()

	report := qp.Report()
	assert.Contains(t, report, "Query Profile: query-42")
	assert.Contains(t, report, "Phase breakdown")
	assert.Contains(t, report, "context_loaded")
	assert.Contains(t, report, "tools_built")
	assert.Contains(t, report, "first_token")
	assert.Contains(t, report, "Total:")
}

func TestQueryProfile_Report_Empty(t *testing.T) {
	qp := agentic.NewQueryProfile("empty")
	qp.End()
	report := qp.Report()
	assert.Contains(t, report, "Query Profile: empty")
	assert.Contains(t, report, "Total:")
}

func TestQueryProfile_Report_WithPercentage(t *testing.T) {
	qp := agentic.NewQueryProfile("pct")
	time.Sleep(20 * time.Millisecond)
	qp.Checkpoint("slow_phase")
	qp.End()

	report := qp.Report()
	assert.Contains(t, report, "%")
}

// --- Well-known checkpoint constants ---

func TestQueryProfile_WellKnownCheckpoints(t *testing.T) {
	assert.Equal(t, "context_loaded", agentic.CPContextLoaded)
	assert.Equal(t, "tools_built", agentic.CPToolsBuilt)
	assert.Equal(t, "prompt_built", agentic.CPPromptBuilt)
	assert.Equal(t, "messages_normalized", agentic.CPMessagesNormalized)
	assert.Equal(t, "compacted", agentic.CPCompacted)
	assert.Equal(t, "request_sent", agentic.CPRequestSent)
	assert.Equal(t, "first_token", agentic.CPFirstToken)
	assert.Equal(t, "tool_calls_received", agentic.CPToolCallsReceived)
	assert.Equal(t, "tools_executed", agentic.CPToolsExecuted)
	assert.Equal(t, "response_complete", agentic.CPResponseComplete)
}

// --- Checkpoints returns copy ---

func TestQueryProfile_Checkpoints_ReturnsCopy(t *testing.T) {
	qp := agentic.NewQueryProfile("test")
	qp.Checkpoint("a")
	cps := qp.Checkpoints()
	cps[0].Name = "modified"

	original := qp.Checkpoints()
	assert.Equal(t, "a", original[0].Name, "should not modify internal state")
}

// --- Typical pipeline flow ---

func TestQueryProfile_TypicalPipeline(t *testing.T) {
	qp := agentic.NewQueryProfile("run-123")

	qp.Checkpoint(agentic.CPContextLoaded)
	qp.Checkpoint(agentic.CPToolsBuilt)
	qp.Checkpoint(agentic.CPPromptBuilt)
	qp.Checkpoint(agentic.CPMessagesNormalized)
	qp.Checkpoint(agentic.CPRequestSent)

	time.Sleep(5 * time.Millisecond) // simulate network
	qp.Checkpoint(agentic.CPFirstToken)
	qp.SetMeta("model", "claude-sonnet")
	qp.SetMeta("inputTokens", 5000)

	qp.Checkpoint(agentic.CPResponseComplete)
	qp.End()

	cps := qp.Checkpoints()
	assert.Len(t, cps, 7)

	ttft := qp.TimeToCheckpoint(agentic.CPFirstToken)
	assert.Greater(t, ttft, time.Duration(0))

	report := qp.Report()
	assert.Contains(t, report, "claude-sonnet")
}
