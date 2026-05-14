package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuggestedFollowup_SeedSuggestedFollowupCount(t *testing.T) {
	assert.Equal(t, 9, SeedSuggestedFollowupCount)
}

func TestSuggestedFollowup_SeedSuggestedFollowupAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedSuggestedFollowupAgentCount)
}

func TestSuggestedFollowup_SeedSuggestedFollowupPerAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedSuggestedFollowupPerAgentCount)
}

func TestSuggestedFollowup_TotalRowsEqualAgentsTimesPerAgent(t *testing.T) {
	assert.Equal(t, SeedSuggestedFollowupCount, SeedSuggestedFollowupAgentCount*SeedSuggestedFollowupPerAgentCount)
}

func TestSuggestedFollowup_DisplayOrderMinimum(t *testing.T) {
	assert.Equal(t, 1, SeedSuggestedFollowupMinDisplayOrder)
}

func TestSuggestedFollowup_DisplayOrderMaximum(t *testing.T) {
	assert.Equal(t, 3, SeedSuggestedFollowupMaxDisplayOrder)
}

func TestSuggestedFollowup_DisplayOrderRangeMatchesPerAgentCount(t *testing.T) {
	got := SeedSuggestedFollowupMaxDisplayOrder - SeedSuggestedFollowupMinDisplayOrder + 1
	assert.Equal(t, SeedSuggestedFollowupPerAgentCount, got)
}

func TestSuggestedFollowup_ResearcherSlugs(t *testing.T) {
	assert.Equal(t, "research-current-sources", SeedResearcherFollowupCurrentSources)
	assert.Equal(t, "compare-official-and-recent", SeedResearcherFollowupCompareRecent)
	assert.Equal(t, "fact-check-claim", SeedResearcherFollowupFactCheckClaim)
}

func TestSuggestedFollowup_AnalystSlugs(t *testing.T) {
	assert.Equal(t, "extract-key-patterns", SeedAnalystFollowupExtractPatterns)
	assert.Equal(t, "compare-options", SeedAnalystFollowupCompareOptions)
	assert.Equal(t, "turn-findings-into-actions", SeedAnalystFollowupFindingsIntoActions)
}

func TestSuggestedFollowup_PlannerSlugs(t *testing.T) {
	assert.Equal(t, "break-into-milestones", SeedPlannerFollowupBreakIntoMilestones)
	assert.Equal(t, "identify-risks-and-owners", SeedPlannerFollowupRisksAndOwners)
	assert.Equal(t, "weekly-checklist", SeedPlannerFollowupWeeklyChecklist)
}

func TestSuggestedFollowup_AllSlugsAreDistinct(t *testing.T) {
	slugs := []string{
		SeedResearcherFollowupCurrentSources,
		SeedResearcherFollowupCompareRecent,
		SeedResearcherFollowupFactCheckClaim,
		SeedAnalystFollowupExtractPatterns,
		SeedAnalystFollowupCompareOptions,
		SeedAnalystFollowupFindingsIntoActions,
		SeedPlannerFollowupBreakIntoMilestones,
		SeedPlannerFollowupRisksAndOwners,
		SeedPlannerFollowupWeeklyChecklist,
	}
	seen := map[string]bool{}
	for _, slug := range slugs {
		assert.False(t, seen[slug], "duplicate slug %q", slug)
		seen[slug] = true
	}
	assert.Len(t, seen, SeedSuggestedFollowupCount)
}

func TestSuggestedFollowup_AllSlugsAreKebabCase(t *testing.T) {
	slugs := []string{
		SeedResearcherFollowupCurrentSources,
		SeedResearcherFollowupCompareRecent,
		SeedResearcherFollowupFactCheckClaim,
		SeedAnalystFollowupExtractPatterns,
		SeedAnalystFollowupCompareOptions,
		SeedAnalystFollowupFindingsIntoActions,
		SeedPlannerFollowupBreakIntoMilestones,
		SeedPlannerFollowupRisksAndOwners,
		SeedPlannerFollowupWeeklyChecklist,
	}
	for _, slug := range slugs {
		assert.Equal(t, strings.ToLower(slug), slug)
		assert.NotContains(t, slug, "_")
		assert.Contains(t, slug, "-")
	}
}

func TestSuggestedFollowup_ResearcherFactCheckPrompt(t *testing.T) {
	assert.Equal(t, "Fact-check this claim, list supporting and conflicting evidence, and include citations.", SeedResearcherFactCheckPrompt)
}

func TestSuggestedFollowup_AnalystCompareOptionsPrompt(t *testing.T) {
	assert.Equal(t, "Compare these options with trade-offs, confidence levels, and a recommended choice.", SeedAnalystCompareOptionsPrompt)
}

func TestSuggestedFollowup_PlannerMilestonesPrompt(t *testing.T) {
	assert.Equal(t, "Break this goal into sequenced milestones with dependencies and acceptance criteria.", SeedPlannerMilestonesPrompt)
}

func TestSuggestedFollowup_PromptsAreQuestionsOrCommands(t *testing.T) {
	prompts := []string{
		SeedResearcherFactCheckPrompt,
		SeedAnalystCompareOptionsPrompt,
		SeedPlannerMilestonesPrompt,
	}
	for _, prompt := range prompts {
		assert.NotEmpty(t, prompt)
		assert.True(t, strings.HasSuffix(prompt, ".") || strings.HasSuffix(prompt, "?"))
	}
}

func TestSuggestedFollowup_ResearcherPromptMentionsCitations(t *testing.T) {
	assert.Contains(t, strings.ToLower(SeedResearcherFactCheckPrompt), "citations")
}

func TestSuggestedFollowup_AnalystPromptMentionsConfidence(t *testing.T) {
	assert.Contains(t, strings.ToLower(SeedAnalystCompareOptionsPrompt), "confidence")
}

func TestSuggestedFollowup_PlannerPromptMentionsMilestones(t *testing.T) {
	assert.Contains(t, strings.ToLower(SeedPlannerMilestonesPrompt), "milestones")
}

func TestSuggestedFollowup_NewLoaderAcceptsNilPool(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewCoreCapabilitySuggestedFollowupLoader(nil)
	})
}

func TestSuggestedFollowup_RowCountIsOddMultipleOfAgents(t *testing.T) {
	assert.Zero(t, SeedSuggestedFollowupCount%SeedSuggestedFollowupAgentCount)
	assert.Equal(t, 3, SeedSuggestedFollowupCount/SeedSuggestedFollowupAgentCount)
}

func TestSuggestedFollowup_DisplayOrdersArePositive(t *testing.T) {
	assert.Positive(t, SeedSuggestedFollowupMinDisplayOrder)
	assert.Positive(t, SeedSuggestedFollowupMaxDisplayOrder)
}

func TestSuggestedFollowup_CoreAgentsExpectedNames(t *testing.T) {
	agents := []string{"core-researcher", "core-analyst", "core-planner"}
	assert.Len(t, agents, SeedSuggestedFollowupAgentCount)
	for _, agent := range agents {
		assert.True(t, strings.HasPrefix(agent, "core-"))
	}
}

func TestSuggestedFollowup_PerAgentSemanticCoverage(t *testing.T) {
	assert.Contains(t, SeedResearcherFollowupFactCheckClaim, "fact")
	assert.Contains(t, SeedAnalystFollowupCompareOptions, "compare")
	assert.Contains(t, SeedPlannerFollowupBreakIntoMilestones, "milestones")
}
