package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// research_methodology_registry_test.go — unit tests for FEAT-048
//
// Coverage targets:
//   - Constructor returns non-nil registry
//   - Count() == SeedMethodologyStepCount (6)
//   - AllSteps() returns 6 steps in narrative order
//   - FindByID: all six valid IDs, one invalid ID
//   - IsValidStepID: valid and invalid
//   - StepsByClaimStrength: strong, moderate, hedged, weak (empty)
//   - HedgedSteps / StrongClaimSteps / DirectObservationSteps
//   - StepsByArtifactType for each artifact type present
//   - StepsByEvidenceCategory for each tier
//   - Invariant: HedgedStepsHaveQualifyingLanguage
//   - Invariant: StrongClaimsHaveDirectObservation
//   - Invariant: AllHaveEvidenceCategory
//   - SeedMethodologyStepIDs length matches SeedMethodologyStepCount
//   - Each step has non-empty Label, Description, CitedSource
//   - AllSteps does not return pointers into internal slice (defensive copy)

func newMethodologyRegistry() *ResearchMethodologyRegistry {
	return NewResearchMethodologyRegistry()
}

// --- Constructor ---

func TestMethodologyRegistry_NewReturnsNonNil(t *testing.T) {
	r := newMethodologyRegistry()
	require.NotNil(t, r)
}

// --- Count ---

func TestMethodologyRegistry_CountEqualsSeedException(t *testing.T) {
	r := newMethodologyRegistry()
	assert.Equal(t, SeedMethodologyStepCount, r.Count())
}

func TestMethodologyRegistry_CountIsSix(t *testing.T) {
	r := newMethodologyRegistry()
	assert.Equal(t, 6, r.Count())
}

// --- SeedMethodologyStepIDs ---

func TestMethodologyRegistry_SeedIDsLengthMatchesSeedCount(t *testing.T) {
	assert.Len(t, SeedMethodologyStepIDs, SeedMethodologyStepCount)
}

func TestMethodologyRegistry_SeedIDsContainsAllExpected(t *testing.T) {
	expected := []string{
		"artifact_acquisition",
		"codebase_exploration",
		"architecture_reconstruction",
		"comparative_analysis",
		"evidence_classification",
		"claim_qualification",
	}
	assert.Equal(t, expected, SeedMethodologyStepIDs)
}

// --- AllSteps ---

func TestMethodologyRegistry_AllStepsReturnsSix(t *testing.T) {
	r := newMethodologyRegistry()
	assert.Len(t, r.AllSteps(), 6)
}

func TestMethodologyRegistry_AllStepsFirstIsArtifactAcquisition(t *testing.T) {
	r := newMethodologyRegistry()
	steps := r.AllSteps()
	assert.Equal(t, "artifact_acquisition", steps[0].StepID)
}

func TestMethodologyRegistry_AllStepsLastIsClaimQualification(t *testing.T) {
	r := newMethodologyRegistry()
	steps := r.AllSteps()
	assert.Equal(t, "claim_qualification", steps[len(steps)-1].StepID)
}

func TestMethodologyRegistry_AllStepsIsDefensiveCopy(t *testing.T) {
	r := newMethodologyRegistry()
	s1 := r.AllSteps()
	s1[0].StepID = "mutated"
	s2 := r.AllSteps()
	assert.Equal(t, "artifact_acquisition", s2[0].StepID, "AllSteps must return a defensive copy")
}

// --- FindByID ---

func TestMethodologyRegistry_FindByIDValidArtifactAcquisition(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("artifact_acquisition")
	require.True(t, ok)
	assert.Equal(t, "artifact_acquisition", step.StepID)
	assert.Equal(t, ClaimStrengthStrong, step.ClaimStrength)
	assert.True(t, step.IsDirectObservation)
	assert.Equal(t, ArtifactTypeSourceCode, step.ArtifactType)
	assert.Equal(t, EvidenceTierLabelB, step.EvidenceCategory)
}

func TestMethodologyRegistry_FindByIDValidCodebaseExploration(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("codebase_exploration")
	require.True(t, ok)
	assert.Equal(t, ClaimStrengthStrong, step.ClaimStrength)
	assert.True(t, step.IsDirectObservation)
	assert.Equal(t, EvidenceTierLabelB, step.EvidenceCategory)
}

func TestMethodologyRegistry_FindByIDValidArchitectureReconstruction(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("architecture_reconstruction")
	require.True(t, ok)
	assert.Equal(t, ClaimStrengthHedged, step.ClaimStrength)
	assert.True(t, step.HasQualifyingLanguage)
	assert.False(t, step.IsDirectObservation)
	assert.Equal(t, ArtifactTypeDesignDoc, step.ArtifactType)
}

func TestMethodologyRegistry_FindByIDValidComparativeAnalysis(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("comparative_analysis")
	require.True(t, ok)
	assert.Equal(t, ClaimStrengthModerate, step.ClaimStrength)
	assert.Equal(t, ArtifactTypeEmpiricalStudy, step.ArtifactType)
	assert.Equal(t, EvidenceTierLabelA, step.EvidenceCategory)
}

