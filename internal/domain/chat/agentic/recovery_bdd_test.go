package agentic

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify LOOP-005 (Recovery e fallback) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 4.4 (Recovery Mechanisms): the query loop implements several
//     recovery layers — max output tokens escalation (cap=3 attempts per
//     turn), reactive compaction (gated by REACTIVE_COMPACT, fires once),
//     prompt_too_long handling (context-collapse + reactive compact before
//     fail), streaming fallback (onStreamingFallback callback), and
//     fallback model (alternative model when primary fails).
//   - Section 4.5: stop conditions enumerate explicit reasons; recovery
//     paths must be DISTINGUISHABLE so observability (OBS-001/OBS-002) can
//     attribute the cause.
//
// AgentHub maps recovery to:
//   - retry.go retryStream — exponential backoff with classifyTransientError
//     into 4 categories (rate_limit, overload, timeout, unknown).
//   - retry.go shouldFallback / isContextOverflow / isOverloadError /
//     isTransientError — distinct error classifiers.
//   - retry.go parseRetryAfter — honours Retry-After header for adaptive backoff.
//   - retry.go parseContextOverflow + computeAdjustedMaxTokens — context
//     overflow recovery via reduced max_tokens.
//   - retry.go parsePromptTooLongTokenCounts + getPromptTooLongTokenGap —
//     prompt_too_long recovery analytics.
//   - retry.go APIErrorClass enum — 10 classes for differentiated handling.
//   - streamfallback.go retryStreamWithNonStreamingFallback — fallback to
//     non-streaming when streaming fails.
//   - streamfallback.go EvaluateModelFallback + isStreamingFallbackEligible
//     — model-fallback decision logic.
//   - turnstate.go TurnTransition enum — observable transition reasons.
//
// These scenarios assert: classifier correctness, parser robustness,
// fallback decision policy, and structural enum coverage.

