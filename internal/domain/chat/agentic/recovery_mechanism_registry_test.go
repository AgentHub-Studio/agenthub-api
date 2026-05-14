package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for RecoveryMechanismRegistry (FEAT-034).
//
// These tests assert the structural registry introduced in §4.4 of the Claude
// Code architecture paper (arXiv:2604.14228v1, "Recovery Mechanisms").
// They are intentionally separate from the BDD-level recovery_bdd_test.go,
// which asserts runtime classifier and parser behaviour. These tests verify
// the immutable profile data and all query methods on the registry.

// ---------------------------------------------------------------------------
// Seed integrity
// ---------------------------------------------------------------------------

func TestFEAT034_SeedCount_IsFive(t *testing.T) {
	// §4.4 enumerates exactly five named mechanisms. A deletion or addition
	// breaks the paper mapping.
	assert.Equal(t, 5, SeedRecoveryMechanismCount,
		"SeedRecoveryMechanismCount const must equal 5 per §4.4")
	reg := NewRecoveryMechanismRegistry()
	assert.Len(t, reg.AllRecoveryMechanisms(), 5,
		"AllRecoveryMechanisms must return exactly 5 profiles")
}

func TestFEAT034_AllMechanismIDsAreDistinct(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	seen := make(map[RecoveryMechanismID]bool)
	for _, m := range reg.AllRecoveryMechanisms() {
		assert.False(t, seen[m.ID],
			"duplicate RecoveryMechanismID %q must not exist in seed data", m.ID)
		seen[m.ID] = true
	}
}

func TestFEAT034_AllMechanismsHaveNonEmptyLabel(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	for _, m := range reg.AllRecoveryMechanisms() {
		assert.NotEmpty(t, m.Label,
			"every §4.4 mechanism must have a human-readable Label; empty for ID=%q", m.ID)
	}
}

func TestFEAT034_AllMechanismsHavePDFSection(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	for _, m := range reg.AllRecoveryMechanisms() {
		assert.Equal(t, "§4.4", m.PDFSection,
			"all mechanisms in this registry are from §4.4; wrong section for ID=%q", m.ID)
	}
}

func TestFEAT034_AllMechanismsHaveNonEmptyTriggerCondition(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	for _, m := range reg.AllRecoveryMechanisms() {
		assert.NotEmpty(t, m.TriggerCondition,
			"every mechanism must describe what triggers it; empty for ID=%q", m.ID)
	}
}

func TestFEAT034_AllMechanismsHaveNonEmptyRecoveryAction(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	for _, m := range reg.AllRecoveryMechanisms() {
		assert.NotEmpty(t, m.RecoveryAction,
			"every mechanism must describe its recovery action; empty for ID=%q", m.ID)
	}
}

func TestFEAT034_AllMechanismsHaveNonEmptyAgenthubMapping(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	for _, m := range reg.AllRecoveryMechanisms() {
		assert.NotEmpty(t, m.AgenthubMapping,
			"every mechanism must map to an AgentHub Go source; empty for ID=%q", m.ID)
	}
}

// ---------------------------------------------------------------------------
// IsValidRecoveryMechanismID
// ---------------------------------------------------------------------------

func TestFEAT034_IsValidRecoveryMechanismID_AcceptsAllFiveIDs(t *testing.T) {
	ids := []RecoveryMechanismID{
		RecoveryMaxOutputTokensEscalation,
		RecoveryReactiveCompaction,
		RecoveryPromptTooLongHandling,
		RecoveryStreamingFallback,
		RecoveryFallbackModel,
	}
	for _, id := range ids {
		assert.Truef(t, IsValidRecoveryMechanismID(id),
			"IsValidRecoveryMechanismID must accept known ID %q", id)
	}
}

func TestFEAT034_IsValidRecoveryMechanismID_RejectsUnknown(t *testing.T) {
	assert.False(t, IsValidRecoveryMechanismID("unknown_mechanism"),
		"IsValidRecoveryMechanismID must reject unknown slugs")
	assert.False(t, IsValidRecoveryMechanismID(""),
		"IsValidRecoveryMechanismID must reject empty string")
}

// ---------------------------------------------------------------------------
// FindRecoveryMechanism
// ---------------------------------------------------------------------------

func TestFEAT034_FindRecoveryMechanism_ReturnsProfileForKnownID(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	p, ok := reg.FindRecoveryMechanism(RecoveryMaxOutputTokensEscalation)
	require.True(t, ok, "FindRecoveryMechanism must find max_output_tokens_escalation")
	assert.Equal(t, RecoveryMaxOutputTokensEscalation, p.ID)
	assert.Equal(t, 3, p.MaxAttemptsPerTurn,
		"§4.4: MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3")
}

