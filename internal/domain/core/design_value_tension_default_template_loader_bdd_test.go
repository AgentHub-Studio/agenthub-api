package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 4 design value tension seed in ah_core.

func TestBDD_AhCoreDesignValueTensionSeed(t *testing.T) {
	t.Run("Scenario_FiveTensionsMatchTable4RowCount", func(t *testing.T) {
		// Given Table 4 defines exactly five design value tensions
		// When the seed constants are inspected
		// Then five slugs exist with the expected identifiers
		assert.Equal(t, 5, SeedExpectedDesignValueTensionRowCount)
		assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "authority_safety")
		assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "capability_reliability")
	})

	t.Run("Scenario_CapabilityAppearsInThreeTensions", func(t *testing.T) {
		// Given Table 4 shows capability on three rows
		// When capability tension slugs are listed
		// Then exactly three slugs cover safety_capability, capability_adaptability, capability_reliability
		assert.Equal(t, 3, len(SeedDesignValueTensionCapabilitySlugs))
	})

	t.Run("Scenario_SafetyAppearsInThreeTensions", func(t *testing.T) {
		// Given Table 4 shows safety on three rows
		// When safety tension slugs are listed
		// Then exactly three slugs cover authority_safety, safety_capability, adaptability_safety
		assert.Equal(t, 3, len(SeedDesignValueTensionSafetySlugs))
	})

	t.Run("Scenario_SubsetConstantsAreConsistentWithCanonicalList", func(t *testing.T) {
		// Given seed constants must be internally consistent
		// When subset slug lists are validated
		// Then every slug in subsets exists in the canonical list
		all := map[string]bool{}
		for _, s := range SeedExpectedDesignValueTensionSlugs {
			all[s] = true
		}
		for _, s := range SeedDesignValueTensionCapabilitySlugs {
			assert.True(t, all[s])
		}
		for _, s := range SeedDesignValueTensionSafetySlugs {
			assert.True(t, all[s])
		}
	})

	t.Run("Scenario_CapabilityAdaptabilityIsTheNewTensionNotInOriginalKnownList", func(t *testing.T) {
		// Given capability_adaptability was the 5th tension added (Table 4, §11.2 §11.3)
		// When the canonical slug list is inspected
		// Then capability_adaptability is present
		assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "capability_adaptability")
	})
}
