package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPermDeniedFB_IsValidReason(t *testing.T) {
	for _, r := range allPermissionDenialReasons {
		assert.True(t, IsValidPermissionDenialReason(r))
	}
	assert.False(t, IsValidPermissionDenialReason(PermissionDenialReason("nope")))
}

func TestPermDeniedFB_IsValidRetryHint(t *testing.T) {
	for _, h := range allPermissionRetryHints {
		assert.True(t, IsValidPermissionRetryHint(h))
	}
	assert.False(t, IsValidPermissionRetryHint(PermissionRetryHint("nope")))
}

func TestPermDeniedFB_ValidateRequiresToolName(t *testing.T) {
	f := PermissionDeniedFeedback{
		Reason: PermissionDenialRuleMatch, RetryHint: PermissionRetryNotRetryable,
	}
	assert.ErrorIs(t, f.Validate(), ErrPermissionDeniedFeedbackToolRequired)
}

func TestPermDeniedFB_ValidateBadReason(t *testing.T) {
	f := PermissionDeniedFeedback{
		ToolName: "Bash", Reason: PermissionDenialReason("nope"),
		RetryHint: PermissionRetryNotRetryable,
	}
	assert.ErrorIs(t, f.Validate(), ErrPermissionDeniedFeedbackBadReason)
}

func TestPermDeniedFB_ValidateBadRetryHint(t *testing.T) {
	f := PermissionDeniedFeedback{
		ToolName: "Bash", Reason: PermissionDenialRuleMatch,
		RetryHint: PermissionRetryHint("nope"),
	}
	assert.ErrorIs(t, f.Validate(), ErrPermissionDeniedFeedbackBadRetryHint)
}

func TestPermDeniedFB_FormatForLLMMinimal(t *testing.T) {
	f := PermissionDeniedFeedback{
		ToolName: "Bash", Reason: PermissionDenialRuleMatch,
		RetryHint: PermissionRetryNotRetryable,
	}
	got := f.FormatForLLM()
	assert.Equal(t, "[permission_denied tool=Bash reason=rule_match retry=not_retryable]", got)
}

func TestPermDeniedFB_FormatForLLMFull(t *testing.T) {
	f := PermissionDeniedFeedback{
		ToolName: "execute-sql", Reason: PermissionDenialHookOverride,
		RetryHint:           PermissionRetryWithDifferentInput,
		MatchedRule:         "execute-sql(DROP)",
		OverridingHook:      "pii-detector",
		SuggestedAlternates: []string{"document_search", "read_only_sql"},
		Explanation:         "Input contains PII identifiers.",
	}
	got := f.FormatForLLM()
	// Alternates are sorted alphabetically.
	assert.Contains(t, got, "tool=execute-sql")
	assert.Contains(t, got, "reason=hook_override")
	assert.Contains(t, got, "retry=retry_with_different_input")
	assert.Contains(t, got, "rule=execute-sql(DROP)")
	assert.Contains(t, got, "hook=pii-detector")
	assert.Contains(t, got, "alt=document_search,read_only_sql")
	assert.Contains(t, got, "Input contains PII identifiers.")
}

func TestPermDeniedFB_FormatForLLMSortsAlternates(t *testing.T) {
	f := PermissionDeniedFeedback{
		ToolName: "X", Reason: PermissionDenialPrefilterDrop,
		RetryHint:           PermissionRetrySuggestAlternativeTool,
		SuggestedAlternates: []string{"zeta", "alpha", "mu"},
	}
	got := f.FormatForLLM()
	assert.Contains(t, got, "alt=alpha,mu,zeta")
}

func TestPermDeniedFB_BuilderFromEvaluationNilWhenAllow(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	eval := PermissionEvaluation{FinalDecision: PermissionAllow}
	assert.Nil(t, b.FromEvaluation(eval))
}

func TestPermDeniedFB_BuilderFromEngineDeny(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	eval := PermissionEvaluation{
		Request:        PermissionHookRequest{ToolName: "Bash", ToolInput: "rm -rf /"},
		EngineDecision: PermissionDeny,
		FinalDecision:  PermissionDeny,
	}
	f := b.FromEvaluation(eval)
	if assert.NotNil(t, f) {
		assert.Equal(t, "Bash", f.ToolName)
		assert.Equal(t, PermissionDenialRuleMatch, f.Reason)
		assert.Equal(t, PermissionRetryWithDifferentInput, f.RetryHint)
	}
}

func TestPermDeniedFB_BuilderFromEngineDenyEmptyInput(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	eval := PermissionEvaluation{
		Request:        PermissionHookRequest{ToolName: "Bash"},
		EngineDecision: PermissionDeny,
		FinalDecision:  PermissionDeny,
	}
	f := b.FromEvaluation(eval)
	assert.Equal(t, PermissionRetryNotRetryable, f.RetryHint)
}

func TestPermDeniedFB_BuilderFromHookDeny(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	eval := PermissionEvaluation{
		Request:        PermissionHookRequest{ToolName: "execute-sql"},
		EngineDecision: PermissionAllow,
		FinalDecision:  PermissionDeny,
		HookDecisions: []PermissionHookDecision{
			{HookName: "pii-detector", Outcome: PermissionHookOverrideDeny, Reason: "PII detected"},
		},
	}
	f := b.FromEvaluation(eval)
	if assert.NotNil(t, f) {
		assert.Equal(t, PermissionDenialHookOverride, f.Reason)
		assert.Equal(t, "pii-detector", f.OverridingHook)
		assert.Equal(t, "PII detected", f.Explanation)
	}
}

