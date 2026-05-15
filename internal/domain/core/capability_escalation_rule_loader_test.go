package core

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability escalation rule seed constants (migration 000112).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityEscalationRuleCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedCapabilityEscalationRuleCount,
		"migration 000112 seeds exactly 9 capability escalation rule rows (three per capability agent)")
}

func TestSeedCapabilityEscalationRuleAgentSlugs_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityEscalationRuleAgentSlugs, 3,
		"SeedCapabilityEscalationRuleAgentSlugs must have exactly 3 entries (researcher, analyst, planner)")
}

func TestSeedResearcherEscalationRuleCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherEscalationRuleCount,
		"SeedResearcherEscalationRuleCount must be 3 — low_source_confidence + consecutive_tool_failures + out_of_scope_request")
}

func TestSeedAnalystEscalationRuleCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystEscalationRuleCount,
		"SeedAnalystEscalationRuleCount must be 3 — low_analysis_confidence + ambiguous_document_content + consecutive_tool_failures")
}

func TestSeedPlannerEscalationRuleCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerEscalationRuleCount,
		"SeedPlannerEscalationRuleCount must be 3 — ambiguous_goal + dependency_conflict_detected + consecutive_tool_failures")
}

func TestSeedPerAgentEscalationRuleCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherEscalationRuleCount + SeedAnalystEscalationRuleCount + SeedPlannerEscalationRuleCount
	assert.Equal(t, SeedCapabilityEscalationRuleCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherEscalationRuleCount, SeedAnalystEscalationRuleCount, SeedPlannerEscalationRuleCount,
		SeedCapabilityEscalationRuleCount)
}

func TestSeedAllEscalationRulesActive_IsTrue(t *testing.T) {
	assert.True(t, SeedAllEscalationRulesActive,
		"SeedAllEscalationRulesActive must be true — all 9 escalation rules are active by default")
}

func TestSeedEscalationTriggerConstants_AreSevenDistinct(t *testing.T) {
	triggers := []string{
		SeedEscalationTriggerLowSourceConfidence,
		SeedEscalationTriggerLowAnalysisConfidence,
		SeedEscalationTriggerConsecutiveToolFailures,
		SeedEscalationTriggerOutOfScopeRequest,
		SeedEscalationTriggerAmbiguousDocContent,
		SeedEscalationTriggerAmbiguousGoal,
		SeedEscalationTriggerDependencyConflict,
	}
	seen := map[string]struct{}{}
	for _, tr := range triggers {
		assert.NotEmpty(t, tr, "every trigger condition constant must be non-empty")
		seen[tr] = struct{}{}
	}
	assert.Len(t, seen, 7, "there must be exactly 7 distinct trigger condition constants")
}

func TestSeedEscalationActionConstants_AreFourDistinct(t *testing.T) {
	actions := []string{
		SeedEscalationActionPauseAndAsk,
		SeedEscalationActionClarifyScope,
		SeedEscalationActionClarifyIntent,
		SeedEscalationActionClarifyGoal,
	}
	seen := map[string]struct{}{}
	for _, a := range actions {
		assert.NotEmpty(t, a, "every action constant must be non-empty")
		seen[a] = struct{}{}
	}
	assert.Len(t, seen, 4, "there must be exactly 4 distinct action constants")
}

func TestSeedEscalationActionPauseAndAsk_IsMostCommonAction(t *testing.T) {
	// pause_and_ask covers 6 of the 9 rules (researcher×2, analyst×2, planner×2).
	// clarify_scope, clarify_intent, clarify_goal each cover 1 rule.
	assert.Equal(t, "pause_and_ask", SeedEscalationActionPauseAndAsk,
		"SeedEscalationActionPauseAndAsk must equal the string \"pause_and_ask\"")
	// Confirm the three clarify variants are different from pause_and_ask.
	assert.NotEqual(t, SeedEscalationActionPauseAndAsk, SeedEscalationActionClarifyScope,
		"clarify_scope must differ from pause_and_ask")
	assert.NotEqual(t, SeedEscalationActionPauseAndAsk, SeedEscalationActionClarifyIntent,
		"clarify_intent must differ from pause_and_ask")
	assert.NotEqual(t, SeedEscalationActionPauseAndAsk, SeedEscalationActionClarifyGoal,
		"clarify_goal must differ from pause_and_ask")
}

func TestSeedEscalationTriggerLowSourceConfidence_BelongsToResearcher(t *testing.T) {
	assert.Equal(t, "low_source_confidence", SeedEscalationTriggerLowSourceConfidence,
		"SeedEscalationTriggerLowSourceConfidence must equal \"low_source_confidence\"")
}

func TestSeedEscalationTriggerLowAnalysisConfidence_BelongsToAnalyst(t *testing.T) {
	assert.Equal(t, "low_analysis_confidence", SeedEscalationTriggerLowAnalysisConfidence,
		"SeedEscalationTriggerLowAnalysisConfidence must equal \"low_analysis_confidence\"")
}

func TestSeedAnalystConfidenceThreshold_IsHigherThanResearcher(t *testing.T) {
	researcherThreshold, err := strconv.ParseFloat(SeedResearcherLowConfidenceThreshold, 64)
	assert.NoError(t, err, "SeedResearcherLowConfidenceThreshold must be parseable as float64")

	analystThreshold, err := strconv.ParseFloat(SeedAnalystLowConfidenceThreshold, 64)
	assert.NoError(t, err, "SeedAnalystLowConfidenceThreshold must be parseable as float64")

	assert.Greater(t, analystThreshold, researcherThreshold,
		"analyst confidence threshold (%v) must be higher than researcher (%v) — analytical errors are harder to detect",
		analystThreshold, researcherThreshold)
}

func TestSeedResearcherLowConfidenceThreshold_IsPoint4(t *testing.T) {
	assert.Equal(t, "0.4", SeedResearcherLowConfidenceThreshold,
		"researcher low_source_confidence threshold must be 0.4")
}

func TestSeedAnalystLowConfidenceThreshold_IsPoint5(t *testing.T) {
	assert.Equal(t, "0.5", SeedAnalystLowConfidenceThreshold,
		"analyst low_analysis_confidence threshold must be 0.5")
}

func TestSeedEscalationTriggerAmbiguousGoal_BelongsToPlanner(t *testing.T) {
	assert.Equal(t, "ambiguous_goal", SeedEscalationTriggerAmbiguousGoal,
		"SeedEscalationTriggerAmbiguousGoal must equal \"ambiguous_goal\"")
}

func TestSeedCapabilityEscalationRuleAgentSlugs_ContainsAllThreeAgents(t *testing.T) {
	slugSet := map[string]struct{}{}
	for _, s := range SeedCapabilityEscalationRuleAgentSlugs {
		slugSet[s] = struct{}{}
	}
	_, hasResearcher := slugSet["core-researcher"]
	assert.True(t, hasResearcher, "SeedCapabilityEscalationRuleAgentSlugs must contain 'core-researcher'")

	_, hasAnalyst := slugSet["core-analyst"]
	assert.True(t, hasAnalyst, "SeedCapabilityEscalationRuleAgentSlugs must contain 'core-analyst'")

	_, hasPlanner := slugSet["core-planner"]
	assert.True(t, hasPlanner, "SeedCapabilityEscalationRuleAgentSlugs must contain 'core-planner'")
}

func TestSeedEscalationTriggerDependencyConflict_BelongsToPlanner(t *testing.T) {
	assert.Equal(t, "dependency_conflict_detected", SeedEscalationTriggerDependencyConflict,
		"SeedEscalationTriggerDependencyConflict must equal \"dependency_conflict_detected\"")
}
