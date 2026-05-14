package agentic

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// PERM-006 — PermissionDenied feedback.
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — when a tool call
// is blocked, the harness must hand the LLM a STRUCTURED payload (not
// a generic error string). The LLM uses the payload to decide whether
// to (a) request user confirmation, (b) retry with different input,
// (c) try an alternative tool, or (d) abandon the path entirely.
// Without this, the LLM hallucinates retries against the same denial
// or gives up too early.
//
// Distinct from existing AgentHub plumbing:
//   - errorclass.go ErrIDPermissionDenied (354) = error code emitted
//     by the tool executor when a call fails permission.
//   - analytics.go ToolErrPermissionDenied = classification bucket.
//   - HookPermissionDeniedExt = post-denial event hook (observation).
//   - permission_denied_feedback.go (this file) = the STRUCTURED
//     PAYLOAD the LLM sees in its next turn. Composes a reason +
//     retry hint + suggestion so the LLM has actionable guidance.

// PermissionDenialReason bounded enum classifies why the denial
// happened. The LLM can use the reason to pick a recovery strategy.
type PermissionDenialReason string

const (
	// PermissionDenialRuleMatch — engine matched a deny rule.
	PermissionDenialRuleMatch PermissionDenialReason = "rule_match"
	// PermissionDenialHookOverride — a PERM-005 hook overrode Allow→Deny.
	PermissionDenialHookOverride PermissionDenialReason = "hook_override"
	// PermissionDenialPrefilterDrop — pool-time pre-filter (PERM-004)
	// removed the tool; the LLM should never have called it.
	PermissionDenialPrefilterDrop PermissionDenialReason = "prefilter_drop"
	// PermissionDenialModeBlock — current PermissionMode forbids this
	// class of operation (e.g., dont_ask mode auto-denies confirm).
	PermissionDenialModeBlock PermissionDenialReason = "mode_block"
	// PermissionDenialSandboxViolation — PERM-008 shell sandbox would
	// be violated (path escape, syscall, etc).
	PermissionDenialSandboxViolation PermissionDenialReason = "sandbox_violation"
	// PermissionDenialRateLimit — rate-limit hook denied due to call
	// frequency.
	PermissionDenialRateLimit PermissionDenialReason = "rate_limit"
)

var allPermissionDenialReasons = []PermissionDenialReason{
	PermissionDenialRuleMatch, PermissionDenialHookOverride,
	PermissionDenialPrefilterDrop, PermissionDenialModeBlock,
	PermissionDenialSandboxViolation, PermissionDenialRateLimit,
}

// IsValidPermissionDenialReason returns true for the bounded set.
func IsValidPermissionDenialReason(r PermissionDenialReason) bool {
	for _, v := range allPermissionDenialReasons {
		if r == v {
			return true
		}
	}
	return false
}

// PermissionRetryHint tells the LLM whether retrying makes sense.
type PermissionRetryHint string

const (
	// PermissionRetryNotRetryable — same call will always be denied
	// (rule says no, sandbox says no, hook says no). Don't retry.
	PermissionRetryNotRetryable PermissionRetryHint = "not_retryable"
	// PermissionRetryWithDifferentInput — the deny was input-specific
	// (e.g., "SELECT * matched DROP TABLE pattern"). Try with a
	// narrower input.
	PermissionRetryWithDifferentInput PermissionRetryHint = "retry_with_different_input"
	// PermissionRetryRequestUserConfirmation — the rule would prompt;
	// ask the user explicitly before retrying.
	PermissionRetryRequestUserConfirmation PermissionRetryHint = "request_user_confirmation"
	// PermissionRetrySuggestAlternativeTool — this tool cannot be used
	// for this purpose; suggest a different tool.
	PermissionRetrySuggestAlternativeTool PermissionRetryHint = "suggest_alternative_tool"
	// PermissionRetryWaitAndRetry — rate-limited; back off and try
	// again later.
	PermissionRetryWaitAndRetry PermissionRetryHint = "wait_and_retry"
)

var allPermissionRetryHints = []PermissionRetryHint{
	PermissionRetryNotRetryable, PermissionRetryWithDifferentInput,
	PermissionRetryRequestUserConfirmation,
	PermissionRetrySuggestAlternativeTool, PermissionRetryWaitAndRetry,
}

// IsValidPermissionRetryHint returns true for the bounded set.
func IsValidPermissionRetryHint(h PermissionRetryHint) bool {
	for _, v := range allPermissionRetryHints {
		if h == v {
			return true
		}
	}
	return false
}

