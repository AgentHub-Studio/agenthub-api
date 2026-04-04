package agentic

import (
	"sync"
	"time"
)

// Context-window budget tracking constants.
//
// Inspired by Claude Code's tokenBudgetTracker.ts — monitors context window
// usage across turns and detects diminishing returns in the agentic loop.
// Complements BudgetTracker (budget.go) which tracks output token budgets.
const (
	// ContextCompletionThreshold is the fraction of the context window at which
	// the runner should consider stopping even if the LLM wants to continue.
	ContextCompletionThreshold = 0.9

	// ContextDiminishingThreshold is the minimum delta (in tokens) per turn that
	// counts as "meaningful progress". Below this, the turn is considered
	// diminishing returns.
	ContextDiminishingThreshold = 500

	// ContextMaxDiminishingTurns is how many consecutive diminishing turns
	// before the tracker recommends stopping.
	ContextMaxDiminishingTurns = 3
)

// ContinuationDecision is the result of a context budget check.
type ContinuationDecision string

const (
	// DecisionContinue means the runner should keep going.
	DecisionContinue ContinuationDecision = "continue"
	// DecisionStopBudget means the context window is nearly exhausted.
	DecisionStopBudget ContinuationDecision = "stop_budget"
	// DecisionStopDiminishing means output is not making meaningful progress.
	DecisionStopDiminishing ContinuationDecision = "stop_diminishing"
)

// ContextBudgetCheckResult holds the decision plus diagnostics.
type ContextBudgetCheckResult struct {
	// Decision is continue, stop_budget, or stop_diminishing.
	Decision ContinuationDecision `json:"decision"`
	// Reason is a human-readable explanation.
	Reason string `json:"reason,omitempty"`
	// UsagePercent is how much of the context window has been consumed (0-100).
	UsagePercent int `json:"usagePercent"`
	// ContinuationCount is how many turns have been executed.
	ContinuationCount int `json:"continuationCount"`
	// DiminishingCount is consecutive turns below the diminishing threshold.
	DiminishingCount int `json:"diminishingCount"`
}

// ContextBudgetTracker monitors context window token usage across turns
// and decides whether the agentic loop should continue.
//
// Unlike BudgetTracker (budget.go) which tracks output token budgets,
// this tracks the overall context window consumption.
type ContextBudgetTracker struct {
	mu sync.Mutex

	// contextWindow is the total context window size in tokens.
	contextWindow int
	// maxOutputTokens is the max output tokens per response.
	maxOutputTokens int

	// continuationCount tracks how many turns have been executed.
	continuationCount int
	// lastDeltaTokens tracks the output tokens of the most recent turn.
	lastDeltaTokens int
	// diminishingCount counts consecutive turns below ContextDiminishingThreshold.
	diminishingCount int
	// totalUsedTokens is the running total of tokens consumed.
	totalUsedTokens int

	// turnTimestamps records when each turn started (for rate tracking).
	turnTimestamps []time.Time
}

// NewContextBudgetTracker creates a tracker for the given context window and max output.
func NewContextBudgetTracker(contextWindow, maxOutputTokens int) *ContextBudgetTracker {
	return &ContextBudgetTracker{
		contextWindow:   contextWindow,
		maxOutputTokens: maxOutputTokens,
	}
}

// RecordTurn records a completed turn's token usage.
func (bt *ContextBudgetTracker) RecordTurn(outputTokens, totalTokensNow int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	bt.continuationCount++
	bt.lastDeltaTokens = outputTokens
	bt.totalUsedTokens = totalTokensNow
	bt.turnTimestamps = append(bt.turnTimestamps, time.Now())

	if outputTokens < ContextDiminishingThreshold {
		bt.diminishingCount++
	} else {
		bt.diminishingCount = 0
	}
}

// Check evaluates whether the loop should continue.
func (bt *ContextBudgetTracker) Check() ContextBudgetCheckResult {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	usagePct := 0
	if bt.contextWindow > 0 {
		usagePct = int(float64(bt.totalUsedTokens) / float64(bt.contextWindow) * 100)
		if usagePct > 100 {
			usagePct = 100
		}
	}

	result := ContextBudgetCheckResult{
		UsagePercent:      usagePct,
		ContinuationCount: bt.continuationCount,
		DiminishingCount:  bt.diminishingCount,
	}

	// Budget exhaustion check.
	if bt.contextWindow > 0 {
		ratio := float64(bt.totalUsedTokens) / float64(bt.contextWindow)
		if ratio >= ContextCompletionThreshold {
			result.Decision = DecisionStopBudget
			result.Reason = "token budget nearly exhausted"
			return result
		}
	}

	// Diminishing returns check.
	if bt.diminishingCount >= ContextMaxDiminishingTurns {
		result.Decision = DecisionStopDiminishing
		result.Reason = "consecutive turns with minimal output"
		return result
	}

	result.Decision = DecisionContinue
	return result
}

// ShouldContinue is a convenience method that returns true if Check().Decision == continue.
func (bt *ContextBudgetTracker) ShouldContinue() bool {
	return bt.Check().Decision == DecisionContinue
}

// Stats returns a snapshot of tracker state.
func (bt *ContextBudgetTracker) Stats() ContextBudgetTrackerStats {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	return ContextBudgetTrackerStats{
		ContextWindow:     bt.contextWindow,
		MaxOutputTokens:   bt.maxOutputTokens,
		TotalUsedTokens:   bt.totalUsedTokens,
		ContinuationCount: bt.continuationCount,
		LastDeltaTokens:   bt.lastDeltaTokens,
		DiminishingCount:  bt.diminishingCount,
	}
}

// ContextBudgetTrackerStats is a read-only snapshot of tracker state.
type ContextBudgetTrackerStats struct {
	ContextWindow     int `json:"contextWindow"`
	MaxOutputTokens   int `json:"maxOutputTokens"`
	TotalUsedTokens   int `json:"totalUsedTokens"`
	ContinuationCount int `json:"continuationCount"`
	LastDeltaTokens   int `json:"lastDeltaTokens"`
	DiminishingCount  int `json:"diminishingCount"`
}

// RemainingTokens returns how many tokens remain before hitting the context window.
func (bt *ContextBudgetTracker) RemainingTokens() int {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	remaining := bt.contextWindow - bt.totalUsedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset clears all tracking state.
func (bt *ContextBudgetTracker) Reset() {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	bt.continuationCount = 0
	bt.lastDeltaTokens = 0
	bt.diminishingCount = 0
	bt.totalUsedTokens = 0
	bt.turnTimestamps = nil
}
