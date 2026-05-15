package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for CoreCapabilitySuggestedFollowup seed (migration 000125).
func TestBDD_AhCoreCapabilitySuggestedFollowupSeed(t *testing.T) {
	t.Run("Scenario_NineFollowupsAcrossThreeAgents", func(t *testing.T) {
		assert.Equal(t, 9, SeedSuggestedFollowupCount)
		assert.Equal(t, 3, SeedSuggestedFollowupAgentCount)
		assert.Equal(t, 3, SeedSuggestedFollowupPerAgentCount)
		assert.Equal(t, SeedSuggestedFollowupCount, SeedSuggestedFollowupAgentCount*SeedSuggestedFollowupPerAgentCount)
	})

	t.Run("Scenario_ResearcherFollowupsGuideSourceWork", func(t *testing.T) {
		assert.Equal(t, "research-current-sources", SeedResearcherFollowupCurrentSources)
		assert.Equal(t, "compare-official-and-recent", SeedResearcherFollowupCompareRecent)
		assert.Equal(t, "fact-check-claim", SeedResearcherFollowupFactCheckClaim)
		assert.Contains(t, strings.ToLower(SeedResearcherFactCheckPrompt), "fact-check")
		assert.Contains(t, strings.ToLower(SeedResearcherFactCheckPrompt), "citations")
	})

	t.Run("Scenario_AnalystFollowupsGuideComparisonWork", func(t *testing.T) {
		assert.Equal(t, "extract-key-patterns", SeedAnalystFollowupExtractPatterns)
		assert.Equal(t, "compare-options", SeedAnalystFollowupCompareOptions)
		assert.Equal(t, "turn-findings-into-actions", SeedAnalystFollowupFindingsIntoActions)
		assert.Contains(t, strings.ToLower(SeedAnalystCompareOptionsPrompt), "trade-offs")
		assert.Contains(t, strings.ToLower(SeedAnalystCompareOptionsPrompt), "recommended")
	})

	t.Run("Scenario_PlannerFollowupsGuideExecutionPlanning", func(t *testing.T) {
		assert.Equal(t, "break-into-milestones", SeedPlannerFollowupBreakIntoMilestones)
		assert.Equal(t, "identify-risks-and-owners", SeedPlannerFollowupRisksAndOwners)
		assert.Equal(t, "weekly-checklist", SeedPlannerFollowupWeeklyChecklist)
		assert.Contains(t, strings.ToLower(SeedPlannerMilestonesPrompt), "dependencies")
		assert.Contains(t, strings.ToLower(SeedPlannerMilestonesPrompt), "acceptance criteria")
	})

	t.Run("Scenario_DisplayOrdersCoverThreeCardsPerAgent", func(t *testing.T) {
		assert.Equal(t, 1, SeedSuggestedFollowupMinDisplayOrder)
		assert.Equal(t, 3, SeedSuggestedFollowupMaxDisplayOrder)
		assert.Equal(t,
			SeedSuggestedFollowupPerAgentCount,
			SeedSuggestedFollowupMaxDisplayOrder-SeedSuggestedFollowupMinDisplayOrder+1,
		)
	})
}
