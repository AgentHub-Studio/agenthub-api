package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 1 design principles (arXiv:2604.14228v1, §2.2).

func TestBDD_DesignPrincipleRegistry(t *testing.T) {
	t.Run("Scenario_ThirteenPrinciplesMatchTable1RowCount", func(t *testing.T) {
		// Given Table 1 defines exactly thirteen design principles
		// When all principles are listed from the registry
		// Then exactly thirteen profiles are returned in Table 1 order
		r := NewDesignPrincipleRegistry()
		all := r.AllPrinciples()
		assert.Equal(t, 13, len(all))
		assert.Equal(t, PrincipleDenyFirstHumanEscalation, all[0].ID)
		assert.Equal(t, PrincipleGracefulRecoveryResilience, all[12].ID)
	})

	t.Run("Scenario_DefenseInDepthAndIsolatedSubagentServeThreeValues", func(t *testing.T) {
		// Given Table 1 shows defense_in_depth and isolated_subagent_boundaries
		//   each mapping to three design values
		// When their profiles are retrieved
		// Then each has exactly three values served
		r := NewDesignPrincipleRegistry()
		p1, ok1 := r.Profile(PrincipleDefenseInDepthLayered)
		p2, ok2 := r.Profile(PrincipleIsolatedSubagentBoundaries)
		assert.True(t, ok1)
		assert.True(t, ok2)
		assert.Equal(t, 3, len(p1.ValuesServed))
		assert.Equal(t, 3, len(p2.ValuesServed))
	})

	t.Run("Scenario_AllPrinciplesHaveDesignQuestionAndSections", func(t *testing.T) {
		// Given every Table 1 row has a design question and at least one referenced section
		// When all principle profiles are inspected
		// Then no design question or section list is empty
		r := NewDesignPrincipleRegistry()
		for _, p := range r.AllPrinciples() {
			assert.NotEmpty(t, p.DesignQuestion)
			assert.NotEmpty(t, p.ReferencedSections)
		}
	})

	t.Run("Scenario_PrinciplesServingCapabilityIncludesMinimalScaffolding", func(t *testing.T) {
		// Given Table 1 shows minimal_scaffolding_maximal_harness serves Capability
		// When principles serving capability are queried
		// Then minimal_scaffolding_maximal_harness is in the result
		r := NewDesignPrincipleRegistry()
		principles := r.PrinciplesServingValue(DesignValueCapability)
		ids := map[DesignPrincipleID]bool{}
		for _, p := range principles {
			ids[p.ID] = true
		}
		assert.True(t, ids[PrincipleMinimalScaffoldingMaximalHarness])
	})

	t.Run("Scenario_InvalidPrincipleIDRejected", func(t *testing.T) {
		// Given only the thirteen Table 1 slugs are valid
		// When an unrecognised ID is passed
		// Then IsValidPrinciple returns false and Profile returns false
		r := NewDesignPrincipleRegistry()
		assert.False(t, r.IsValidPrinciple("not_a_principle"))
		_, ok := r.Profile("not_a_principle")
		assert.False(t, ok)
	})
}
