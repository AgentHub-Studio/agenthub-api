package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 2 extension mechanism seed in ah_core.

func TestBDD_AhCoreExtensionMechanismSeed(t *testing.T) {
	t.Run("Scenario_FourMechanismsWithGraduatedContextCostFromTable2", func(t *testing.T) {
		// Given Table 2 defines exactly four extension mechanisms with context cost ordering
		// When the seed constants are inspected
		// Then four slugs exist: hooks/skills/plugins/mcp_servers
		assert.Equal(t, 4, SeedExpectedExtensionMechanismRowCount)
		assert.Contains(t, SeedExpectedExtensionMechanismSlugs, "hooks")
		assert.Contains(t, SeedExpectedExtensionMechanismSlugs, "mcp_servers")
	})

	t.Run("Scenario_OnlyHooksHaveZeroContextCostByDefault", func(t *testing.T) {
		// Given Table 2 states hooks have "Zero by default" context cost
		// When zero-cost mechanisms are listed
		// Then only hooks appears
		assert.Equal(t, 1, len(SeedExtensionMechanismZeroCostSlugs))
		assert.Equal(t, "hooks", SeedExtensionMechanismZeroCostSlugs[0])
	})

	t.Run("Scenario_OnlyPluginsCoverAllThreeInsertionPoints", func(t *testing.T) {
		// Given Table 2 shows plugins insertion point as "All three points"
		// When all-insertion-point mechanisms are listed
		// Then only plugins appears
		assert.Equal(t, 1, len(SeedExtensionMechanismAllInsertPointSlugs))
		assert.Equal(t, "plugins", SeedExtensionMechanismAllInsertPointSlugs[0])
	})

	t.Run("Scenario_FourDistinctInsertionPointsInSchema", func(t *testing.T) {
		// Given Figure 5 defines three loop phases plus a "all" wildcard
		// When valid insertion_point values are enumerated
		// Then assemble/model/execute/all are all present
		assert.Equal(t, 4, len(SeedExtensionMechanismInsertionPoints))
		for _, ip := range []string{"assemble", "model", "execute", "all"} {
			assert.Contains(t, SeedExtensionMechanismInsertionPoints, ip)
		}
	})

	t.Run("Scenario_SubsetConstantsAreConsistentWithCanonicalList", func(t *testing.T) {
		// Given seed constants must be internally consistent
		// When subset slug lists are validated
		// Then every slug in subsets exists in the canonical list
		all := map[string]bool{}
		for _, s := range SeedExpectedExtensionMechanismSlugs {
			all[s] = true
		}
		for _, s := range SeedExtensionMechanismZeroCostSlugs {
			assert.True(t, all[s])
		}
		for _, s := range SeedExtensionMechanismAllInsertPointSlugs {
			assert.True(t, all[s])
		}
	})
}
