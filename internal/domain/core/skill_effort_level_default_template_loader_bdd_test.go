package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for SkillEffortLevelDefaultTemplate seed.
// Effort levels control extended thinking token budgets for skills,
// forming a five-rung ladder from lowest (no thinking) to highest
// (maximum thinking budget).

func TestBDD_AhCoreSkillEffortLevelSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesFiveEffortLevels", func(t *testing.T) {
		// Given a new tenant configures skill execution budgets
		// When the effort level catalog is loaded
		// Then exactly 5 levels are available: lowest/low/medium/high/highest
		assert.Equal(t, 5, SeedExpectedEffortLevelRowCount)
		assert.Equal(t, 5, len(SeedExpectedEffortLevelSlugs))
	})

	t.Run("Scenario_MediumIsTheDefaultEffortLevel", func(t *testing.T) {
		// Given most skills should operate with balanced thinking budgets
		// When the default effort level is resolved
		// Then medium is selected (moderate token budget, wide applicability)
		assert.Equal(t, "medium", SeedEffortLevelDefaultSlug)
		found := false
		for _, s := range SeedExpectedEffortLevelSlugs {
			if s == SeedEffortLevelDefaultSlug {
				found = true
			}
		}
		assert.True(t, found, "default slug must be in main catalog")
	})

	t.Run("Scenario_LowestLevelDisablesExtendedThinking", func(t *testing.T) {
		// Given some lightweight skills do not need extended thinking
		// When the lowest effort level is selected
		// Then thinking_tokens = 0 (extended thinking is disabled)
		assert.Equal(t, "lowest", SeedEffortLevelDisabledSlug)
		found := false
		for _, s := range SeedExpectedEffortLevelSlugs {
			if s == SeedEffortLevelDisabledSlug {
				found = true
			}
		}
		assert.True(t, found, "disabled slug must be in main catalog")
	})

	t.Run("Scenario_HighestLevelCapAt65536Tokens", func(t *testing.T) {
		// Given the highest effort level should cap at the model's thinking limit
		// When the maximum thinking_tokens constant is inspected
		// Then the cap is set to 65,536 tokens
		assert.Equal(t, 65536, SeedEffortLevelMaxTokens)
	})

	t.Run("Scenario_FiveRungedLadderCoversAllSlugs", func(t *testing.T) {
		// Given the effort ladder must span from disabled to maximum
		// When all expected slugs are inspected
		// Then all five rungs are present: lowest, low, medium, high, highest
		slugSet := map[string]bool{}
		for _, s := range SeedExpectedEffortLevelSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["lowest"])
		assert.True(t, slugSet["low"])
		assert.True(t, slugSet["medium"])
		assert.True(t, slugSet["high"])
		assert.True(t, slugSet["highest"])
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each effort level slug is validated
		// Then every slug matches ^[a-z0-9][a-z0-9-]*[a-z0-9]$
		for _, s := range SeedExpectedEffortLevelSlugs {
			assert.True(t, SeedEffortLevelSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})
}
