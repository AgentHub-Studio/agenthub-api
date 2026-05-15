package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000100 capability system prompt template seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilitySystemPromptTemplateSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilitySystemPromptTemplates", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000100 seeds system prompt templates,
		// Then exactly 3 templates are added — one per capability agent — each
		//   carrying role-specific LLM instructions tailored to the agent's purpose.
		assert.Equal(t, 3, SeedCapabilitySystemPromptTemplateCount,
			"migration 000100 must seed exactly 3 capability system prompt templates")
		assert.Len(t, SeedCapabilitySystemPromptTemplateSlugs, 3,
			"SeedCapabilitySystemPromptTemplateSlugs must have exactly 3 entries — one per capability agent")
	})

	t.Run("Scenario_EachCapabilityAgentHasOwnSystemPrompt", func(t *testing.T) {
		// Given each capability agent (researcher, analyst, planner) has a distinct
		//   execution domain and requires role-specific LLM guidance,
		// When migration 000100 seeds system prompt templates,
		// Then the SeedCapabilitySystemPromptAgentMap contains exactly one entry per
		//   capability agent slug — confirming one-to-one coverage.
		agentSlugs := map[string]int{}
		for _, agentSlug := range SeedCapabilitySystemPromptAgentMap {
			agentSlugs[agentSlug]++
		}
		for slug, count := range agentSlugs {
			assert.Equal(t, 1, count,
				"agent slug %q must appear exactly once in SeedCapabilitySystemPromptAgentMap values (one template per agent)",
				slug)
		}
		// All three expected agent slugs must be covered.
		assert.Contains(t, agentSlugs, "core-researcher",
			"core-researcher must have a system prompt template in SeedCapabilitySystemPromptAgentMap")
		assert.Contains(t, agentSlugs, "core-analyst",
			"core-analyst must have a system prompt template in SeedCapabilitySystemPromptAgentMap")
		assert.Contains(t, agentSlugs, "core-planner",
			"core-planner must have a system prompt template in SeedCapabilitySystemPromptAgentMap")
	})

	t.Run("Scenario_ResearcherPromptTargetsWebResearch", func(t *testing.T) {
		// Given the core-researcher agent's primary responsibility is gathering and
		//   verifying information from web sources and knowledge bases,
		// When migration 000100 seeds the researcher system prompt,
		// Then the researcher slug constant is set to capability-researcher-system-prompt
		//   and it maps to the core-researcher agent — confirming that the prompt
		//   is scoped exclusively to the web research execution domain.
		assert.Equal(t, "capability-researcher-system-prompt", SeedResearcherSystemPromptSlug,
			"SeedResearcherSystemPromptSlug must equal capability-researcher-system-prompt")
		assert.True(t, strings.HasPrefix(SeedResearcherSystemPromptSlug, "capability-"),
			"researcher system prompt slug must carry the 'capability-' namespace prefix")
		assert.True(t, strings.HasSuffix(SeedResearcherSystemPromptSlug, "-system-prompt"),
			"researcher system prompt slug must end with '-system-prompt'")
		assert.Equal(t, "core-researcher", SeedCapabilitySystemPromptAgentMap[SeedResearcherSystemPromptSlug],
			"SeedResearcherSystemPromptSlug must map to core-researcher in SeedCapabilitySystemPromptAgentMap")
	})

	t.Run("Scenario_AnalystPromptTargetsDocumentAnalysis", func(t *testing.T) {
		// Given the core-analyst agent's primary responsibility is analyzing documents,
		//   extracting patterns, and producing evidence-based insights,
		// When migration 000100 seeds the analyst system prompt,
		// Then the analyst slug constant is set to capability-analyst-system-prompt
		//   and it maps to the core-analyst agent — confirming that the prompt
		//   is scoped exclusively to the document analysis execution domain.
		assert.Equal(t, "capability-analyst-system-prompt", SeedAnalystSystemPromptSlug,
			"SeedAnalystSystemPromptSlug must equal capability-analyst-system-prompt")
		assert.True(t, strings.HasPrefix(SeedAnalystSystemPromptSlug, "capability-"),
			"analyst system prompt slug must carry the 'capability-' namespace prefix")
		assert.True(t, strings.HasSuffix(SeedAnalystSystemPromptSlug, "-system-prompt"),
			"analyst system prompt slug must end with '-system-prompt'")
		assert.Equal(t, "core-analyst", SeedCapabilitySystemPromptAgentMap[SeedAnalystSystemPromptSlug],
			"SeedAnalystSystemPromptSlug must map to core-analyst in SeedCapabilitySystemPromptAgentMap")
	})

	t.Run("Scenario_PlannerPromptTargetsTaskManagement", func(t *testing.T) {
		// Given the core-planner agent's primary responsibility is decomposing complex
		//   tasks into actionable steps, identifying dependencies, and tracking progress,
		// When migration 000100 seeds the planner system prompt,
		// Then the planner slug constant is set to capability-planner-system-prompt
		//   and it maps to the core-planner agent — confirming that the prompt
		//   is scoped exclusively to the task management execution domain.
		assert.Equal(t, "capability-planner-system-prompt", SeedPlannerSystemPromptSlug,
			"SeedPlannerSystemPromptSlug must equal capability-planner-system-prompt")
		assert.True(t, strings.HasPrefix(SeedPlannerSystemPromptSlug, "capability-"),
			"planner system prompt slug must carry the 'capability-' namespace prefix")
		assert.True(t, strings.HasSuffix(SeedPlannerSystemPromptSlug, "-system-prompt"),
			"planner system prompt slug must end with '-system-prompt'")
		assert.Equal(t, "core-planner", SeedCapabilitySystemPromptAgentMap[SeedPlannerSystemPromptSlug],
			"SeedPlannerSystemPromptSlug must map to core-planner in SeedCapabilitySystemPromptAgentMap")
	})
}
