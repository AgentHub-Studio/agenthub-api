package agentic

// RecoveryMechanismRegistry models the five query-loop recovery mechanisms
// enumerated in §4.4 of the Claude Code architecture paper
// (arXiv:2604.14228v1, "Recovery Mechanisms").
//
// §4.4: "The query loop implements several recovery mechanisms for edge cases"
// and then lists five named mechanisms, each with a distinct trigger condition,
// optional feature flag, recovery action, and composability constraint.
//
// The five mechanisms in PDF enumeration order:
//  1. Max output tokens escalation  — retries up to MAX_OUTPUT_TOKENS_RECOVERY_LIMIT=3
//  2. Reactive compaction           — gated by REACTIVE_COMPACT; fires at most once
//  3. Prompt-too-long handling      — context-collapse + reactive compact before abort
//  4. Streaming fallback            — onStreamingFallback callback; retries non-streaming
//  5. Fallback model                — fallbackModel parameter; swaps model on primary fail
//
// This registry is intentionally distinct from the BDD-level recovery_bdd_test.go
// (which asserts behavioral classifiers) and stopconditions_bdd_test.go (§4.5).
// §4.4 captures the *structural profile* of each mechanism: what triggers it,
// which flag gates it, how many retries it allows, and how it composes with
// others in the same turn.

// RecoveryMechanismID is the canonical slug for one of the five §4.4 mechanisms.
type RecoveryMechanismID string

const (
	// RecoveryMaxOutputTokensEscalation is mechanism 1 (§4.4):
	// "Max output tokens escalation: When the response hits the output token cap,
	// the system can retry with an escalated limit, subject to a GrowthBook flag
	// and the absence of an existing override or environment-variable cap. Up to
	// three recovery attempts are allowed per turn (MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3)."
	RecoveryMaxOutputTokensEscalation RecoveryMechanismID = "max_output_tokens_escalation"

	// RecoveryReactiveCompaction is mechanism 2 (§4.4):
	// "Reactive compaction (gated by REACTIVE_COMPACT): When the context is near
	// capacity, reactive compact summarizes just enough to free space. The
	// hasAttemptedReactiveCompact flag ensures this fires at most once per turn."
	RecoveryReactiveCompaction RecoveryMechanismID = "reactive_compaction"

	// RecoveryPromptTooLongHandling is mechanism 3 (§4.4):
	// "Prompt-too-long handling: If the API returns a prompt_too_long error, the
	// loop first attempts context-collapse overflow recovery and reactive compaction.
	// Only after these fail does it terminate with reason: 'prompt_too_long'."
	RecoveryPromptTooLongHandling RecoveryMechanismID = "prompt_too_long_handling"

	// RecoveryStreamingFallback is mechanism 4 (§4.4):
	// "Streaming fallback: The onStreamingFallback callback handles streaming API
	// issues, allowing the loop to retry with a different strategy."
	RecoveryStreamingFallback RecoveryMechanismID = "streaming_fallback"

	// RecoveryFallbackModel is mechanism 5 (§4.4):
	// "Fallback model: The fallbackModel parameter enables switching to an
	// alternative model if the primary model fails."
	RecoveryFallbackModel RecoveryMechanismID = "fallback_model"
)

// RecoveryMechanismProfile holds the immutable structural characteristics of
// one §4.4 recovery mechanism.
type RecoveryMechanismProfile struct {
	// ID is the canonical slug from the PDF §4.4 enumeration.
	ID RecoveryMechanismID

	// PDFSection is the exact paper section reference.
	PDFSection string

	// Label is the human-readable name used in the paper.
	Label string

	// TriggerCondition describes what API or runtime condition activates
	// this mechanism.
	TriggerCondition string

	// FeatureFlag is the GrowthBook or environment flag that gates this
	// mechanism. Empty string means the mechanism is unconditionally active
	// when its TriggerCondition fires.
	FeatureFlag string

	// MaxAttemptsPerTurn is the maximum number of times this mechanism may
	// fire within a single agent turn. Zero means unlimited (or externally
	// capped). A value of 1 means the mechanism is idempotent per turn.
	MaxAttemptsPerTurn int

	// IsIdempotentPerTurn indicates that the mechanism carries a boolean
	// guard (e.g. hasAttemptedReactiveCompact) ensuring it fires at most
	// once per turn even if the trigger recurs.
	IsIdempotentPerTurn bool

	// RecoveryAction describes the recovery action taken when triggered.
	RecoveryAction string

	// FallsBackToTermination indicates that when all recovery attempts
	// exhaust, the mechanism allows the turn to terminate with a named
	// reason rather than silently succeeding.
	FallsBackToTermination bool

	// TerminationReason is the human-readable reason emitted when this
	// mechanism exhausts. Empty if FallsBackToTermination is false.
	TerminationReason string

	// ComposabilityNote describes how this mechanism interacts with other
	// §4.4 mechanisms — specifically which other mechanisms it delegates to
	// or must run before/after.
	ComposabilityNote string

	// AgenthubMapping is the AgentHub Go source location that implements
	// the equivalent behaviour.
	AgenthubMapping string
}

