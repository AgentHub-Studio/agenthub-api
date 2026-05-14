package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000107 capability UI hint seeds.
// These assert seed shape and hint rationale without a database.

func TestBDD_CapabilityUIHintSeed(t *testing.T) {
	t.Run("Scenario_SixCapabilityUIHintsTwoPerAgent", func(t *testing.T) {
		// Given the AgentHub web UI needs contextual guidance for three core
		//   capability agents (researcher, analyst, planner),
		// When migration 000107 seeds capability_ui_hint rows,
		// Then exactly 6 rows are added — two per agent — and per-agent counts
		//   sum to the total count constant.
		assert.Equal(t, 6, SeedCapabilityUIHintCount,
			"migration 000107 must seed exactly 6 capability UI hint rows")
		assert.Len(t, SeedCapabilityUIHintSlugs, 6,
			"SeedCapabilityUIHintSlugs must list exactly 6 slugs")
		sum := SeedResearcherUIHintCount + SeedAnalystUIHintCount + SeedPlannerUIHintCount
		assert.Equal(t, SeedCapabilityUIHintCount, sum,
			"researcher(%d)+analyst(%d)+planner(%d) must equal total count(%d)",
			SeedResearcherUIHintCount, SeedAnalystUIHintCount, SeedPlannerUIHintCount,
			SeedCapabilityUIHintCount)
	})

	t.Run("Scenario_FourTipsAndTwoInfoHints", func(t *testing.T) {
		// Given users benefit from a mix of actionable tips (how to phrase a
		//   message, how to phrase a follow-up) and informational hints (what
		//   the agent does automatically),
		// When migration 000107 seeds hint_type values,
		// Then exactly 4 hints have type 'tip' and exactly 2 have type 'info',
		//   and these counts sum to the total constant.
		assert.Equal(t, 4, SeedUIHintTipCount,
			"SeedUIHintTipCount must be 4 (researcher-start, analyst-upload, analyst-confidence, planner-breakdown)")
		assert.Equal(t, 2, SeedUIHintInfoCount,
			"SeedUIHintInfoCount must be 2 (researcher-citation, planner-task)")
		sum := SeedUIHintTipCount + SeedUIHintInfoCount
		assert.Equal(t, SeedCapabilityUIHintCount, sum,
			"tip(%d) + info(%d) must equal total hint count(%d)",
			SeedUIHintTipCount, SeedUIHintInfoCount, SeedCapabilityUIHintCount)
		assert.NotEqual(t, SeedUIHintTypeTip, SeedUIHintTypeInfo,
			"SeedUIHintTypeTip and SeedUIHintTypeInfo must be distinct type strings")
	})

	t.Run("Scenario_ResearcherHintsCoverStartAndCitation", func(t *testing.T) {
		// Given a Researcher agent's most common new-user friction points are:
		//   (a) writing vague queries instead of specific questions, and
		//   (b) not knowing that citations are included automatically,
		// When migration 000107 seeds researcher hints,
		// Then 2 researcher hints exist: one 'tip' for agent_chat_start and one
		//   'info' for post_tool_result, covering both friction points.
		assert.Equal(t, 2, SeedResearcherUIHintCount,
			"SeedResearcherUIHintCount must be 2 for core-researcher")
		// Both slugs must be present in the full slug list.
		slugSet := map[string]struct{}{}
		for _, s := range SeedCapabilityUIHintSlugs {
			slugSet[s] = struct{}{}
		}
		_, hasStart := slugSet["hint-researcher-start-tip"]
		assert.True(t, hasStart,
			"hint-researcher-start-tip must be present in SeedCapabilityUIHintSlugs")
		_, hasCitation := slugSet["hint-researcher-citation-tip"]
		assert.True(t, hasCitation,
			"hint-researcher-citation-tip must be present in SeedCapabilityUIHintSlugs")
		assert.Equal(t, SeedUIHintTriggerChatStart, "agent_chat_start",
			"SeedUIHintTriggerChatStart must equal 'agent_chat_start'")
		assert.Equal(t, SeedUIHintTriggerPostToolResult, "post_tool_result",
			"SeedUIHintTriggerPostToolResult must equal 'post_tool_result'")
	})

	t.Run("Scenario_AnalystHintsCoverDocUploadAndConfidence", func(t *testing.T) {
		// Given an Analyst agent's most common new-user friction points are:
		//   (a) starting analysis without uploading documents first, and
		//   (b) not knowing they can request confidence ratings for findings,
		// When migration 000107 seeds analyst hints,
		// Then 2 analyst hints exist: one 'tip' for agent_chat_start (doc-upload)
		//   and one 'tip' for post_tool_result (confidence), both as 'tip' type.
		assert.Equal(t, 2, SeedAnalystUIHintCount,
			"SeedAnalystUIHintCount must be 2 for core-analyst")
		slugSet := map[string]struct{}{}
		for _, s := range SeedCapabilityUIHintSlugs {
			slugSet[s] = struct{}{}
		}
		_, hasDocUpload := slugSet["hint-analyst-doc-upload-tip"]
		assert.True(t, hasDocUpload,
			"hint-analyst-doc-upload-tip must be present in SeedCapabilityUIHintSlugs")
		_, hasConfidence := slugSet["hint-analyst-confidence-tip"]
		assert.True(t, hasConfidence,
			"hint-analyst-confidence-tip must be present in SeedCapabilityUIHintSlugs")
	})

	t.Run("Scenario_PlannerHintsCoverTaskTrackingAndGoalBreakdown", func(t *testing.T) {
		// Given a Planner agent's most common new-user friction points are:
		//   (a) not knowing the agent tracks tasks automatically throughout the
		//       conversation, and
		//   (b) providing overly granular goals instead of high-level objectives,
		// When migration 000107 seeds planner hints,
		// Then 2 planner hints exist: one 'info' for agent_chat_start (task tracking)
		//   and one 'tip' for agent_chat_start (goal breakdown), covering both.
		assert.Equal(t, 2, SeedPlannerUIHintCount,
			"SeedPlannerUIHintCount must be 2 for core-planner")
		slugSet := map[string]struct{}{}
		for _, s := range SeedCapabilityUIHintSlugs {
			slugSet[s] = struct{}{}
		}
		_, hasTask := slugSet["hint-planner-task-tip"]
		assert.True(t, hasTask,
			"hint-planner-task-tip must be present in SeedCapabilityUIHintSlugs")
		_, hasBreakdown := slugSet["hint-planner-breakdown-tip"]
		assert.True(t, hasBreakdown,
			"hint-planner-breakdown-tip must be present in SeedCapabilityUIHintSlugs")
		// Verify planner count accounts for its share of the total.
		remaining := SeedCapabilityUIHintCount - SeedResearcherUIHintCount - SeedAnalystUIHintCount
		assert.Equal(t, SeedPlannerUIHintCount, remaining,
			"SeedPlannerUIHintCount must equal total minus researcher and analyst counts")
	})
}
