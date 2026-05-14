package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000097 capability webhook notification template seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityWebhookTemplateSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityWebhookTemplates", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000097 seeds capability webhook notification templates,
		// Then exactly 3 templates are added — one per capability agent — covering
		//   research completion, analysis completion, and task list updates.
		assert.Equal(t, 3, SeedCapabilityWebhookTemplateCount,
			"migration 000097 must seed exactly 3 capability webhook notification templates")
		assert.Equal(t, 3, len(SeedCapabilityWebhookTemplateSlugs),
			"slug list must have exactly 3 entries — one per capability agent")
		assert.Equal(t, 3, len(SeedCapabilityWebhookTemplateEvents),
			"event type list must have exactly 3 entries — one per capability agent")
	})

	t.Run("Scenario_EachWebhookHasDistinctEventType", func(t *testing.T) {
		// Given webhook consumers subscribe by event_type to receive only the
		//   notifications they care about,
		// When migration 000097 seeds capability webhook templates,
		// Then all 3 event types are distinct — research_complete, analysis_done,
		//   tasks_updated — so consumers can subscribe independently.
		eventSet := map[string]bool{}
		for _, ev := range SeedCapabilityWebhookTemplateEvents {
			assert.False(t, eventSet[ev],
				"event type %q must be unique in capability webhook event list", ev)
			eventSet[ev] = true
		}
		assert.True(t, eventSet["research_complete"],
			"research_complete event type must be present for core-researcher notifications")
		assert.True(t, eventSet["analysis_done"],
			"analysis_done event type must be present for core-analyst notifications")
		assert.True(t, eventSet["tasks_updated"],
			"tasks_updated event type must be present for core-planner notifications")
	})

	t.Run("Scenario_ResearchCompleteWebhookForResearcherAgent", func(t *testing.T) {
		// Given the core-researcher agent completes research tasks that produce
		//   findings and populate knowledge bases,
		// When migration 000097 seeds the research completion webhook template,
		// Then the template slug is 'capability-research-complete', the event type
		//   is 'research_complete', and the constant SeedResearchCompleteWebhookSlug
		//   references it — forming a stable Go-to-SQL contract.
		assert.Equal(t, "capability-research-complete", SeedResearchCompleteWebhookSlug,
			"SeedResearchCompleteWebhookSlug must be 'capability-research-complete'")
		assert.True(t, strings.HasPrefix(SeedResearchCompleteWebhookSlug, "capability-"),
			"researcher webhook slug must start with 'capability-' (namespace contract)")

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityWebhookTemplateSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet[SeedResearchCompleteWebhookSlug],
			"SeedResearchCompleteWebhookSlug must appear in SeedCapabilityWebhookTemplateSlugs")
	})

	t.Run("Scenario_AnalysisDoneWebhookForAnalystAgent", func(t *testing.T) {
		// Given the core-analyst agent processes documents and produces analysis
		//   briefs with confidence scores,
		// When migration 000097 seeds the analysis completion webhook template,
		// Then the template slug is 'capability-analysis-done', the event type
		//   is 'analysis_done', and the constant SeedAnalysisDoneWebhookSlug
		//   references it — forming a stable Go-to-SQL contract.
		assert.Equal(t, "capability-analysis-done", SeedAnalysisDoneWebhookSlug,
			"SeedAnalysisDoneWebhookSlug must be 'capability-analysis-done'")
		assert.True(t, strings.HasPrefix(SeedAnalysisDoneWebhookSlug, "capability-"),
			"analyst webhook slug must start with 'capability-' (namespace contract)")

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityWebhookTemplateSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet[SeedAnalysisDoneWebhookSlug],
			"SeedAnalysisDoneWebhookSlug must appear in SeedCapabilityWebhookTemplateSlugs")
	})

	t.Run("Scenario_TasksUpdatedWebhookForPlannerAgent", func(t *testing.T) {
		// Given the core-planner agent manages task lists by adding, completing,
		//   and tracking pending work items,
		// When migration 000097 seeds the task update webhook template,
		// Then the template slug is 'capability-tasks-updated', the event type
		//   is 'tasks_updated', and the constant SeedTasksUpdatedWebhookSlug
		//   references it — forming a stable Go-to-SQL contract.
		assert.Equal(t, "capability-tasks-updated", SeedTasksUpdatedWebhookSlug,
			"SeedTasksUpdatedWebhookSlug must be 'capability-tasks-updated'")
		assert.True(t, strings.HasPrefix(SeedTasksUpdatedWebhookSlug, "capability-"),
			"planner webhook slug must start with 'capability-' (namespace contract)")

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityWebhookTemplateSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet[SeedTasksUpdatedWebhookSlug],
			"SeedTasksUpdatedWebhookSlug must appear in SeedCapabilityWebhookTemplateSlugs")
	})
}
