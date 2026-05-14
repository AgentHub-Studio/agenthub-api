package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// research_methodology_registry_bdd_test.go — BDD tests for FEAT-048
//
// Scenarios derived from Appendix B §B.2 "Research Methodology" of
// arXiv:2604.14228v1:
//
//   Scenario 1: Registry is seeded with exactly six §B.2 methodology steps
//   Scenario 2: Artifact acquisition is the sole Tier B strong direct observation seed
//   Scenario 3: Architecture reconstruction and claim qualification require hedging language
//   Scenario 4: All structural invariants hold simultaneously on a fresh registry
//   Scenario 5: Source-code artifact type exclusively backs strong-claim steps
//   Scenario 6: Comparative analysis step is backed by empirical-study artifacts at Tier A

// Scenario 1: Registry is seeded with exactly six §B.2 methodology steps
// Given §B.2 describes six distinct research phases
// When the registry is constructed
// Then Count() == 6, AllSteps() has 6 items, and each has a non-empty StepID and Label
func TestBDD_ResearchMethodology_Scenario1_SixStepsSeeded(t *testing.T) {
	// Given
	r := NewResearchMethodologyRegistry()

	// When
	count := r.Count()
	steps := r.AllSteps()

	// Then
	assert.Equal(t, 6, count, "Count() must equal 6 per §B.2")
	require.Len(t, steps, 6, "AllSteps() must return exactly 6 items")

	for _, s := range steps {
		assert.NotEmpty(t, s.StepID, "every step must have a non-empty StepID")
		assert.NotEmpty(t, s.Label, "every step must have a non-empty Label")
		assert.NotEmpty(t, s.PDFSection, "every step must cite its PDF section")
	}
}

// Scenario 2: Artifact acquisition is the sole Tier B strong direct observation seed step
// Given §B.2 describes npm extraction as the primary evidence base grounding Tier B claims
// When filtering by EvidenceTierLabelB
// Then both artifact_acquisition and codebase_exploration are Tier B, both strong and direct
func TestBDD_ResearchMethodology_Scenario2_TierBStepsAreStrongAndDirect(t *testing.T) {
	// Given
	r := NewResearchMethodologyRegistry()

	// When
	tierBSteps := r.StepsByEvidenceCategory(EvidenceTierLabelB)

	// Then
	require.NotEmpty(t, tierBSteps, "must have at least one Tier B step")

	for _, s := range tierBSteps {
		assert.Equal(t, ClaimStrengthStrong, s.ClaimStrength,
			"Tier B step %q must have ClaimStrengthStrong", s.StepID)
		assert.True(t, s.IsDirectObservation,
			"Tier B step %q must be a direct observation", s.StepID)
		assert.False(t, s.HasQualifyingLanguage,
			"Tier B step %q must not require qualifying language", s.StepID)
	}

	// artifact_acquisition must be among them
	ids := make([]string, len(tierBSteps))
	for i, s := range tierBSteps {
		ids[i] = s.StepID
	}
	assert.Contains(t, ids, "artifact_acquisition",
		"artifact_acquisition must be a Tier B step")
	assert.Contains(t, ids, "codebase_exploration",
		"codebase_exploration must be a Tier B step")
}

// Scenario 3: Architecture reconstruction and claim qualification require hedging language
// Given §B.2 states that reconstructed and inferred claims use mandatory hedging phrases
// When retrieving HedgedSteps()
// Then exactly architecture_reconstruction and claim_qualification are present,
//
//	and both have ClaimStrengthHedged, HasQualifyingLanguage==true, IsDirectObservation==false
func TestBDD_ResearchMethodology_Scenario3_HedgedStepsRequireQualifyingLanguage(t *testing.T) {
	// Given
	r := NewResearchMethodologyRegistry()

	// When
	hedged := r.HedgedSteps()

	// Then
	require.Len(t, hedged, 2,
		"exactly architecture_reconstruction and claim_qualification must be hedged")

	hedgedIDs := make(map[string]bool, 2)
	for _, s := range hedged {
		hedgedIDs[s.StepID] = true
		assert.Equal(t, ClaimStrengthHedged, s.ClaimStrength,
			"hedged step %q must have ClaimStrengthHedged", s.StepID)
		assert.True(t, s.HasQualifyingLanguage,
			"hedged step %q must have HasQualifyingLanguage=true", s.StepID)
		assert.False(t, s.IsDirectObservation,
			"hedged step %q must not be a direct observation", s.StepID)
	}

	assert.True(t, hedgedIDs["architecture_reconstruction"],
		"architecture_reconstruction must be a hedged step")
	assert.True(t, hedgedIDs["claim_qualification"],
		"claim_qualification must be a hedged step")
}

