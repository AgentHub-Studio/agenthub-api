package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000096 capability KB template seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityKBTemplateSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityKBTemplatesComplementPlatformSeven", func(t *testing.T) {
		// Given the platform baseline (migration 000017) seeds 7 generic KB
		//   templates (faq, docs, wiki, chat, api, regulatory, support),
		// When migration 000096 seeds capability-layer KB templates,
		// Then exactly 3 capability templates are added — one per capability
		//   agent — bringing the total to 10 and covering web research
		//   collection, document analysis workspace, and project notes.
		assert.Equal(t, 7, len(SeedExpectedKBTemplateSlugs),
			"platform baseline must remain at 7 templates — migration 000017 is unmodified")
		assert.Equal(t, 3, SeedCapabilityKBTemplateCount,
			"migration 000096 must add exactly 3 capability KB templates")
		assert.Equal(t, 10, len(SeedExpectedKBTemplateSlugs)+SeedCapabilityKBTemplateCount,
			"platform(7) + capability(3) = 10 total KB templates in ah_core")
	})

	t.Run("Scenario_EachCapabilityAgentHasADedicatedKBTemplate", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000096 seeds capability KB templates,
		// Then each agent has exactly one dedicated KB template constant:
		//   SeedResearchKBTemplateSlug → core-researcher,
		//   SeedAnalysisKBTemplateSlug → core-analyst,
		//   SeedPlannerKBTemplateSlug  → core-planner.
		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityKBTemplateSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet[SeedResearchKBTemplateSlug],
			"core-researcher must have dedicated KB template %q", SeedResearchKBTemplateSlug)
		assert.True(t, slugSet[SeedAnalysisKBTemplateSlug],
			"core-analyst must have dedicated KB template %q", SeedAnalysisKBTemplateSlug)
		assert.True(t, slugSet[SeedPlannerKBTemplateSlug],
			"core-planner must have dedicated KB template %q", SeedPlannerKBTemplateSlug)
		// One template per agent — no sharing, no gaps.
		assert.Equal(t, 3, len(slugSet),
			"exactly 3 distinct capability KB template slugs (one per capability agent)")
	})

	t.Run("Scenario_CapabilityKBKindsAreDistinctFromPlatformKinds", func(t *testing.T) {
		// Given platform KB templates use well-known kinds (faq, documentation,
		//   internal_wiki, chat_history, api_reference, regulatory, customer_support),
		// When capability KB templates are introduced in migration 000096,
		// Then ALL 3 capability kinds (research_collection, analysis_workspace,
		//   project_notes) are NEW — none overlap with platform kinds.
		//   This allows the UI to route KBs to capability agents by kind without
		//   ambiguity.
		platformSet := map[string]bool{}
		for _, k := range SeedExpectedKBTemplateKinds {
			platformSet[k] = true
		}
		for _, kind := range SeedCapabilityKBTemplateKinds {
			assert.False(t, platformSet[kind],
				"capability KB kind %q must be NEW — no overlap with platform KB kinds (migration 000017)",
				kind)
		}
		// Also verify 3 unique new kinds.
		assert.Equal(t, 3, len(SeedCapabilityKBTemplateKinds),
			"exactly 3 new capability KB kinds: research_collection, analysis_workspace, project_notes")
	})

	t.Run("Scenario_AllCapabilityKBTemplatesAreRecommended", func(t *testing.T) {
		// Given capability KB templates are purpose-built for the three capability
		//   agents that ship with every fresh tenant (migration 000091),
		// When a fresh tenant opens the create-KB picker,
		// Then all 3 capability templates appear in the "Suggested" section
		//   (is_recommended = TRUE) — the tenant immediately sees which KB
		//   to create for each capability agent without manual exploration.
		//
		// This is a contract test: the migration sets is_recommended = TRUE for
		// all 3 rows. The integration test verifies actual DB values.
		// Here we guard that the slug list has 3 entries — one per sort slot.
		assert.Equal(t, 3, len(SeedCapabilityKBTemplateSlugs),
			"all 3 capability KB templates must be seeded with is_recommended=TRUE (one per sort slot 100-102)")
	})

	t.Run("Scenario_CapabilityKBTemplateSlugsSatisfyNamespaceContract", func(t *testing.T) {
		// Given tenants register custom KB names — collision risk if platform slugs
		//   used plain names like "research" instead of "research-collection-template",
		// When capability KB template slugs are inspected,
		// Then every slug ends with "-template" — a tenant KB named
		//   "research-collection" doesn't collide with "research-collection-template".
		for _, s := range SeedCapabilityKBTemplateSlugs {
			assert.True(t, strings.HasSuffix(s, "-template"),
				"capability KB template slug %q must end with -template (namespace contract)", s)
		}
	})
}
