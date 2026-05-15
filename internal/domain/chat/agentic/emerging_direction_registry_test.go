package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emerging_direction_registry_test.go — unit tests for §11.6 EmergingDirectionRegistry.

// ── construction ─────────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_NewRegistry_NotNil(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	require.NotNil(t, r)
}

func TestEmergingDirectionRegistry_Count_EqualsSeedCount(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	assert.Equal(t, SeedEmergingDirectionCount, r.Count())
}

func TestEmergingDirectionRegistry_Count_IsFive(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	assert.Equal(t, 5, r.Count())
}

func TestEmergingDirectionRegistry_All_LengthMatchesCount(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	assert.Len(t, r.All(), r.Count())
}

func TestEmergingDirectionRegistry_All_ReturnsCopy(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	a := r.All()
	b := r.All()
	assert.Equal(t, a, b)
	// Mutating the returned slice must not affect the registry.
	a[0].Name = "mutated"
	c := r.All()
	assert.NotEqual(t, "mutated", c[0].Name)
}

// ── FindByID ─────────────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_FindByID_AllSeedIDsFound(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, id := range SeedEmergingDirectionIDs {
		p, ok := r.FindByID(id)
		assert.True(t, ok, "expected id %q to be found", id)
		assert.Equal(t, id, p.ID)
	}
}

func TestEmergingDirectionRegistry_FindByID_UnknownReturnsFalse(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	_, ok := r.FindByID("no_such_direction")
	assert.False(t, ok)
}

func TestEmergingDirectionRegistry_FindByID_ArchitecturalDecoupling_CorrectName(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	p, ok := r.FindByID(DirectionArchitecturalDecoupling)
	require.True(t, ok)
	assert.Equal(t, "Architectural Decoupling", p.Name)
}

func TestEmergingDirectionRegistry_FindByID_MemoryFirstClass_CorrectCategory(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	p, ok := r.FindByID(DirectionMemoryFirstClass)
	require.True(t, ok)
	assert.Equal(t, CategoryMemory, p.Category)
}

func TestEmergingDirectionRegistry_FindByID_ObservabilityAndSilentFailure_ThreeEvidence(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	p, ok := r.FindByID(DirectionObservabilityAndSilentFailure)
	require.True(t, ok)
	assert.Len(t, p.SupportingEvidence, 3)
}

func TestEmergingDirectionRegistry_FindByID_Governance_CategoryGovernance(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	p, ok := r.FindByID(DirectionGovernance)
	require.True(t, ok)
	assert.Equal(t, CategoryGovernance, p.Category)
}

func TestEmergingDirectionRegistry_FindByID_ProactiveArchitectures_NotRequiresHarness(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	p, ok := r.FindByID(DirectionProactiveArchitectures)
	require.True(t, ok)
	// KAIROS is already a harness-level mechanism; only model/config changes needed.
	assert.False(t, p.RequiresHarnessChange)
}

// ── IsValid ───────────────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_IsValid_KnownIDsReturnTrue(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, id := range SeedEmergingDirectionIDs {
		assert.True(t, r.IsValid(id), "expected %q to be valid", id)
	}
}

func TestEmergingDirectionRegistry_IsValid_UnknownReturnsFalse(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	assert.False(t, r.IsValid("phantom_direction"))
}

// ── ByCategory ────────────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_ByCategory_ArchitectureHasOne(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ByCategory(CategoryArchitecture)
	assert.Len(t, got, 1)
	assert.Equal(t, DirectionArchitecturalDecoupling, got[0].ID)
}

func TestEmergingDirectionRegistry_ByCategory_GovernanceHasOne(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ByCategory(CategoryGovernance)
	assert.Len(t, got, 1)
	assert.Equal(t, DirectionGovernance, got[0].ID)
}

func TestEmergingDirectionRegistry_ByCategory_ProactivityHasOne(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ByCategory(CategoryProactivity)
	assert.Len(t, got, 1)
	assert.Equal(t, DirectionProactiveArchitectures, got[0].ID)
}

func TestEmergingDirectionRegistry_ByCategory_UnknownCategoryReturnsEmpty(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ByCategory("nonexistent_cat")
	assert.Empty(t, got)
}

func TestEmergingDirectionRegistry_ByCategory_AllCategoriesSumToFive(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	cats := []EmergingDirectionCategory{
		CategoryArchitecture,
		CategoryMemory,
		CategoryObservability,
		CategoryGovernance,
		CategoryProactivity,
	}
	total := 0
	for _, c := range cats {
		total += len(r.ByCategory(c))
	}
	assert.Equal(t, 5, total)
}

// ── ByHorizon ─────────────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_ByHorizon_NearTermHasFour(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	// ArchitecturalDecoupling, ObservabilityAndSilentFailure, Governance, Proactive
	got := r.ByHorizon(HorizonNearTerm)
	assert.Len(t, got, 4)
}

func TestEmergingDirectionRegistry_ByHorizon_MidTermHasOne(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ByHorizon(HorizonMidTerm)
	assert.Len(t, got, 1)
	assert.Equal(t, DirectionMemoryFirstClass, got[0].ID)
}