// PermissionDeniedFeedback is the structured payload returned to the
// LLM after a denied call. It is intentionally small and parseable so
// the LLM can switch strategies without re-reading prose.
type PermissionDeniedFeedback struct {
	ToolName            string
	ToolInput           string
	Reason              PermissionDenialReason
	RetryHint           PermissionRetryHint
	MatchedRule         string   // empty when not rule-driven
	OverridingHook      string   // empty when not hook-driven
	SuggestedAlternates []string // canonical tool names the LLM may try instead
	Explanation         string   // 1-2 sentences in natural language
}

// Validate ensures invariants the LLM and audit consumers rely on.
func (f PermissionDeniedFeedback) Validate() error {
	if strings.TrimSpace(f.ToolName) == "" {
		return ErrPermissionDeniedFeedbackToolRequired
	}
	if !IsValidPermissionDenialReason(f.Reason) {
		return fmt.Errorf("%w: %q", ErrPermissionDeniedFeedbackBadReason, f.Reason)
	}
	if !IsValidPermissionRetryHint(f.RetryHint) {
		return fmt.Errorf("%w: %q", ErrPermissionDeniedFeedbackBadRetryHint, f.RetryHint)
	}
	return nil
}

// FormatForLLM renders the feedback as a compact tagged string the LLM
// can parse without ambiguity. Format:
//   [permission_denied tool=X reason=Y retry=Z rule=A hook=B alt=C,D]
//   <explanation>
// All fields except tool/reason/retry are optional and omitted when empty.
func (f PermissionDeniedFeedback) FormatForLLM() string {
	parts := []string{
		"tool=" + f.ToolName,
		"reason=" + string(f.Reason),
		"retry=" + string(f.RetryHint),
	}
	if f.MatchedRule != "" {
		parts = append(parts, "rule="+f.MatchedRule)
	}
	if f.OverridingHook != "" {
		parts = append(parts, "hook="+f.OverridingHook)
	}
	if len(f.SuggestedAlternates) > 0 {
		alts := append([]string(nil), f.SuggestedAlternates...)
		sort.Strings(alts)
		parts = append(parts, "alt="+strings.Join(alts, ","))
	}
	header := "[permission_denied " + strings.Join(parts, " ") + "]"
	if strings.TrimSpace(f.Explanation) == "" {
		return header
	}
	return header + "\n" + f.Explanation
}

// Sentinel errors.
var (
	ErrPermissionDeniedFeedbackToolRequired = errors.New("permission denied feedback: tool name required")
	ErrPermissionDeniedFeedbackBadReason    = errors.New("permission denied feedback: invalid reason")
	ErrPermissionDeniedFeedbackBadRetryHint = errors.New("permission denied feedback: invalid retry hint")
)

// PermissionDeniedFeedbackBuilder converts internal denial signals
// (engine match, hook override, pre-filter drop) into a feedback
// payload. It is configured with an alternative-tool map and a
// hook→reason classifier.
type PermissionDeniedFeedbackBuilder struct {
	// AlternativesFor maps a denied tool name to suggested alternates.
	// Optional; empty alternates result in no `alt=` field.
	AlternativesFor map[string][]string
	// HookClassifies maps hook name → denial reason for After-phase
	// hook overrides. Defaults to PermissionDenialHookOverride.
	HookClassifies map[string]PermissionDenialReason
}

// FromEvaluation derives a feedback payload from a PERM-005
// PermissionEvaluation that resulted in Deny. Returns nil if the
// final decision was not Deny.
func (b PermissionDeniedFeedbackBuilder) FromEvaluation(eval PermissionEvaluation) *PermissionDeniedFeedback {
	if eval.FinalDecision != PermissionDeny {
		return nil
	}
	feedback := PermissionDeniedFeedback{
		ToolName:            eval.Request.ToolName,
		ToolInput:           eval.Request.ToolInput,
		SuggestedAlternates: append([]string(nil), b.AlternativesFor[eval.Request.ToolName]...),
	}

	// Classify reason + retry hint by which layer produced the deny.
	overridingHook := b.findDenyingHook(eval)
	switch {
	case overridingHook != nil:
		feedback.OverridingHook = overridingHook.HookName
		reason, ok := b.HookClassifies[overridingHook.HookName]
		if !ok {
			reason = PermissionDenialHookOverride
		}
		feedback.Reason = reason
		feedback.RetryHint = retryHintFor(reason)
		if overridingHook.Reason != "" {
			feedback.Explanation = overridingHook.Reason
		} else {
			feedback.Explanation = fmt.Sprintf("Hook %q denied this call.", overridingHook.HookName)
		}
	default:
		// Engine-driven deny.
		feedback.Reason = PermissionDenialRuleMatch
		feedback.RetryHint = PermissionRetryNotRetryable
		feedback.Explanation = fmt.Sprintf("Tool %q matched a deny rule for this tenant.", eval.Request.ToolName)
		if strings.TrimSpace(eval.Request.ToolInput) != "" {
			feedback.RetryHint = PermissionRetryWithDifferentInput
			feedback.Explanation = fmt.Sprintf("Tool %q matched a deny rule given the current input. A narrower input may pass.", eval.Request.ToolName)
		}
	}

	return &feedback
}