// seedRecoveryMechanisms is the canonical §4.4 registry ordered by paper appearance.
var seedRecoveryMechanisms = []RecoveryMechanismProfile{
	{
		ID:                     RecoveryMaxOutputTokensEscalation,
		PDFSection:             "§4.4",
		Label:                  "Max output tokens escalation",
		TriggerCondition:       "API response hits the output token cap (finish_reason=max_tokens)",
		FeatureFlag:            "GROWTHBOOK_MAX_OUTPUT_TOKENS",
		MaxAttemptsPerTurn:     3,
		IsIdempotentPerTurn:    false,
		RecoveryAction:         "Retry with an escalated max_output_tokens limit up to MAX_OUTPUT_TOKENS_RECOVERY_LIMIT=3",
		FallsBackToTermination: true,
		TerminationReason:      "max_output_tokens_exhausted",
		ComposabilityNote:      "Runs before reactive compaction; escalation count is tracked on State.maxOutputTokensRecoveryCount",
		AgenthubMapping:        "turnstate.go: MaxOutputTokensRecoveryCount; runner.go: max-tokens recovery branch",
	},
	{
		ID:                     RecoveryReactiveCompaction,
		PDFSection:             "§4.4",
		Label:                  "Reactive compaction",
		TriggerCondition:       "Context is near capacity (context window utilisation above threshold)",
		FeatureFlag:            "REACTIVE_COMPACT",
		MaxAttemptsPerTurn:     1,
		IsIdempotentPerTurn:    true,
		RecoveryAction:         "Summarize just enough conversation to free space; does not compact the full history",
		FallsBackToTermination: false,
		TerminationReason:      "",
		ComposabilityNote:      "hasAttemptedReactiveCompact guard ensures single-fire per turn; prompt_too_long handling delegates to this mechanism as a sub-step",
		AgenthubMapping:        "compaction_pipeline_layer.go: reactive compaction trigger; autocompact.go",
	},
	{
		ID:                     RecoveryPromptTooLongHandling,
		PDFSection:             "§4.4",
		Label:                  "Prompt-too-long handling",
		TriggerCondition:       "API returns prompt_too_long error",
		FeatureFlag:            "",
		MaxAttemptsPerTurn:     0,
		IsIdempotentPerTurn:    false,
		RecoveryAction:         "Attempt context-collapse overflow recovery first, then reactive compaction; terminate with 'prompt_too_long' only if both fail",
		FallsBackToTermination: true,
		TerminationReason:      "prompt_too_long",
		ComposabilityNote:      "Composes RecoveryReactiveCompaction as a sub-step; must run context-collapse before reactive compaction per §4.4 ordering",
		AgenthubMapping:        "retry.go: isPromptTooLong + getPromptTooLongTokenGap; turnstate.go: TransitionPromptTooLong",
	},
	{
		ID:                     RecoveryStreamingFallback,
		PDFSection:             "§4.4",
		Label:                  "Streaming fallback",
		TriggerCondition:       "Streaming API call fails (e.g. 404 endpoint not found, 503 overload, stale connection)",
		FeatureFlag:            "",
		MaxAttemptsPerTurn:     1,
		IsIdempotentPerTurn:    true,
		RecoveryAction:         "onStreamingFallback callback retries the same model call using the non-streaming API variant",
		FallsBackToTermination: true,
		TerminationReason:      "streaming_unavailable",
		ComposabilityNote:      "Independent of context-management mechanisms; fires before fallback model selection",
		AgenthubMapping:        "streamfallback.go: retryStreamWithNonStreamingFallback + isStreamingFallbackEligible",
	},
	{
		ID:                     RecoveryFallbackModel,
		PDFSection:             "§4.4",
		Label:                  "Fallback model",
		TriggerCondition:       "Primary model fails with consecutive capacity/availability errors beyond threshold",
		FeatureFlag:            "",
		MaxAttemptsPerTurn:     0,
		IsIdempotentPerTurn:    false,
		RecoveryAction:         "fallbackModel parameter enables switching to an alternative model for the remainder of the run",
		FallsBackToTermination: true,
		TerminationReason:      "no_fallback_model_available",
		ComposabilityNote:      "Last-resort mechanism; only activates after streaming fallback has been attempted; requires operators to configure candidate models",
		AgenthubMapping:        "streamfallback.go: EvaluateModelFallback + FallbackDecision; retry.go: shouldFallback",
	},
}

// allRecoveryMechanisms is the lookup map built at init time.
var allRecoveryMechanisms map[RecoveryMechanismID]RecoveryMechanismProfile

func init() {
	allRecoveryMechanisms = make(map[RecoveryMechanismID]RecoveryMechanismProfile, len(seedRecoveryMechanisms))
	for _, m := range seedRecoveryMechanisms {
		allRecoveryMechanisms[m.ID] = m
	}
}

// SeedRecoveryMechanismCount is the number of §4.4 mechanisms captured in the registry.
// A compile-time-visible constant so tests can guard against silent deletions.
const SeedRecoveryMechanismCount = 5

