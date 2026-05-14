package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for design_philosophy_registry.go — FEAT-045 — §11.1 Design Philosophy

func TestDesignPhilosophyRegistry_NewRegistry(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	require.NotNil(t, r, "NewDesignPhilosophyRegistry must return a non-nil registry")
}

func TestDesignPhilosophyRegistry_Count(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	assert.Equal(t, 5, r.Count(), "§11.1 defines exactly five design-philosophy concepts")
}

func TestDesignPhilosophyRegistry_AllConcepts_Length(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	all := r.AllConcepts()
	assert.Len(t, all, 5, "AllConcepts must return all five §11.1 concepts")
}

func TestDesignPhilosophyRegistry_AllConcepts_Order(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	all := r.AllConcepts()
	expectedOrder := []PhilosophyConceptID{
		ConceptDecisionLogicRatio,
		ConceptMinimalScaffoldingThesis,
		ConceptOSKernelAnalogy,
		ConceptConvergenceThesis,
		ConceptAlternativeDesignFamilies,
	}
	for i, expected := range expectedOrder {
		assert.Equal(t, expected, all[i].ID, "concept at position %d should be %s", i, expected)
	}
}

func TestDesignPhilosophyRegistry_FindByID_KnownConcepts(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	knownIDs := []PhilosophyConceptID{
		ConceptDecisionLogicRatio,
		ConceptMinimalScaffoldingThesis,
		ConceptOSKernelAnalogy,
		ConceptConvergenceThesis,
		ConceptAlternativeDesignFamilies,
	}
	for _, id := range knownIDs {
		c, ok := r.FindByID(id)
		assert.True(t, ok, "FindByID(%q) should succeed", id)
		assert.Equal(t, id, c.ID, "concept ID mismatch for %q", id)
	}
}

func TestDesignPhilosophyRegistry_FindByID_Unknown(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	_, ok := r.FindByID("nonexistent_concept")
	assert.False(t, ok, "FindByID with unknown ID must return false")
}

func TestDesignPhilosophyRegistry_IsValidConcept(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	assert.True(t, r.IsValidConcept(ConceptDecisionLogicRatio))
	assert.True(t, r.IsValidConcept(ConceptMinimalScaffoldingThesis))
	assert.True(t, r.IsValidConcept(ConceptOSKernelAnalogy))
	assert.True(t, r.IsValidConcept(ConceptConvergenceThesis))
	assert.True(t, r.IsValidConcept(ConceptAlternativeDesignFamilies))
	assert.False(t, r.IsValidConcept("bogus"))
}

func TestDesignPhilosophyRegistry_DecisionLogicRatio_IsQuantified(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptDecisionLogicRatio)
	require.True(t, ok)
	assert.True(t, c.IsQuantified, "DecisionLogicRatio must be marked as quantified")
	assert.Contains(t, c.QuantitativeClaim, "1.6%", "QuantitativeClaim must mention 1.6%%")
	assert.Contains(t, c.QuantitativeClaim, "98.4%", "QuantitativeClaim must mention 98.4%%")
}

func TestDesignPhilosophyRegistry_QuantifiedConcepts_ExactlyOne(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	q := r.QuantifiedConcepts()
	require.Len(t, q, 1, "exactly one §11.1 concept carries a numeric claim")
	assert.Equal(t, ConceptDecisionLogicRatio, q[0].ID)
}

func TestDesignPhilosophyRegistry_NonQuantifiedConcepts_HaveEmptyQuantitativeClaim(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, c := range r.AllConcepts() {
		if c.IsQuantified {
			continue
		}
		assert.Empty(t, c.QuantitativeClaim,
			"non-quantified concept %q must have empty QuantitativeClaim", c.ID)
	}
}

func TestDesignPhilosophyRegistry_AllConcepts_HaveNonEmptyLabels(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, c := range r.AllConcepts() {
		assert.NotEmpty(t, c.Label, "concept %q must have a non-empty Label", c.ID)
	}
}

func TestDesignPhilosophyRegistry_AllConcepts_HaveDescriptions(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, c := range r.AllConcepts() {
		assert.NotEmpty(t, c.Description, "concept %q must have a non-empty Description", c.ID)
	}
}

func TestDesignPhilosophyRegistry_AllConcepts_HaveDesignImplications(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, c := range r.AllConcepts() {
		assert.NotEmpty(t, c.DesignImplication,
			"concept %q must have a non-empty DesignImplication", c.ID)
	}
}

func TestDesignPhilosophyRegistry_AllConcepts_HavePDFSection(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, c := range r.AllConcepts() {
		assert.NotEmpty(t, c.PDFSection, "concept %q must reference a PDF section", c.ID)
	}
}

