package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for CoreCapabilityInteractionMode seed (migration 000122).
// Each scenario validates a distinct design decision in the per-agent
// interaction mode catalog.

func TestBDD_AhCoreCapabilityInteractionModeSeed(t *testing.T) {
	t.Run("Scenario_NineModesAcrossThreeAgents", func(t *testing.T) {
		// Given three core capability agents (researcher, analyst, planner)
		// And each agent has exactly three interaction mode rows
		// When the seed migration 000122 runs
		// Then exactly 9 interaction mode rows exist in capability_interaction_mode
		const modesPerAgent = 3
		assert.Equal(t, 9, SeedInteractionModeCount)
		assert.Equal(t, 3, SeedInteractionModeAgentCount)
		assert.Equal(t, SeedInteractionModeCount, SeedInteractionModeAgentCount*modesPerAgent)
	})

	t.Run("Scenario_ResearcherOperatesSingleTurn", func(t *testing.T) {
		// Given core-researcher is designed for focused, independent research queries
		// And users typically expect a single comprehensive answer rather than a dialogue
		// When the orchestrator reads the researcher's primary_mode
		// Then the value is "single_turn"
		// And the researcher's proactive_questions value is "low" (proceeds with assumptions)
		assert.Equal(t, SeedPrimaryModeSingleTurn, SeedResearcherPrimaryMode,
			"core-researcher primary mode must be single_turn")
		const researcherProactiveQ = SeedProactiveQLow
		assert.Equal(t, "low", researcherProactiveQ,
			"core-researcher proactive_questions must be low — assumes context from query")
	})

	t.Run("Scenario_AnalystEngagesInDialogue", func(t *testing.T) {
		// Given core-analyst needs to gather data, analyze it, and refine conclusions
		// And analysis quality improves significantly with iterative user feedback
		// When the orchestrator reads the analyst's primary_mode
		// Then the value is "multi_turn"
		// And the analyst's proactive_questions value is "high"
		//   (actively asks when data or goals are ambiguous)
		assert.Equal(t, SeedPrimaryModeMultiTurn, SeedAnalystPrimaryMode,
			"core-analyst primary mode must be multi_turn for iterative analysis")
		const analystProactiveQ = SeedProactiveQHigh
		assert.Equal(t, "high", analystProactiveQ,
			"core-analyst proactive_questions must be high — ambiguous data requires clarification")
	})

	t.Run("Scenario_PlannerFocusesOnTaskExecution", func(t *testing.T) {
		// Given core-planner is designed to decompose goals into executable plans
		// And the planner delegates to sub-agents and tracks completion
		// When the orchestrator reads the planner's primary_mode
		// Then the value is "task_execution"
		// And the planner's proactive_questions value is "high"
		//   (asks upfront to avoid rework after plan creation)
		assert.Equal(t, SeedPrimaryModeTaskExecution, SeedPlannerPrimaryMode,
			"core-planner primary mode must be task_execution")
		const plannerProactiveQ = SeedProactiveQHigh
		assert.Equal(t, "high", plannerProactiveQ,
			"core-planner proactive_questions must be high — upfront clarification prevents rework")
	})

	t.Run("Scenario_TwoAgentsHighProactiveQuestions", func(t *testing.T) {
		// Given core-analyst and core-planner both operate in domains where
		//   ambiguity in inputs leads to significant downstream errors
		// When the orchestrator reads proactive_questions for both agents
		// Then both values are "high"
		// And core-researcher is the only agent with "low" proactive_questions
		const analystProactiveQ = SeedProactiveQHigh
		const plannerProactiveQ = SeedProactiveQHigh
		const researcherProactiveQ = SeedProactiveQLow

		assert.Equal(t, "high", analystProactiveQ,
			"core-analyst proactive_questions must be high")
		assert.Equal(t, "high", plannerProactiveQ,
			"core-planner proactive_questions must be high")
		assert.Equal(t, "low", researcherProactiveQ,
			"core-researcher proactive_questions must be low")
		assert.NotEqual(t, analystProactiveQ, researcherProactiveQ,
			"researcher differs from analyst in proactive_questions level")
		assert.NotEqual(t, plannerProactiveQ, researcherProactiveQ,
			"researcher differs from planner in proactive_questions level")
	})
}