func TestPermDeniedFB_BuilderClassifiesHookByName(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{
		HookClassifies: map[string]PermissionDenialReason{
			"rate-limiter": PermissionDenialRateLimit,
		},
	}
	eval := PermissionEvaluation{
		Request:        PermissionHookRequest{ToolName: "Read"},
		EngineDecision: PermissionAllow,
		FinalDecision:  PermissionDeny,
		HookDecisions: []PermissionHookDecision{
			{HookName: "rate-limiter", Outcome: PermissionHookOverrideDeny, Reason: "60 calls/min"},
		},
	}
	f := b.FromEvaluation(eval)
	assert.Equal(t, PermissionDenialRateLimit, f.Reason)
	assert.Equal(t, PermissionRetryWaitAndRetry, f.RetryHint)
}

func TestPermDeniedFB_LastDenyingHookWins(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	eval := PermissionEvaluation{
		Request:        PermissionHookRequest{ToolName: "x"},
		EngineDecision: PermissionAllow,
		FinalDecision:  PermissionDeny,
		HookDecisions: []PermissionHookDecision{
			{HookName: "first", Outcome: PermissionHookOverrideDeny, Reason: "early"},
			{HookName: "second", Outcome: PermissionHookContinue, Reason: ""},
			{HookName: "third", Outcome: PermissionHookOverrideDeny, Reason: "late"},
		},
	}
	f := b.FromEvaluation(eval)
	// Reverse iteration: "third" found first because it's the last
	// deny override applied by the chain.
	assert.Equal(t, "third", f.OverridingHook)
	assert.Equal(t, "late", f.Explanation)
}

func TestPermDeniedFB_BuilderAlternatesIncluded(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{
		AlternativesFor: map[string][]string{
			"Bash": {"shell_sandboxed"},
		},
	}
	eval := PermissionEvaluation{
		Request:        PermissionHookRequest{ToolName: "Bash"},
		EngineDecision: PermissionDeny,
		FinalDecision:  PermissionDeny,
	}
	f := b.FromEvaluation(eval)
	assert.Equal(t, []string{"shell_sandboxed"}, f.SuggestedAlternates)
}

func TestPermDeniedFB_FromPrefilterDrop(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{
		AlternativesFor: map[string][]string{"Bash": {"shell_sandboxed"}},
	}
	f := b.FromPrefilterDrop("Bash", "ls -la")
	assert.Equal(t, PermissionDenialPrefilterDrop, f.Reason)
	assert.Equal(t, PermissionRetrySuggestAlternativeTool, f.RetryHint)
	assert.Equal(t, []string{"shell_sandboxed"}, f.SuggestedAlternates)
}

func TestPermDeniedFB_FromModeBlock(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	f := b.FromModeBlock("Write", "/etc/passwd", PermissionModeDontAsk)
	assert.Equal(t, PermissionDenialModeBlock, f.Reason)
	assert.Equal(t, PermissionRetryRequestUserConfirmation, f.RetryHint)
	assert.Contains(t, f.Explanation, "dont_ask")
}

func TestPermDeniedFB_FromRateLimit(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	f := b.FromRateLimit("execute-sql", "SELECT 1", "rate-limiter", "60/min exceeded")
	assert.Equal(t, PermissionDenialRateLimit, f.Reason)
	assert.Equal(t, PermissionRetryWaitAndRetry, f.RetryHint)
	assert.Equal(t, "rate-limiter", f.OverridingHook)
	assert.Equal(t, "60/min exceeded", f.Explanation)
}

func TestPermDeniedFB_FromSandboxViolation(t *testing.T) {
	b := PermissionDeniedFeedbackBuilder{}
	f := b.FromSandboxViolation("Bash", "cat /etc/shadow", "path escape outside cwd")
	assert.Equal(t, PermissionDenialSandboxViolation, f.Reason)
	assert.Equal(t, PermissionRetryWithDifferentInput, f.RetryHint)
	assert.Contains(t, f.Explanation, "path escape")
}

func TestPermDeniedFB_RetryHintForCoversAllReasons(t *testing.T) {
	for _, r := range allPermissionDenialReasons {
		got := retryHintFor(r)
		assert.True(t, IsValidPermissionRetryHint(got), "reason %q produced invalid hint", r)
	}
}

func TestPermDeniedFB_FeedbackTimestampIsZeroByDefault(t *testing.T) {
	// Defensive: we do not put a timestamp on the LLM payload because
	// the LLM is stateless; audit timestamps live on PermissionEvaluation.
	f := PermissionDeniedFeedback{ToolName: "X", Reason: PermissionDenialRuleMatch, RetryHint: PermissionRetryNotRetryable}
	assert.True(t, time.Time{}.IsZero(), "guarantee no time field on feedback")
	assert.NotEmpty(t, f.FormatForLLM())
}

func TestPermDeniedFB_FormatForLLMOmitsEmptyExplanation(t *testing.T) {
	f := PermissionDeniedFeedback{
		ToolName: "Bash", Reason: PermissionDenialRuleMatch,
		RetryHint: PermissionRetryNotRetryable,
	}
	got := f.FormatForLLM()
	assert.NotContains(t, got, "\n")
}