func TestDesignPhilosophyRegistry_ConceptsGroundedInPrinciple_MinimalScaffolding(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	grounded := r.ConceptsGroundedInPrinciple(PrincipleMinimalScaffoldingMaximalHarness)
	// DecisionLogicRatio, MinimalScaffoldingThesis, ConvergenceThesis are all grounded in it.
	assert.GreaterOrEqual(t, len(grounded), 3,
		"at least 3 §11.1 concepts are grounded in PrincipleMinimalScaffoldingMaximalHarness")
	ids := make([]PhilosophyConceptID, len(grounded))
	for i, c := range grounded {
		ids[i] = c.ID
	}
	assert.Contains(t, ids, ConceptDecisionLogicRatio)
	assert.Contains(t, ids, ConceptMinimalScaffoldingThesis)
	assert.Contains(t, ids, ConceptConvergenceThesis)
}

func TestDesignPhilosophyRegistry_ConceptsGroundedInPrinciple_UnknownPrinciple(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	grounded := r.ConceptsGroundedInPrinciple("nonexistent_principle")
	assert.Empty(t, grounded, "no concepts should be grounded in a nonexistent principle")
}

func TestDesignPhilosophyRegistry_OSKernelAnalogy_NotGroundedInPrinciple(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptOSKernelAnalogy)
	require.True(t, ok)
	assert.Empty(t, c.GroundedInPrinciple,
		"OSKernelAnalogy is a synthesis concept with no single grounding principle")
}

func TestDesignPhilosophyRegistry_AlternativeDesignFamilies_ConceptDescription(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptAlternativeDesignFamilies)
	require.True(t, ok)
	// Must mention all three alternative family systems.
	assert.True(t, strings.Contains(c.Description, "LangGraph"), "must mention LangGraph")
	assert.True(t, strings.Contains(c.Description, "SWEAgent"), "must mention SWEAgent")
	assert.True(t, strings.Contains(c.Description, "Aider"), "must mention Aider")
}

// --- Alternative Design Families ---

func TestDesignPhilosophyRegistry_FamilyCount(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	assert.Equal(t, 3, r.FamilyCount(), "§2.2 names exactly three alternative design families")
}

func TestDesignPhilosophyRegistry_AllAlternativeDesignFamilies_Length(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	fams := r.AllAlternativeDesignFamilies()
	assert.Len(t, fams, 3)
}

func TestDesignPhilosophyRegistry_AllAlternativeDesignFamilies_Order(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	fams := r.AllAlternativeDesignFamilies()
	assert.Equal(t, FamilyRuleBasedOrchestration, fams[0].ID)
	assert.Equal(t, FamilyContainerIsolatedExecution, fams[1].ID)
	assert.Equal(t, FamilyVersionControlAsSafety, fams[2].ID)
}

func TestDesignPhilosophyRegistry_FindFamilyByID_Known(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	knownIDs := []AlternativeDesignFamilyID{
		FamilyRuleBasedOrchestration,
		FamilyContainerIsolatedExecution,
		FamilyVersionControlAsSafety,
	}
	for _, id := range knownIDs {
		f, ok := r.FindFamilyByID(id)
		assert.True(t, ok, "FindFamilyByID(%q) should succeed", id)
		assert.Equal(t, id, f.ID)
	}
}

func TestDesignPhilosophyRegistry_FindFamilyByID_Unknown(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	_, ok := r.FindFamilyByID("no_such_family")
	assert.False(t, ok)
}

func TestDesignPhilosophyRegistry_IsValidFamilyID(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	assert.True(t, r.IsValidFamilyID(FamilyRuleBasedOrchestration))
	assert.True(t, r.IsValidFamilyID(FamilyContainerIsolatedExecution))
	assert.True(t, r.IsValidFamilyID(FamilyVersionControlAsSafety))
	assert.False(t, r.IsValidFamilyID("bogus_family"))
}

func TestDesignPhilosophyRegistry_FamilyByExemplar_LangGraph(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	f, ok := r.FamilyByExemplar("LangGraph")
	require.True(t, ok, "LangGraph must be found as an exemplar")
	assert.Equal(t, FamilyRuleBasedOrchestration, f.ID)
}

func TestDesignPhilosophyRegistry_FamilyByExemplar_SWEAgent(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	f, ok := r.FamilyByExemplar("SWEAgent")
	require.True(t, ok, "SWEAgent must be found as an exemplar")
	assert.Equal(t, FamilyContainerIsolatedExecution, f.ID)
}

func TestDesignPhilosophyRegistry_FamilyByExemplar_OpenHands(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	f, ok := r.FamilyByExemplar("OpenHands")
	require.True(t, ok, "OpenHands must be found as an exemplar")
	assert.Equal(t, FamilyContainerIsolatedExecution, f.ID)
}

func TestDesignPhilosophyRegistry_FamilyByExemplar_Aider(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	f, ok := r.FamilyByExemplar("Aider")
	require.True(t, ok, "Aider must be found as an exemplar")
	assert.Equal(t, FamilyVersionControlAsSafety, f.ID)
}

func TestDesignPhilosophyRegistry_FamilyByExemplar_Unknown(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	_, ok := r.FamilyByExemplar("Devin")
	assert.False(t, ok, "Devin is not named as an exemplar in §2.2")
}

