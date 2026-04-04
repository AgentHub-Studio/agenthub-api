package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestContextBudgetTracker_Constants(t *testing.T) {
	assert.Equal(t, 0.9, agentic.ContextCompletionThreshold)
	assert.Equal(t, 500, agentic.ContextDiminishingThreshold)
	assert.Equal(t, 3, agentic.ContextMaxDiminishingTurns)
}

// --- Decision types ---

func TestContinuationDecision_Values(t *testing.T) {
	assert.Equal(t, agentic.ContinuationDecision("continue"), agentic.DecisionContinue)
	assert.Equal(t, agentic.ContinuationDecision("stop_budget"), agentic.DecisionStopBudget)
	assert.Equal(t, agentic.ContinuationDecision("stop_diminishing"), agentic.DecisionStopDiminishing)
}

// --- New tracker ---

func TestNewContextBudgetTracker(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	require.NotNil(t, bt)
	assert.True(t, bt.ShouldContinue())
}

// --- RecordTurn + Check ---

func TestContextBudgetTracker_RecordTurn_Normal(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	bt.RecordTurn(2000, 50_000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionContinue, result.Decision)
	assert.Equal(t, 1, result.ContinuationCount)
	assert.Equal(t, 25, result.UsagePercent)
	assert.Equal(t, 0, result.DiminishingCount)
}

func TestContextBudgetTracker_StopBudget(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	bt.RecordTurn(5000, 180_000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionStopBudget, result.Decision)
	assert.Contains(t, result.Reason, "budget")
	assert.Equal(t, 90, result.UsagePercent)
}

func TestContextBudgetTracker_StopBudget_Over100(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	bt.RecordTurn(5000, 210_000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionStopBudget, result.Decision)
	assert.Equal(t, 100, result.UsagePercent)
}

func TestContextBudgetTracker_StopDiminishing(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)

	bt.RecordTurn(100, 20_000)
	bt.RecordTurn(50, 20_200)
	bt.RecordTurn(30, 20_300)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionStopDiminishing, result.Decision)
	assert.Contains(t, result.Reason, "minimal output")
	assert.Equal(t, 3, result.DiminishingCount)
}

func TestContextBudgetTracker_DiminishingResets(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)

	bt.RecordTurn(100, 10_000)
	bt.RecordTurn(50, 10_200)
	bt.RecordTurn(2000, 30_000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionContinue, result.Decision)
	assert.Equal(t, 0, result.DiminishingCount)
}

func TestContextBudgetTracker_BudgetBeforeDiminishing(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)

	bt.RecordTurn(100, 180_000)
	bt.RecordTurn(100, 185_000)
	bt.RecordTurn(100, 190_000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionStopBudget, result.Decision)
}

// --- ShouldContinue ---

func TestContextBudgetTracker_ShouldContinue(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	assert.True(t, bt.ShouldContinue())

	bt.RecordTurn(5000, 185_000)
	assert.False(t, bt.ShouldContinue())
}

// --- RemainingTokens ---

func TestContextBudgetTracker_RemainingTokens(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	assert.Equal(t, 200_000, bt.RemainingTokens())

	bt.RecordTurn(5000, 150_000)
	assert.Equal(t, 50_000, bt.RemainingTokens())
}

func TestContextBudgetTracker_RemainingTokens_Negative(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	bt.RecordTurn(5000, 210_000)
	assert.Equal(t, 0, bt.RemainingTokens())
}

// --- Stats ---

func TestContextBudgetTracker_Stats(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	bt.RecordTurn(3000, 50_000)
	bt.RecordTurn(100, 55_000)

	stats := bt.Stats()
	assert.Equal(t, 200_000, stats.ContextWindow)
	assert.Equal(t, 8192, stats.MaxOutputTokens)
	assert.Equal(t, 55_000, stats.TotalUsedTokens)
	assert.Equal(t, 2, stats.ContinuationCount)
	assert.Equal(t, 100, stats.LastDeltaTokens)
	assert.Equal(t, 1, stats.DiminishingCount)
}

// --- Reset ---

func TestContextBudgetTracker_Reset(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(200_000, 8192)
	bt.RecordTurn(5000, 100_000)
	bt.RecordTurn(100, 100_500)

	bt.Reset()

	stats := bt.Stats()
	assert.Equal(t, 0, stats.TotalUsedTokens)
	assert.Equal(t, 0, stats.ContinuationCount)
	assert.Equal(t, 0, stats.LastDeltaTokens)
	assert.Equal(t, 0, stats.DiminishingCount)
	assert.True(t, bt.ShouldContinue())
	assert.Equal(t, 200_000, bt.RemainingTokens())
}

// --- Zero context window ---

func TestContextBudgetTracker_ZeroContextWindow(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(0, 0)
	bt.RecordTurn(1000, 1000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionContinue, result.Decision)
	assert.Equal(t, 0, result.UsagePercent)
}

// --- Multiple turns progression ---

func TestContextBudgetTracker_MultiTurnProgression(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(100_000, 4096)

	turns := []struct {
		delta int
		total int
	}{
		{5000, 20_000},
		{4000, 35_000},
		{3000, 50_000},
		{2000, 65_000},
		{1500, 80_000},
	}

	for _, turn := range turns {
		bt.RecordTurn(turn.delta, turn.total)
		assert.True(t, bt.ShouldContinue(), "should continue at %d tokens", turn.total)
	}

	bt.RecordTurn(1000, 91_000)
	assert.False(t, bt.ShouldContinue())
}

// --- Edge: exactly at threshold ---

func TestContextBudgetTracker_ExactlyAtThreshold(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(100_000, 4096)
	bt.RecordTurn(1000, 90_000)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionStopBudget, result.Decision)
}

func TestContextBudgetTracker_JustBelowThreshold(t *testing.T) {
	bt := agentic.NewContextBudgetTracker(100_000, 4096)
	bt.RecordTurn(1000, 89_999)

	result := bt.Check()
	assert.Equal(t, agentic.DecisionContinue, result.Decision)
}
