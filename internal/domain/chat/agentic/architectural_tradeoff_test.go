package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FEAT023 unit tests for ArchitecturalTradeoffRegistry (§11.3 + §11.7).

func TestFEAT023_SeedCount(t *testing.T) {
	assert.Equal(t, 6, SeedArchitecturalTradeoffCount,
		"SeedArchitecturalTradeoffCount must be 6 (3 trade-offs + 3 recurring choices)")
}

func TestFEAT023_SeedSlugsLength(t *testing.T) {
	assert.Len(t, SeedArchitecturalTradeoffSlugs, SeedArchitecturalTradeoffCount,
		"SeedArchitecturalTradeoffSlugs length must match SeedArchitecturalTradeoffCount")
}

func TestFEAT023_NewRegistry_NotNil(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	require.NotNil(t, r)
}

func TestFEAT023_AllProfiles_Count(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	assert.Len(t, r.AllProfiles(), SeedArchitecturalTradeoffCount)
}

func TestFEAT023_AllSlugsPresent(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, slug := range SeedArchitecturalTradeoffSlugs {
		t.Run(string(slug), func(t *testing.T) {
			p, ok := r.FindArchitecturalTradeoffBySlug(slug)
			require.True(t, ok, "slug %q not found", slug)
			assert.Equal(t, slug, p.ID)
		})
	}
}

func TestFEAT023_FindBySlug_InvalidReturnsNotFound(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	p, ok := r.FindArchitecturalTradeoffBySlug("nonexistent_slug")
	assert.False(t, ok)
	assert.Nil(t, p)
}

func TestFEAT023_ConcreteTradeoffs_Count(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	tradeoffs := r.ConcreteTradeoffs()
	assert.Len(t, tradeoffs, 3, "§11.3 defines exactly three concrete trade-offs")
}

func TestFEAT023_RecurringChoices_Count(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	choices := r.RecurringChoices()
	assert.Len(t, choices, 3, "§11.7 defines exactly three recurring design choices")
}

func TestFEAT023_ConcreteTradeoffs_AllKindCorrect(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.ConcreteTradeoffs() {
		assert.Equal(t, KindConcreteTradeoff, p.Kind, "profile %q should have KindConcreteTradeoff", p.ID)
	}
}

func TestFEAT023_RecurringChoices_AllKindCorrect(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.RecurringChoices() {
		assert.Equal(t, KindRecurringChoice, p.Kind, "profile %q should have KindRecurringChoice", p.ID)
	}
}

func TestFEAT023_AllProfiles_NoEmptyLabel(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.AllProfiles() {
		assert.NotEmpty(t, p.Label, "profile %q must have non-empty Label", p.ID)
	}
}

func TestFEAT023_AllProfiles_PDFSectionCorrect(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.AllProfiles() {
		switch p.Kind {
		case KindConcreteTradeoff:
			assert.Equal(t, "11.3", p.PDFSection, "profile %q (§11.3) has wrong PDFSection", p.ID)
		case KindRecurringChoice:
			assert.Equal(t, "11.7", p.PDFSection, "profile %q (§11.7) has wrong PDFSection", p.ID)
		}
	}
}

func TestFEAT023_AllProfiles_ValuesInTensionNonEmpty(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.AllProfiles() {
		assert.NotEmpty(t, p.ValuesInTension,
			"profile %q must list at least one design value in tension", p.ID)
	}
}

func TestFEAT023_AllProfiles_SubsystemsAffectedNonEmpty(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.AllProfiles() {
		assert.NotEmpty(t, p.SubsystemsAffected,
			"profile %q must list at least one affected subsystem", p.ID)
	}
}

func TestFEAT023_AllProfiles_EvidenceSummaryNonEmpty(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, p := range r.AllProfiles() {
		assert.NotEmpty(t, p.EvidenceSummary,
			"profile %q must have a non-empty EvidenceSummary", p.ID)
	}
}

func TestFEAT023_InvolvingValue_Safety(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	results := r.InvolvingValue(DesignValueSafety)
	// safety_vs_autonomy, simplicity_vs_extensibility, graduated_layering all involve safety
	assert.GreaterOrEqual(t, len(results), 3,
		"at least 3 profiles involve DesignValueSafety")
}

func TestFEAT023_InvolvingValue_Capability(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	results := r.InvolvingValue(DesignValueCapability)
	// safety_vs_autonomy, context_efficiency_vs_transparency, model_judgment_in_harness
	assert.GreaterOrEqual(t, len(results), 2,
		"at least 2 profiles involve DesignValueCapability")
}

func TestFEAT023_AffectingSubsystem_Permissions(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	results := r.AffectingSubsystem("permissions")
	assert.GreaterOrEqual(t, len(results), 3,
		"at least 3 profiles should affect the permissions subsystem")
}

func TestFEAT023_AffectingSubsystem_Sessions(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	results := r.AffectingSubsystem("sessions")
	assert.GreaterOrEqual(t, len(results), 2,
		"safety_vs_autonomy and append_only_auditability both affect sessions")
}

func TestFEAT023_AffectingSubsystem_Unknown(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	results := r.AffectingSubsystem("does_not_exist")
	assert.Empty(t, results)
}

func TestFEAT023_IsValidSlug_Known(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	for _, slug := range SeedArchitecturalTradeoffSlugs {
		assert.True(t, r.IsValidSlug(slug), "expected %q to be valid", slug)
	}
}

func TestFEAT023_IsValidSlug_Unknown(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	assert.False(t, r.IsValidSlug("unknown_slug"))
}

func TestFEAT023_AllProfiles_ImmutableCopy(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	profiles1 := r.AllProfiles()
	profiles2 := r.AllProfiles()
	// Mutating the returned slice must not affect subsequent calls
	profiles1[0].Label = "mutated"
	assert.NotEqual(t, "mutated", profiles2[0].Label,
		"AllProfiles should return independent copies")
}

func TestFEAT023_ByKind_ExhaustiveCategories(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	concretes := r.ByKind(KindConcreteTradeoff)
	choices := r.ByKind(KindRecurringChoice)
	assert.Equal(t, SeedArchitecturalTradeoffCount, len(concretes)+len(choices),
		"concrete trade-offs and recurring choices must cover all profiles")
}

func TestFEAT023_SafetyVsAutonomy_Profile(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	p, ok := r.FindArchitecturalTradeoffBySlug(TradeoffSafetyVsAutonomy)
	require.True(t, ok)
	assert.Equal(t, KindConcreteTradeoff, p.Kind)
	assert.Contains(t, p.EvidenceSummary, "93%")
	assert.Contains(t, p.SubsystemsAffected, "permissions")
}

func TestFEAT023_ModelJudgmentInHarness_Profile(t *testing.T) {
	r := NewArchitecturalTradeoffRegistry()
	p, ok := r.FindArchitecturalTradeoffBySlug(ChoiceModelJudgmentInHarness)
	require.True(t, ok)
	assert.Equal(t, KindRecurringChoice, p.Kind)
	assert.Equal(t, "11.7", p.PDFSection)
	assert.Contains(t, p.ConsequenceDescription, "1.6%")
}
