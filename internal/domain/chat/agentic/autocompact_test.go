package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestAutoCompactConstants(t *testing.T) {
	assert.Equal(t, 20_000, agentic.MaxOutputTokensForSummary)
	assert.Equal(t, 13_000, agentic.AutoCompactBufferTokens)
	assert.Equal(t, 20_000, agentic.WarningThresholdBufferTokens)
	assert.Equal(t, 20_000, agentic.ErrorThresholdBufferTokens)
	assert.Equal(t, 3_000, agentic.ManualCompactBufferTokens)
	assert.Equal(t, 3, agentic.MaxConsecutiveAutoCompactFailures)
}

// --- GetEffectiveContextWindowSize ---

func TestGetEffectiveContextWindowSize_Normal(t *testing.T) {
	// 200K window, 8K max output → reserve min(8K, 20K) = 8K
	effective := agentic.GetEffectiveContextWindowSize(200_000, 8_000)
	assert.Equal(t, 192_000, effective)
}

func TestGetEffectiveContextWindowSize_LargeOutput(t *testing.T) {
	// 200K window, 50K max output → reserve min(50K, 20K) = 20K
	effective := agentic.GetEffectiveContextWindowSize(200_000, 50_000)
	assert.Equal(t, 180_000, effective)
}

func TestGetEffectiveContextWindowSize_Small(t *testing.T) {
	effective := agentic.GetEffectiveContextWindowSize(10_000, 8_000)
	assert.Equal(t, 2_000, effective)
}

func TestGetEffectiveContextWindowSize_Zero(t *testing.T) {
	effective := agentic.GetEffectiveContextWindowSize(0, 0)
	assert.Equal(t, 0, effective)
}

// --- GetAutoCompactThreshold ---

func TestGetAutoCompactThreshold(t *testing.T) {
	threshold := agentic.GetAutoCompactThreshold(180_000)
	assert.Equal(t, 167_000, threshold) // 180K - 13K
}

func TestGetAutoCompactThreshold_Small(t *testing.T) {
	threshold := agentic.GetAutoCompactThreshold(5_000)
	assert.Equal(t, 0, threshold) // 5K - 13K = negative → 0
}

// --- CalculateTokenWarningState ---

func TestCalculateTokenWarningState_LowUsage(t *testing.T) {
	state := agentic.CalculateTokenWarningState(10_000, 180_000, true)
	assert.Greater(t, state.PercentLeft, 90)
	assert.False(t, state.IsAboveWarningThreshold)
	assert.False(t, state.IsAboveErrorThreshold)
	assert.False(t, state.IsAboveAutoCompactThreshold)
	assert.False(t, state.IsAtBlockingLimit)
}

func TestCalculateTokenWarningState_WarningLevel(t *testing.T) {
	// 180K effective, warning at 180K - 20K = 160K
	state := agentic.CalculateTokenWarningState(165_000, 180_000, true)
	assert.True(t, state.IsAboveWarningThreshold)
	// autocompact threshold = 180K - 13K = 167K; 165K < 167K
	assert.False(t, state.IsAboveAutoCompactThreshold)
}

func TestCalculateTokenWarningState_AutoCompactLevel(t *testing.T) {
	// 180K effective, autocompact at 180K - 13K = 167K
	state := agentic.CalculateTokenWarningState(170_000, 180_000, true)
	assert.True(t, state.IsAboveAutoCompactThreshold)
	assert.True(t, state.IsAboveWarningThreshold)
}

func TestCalculateTokenWarningState_ErrorLevel(t *testing.T) {
	// 180K effective, error at 180K - 20K = 160K
	state := agentic.CalculateTokenWarningState(165_000, 180_000, true)
	assert.True(t, state.IsAboveErrorThreshold)
}

func TestCalculateTokenWarningState_BlockingLimit(t *testing.T) {
	// 180K effective, blocking at 180K - 3K = 177K
	state := agentic.CalculateTokenWarningState(178_000, 180_000, true)
	assert.True(t, state.IsAtBlockingLimit)
	assert.Equal(t, 1, state.PercentLeft)
}

func TestCalculateTokenWarningState_AutoCompactDisabled(t *testing.T) {
	state := agentic.CalculateTokenWarningState(170_000, 180_000, false)
	assert.False(t, state.IsAboveAutoCompactThreshold, "should not trigger when disabled")
	assert.True(t, state.IsAboveWarningThreshold, "warning should still fire")
}

func TestCalculateTokenWarningState_OverLimit(t *testing.T) {
	state := agentic.CalculateTokenWarningState(200_000, 180_000, true)
	assert.Equal(t, 0, state.PercentLeft)
	assert.True(t, state.IsAtBlockingLimit)
}

