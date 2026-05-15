package agentic

// LoopStopConditionRegistry models the five named stop conditions that can
// terminate the Claude Code agent loop, as enumerated in §4.5 ("Stop
// Conditions") of arXiv:2604.14228v1.
//
// §4.5 states: "Multiple conditions can terminate the loop:" and then
// lists five numbered items with concise descriptions.
//
// The five conditions in PDF enumeration order:
//  1. no_tool_use       — model produces only text content (primary stop)
//  2. max_turns         — configurable maxTurns limit is reached
//  3. context_overflow  — API returns prompt_too_long
//  4. hook_intervention — PostToolUse hook sets hook_stopped_continuation
//  5. explicit_abort    — abortController signal fires
//
// This registry is intentionally distinct from the §4.4 recovery mechanisms
// (recovery_mechanism_registry.go) and from stopconditions_bdd_test.go
// (which tests runtime classifier behaviour). The registry models the
// *structural profile* of each condition: its category, trigger source,
// recoverability, and AgentHub mapping.
//
// Relationship to existing code:
//   - query_pipeline.go references QueryStepStopCondition (§4.5 evaluation step)
//   - stophooks.go models hook mechanics that trigger condition 4
//   - recovery_mechanism_registry.go models the §4.4 layer that may run BEFORE
//     conditions 3 and 5 are raised as permanent stops.
//   - turnstate.go uses TransitionContextOverflow for condition 3 transitions.

// LoopStopConditionID is the canonical slug for one of the five §4.5 stop conditions.
type LoopStopConditionID string

const (
	// LoopStopNoToolUse is condition 1 (§4.5):
	// "No tool use: The model produces only text content (the primary stop
	// condition)."
	// This is the normal, healthy completion path — the model decided it is done.
	LoopStopNoToolUse LoopStopConditionID = "no_tool_use"

	// LoopStopMaxTurns is condition 2 (§4.5):
	// "Max turns: The configurable maxTurns limit is reached."
	// The harness imposes an upper bound on loop iterations.
	LoopStopMaxTurns LoopStopConditionID = "max_turns"

	// LoopStopContextOverflow is condition 3 (§4.5):
	// "Context overflow: The API returns prompt_too_long."
	// §4.4 recovery mechanisms (reactive compaction, context-collapse) may be
	// attempted first; this condition fires only after they are exhausted.
	LoopStopContextOverflow LoopStopConditionID = "context_overflow"

	// LoopStopHookIntervention is condition 4 (§4.5):
	// "Hook intervention: A PostToolUse hook sets hook_stopped_continuation."
	// A registered hook asserted that the loop must not continue after the
	// current tool result was collected.
	LoopStopHookIntervention LoopStopConditionID = "hook_intervention"

	// LoopStopExplicitAbort is condition 5 (§4.5):
	// "Explicit abort: The abortController signal fires."
	// The host process (CLI, coordinator, or user interrupt) cancelled execution
	// via the AbortController / OS signal.
	LoopStopExplicitAbort LoopStopConditionID = "explicit_abort"
)

// loopStopConditionOrder is the canonical §4.5 enumeration order (1..5).
var loopStopConditionOrder = []LoopStopConditionID{
	LoopStopNoToolUse,
	LoopStopMaxTurns,
	LoopStopContextOverflow,
	LoopStopHookIntervention,
	LoopStopExplicitAbort,
}

// LoopStopCategory classifies a stop condition's nature.
// "normal_completion" means the loop ended as expected.
// "resource_limit" means a configured ceiling was reached.
// "error" means the API or environment raised an unrecoverable error.
// "user_or_harness_action" means an external actor cancelled the loop.
type LoopStopCategory string

const (
	LoopStopCategoryNormalCompletion    LoopStopCategory = "normal_completion"
	LoopStopCategoryResourceLimit       LoopStopCategory = "resource_limit"
	LoopStopCategoryError               LoopStopCategory = "error"
	LoopStopCategoryUserOrHarnessAction LoopStopCategory = "user_or_harness_action"
)

// LoopStopTriggerSource identifies what detected and raised the condition.
// "model_output" — the model's own response content triggered the check.
// "harness"      — the loop harness evaluated a limit or configuration value.
// "api_error"    — the Anthropic API returned an error response.
// "hook"         — a registered hook script emitted a stop signal.
// "os_signal"    — an OS-level signal or AbortController fired.
type LoopStopTriggerSource string

