package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000108 capability example prompt seeds.
// These assert seed shape and example prompt rationale without a database.

func TestBDD_CapabilityExamplePromptSeed(t *testing.T) {
	t.Run("Scenario_TwelveExamplePromptsFourPerAgent", func(t *testing.T) {
		// Given the AgentHub web UI needs example prompts to help new users
		//   understand what each capability agent can do,
		// When migration 000108 seeds capability_example_prompt rows,
		// Then exactly 12 rows are added — four per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 12, SeedCapabilityExamplePromptCount,
			"migration 000108 must seed exactly 12 capability example prompt rows")
		assert.Len(t, SeedCapabilityExamplePromptSlugs, 12,
			"SeedCapabilityExamplePromptSlugs must list exactly 12 slugs")
		sum := SeedResearcherExamplePromptCount + SeedAnalystExamplePromptCount + SeedPlannerExamplePromptCount
		assert.Equal(t, SeedCapabilityExamplePromptCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherExamplePromptCount, SeedAnalystExamplePromptCount, SeedPlannerExamplePromptCount,
			SeedCapabilityExamplePromptCount)
		assert.Equal(t, 4, SeedResearcherExamplePromptCount,
			"researcher must have exactly 4 example prompts")
		assert.Equal(t, 4, SeedAnalystExamplePromptCount,
			"analyst must have exactly 4 example prompts")
		assert.Equal(t, 4, SeedPlannerExamplePromptCount,
			"planner must have exactly 4 example prompts")
	})

	t.Run("Scenario_ResearcherExamplesCoversWebAndCompetitiveResearch", func(t *testing.T) {
		// Given a Researcher agent excels at web research, competitive analysis,
		//   technical research, and market research,
		// When migration 000108 seeds researcher example prompts,
		// Then 4 researcher example prompts exist covering all four research
		//   categories, and slugs follow the 'example-researcher-*' naming convention.
		assert.Len(t, SeedResearcherExamplePromptSlugs, 4,
			"SeedResearcherExamplePromptSlugs must have exactly 4 entries")
		slugSet := map[string]struct{}{}
		for _, s := range SeedCapabilityExamplePromptSlugs {
			slugSet[s] = struct{}{}
		}
		_, hasWebResearch := slugSet["example-researcher-web-research"]
		assert.True(t, hasWebResearch,
			"example-researcher-web-research must be present in SeedCapabilityExamplePromptSlugs")
		_, hasCompetitor := slugSet["example-researcher-competitor"]
		assert.True(t, hasCompetitor,
			"example-researcher-competitor must be present in SeedCapabilityExamplePromptSlugs")
		_, hasTechnical := slugSet["example-researcher-technical"]
		assert.True(t, hasTechnical,
			"example-researcher-technical must be present in SeedCapabilityExamplePromptSlugs")
		_, hasMarket := slugSet["example-researcher-market"]
		assert.True(t, hasMarket,
			"example-researcher-market must be present in SeedCapabilityExamplePromptSlugs")
		for _, s := range SeedResearcherExamplePromptSlugs {
			assert.True(t, strings.HasPrefix(s, "example-researcher-"),
				"researcher slug %q must start with 'example-researcher-'", s)
		}
	})

	t.Run("Scenario_AnalystExamplesCoverDocumentAnalysisWorkflows", func(t *testing.T) {
		// Given an Analyst agent excels at document summarization, comparative
		//   analysis, structured data extraction, and pattern identification,
		// When migration 000108 seeds analyst example prompts,
		// Then 4 analyst example prompts exist covering all four analysis
		//   workflows, and slugs follow the 'example-analyst-*' naming convention.
		assert.Len(t, SeedAnalystExamplePromptSlugs, 4,
			"SeedAnalystExamplePromptSlugs must have exactly 4 entries")
		slugSet := map[string]struct{}{}
		for _, s := range SeedCapabilityExamplePromptSlugs {
			slugSet[s] = struct{}{}
		}
		_, hasDocSummary := slugSet["example-analyst-doc-summary"]
		assert.True(t, hasDocSummary,
			"example-analyst-doc-summary must be present in SeedCapabilityExamplePromptSlugs")
		_, hasCompare := slugSet["example-analyst-compare"]
		assert.True(t, hasCompare,
			"example-analyst-compare must be present in SeedCapabilityExamplePromptSlugs")
		_, hasExtract := slugSet["example-analyst-extract"]
		assert.True(t, hasExtract,
			"example-analyst-extract must be present in SeedCapabilityExamplePromptSlugs")
		_, hasPattern := slugSet["example-analyst-pattern"]
		assert.True(t, hasPattern,
			"example-analyst-pattern must be present in SeedCapabilityExamplePromptSlugs")
		for _, s := range SeedAnalystExamplePromptSlugs {
			assert.True(t, strings.HasPrefix(s, "example-analyst-"),
				"analyst slug %q must start with 'example-analyst-'", s)
		}
	})

	t.Run("Scenario_PlannerExamplesCoverProjectAndTaskPlanning", func(t *testing.T) {
		// Given a Planner agent excels at project planning, task breakdown,
		//   release planning, and debugging approach design,
		// When migration 000108 seeds planner example prompts,
		// Then 4 planner example prompts exist covering all four planning
		//   scenarios, and slugs follow the 'example-planner-*' naming convention.
		assert.Len(t, SeedPlannerExamplePromptSlugs, 4,
			"SeedPlannerExamplePromptSlugs must have exactly 4 entries")
		slugSet := map[string]struct{}{}
		for _, s := range SeedCapabilityExamplePromptSlugs {
			slugSet[s] = struct{}{}
		}
		_, hasProject := slugSet["example-planner-project"]
		assert.True(t, hasProject,
			"example-planner-project must be present in SeedCapabilityExamplePromptSlugs")
		_, hasSprint := slugSet["example-planner-sprint"]
		assert.True(t, hasSprint,
			"example-planner-sprint must be present in SeedCapabilityExamplePromptSlugs")
		_, hasRelease := slugSet["example-planner-release"]
		assert.True(t, hasRelease,
			"example-planner-release must be present in SeedCapabilityExamplePromptSlugs")
		_, hasDebug := slugSet["example-planner-debug"]
		assert.True(t, hasDebug,
			"example-planner-debug must be present in SeedCapabilityExamplePromptSlugs")
		for _, s := range SeedPlannerExamplePromptSlugs {
			assert.True(t, strings.HasPrefix(s, "example-planner-"),
				"planner slug %q must start with 'example-planner-'", s)
		}
		// Verify planner count accounts for its share of the total.
		remaining := SeedCapabilityExamplePromptCount - SeedResearcherExamplePromptCount - SeedAnalystExamplePromptCount
		assert.Equal(t, SeedPlannerExamplePromptCount, remaining,
			"SeedPlannerExamplePromptCount must equal total minus researcher and analyst counts")
	})

	t.Run("Scenario_AllExampleSlugsStartWithExamplePrefix", func(t *testing.T) {
		// Given a consistent slug namespace is required for all example prompts
		//   to enable prefix-based filtering in the UI and API,
		// When migration 000108 seeds all 12 example prompt rows,
		// Then every slug in SeedCapabilityExamplePromptSlugs starts with
		//   'example-', is unique, and the combined list has 12 entries.
		assert.Len(t, SeedCapabilityExamplePromptSlugs, 12,
			"combined slug list must have exactly 12 entries")
		unique := map[string]struct{}{}
		for _, slug := range SeedCapabilityExamplePromptSlugs {
			assert.True(t, strings.HasPrefix(slug, "example-"),
				"slug %q must start with 'example-' prefix", slug)
			unique[slug] = struct{}{}
		}
		assert.Len(t, unique, 12,
			"all 12 slugs must be distinct — no duplicates across agents")
		// Agent slug list must have 3 distinct entries starting with 'core-'.
		assert.Len(t, SeedCapabilityExamplePromptAgentSlugs, 3,
			"SeedCapabilityExamplePromptAgentSlugs must list exactly 3 agents")
		for _, agentSlug := range SeedCapabilityExamplePromptAgentSlugs {
			assert.True(t, strings.HasPrefix(agentSlug, "core-"),
				"agent slug %q must start with 'core-'", agentSlug)
		}
	})
}
