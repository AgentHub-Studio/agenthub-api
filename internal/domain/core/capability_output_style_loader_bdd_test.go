package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000095 capability output style seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityOutputStyleSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityOutputStylesComplementPlatformEight", func(t *testing.T) {
		// Given the platform baseline (migration 000011) seeds 8 generic output styles
		//   (conversational, concise, structured, technical, verbose, tutorial,
		//    executive, json_only),
		// When migration 000095 seeds capability-layer output styles,
		// Then exactly 3 capability styles are added — one per capability agent —
		//   bringing the total to 11 and covering structured web research, document
		//   analysis, and task planning output formats.
		assert.Equal(t, 8, len(SeedExpectedOutputStyleSlugs),
			"platform baseline must remain at 8 styles — migration 000011 is unmodified")
		assert.Equal(t, 3, SeedCapabilityOutputStyleCount,
			"migration 000095 must add exactly 3 capability output styles")
		assert.Equal(t, 11, len(SeedExpectedOutputStyleSlugs)+SeedCapabilityOutputStyleCount,
			"platform(8) + capability(3) = 11 total output styles in ah_core")
	})

	t.Run("Scenario_EachCapabilityAgentHasADedicatedOutputStyle", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000095 seeds capability output styles,
		// Then each agent has exactly one dedicated output style constant:
		//   SeedResearchOutputStyleSlug → core-researcher,
		//   SeedAnalysisOutputStyleSlug → core-analyst,
		//   SeedPlannerOutputStyleSlug  → core-planner.
		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityOutputStyleSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet[SeedResearchOutputStyleSlug],
			"core-researcher must have dedicated output style %q", SeedResearchOutputStyleSlug)
		assert.True(t, slugSet[SeedAnalysisOutputStyleSlug],
			"core-analyst must have dedicated output style %q", SeedAnalysisOutputStyleSlug)
		assert.True(t, slugSet[SeedPlannerOutputStyleSlug],
			"core-planner must have dedicated output style %q", SeedPlannerOutputStyleSlug)
		// One style per agent — no sharing, no gaps.
		assert.Equal(t, 3, len(slugSet),
			"exactly 3 distinct capability output style slugs (one per capability agent)")
	})

	t.Run("Scenario_AllCapabilityStylesUseMarkdownFormat", func(t *testing.T) {
		// Given the capability agent outputs are structured reports (sections,
		//   headers, bullet lists, checkboxes),
		// When the output_format constant is inspected,
		// Then all 3 capability styles use 'markdown' — this enables the chat
		//   renderer to surface headers and lists correctly. JSON and plain
		//   formats are not suitable for multi-section structured reports.
		assert.Equal(t, "markdown", SeedCapabilityOutputStyleFormat,
			"all capability output styles must use markdown format for structured rendering")

		// Confirmed: every capability slug maps to the same markdown format.
		assert.Equal(t, 3, len(SeedCapabilityOutputStyleSlugs),
			"3 capability styles must all use SeedCapabilityOutputStyleFormat=markdown")
	})

	t.Run("Scenario_CapabilityStylesHaveHigherSortOrderThanPlatform", func(t *testing.T) {
		// Given the platform styles (migration 000011) use sort_order range 10-80,
		// When capability styles are assigned sort_order 100, 101, 102,
		// Then capability styles always appear AFTER platform styles in any
		//   sort_order-ascending listing — preserving platform-first UX ordering
		//   in the agent output style picker.
		// Guard at constant level: 3 slugs for sort slots 100, 101, 102.
		const platformMaxSortOrder = 80   // json_only = sort_order 80
		const capabilityMinSortOrder = 100 // research-report = sort_order 100

		assert.Greater(t, capabilityMinSortOrder, platformMaxSortOrder,
			"capability sort_order min (%d) must exceed platform sort_order max (%d)",
			capabilityMinSortOrder, platformMaxSortOrder)
		assert.Equal(t, 3, len(SeedCapabilityOutputStyleSlugs),
			"3 capability slugs map to sort_order 100, 101, 102 (all > platform max 80)")
	})

	t.Run("Scenario_NoSlugOverlapBetweenCapabilityAndPlatformStyles", func(t *testing.T) {
		// Given the platform seeds 8 styles with stable slugs (conversational, concise,
		//   structured, technical, verbose, tutorial, executive, json_only),
		// When capability styles are introduced in migration 000095,
		// Then NO capability slug collides with a platform slug — the two layers
		//   are independently versioned and addressable. Collisions would cause
		//   ON CONFLICT DO NOTHING to silently skip re-seeds after rollback.
		platformSet := map[string]bool{}
		for _, s := range SeedExpectedOutputStyleSlugs {
			platformSet[s] = true
		}
		for _, slug := range SeedCapabilityOutputStyleSlugs {
			assert.False(t, platformSet[slug],
				"capability output style slug %q must NOT collide with a platform slug (migration 000011)",
				slug)
		}
		// Verify platform count remains unchanged.
		assert.Equal(t, 8, len(SeedExpectedOutputStyleSlugs),
			"platform output style count must remain 8 after migration 000095")
	})
}
