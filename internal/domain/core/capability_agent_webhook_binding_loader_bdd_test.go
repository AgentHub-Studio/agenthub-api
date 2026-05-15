package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000099 capability agent–webhook binding seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityAgentWebhookBindingSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityAgentWebhookBindings", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// And the capability webhook template layer (migration 000097) introduces
		//   three notification templates:
		//   capability-research-complete, capability-analysis-done, capability-tasks-updated,
		// When migration 000099 seeds capability agent–webhook bindings,
		// Then exactly 3 bindings are added — one per capability agent — each
		//   connecting the agent to the webhook event it emits when its primary
		//   task completes.
		assert.Equal(t, 3, SeedCapabilityAgentWebhookBindingCount,
			"migration 000099 must seed exactly 3 capability agent–webhook bindings")
		assert.Len(t, SeedCapabilityAgentWebhookBindings, 3,
			"SeedCapabilityAgentWebhookBindings must have exactly 3 entries — one per capability agent")
	})

	t.Run("Scenario_OneBindingPerCapabilityAgent", func(t *testing.T) {
		// Given each capability agent (researcher, analyst, planner) owns a distinct
		//   execution domain and emits a distinct completion event,
		// When migration 000099 seeds bindings,
		// Then each capability agent slug appears exactly once in the binding list —
		//   ensuring a one-to-one mapping between agent and its primary webhook event.
		agentSlugs := map[string]int{}
		for _, b := range SeedCapabilityAgentWebhookBindings {
			agentSlugs[b.AgentSlug]++
		}
		for slug, count := range agentSlugs {
			assert.Equal(t, 1, count,
				"agent slug %q must appear exactly once in SeedCapabilityAgentWebhookBindings (one binding per capability agent)",
				slug)
		}
		// All three expected agent slugs must be present.
		assert.Contains(t, agentSlugs, "core-researcher", "core-researcher must have a binding")
		assert.Contains(t, agentSlugs, "core-analyst", "core-analyst must have a binding")
		assert.Contains(t, agentSlugs, "core-planner", "core-planner must have a binding")
	})

	t.Run("Scenario_ResearcherBoundToResearchCompleteWebhook", func(t *testing.T) {
		// Given the core-researcher agent produces research summaries and
		//   source attributions as its primary output,
		// When migration 000099 seeds its webhook binding,
		// Then the researcher is bound to the capability-research-complete template —
		//   confirming that a "research_complete" event fires upon task completion.
		var researcherBinding *struct{ AgentSlug, WebhookSlug string }
		for _, b := range SeedCapabilityAgentWebhookBindings {
			b := b // capture range variable
			if b.AgentSlug == "core-researcher" {
				researcherBinding = &b
				break
			}
		}
		assert.NotNil(t, researcherBinding,
			"core-researcher must have a webhook binding in SeedCapabilityAgentWebhookBindings")
		if researcherBinding != nil {
			assert.Equal(t, "capability-research-complete", researcherBinding.WebhookSlug,
				"core-researcher must be bound to capability-research-complete (research_complete event)")
			assert.True(t, strings.HasPrefix(researcherBinding.AgentSlug, "core-"),
				"researcher agent slug must carry the 'core-' namespace prefix")
		}
	})

	t.Run("Scenario_AnalystBoundToAnalysisDoneWebhook", func(t *testing.T) {
		// Given the core-analyst agent produces structured document analysis
		//   findings as its primary output,
		// When migration 000099 seeds its webhook binding,
		// Then the analyst is bound to the capability-analysis-done template —
		//   confirming that an "analysis_done" event fires upon task completion.
		var analystBinding *struct{ AgentSlug, WebhookSlug string }
		for _, b := range SeedCapabilityAgentWebhookBindings {
			b := b // capture range variable
			if b.AgentSlug == "core-analyst" {
				analystBinding = &b
				break
			}
		}
		assert.NotNil(t, analystBinding,
			"core-analyst must have a webhook binding in SeedCapabilityAgentWebhookBindings")
		if analystBinding != nil {
			assert.Equal(t, "capability-analysis-done", analystBinding.WebhookSlug,
				"core-analyst must be bound to capability-analysis-done (analysis_done event)")
			assert.True(t, strings.HasPrefix(analystBinding.AgentSlug, "core-"),
				"analyst agent slug must carry the 'core-' namespace prefix")
		}
	})

	t.Run("Scenario_PlannerBoundToTasksUpdatedWebhook", func(t *testing.T) {
		// Given the core-planner agent decomposes goals into structured task lists
		//   and updates them as its primary output,
		// When migration 000099 seeds its webhook binding,
		// Then the planner is bound to the capability-tasks-updated template —
		//   confirming that a "tasks_updated" event fires upon task list update.
		var plannerBinding *struct{ AgentSlug, WebhookSlug string }
		for _, b := range SeedCapabilityAgentWebhookBindings {
			b := b // capture range variable
			if b.AgentSlug == "core-planner" {
				plannerBinding = &b
				break
			}
		}
		assert.NotNil(t, plannerBinding,
			"core-planner must have a webhook binding in SeedCapabilityAgentWebhookBindings")
		if plannerBinding != nil {
			assert.Equal(t, "capability-tasks-updated", plannerBinding.WebhookSlug,
				"core-planner must be bound to capability-tasks-updated (tasks_updated event)")
			assert.True(t, strings.HasPrefix(plannerBinding.AgentSlug, "core-"),
				"planner agent slug must carry the 'core-' namespace prefix")
		}
	})
}