func TestFEAT034_FindRecoveryMechanism_ReturnsFalseForUnknownID(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	_, ok := reg.FindRecoveryMechanism("nonexistent")
	assert.False(t, ok, "FindRecoveryMechanism must return false for unknown IDs")
}

// ---------------------------------------------------------------------------
// Feature flag queries
// ---------------------------------------------------------------------------

func TestFEAT034_MechanismsWithFeatureFlag_ReturnsTwoMechanisms(t *testing.T) {
	// §4.4 names GROWTHBOOK_MAX_OUTPUT_TOKENS (mechanism 1) and
	// REACTIVE_COMPACT (mechanism 2) as explicit feature flags.
	reg := NewRecoveryMechanismRegistry()
	flagged := reg.MechanismsWithFeatureFlag()
	assert.Len(t, flagged, 2,
		"exactly 2 mechanisms are feature-flag-gated per §4.4")
}

func TestFEAT034_MechanismsWithFeatureFlag_IncludesReactiveCompaction(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	flagged := reg.MechanismsWithFeatureFlag()
	ids := make(map[RecoveryMechanismID]bool)
	for _, m := range flagged {
		ids[m.ID] = true
	}
	assert.True(t, ids[RecoveryReactiveCompaction],
		"reactive_compaction must appear in feature-flag-gated list (REACTIVE_COMPACT)")
}

// ---------------------------------------------------------------------------
// Idempotent per-turn
// ---------------------------------------------------------------------------

func TestFEAT034_IdempotentPerTurnMechanisms_IncludesReactiveCompaction(t *testing.T) {
	// §4.4: "The hasAttemptedReactiveCompact flag ensures this fires at most
	// once per turn."
	reg := NewRecoveryMechanismRegistry()
	idempotent := reg.IdempotentPerTurnMechanisms()
	ids := make(map[RecoveryMechanismID]bool)
	for _, m := range idempotent {
		ids[m.ID] = true
	}
	assert.True(t, ids[RecoveryReactiveCompaction],
		"reactive_compaction must be marked idempotent per turn")
}

func TestFEAT034_IdempotentPerTurnMechanisms_IncludesStreamingFallback(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	idempotent := reg.IdempotentPerTurnMechanisms()
	ids := make(map[RecoveryMechanismID]bool)
	for _, m := range idempotent {
		ids[m.ID] = true
	}
	assert.True(t, ids[RecoveryStreamingFallback],
		"streaming_fallback must be marked idempotent per turn")
}

// ---------------------------------------------------------------------------
// Termination on exhaustion
// ---------------------------------------------------------------------------

func TestFEAT034_MechanismsThatTerminateOnExhaustion_AreAtLeastThree(t *testing.T) {
	// §4.4 implies max_output_tokens_escalation, prompt_too_long, and fallback
	// model all terminate with a named reason; streaming_fallback also terminates.
	reg := NewRecoveryMechanismRegistry()
	terminating := reg.MechanismsThatTerminateOnExhaustion()
	assert.GreaterOrEqual(t, len(terminating), 3,
		"at least 3 mechanisms should fall back to named termination per §4.4")
}

func TestFEAT034_PromptTooLongTerminationReason_IsCorrectString(t *testing.T) {
	// §4.4: "terminate with reason: 'prompt_too_long'"
	reg := NewRecoveryMechanismRegistry()
	reason := reg.PromptTooLongTerminationReason()
	assert.Equal(t, "prompt_too_long", reason,
		"§4.4 names 'prompt_too_long' as the termination reason")
}

func TestFEAT034_TerminatingMechanismsHaveNonEmptyReason(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	for _, m := range reg.MechanismsThatTerminateOnExhaustion() {
		assert.NotEmptyf(t, m.TerminationReason,
			"mechanism %q is marked FallsBackToTermination=true but has empty TerminationReason", m.ID)
	}
}

// ---------------------------------------------------------------------------
// Attempt limit queries
// ---------------------------------------------------------------------------

func TestFEAT034_MaxOutputTokensRecoveryLimit_IsThree(t *testing.T) {
	// §4.4: "Up to three recovery attempts are allowed per turn
	// (MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3)."
	reg := NewRecoveryMechanismRegistry()
	assert.Equal(t, 3, reg.MaxOutputTokensRecoveryLimit(),
		"§4.4 specifies MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3")
}

func TestFEAT034_MechanismWithMaxAttemptLimit_IncludesMaxOutputTokens(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	limited := reg.MechanismWithMaxAttemptLimit()
	ids := make(map[RecoveryMechanismID]bool)
	for _, m := range limited {
		ids[m.ID] = true
	}
	assert.True(t, ids[RecoveryMaxOutputTokensEscalation],
		"max_output_tokens_escalation must appear in mechanisms with attempt limit")
}