// RecoveryMechanismRegistry provides structured queries over the §4.4 recovery
// mechanisms. All methods are pure read operations; the registry carries no
// mutable state.
type RecoveryMechanismRegistry struct{}

// NewRecoveryMechanismRegistry returns a ready-to-use registry.
func NewRecoveryMechanismRegistry() *RecoveryMechanismRegistry {
	return &RecoveryMechanismRegistry{}
}

// FindRecoveryMechanism returns the profile for the given ID.
// Returns (zero-value, false) when the ID is not in the registry.
func (r *RecoveryMechanismRegistry) FindRecoveryMechanism(id RecoveryMechanismID) (RecoveryMechanismProfile, bool) {
	m, ok := allRecoveryMechanisms[id]
	return m, ok
}

// AllRecoveryMechanisms returns all five §4.4 mechanisms in PDF enumeration order.
func (r *RecoveryMechanismRegistry) AllRecoveryMechanisms() []RecoveryMechanismProfile {
	result := make([]RecoveryMechanismProfile, len(seedRecoveryMechanisms))
	copy(result, seedRecoveryMechanisms)
	return result
}

// MechanismsWithFeatureFlag returns mechanisms that require a feature flag to
// activate. §4.4 names GROWTHBOOK_MAX_OUTPUT_TOKENS and REACTIVE_COMPACT as
// explicit gate flags.
func (r *RecoveryMechanismRegistry) MechanismsWithFeatureFlag() []RecoveryMechanismProfile {
	var result []RecoveryMechanismProfile
	for _, m := range seedRecoveryMechanisms {
		if m.FeatureFlag != "" {
			result = append(result, m)
		}
	}
	return result
}

// IdempotentPerTurnMechanisms returns mechanisms whose design enforces a
// single-fire constraint per turn via an explicit boolean guard.
// §4.4 names the hasAttemptedReactiveCompact guard for reactive compaction.
func (r *RecoveryMechanismRegistry) IdempotentPerTurnMechanisms() []RecoveryMechanismProfile {
	var result []RecoveryMechanismProfile
	for _, m := range seedRecoveryMechanisms {
		if m.IsIdempotentPerTurn {
			result = append(result, m)
		}
	}
	return result
}

// MechanismsThatTerminateOnExhaustion returns mechanisms that can produce a
// named termination reason when all recovery attempts are exhausted.
func (r *RecoveryMechanismRegistry) MechanismsThatTerminateOnExhaustion() []RecoveryMechanismProfile {
	var result []RecoveryMechanismProfile
	for _, m := range seedRecoveryMechanisms {
		if m.FallsBackToTermination {
			result = append(result, m)
		}
	}
	return result
}

// MechanismWithMaxAttemptLimit returns mechanisms that carry an explicit
// per-turn attempt ceiling (MaxAttemptsPerTurn > 0).
// §4.4 names MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3 for mechanism 1.
func (r *RecoveryMechanismRegistry) MechanismWithMaxAttemptLimit() []RecoveryMechanismProfile {
	var result []RecoveryMechanismProfile
	for _, m := range seedRecoveryMechanisms {
		if m.MaxAttemptsPerTurn > 0 {
			result = append(result, m)
		}
	}
	return result
}

// ComposingMechanisms returns mechanisms whose ComposabilityNote references
// another §4.4 mechanism as a sub-step (i.e. they delegate to another
// mechanism before deciding whether to terminate).
// §4.4: prompt_too_long handling explicitly delegates to reactive compaction.
func (r *RecoveryMechanismRegistry) ComposingMechanisms() []RecoveryMechanismProfile {
	var result []RecoveryMechanismProfile
	for _, m := range seedRecoveryMechanisms {
		// A mechanism "composes" another when its recovery action involves running
		// a sibling mechanism. The canonical signal is "Composes" appearing in the
		// ComposabilityNote.
		if containsSubstring(m.ComposabilityNote, "Composes") {
			result = append(result, m)
		}
	}
	return result
}

// MaxOutputTokensRecoveryLimit returns the PDF-specified ceiling for
// mechanism 1 (max output tokens escalation).
// §4.4: "Up to three recovery attempts are allowed per turn
// (MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3)."
func (r *RecoveryMechanismRegistry) MaxOutputTokensRecoveryLimit() int {
	p, ok := allRecoveryMechanisms[RecoveryMaxOutputTokensEscalation]
	if !ok {
		return 0
	}
	return p.MaxAttemptsPerTurn
}

// PromptTooLongTerminationReason returns the exact termination reason string
// the PDF names for the prompt_too_long handling mechanism.
func (r *RecoveryMechanismRegistry) PromptTooLongTerminationReason() string {
	p, ok := allRecoveryMechanisms[RecoveryPromptTooLongHandling]
	if !ok {
		return ""
	}
	return p.TerminationReason
}

// IsValidRecoveryMechanismID returns true if id corresponds to one of the
// five §4.4 mechanisms.
func IsValidRecoveryMechanismID(id RecoveryMechanismID) bool {
	_, ok := allRecoveryMechanisms[id]
	return ok
}

// containsSubstring is a small helper to avoid importing strings in this file.
func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
