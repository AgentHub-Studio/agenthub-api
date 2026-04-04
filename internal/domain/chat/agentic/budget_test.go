package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestBudgetTracker_NoBudget(t *testing.T) {
	bt := &agentic.BudgetTracker{}
	decision := bt.CheckTokenBudget(0, 5000)
	assert.Equal(t, "stop", decision.Action)
}

func TestBudgetTracker_ContinuesBelowThreshold(t *testing.T) {
	bt := &agentic.BudgetTracker{}
	// 10k tokens produced out of 100k budget = 10% → continue.
	decision := bt.CheckTokenBudget(100000, 10000)
	assert.Equal(t, "continue", decision.Action)
	assert.Contains(t, decision.NudgeMessage, "10%")
	assert.Equal(t, 1, bt.ContinuationCount)
}

func TestBudgetTracker_StopsAtThreshold(t *testing.T) {
	bt := &agentic.BudgetTracker{}
	// 95k tokens out of 100k = 95% → above 90% threshold → stop.
	decision := bt.CheckTokenBudget(100000, 95000)
	assert.Equal(t, "stop", decision.Action)
}

func TestBudgetTracker_DiminishingReturns(t *testing.T) {
	bt := &agentic.BudgetTracker{}
	budget := 1000000 // 1M tokens

	// First 3 continuations with good progress.
	bt.CheckTokenBudget(budget, 50000)  // +50k
	bt.CheckTokenBudget(budget, 100000) // +50k
	bt.CheckTokenBudget(budget, 150000) // +50k

	// Now 2 checks with < 500 tokens each → diminishing returns.
	bt.CheckTokenBudget(budget, 150300) // +300
	decision := bt.CheckTokenBudget(budget, 150500) // +200
	assert.Equal(t, "stop", decision.Action)
	assert.True(t, decision.DiminishingReturns)
}

func TestBudgetTracker_NoFalsePositiveDiminishing(t *testing.T) {
	bt := &agentic.BudgetTracker{}
	budget := 1000000

	// Only 2 continuations — not enough for diminishing returns check.
	bt.CheckTokenBudget(budget, 100)
	decision := bt.CheckTokenBudget(budget, 200)
	assert.Equal(t, "continue", decision.Action)
	assert.False(t, decision.DiminishingReturns)
}

func TestBudgetTracker_NudgeMessageFormat(t *testing.T) {
	bt := &agentic.BudgetTracker{}
	decision := bt.CheckTokenBudget(10000, 2500) // 25%
	assert.Contains(t, decision.NudgeMessage, "25%")
	assert.Contains(t, decision.NudgeMessage, "2500")
	assert.Contains(t, decision.NudgeMessage, "10000")
	assert.Contains(t, decision.NudgeMessage, "Keep working")
}

// --- ParseTokenBudget tests ---

func TestParseTokenBudget_ShorthandStart(t *testing.T) {
	assert.Equal(t, 500000, agentic.ParseTokenBudget("+500k do this task"))
	assert.Equal(t, 2000000, agentic.ParseTokenBudget("+2m solve everything"))
	assert.Equal(t, 1500000, agentic.ParseTokenBudget("+1.5m complete the refactor"))
}

func TestParseTokenBudget_ShorthandEnd(t *testing.T) {
	assert.Equal(t, 500000, agentic.ParseTokenBudget("do this task +500k"))
	assert.Equal(t, 200000, agentic.ParseTokenBudget("refactor the codebase +200k."))
}

func TestParseTokenBudget_Verbose(t *testing.T) {
	assert.Equal(t, 500000, agentic.ParseTokenBudget("use 500k tokens to solve this"))
	assert.Equal(t, 2000000, agentic.ParseTokenBudget("spend 2M tokens on this problem"))
	assert.Equal(t, 100000, agentic.ParseTokenBudget("please use 100k token for this"))
}

func TestParseTokenBudget_CaseInsensitive(t *testing.T) {
	assert.Equal(t, 500000, agentic.ParseTokenBudget("+500K"))
	assert.Equal(t, 2000000, agentic.ParseTokenBudget("USE 2M TOKENS"))
	assert.Equal(t, 1000000000, agentic.ParseTokenBudget("+1B"))
}

func TestParseTokenBudget_NoMatch(t *testing.T) {
	assert.Equal(t, 0, agentic.ParseTokenBudget("just do the task"))
	assert.Equal(t, 0, agentic.ParseTokenBudget("add 500 items"))
	assert.Equal(t, 0, agentic.ParseTokenBudget(""))
	// Mid-sentence shorthand should NOT match (avoid false positives).
	assert.Equal(t, 0, agentic.ParseTokenBudget("there are +500k users in the DB"))
}

func TestParseTokenBudget_Fractional(t *testing.T) {
	assert.Equal(t, 500, agentic.ParseTokenBudget("+0.5k"))
	assert.Equal(t, 2500000, agentic.ParseTokenBudget("use 2.5m tokens"))
}
