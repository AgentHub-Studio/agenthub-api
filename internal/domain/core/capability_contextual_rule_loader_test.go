package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability contextual rule seed constants (migration 000114).
// These run without a database and guard against accidental constant drift.

func TestSeedContextualRuleCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedContextualRuleCount,
		"migration 000114 seeds exactly 9 capability contextual rule rows (three per capability agent)")
}

func TestSeedContextualRuleAgentCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedContextualRuleAgentCount,
		"SeedContextualRuleAgentCount must be 3 — researcher, analyst, planner")
}

func TestSeedResearcherContextualRuleCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherContextualRuleCount,
		"SeedResearcherContextualRuleCount must be 3 — cite_sources + prefer_search + summarize_findings")
}

func TestSeedAnalystContextualRuleCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystContextualRuleCount,
		"SeedAnalystContextualRuleCount must be 3 — break_steps + show_uncertainty + validate_assumptions")
}

func TestSeedPlannerContextualRuleCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerContextualRuleCount,
		"SeedPlannerContextualRuleCount must be 3 — ask_clarification + break_tasks + present_tradeoffs")
}

func TestSeedPerAgentContextualRuleCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherContextualRuleCount + SeedAnalystContextualRuleCount + SeedPlannerContextualRuleCount
	assert.Equal(t, SeedContextualRuleCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherContextualRuleCount, SeedAnalystContextualRuleCount, SeedPlannerContextualRuleCount,
		SeedContextualRuleCount)
}

func TestSeedAllContextualRulesActive_IsTrue(t *testing.T) {
	assert.True(t, SeedAllContextualRulesActive,
		"SeedAllContextualRulesActive must be true — all 9 contextual rules are active by default")
}

func TestSeedContextualRuleCount_EqualsAgentCountTimesThree(t *testing.T) {
	assert.Equal(t, SeedContextualRuleAgentCount*3, SeedContextualRuleCount,
		"total rule count must equal agent count × 3 (each agent has exactly 3 rules)")
}

func TestSeedResearcherCiteSourcesRule_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedResearcherCiteSourcesRule,
		"SeedResearcherCiteSourcesRule must be a non-empty string")
	assert.Equal(t, "always cite sources when making factual claims", SeedResearcherCiteSourcesRule,
		"SeedResearcherCiteSourcesRule must match the seeded rule text verbatim")
}

func TestSeedAnalyzerBreakStepsRule_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedAnalyzerBreakStepsRule,
		"SeedAnalyzerBreakStepsRule must be a non-empty string")
	assert.Equal(t, "break complex analysis into labeled steps before presenting conclusions", SeedAnalyzerBreakStepsRule,
		"SeedAnalyzerBreakStepsRule must match the seeded rule text verbatim")
}

func TestSeedPlannerAskClarificationRule_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedPlannerAskClarificationRule,
		"SeedPlannerAskClarificationRule must be a non-empty string")
	assert.Equal(t, "ask for clarification before generating implementation plans", SeedPlannerAskClarificationRule,
		"SeedPlannerAskClarificationRule must match the seeded rule text verbatim")
}

func TestSeedTriggerContextAlways_Value(t *testing.T) {
	assert.Equal(t, "always", SeedTriggerContextAlways,
		"SeedTriggerContextAlways must equal \"always\"")
}

func TestSeedTriggerContextOnUncertainty_Value(t *testing.T) {
	assert.Equal(t, "on_uncertainty", SeedTriggerContextOnUncertainty,
		"SeedTriggerContextOnUncertainty must equal \"on_uncertainty\"")
}

func TestSeedTriggerContextOnTaskStart_Value(t *testing.T) {
	assert.Equal(t, "on_task_start", SeedTriggerContextOnTaskStart,
		"SeedTriggerContextOnTaskStart must equal \"on_task_start\"")
}

func TestSeedTriggerContextOnAmbiguity_Value(t *testing.T) {
	assert.Equal(t, "on_ambiguity", SeedTriggerContextOnAmbiguity,
		"SeedTriggerContextOnAmbiguity must equal \"on_ambiguity\"")
}

func TestSeedTriggerContextOnPlanRequest_Value(t *testing.T) {
	assert.Equal(t, "on_plan_request", SeedTriggerContextOnPlanRequest,
		"SeedTriggerContextOnPlanRequest must equal \"on_plan_request\"")
}

func TestSeedTriggerContextOnMultipleApproaches_Value(t *testing.T) {
	assert.Equal(t, "on_multiple_approaches", SeedTriggerContextOnMultipleApproaches,
		"SeedTriggerContextOnMultipleApproaches must equal \"on_multiple_approaches\"")
}

func TestSeedContextualRulePriorityHigh_IsTen(t *testing.T) {
	assert.Equal(t, 10, SeedContextualRulePriorityHigh,
		"SeedContextualRulePriorityHigh must be 10")
}

func TestSeedContextualRulePriorityMedium_IsFive(t *testing.T) {
	assert.Equal(t, 5, SeedContextualRulePriorityMedium,
		"SeedContextualRulePriorityMedium must be 5")
}

func TestSeedContextualRulePriorityLow_IsZero(t *testing.T) {
	assert.Equal(t, 0, SeedContextualRulePriorityLow,
		"SeedContextualRulePriorityLow must be 0 (default)")
}

func TestSeedContextualRulePriority_HighGreaterThanMediumGreaterThanLow(t *testing.T) {
	assert.Greater(t, SeedContextualRulePriorityHigh, SeedContextualRulePriorityMedium,
		"high priority must be greater than medium priority")
	assert.Greater(t, SeedContextualRulePriorityMedium, SeedContextualRulePriorityLow,
		"medium priority must be greater than low priority")
}

func TestSeedTriggerContexts_AreDistinct(t *testing.T) {
	contexts := []string{
		SeedTriggerContextAlways,
		SeedTriggerContextOnUncertainty,
		SeedTriggerContextOnTaskStart,
		SeedTriggerContextOnAmbiguity,
		SeedTriggerContextOnPlanRequest,
		SeedTriggerContextOnMultipleApproaches,
	}
	seen := map[string]struct{}{}
	for _, tc := range contexts {
		assert.NotEmpty(t, tc, "every trigger context constant must be non-empty")
		seen[tc] = struct{}{}
	}
	assert.Len(t, seen, 6,
		"there must be exactly 6 distinct trigger context constants (always, on_uncertainty, on_task_start, on_ambiguity, on_plan_request, on_multiple_approaches)")
}
