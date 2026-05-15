package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD tests for design_philosophy_registry.go — FEAT-045 — §11.1 Design Philosophy
//
// Scenarios follow the Given / When / Then structure.

// Scenario 1: The registry exposes exactly five §11.1 design-philosophy concepts
// and three §2.2 alternative design families.
func TestDesignPhilosophyRegistry_BDD_RegistryShape(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	// When I ask for the concept count and the family count
	conceptCount := r.Count()
	familyCount := r.FamilyCount()

	// Then the counts match what §11.1 and §2.2 define
	assert.Equal(t, 5, conceptCount, "§11.1 defines exactly five design-philosophy concepts")
	assert.Equal(t, 3, familyCount, "§2.2 names exactly three alternative design families")
}

// Scenario 2: The 1.6%/98.4% decision-logic ratio is the only quantified claim
// and carries the correct numeric assertions.
func TestDesignPhilosophyRegistry_BDD_DecisionLogicRatioIsOnlyQuantifiedConcept(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	// When I query for quantified concepts
	quantified := r.QuantifiedConcepts()

	// Then exactly one concept is quantified
	require.Len(t, quantified, 1, "exactly one §11.1 concept carries a numeric claim")

	// And that concept is DecisionLogicRatio with the §11.1 values
	c := quantified[0]
	assert.Equal(t, ConceptDecisionLogicRatio, c.ID)
	assert.True(t, c.IsQuantified)
	assert.Contains(t, c.QuantitativeClaim, "1.6%",
		"QuantitativeClaim must capture the 1.6%% decision-logic figure")
	assert.Contains(t, c.QuantitativeClaim, "98.4%",
		"QuantitativeClaim must capture the 98.4%% harness figure")

	// And the concept is grounded in the MinimalScaffoldingMaximalHarness principle
	assert.Equal(t, PrincipleMinimalScaffoldingMaximalHarness, c.GroundedInPrinciple)
}