// ---------------------------------------------------------------------------
// Composability
// ---------------------------------------------------------------------------

func TestFEAT034_ComposingMechanisms_IncludesPromptTooLong(t *testing.T) {
	// §4.4: "If the API returns a prompt_too_long error, the loop first
	// attempts context-collapse overflow recovery and reactive compaction."
	// This means prompt_too_long handling composes reactive_compaction.
	reg := NewRecoveryMechanismRegistry()
	composing := reg.ComposingMechanisms()
	ids := make(map[RecoveryMechanismID]bool)
	for _, m := range composing {
		ids[m.ID] = true
	}
	assert.True(t, ids[RecoveryPromptTooLongHandling],
		"prompt_too_long_handling must be in composing mechanisms (delegates to reactive_compaction)")
}

// ---------------------------------------------------------------------------
// AllRecoveryMechanisms ordering (defensive copy)
// ---------------------------------------------------------------------------

func TestFEAT034_AllRecoveryMechanisms_ReturnsDefensiveCopy(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	first := reg.AllRecoveryMechanisms()
	second := reg.AllRecoveryMechanisms()
	// Mutating the first slice must not affect the second.
	first[0] = RecoveryMechanismProfile{}
	assert.NotEqual(t, first[0], second[0],
		"AllRecoveryMechanisms must return an independent defensive copy")
}

// ---------------------------------------------------------------------------
// Specific profile attributes
// ---------------------------------------------------------------------------

func TestFEAT034_ReactiveCompactionProfile_FeatureFlag(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	p, ok := reg.FindRecoveryMechanism(RecoveryReactiveCompaction)
	require.True(t, ok)
	assert.Equal(t, "REACTIVE_COMPACT", p.FeatureFlag,
		"§4.4: reactive compaction is gated by REACTIVE_COMPACT flag")
	assert.True(t, p.IsIdempotentPerTurn,
		"§4.4: hasAttemptedReactiveCompact ensures single-fire per turn")
	assert.Equal(t, 1, p.MaxAttemptsPerTurn,
		"idempotent per-turn means max 1 attempt per turn")
}

func TestFEAT034_FallbackModelProfile_MaxAttemptsIsUnlimited(t *testing.T) {
	// Fallback model does not have a per-turn attempt ceiling; it swaps
	// the model for the remainder of the run.
	reg := NewRecoveryMechanismRegistry()
	p, ok := reg.FindRecoveryMechanism(RecoveryFallbackModel)
	require.True(t, ok)
	assert.Equal(t, 0, p.MaxAttemptsPerTurn,
		"fallback_model has no per-turn ceiling (0 = unlimited)")
	assert.False(t, p.IsIdempotentPerTurn,
		"fallback_model may switch models multiple times across turns")
}

func TestFEAT034_StreamingFallbackProfile_HasNoFeatureFlag(t *testing.T) {
	reg := NewRecoveryMechanismRegistry()
	p, ok := reg.FindRecoveryMechanism(RecoveryStreamingFallback)
	require.True(t, ok)
	assert.Empty(t, p.FeatureFlag,
		"streaming_fallback is unconditionally available — no feature flag")
}

// ---------------------------------------------------------------------------
// containsSubstring helper (internal utility)
// ---------------------------------------------------------------------------

func TestFEAT034_ContainsSubstring_Basic(t *testing.T) {
	assert.True(t, containsSubstring("Composes RecoveryReactiveCompaction", "Composes"))
	assert.False(t, containsSubstring("no match here", "Composes"))
	assert.True(t, containsSubstring("any string", ""),
		"empty substring is always contained")
	assert.False(t, containsSubstring("short", "longer string than input"))
}

// ---------------------------------------------------------------------------
// BDD-style scenarios
// ---------------------------------------------------------------------------

func TestFEAT034_BDD_MaxOutputTokensEscalationCapIsEnforced(t *testing.T) {
	t.Run("Scenario_PaperSpecifiesThreeAttemptCeiling", func(t *testing.T) {
		// Given the query loop implements max output tokens escalation (§4.4),
		// When the registry profile is consulted for the attempt ceiling,
		reg := NewRecoveryMechanismRegistry()
		limit := reg.MaxOutputTokensRecoveryLimit()

		// Then the ceiling is exactly 3 — as named in §4.4:
		//      "MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3".
		// AgentHub's TurnState.MaxOutputTokensRecoveryCount must never exceed
		// this value; exceeding it means the loop is running unguarded retries.
		assert.Equal(t, 3, limit,
			"§4.4: ceiling must be exactly 3 recovery attempts per turn")
	})
}

