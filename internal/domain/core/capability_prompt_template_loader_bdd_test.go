package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000094 capability prompt templates.
// These assert seed shape and subgroup coverage without requiring a database.

func TestBDD_CapabilityPromptTemplateSeed(t *testing.T) {
	t.Run("Scenario_FiveCapabilityTemplatesComplementThreeCapabilityAgents", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000094 seeds capability prompt templates,
		// Then exactly 5 templates exist — 2 for web research (researcher),
		//   2 for doc analysis (analyst), 1 for task workflow (planner).
		assert.Equal(t, 5, SeedCapabilityPromptTemplateCount,
			"migration 000094 must seed exactly 5 capability prompt templates")
		assert.Equal(t, 5, len(SeedCapabilityPromptTemplateSlugs),
			"slug list must contain exactly 5 entries")

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityPromptTemplateSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["capability-web-research-brief"],
			"web research brief template must exist for core-researcher")
		assert.True(t, slugSet["capability-doc-analysis-summary"],
			"doc analysis summary template must exist for core-analyst")
		assert.True(t, slugSet["capability-task-breakdown"],
			"task breakdown template must exist for core-planner")
		assert.True(t, slugSet["capability-competitive-research"],
			"competitive research template must exist (web research variant)")
		assert.True(t, slugSet["capability-knowledge-synthesis"],
			"knowledge synthesis template must exist (doc analysis variant)")
	})

	t.Run("Scenario_CapabilityTemplateSlugsDoNotOverlapWithPlatformTemplates", func(t *testing.T) {
		// Given the platform baseline (migration 000019) seeds 8 generic templates
		//   (general-assistant, code-reviewer, data-analyst, researcher, etc.),
		// When capability templates are introduced in migration 000094,
		// Then NO capability slug collides with a platform slug — the two layers
		//      are independently versioned and separately queryable.
		platformSet := map[string]bool{}
		for _, s := range SeedExpectedPromptTemplateSlugs {
			platformSet[s] = true
		}
		for _, slug := range SeedCapabilityPromptTemplateSlugs {
			assert.False(t, platformSet[slug],
				"capability template slug %q must NOT collide with platform template slug (migration 000019)",
				slug)
		}
		// Platform baseline count must remain unchanged.
		assert.Equal(t, 8, len(SeedExpectedPromptTemplateSlugs),
			"platform template count must remain 8 after migration 000094")
	})

	t.Run("Scenario_CapabilityKindIsDistinctFromAllPlatformKinds", func(t *testing.T) {
		// Given the platform template kinds are: assistant, coder, analyst, researcher,
		//   writer, translator, customer_support, data_extractor,
		// When the capability kind constant is inspected,
		// Then 'capability' is NOT in the platform kind set — it is a new kind
		//      introduced to distinguish user-facing capability starters from generic starters.
		assert.Equal(t, "capability", SeedCapabilityPromptTemplateKind,
			"capability kind must equal 'capability'")

		platformKindSet := map[string]bool{}
		for _, k := range SeedExpectedPromptTemplateKinds {
			platformKindSet[k] = true
		}
		assert.False(t, platformKindSet["capability"],
			"'capability' kind must NOT be present in the platform kind set (migration 000019)")
	})

	t.Run("Scenario_WebResearchAndDocAnalysisGroupsCoverFourOfFiveSlugs", func(t *testing.T) {
		// Given the two main capability skills are web research and doc analysis,
		// When the sub-group slices are inspected,
		// Then web research covers 2 slugs and doc analysis covers 2 slugs —
		//   together accounting for 4 of the 5 capability templates.
		assert.Equal(t, 2, len(SeedWebResearchPromptSlugs),
			"web research group must have 2 templates (brief + competitive)")
		assert.Equal(t, 2, len(SeedDocAnalysisPromptSlugs),
			"doc analysis group must have 2 templates (summary + synthesis)")

		combined := len(SeedWebResearchPromptSlugs) + len(SeedDocAnalysisPromptSlugs) + len(SeedTaskWorkflowPromptSlugs)
		assert.Equal(t, SeedCapabilityPromptTemplateCount, combined,
			"web research + doc analysis + task workflow must sum to the total capability template count")
	})

	t.Run("Scenario_TemplateSubgroupsPartitionAllFiveSlugsWithNoGapsOrOverlaps", func(t *testing.T) {
		// Given three functional sub-groups: web research (2), doc analysis (2),
		//   task workflow (1),
		// When the sub-group slices are unioned,
		// Then they cover exactly the 5 slugs in SeedCapabilityPromptTemplateSlugs
		//      with no duplicates and no missing entries — a clean partition.
		allGroups := map[string]int{}
		for _, s := range SeedWebResearchPromptSlugs {
			allGroups[s]++
		}
		for _, s := range SeedDocAnalysisPromptSlugs {
			allGroups[s]++
		}
		for _, s := range SeedTaskWorkflowPromptSlugs {
			allGroups[s]++
		}

		assert.Equal(t, SeedCapabilityPromptTemplateCount, len(allGroups),
			"sub-groups combined must cover exactly %d distinct slugs", SeedCapabilityPromptTemplateCount)

		for _, slug := range SeedCapabilityPromptTemplateSlugs {
			count := allGroups[slug]
			assert.Equal(t, 1, count,
				"slug %q must appear in exactly 1 sub-group (got %d)", slug, count)
		}
	})
}
