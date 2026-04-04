package agentic

import (
	"time"
)

// TurnTransition describes why the previous loop iteration continued
// instead of stopping. This is used for diagnostics, analytics, and
// recovery path tracking.
//
// Inspired by Claude Code's State.transition in query.ts.
type TurnTransition string

const (
	// TransitionNone means no previous transition (first turn).
	TransitionNone TurnTransition = ""
	// TransitionToolUse means the LLM emitted tool_calls and needs to continue.
	TransitionToolUse TurnTransition = "tool_use"
	// TransitionBudgetContinue means the token budget nudge triggered continuation.
	TransitionBudgetContinue TurnTransition = "budget_continue"
	// TransitionMaxTokensRecovery means max_output_tokens was hit and recovered.
	TransitionMaxTokensRecovery TurnTransition = "max_tokens_recovery"
	// TransitionReactiveCompact means context was too large and compact was triggered.
	TransitionReactiveCompact TurnTransition = "reactive_compact"
	// TransitionPromptTooLong means prompt_too_long was hit and messages were dropped.
	TransitionPromptTooLong TurnTransition = "prompt_too_long"
	// TransitionContextOverflow means input+max_tokens exceeded context and max_tokens was reduced.
	TransitionContextOverflow TurnTransition = "context_overflow"
	// TransitionStopHook means a stop hook requested continuation.
	TransitionStopHook TurnTransition = "stop_hook"
)

// TurnState carries per-iteration mutable state through the agentic runner loop.
// It is destructured at the top of each iteration and reassigned at continue sites.
// This consolidation prevents scattered state variables and makes recovery paths testable.
//
// Inspired by Claude Code's loop-local State struct in query.ts.
type TurnState struct {
	// TurnCount is the current iteration number (1-indexed).
	TurnCount int

	// Transition describes why the previous iteration continued.
	Transition TurnTransition

	// MaxOutputTokensRecoveryCount tracks how many times max_output_tokens recovery fired.
	// After too many recoveries, the runner should stop instead of retrying.
	MaxOutputTokensRecoveryCount int

	// HasAttemptedReactiveCompact is set to true after the first reactive compaction.
	// Prevents infinite compact→retry→compact loops on a single turn.
	HasAttemptedReactiveCompact bool

	// MaxOutputTokensOverride is set when context overflow requires reducing max_tokens.
	// Zero means use the default from RunConfig.
	MaxOutputTokensOverride int

	// StopHookActive indicates a stop hook is currently being evaluated.
	// When true, the runner should not process new tool calls.
	StopHookActive bool

	// StartedAt records when the current run started (for duration tracking).
	StartedAt time.Time

	// TurnStartedAt records when the current turn started.
	TurnStartedAt time.Time
}

// NewTurnState creates initial turn state for a new run.
func NewTurnState() TurnState {
	now := time.Now()
	return TurnState{
		TurnCount:     0,
		StartedAt:     now,
		TurnStartedAt: now,
	}
}

// NextTurn advances the turn counter and records the transition reason.
// Call this at the top of each loop iteration.
func (ts *TurnState) NextTurn(transition TurnTransition) {
	ts.TurnCount++
	ts.Transition = transition
	ts.TurnStartedAt = time.Now()
}

// RecordMaxTokensRecovery increments the recovery counter and sets the override.
func (ts *TurnState) RecordMaxTokensRecovery(adjustedMaxTokens int) {
	ts.MaxOutputTokensRecoveryCount++
	ts.MaxOutputTokensOverride = adjustedMaxTokens
	ts.Transition = TransitionMaxTokensRecovery
}

// RecordReactiveCompact marks that reactive compaction was attempted.
func (ts *TurnState) RecordReactiveCompact() {
	ts.HasAttemptedReactiveCompact = true
	ts.Transition = TransitionReactiveCompact
}

// RecordContextOverflow sets the adjusted max_tokens from a context overflow error.
func (ts *TurnState) RecordContextOverflow(adjustedMaxTokens int) {
	ts.MaxOutputTokensOverride = adjustedMaxTokens
	ts.Transition = TransitionContextOverflow
}

// ShouldRetryMaxTokens returns true if max_output_tokens recovery should be attempted.
// Returns false after too many recoveries (max 3).
func (ts *TurnState) ShouldRetryMaxTokens() bool {
	return ts.MaxOutputTokensRecoveryCount < 3
}

// CanAttemptReactiveCompact returns true if reactive compaction hasn't been tried yet.
func (ts *TurnState) CanAttemptReactiveCompact() bool {
	return !ts.HasAttemptedReactiveCompact
}

// EffectiveMaxTokens returns the max tokens to use, considering any override.
func (ts *TurnState) EffectiveMaxTokens(defaultMax int) int {
	if ts.MaxOutputTokensOverride > 0 {
		return ts.MaxOutputTokensOverride
	}
	return defaultMax
}

// RunDuration returns the elapsed time since the run started.
func (ts *TurnState) RunDuration() time.Duration {
	return time.Since(ts.StartedAt)
}

// TurnDuration returns the elapsed time since the current turn started.
func (ts *TurnState) TurnDuration() time.Duration {
	return time.Since(ts.TurnStartedAt)
}

// --- Query Dependencies ---

// QueryDeps abstracts the 4 core I/O dependencies of the query loop,
// enabling unit tests to replace them with fakes without module-level mocking.
//
// Inspired by Claude Code's QueryDeps in query/deps.ts.
type QueryDeps struct {
	// CallModel invokes the LLM. In production, this wraps retryStreamWithFallback.
	CallModel CallModelFn
	// Compact compresses messages when context exceeds threshold.
	Compact CompactFn
	// GenerateID produces unique identifiers for turns and events.
	GenerateID func() string
}

// CallModelFn is the signature for the LLM invocation dependency.
type CallModelFn func(ctx interface{}, messages interface{}, opts interface{}) (interface{}, error)

// CompactFn is the signature for the message compaction dependency.
type CompactFn func(ctx interface{}, messages interface{}) (interface{}, error)

// DefaultQueryDeps returns production dependencies (nil — to be wired by the runner).
func DefaultQueryDeps() QueryDeps {
	return QueryDeps{}
}