// --- ShouldAutoCompact ---

func TestShouldAutoCompact_NilTracking(t *testing.T) {
	assert.False(t, agentic.ShouldAutoCompact(170_000, 180_000, nil))
}

func TestShouldAutoCompact_BelowThreshold(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{}
	assert.False(t, agentic.ShouldAutoCompact(10_000, 180_000, tracking))
}

func TestShouldAutoCompact_AboveThreshold(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{}
	// Threshold = 180K - 13K = 167K
	assert.True(t, agentic.ShouldAutoCompact(170_000, 180_000, tracking))
}

func TestShouldAutoCompact_CircuitBreakerOpen(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{
		ConsecutiveFailures: 3,
	}
	assert.False(t, agentic.ShouldAutoCompact(170_000, 180_000, tracking),
		"should not compact when circuit breaker is open")
}

func TestShouldAutoCompact_CircuitBreakerNotYetOpen(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{
		ConsecutiveFailures: 2,
	}
	assert.True(t, agentic.ShouldAutoCompact(170_000, 180_000, tracking))
}

// --- RecordAutoCompactSuccess ---

func TestRecordAutoCompactSuccess(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{
		ConsecutiveFailures: 2,
	}
	result := agentic.RecordAutoCompactSuccess(tracking)
	assert.True(t, result.WasCompacted)
	assert.Equal(t, 0, result.ConsecutiveFailures)
	assert.True(t, tracking.Compacted)
	assert.Equal(t, 0, tracking.ConsecutiveFailures)
}

// --- RecordAutoCompactFailure ---

func TestRecordAutoCompactFailure(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{}
	result := agentic.RecordAutoCompactFailure(tracking)
	assert.False(t, result.WasCompacted)
	assert.Equal(t, 1, result.ConsecutiveFailures)
	assert.Equal(t, 1, tracking.ConsecutiveFailures)
}

func TestRecordAutoCompactFailure_TripsCircuitBreaker(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{
		ConsecutiveFailures: 2,
	}
	result := agentic.RecordAutoCompactFailure(tracking)
	assert.Equal(t, 3, result.ConsecutiveFailures)
	assert.False(t, agentic.ShouldAutoCompact(170_000, 180_000, tracking),
		"circuit breaker should be open after 3 failures")
}

func TestRecordAutoCompactFailure_NilTracking(t *testing.T) {
	result := agentic.RecordAutoCompactFailure(nil)
	assert.False(t, result.WasCompacted)
	assert.Equal(t, 0, result.ConsecutiveFailures)
}

// --- Full lifecycle ---

func TestAutoCompact_Lifecycle(t *testing.T) {
	tracking := &agentic.AutoCompactTrackingState{}
	effectiveWindow := 180_000

	// Phase 1: Low usage — no compaction needed.
	assert.False(t, agentic.ShouldAutoCompact(50_000, effectiveWindow, tracking))

	// Phase 2: High usage — compaction triggers.
	assert.True(t, agentic.ShouldAutoCompact(170_000, effectiveWindow, tracking))

	// Phase 3: Simulate failure.
	agentic.RecordAutoCompactFailure(tracking)
	assert.Equal(t, 1, tracking.ConsecutiveFailures)
	assert.True(t, agentic.ShouldAutoCompact(170_000, effectiveWindow, tracking))

	// Phase 4: Second failure.
	agentic.RecordAutoCompactFailure(tracking)
	assert.True(t, agentic.ShouldAutoCompact(170_000, effectiveWindow, tracking))

	// Phase 5: Third failure — circuit breaker trips.
	agentic.RecordAutoCompactFailure(tracking)
	assert.False(t, agentic.ShouldAutoCompact(170_000, effectiveWindow, tracking))

	// Phase 6: Manual success resets circuit breaker.
	agentic.RecordAutoCompactSuccess(tracking)
	assert.True(t, agentic.ShouldAutoCompact(170_000, effectiveWindow, tracking))
}

// --- FormatTokenUsageSummary ---

func TestFormatTokenUsageSummary(t *testing.T) {
	result := agentic.FormatTokenUsageSummary(90_000, 180_000)
	assert.Equal(t, "90000/180000 tokens (50%)", result)
}

func TestFormatTokenUsageSummary_Zero(t *testing.T) {
	result := agentic.FormatTokenUsageSummary(0, 0)
	assert.Equal(t, "0/0 tokens (0%)", result)
}

func TestFormatTokenUsageSummary_Full(t *testing.T) {
	result := agentic.FormatTokenUsageSummary(180_000, 180_000)
	assert.Equal(t, "180000/180000 tokens (100%)", result)
}
