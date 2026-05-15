package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for coding_agent_category_template seed.
// Maps §13.1 Table 5 (four coding tool categories) to AgentHub web platform positioning.

func TestBDD_AhCoreCodingAgentCategorySeed(t *testing.T) {
	t.Run("Scenario_FourCategoriesCoverFullAutonomySpectrum", func(t *testing.T) {
		// Given the §13.1 taxonomy defining the design space of AI coding tools
		// When the platform loads category presets
		// Then exactly 4 categories are available spanning passive to autonomous
		assert.Equal(t, 4, SeedExpectedCodingAgentCategoryRowCount)
		assert.Equal(t, 4, len(SeedExpectedCodingAgentCategorySlugs))
	})

	t.Run("Scenario_AgentHubPrimarilyTargetsChatIntegrated", func(t *testing.T) {
		// Given AgentHub is a web-first multi-tenant agent platform
		// When the platform selects its default deployment category
		// Then chat_integrated is used (web chat interface, multi-turn, not CLI-bound)
		assert.Equal(t, "chat_integrated", SeedCodingAgentCategoryDefaultSlug)
		found := false
		for _, s := range SeedExpectedCodingAgentCategorySlugs {
			if s == SeedCodingAgentCategoryDefaultSlug {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("Scenario_ExactlyTwoCategoriesAreAgentHubTargets", func(t *testing.T) {
		// Given AgentHub supports both interactive and background agents
		// When querying which categories AgentHub covers
		// Then chat_integrated (interactive) and agentic_cli (background/KAIROS) are targets
		assert.Equal(t, 2, len(SeedCodingAgentCategoryAgentHubSlugs))
		assert.Contains(t, SeedCodingAgentCategoryAgentHubSlugs, "chat_integrated")
		assert.Contains(t, SeedCodingAgentCategoryAgentHubSlugs, "agentic_cli")
	})

	t.Run("Scenario_GradientOrderPassiveToAutonomous", func(t *testing.T) {
		// Given the taxonomy is an ordered spectrum
		// When categories are listed in gradient order
		// Then inline_completion is first and fully_autonomous is last
		assert.Equal(t, "inline_completion", SeedExpectedCodingAgentCategorySlugs[0])
		assert.Equal(t, "fully_autonomous", SeedExpectedCodingAgentCategorySlugs[3])
	})

	t.Run("Scenario_AllSlugsMustMatchSnakeCasePattern", func(t *testing.T) {
		// Given naming conventions require snake_case slugs for categories
		// Then every seeded slug matches the pattern
		for _, s := range SeedExpectedCodingAgentCategorySlugs {
			assert.True(t, SeedCodingAgentCategorySlugRE.MatchString(s),
				"slug %q violates snake_case pattern", s)
		}
	})
}