func TestEmergingDirectionRegistry_ByHorizon_LongTermIsEmpty(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ByHorizon(HorizonLongTerm)
	// §11.6 names no direction as long-term; all are near or mid.
	assert.Empty(t, got)
}

// ── RequiringHarnessChange ────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_RequiringHarnessChange_FourDirections(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.RequiringHarnessChange()
	assert.Len(t, got, 4, "exactly four §11.6 directions require harness-layer engineering")
}

func TestEmergingDirectionRegistry_RequiringHarnessChange_ProactiveExcluded(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, p := range r.RequiringHarnessChange() {
		assert.NotEqual(t, DirectionProactiveArchitectures, p.ID,
			"proactive architectures already have a harness-level mechanism (KAIROS)")
	}
}

// ── ServingValue ──────────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_ServingValue_SafetyServesTwo(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	// Observability + Governance both list DesignValueSafety.
	got := r.ServingValue(DesignValueSafety)
	assert.Len(t, got, 2)
}

func TestEmergingDirectionRegistry_ServingValue_ReliabilityServesThree(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	// ArchitecturalDecoupling, MemoryFirstClass, ObservabilityAndSilentFailure.
	got := r.ServingValue(DesignValueReliability)
	assert.Len(t, got, 3)
}

func TestEmergingDirectionRegistry_ServingValue_CapabilityServesTwo(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	// ArchitecturalDecoupling + ProactiveArchitectures.
	got := r.ServingValue(DesignValueCapability)
	assert.Len(t, got, 2)
}

func TestEmergingDirectionRegistry_ServingValue_HumanAuthorityServesOne(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ServingValue(DesignValueHumanAuthority)
	assert.Len(t, got, 1)
	assert.Equal(t, DirectionGovernance, got[0].ID)
}

func TestEmergingDirectionRegistry_ServingValue_UnknownValueReturnsEmpty(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	got := r.ServingValue("no_such_value")
	assert.Empty(t, got)
}

// ── WithEmpiricalEvidence ─────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_WithEmpiricalEvidence_FourDirections(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	// ArchitecturalDecoupling has NO empirical study (both sources are analytical).
	got := r.WithEmpiricalEvidence()
	assert.Len(t, got, 4)
}

func TestEmergingDirectionRegistry_WithEmpiricalEvidence_ArchitecturalDecouplingExcluded(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, p := range r.WithEmpiricalEvidence() {
		assert.NotEqual(t, DirectionArchitecturalDecoupling, p.ID,
			"architectural decoupling is cited from analytical sources only")
	}
}

// ── TotalEvidenceCount ────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_TotalEvidenceCount_IsSevenOrMore(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	// Arch=2, Memory=2, Observability=3, Governance=2, Proactive=1 → 10 total.
	assert.GreaterOrEqual(t, r.TotalEvidenceCount(), 7)
}

func TestEmergingDirectionRegistry_TotalEvidenceCount_IsTen(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	assert.Equal(t, 10, r.TotalEvidenceCount())
}

// ── StructuralInvariants ──────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_StructuralInvariants_AllThreeHold(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	five, openQ, harness := r.StructuralInvariants()
	assert.True(t, five, "invariant 1: exactly five directions registered")
	assert.True(t, openQ, "invariant 2: every direction has a non-empty OpenQuestion")
	assert.True(t, harness, "invariant 3: at least four directions require harness change")
}

// ── PDFSection field ─────────────────────────────────────────────────────────

func TestEmergingDirectionRegistry_AllProfiles_PDFSectionIsSection116(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, p := range r.All() {
		assert.Equal(t, "§11.6", p.PDFSection, "direction %q must reference §11.6", p.ID)
	}
}

// ── OpenQuestion completeness ─────────────────────────────────────────────────

func TestEmergingDirectionRegistry_AllProfiles_OpenQuestionNonEmpty(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, p := range r.All() {
		assert.NotEmpty(t, p.OpenQuestion, "direction %q must have a non-empty OpenQuestion", p.ID)
	}
}

// ── AffectedValues completeness ───────────────────────────────────────────────

func TestEmergingDirectionRegistry_AllProfiles_AtLeastOneAffectedValue(t *testing.T) {
	r := NewEmergingDirectionRegistry()
	for _, p := range r.All() {
		assert.NotEmpty(t, p.AffectedValues, "direction %q must affect at least one design value", p.ID)
	}
}

// ── SeedEmergingDirectionIDs order ────────────────────────────────────────────

func TestEmergingDirectionRegistry_SeedIDs_FirstIsArchitecturalDecoupling(t *testing.T) {
	assert.Equal(t, DirectionArchitecturalDecoupling, SeedEmergingDirectionIDs[0])
}

func TestEmergingDirectionRegistry_SeedIDs_LastIsProactiveArchitectures(t *testing.T) {
	assert.Equal(t, DirectionProactiveArchitectures, SeedEmergingDirectionIDs[len(SeedEmergingDirectionIDs)-1])
}