func TestDesignPhilosophyRegistry_AllFamilies_HaveNonEmptyFields(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, f := range r.AllAlternativeDesignFamilies() {
		assert.NotEmpty(t, f.Label, "family %q must have a Label", f.ID)
		assert.NotEmpty(t, f.CoreMechanism, "family %q must have a CoreMechanism", f.ID)
		assert.NotEmpty(t, f.Contrast, "family %q must have a Contrast", f.ID)
		assert.NotEmpty(t, f.PDFSection, "family %q must have a PDFSection", f.ID)
		assert.NotEmpty(t, f.ExemplarSystems, "family %q must have at least one ExemplarSystem", f.ID)
	}
}

func TestDesignPhilosophyRegistry_AllFamilies_PDFSection_Is22(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	for _, f := range r.AllAlternativeDesignFamilies() {
		assert.Equal(t, "2.2", f.PDFSection,
			"all §2.2 alternative families should reference section 2.2")
	}
}

// --- Structural invariants ---

func TestDesignPhilosophyRegistry_StructuralInvariants_AllHold(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	results := r.StructuralInvariants()
	require.Len(t, results, 3, "StructuralInvariants must return exactly 3 results")
	for _, res := range results {
		assert.True(t, res.Holds, "invariant %q must hold: %s", res.Name, res.Message)
	}
}

func TestDesignPhilosophyRegistry_StructuralInvariant_FiveConcepts(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	var fiveConcepts *PhilosophyInvariantResult
	for i, res := range r.StructuralInvariants() {
		if res.Name == "FiveConcepts" {
			cp := r.StructuralInvariants()[i]
			fiveConcepts = &cp
			break
		}
	}
	require.NotNil(t, fiveConcepts, "FiveConcepts invariant must be present")
	assert.True(t, fiveConcepts.Holds)
}

func TestDesignPhilosophyRegistry_StructuralInvariant_OneQuantifiedConcept(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	var oneQ *PhilosophyInvariantResult
	for _, res := range r.StructuralInvariants() {
		if res.Name == "OneQuantifiedConcept" {
			cp := res
			oneQ = &cp
			break
		}
	}
	require.NotNil(t, oneQ, "OneQuantifiedConcept invariant must be present")
	assert.True(t, oneQ.Holds)
}

func TestDesignPhilosophyRegistry_StructuralInvariant_ThreeAlternativeFamilies(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	var threeF *PhilosophyInvariantResult
	for _, res := range r.StructuralInvariants() {
		if res.Name == "ThreeAlternativeFamilies" {
			cp := res
			threeF = &cp
			break
		}
	}
	require.NotNil(t, threeF, "ThreeAlternativeFamilies invariant must be present")
	assert.True(t, threeF.Holds)
}

func TestDesignPhilosophyRegistry_ConceptsWithDesignImplication_AllFive(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	withImpl := r.ConceptsWithDesignImplication()
	assert.Len(t, withImpl, 5,
		"all five §11.1 concepts have a DesignImplication")
}

func TestDesignPhilosophyRegistry_DecisionLogicRatio_GroundedInMinimalScaffolding(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptDecisionLogicRatio)
	require.True(t, ok)
	assert.Equal(t, PrincipleMinimalScaffoldingMaximalHarness, c.GroundedInPrinciple)
}

func TestDesignPhilosophyRegistry_ConvergenceThesis_PDFSection_Is111(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptConvergenceThesis)
	require.True(t, ok)
	assert.Equal(t, "11.1", c.PDFSection)
}

func TestDesignPhilosophyRegistry_AlternativeDesignFamilies_ConceptPDFSection_Is22(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptAlternativeDesignFamilies)
	require.True(t, ok)
	// The concept's PDFSection is §2.2 where the alternatives are named.
	assert.Equal(t, "2.2", c.PDFSection)
}

func TestDesignPhilosophyRegistry_RuleBasedOrchestration_ExemplarIsLangGraph(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	f, ok := r.FindFamilyByID(FamilyRuleBasedOrchestration)
	require.True(t, ok)
	assert.Contains(t, f.ExemplarSystems, "LangGraph")
}

func TestDesignPhilosophyRegistry_VersionControlAsSafety_ExemplarIsAider(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	f, ok := r.FindFamilyByID(FamilyVersionControlAsSafety)
	require.True(t, ok)
	assert.Contains(t, f.ExemplarSystems, "Aider")
}

func TestDesignPhilosophyRegistry_MinimalScaffoldingThesis_Description_MentionsHarness(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptMinimalScaffoldingThesis)
	require.True(t, ok)
	assert.True(t, strings.Contains(strings.ToLower(c.Description), "harness"),
		"MinimalScaffoldingThesis description must mention 'harness'")
}

func TestDesignPhilosophyRegistry_OSKernelAnalogy_Description_MentionsQueryLoop(t *testing.T) {
	r := NewDesignPhilosophyRegistry()
	c, ok := r.FindByID(ConceptOSKernelAnalogy)
	require.True(t, ok)
	assert.True(t, strings.Contains(c.Description, "queryLoop()"),
		"OSKernelAnalogy description must mention 'queryLoop()'")
}