const (
	LoopStopTriggerModelOutput LoopStopTriggerSource = "model_output"
	LoopStopTriggerHarness     LoopStopTriggerSource = "harness"
	LoopStopTriggerAPIError    LoopStopTriggerSource = "api_error"
	LoopStopTriggerHook        LoopStopTriggerSource = "hook"
	LoopStopTriggerOSSignal    LoopStopTriggerSource = "os_signal"
)

// LoopStopConditionProfile holds the immutable structural characteristics of
// one §4.5 stop condition.
type LoopStopConditionProfile struct {
	// ID is the canonical slug matching the §4.5 enumeration.
	ID LoopStopConditionID

	// EnumerationOrder is the 1-based position in the §4.5 numbered list.
	EnumerationOrder int

	// PDFSection is the exact paper section reference (always "4.5").
	PDFSection string

	// Label is the human-readable name used verbatim in §4.5.
	Label string

	// Description is the paper's own concise description of this condition.
	Description string

	// Category classifies the nature of this stop (normal / limit / error / external).
	Category LoopStopCategory

	// TriggerSource identifies what entity detects and raises this condition.
	TriggerSource LoopStopTriggerSource

	// IsPrimaryStopCondition is true only for LoopStopNoToolUse, which the
	// paper explicitly labels "(the primary stop condition)".
	IsPrimaryStopCondition bool

	// IsRecoverable indicates whether §4.4 recovery mechanisms may be
	// attempted before this condition becomes a definitive stop.
	// true for context_overflow (reactive compaction + context-collapse run first).
	// false for all others.
	IsRecoverable bool

	// RecoveryMechanismSlugs names the §4.4 recovery mechanisms tried before
	// this condition finalises. Empty for non-recoverable conditions.
	// Values match RecoveryMechanismID constants in recovery_mechanism_registry.go.
	RecoveryMechanismSlugs []string

	// NotifiesParentAgent is true when the stop propagates a result upward
	// to a parent agent context (subagent scenarios). Explicit abort and
	// hook intervention both propagate; normal completion may return a summary.
	NotifiesParentAgent bool

	// RuntimeSignal is the Go-land / TypeScript-land signal or flag associated
	// with detecting this condition at runtime.
	// e.g. "hook_stopped_continuation", "prompt_too_long", "AbortController",
	// or "" for conditions detected by structural inspection (no_tool_use, max_turns).
	RuntimeSignal string

	// AgenthubMapping describes how the AgentHub runner maps this condition.
	// e.g. the AgentHub SSE event or runner state that corresponds to this stop.
	AgenthubMapping string
}

