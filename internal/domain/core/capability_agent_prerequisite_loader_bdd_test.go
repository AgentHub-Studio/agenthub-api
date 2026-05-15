package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000110 capability agent prerequisite seeds.
// These assert seed shape and prerequisite rationale without a database.

func TestBDD_CapabilityAgentPrerequisiteSeed(t *testing.T) {
	t.Run("Scenario_SixPrerequisitesTwoPerCapabilityAgent", func(t *testing.T) {
		// Given the AgentHub capability system needs a dependency manifest per agent
		//   to encode the minimum viable configuration each agent requires to function,
		// When migration 000110 seeds capability_agent_prerequisite rows,
		// Then exactly 6 rows are added — two per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 6, SeedCapabilityAgentPrerequisiteCount,
			"migration 000110 must seed exactly 6 capability agent prerequisite rows")
		sum := SeedResearcherPrerequisiteCount + SeedAnalystPrerequisiteCount + SeedPlannerPrerequisiteCount
		assert.Equal(t, SeedCapabilityAgentPrerequisiteCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherPrerequisiteCount, SeedAnalystPrerequisiteCount, SeedPlannerPrerequisiteCount,
			SeedCapabilityAgentPrerequisiteCount)
		assert.Equal(t, 2, SeedResearcherPrerequisiteCount,
			"researcher must have exactly 2 prerequisites")
		assert.Equal(t, 2, SeedAnalystPrerequisiteCount,
			"analyst must have exactly 2 prerequisites")
		assert.Equal(t, 2, SeedPlannerPrerequisiteCount,
			"planner must have exactly 2 prerequisites")
		// Agent slug list must have 3 distinct entries.
		assert.Len(t, SeedCapabilityAgentPrerequisiteAgentSlugs, 3,
			"SeedCapabilityAgentPrerequisiteAgentSlugs must list exactly 3 agents")
	})

	t.Run("Scenario_EachAgentRequiresOneFeatureFlagAndOneSkill", func(t *testing.T) {
		// Given each capability agent must have exactly one feature_flag prerequisite
		//   and one skill prerequisite to capture both flag gating and tool availability,
		// When migration 000110 seeds prerequisites,
		// Then the two prerequisite type constants ("feature_flag" and "skill") are
		//   non-empty, distinct, and cover both dependency dimensions.
		assert.Equal(t, "feature_flag", SeedPrerequisiteTypeFeatureFlag,
			"feature flag type constant must equal 'feature_flag'")
		assert.Equal(t, "skill", SeedPrerequisiteTypeSkill,
			"skill type constant must equal 'skill'")
		assert.NotEqual(t, SeedPrerequisiteTypeFeatureFlag, SeedPrerequisiteTypeSkill,
			"the two prerequisite type constants must be distinct")
		// All 6 seeded prerequisites are required.
		assert.True(t, SeedAllPrerequisitesRequired,
			"SeedAllPrerequisitesRequired must be true — all prerequisites are mandatory")
		// 6 unique slugs confirm two distinct rows per agent.
		slugs := []string{
			SeedResearcherCitationsFlagPrereqSlug,
			SeedResearcherWebResearchSkillPrereqSlug,
			SeedAnalystDocCitationsFlagPrereqSlug,
			SeedAnalystDocAnalysisSkillPrereqSlug,
			SeedPlannerTaskTrackingFlagPrereqSlug,
			SeedPlannerTaskWorkflowSkillPrereqSlug,
		}
		unique := map[string]struct{}{}
		for _, s := range slugs {
			unique[s] = struct{}{}
		}
		assert.Len(t, unique, 6,
			"all 6 prerequisite slug constants must be distinct")
	})

	t.Run("Scenario_ResearcherPrerequisitesCitationsAndWebResearch", func(t *testing.T) {
		// Given the Researcher agent cites sources automatically and performs web searches,
		// When migration 000110 seeds researcher prerequisites,
		// Then the researcher has a feature_flag prerequisite referencing capability-citations
		//   and a skill prerequisite referencing core-web-research.
		assert.NotEmpty(t, SeedResearcherCitationsFlagPrereqSlug,
			"researcher citations flag prerequisite slug must not be empty")
		assert.NotEmpty(t, SeedResearcherWebResearchSkillPrereqSlug,
			"researcher web research skill prerequisite slug must not be empty")
		assert.Contains(t, SeedResearcherCitationsFlagPrereqSlug, "citations",
			"researcher citations flag slug must reference 'citations'")
		assert.Contains(t, SeedResearcherWebResearchSkillPrereqSlug, "web-research",
			"researcher web research skill slug must reference 'web-research'")
		assert.Equal(t, 2, SeedResearcherPrerequisiteCount,
			"researcher must have exactly 2 prerequisites (1 flag + 1 skill)")
	})

	t.Run("Scenario_AnalystPrerequisitesDocCitationsAndDocAnalysis", func(t *testing.T) {
		// Given the Analyst agent cites document sources and performs document analysis,
		// When migration 000110 seeds analyst prerequisites,
		// Then the analyst has a feature_flag prerequisite referencing capability-doc-citations
		//   and a skill prerequisite referencing core-doc-analysis.
		assert.NotEmpty(t, SeedAnalystDocCitationsFlagPrereqSlug,
			"analyst doc citations flag prerequisite slug must not be empty")
		assert.NotEmpty(t, SeedAnalystDocAnalysisSkillPrereqSlug,
			"analyst doc analysis skill prerequisite slug must not be empty")
		assert.Contains(t, SeedAnalystDocCitationsFlagPrereqSlug, "doc-citations",
			"analyst doc citations flag slug must reference 'doc-citations'")
		assert.Contains(t, SeedAnalystDocAnalysisSkillPrereqSlug, "doc-analysis",
			"analyst doc analysis skill slug must reference 'doc-analysis'")
		assert.Equal(t, 2, SeedAnalystPrerequisiteCount,
			"analyst must have exactly 2 prerequisites (1 flag + 1 skill)")
	})

	t.Run("Scenario_PlannerPrerequisitesTaskTrackingAndTaskWorkflow", func(t *testing.T) {
		// Given the Planner agent tracks tasks automatically and manages workflows,
		// When migration 000110 seeds planner prerequisites,
		// Then the planner has a feature_flag prerequisite referencing capability-task-tracking
		//   and a skill prerequisite referencing core-task-workflow.
		assert.NotEmpty(t, SeedPlannerTaskTrackingFlagPrereqSlug,
			"planner task tracking flag prerequisite slug must not be empty")
		assert.NotEmpty(t, SeedPlannerTaskWorkflowSkillPrereqSlug,
			"planner task workflow skill prerequisite slug must not be empty")
		assert.Contains(t, SeedPlannerTaskTrackingFlagPrereqSlug, "task-tracking",
			"planner task tracking flag slug must reference 'task-tracking'")
		assert.Contains(t, SeedPlannerTaskWorkflowSkillPrereqSlug, "task-workflow",
			"planner task workflow skill slug must reference 'task-workflow'")
		assert.Equal(t, 2, SeedPlannerPrerequisiteCount,
			"planner must have exactly 2 prerequisites (1 flag + 1 skill)")
		// Verify planner count accounts for its share of the total.
		remaining := SeedCapabilityAgentPrerequisiteCount - SeedResearcherPrerequisiteCount - SeedAnalystPrerequisiteCount
		assert.Equal(t, SeedPlannerPrerequisiteCount, remaining,
			"SeedPlannerPrerequisiteCount must equal total minus researcher and analyst counts")
	})
}
