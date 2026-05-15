package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 6 context management approach seed in ah_core.

func TestBDD_AhCoreContextManagementApproachSeed(t *testing.T) {
	t.Run("Scenario_FiveApproachesMatchTable6RowCount", func(t *testing.T) {
		// Given Table 6 defines exactly five context-management strategies
		// When the seed constants are inspected
		// Then five slugs exist with Table 6 identifiers
		assert.Equal(t, 5, SeedExpectedContextManagementApproachRowCount)
		assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "simple_truncation")
		assert.Contains(t, SeedExpectedContextManagementApproachSlugs, "graduated_compaction")
	})

	t.Run("Scenario_GraduatedCompactionIsTheAgentHubApproach", func(t *testing.T) {
		// Given AgentHub adapts Claude Code's multi-layer compaction pipeline (§7.3)
		// When the AgentHub approach slug is inspected
		// Then it is graduated_compaction and exists in the canonical list
		assert.Equal(t, "graduated_compaction", SeedContextManagementAgentHubApproachSlug)
		assert.Contains(t, SeedExpectedContextManagementApproachSlugs, SeedContextManagementAgentHubApproachSlug)
	})

	t.Run("Scenario_TwoCoarseApproachesMatchTable6", func(t *testing.T) {
		// Given Table 6 marks simple_truncation and single_summarization as Coarse
		// When coarse approach slugs are listed
		// Then exactly two slugs are present
		assert.Equal(t, 2, len(SeedContextManagementCoarseApproachSlugs))
	})

	t.Run("Scenario_FourGranularityLevelsFormClosedSet", func(t *testing.T) {
		// Given Table 6 uses four granularity levels: coarse, medium, fine, very_fine
		// When all granularity values are enumerated
		// Then all four are present
		for _, g := range []string{"coarse", "medium", "fine", "very_fine"} {
			assert.Contains(t, SeedContextManagementGranularityValues, g)
		}
	})

	t.Run("Scenario_CoarseSlugsAreConsistentWithCanonicalList", func(t *testing.T) {
		// Given seed constants must be internally consistent
		// When coarse subset slugs are validated
		// Then every slug in the subset exists in the canonical list
		all := map[string]bool{}
		for _, s := range SeedExpectedContextManagementApproachSlugs {
			all[s] = true
		}
		for _, s := range SeedContextManagementCoarseApproachSlugs {
			assert.True(t, all[s])
		}
	})
}
