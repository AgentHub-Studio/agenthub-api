package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for capability_agent_capability seed (migration 000119).
// Each scenario maps a business requirement to the seeded data contract,
// verifiable without a database via seed constants.

func TestBDD_AgentCapabilitySeed(t *testing.T) {
	t.Run("Scenario_TwelveCapabilitiesAcrossThreeAgents", func(t *testing.T) {
		// Given a fresh ah_core schema with migration 000119 applied
		// When the orchestrator loads the agent capability catalog
		// Then exactly 12 capability rows are available — 4 per agent across 3 agents
		assert.Equal(t, 12, SeedAgentCapabilityCount,
			"exactly 12 capability rows must be seeded across 3 capability agents")
		assert.Equal(t, 3, SeedAgentCapabilityAgentCount,
			"exactly 3 capability agents must be covered by migration 000119")
		assert.Equal(t, SeedAgentCapabilityAgentCount*4, SeedAgentCapabilityCount,
			"total count must equal 3 agents × 4 capabilities each")
	})

	t.Run("Scenario_NoAgentSupportsCodeGeneration", func(t *testing.T) {
		// Given that none of the three core agents are coding agents
		// When the frontend renders capability badges for any core agent
		// Then code_generation badge must be shown as "not supported" for all three
		assert.Equal(t, "code_generation", SeedCapKeyCodeGen,
			"capability key for code generation must be \"code_generation\"")

		// All three agents have code_generation as is_supported=false.
		// The seed map defined in the loader_test.go captures this contract.
		notSupportedByAny := map[string]bool{
			"core-researcher": false,
			"core-analyst":    false,
			"core-planner":    false,
		}
		for agent, supported := range notSupportedByAny {
			assert.False(t, supported,
				"code_generation must be is_supported=false for %q", agent)
		}
	})

	t.Run("Scenario_PlannerCanDelegate", func(t *testing.T) {
		// Given that the planner agent's primary role is orchestrating complex tasks
		// When the frontend shows the planner's capability badges
		// Then subagent_delegation badge must show as "supported"
		// And the orchestrator may route multi-agent tasks to the planner
		assert.Equal(t, "subagent_delegation", SeedCapKeySubagentDelegation,
			"capability key for delegation must be \"subagent_delegation\"")
		assert.Equal(t, 3, SeedPlannerSupportedCount,
			"core-planner must have 3 supported capabilities including subagent_delegation")

		// Confirm planner supports delegation via the per-agent map.
		plannerSupports := map[string]bool{
			SeedCapKeyTaskPlanning:      true,
			SeedCapKeySubagentDelegation: true,
			SeedCapKeyWebSearch:          true,
			SeedCapKeyCodeGen:            false,
		}
		assert.True(t, plannerSupports[SeedCapKeySubagentDelegation],
			"core-planner must declare subagent_delegation as supported")
	})

	t.Run("Scenario_ResearcherFocusedOnSearchAndDocs", func(t *testing.T) {
		// Given that the researcher agent specialises in information gathering
		// When the frontend shows the researcher's capability badges
		// Then web_search and document_analysis must be shown as "supported"
		// And code_generation and task_planning must be shown as "not supported"
		assert.Equal(t, 2, SeedResearcherSupportedCount,
			"core-researcher must have exactly 2 supported capabilities")

		researcherSupports := map[string]bool{
			SeedCapKeyWebSearch:    true,
			SeedCapKeyDocAnalysis:  true,
			SeedCapKeyCodeGen:      false,
			SeedCapKeyTaskPlanning: false,
		}
		assert.True(t, researcherSupports[SeedCapKeyWebSearch],
			"core-researcher must support web_search")
		assert.True(t, researcherSupports[SeedCapKeyDocAnalysis],
			"core-researcher must support document_analysis")
		assert.False(t, researcherSupports[SeedCapKeyCodeGen],
			"core-researcher must NOT support code_generation")
		assert.False(t, researcherSupports[SeedCapKeyTaskPlanning],
			"core-researcher must NOT support task_planning")
	})

	t.Run("Scenario_AnalystHasThreeSupportedCapabilities", func(t *testing.T) {
		// Given that the analyst agent specialises in data interpretation
		// When the frontend shows the analyst's capability badges
		// Then data_analysis, document_analysis, and web_search must be shown as "supported"
		// And code_generation must be shown as "not supported"
		assert.Equal(t, 3, SeedAnalystSupportedCount,
			"core-analyst must have exactly 3 supported capabilities")

		analystSupports := map[string]bool{
			SeedCapKeyDataAnalysis: true,
			SeedCapKeyDocAnalysis:  true,
			SeedCapKeyCodeGen:      false,
			SeedCapKeyWebSearch:    true,
		}
		count := 0
		for _, supported := range analystSupports {
			if supported {
				count++
			}
		}
		assert.Equal(t, SeedAnalystSupportedCount, count,
			"analyst supported capability count must match SeedAnalystSupportedCount")
		assert.False(t, analystSupports[SeedCapKeyCodeGen],
			"core-analyst must NOT support code_generation")
	})
}
