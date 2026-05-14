package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000114 capability contextual rule seeds.
// These assert seed shape and contextual rule rationale without a database.

func TestBDD_CapabilityContextualRuleSeed(t *testing.T) {
	t.Run("Scenario_NineRulesAcrossThreeAgents", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent contextual behavior
		//   rules to instruct capability agents based on the current conversational
		//   or task context,
		// When migration 000114 seeds capability_contextual_rule rows,
		// Then exactly 9 rows are added — three per agent — and per-agent counts
		//   sum to the total count constant, covering researcher, analyst, and
		//   planner with balanced rule sets.
		assert.Equal(t, 9, SeedContextualRuleCount,
			"migration 000114 must seed exactly 9 capability contextual rule rows")

		sum := SeedResearcherContextualRuleCount + SeedAnalystContextualRuleCount + SeedPlannerContextualRuleCount
		assert.Equal(t, SeedContextualRuleCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherContextualRuleCount, SeedAnalystContextualRuleCount,
			SeedPlannerContextualRuleCount, SeedContextualRuleCount)

		assert.Equal(t, 3, SeedContextualRuleAgentCount,
			"SeedContextualRuleAgentCount must be 3 — researcher, analyst, planner")
	})

	t.Run("Scenario_ThreeRulesPerAgent", func(t *testing.T) {
		// Given each capability agent has a distinct role (research, analysis,
		//   planning) that requires a focused set of behavioral constraints,
		// When migration 000114 seeds contextual rule rows,
		// Then each agent receives exactly 3 rules — not more, not fewer — so
		//   agents are not over-constrained (which would reduce autonomy) or
		//   under-constrained (which would allow unsafe shortcuts).
		assert.Equal(t, 3, SeedResearcherContextualRuleCount,
			"core-researcher must have exactly 3 contextual rules")
		assert.Equal(t, 3, SeedAnalystContextualRuleCount,
			"core-analyst must have exactly 3 contextual rules")
		assert.Equal(t, 3, SeedPlannerContextualRuleCount,
			"core-planner must have exactly 3 contextual rules")

		// Per-agent counts are equal — balanced rule sets.
		assert.Equal(t, SeedResearcherContextualRuleCount, SeedAnalystContextualRuleCount,
			"researcher and analyst must have equal rule counts (3 each)")
		assert.Equal(t, SeedAnalystContextualRuleCount, SeedPlannerContextualRuleCount,
			"analyst and planner must have equal rule counts (3 each)")
	})

	t.Run("Scenario_AllRulesActiveByDefault", func(t *testing.T) {
		// Given contextual rules should apply immediately upon deployment without
		//   requiring manual activation steps,
		// When migration 000114 seeds capability_contextual_rule rows,
		// Then all 9 rows have is_active = TRUE and the SeedAllContextualRulesActive
		//   constant correctly reflects this default state. Operators may disable
		//   individual rules post-deployment without deleting them.
		assert.True(t, SeedAllContextualRulesActive,
			"SeedAllContextualRulesActive must be true — all seeded contextual rules are active by default")

		// Confirm all trigger context constants are non-empty (rules are
		// well-formed even before hitting the database).
		contexts := []string{
			SeedTriggerContextAlways,
			SeedTriggerContextOnUncertainty,
			SeedTriggerContextOnTaskStart,
			SeedTriggerContextOnAmbiguity,
			SeedTriggerContextOnPlanRequest,
			SeedTriggerContextOnMultipleApproaches,
		}
		for _, tc := range contexts {
			assert.NotEmpty(t, tc, "trigger context constant must not be empty")
		}

		// Priority ordering holds: high > medium > low.
		assert.Greater(t, SeedContextualRulePriorityHigh, SeedContextualRulePriorityMedium,
			"high priority must exceed medium priority")
		assert.Greater(t, SeedContextualRulePriorityMedium, SeedContextualRulePriorityLow,
			"medium priority must exceed low priority")
	})

	t.Run("Scenario_PlannerRequiresClarificationBeforeCode", func(t *testing.T) {
		// Given the core-planner agent is responsible for decomposing high-level
		//   goals into structured task plans, and plans built on ambiguous or
		//   incomplete goals must be entirely redone — making this the highest-cost
		//   planning mistake,
		// When migration 000114 seeds contextual rules for core-planner,
		// Then the highest-priority rule (priority 10) requires asking for
		//   clarification before generating implementation plans, ensuring planners
		//   never start planning without a well-defined goal.
		assert.Equal(t, "ask for clarification before generating implementation plans",
			SeedPlannerAskClarificationRule,
			"SeedPlannerAskClarificationRule must match seeded rule text verbatim")

		// The rule fires on_plan_request — not always — so it only activates
		// when a plan is explicitly requested (avoids over-interruption for
		// exploratory conversations).
		assert.Equal(t, "on_plan_request", SeedTriggerContextOnPlanRequest,
			"on_plan_request trigger context must be correct")

		// High priority (10) — evaluated before break_tasks and present_tradeoffs.
		assert.Equal(t, 10, SeedContextualRulePriorityHigh,
			"SeedContextualRulePriorityHigh must be 10 (planner ask-clarification rule priority)")
	})

	t.Run("Scenario_AnalystBreaksDownBeforeConcluding", func(t *testing.T) {
		// Given the core-analyst agent produces conclusions from structured data
		//   and complex documents, and presenting a conclusion without showing the
		//   reasoning steps makes it impossible for the user to verify correctness,
		// When migration 000114 seeds contextual rules for core-analyst,
		// Then a rule requires breaking complex analysis into labeled steps before
		//   presenting conclusions, fired at task start so the agent commits to a
		//   structured approach before beginning work.
		assert.Equal(t, "break complex analysis into labeled steps before presenting conclusions",
			SeedAnalyzerBreakStepsRule,
			"SeedAnalyzerBreakStepsRule must match seeded rule text verbatim")

		// The rule fires on_task_start so the analyst structures work upfront.
		assert.Equal(t, "on_task_start", SeedTriggerContextOnTaskStart,
			"on_task_start trigger context must be correct")

		// Also verify the highest-priority analyst rule is validate-assumptions
		// (priority 10) — invalid assumptions cascade into downstream errors.
		assert.Equal(t, 10, SeedContextualRulePriorityHigh,
			"validate_assumptions rule has priority 10 — highest for analyst")

		// The break-steps rule is medium priority (5) — important but below
		// validate_assumptions because you can still recover from an un-labeled
		// analysis more easily than from an analysis built on a false assumption.
		assert.Equal(t, 5, SeedContextualRulePriorityMedium,
			"SeedContextualRulePriorityMedium must be 5 (break-steps rule priority)")
	})
}
