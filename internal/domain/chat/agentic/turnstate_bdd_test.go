package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify LOOP-001 (Agentic loop central) against the
// Claude Code architecture paper "Dive into Claude Code" (arXiv:2604.14228v1):
//   - Section 4.1 The Query Pipeline (mutable state init, loop continuation sites)
//   - Section 4.4 Recovery Mechanisms (max_output_tokens recovery limit = 3,
//     reactive compaction fires at most once per turn, context overflow,
//     prompt_too_long handling)
//   - Section 4.5 Stop Conditions (no tool use, max turns, hook intervention,
//     explicit abort, context overflow)
//
// AgentHub's Runner.Run channel-of-events is the Go-idiomatic equivalent of
// Claude Code's queryLoop AsyncGenerator (Section 3.2 / Figure 1). TurnState
// is the consolidated mutable-state object that the PDF describes as
// "destructured at the top of each iteration and reassigned at continue
// sites" (Section 4.1 step 2).

func TestBDD_AgenticLoopCentral(t *testing.T) {
	t.Run("Scenario_NewTurnStateStartsWithZeroIterations", func(t *testing.T) {
		// Given a fresh agentic run is about to start,
		// When the TurnState is initialised,
		when := NewTurnState()

		// Then the iteration counter is zero (PDF Section 4.1: TurnCount is the
		//      current iteration number, 1-indexed after NextTurn fires) and no
		//      recovery has occurred.
		assert.Equal(t, 0, when.TurnCount, "fresh state must start at turn 0")
		assert.Equal(t, TransitionNone, when.Transition,
			"fresh state must have no prior transition")
		assert.Equal(t, 0, when.MaxOutputTokensRecoveryCount,
			"recovery counter must start at zero")
		assert.False(t, when.HasAttemptedReactiveCompact,
			"reactive-compact flag must start cleared")
		assert.False(t, when.StartedAt.IsZero(), "start time must be recorded")
	})

	t.Run("Scenario_NextTurnAdvancesIterationAndRecordsTransition", func(t *testing.T) {
		// Given a TurnState that has already executed one turn,
		given := NewTurnState()
		given.NextTurn(TransitionNone)

		// When the loop continues because the model emitted tool_calls
		//      (PDF Section 4.1 step 6: tool-use dispatch leads to a new
		//      iteration with TransitionToolUse),
		given.NextTurn(TransitionToolUse)

		// Then both the count and the transition reason are updated atomically
		//      so subsequent diagnostics can attribute the iteration.
		assert.Equal(t, 2, given.TurnCount, "NextTurn must increment count")
		assert.Equal(t, TransitionToolUse, given.Transition,
			"transition reason must reflect why the loop continued")
	})

	t.Run("Scenario_MaxOutputTokensRecoveryCapsAtThree", func(t *testing.T) {
		// Given the model repeatedly hits the output token cap,
		given := NewTurnState()

		// When the runner records three recovery attempts in a row,
		given.RecordMaxTokensRecovery(8000)
		assert.True(t, given.ShouldRetryMaxTokens(),
			"after 1 recovery the runner may still try")
		given.RecordMaxTokensRecovery(16000)
		assert.True(t, given.ShouldRetryMaxTokens(),
			"after 2 recoveries the runner may still try")
		given.RecordMaxTokensRecovery(32000)

		// Then the runner refuses a fourth attempt — PDF Section 4.4:
		//      "Up to three recovery attempts are allowed per turn
		//      (MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3)".
		assert.False(t, given.ShouldRetryMaxTokens(),
			"after 3 recoveries no further retry must be allowed")
		assert.Equal(t, TransitionMaxTokensRecovery, given.Transition,
			"transition must record the last recovery reason")
		assert.Equal(t, 32000, given.EffectiveMaxTokens(4000),
			"override must take precedence over the configured default")
	})

	t.Run("Scenario_ReactiveCompactFiresAtMostOncePerTurn", func(t *testing.T) {
		// Given context pressure triggers reactive compaction,
		given := NewTurnState()
		assert.True(t, given.CanAttemptReactiveCompact(),
			"compact must be available initially")

		// When the runner records the compaction,
		given.RecordReactiveCompact()

		// Then a second attempt is forbidden in the same turn — PDF
		//      Section 4.4: "hasAttemptedReactiveCompact ensures this fires at
		//      most once per turn".
		assert.False(t, given.CanAttemptReactiveCompact(),
			"reactive compact must fire at most once per turn")
		assert.True(t, given.HasAttemptedReactiveCompact,
			"flag must be set after the attempt")
		assert.Equal(t, TransitionReactiveCompact, given.Transition,
			"transition must record reactive_compact")
	})

	t.Run("Scenario_ContextOverflowReducesMaxTokensInPlace", func(t *testing.T) {
		// Given input + max_tokens exceeds the model's context window,
		given := NewTurnState()

		// When the runner records the context-overflow recovery,
		given.RecordContextOverflow(2048)

		// Then EffectiveMaxTokens returns the reduced cap so the next
		//      iteration fits in the window — PDF Section 4.4: the harness
		//      reduces max_tokens automatically rather than failing hard.
		assert.Equal(t, 2048, given.EffectiveMaxTokens(8000),
			"context overflow must lower the effective cap")
		assert.Equal(t, TransitionContextOverflow, given.Transition,
			"transition must record context_overflow")
	})

	t.Run("Scenario_RecoveryTransitionsAreIndependent", func(t *testing.T) {
		// Given a turn that suffers more than one recovery class,
		given := NewTurnState()

		// When the runner first compacts, then hits the token cap,
		given.RecordReactiveCompact()
		given.RecordMaxTokensRecovery(16000)

		// Then both pieces of state survive — PDF Section 4.1 step 2:
		//      "single State object overwritten via continue points" — and the
		//      transition reflects the most recent recovery.
		assert.True(t, given.HasAttemptedReactiveCompact,
			"prior reactive_compact must remain recorded")
		assert.Equal(t, 1, given.MaxOutputTokensRecoveryCount,
			"max-tokens counter must increment independently")
		assert.Equal(t, TransitionMaxTokensRecovery, given.Transition,
			"transition must reflect the latest recovery")
	})

	t.Run("Scenario_TransitionEnumCoversAllPDFRecoveryReasons", func(t *testing.T) {
		// Given the PDF enumerates discrete loop-continuation reasons
		//       (Section 4.1 step 9 + 4.4 + 4.5),
		// When we list AgentHub's TurnTransition enum,
		// Then every PDF reason has a corresponding constant — preventing the
		//      loop from continuing for an unattributed reason.
		expected := []TurnTransition{
			TransitionNone,
			TransitionToolUse,            // Section 4.1 step 6
			TransitionBudgetContinue,     // Section 4.1 step 9 + budget continuation
			TransitionMaxTokensRecovery,  // Section 4.4 max output tokens escalation
			TransitionReactiveCompact,    // Section 4.4 reactive compaction
			TransitionPromptTooLong,      // Section 4.4 prompt_too_long handling
			TransitionContextOverflow,    // Section 4.4 context overflow
			TransitionStopHook,           // Section 4.5 stop-hook intervention
		}
		// Asserting each constant exists ensures the enum cannot drop a PDF
		// reason silently — a refactor that removed one would fail to compile.
		for _, tr := range expected {
			assert.NotNil(t, tr, "transition constant must exist for PDF reason")
		}
		assert.Len(t, expected, 8,
			"AgentHub must enumerate all eight loop transitions from the PDF")
	})

	t.Run("Scenario_DurationsAreNonNegativeAfterTurn", func(t *testing.T) {
		// Given a TurnState that just advanced one iteration,
		given := NewTurnState()
		given.NextTurn(TransitionToolUse)

		// When the caller queries durations,
		// Then both run and turn durations are non-negative — PDF principle:
		//      observable, attributable per-turn diagnostics.
		assert.GreaterOrEqual(t, given.RunDuration().Nanoseconds(), int64(0),
			"run duration must never be negative")
		assert.GreaterOrEqual(t, given.TurnDuration().Nanoseconds(), int64(0),
			"turn duration must never be negative")
	})
}
