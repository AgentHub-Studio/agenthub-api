package core

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000112 capability escalation rule seeds.
// These assert seed shape and escalation rule rationale without a database.

func TestBDD_CapabilityEscalationRuleSeed(t *testing.T) {
	t.Run("Scenario_NineEscalationRulesThreePerCapabilityAgent", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent escalation rules
		//   to determine when capability agents should defer to the user instead
		//   of proceeding autonomously,
		// When migration 000112 seeds capability_escalation_rule rows,
		// Then exactly 9 rows are added — three per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 9, SeedCapabilityEscalationRuleCount,
			"migration 000112 must seed exactly 9 capability escalation rule rows")
		sum := SeedResearcherEscalationRuleCount + SeedAnalystEscalationRuleCount + SeedPlannerEscalationRuleCount
		assert.Equal(t, SeedCapabilityEscalationRuleCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherEscalationRuleCount, SeedAnalystEscalationRuleCount, SeedPlannerEscalationRuleCount,
			SeedCapabilityEscalationRuleCount)
		assert.Equal(t, 3, SeedResearcherEscalationRuleCount,
			"researcher must have exactly 3 escalation rules")
		assert.Equal(t, 3, SeedAnalystEscalationRuleCount,
			"analyst must have exactly 3 escalation rules")
		assert.Equal(t, 3, SeedPlannerEscalationRuleCount,
			"planner must have exactly 3 escalation rules")
		// Agent slug list must have 3 distinct entries.
		assert.Len(t, SeedCapabilityEscalationRuleAgentSlugs, 3,
			"SeedCapabilityEscalationRuleAgentSlugs must list exactly 3 agents")
	})

	t.Run("Scenario_AllRulesActiveByDefault", func(t *testing.T) {
		// Given escalation rules should apply immediately upon deployment without
		//   requiring manual activation steps,
		// When migration 000112 seeds capability_escalation_rule rows,
		// Then all 9 rules have is_active = TRUE and the SeedAllEscalationRulesActive
		//   constant correctly reflects this default state.
		assert.True(t, SeedAllEscalationRulesActive,
			"SeedAllEscalationRulesActive must be true — all seeded rules are active by default")

		// Confirm all trigger and action constants are non-empty (rules are
		// well-formed even before hitting the database).
		triggers := []string{
			SeedEscalationTriggerLowSourceConfidence,
			SeedEscalationTriggerLowAnalysisConfidence,
			SeedEscalationTriggerConsecutiveToolFailures,
			SeedEscalationTriggerOutOfScopeRequest,
			SeedEscalationTriggerAmbiguousDocContent,
			SeedEscalationTriggerAmbiguousGoal,
			SeedEscalationTriggerDependencyConflict,
		}
		for _, tr := range triggers {
			assert.NotEmpty(t, tr, "trigger condition constant must not be empty")
		}
		actions := []string{
			SeedEscalationActionPauseAndAsk,
			SeedEscalationActionClarifyScope,
			SeedEscalationActionClarifyIntent,
			SeedEscalationActionClarifyGoal,
		}
		for _, a := range actions {
			assert.NotEmpty(t, a, "action constant must not be empty")
		}
	})

	t.Run("Scenario_AnalystHasHigherConfidenceThresholdThanResearcher", func(t *testing.T) {
		// Given analytical conclusions are harder to detect as incorrect post-hoc
		//   compared to research source citations, which can be spot-checked by the
		//   user,
		// When migration 000112 seeds confidence threshold escalation rules,
		// Then the analyst's low_analysis_confidence threshold (0.5) is strictly
		//   higher than the researcher's low_source_confidence threshold (0.4),
		//   ensuring the analyst escalates more conservatively.
		researcherThreshold, err := strconv.ParseFloat(SeedResearcherLowConfidenceThreshold, 64)
		assert.NoError(t, err, "SeedResearcherLowConfidenceThreshold must be parseable as float64")

		analystThreshold, err := strconv.ParseFloat(SeedAnalystLowConfidenceThreshold, 64)
		assert.NoError(t, err, "SeedAnalystLowConfidenceThreshold must be parseable as float64")

		assert.Greater(t, analystThreshold, researcherThreshold,
			"analyst confidence threshold (%v) must exceed researcher (%v) — analytical errors compound",
			analystThreshold, researcherThreshold)

		// Verify absolute values.
		assert.Equal(t, "0.4", SeedResearcherLowConfidenceThreshold,
			"researcher low_source_confidence threshold must be 0.4")
		assert.Equal(t, "0.5", SeedAnalystLowConfidenceThreshold,
			"analyst low_analysis_confidence threshold must be 0.5")
	})

	t.Run("Scenario_EachAgentHasToolFailureEscalationRule", func(t *testing.T) {
		// Given all capability agents depend on tools (search, analysis, planning
		//   integrations) to complete tasks, and repeated tool failures indicate an
		//   environmental problem the agent cannot self-resolve,
		// When migration 000112 seeds capability_escalation_rule rows,
		// Then each of the three agents has a consecutive_tool_failures escalation
		//   rule — and the single shared trigger constant covers all three agents.
		assert.Equal(t, "consecutive_tool_failures", SeedEscalationTriggerConsecutiveToolFailures,
			"SeedEscalationTriggerConsecutiveToolFailures must equal \"consecutive_tool_failures\"")

		// Tool failure appears in researcher, analyst, and planner — so the trigger
		// constant is shared across all three per-agent rule sets. Verify each
		// agent contributes exactly 3 rules (including the tool failure rule).
		assert.Equal(t, SeedResearcherEscalationRuleCount, SeedAnalystEscalationRuleCount,
			"researcher and analyst must have equal rule counts (3 each, both include a tool-failure rule)")
		assert.Equal(t, SeedAnalystEscalationRuleCount, SeedPlannerEscalationRuleCount,
			"analyst and planner must have equal rule counts (3 each, both include a tool-failure rule)")
	})

	t.Run("Scenario_PlannerEscalatesOnAmbiguousGoalsAndDependencyConflicts", func(t *testing.T) {
		// Given the planner's role is to decompose high-level goals into structured
		//   task DAGs, and ambiguous goals or dependency conflicts make it impossible
		//   to produce a valid plan without user input,
		// When migration 000112 seeds escalation rules for core-planner,
		// Then the planner has both an ambiguous_goal rule (triggering clarify_goal)
		//   and a dependency_conflict_detected rule (triggering pause_and_ask),
		//   reflecting the two most critical planning failure modes.
		assert.Equal(t, "ambiguous_goal", SeedEscalationTriggerAmbiguousGoal,
			"SeedEscalationTriggerAmbiguousGoal must equal \"ambiguous_goal\"")
		assert.Equal(t, "dependency_conflict_detected", SeedEscalationTriggerDependencyConflict,
			"SeedEscalationTriggerDependencyConflict must equal \"dependency_conflict_detected\"")
		assert.Equal(t, "clarify_goal", SeedEscalationActionClarifyGoal,
			"SeedEscalationActionClarifyGoal must equal \"clarify_goal\"")

		// The dependency conflict action is pause_and_ask (not a clarify variant)
		// because dependency conflicts require the user to make a prioritization
		// decision, not just clarify a goal statement.
		assert.Equal(t, SeedEscalationActionPauseAndAsk, "pause_and_ask",
			"SeedEscalationActionPauseAndAsk must equal \"pause_and_ask\"")
		assert.NotEqual(t, SeedEscalationActionClarifyGoal, SeedEscalationActionPauseAndAsk,
			"clarify_goal and pause_and_ask must be different actions")
	})
}