func TestMethodologyRegistry_FindByIDValidEvidenceClassification(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("evidence_classification")
	require.True(t, ok)
	assert.Equal(t, ClaimStrengthModerate, step.ClaimStrength)
	assert.Equal(t, ArtifactTypeDocumentation, step.ArtifactType)
	assert.Equal(t, EvidenceTierLabelA, step.EvidenceCategory)
}

func TestMethodologyRegistry_FindByIDValidClaimQualification(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("claim_qualification")
	require.True(t, ok)
	assert.Equal(t, ClaimStrengthHedged, step.ClaimStrength)
	assert.True(t, step.HasQualifyingLanguage)
	assert.False(t, step.IsDirectObservation)
}

func TestMethodologyRegistry_FindByIDUnknownIDReturnsFalse(t *testing.T) {
	r := newMethodologyRegistry()
	step, ok := r.FindByID("nonexistent_step")
	assert.False(t, ok)
	assert.Nil(t, step)
}

func TestMethodologyRegistry_FindByIDEmptyIDReturnsFalse(t *testing.T) {
	r := newMethodologyRegistry()
	_, ok := r.FindByID("")
	assert.False(t, ok)
}

// --- IsValidStepID ---

func TestMethodologyRegistry_IsValidStepIDForEachSeedID(t *testing.T) {
	r := newMethodologyRegistry()
	for _, id := range SeedMethodologyStepIDs {
		assert.True(t, r.IsValidStepID(id), "expected %q to be a valid step ID", id)
	}
}

func TestMethodologyRegistry_IsValidStepIDFalseForUnknown(t *testing.T) {
	r := newMethodologyRegistry()
	assert.False(t, r.IsValidStepID("unknown_step"))
}

// --- StepsByClaimStrength ---

func TestMethodologyRegistry_StepsByClaimStrengthStrong(t *testing.T) {
	r := newMethodologyRegistry()
	strong := r.StepsByClaimStrength(ClaimStrengthStrong)
	require.NotEmpty(t, strong)
	for _, s := range strong {
		assert.Equal(t, ClaimStrengthStrong, s.ClaimStrength)
	}
}

func TestMethodologyRegistry_StepsByClaimStrengthStrongCountIsTwo(t *testing.T) {
	r := newMethodologyRegistry()
	strong := r.StepsByClaimStrength(ClaimStrengthStrong)
	// artifact_acquisition + codebase_exploration are both Tier B strong
	assert.Len(t, strong, 2)
}

func TestMethodologyRegistry_StepsByClaimStrengthModerate(t *testing.T) {
	r := newMethodologyRegistry()
	moderate := r.StepsByClaimStrength(ClaimStrengthModerate)
	require.NotEmpty(t, moderate)
	for _, s := range moderate {
		assert.Equal(t, ClaimStrengthModerate, s.ClaimStrength)
	}
}

func TestMethodologyRegistry_StepsByClaimStrengthHedged(t *testing.T) {
	r := newMethodologyRegistry()
	hedged := r.StepsByClaimStrength(ClaimStrengthHedged)
	require.NotEmpty(t, hedged)
	for _, s := range hedged {
		assert.Equal(t, ClaimStrengthHedged, s.ClaimStrength)
	}
}

func TestMethodologyRegistry_StepsByClaimStrengthWeakIsEmpty(t *testing.T) {
	r := newMethodologyRegistry()
	weak := r.StepsByClaimStrength(ClaimStrengthWeak)
	assert.Empty(t, weak, "no seed step has ClaimStrengthWeak — weak is reserved for partial evidence scenarios")
}

// --- HedgedSteps ---

func TestMethodologyRegistry_HedgedStepsNotEmpty(t *testing.T) {
	r := newMethodologyRegistry()
	hedged := r.HedgedSteps()
	require.NotEmpty(t, hedged)
}

func TestMethodologyRegistry_HedgedStepsAllHaveQualifyingLanguage(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.HedgedSteps() {
		assert.True(t, s.HasQualifyingLanguage, "hedged step %q must have qualifying language", s.StepID)
	}
}

func TestMethodologyRegistry_HedgedStepsCountIsTwo(t *testing.T) {
	r := newMethodologyRegistry()
	// architecture_reconstruction + claim_qualification
	assert.Len(t, r.HedgedSteps(), 2)
}

// --- StrongClaimSteps ---

func TestMethodologyRegistry_StrongClaimStepsNotEmpty(t *testing.T) {
	r := newMethodologyRegistry()
	assert.NotEmpty(t, r.StrongClaimSteps())
}

func TestMethodologyRegistry_StrongClaimStepsAllAreDirectObservations(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.StrongClaimSteps() {
		assert.True(t, s.IsDirectObservation, "strong step %q must be a direct observation", s.StepID)
	}
}

// --- DirectObservationSteps ---

