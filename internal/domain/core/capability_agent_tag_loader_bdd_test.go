package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000109 capability agent tag seeds.
// These assert seed shape and tag rationale without a database.

func TestBDD_CapabilityAgentTagSeed(t *testing.T) {
	t.Run("Scenario_TwelveTagsFourPerCapabilityAgent", func(t *testing.T) {
		// Given the AgentHub marketplace needs searchable tags per capability agent
		//   to enable tag-based discovery and filtering in the catalog UI,
		// When migration 000109 seeds capability_agent_tag rows,
		// Then exactly 12 rows are added — four per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 12, SeedCapabilityAgentTagCount,
			"migration 000109 must seed exactly 12 capability agent tag rows")
		assert.Len(t, SeedCapabilityAgentAllTags, 12,
			"SeedCapabilityAgentAllTags must list exactly 12 tags")
		sum := SeedResearcherTagCount + SeedAnalystTagCount + SeedPlannerTagCount
		assert.Equal(t, SeedCapabilityAgentTagCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherTagCount, SeedAnalystTagCount, SeedPlannerTagCount,
			SeedCapabilityAgentTagCount)
		assert.Equal(t, 4, SeedResearcherTagCount,
			"researcher must have exactly 4 tags")
		assert.Equal(t, 4, SeedAnalystTagCount,
			"analyst must have exactly 4 tags")
		assert.Equal(t, 4, SeedPlannerTagCount,
			"planner must have exactly 4 tags")
	})

	t.Run("Scenario_ResearcherTagsEnableWebAndKnowledgeDiscovery", func(t *testing.T) {
		// Given a Researcher agent is discoverable via research-oriented query terms,
		// When migration 000109 seeds researcher tags,
		// Then 4 researcher tags exist covering research, web, sources, and knowledge
		//   — the primary discovery dimensions for the researcher agent.
		assert.Len(t, SeedResearcherTags, 4,
			"SeedResearcherTags must have exactly 4 entries")
		tagSet := map[string]struct{}{}
		for _, tag := range SeedResearcherTags {
			tagSet[tag] = struct{}{}
		}
		_, hasResearch := tagSet["research"]
		assert.True(t, hasResearch, "'research' must be in SeedResearcherTags")
		_, hasWeb := tagSet["web"]
		assert.True(t, hasWeb, "'web' must be in SeedResearcherTags")
		_, hasSources := tagSet["sources"]
		assert.True(t, hasSources, "'sources' must be in SeedResearcherTags")
		_, hasKnowledge := tagSet["knowledge"]
		assert.True(t, hasKnowledge, "'knowledge' must be in SeedResearcherTags")
	})

	t.Run("Scenario_AnalystTagsEnableDocumentWorkflowDiscovery", func(t *testing.T) {
		// Given an Analyst agent is discoverable via document and analysis query terms,
		// When migration 000109 seeds analyst tags,
		// Then 4 analyst tags exist covering analysis, documents, insights, and patterns
		//   — the primary discovery dimensions for the analyst agent.
		assert.Len(t, SeedAnalystTags, 4,
			"SeedAnalystTags must have exactly 4 entries")
		tagSet := map[string]struct{}{}
		for _, tag := range SeedAnalystTags {
			tagSet[tag] = struct{}{}
		}
		_, hasAnalysis := tagSet["analysis"]
		assert.True(t, hasAnalysis, "'analysis' must be in SeedAnalystTags")
		_, hasDocuments := tagSet["documents"]
		assert.True(t, hasDocuments, "'documents' must be in SeedAnalystTags")
		_, hasInsights := tagSet["insights"]
		assert.True(t, hasInsights, "'insights' must be in SeedAnalystTags")
		_, hasPatterns := tagSet["patterns"]
		assert.True(t, hasPatterns, "'patterns' must be in SeedAnalystTags")
	})

	t.Run("Scenario_PlannerTagsEnableProjectAndTaskDiscovery", func(t *testing.T) {
		// Given a Planner agent is discoverable via planning and task management terms,
		// When migration 000109 seeds planner tags,
		// Then 4 planner tags exist covering planning, tasks, projects, and workflows
		//   — the primary discovery dimensions for the planner agent.
		assert.Len(t, SeedPlannerTags, 4,
			"SeedPlannerTags must have exactly 4 entries")
		tagSet := map[string]struct{}{}
		for _, tag := range SeedPlannerTags {
			tagSet[tag] = struct{}{}
		}
		_, hasPlanning := tagSet["planning"]
		assert.True(t, hasPlanning, "'planning' must be in SeedPlannerTags")
		_, hasTasks := tagSet["tasks"]
		assert.True(t, hasTasks, "'tasks' must be in SeedPlannerTags")
		_, hasProjects := tagSet["projects"]
		assert.True(t, hasProjects, "'projects' must be in SeedPlannerTags")
		_, hasWorkflows := tagSet["workflows"]
		assert.True(t, hasWorkflows, "'workflows' must be in SeedPlannerTags")
		// Verify planner count accounts for its share of the total.
		remaining := SeedCapabilityAgentTagCount - SeedResearcherTagCount - SeedAnalystTagCount
		assert.Equal(t, SeedPlannerTagCount, remaining,
			"SeedPlannerTagCount must equal total minus researcher and analyst counts")
	})

	t.Run("Scenario_AllTwelveTagsAreDistinctAcrossAgents", func(t *testing.T) {
		// Given tag-based discovery must return unambiguous results per agent,
		// When migration 000109 seeds all 12 agent tag rows,
		// Then every tag in SeedCapabilityAgentAllTags is unique — no tag appears
		//   in more than one agent's tag list — and the combined list has 12 entries.
		assert.Len(t, SeedCapabilityAgentAllTags, 12,
			"combined tag list must have exactly 12 entries")
		unique := map[string]struct{}{}
		for _, tag := range SeedCapabilityAgentAllTags {
			unique[tag] = struct{}{}
		}
		assert.Len(t, unique, 12,
			"all 12 tags must be distinct — no duplicates across agents")
		// Agent slug list must have 3 distinct entries.
		assert.Len(t, SeedCapabilityAgentTagAgentSlugs, 3,
			"SeedCapabilityAgentTagAgentSlugs must list exactly 3 agents")
	})
}