// allLoopStopConditionProfiles is the authoritative §4.5 condition data.
var allLoopStopConditionProfiles = map[LoopStopConditionID]LoopStopConditionProfile{
	LoopStopNoToolUse: {
		ID:                     LoopStopNoToolUse,
		EnumerationOrder:       1,
		PDFSection:             "4.5",
		Label:                  "No tool use",
		Description:            "The model produces only text content (the primary stop condition).",
		Category:               LoopStopCategoryNormalCompletion,
		TriggerSource:          LoopStopTriggerModelOutput,
		IsPrimaryStopCondition: true,
		IsRecoverable:          false,
		RecoveryMechanismSlugs: nil,
		NotifiesParentAgent:    true,
		RuntimeSignal:          "",
		AgenthubMapping:        "run_complete (stop_reason=end_turn)",
	},
	LoopStopMaxTurns: {
		ID:                     LoopStopMaxTurns,
		EnumerationOrder:       2,
		PDFSection:             "4.5",
		Label:                  "Max turns",
		Description:            "The configurable maxTurns limit is reached.",
		Category:               LoopStopCategoryResourceLimit,
		TriggerSource:          LoopStopTriggerHarness,
		IsPrimaryStopCondition: false,
		IsRecoverable:          false,
		RecoveryMechanismSlugs: nil,
		NotifiesParentAgent:    true,
		RuntimeSignal:          "",
		AgenthubMapping:        "run_complete (stop_reason=max_turns)",
	},
	LoopStopContextOverflow: {
		ID:                     LoopStopContextOverflow,
		EnumerationOrder:       3,
		PDFSection:             "4.5",
		Label:                  "Context overflow",
		Description:            "The API returns prompt_too_long.",
		Category:               LoopStopCategoryError,
		TriggerSource:          LoopStopTriggerAPIError,
		IsPrimaryStopCondition: false,
		IsRecoverable:          true,
		RecoveryMechanismSlugs: []string{"prompt_too_long_handling", "reactive_compaction"},
		NotifiesParentAgent:    true,
		RuntimeSignal:          "prompt_too_long",
		AgenthubMapping:        "run_complete (stop_reason=context_overflow)",
	},
	LoopStopHookIntervention: {
		ID:                     LoopStopHookIntervention,
		EnumerationOrder:       4,
		PDFSection:             "4.5",
		Label:                  "Hook intervention",
		Description:            "A PostToolUse hook sets hook_stopped_continuation.",
		Category:               LoopStopCategoryUserOrHarnessAction,
		TriggerSource:          LoopStopTriggerHook,
		IsPrimaryStopCondition: false,
		IsRecoverable:          false,
		RecoveryMechanismSlugs: nil,
		NotifiesParentAgent:    true,
		RuntimeSignal:          "hook_stopped_continuation",
		AgenthubMapping:        "run_complete (stop_reason=hook_intervention)",
	},
	LoopStopExplicitAbort: {
		ID:                     LoopStopExplicitAbort,
		EnumerationOrder:       5,
		PDFSection:             "4.5",
		Label:                  "Explicit abort",
		Description:            "The abortController signal fires.",
		Category:               LoopStopCategoryUserOrHarnessAction,
		TriggerSource:          LoopStopTriggerOSSignal,
		IsPrimaryStopCondition: false,
		IsRecoverable:          false,
		RecoveryMechanismSlugs: nil,
		NotifiesParentAgent:    false,
		RuntimeSignal:          "AbortController",
		AgenthubMapping:        "run_complete (stop_reason=abort)",
	},
}

// SeedLoopStopConditionCount is the number of stop conditions defined in §4.5 (always 5).
const SeedLoopStopConditionCount = 5

// LoopStopConditionRegistry provides typed, query-oriented access to the five
// §4.5 stop conditions.
type LoopStopConditionRegistry struct{}

// NewLoopStopConditionRegistry returns a ready-to-use registry.
func NewLoopStopConditionRegistry() *LoopStopConditionRegistry {
	return &LoopStopConditionRegistry{}
}

// Profile returns the §4.5 profile for the given condition ID.
// Returns (zero-value, false) if id is not one of the five canonical values.
func (r *LoopStopConditionRegistry) Profile(id LoopStopConditionID) (LoopStopConditionProfile, bool) {
	p, ok := allLoopStopConditionProfiles[id]
	return p, ok
}

// AllConditions returns all five profiles in §4.5 enumeration order (1..5).
// The result is a defensive copy.
func (r *LoopStopConditionRegistry) AllConditions() []LoopStopConditionProfile {
	result := make([]LoopStopConditionProfile, 0, len(loopStopConditionOrder))
	for _, id := range loopStopConditionOrder {
		result = append(result, allLoopStopConditionProfiles[id])
	}
	return result
}

// Count returns the number of registered stop conditions (always 5 per §4.5).
func (r *LoopStopConditionRegistry) Count() int {
	return len(loopStopConditionOrder)
}

// IsValidConditionID returns true iff id is one of the five §4.5 condition slugs.
func (r *LoopStopConditionRegistry) IsValidConditionID(id LoopStopConditionID) bool {
	_, ok := allLoopStopConditionProfiles[id]
	return ok
}

// PrimaryStopCondition returns the profile of the condition designated as the
// primary stop condition in §4.5. The paper labels LoopStopNoToolUse "(the
// primary stop condition)".
func (r *LoopStopConditionRegistry) PrimaryStopCondition() LoopStopConditionProfile {
	return allLoopStopConditionProfiles[LoopStopNoToolUse]
}

// ConditionsByCategory returns all conditions whose Category matches the given
// category. The returned slice preserves §4.5 enumeration order.
func (r *LoopStopConditionRegistry) ConditionsByCategory(cat LoopStopCategory) []LoopStopConditionProfile {
	var result []LoopStopConditionProfile
	for _, id := range loopStopConditionOrder {
		if p := allLoopStopConditionProfiles[id]; p.Category == cat {
			result = append(result, p)
		}
	}
	return result
}

