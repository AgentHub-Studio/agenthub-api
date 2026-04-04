package agentic

import (
	"fmt"
	"log/slog"
)

// Auto-compact constants controlling when context compaction triggers.
//
// Inspired by Claude Code's autoCompact.ts constants.
const (
	// MaxOutputTokensForSummary reserves tokens for the compact summary output.
	// Based on p99.99 of compact summary output being 17,387 tokens.
	MaxOutputTokensForSummary = 20_000

	// AutoCompactBufferTokens is the buffer below the effective context window
	// at which automatic compaction triggers.
	AutoCompactBufferTokens = 13_000

	// WarningThresholdBufferTokens is the buffer for warning threshold.
	WarningThresholdBufferTokens = 20_000

	// ErrorThresholdBufferTokens is the buffer for error threshold.
	ErrorThresholdBufferTokens = 20_000

	// ManualCompactBufferTokens is the buffer for the blocking limit
	// at which the system forces manual compaction.
	ManualCompactBufferTokens = 3_000

	// MaxConsecutiveAutoCompactFailures is the circuit breaker threshold.
	// After this many consecutive failures, auto-compact stops retrying
	// to prevent wasting API calls on irrecoverable situations.
	MaxConsecutiveAutoCompactFailures = 3
)

// AutoCompactTrackingState tracks the state of auto-compaction across turns.
//
// Inspired by Claude Code's AutoCompactTrackingState.
type AutoCompactTrackingState struct {
	// Compacted indicates if compaction occurred on the last turn.
	Compacted bool `json:"compacted"`
	// TurnCounter increments each turn for periodic checks.
	TurnCounter int `json:"turnCounter"`
	// TurnID is a unique identifier for the current turn.
	TurnID string `json:"turnId"`
	// ConsecutiveFailures counts consecutive auto-compact failures.
	// Reset on success. Used as a circuit breaker.
	ConsecutiveFailures int `json:"consecutiveFailures"`
}

// TokenWarningState holds computed thresholds for context window usage.
//
// Inspired by Claude Code's calculateTokenWarningState return type.
type TokenWarningState struct {
	// PercentLeft is the percentage of context window remaining (0-100).
	PercentLeft int `json:"percentLeft"`
	// IsAboveWarningThreshold indicates token usage exceeded the warning level.
	IsAboveWarningThreshold bool `json:"isAboveWarningThreshold"`
	// IsAboveErrorThreshold indicates token usage exceeded the error level.
	IsAboveErrorThreshold bool `json:"isAboveErrorThreshold"`
	// IsAboveAutoCompactThreshold indicates auto-compact should trigger.
	IsAboveAutoCompactThreshold bool `json:"isAboveAutoCompactThreshold"`
	// IsAtBlockingLimit indicates the context is nearly full and must be compacted.
	IsAtBlockingLimit bool `json:"isAtBlockingLimit"`
}

// GetEffectiveContextWindowSize calculates the usable context window
// for a model after reserving space for summary output.
//
// Inspired by Claude Code's getEffectiveContextWindowSize.
func GetEffectiveContextWindowSize(contextWindow int, maxOutputTokens int) int {
	reserved := maxOutputTokens
	if reserved > MaxOutputTokensForSummary {
		reserved = MaxOutputTokensForSummary
	}
	effective := contextWindow - reserved
	if effective < 0 {
		effective = 0
	}
	return effective
}

// GetAutoCompactThreshold calculates the token count at which auto-compaction triggers.
//
// Inspired by Claude Code's getAutoCompactThreshold.
func GetAutoCompactThreshold(effectiveContextWindow int) int {
	threshold := effectiveContextWindow - AutoCompactBufferTokens
	if threshold < 0 {
		threshold = 0
	}
	return threshold
}

// CalculateTokenWarningState computes all thresholds for a given token usage.
//
// Inspired by Claude Code's calculateTokenWarningState.
func CalculateTokenWarningState(tokenUsage, effectiveContextWindow int, autoCompactEnabled bool) TokenWarningState {
	autoCompactThreshold := GetAutoCompactThreshold(effectiveContextWindow)
	blockingLimit := effectiveContextWindow - ManualCompactBufferTokens
	if blockingLimit < 0 {
		blockingLimit = 0
	}

	percentLeft := 0
	if effectiveContextWindow > 0 {
		percentLeft = int(float64(effectiveContextWindow-tokenUsage) / float64(effectiveContextWindow) * 100)
		if percentLeft < 0 {
			percentLeft = 0
		}
	}

	return TokenWarningState{
		PercentLeft:                 percentLeft,
		IsAboveWarningThreshold:    tokenUsage >= (effectiveContextWindow - WarningThresholdBufferTokens),
		IsAboveErrorThreshold:      tokenUsage >= (effectiveContextWindow - ErrorThresholdBufferTokens),
		IsAboveAutoCompactThreshold: autoCompactEnabled && tokenUsage >= autoCompactThreshold,
		IsAtBlockingLimit:          tokenUsage >= blockingLimit,
	}
}

// ShouldAutoCompact determines if auto-compaction should run based on
// token usage and tracking state. Returns false if circuit breaker is open.
//
// Inspired by Claude Code's shouldAutoCompact.
func ShouldAutoCompact(tokenUsage, effectiveContextWindow int, tracking *AutoCompactTrackingState) bool {
	if tracking == nil {
		return false
	}

	// Circuit breaker: stop after too many consecutive failures.
	if tracking.ConsecutiveFailures >= MaxConsecutiveAutoCompactFailures {
		return false
	}

	state := CalculateTokenWarningState(tokenUsage, effectiveContextWindow, true)
	return state.IsAboveAutoCompactThreshold
}

// AutoCompactResult holds the outcome of an auto-compaction attempt.
type AutoCompactResult struct {
	WasCompacted        bool `json:"wasCompacted"`
	ConsecutiveFailures int  `json:"consecutiveFailures"`
}

// RecordAutoCompactSuccess records a successful compaction, resetting the circuit breaker.
func RecordAutoCompactSuccess(tracking *AutoCompactTrackingState) AutoCompactResult {
	if tracking != nil {
		tracking.Compacted = true
		tracking.ConsecutiveFailures = 0
	}
	return AutoCompactResult{WasCompacted: true, ConsecutiveFailures: 0}
}

// RecordAutoCompactFailure records a failed compaction, incrementing the circuit breaker.
func RecordAutoCompactFailure(tracking *AutoCompactTrackingState) AutoCompactResult {
	failures := 0
	if tracking != nil {
		tracking.ConsecutiveFailures++
		failures = tracking.ConsecutiveFailures
		tracking.Compacted = false

		if failures >= MaxConsecutiveAutoCompactFailures {
			slog.Warn("auto-compact circuit breaker tripped",
				"consecutiveFailures", failures,
				"threshold", MaxConsecutiveAutoCompactFailures)
		}
	}
	return AutoCompactResult{WasCompacted: false, ConsecutiveFailures: failures}
}

// FormatTokenUsageSummary returns a human-readable string of token usage status.
func FormatTokenUsageSummary(tokenUsage, effectiveContextWindow int) string {
	pct := 0
	if effectiveContextWindow > 0 {
		pct = int(float64(tokenUsage) / float64(effectiveContextWindow) * 100)
	}
	return fmt.Sprintf("%d/%d tokens (%d%%)", tokenUsage, effectiveContextWindow, pct)
}
