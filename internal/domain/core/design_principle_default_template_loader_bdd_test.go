package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 1 design principle seed in ah_core.

func TestBDD_AhCoreDesignPrincipleSeed(t *testing.T) {
	t.Run("Scenario_ThirteenPrinciplesMatchTable1RowCount", func(t *testing.T) {
		// Given Table 1 defines exactly thirteen design principles
		// When the seed constants are inspected
		// Then thirteen slugs exist with the expected identifiers
		assert.Equal(t, 13, SeedExpectedDesignPrincipleRowCount)
		assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "deny_first_human_escalation")
		assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "graceful_recovery_resilience")
	})

	t.Run("Scenario_ThreePrinciplesServeBothSafetyAndAuthority", func(t *testing.T) {
		// Given Table 1 shows deny_first, defense_in_depth, externalized_policy
		//   each serving both safety and authority
		// When safety+authority serving slugs are listed
		// Then exactly three slugs are present
		assert.Equal(t, 3, len(SeedDesignPrincipleSafetyAuthorityServingSlugs))
	})

	t.Run("Scenario_SevenPrinciplesServeCapability", func(t *testing.T) {
		// Given Table 1 shows seven principles serving the capability value
		// When capability serving slugs are listed
		// Then exactly seven slugs are present
		assert.Equal(t, 7, len(SeedDesignPrincipleCapabilityServingSlugs))
	})

	t.Run("Scenario_SubsetSlugsAreConsistentWithCanonicalList", func(t *testing.T) {
		// Given seed constants must be internally consistent
		// When subset slug lists are validated against the canonical list
		// Then every slug in subsets exists in the canonical list
		all := map[string]bool{}
		for _, s := range SeedExpectedDesignPrincipleSlugs {
			all[s] = true
		}
		for _, s := range SeedDesignPrincipleSafetyAuthorityServingSlugs {
			assert.True(t, all[s])
		}
		for _, s := range SeedDesignPrincipleCapabilityServingSlugs {
			assert.True(t, all[s])
		}
	})

	t.Run("Scenario_ContextAsScarceResourceAndMinimalScaffoldingServeCapabilityReliability", func(t *testing.T) {
		// Given Table 1 marks these two principles as serving Reliability+Capability
		// When the canonical slugs are inspected
		// Then both are present in the capability subset
		assert.Contains(t, SeedDesignPrincipleCapabilityServingSlugs, "context_as_scarce_resource")
		assert.Contains(t, SeedDesignPrincipleCapabilityServingSlugs, "minimal_scaffolding_maximal_harness")
	})
}
