package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_PermissionDeniedFeedback(t *testing.T) {
	t.Run("Scenario_LLMReceivesStructuredPayloadInsteadOfGenericError", func(t *testing.T) {
		// Given a denied tool call,
		// When the harness formats feedback for the LLM,
		// Then the LLM sees a tagged header it can parse — not free-form
		// prose that varies between turns.
		f := PermissionDeniedFeedback{
			ToolName: "Bash", Reason: PermissionDenialRuleMatch,
			RetryHint: PermissionRetryNotRetryable,
		}
		got := f.FormatForLLM()
		assert.Contains(t, got, "[permission_denied")
		assert.Contains(t, got, "tool=Bash")
	})

	t.Run("Scenario_HookOverrideClassifiedSoLLMKnowsItIsNotEngineRule", func(t *testing.T) {
		// Given a hook denied an otherwise-allowed call,
		// When the LLM reads the feedback,
		// Then reason=hook_override (not rule_match) so the LLM does
		// not blame static rules.
		b := PermissionDeniedFeedbackBuilder{}
		eval := PermissionEvaluation{
			Request: PermissionHookRequest{ToolName: "execute-sql"},
			EngineDecision: PermissionAllow,
			FinalDecision:  PermissionDeny,
			HookDecisions: []PermissionHookDecision{
				{HookName: "pii", Outcome: PermissionHookOverrideDeny, Reason: "PII detected"},
			},
		}
		f := b.FromEvaluation(eval)
		assert.Equal(t, PermissionDenialHookOverride, f.Reason)
		assert.Equal(t, "pii", f.OverridingHook)
	})

	t.Run("Scenario_RetryHintGuidesLLMRecoveryStrategy", func(t *testing.T) {
		// Given a rate-limited call,
		// When the LLM reads retry=wait_and_retry,
		// Then it knows backing off is appropriate (vs trying a
		// different input — which would not help).
		b := PermissionDeniedFeedbackBuilder{}
		f := b.FromRateLimit("execute-sql", "", "rate-limiter", "60/min exceeded")
		assert.Equal(t, PermissionRetryWaitAndRetry, f.RetryHint)
	})

	t.Run("Scenario_AlternativeToolsSurfacedSoLLMCanPivot", func(t *testing.T) {
		// Given Bash is denied but a sandboxed shell is available,
		// When the LLM reads alt=shell_sandboxed,
		// Then it can switch tools without hallucinating an alternative.
		b := PermissionDeniedFeedbackBuilder{
			AlternativesFor: map[string][]string{"Bash": {"shell_sandboxed"}},
		}
		eval := PermissionEvaluation{
			Request: PermissionHookRequest{ToolName: "Bash"},
			EngineDecision: PermissionDeny, FinalDecision: PermissionDeny,
		}
		f := b.FromEvaluation(eval)
		assert.Contains(t, f.FormatForLLM(), "alt=shell_sandboxed")
	})

	t.Run("Scenario_PrefilterDropTellsLLMToolIsNotAvailable", func(t *testing.T) {
		// Given pool-time pre-filter dropped a tool but the LLM still
		// tried to call it (stale tool id from previous turn),
		// When the feedback returns,
		// Then reason=prefilter_drop + retry=suggest_alternative — the
		// LLM stops trying this specific tool.
		b := PermissionDeniedFeedbackBuilder{}
		f := b.FromPrefilterDrop("Bash", "")
		assert.Equal(t, PermissionDenialPrefilterDrop, f.Reason)
		assert.Equal(t, PermissionRetrySuggestAlternativeTool, f.RetryHint)
	})

	t.Run("Scenario_ModeBlockTellsLLMToAskUser", func(t *testing.T) {
		// Given the session is in dont_ask mode and a confirm-tier
		// tool was attempted,
		// When feedback returns,
		// Then retry=request_user_confirmation tells the LLM the path
		// forward is human approval, not retrying.
		b := PermissionDeniedFeedbackBuilder{}
		f := b.FromModeBlock("Write", "/tmp/file", PermissionModeDontAsk)
		assert.Equal(t, PermissionRetryRequestUserConfirmation, f.RetryHint)
	})

	t.Run("Scenario_SandboxViolationDetailFedBackToLLM", func(t *testing.T) {
		// Given a Bash call would escape the cwd sandbox,
		// When PERM-008 produces the violation,
		// Then the feedback explains WHAT was blocked so the LLM can
		// adjust input.
		b := PermissionDeniedFeedbackBuilder{}
		f := b.FromSandboxViolation("Bash", "cat /etc/shadow", "path escape outside cwd")
		assert.Equal(t, PermissionDenialSandboxViolation, f.Reason)
		assert.Contains(t, f.Explanation, "path escape")
	})

	t.Run("Scenario_LastDenyingHookIsTheOneReported", func(t *testing.T) {
		// Given After-phase has multiple hooks and the LAST override is
		// the one that actually wins (PERM-005 semantics),
		// When feedback is built,
		// Then the reported OverridingHook is the LAST denying hook —
		// matches the chain's "last override wins" rule.
		b := PermissionDeniedFeedbackBuilder{}
		eval := PermissionEvaluation{
			Request: PermissionHookRequest{ToolName: "x"},
			EngineDecision: PermissionAllow, FinalDecision: PermissionDeny,
			HookDecisions: []PermissionHookDecision{
				{HookName: "first-deny", Outcome: PermissionHookOverrideDeny, Reason: "first reason"},
				{HookName: "second-deny", Outcome: PermissionHookOverrideDeny, Reason: "second reason"},
			},
		}
		f := b.FromEvaluation(eval)
		assert.Equal(t, "second-deny", f.OverridingHook)
	})

	t.Run("Scenario_HookClassifierProducesSpecificReasonOverGeneric", func(t *testing.T) {
		// Given the platform knows "rate-limiter" hook means RateLimit,
		// When that hook denies,
		// Then the LLM sees reason=rate_limit (not generic hook_override).
		b := PermissionDeniedFeedbackBuilder{
			HookClassifies: map[string]PermissionDenialReason{
				"rate-limiter": PermissionDenialRateLimit,
			},
		}
		eval := PermissionEvaluation{
			Request: PermissionHookRequest{ToolName: "Read"},
			EngineDecision: PermissionAllow, FinalDecision: PermissionDeny,
			HookDecisions: []PermissionHookDecision{
				{HookName: "rate-limiter", Outcome: PermissionHookOverrideDeny},
			},
		}
		f := b.FromEvaluation(eval)
		assert.Equal(t, PermissionDenialRateLimit, f.Reason)
	})

	t.Run("Scenario_AllowedDecisionProducesNoFeedback", func(t *testing.T) {
		// Given the call was Allowed,
		// When the LLM reads feedback,
		// Then there is nothing to read — the builder returns nil so
		// callers do not surface phantom denials.
		b := PermissionDeniedFeedbackBuilder{}
		eval := PermissionEvaluation{
			Request: PermissionHookRequest{ToolName: "Read"},
			FinalDecision: PermissionAllow,
		}
		assert.Nil(t, b.FromEvaluation(eval))
	})
}