// Scenario 4: All three structural invariants hold simultaneously on a fresh registry
// Given the §B.2 seed data encodes consistent epistemological constraints
// When all three invariants are evaluated
// Then each returns (true, nil) — no violations
func TestBDD_ResearchMethodology_Scenario4_AllInvariantsHold(t *testing.T) {
	// Given
	r := NewResearchMethodologyRegistry()

	// When + Then — invariant 1
	ok1, violations1 := r.HedgedStepsHaveQualifyingLanguage()
	assert.True(t, ok1, "HedgedStepsHaveQualifyingLanguage must hold; violations: %v", violations1)
	assert.Empty(t, violations1)

	// When + Then — invariant 2
	ok2, violations2 := r.StrongClaimsHaveDirectObservation()
	assert.True(t, ok2, "StrongClaimsHaveDirectObservation must hold; violations: %v", violations2)
	assert.Empty(t, violations2)

	// When + Then — invariant 3
	ok3, violations3 := r.AllHaveEvidenceCategory()
	assert.True(t, ok3, "AllHaveEvidenceCategory must hold; violations: %v", violations3)
	assert.Empty(t, violations3)
}

// Scenario 5: Source-code artifact type exclusively backs strong-claim steps
// Given that direct TypeScript inspection is the only basis for Tier B strong claims
// When retrieving StepsByArtifactType(ArtifactTypeSourceCode)
// Then all returned steps are ClaimStrengthStrong and all strong steps use source code
func TestBDD_ResearchMethodology_Scenario5_SourceCodeArtifactBacksStrongClaims(t *testing.T) {
	// Given
	r := NewResearchMethodologyRegistry()

	// When
	sourceCodeSteps := r.StepsByArtifactType(ArtifactTypeSourceCode)
	strongSteps := r.StrongClaimSteps()

	// Then — every source-code step is strong
	require.NotEmpty(t, sourceCodeSteps, "must have source-code-backed steps")
	for _, s := range sourceCodeSteps {
		assert.Equal(t, ClaimStrengthStrong, s.ClaimStrength,
			"step %q backed by source code must be ClaimStrengthStrong", s.StepID)
	}

	// Then — every strong step uses source code
	require.NotEmpty(t, strongSteps, "must have strong-claim steps")
	for _, s := range strongSteps {
		assert.Equal(t, ArtifactTypeSourceCode, s.ArtifactType,
			"strong-claim step %q must be backed by ArtifactTypeSourceCode", s.StepID)
	}
}

// Scenario 6: Comparative analysis step is backed by empirical-study artifacts at Tier A
// Given §B.2 notes that comparator analyses draw on published empirical studies
// When FindByID("comparative_analysis") is called
// Then the step has ArtifactTypeEmpiricalStudy, EvidenceTierLabelA, ClaimStrengthModerate,
//
//	and IsDirectObservation == false (because comparators are not inspected at source level)
func TestBDD_ResearchMethodology_Scenario6_ComparativeAnalysisEmpiricalStudyTierA(t *testing.T) {
	// Given
	r := NewResearchMethodologyRegistry()

	// When
	step, ok := r.FindByID("comparative_analysis")

	// Then
	require.True(t, ok, "comparative_analysis must be a registered step")
	assert.Equal(t, ArtifactTypeEmpiricalStudy, step.ArtifactType,
		"comparative_analysis must use ArtifactTypeEmpiricalStudy")
	assert.Equal(t, EvidenceTierLabelA, step.EvidenceCategory,
		"comparative_analysis must be Tier A")
	assert.Equal(t, ClaimStrengthModerate, step.ClaimStrength,
		"comparative_analysis must be ClaimStrengthModerate")
	assert.False(t, step.IsDirectObservation,
		"comparative_analysis must not be a direct observation — comparators are not inspected at source level")
	assert.NotEmpty(t, step.CitedSource,
		"comparative_analysis must cite its comparator sources")
}
