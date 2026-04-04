package agentic

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// BudgetTracker tracks output token production to detect when the LLM
// should continue working vs stop. Inspired by Claude Code's tokenBudget.ts.
//
// When a token budget is set (e.g. user says "use 500k tokens"), the runner
// continues feeding nudge messages to the LLM until the budget is reached
// or diminishing returns are detected.
type BudgetTracker struct {
	// ContinuationCount is the number of times the LLM was nudged to continue.
	ContinuationCount int
	// LastDeltaTokens is the output token delta from the previous check.
	LastDeltaTokens int
	// LastGlobalTurnTokens is the cumulative output tokens at the previous check.
	LastGlobalTurnTokens int
}

// BudgetDecision is the outcome of a budget check.
type BudgetDecision struct {
	// Action is "continue" or "stop".
	Action string
	// NudgeMessage is the message to inject when action is "continue".
	NudgeMessage string
	// DiminishingReturns is true when the stop was triggered by low output.
	DiminishingReturns bool
}

const (
	// budgetCompletionThreshold is the fraction of the budget that triggers a stop.
	budgetCompletionThreshold = 0.90
	// budgetDiminishingThreshold is the minimum token delta to avoid diminishing returns.
	budgetDiminishingThreshold = 500
	// budgetMinContinuationsForDiminishing is how many continuations before checking.
	budgetMinContinuationsForDiminishing = 3
)

// CheckTokenBudget evaluates whether the runner should continue producing output.
// budget is the target output token count (0 = no budget, always stop).
// globalTurnTokens is the cumulative output tokens produced in this run.
func (bt *BudgetTracker) CheckTokenBudget(budget int, globalTurnTokens int) BudgetDecision {
	if budget <= 0 {
		return BudgetDecision{Action: "stop"}
	}

	pct := (globalTurnTokens * 100) / budget
	deltaSinceLastCheck := globalTurnTokens - bt.LastGlobalTurnTokens

	// Diminishing returns: after N continuations, if output production
	// drops below threshold for 2 consecutive checks, stop.
	isDiminishing := bt.ContinuationCount >= budgetMinContinuationsForDiminishing &&
		deltaSinceLastCheck < budgetDiminishingThreshold &&
		bt.LastDeltaTokens < budgetDiminishingThreshold

	// Continue if not diminishing and below completion threshold.
	if !isDiminishing && globalTurnTokens < int(float64(budget)*budgetCompletionThreshold) {
		bt.ContinuationCount++
		bt.LastDeltaTokens = deltaSinceLastCheck
		bt.LastGlobalTurnTokens = globalTurnTokens
		return BudgetDecision{
			Action:       "continue",
			NudgeMessage: formatBudgetNudge(pct, globalTurnTokens, budget),
		}
	}

	return BudgetDecision{
		Action:             "stop",
		DiminishingReturns: isDiminishing,
	}
}

// formatBudgetNudge creates the nudge message injected to keep the LLM working.
func formatBudgetNudge(pct, turnTokens, budget int) string {
	return fmt.Sprintf(
		"Stopped at %d%% of token target (%d / %d). Keep working — do not summarize.",
		pct, turnTokens, budget,
	)
}

// --- Token Budget Parsing ---

// Token budget regex patterns. Inspired by Claude Code's tokenBudget.ts.
// Shorthand: "+500k", "+2m" (anchored to start or end to avoid false positives).
// Verbose: "use 500k tokens", "spend 2M tokens" (matches anywhere).
var (
	shorthandStartRe = regexp.MustCompile(`(?i)^\s*\+(\d+(?:\.\d+)?)\s*(k|m|b)`)
	shorthandEndRe   = regexp.MustCompile(`(?i)\s\+(\d+(?:\.\d+)?)\s*(k|m|b)\s*[.!?]?\s*$`)
	verboseRe        = regexp.MustCompile(`(?i)\b(?:use|spend)\s+(\d+(?:\.\d+)?)\s*(k|m|b)\s*tokens?`)
)

// multipliers maps suffix letters to their numeric multiplier.
var multipliers = map[string]float64{
	"k": 1_000,
	"m": 1_000_000,
	"b": 1_000_000_000,
}

// ParseTokenBudget extracts a token budget from user input text.
// Returns 0 if no budget expression is found.
// Supports formats: "+500k", "+2m", "use 500k tokens", "spend 2M tokens".
func ParseTokenBudget(text string) int {
	// Try shorthand at start: "+500k do this task"
	if m := shorthandStartRe.FindStringSubmatch(text); m != nil {
		return parseBudgetMatch(m[1], m[2])
	}

	// Try shorthand at end: "do this task +500k"
	if m := shorthandEndRe.FindStringSubmatch(text); m != nil {
		return parseBudgetMatch(m[1], m[2])
	}

	// Try verbose: "use 500k tokens to solve this"
	if m := verboseRe.FindStringSubmatch(text); m != nil {
		return parseBudgetMatch(m[1], m[2])
	}

	return 0
}

// parseBudgetMatch converts a numeric string and suffix into a token count.
func parseBudgetMatch(value, suffix string) int {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f <= 0 {
		return 0
	}
	mult := multipliers[strings.ToLower(suffix)]
	if mult == 0 {
		return 0
	}
	return int(math.Round(f * mult))
}
