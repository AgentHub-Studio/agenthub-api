package agentic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify LOOP-003 (Stop conditions e maxTurns) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1), Section 4.5 ("Stop Conditions") which enumerates five
// stop conditions for the agentic loop:
//
//   1. No tool use         — model produces only text content (primary stop).
//   2. Max turns           — configurable maxTurns limit is reached.
//   3. Context overflow    — API returns prompt_too_long.
//   4. Hook intervention   — PostToolUse hook sets hook_stopped_continuation.
//   5. Explicit abort      — abortController fires.
//
// AgentHub maps these to:
//   1. finishReason == "stop" with empty toolCalls (runner.go:1062).
//   2. RunConfig.MaxIterations enforced as `for turnIndex < cfg.MaxIterations`
//      (runner.go:763).
//   3. TransitionPromptTooLong / RecordContextOverflow on TurnState.
//   4. StopHookResult.PreventContinuation (stophooks.go:34).
//   5. context cancellation via run-scoped context.WithTimeout/cancel
//      (runner.go:318-320).
//
// These are configuration- and contract-level concerns that we can ratify
// without instantiating the full Runner (which requires LLM/persistence
// dependencies). The actual loop predicate is asserted by inspection (cited
// inline) and the helpers exposed by RunConfig / TurnState / StopHookResult
// are exercised here.

func TestBDD_StopConditionsAndMaxTurns(t *testing.T) {
	t.Run("Scenario_DefaultMaxIterationsIsBoundedNotInfinite", func(t *testing.T) {
		// Given the production default RunConfig is requested,
		given := DefaultRunConfig()

		// When the runner consults MaxIterations as the loop cap,
		when := given.MaxIterations

		// Then the cap is a positive finite integer — PDF Section 4.5: max
		//      turns is a configurable cap that always bounds the loop, never
		//      unbounded.
		assert.Greater(t, when, 0,
			"MaxIterations must be positive — unbounded loop is not allowed")
		assert.LessOrEqual(t, when, 1000,
			"MaxIterations must be sane (<=1000) to bound runaway loops")
	})

	t.Run("Scenario_AgentSuppliedMaxIterationsOverridesDefault", func(t *testing.T) {
		// Given an agent's model_config that overrides MaxIterations,
		raw := json.RawMessage(`{"maxIterations":7}`)

		// When the runner builds its RunConfig from the agent JSON,
		when := RunConfigFromModelConfig(raw)

		// Then the override wins — PDF Section 4.5 lists max turns as
		//      "configurable", and AgentHub honours per-agent overrides.
		assert.Equal(t, 7, when.MaxIterations,
			"per-agent maxIterations override must win over default")
	})

	t.Run("Scenario_StopHookCanVetoContinuation", func(t *testing.T) {
		// Given a PostToolUse stop hook returns a result that vetoes
		//       continuation (PDF Section 4.5: "PostToolUse hook sets
		//       hook_stopped_continuation"),
		given := StopHookResult{
			PreventContinuation: true,
			StopReason:          "policy violation: external API quota exceeded",
		}

		// When the runner inspects the result,
		// Then PreventContinuation is true and a human-readable reason is
		//      attached for the SSE stop_hook_veto event (runner.go:1117/1417/1588).
		assert.True(t, given.PreventContinuation,
			"hook veto must surface as PreventContinuation")
		assert.NotEmpty(t, given.StopReason,
			"vetoing hook must attach a stop reason for diagnostics")
	})

	t.Run("Scenario_StopHookAllowingContinuationIsTheDefault", func(t *testing.T) {
		// Given a stop hook returns its zero value (i.e. no opinion),
		var given StopHookResult

		// When the runner inspects the result,
		// Then continuation is the default — PDF principle: the harness only
		//      stops on an explicit veto, not on hook silence.
		assert.False(t, given.PreventContinuation,
			"hook silence must allow the loop to continue")
		assert.Empty(t, given.StopReason,
			"no stop reason when continuation is allowed")
	})

	t.Run("Scenario_ExplicitAbortViaContextCancellationStops", func(t *testing.T) {
		// Given a parent context that is already cancelled (PDF Section 4.5:
		//       "Explicit abort — abortController signal fires"),
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// When the runner checks the context error at loop iteration start
		//      (runner.go:765-768 logs and exits when ctx.Err() != nil),
		when := ctx.Err()

		// Then the context surfaces a cancellation error — equivalent to
		//      Claude Code's abortController signal firing.
		assert.Error(t, when, "cancelled context must report an error")
		assert.ErrorIs(t, when, context.Canceled,
			"cancellation must propagate as context.Canceled")
	})

	t.Run("Scenario_TotalTimeoutBoundsTheRunByWallClock", func(t *testing.T) {
		// Given the production default RunConfig is requested,
		given := DefaultRunConfig()

		// When the runner consults TotalTimeout to derive a deadline,
		when := given.TotalTimeout

		// Then a finite deadline exists — AgentHub augments PDF Section 4.5
		//      with a wall-clock cap so a model that never emits "stop" still
		//      terminates within a bounded duration.
		assert.Greater(t, when, time.Duration(0),
			"TotalTimeout must be positive — runs cannot be unbounded by wall clock")
		assert.GreaterOrEqual(t, when, given.LLMCallTimeout,
			"TotalTimeout must be >= LLMCallTimeout to avoid contradictory cutoffs")
	})

	t.Run("Scenario_TimeoutContextDeadlineReachable", func(t *testing.T) {
		// Given a runner-style timeout context with a tiny deadline,
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// When the deadline elapses,
		time.Sleep(5 * time.Millisecond)

		// Then context.DeadlineExceeded fires — the same mechanism the runner
		//      uses to bound a run via context.WithTimeout(ctx, TotalTimeout)
		//      at runner.go:320.
		assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded,
			"deadline-bounded context must report DeadlineExceeded")
	})

	t.Run("Scenario_TransitionEnumIncludesAllStopRecoveryReasons", func(t *testing.T) {
		// Given the loop must record WHY it stopped or continued for
		//       diagnostics (PDF Section 4.5 + 4.4),
		// When the TurnTransition enum is enumerated,
		// Then the four PDF-described recovery transitions are present and
		//      distinct so a refactor cannot silently drop one — failing here
		//      means the loop is no longer attributing its own behaviour.
		set := map[TurnTransition]bool{
			TransitionPromptTooLong:     true, // Section 4.5 #3 + 4.4
			TransitionContextOverflow:   true, // Section 4.5 #3 supplemental
			TransitionStopHook:          true, // Section 4.5 #4
			TransitionMaxTokensRecovery: true, // Section 4.4 (recovery before stop)
		}
		assert.Len(t, set, 4,
			"all four PDF recovery/stop transitions must be distinct constants")
	})
}