// ConditionsByTriggerSource returns all conditions whose TriggerSource matches
// the given source. The returned slice preserves §4.5 enumeration order.
func (r *LoopStopConditionRegistry) ConditionsByTriggerSource(src LoopStopTriggerSource) []LoopStopConditionProfile {
	var result []LoopStopConditionProfile
	for _, id := range loopStopConditionOrder {
		if p := allLoopStopConditionProfiles[id]; p.TriggerSource == src {
			result = append(result, p)
		}
	}
	return result
}

// RecoverableConditions returns the subset of conditions for which §4.4
// recovery mechanisms may be attempted before the stop becomes definitive.
// Per §4.5 only context_overflow is recoverable.
func (r *LoopStopConditionRegistry) RecoverableConditions() []LoopStopConditionProfile {
	var result []LoopStopConditionProfile
	for _, id := range loopStopConditionOrder {
		if p := allLoopStopConditionProfiles[id]; p.IsRecoverable {
			result = append(result, p)
		}
	}
	return result
}

// ConditionByEnumerationOrder returns the profile for the given 1-based
// §4.5 enumeration position (1..5).
// Returns (zero-value, false) if order is out of range.
func (r *LoopStopConditionRegistry) ConditionByEnumerationOrder(order int) (LoopStopConditionProfile, bool) {
	if order < 1 || order > len(loopStopConditionOrder) {
		return LoopStopConditionProfile{}, false
	}
	id := loopStopConditionOrder[order-1]
	return allLoopStopConditionProfiles[id], true
}

// ConditionByRuntimeSignal looks up a profile by its RuntimeSignal field.
// Returns (zero-value, false) if no condition carries that signal.
// Conditions without a RuntimeSignal (empty string) are never matched.
func (r *LoopStopConditionRegistry) ConditionByRuntimeSignal(signal string) (LoopStopConditionProfile, bool) {
	if signal == "" {
		return LoopStopConditionProfile{}, false
	}
	for _, id := range loopStopConditionOrder {
		if p := allLoopStopConditionProfiles[id]; p.RuntimeSignal == signal {
			return p, true
		}
	}
	return LoopStopConditionProfile{}, false
}

// ExternallyTriggeredConditions returns conditions whose stop is initiated
// by an actor outside the model itself (harness, hook, OS signal).
// Excludes LoopStopNoToolUse (model_output) and LoopStopContextOverflow (api_error).
func (r *LoopStopConditionRegistry) ExternallyTriggeredConditions() []LoopStopConditionProfile {
	external := map[LoopStopTriggerSource]bool{
		LoopStopTriggerHarness:  true,
		LoopStopTriggerHook:     true,
		LoopStopTriggerOSSignal: true,
	}
	var result []LoopStopConditionProfile
	for _, id := range loopStopConditionOrder {
		if p := allLoopStopConditionProfiles[id]; external[p.TriggerSource] {
			result = append(result, p)
		}
	}
	return result
}

// ConditionsNotifyingParentAgent returns conditions that propagate a result
// upward to a parent agent context (subagent scenarios).
func (r *LoopStopConditionRegistry) ConditionsNotifyingParentAgent() []LoopStopConditionProfile {
	var result []LoopStopConditionProfile
	for _, id := range loopStopConditionOrder {
		if p := allLoopStopConditionProfiles[id]; p.NotifiesParentAgent {
			result = append(result, p)
		}
	}
	return result
}

// AllEnumerationOrdersAreContiguous validates that the five profiles carry
// strictly increasing, gapless EnumerationOrder values 1..5. This is a
// compile-time-equivalent structural invariant for the §4.5 registry.
func LoopStopEnumerationOrdersAreContiguous() bool {
	for i, id := range loopStopConditionOrder {
		if allLoopStopConditionProfiles[id].EnumerationOrder != i+1 {
			return false
		}
	}
	return true
}

// LoopStopAllPDFSectionsAreCanonical validates that every profile carries
// the canonical PDF section "4.5".
func LoopStopAllPDFSectionsAreCanonical() bool {
	for _, id := range loopStopConditionOrder {
		if allLoopStopConditionProfiles[id].PDFSection != "4.5" {
			return false
		}
	}
	return true
}