// Scenario 3: Looking up an alternative design family by exemplar system name
// returns the correct family and a clean not-found for unlisted systems.
func TestDesignPhilosophyRegistry_BDD_FamilyByExemplarLookup(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	cases := []struct {
		exemplar       string
		expectedFamily AlternativeDesignFamilyID
		shouldFind     bool
	}{
		{"LangGraph", FamilyRuleBasedOrchestration, true},
		{"SWEAgent", FamilyContainerIsolatedExecution, true},
		{"OpenHands", FamilyContainerIsolatedExecution, true},
		{"Aider", FamilyVersionControlAsSafety, true},
		{"Devin", "", false},
		{"GPT-Engineer", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("exemplar="+tc.exemplar, func(t *testing.T) {
			// When I look up the family by exemplar name
			f, ok := r.FamilyByExemplar(tc.exemplar)

			// Then the result matches the expected outcome
			assert.Equal(t, tc.shouldFind, ok, "lookup result for %q", tc.exemplar)
			if tc.shouldFind {
				assert.Equal(t, tc.expectedFamily, f.ID,
					"family for %q should be %q", tc.exemplar, tc.expectedFamily)
			}
		})
	}
}

// Scenario 4: The OSKernelAnalogy concept references queryLoop() in its description
// and is explicitly NOT grounded in any single Table 1 principle (it is a synthesis concept).
func TestDesignPhilosophyRegistry_BDD_OSKernelAnalogyIsSynthesisConcept(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	// When I retrieve the OSKernelAnalogy concept
	c, ok := r.FindByID(ConceptOSKernelAnalogy)
	require.True(t, ok, "OSKernelAnalogy must be in the registry")

	// Then it is not quantified
	assert.False(t, c.IsQuantified, "OSKernelAnalogy does not carry a numeric claim")

	// And its GroundedInPrinciple is empty (it synthesises the whole architecture)
	assert.Empty(t, c.GroundedInPrinciple,
		"OSKernelAnalogy is a synthesis concept with no single grounding principle")

	// And its description mentions queryLoop() as the kernel component
	assert.True(t, strings.Contains(c.Description, "queryLoop()"),
		"description must name queryLoop() as the kernel")

	// And its DesignImplication is non-empty
	assert.NotEmpty(t, c.DesignImplication)
}

// Scenario 5: Three concepts are grounded in PrincipleMinimalScaffoldingMaximalHarness;
// the convergence thesis explicitly links harness quality to frontier-model improvement.
func TestDesignPhilosophyRegistry_BDD_MinimalScaffoldingGrounding(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	// When I query for concepts grounded in the MinimalScaffoldingMaximalHarness principle
	grounded := r.ConceptsGroundedInPrinciple(PrincipleMinimalScaffoldingMaximalHarness)

	// Then at least three concepts are grounded in it
	require.GreaterOrEqual(t, len(grounded), 3,
		"DecisionLogicRatio, MinimalScaffoldingThesis, and ConvergenceThesis must all be grounded")

	groundedIDs := make(map[PhilosophyConceptID]bool)
	for _, c := range grounded {
		groundedIDs[c.ID] = true
	}

	// And the three expected concepts are present
	assert.True(t, groundedIDs[ConceptDecisionLogicRatio],
		"DecisionLogicRatio must be grounded in MinimalScaffoldingMaximalHarness")
	assert.True(t, groundedIDs[ConceptMinimalScaffoldingThesis],
		"MinimalScaffoldingThesis must be grounded in MinimalScaffoldingMaximalHarness")
	assert.True(t, groundedIDs[ConceptConvergenceThesis],
		"ConvergenceThesis must be grounded in MinimalScaffoldingMaximalHarness")

	// And the ConvergenceThesis description mentions frontier models and harness quality
	convergence, ok := r.FindByID(ConceptConvergenceThesis)
	require.True(t, ok)
	assert.True(t, strings.Contains(strings.ToLower(convergence.Description), "frontier"),
		"ConvergenceThesis description must mention frontier models")
	assert.True(t, strings.Contains(strings.ToLower(convergence.Description), "harness"),
		"ConvergenceThesis description must mention the operational harness")
}

// Scenario 6: All structural invariants defined in §11.1 hold for a fresh registry.
func TestDesignPhilosophyRegistry_BDD_StructuralInvariantsAllHold(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	// When I check the structural invariants
	results := r.StructuralInvariants()

	// Then the registry returns exactly three invariant results
	require.Len(t, results, 3,
		"StructuralInvariants must return one result per invariant")

	// And each invariant holds
	names := make([]string, len(results))
	for i, res := range results {
		names[i] = res.Name
		assert.True(t, res.Holds,
			"invariant %q must hold — message: %s", res.Name, res.Message)
	}

	// And the three named invariants are all present
	assert.Contains(t, names, "FiveConcepts")
	assert.Contains(t, names, "OneQuantifiedConcept")
	assert.Contains(t, names, "ThreeAlternativeFamilies")
}

// Scenario 7: The AlternativeDesignFamilies concept's description names all three
// exemplar systems and the concept's PDFSection refers to §2.2.
func TestDesignPhilosophyRegistry_BDD_AlternativeDesignFamiliesConceptContent(t *testing.T) {
	// Given a fresh DesignPhilosophyRegistry
	r := NewDesignPhilosophyRegistry()

	// When I retrieve the AlternativeDesignFamilies concept
	c, ok := r.FindByID(ConceptAlternativeDesignFamilies)
	require.True(t, ok, "AlternativeDesignFamilies must be in the registry")

	// Then its PDFSection is §2.2 where the alternatives are named
	assert.Equal(t, "2.2", c.PDFSection)

	// And its description mentions all three representative systems cited by §2.2
	exemplars := []string{"LangGraph", "SWEAgent", "Aider"}
	for _, ex := range exemplars {
		assert.True(t, strings.Contains(c.Description, ex),
			"description of AlternativeDesignFamilies must name %q", ex)
	}

	// And the concept is not quantified (it is a qualitative survey)
	assert.False(t, c.IsQuantified)

	// And it has a non-empty DesignImplication
	assert.NotEmpty(t, c.DesignImplication)
}