func TestMethodologyRegistry_DirectObservationStepsNotEmpty(t *testing.T) {
	r := newMethodologyRegistry()
	assert.NotEmpty(t, r.DirectObservationSteps())
}

func TestMethodologyRegistry_DirectObservationStepsAllDirectTrue(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.DirectObservationSteps() {
		assert.True(t, s.IsDirectObservation, "step %q should be a direct observation", s.StepID)
	}
}

// --- StepsByArtifactType ---

func TestMethodologyRegistry_StepsByArtifactTypeSourceCode(t *testing.T) {
	r := newMethodologyRegistry()
	sc := r.StepsByArtifactType(ArtifactTypeSourceCode)
	require.NotEmpty(t, sc)
	for _, s := range sc {
		assert.Equal(t, ArtifactTypeSourceCode, s.ArtifactType)
	}
}

func TestMethodologyRegistry_StepsByArtifactTypeDocumentation(t *testing.T) {
	r := newMethodologyRegistry()
	docs := r.StepsByArtifactType(ArtifactTypeDocumentation)
	require.NotEmpty(t, docs)
}

func TestMethodologyRegistry_StepsByArtifactTypeEmpiricalStudy(t *testing.T) {
	r := newMethodologyRegistry()
	emp := r.StepsByArtifactType(ArtifactTypeEmpiricalStudy)
	require.NotEmpty(t, emp)
}

func TestMethodologyRegistry_StepsByArtifactTypeDesignDoc(t *testing.T) {
	r := newMethodologyRegistry()
	dd := r.StepsByArtifactType(ArtifactTypeDesignDoc)
	require.NotEmpty(t, dd)
}

func TestMethodologyRegistry_StepsByArtifactTypeBenchmarkIsEmpty(t *testing.T) {
	r := newMethodologyRegistry()
	// No seed step uses ArtifactTypeBenchmark as primary type
	bm := r.StepsByArtifactType(ArtifactTypeBenchmark)
	assert.Empty(t, bm)
}

// --- StepsByEvidenceCategory ---

func TestMethodologyRegistry_StepsByEvidenceCategoryTierB(t *testing.T) {
	r := newMethodologyRegistry()
	tierB := r.StepsByEvidenceCategory(EvidenceTierLabelB)
	require.NotEmpty(t, tierB)
	for _, s := range tierB {
		assert.Equal(t, EvidenceTierLabelB, s.EvidenceCategory)
	}
}

func TestMethodologyRegistry_StepsByEvidenceCategoryTierA(t *testing.T) {
	r := newMethodologyRegistry()
	tierA := r.StepsByEvidenceCategory(EvidenceTierLabelA)
	require.NotEmpty(t, tierA)
}

func TestMethodologyRegistry_StepsByEvidenceCategoryTierC(t *testing.T) {
	r := newMethodologyRegistry()
	tierC := r.StepsByEvidenceCategory(EvidenceTierLabelC)
	require.NotEmpty(t, tierC)
}

// --- Structural Invariants ---

func TestMethodologyRegistry_InvariantHedgedStepsHaveQualifyingLanguage(t *testing.T) {
	r := newMethodologyRegistry()
	ok, violations := r.HedgedStepsHaveQualifyingLanguage()
	assert.True(t, ok, "HedgedStepsHaveQualifyingLanguage violated for: %v", violations)
	assert.Empty(t, violations)
}

func TestMethodologyRegistry_InvariantStrongClaimsHaveDirectObservation(t *testing.T) {
	r := newMethodologyRegistry()
	ok, violations := r.StrongClaimsHaveDirectObservation()
	assert.True(t, ok, "StrongClaimsHaveDirectObservation violated for: %v", violations)
	assert.Empty(t, violations)
}

func TestMethodologyRegistry_InvariantAllHaveEvidenceCategory(t *testing.T) {
	r := newMethodologyRegistry()
	ok, violations := r.AllHaveEvidenceCategory()
	assert.True(t, ok, "AllHaveEvidenceCategory violated for: %v", violations)
	assert.Empty(t, violations)
}

// --- Non-empty field checks ---

func TestMethodologyRegistry_AllStepsHaveNonEmptyLabel(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.AllSteps() {
		assert.NotEmpty(t, s.Label, "step %q must have a non-empty Label", s.StepID)
	}
}

func TestMethodologyRegistry_AllStepsHaveNonEmptyDescription(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.AllSteps() {
		assert.NotEmpty(t, s.Description, "step %q must have a non-empty Description", s.StepID)
	}
}

func TestMethodologyRegistry_AllStepsHaveNonEmptyCitedSource(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.AllSteps() {
		assert.NotEmpty(t, s.CitedSource, "step %q must have a non-empty CitedSource", s.StepID)
	}
}

func TestMethodologyRegistry_AllStepsHaveNonEmptyPDFSection(t *testing.T) {
	r := newMethodologyRegistry()
	for _, s := range r.AllSteps() {
		assert.NotEmpty(t, s.PDFSection, "step %q must have a non-empty PDFSection", s.StepID)
	}
}
