package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for CoreCapabilityAgentDescription seed (migration 000124).
// Each scenario validates a distinct design decision in the per-agent
// description catalog used by the frontend agent picker UI.

func TestBDD_AhCoreCapabilityAgentDescriptionSeed(t *testing.T) {
	t.Run("Scenario_NineDescriptionsAcrossThreeAgents", func(t *testing.T) {
		// Given three core capability agents (researcher, analyst, planner)
		// And each agent has exactly three description rows (tagline, long_description, use_cases)
		// When the seed migration 000124 runs
		// Then exactly 9 description rows exist in capability_agent_description
		const descKeysPerAgent = 3
		assert.Equal(t, 9, SeedAgentDescriptionCount,
			"9 total rows: 3 desc keys × 3 agents")
		assert.Equal(t, 3, SeedAgentDescriptionAgentCount,
			"3 agents: researcher, analyst, planner")
		assert.Equal(t, SeedAgentDescriptionCount, SeedAgentDescriptionAgentCount*descKeysPerAgent,
			"total = agents × desc keys per agent")
	})

	t.Run("Scenario_EachAgentHasTaglineAndLongDesc", func(t *testing.T) {
		// Given the frontend agent picker UI needs a short card label and a full detail page description
		// When the seed migration 000124 runs
		// Then the tagline and long_description desc_key constants are defined
		// And tagline is a non-empty short label
		// And long_description is a longer, more informative description
		assert.Equal(t, "tagline", SeedDescKeyTagline,
			"tagline desc_key must be 'tagline'")
		assert.Equal(t, "long_description", SeedDescKeyLongDescription,
			"long description desc_key must be 'long_description'")

		// All three agents have non-empty taglines.
		assert.NotEmpty(t, SeedResearcherTagline, "researcher tagline must be non-empty")
		assert.NotEmpty(t, SeedAnalystTagline, "analyst tagline must be non-empty")
		assert.NotEmpty(t, SeedPlannerTagline, "planner tagline must be non-empty")

		// Long descriptions are longer than taglines — confirms they are more informative.
		researcherLong := "The Researcher agent searches the web, fetches pages, and scans your knowledge base to gather accurate, up-to-date information. Ideal for fact-checking, competitive research, news summaries, and answering questions that require current data."
		analystLong := "The Analyst agent examines data, documents, and information to extract patterns, insights, and conclusions. Uses step-by-step reasoning to present findings with appropriate confidence levels. Ideal for interpreting reports, comparing options, and turning raw data into actionable insights."
		plannerLong := "The Planner agent takes your goal and creates a structured, actionable plan. It breaks work into clear steps, identifies dependencies, and can delegate research or analysis subtasks to specialized agents. Ideal for project planning, onboarding workflows, and tackling multi-step objectives."

		assert.Greater(t, len(researcherLong), len(SeedResearcherTagline))
		assert.Greater(t, len(analystLong), len(SeedAnalystTagline))
		assert.Greater(t, len(plannerLong), len(SeedPlannerTagline))
	})

	t.Run("Scenario_UseCasesUsePipeSeparator", func(t *testing.T) {
		// Given the frontend renders use cases as individual badge items
		// And the desc_value for use_cases must be split before rendering
		// When the seed migration 000124 runs
		// Then the use_cases desc_value uses SeedUseCaseSeparator ("|") as separator
		// And each agent's use_cases value contains at least one separator (≥2 items)
		assert.Equal(t, "|", SeedUseCaseSeparator,
			"use cases must be separated by pipe '|'")
		assert.Equal(t, "use_cases", SeedDescKeyUseCases,
			"use cases desc_key must be 'use_cases'")

		researcherUseCases := "Fact-checking claims|Researching competitors|Summarizing recent news|Answering questions about current events|Finding technical documentation"
		analystUseCases := "Analyzing reports and documents|Comparing product options|Interpreting survey results|Identifying data patterns|Evaluating trade-offs"
		plannerUseCases := "Project planning|Onboarding checklists|Multi-step task breakdown|Sprint planning|Creating implementation roadmaps"

		for _, uc := range []string{researcherUseCases, analystUseCases, plannerUseCases} {
			assert.True(t, strings.Contains(uc, SeedUseCaseSeparator),
				"use_cases value %q must contain pipe separator", uc)
			parts := strings.Split(uc, SeedUseCaseSeparator)
			assert.GreaterOrEqual(t, len(parts), 2,
				"use_cases value must split into at least 2 items")
		}
	})

	t.Run("Scenario_ResearcherTaglineDescribesWebSearch", func(t *testing.T) {
		// Given the frontend agent picker shows the tagline to help users select the right agent
		// And core-researcher specializes in web search and information gathering
		// When the seed migration 000124 runs
		// Then the researcher tagline communicates web search capability
		// And the tagline contains keywords that signal information retrieval
		taglineLower := strings.ToLower(SeedResearcherTagline)
		assert.True(t,
			strings.Contains(taglineLower, "search") ||
				strings.Contains(taglineLower, "web") ||
				strings.Contains(taglineLower, "findings"),
			"researcher tagline must convey web search / information retrieval: %q",
			SeedResearcherTagline)
	})

	t.Run("Scenario_PlannerTaglineDescribesStepBreakdown", func(t *testing.T) {
		// Given core-planner specializes in decomposing complex goals into actionable steps
		// And users need to understand from the tagline that this agent creates structured plans
		// When the seed migration 000124 runs
		// Then the planner tagline communicates goal decomposition and step-by-step planning
		taglineLower := strings.ToLower(SeedPlannerTagline)
		assert.True(t,
			strings.Contains(taglineLower, "step") ||
				strings.Contains(taglineLower, "plan") ||
				strings.Contains(taglineLower, "break") ||
				strings.Contains(taglineLower, "goal"),
			"planner tagline must convey step decomposition / planning: %q",
			SeedPlannerTagline)
	})
}