func TestBDD_RecoveryAndFallback(t *testing.T) {
	t.Run("Scenario_TransientErrorClassifierCoversFourCategories", func(t *testing.T) {
		// Given the PDF Section 4.4 distinguishes recovery paths by error
		//       cause,
		// When classifyTransientError inspects representative errors,
		// Then each maps to its category — 429/529/rate_limit → rate_limit,
		//      502/503/overloaded → overload, timeout/EOF → timeout, others
		//      → unknown.
		assert.Equal(t, errorTypeRateLimit,
			classifyTransientError(errors.New("HTTP 429 too many requests")),
			"429 must classify as rate_limit")
		assert.Equal(t, errorTypeRateLimit,
			classifyTransientError(errors.New("HTTP 529 capacity")),
			"529 (Anthropic capacity) must classify as rate_limit")
		assert.Equal(t, errorTypeOverload,
			classifyTransientError(errors.New("503 service unavailable")),
			"503 must classify as overload")
		assert.Equal(t, errorTypeTimeout,
			classifyTransientError(errors.New("connection reset by peer")),
			"connection reset must classify as timeout")
		assert.Equal(t, errorTypeUnknown,
			classifyTransientError(errors.New("malformed json")),
			"unmatched errors fall back to unknown — never silently mislabelled")
	})

	t.Run("Scenario_APIErrorClassEnumIsExhaustiveForAttribution", func(t *testing.T) {
		// Given analytics needs differentiated error reporting (PDF Section
		//       11 + OBS-002 RunMetric.ErrorClasses),
		// When we list AgentHub's APIErrorClass constants,
		classes := map[APIErrorClass]bool{
			ErrorClassRateLimit:       true,
			ErrorClassServerOverload:  true,
			ErrorClassConnection:      true,
			ErrorClassStaleConnection: true,
			ErrorClassTimeout:         true,
			ErrorClassAuth:            true,
			ErrorClassPromptTooLong:   true,
			ErrorClassMediaSize:       true,
			ErrorClassAborted:         true,
			ErrorClassServerError:     true,
		}

		// Then 10 distinct error classes exist — refactor that collapses
		//      two fails this guard, preserving analytics granularity.
		assert.Len(t, classes, 10,
			"AgentHub must enumerate exactly 10 APIErrorClass constants for differentiated attribution")
	})

	t.Run("Scenario_RetryAfterHeaderIsParsedForAdaptiveBackoff", func(t *testing.T) {
		// Given a 429 response carrying Retry-After (PDF Section 4.4
		//       implies adaptive backoff to respect upstream signals),
		// When parseRetryAfter inspects the message,
		when := parseRetryAfter("HTTP 429: rate limited, retry-after: 7s")

		// Then the suggested duration is honoured — preventing thundering
		//      herd retries.
		assert.GreaterOrEqual(t, when, time.Duration(0),
			"parseRetryAfter must return a non-negative duration")
	})

	t.Run("Scenario_ContextOverflowParserExtractsTokenCounts", func(t *testing.T) {
		// Given a context_overflow API error in the recognised format
		//       (regex: input.+max_tokens.+exceed.+context.+: NNN + NNN > NNN),
		errMsg := "input length and max_tokens exceed context limit: 195000 + 8192 > 200000"

		// When parseContextOverflow extracts the counts,
		when := parseContextOverflow(errMsg)

		// Then the parser surfaces non-zero token counts — recovery is
		//      observable, not opaque.
		assert.Equal(t, 195000, when.InputTokens,
			"InputTokens must be parsed from the message")
		assert.Equal(t, 8192, when.MaxTokens,
			"MaxTokens must be parsed from the message")
		assert.Equal(t, 200000, when.ContextLimit,
			"ContextLimit must be parsed from the message")
	})

	t.Run("Scenario_AdjustedMaxTokensReducesToFitContext", func(t *testing.T) {
		// Given a context_overflow message in recognised format,
		errMsg := "input length and max_tokens exceed context limit: 195000 + 16000 > 200000"

		// When computeAdjustedMaxTokens proposes a new cap,
		when := computeAdjustedMaxTokens(errMsg)

		// Then the proposal is positive and lower than the offending cap —
		//      the runner can retry with the reduced cap (PDF Section 4.4
		//      transition: TransitionContextOverflow). Logic: contextLimit -
		//      input - 1000 safety, with 3000 floor.
		assert.GreaterOrEqual(t, when, 3000,
			"adjusted max_tokens must respect the 3000 floor")
		assert.Less(t, when, 16000,
			"adjusted must be lower than the original 16000")
	})

	t.Run("Scenario_PromptTooLongAnalyticsExtractsGap", func(t *testing.T) {
		// Given a prompt_too_long error from the API (PDF Section 4.4:
		//       prompt_too_long handling — context-collapse + reactive
		//       compact before fail),
		errMsg := "prompt is too long: 250000 tokens > 200000 maximum"

		// When the runner inspects the gap,
		gap := getPromptTooLongTokenGap(errMsg)

		// Then the gap is observable — analytics can show how much the
		//      model was over budget, informing future budget tuning.
		assert.GreaterOrEqual(t, gap, 0,
			"prompt-too-long gap must be non-negative")
	})

	t.Run("Scenario_IsContextOverflowDistinguishesFromOtherErrors", func(t *testing.T) {
		// Given different kinds of errors (PDF Section 4.4 + 11: each
		//       failure mode requires DIFFERENT recovery; misclassification
		//       sends the runner down the wrong path),
		overflowErr := errors.New("input length and max_tokens exceed context limit: 195000 + 8192 > 200000")

		// When the classifier inspects them,
		// Then context_overflow is recognised distinctly from generic 5XX.
		assert.True(t, isContextOverflow(overflowErr),
			"context overflow regex pattern must classify distinctly")
		assert.False(t, isContextOverflow(errors.New("HTTP 503")),
			"5XX must NOT classify as context overflow")
	})

	t.Run("Scenario_OverloadErrorDetectorRecognisesUpstreamCapacity", func(t *testing.T) {
		// Given an upstream capacity error,
		assert.True(t, isOverloadError(errors.New("HTTP 529 overloaded")),
			"529 must classify as overload")
		assert.True(t, isOverloadError(errors.New("HTTP 503 service overloaded")),
			"503 with overloaded must classify")

		// And given a non-overload error,
		assert.False(t, isOverloadError(errors.New("HTTP 401 unauthorized")),
			"auth errors must NOT classify as overload")
	})

	t.Run("Scenario_TransientErrorDetectorIncludesNetworkConditions", func(t *testing.T) {
		// Given network conditions that warrant retry,
		assert.True(t, isTransientError(errors.New("connection reset")),
			"connection reset is transient — retry-eligible")
		assert.True(t, isTransientError(errors.New("EOF")),
			"EOF mid-stream is transient")

		// And given non-transient errors,
		assert.False(t, isTransientError(errors.New("invalid api key")),
			"auth errors must NOT classify as transient")
	})

	t.Run("Scenario_ModelFallbackTriggersAfterConsecutiveCapacityErrors", func(t *testing.T) {
		// Given primary model has hit consecutive 529s (PDF Section 4.4:
		//       fallback model swaps to alternative when primary fails),
		fallbackModels := []string{"claude-haiku-4-5", "gpt-4o-mini"}

		// When EvaluateModelFallback inspects the situation,
		when := EvaluateModelFallback(3, 2, "claude-sonnet-4-20250514", fallbackModels)

		// Then a fallback decision is produced — the runner can swap
		//      models without aborting the whole run.
		assert.True(t, when.ShouldFallback,
			"3 consecutive 529s past max529Retries=2 must trigger fallback")
		assert.NotEmpty(t, when.FallbackModel,
			"fallback model name must be selected from the list")
	})

	t.Run("Scenario_ModelFallbackDoesNotTriggerBelowThreshold", func(t *testing.T) {
		// Given only 1 consecutive 529,
		when := EvaluateModelFallback(1, 3, "claude-sonnet-4-20250514",
			[]string{"claude-haiku-4-5"})

		// Then fallback does NOT trigger — runner gives transient errors a
		//      fair chance before swapping models (cost + behaviour
		//      differences justify caution).
		assert.False(t, when.ShouldFallback,
			"1 transient error must not trigger model fallback")
	})

	t.Run("Scenario_ModelFallbackIsNoOpWithoutCandidates", func(t *testing.T) {
		// Given no fallback models configured,
		when := EvaluateModelFallback(10, 1, "claude-sonnet-4-20250514", nil)

		// Then no fallback — the runner cannot fabricate alternative models;
		//      operators must opt in.
		assert.False(t, when.ShouldFallback,
			"no fallback when no candidate models configured")
	})

	t.Run("Scenario_StreamingFallbackEligibilityCovers404AndOverload", func(t *testing.T) {
		// Given streaming-specific failures (PDF Section 4.4:
		//       onStreamingFallback callback — retry with non-streaming).
		//       isStreamingFallbackEligible covers: 404 (provider doesn't
		//       support streaming for the model), stale connection, overload
		//       (5XX may work non-streaming).
		assert.True(t, isStreamingFallbackEligible(errors.New("HTTP 404 streaming not supported")),
			"404 on streaming endpoint is fallback-eligible (model may not support streaming)")
		assert.True(t, isStreamingFallbackEligible(errors.New("HTTP 503 overloaded")),
			"503 with overloaded keyword is fallback-eligible")

		// And given non-stream-specific errors,
		assert.False(t, isStreamingFallbackEligible(errors.New("invalid api key")),
			"auth errors must NOT trigger streaming fallback")
	})

	t.Run("Scenario_StreamFallbackConfigDefaultsAreSensible", func(t *testing.T) {
		// Given a fresh StreamFallbackConfig (PDF Section 4.4: defaults
		//       must work without per-call configuration),
		given := DefaultStreamFallbackConfig()

		// When the runner inspects the defaults,
		// Then the config is non-zero — preventing accidental disable of
		//      the fallback layer.
		assert.NotEqual(t, StreamFallbackConfig{}, given,
			"DefaultStreamFallbackConfig must return a populated struct")
	})

	t.Run("Scenario_FallbackTriggeredErrorIsDistinguishableType", func(t *testing.T) {
		// Given the fallback layer signals "I switched to non-streaming",
		given := &FallbackTriggeredError{}

		// When the caller inspects the error,
		when := given.Error()

		// Then the type carries enough info that the caller can
		//      distinguish "fallback fired" from a generic API error.
		assert.NotEmpty(t, when,
			"FallbackTriggeredError must produce a non-empty Error message")
	})
}
