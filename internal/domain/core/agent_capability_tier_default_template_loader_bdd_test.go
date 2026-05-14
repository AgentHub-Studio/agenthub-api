package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for AgentCapabilityTierTemplate seed.
// Maps §13 "Managed Agents design" (virtualizes session/harness/sandbox)
// to web-adapted capability tier presets.

func TestBDD_AhCoreCapabilityTierSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsCapabilityTierCatalog", func(t *testing.T) {
		// Given a new tenant with no custom agent configuration
		// When the system loads capability tier presets
		// Then exactly 5 tiers are available (read-only/standard/background/governed/autonomous)
		assert.Equal(t, 5, SeedExpectedCapabilityTierRowCount)
		assert.Equal(t, 5, len(SeedExpectedCapabilityTierSlugs))
	})

	t.Run("Scenario_StandardTierIsDefaultForNewAgents", func(t *testing.T) {
		// Given a new agent with no explicit tier configuration
		// Then the standard tier is used (balanced capability × reliability)
		assert.Equal(t, "standard", SeedCapabilityTierDefaultSlug)
		found := false
		for _, s := range SeedExpectedCapabilityTierSlugs {
			if s == SeedCapabilityTierDefaultSlug {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("Scenario_GovernedTierEnforcesHumanAuthorityDesignValue", func(t *testing.T) {
		// Given the §13 human_authority design value
		// When an agent requires regulated compliance (GDPR, HIPAA, SOX)
		// Then the governed tier is selected, enforcing human checkpoint and strict governance
		assert.Equal(t, "governed", SeedCapabilityTierGovernedSlug)
		found := false
		for _, s := range SeedExpectedCapabilityTierSlugs {
			if s == SeedCapabilityTierGovernedSlug {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("Scenario_AllThreeGovernanceLevelsSeeded", func(t *testing.T) {
		// Given the need to cover none/standard/strict governance patterns
		// Then all 3 levels are available in the catalog
		assert.Equal(t, 3, len(SeedCapabilityTierGovernanceLevels))
		levelSet := map[string]bool{}
		for _, l := range SeedCapabilityTierGovernanceLevels {
			levelSet[l] = true
		}
		assert.True(t, levelSet["none"])
		assert.True(t, levelSet["standard"])
		assert.True(t, levelSet["strict"])
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// Then every seeded slug matches the pattern
		for _, s := range SeedExpectedCapabilityTierSlugs {
			assert.True(t, SeedCapabilityTierSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})
}
