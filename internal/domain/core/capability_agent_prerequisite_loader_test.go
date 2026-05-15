package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent prerequisite seed constants (migration 000110).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentPrerequisiteCount_IsSix(t *testing.T) {
	assert.Equal(t, 6, SeedCapabilityAgentPrerequisiteCount,
		"migration 000110 seeds exactly 6 capability agent prerequisite rows (two per capability agent)")
}

func TestSeedCapabilityAgentPrerequisiteAgentSlugs_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentPrerequisiteAgentSlugs, 3,
		"SeedCapabilityAgentPrerequisiteAgentSlugs must have exactly 3 entries (researcher, analyst, planner)")
}

func TestSeedResearcherPrerequisiteCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedResearcherPrerequisiteCount,
		"SeedResearcherPrerequisiteCount must be 2 — one feature_flag + one skill")
}

func TestSeedAnalystPrerequisiteCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedAnalystPrerequisiteCount,
		"SeedAnalystPrerequisiteCount must be 2 — one feature_flag + one skill")
}

func TestSeedPlannerPrerequisiteCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedPlannerPrerequisiteCount,
		"SeedPlannerPrerequisiteCount must be 2 — one feature_flag + one skill")
}

func TestSeedPerAgentPrerequisiteCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherPrerequisiteCount + SeedAnalystPrerequisiteCount + SeedPlannerPrerequisiteCount
	assert.Equal(t, SeedCapabilityAgentPrerequisiteCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherPrerequisiteCount, SeedAnalystPrerequisiteCount, SeedPlannerPrerequisiteCount,
		SeedCapabilityAgentPrerequisiteCount)
}

func TestSeedPrerequisiteTypeFeatureFlag_IsCorrectString(t *testing.T) {
	assert.Equal(t, "feature_flag", SeedPrerequisiteTypeFeatureFlag,
		"SeedPrerequisiteTypeFeatureFlag must equal the string \"feature_flag\"")
}

func TestSeedPrerequisiteTypeSkill_IsCorrectString(t *testing.T) {
	assert.Equal(t, "skill", SeedPrerequisiteTypeSkill,
		"SeedPrerequisiteTypeSkill must equal the string \"skill\"")
}

func TestSeedAllPrerequisitesRequired_IsTrue(t *testing.T) {
	assert.True(t, SeedAllPrerequisitesRequired,
		"SeedAllPrerequisitesRequired must be true — all 6 seeded prerequisites are required")
}

func TestSeedPrerequisiteSlugConstants_AllNonEmpty(t *testing.T) {
	slugs := []string{
		SeedResearcherCitationsFlagPrereqSlug,
		SeedResearcherWebResearchSkillPrereqSlug,
		SeedAnalystDocCitationsFlagPrereqSlug,
		SeedAnalystDocAnalysisSkillPrereqSlug,
		SeedPlannerTaskTrackingFlagPrereqSlug,
		SeedPlannerTaskWorkflowSkillPrereqSlug,
	}
	for _, s := range slugs {
		assert.NotEmpty(t, s, "every prerequisite slug constant must be non-empty")
	}
}

func TestSeedPrerequisiteSlugConstants_AllSixAreUnique(t *testing.T) {
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
		"all 6 prerequisite slug constants must be distinct — no duplicates")
}

func TestSeedResearcherCitationsFlagPrereqSlug_ReferencesCitationsFlag(t *testing.T) {
	assert.Contains(t, SeedResearcherCitationsFlagPrereqSlug, "citations",
		"researcher citations flag prerequisite slug must reference 'citations'")
}

func TestSeedAnalystDocAnalysisSkillPrereqSlug_ReferencesDocAnalysis(t *testing.T) {
	assert.Contains(t, SeedAnalystDocAnalysisSkillPrereqSlug, "doc-analysis",
		"analyst doc analysis skill prerequisite slug must reference 'doc-analysis'")
}

func TestSeedPlannerTaskWorkflowSkillPrereqSlug_ReferencesTaskWorkflow(t *testing.T) {
	assert.Contains(t, SeedPlannerTaskWorkflowSkillPrereqSlug, "task-workflow",
		"planner task workflow skill prerequisite slug must reference 'task-workflow'")
}

func TestSeedCapabilityAgentPrerequisiteAgentSlugs_ContainsAllThreeAgents(t *testing.T) {
	slugSet := map[string]struct{}{}
	for _, s := range SeedCapabilityAgentPrerequisiteAgentSlugs {
		slugSet[s] = struct{}{}
	}
	_, hasResearcher := slugSet["core-researcher"]
	assert.True(t, hasResearcher, "SeedCapabilityAgentPrerequisiteAgentSlugs must contain 'core-researcher'")

	_, hasAnalyst := slugSet["core-analyst"]
	assert.True(t, hasAnalyst, "SeedCapabilityAgentPrerequisiteAgentSlugs must contain 'core-analyst'")

	_, hasPlanner := slugSet["core-planner"]
	assert.True(t, hasPlanner, "SeedCapabilityAgentPrerequisiteAgentSlugs must contain 'core-planner'")
}