func TestFEAT034_BDD_ReactiveCompactionIsIdempotentPerTurn(t *testing.T) {
	t.Run("Scenario_HasAttemptedReactiveCompactGuardPreventsRepeat", func(t *testing.T) {
		// Given the hasAttemptedReactiveCompact flag described in §4.4,
		// When the registry is queried for idempotent mechanisms,
		reg := NewRecoveryMechanismRegistry()
		idempotent := reg.IdempotentPerTurnMechanisms()

		// Then reactive_compaction is in the list — the guard is a documented
		// structural commitment, not an implementation detail. A refactor that
		// allows reactive compaction to fire twice per turn contradicts §4.4.
		ids := make(map[RecoveryMechanismID]bool)
		for _, m := range idempotent {
			ids[m.ID] = true
		}
		assert.True(t, ids[RecoveryReactiveCompaction],
			"reactive_compaction must be marked IsIdempotentPerTurn=true per §4.4")
	})
}

func TestFEAT034_BDD_PromptTooLongHandlingDelegatesToReactiveCompaction(t *testing.T) {
	t.Run("Scenario_ComposabilityChainIsDocumented", func(t *testing.T) {
		// Given §4.4: "If the API returns a prompt_too_long error, the loop
		//       first attempts context-collapse overflow recovery and reactive
		//       compaction. Only after these fail does it terminate."
		// When the registry is queried for composing mechanisms,
		reg := NewRecoveryMechanismRegistry()
		composing := reg.ComposingMechanisms()

		// Then prompt_too_long_handling appears — it does not directly terminate
		// on first trigger but delegates to sub-mechanisms first. This documents
		// the composability chain and prevents a refactor from converting the
		// two-step recovery into an immediate abort.
		ids := make(map[RecoveryMechanismID]bool)
		for _, m := range composing {
			ids[m.ID] = true
		}
		assert.True(t, ids[RecoveryPromptTooLongHandling],
			"prompt_too_long_handling must be in ComposingMechanisms")
	})
}

func TestFEAT034_BDD_FallbackModelIsLastResort(t *testing.T) {
	t.Run("Scenario_FallbackModelComposabilityNoteReferencesStreamingFallback", func(t *testing.T) {
		// Given §4.4 lists fallback model as the last listed mechanism,
		// When the profile is inspected for its composability note,
		reg := NewRecoveryMechanismRegistry()
		p, ok := reg.FindRecoveryMechanism(RecoveryFallbackModel)
		require.True(t, ok)

		// Then the note captures the "last-resort" ordering — streaming
		// fallback runs first, and fallback model is the final option. This
		// prevents architectural drift where fallback model bypasses streaming
		// fallback.
		assert.NotEmpty(t, p.ComposabilityNote,
			"fallback_model must have a ComposabilityNote documenting ordering")
		assert.True(t, containsSubstring(p.ComposabilityNote, "streaming fallback"),
			"fallback model note must reference streaming fallback as a prerequisite step")
	})
}

func TestFEAT034_BDD_AllFiveIDsAreRegistered(t *testing.T) {
	t.Run("Scenario_RegistryCoversAllPaperMechanisms", func(t *testing.T) {
		// Given §4.4 enumerates five mechanisms by name,
		// When the registry validates all five IDs,
		paperMechanisms := []RecoveryMechanismID{
			RecoveryMaxOutputTokensEscalation,
			RecoveryReactiveCompaction,
			RecoveryPromptTooLongHandling,
			RecoveryStreamingFallback,
			RecoveryFallbackModel,
		}

		// Then every paper-named mechanism is valid — a refactor that renames
		// one silently breaks the registry-paper mapping.
		for _, id := range paperMechanisms {
			assert.Truef(t, IsValidRecoveryMechanismID(id),
				"§4.4 mechanism %q must be registered", id)
		}
		assert.Len(t, paperMechanisms, SeedRecoveryMechanismCount,
			"the test enumeration must match the seed count constant")
	})
}

func TestFEAT034_BDD_MechanismsWithAttemptLimitCarryPositiveLimit(t *testing.T) {
	t.Run("Scenario_ExplicitLimitsMustBePositive", func(t *testing.T) {
		// Given mechanisms that carry an explicit per-turn attempt ceiling,
		// When the ceiling is inspected,
		reg := NewRecoveryMechanismRegistry()
		limited := reg.MechanismWithMaxAttemptLimit()

		// Then every ceiling is strictly positive — a ceiling of 0 in
		// MechanismWithMaxAttemptLimit would contradict the filter's semantics.
		for _, m := range limited {
			assert.Greaterf(t, m.MaxAttemptsPerTurn, 0,
				"mechanism %q: MaxAttemptsPerTurn must be > 0 if included in MechanismWithMaxAttemptLimit",
				m.ID)
		}
	})
}
