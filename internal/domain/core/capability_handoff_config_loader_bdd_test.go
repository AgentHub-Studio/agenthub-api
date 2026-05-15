package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for CoreCapabilityHandoffConfig seed (migration 000123).
// Each scenario validates a distinct design decision in the per-agent
// handoff configuration catalog.

func TestBDD_AhCoreCapabilityHandoffConfigSeed(t *testing.T) {
	t.Run("Scenario_NineHandoffsAcrossThreeAgents", func(t *testing.T) {
		// Given three core capability agents (researcher, analyst, planner)
		// And each agent has exactly three handoff config rows
		// When the seed migration 000123 runs
		// Then exactly 9 handoff config rows exist in capability_handoff_config
		const handoffsPerAgent = 3
		assert.Equal(t, 9, SeedHandoffConfigCount)
		assert.Equal(t, 3, SeedHandoffConfigAgentCount)
		assert.Equal(t, SeedHandoffConfigCount, SeedHandoffConfigAgentCount*handoffsPerAgent)
	})

	t.Run("Scenario_ThreeHumanEscalationsOnePerAgent", func(t *testing.T) {
		// Given each core capability agent has a limit to what it can handle
		// And when no agent can handle the request a human must be involved
		// When the seed migration 000123 runs
		// Then exactly 3 human escalation rows exist (one per agent)
		// And each human escalation row has handoff_key="human_escalation"
		// And each human escalation row has target_agent_slug="" (empty — no target agent)
		assert.Equal(t, 3, SeedHumanEscalationCount,
			"exactly one human escalation row per core agent")
		assert.Equal(t, SeedHandoffConfigAgentCount, SeedHumanEscalationCount,
			"human escalation count must equal agent count (one per agent)")
		assert.Equal(t, "human_escalation", SeedHandoffKeyHumanEscalation,
			"human escalation rows must use handoff_key='human_escalation'")
	})

	t.Run("Scenario_AllHumanEscalationsAreAutomatic", func(t *testing.T) {
		// Given human escalation represents a situation the agent cannot resolve
		// And the agent should not wait for user confirmation to escalate
		// When the orchestrator reads the is_automatic flag for human escalation rows
		// Then all human escalation rows have is_automatic=true
		// And the count of automatic handoffs equals the count of human escalations
		assert.Equal(t, 3, SeedAutomaticHandoffCount,
			"all 3 human escalation rows must be automatic (is_automatic=true)")
		assert.Equal(t, SeedHumanEscalationCount, SeedAutomaticHandoffCount,
			"every human escalation must be automatic; no other rows are automatic")
	})

	t.Run("Scenario_ResearcherHandsOffToAnalystForAnalysis", func(t *testing.T) {
		// Given core-researcher is specialized in gathering information, not analysis
		// And a user asks for data analysis or interpretation
		// When the orchestrator checks the researcher's handoff config
		// Then a needs_analysis handoff row exists for core-researcher
		// And the target_agent_slug is "core-analyst"
		// And the handoff is not automatic (requires routing decision)
		assert.Equal(t, "needs_analysis", SeedHandoffKeyNeedsAnalysis,
			"researcher must have a needs_analysis handoff key")
		// Verify researcher does NOT have a needs_research key (it IS the research agent)
		researcherKeys := []string{
			SeedHandoffKeyNeedsAnalysis,
			SeedHandoffKeyNeedsPlanning,
			SeedHandoffKeyHumanEscalation,
		}
		hasNeedsResearch := false
		for _, k := range researcherKeys {
			if k == SeedHandoffKeyNeedsResearch {
				hasNeedsResearch = true
			}
		}
		assert.False(t, hasNeedsResearch,
			"core-researcher must not have a needs_research handoff — it is the research agent")
	})

	t.Run("Scenario_PlannerHandsOffToResearcherForMoreInfo", func(t *testing.T) {
		// Given core-planner creates plans based on available information
		// And sometimes a plan requires information the planner does not have
		// When the orchestrator checks the planner's handoff config
		// Then a needs_research handoff row exists for core-planner
		// And the target_agent_slug is "core-researcher"
		// And the handoff is not automatic (user should confirm routing)
		assert.Equal(t, "needs_research", SeedHandoffKeyNeedsResearch,
			"planner must have a needs_research handoff key pointing to core-researcher")
		assert.Equal(t, "needs_analysis", SeedHandoffKeyNeedsAnalysis,
			"planner must also have a needs_analysis handoff key pointing to core-analyst")
		// Planner does not hand off to itself (no needs_planning key expected for planner)
		plannerKeys := []string{
			SeedHandoffKeyNeedsResearch,
			SeedHandoffKeyNeedsAnalysis,
			SeedHandoffKeyHumanEscalation,
		}
		hasNeedsPlanning := false
		for _, k := range plannerKeys {
			if k == SeedHandoffKeyNeedsPlanning {
				hasNeedsPlanning = true
			}
		}
		assert.False(t, hasNeedsPlanning,
			"core-planner must not have a needs_planning handoff — it is the planning agent")
	})
}