// FromPrefilterDrop builds feedback when the pool-time PERM-004
// pre-filter dropped a tool the LLM nonetheless attempted to call
// (e.g., a stale tool ID from a prior turn).
func (b PermissionDeniedFeedbackBuilder) FromPrefilterDrop(toolName, toolInput string) *PermissionDeniedFeedback {
	return &PermissionDeniedFeedback{
		ToolName:            toolName,
		ToolInput:           toolInput,
		Reason:              PermissionDenialPrefilterDrop,
		RetryHint:           PermissionRetrySuggestAlternativeTool,
		SuggestedAlternates: append([]string(nil), b.AlternativesFor[toolName]...),
		Explanation:        fmt.Sprintf("Tool %q is not currently available in the pool. Try an alternative.", toolName),
	}
}

// FromModeBlock builds feedback when the PermissionMode (e.g.,
// dont_ask) blocked a call that would normally prompt.
func (b PermissionDeniedFeedbackBuilder) FromModeBlock(toolName, toolInput string, mode PermissionMode) *PermissionDeniedFeedback {
	return &PermissionDeniedFeedback{
		ToolName:            toolName,
		ToolInput:           toolInput,
		Reason:              PermissionDenialModeBlock,
		RetryHint:           PermissionRetryRequestUserConfirmation,
		SuggestedAlternates: append([]string(nil), b.AlternativesFor[toolName]...),
		Explanation:        fmt.Sprintf("Session is in %q mode; this tool requires explicit confirmation.", mode),
	}
}

// FromRateLimit builds feedback when a rate-limit hook denied the call.
func (b PermissionDeniedFeedbackBuilder) FromRateLimit(toolName, toolInput, hookName, detail string) *PermissionDeniedFeedback {
	return &PermissionDeniedFeedback{
		ToolName:            toolName,
		ToolInput:           toolInput,
		Reason:              PermissionDenialRateLimit,
		RetryHint:           PermissionRetryWaitAndRetry,
		OverridingHook:      hookName,
		SuggestedAlternates: append([]string(nil), b.AlternativesFor[toolName]...),
		Explanation:        detail,
	}
}

// FromSandboxViolation builds feedback when PERM-008 sandbox would be
// violated.
func (b PermissionDeniedFeedbackBuilder) FromSandboxViolation(toolName, toolInput, violation string) *PermissionDeniedFeedback {
	return &PermissionDeniedFeedback{
		ToolName:            toolName,
		ToolInput:           toolInput,
		Reason:              PermissionDenialSandboxViolation,
		RetryHint:           PermissionRetryWithDifferentInput,
		SuggestedAlternates: append([]string(nil), b.AlternativesFor[toolName]...),
		Explanation:        fmt.Sprintf("Sandbox violation: %s", violation),
	}
}

// findDenyingHook scans the After-phase decisions for an override_deny
// outcome. Returns nil if no hook denied (engine drove the decision).
func (b PermissionDeniedFeedbackBuilder) findDenyingHook(eval PermissionEvaluation) *PermissionHookDecision {
	// Iterate in reverse so the LAST overriding hook wins (matches
	// PermissionHookChain semantics).
	for i := len(eval.HookDecisions) - 1; i >= 0; i-- {
		d := eval.HookDecisions[i]
		if d.Outcome == PermissionHookOverrideDeny {
			out := d
			return &out
		}
	}
	return nil
}

// retryHintFor picks the default retry hint per denial reason.
func retryHintFor(reason PermissionDenialReason) PermissionRetryHint {
	switch reason {
	case PermissionDenialRuleMatch, PermissionDenialHookOverride,
		PermissionDenialSandboxViolation:
		return PermissionRetryNotRetryable
	case PermissionDenialPrefilterDrop:
		return PermissionRetrySuggestAlternativeTool
	case PermissionDenialModeBlock:
		return PermissionRetryRequestUserConfirmation
	case PermissionDenialRateLimit:
		return PermissionRetryWaitAndRetry
	}
	return PermissionRetryNotRetryable
}
