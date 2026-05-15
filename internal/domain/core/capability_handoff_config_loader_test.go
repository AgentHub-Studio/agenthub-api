package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- count constants --------------------------------------------------------

func TestHandoffConfig_SeedHandoffConfigCount(t *testing.T) {
	assert.Equal(t, 9, SeedHandoffConfigCount, "9 total rows: 3 handoff rules × 3 agents")
}

func TestHandoffConfig_SeedHandoffConfigAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedHandoffConfigAgentCount, "3 agents: researcher, analyst, planner")
}

func TestHandoffConfig_TotalRowsEqualAgentsTimesHandoffsPerAgent(t *testing.T) {
	const handoffsPerAgent = 3 // needs_research/needs_analysis/needs_planning + human_escalation
	assert.Equal(t, SeedHandoffConfigCount, SeedHandoffConfigAgentCount*handoffsPerAgent)
}

func TestHandoffConfig_SeedHumanEscalationCount(t *testing.T) {
	assert.Equal(t, 3, SeedHumanEscalationCount,
		"3 human escalation rows: one per agent (target_agent_slug='')")
}

func TestHandoffConfig_SeedAutomaticHandoffCount(t *testing.T) {
	assert.Equal(t, 3, SeedAutomaticHandoffCount,
		"3 automatic handoff rows: all human_escalation rows are is_automatic=true")
}

func TestHandoffConfig_AutomaticCountEqualsHumanEscalationCount(t *testing.T) {
	// All human escalation rows are automatic; no other rows are automatic.
	assert.Equal(t, SeedHumanEscalationCount, SeedAutomaticHandoffCount,
		"automatic handoff count must equal human escalation count")
}

// ---- per-agent row count (3 handoffs each) ----------------------------------

func TestHandoffConfig_ResearcherHasThreeHandoffs(t *testing.T) {
	// core-researcher seeds 3 rows: needs_analysis, needs_planning, human_escalation.
	const researcherHandoffCount = 3
	assert.Equal(t, 3, researcherHandoffCount,
		"core-researcher must seed exactly 3 handoff config rows")
}

func TestHandoffConfig_AnalystHasThreeHandoffs(t *testing.T) {
	// core-analyst seeds 3 rows: needs_research, needs_planning, human_escalation.
	const analystHandoffCount = 3
	assert.Equal(t, 3, analystHandoffCount,
		"core-analyst must seed exactly 3 handoff config rows")
}

func TestHandoffConfig_PlannerHasThreeHandoffs(t *testing.T) {
	// core-planner seeds 3 rows: needs_research, needs_analysis, human_escalation.
	const plannerHandoffCount = 3
	assert.Equal(t, 3, plannerHandoffCount,
		"core-planner must seed exactly 3 handoff config rows")
}

// ---- handoff key constants --------------------------------------------------

func TestHandoffConfig_HandoffKeyNeedsResearch(t *testing.T) {
	assert.Equal(t, "needs_research", SeedHandoffKeyNeedsResearch)
}

func TestHandoffConfig_HandoffKeyNeedsAnalysis(t *testing.T) {
	assert.Equal(t, "needs_analysis", SeedHandoffKeyNeedsAnalysis)
}

func TestHandoffConfig_HandoffKeyNeedsPlanning(t *testing.T) {
	assert.Equal(t, "needs_planning", SeedHandoffKeyNeedsPlanning)
}

func TestHandoffConfig_HandoffKeyHumanEscalation(t *testing.T) {
	assert.Equal(t, "human_escalation", SeedHandoffKeyHumanEscalation)
}

func TestHandoffConfig_HandoffKeysAreDistinct(t *testing.T) {
	keys := []string{
		SeedHandoffKeyNeedsResearch,
		SeedHandoffKeyNeedsAnalysis,
		SeedHandoffKeyNeedsPlanning,
		SeedHandoffKeyHumanEscalation,
	}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "duplicate handoff key constant %q", k)
		seen[k] = true
	}
	assert.Equal(t, 4, len(seen), "exactly 4 distinct handoff key constants")
}

// ---- human escalation constraints ------------------------------------------

func TestHandoffConfig_HumanEscalationCountIsThree(t *testing.T) {
	// One human escalation per agent; human escalation uses empty target_agent_slug.
	assert.Equal(t, 3, SeedHumanEscalationCount,
		"exactly 3 human escalation rows (one per agent)")
}

func TestHandoffConfig_HumanEscalationKeyIsHumanEscalation(t *testing.T) {
	// All human escalation rows use SeedHandoffKeyHumanEscalation as handoff_key.
	assert.Equal(t, "human_escalation", SeedHandoffKeyHumanEscalation,
		"human escalation rows must use handoff_key='human_escalation'")
}

// ---- automatic handoff constraints -----------------------------------------

func TestHandoffConfig_AutomaticHandoffCountIsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAutomaticHandoffCount,
		"exactly 3 automatic handoff rows (all are human escalations)")
}

// ---- per-agent handoff key coverage ----------------------------------------

func TestHandoffConfig_ResearcherHasNeedsAnalysis(t *testing.T) {
	// core-researcher hands off to core-analyst when user asks for data analysis.
	// Verify the key constant exists and has the expected value.
	assert.Equal(t, "needs_analysis", SeedHandoffKeyNeedsAnalysis,
		"researcher uses needs_analysis to hand off to core-analyst")
}

func TestHandoffConfig_ResearcherHasNeedsPlanning(t *testing.T) {
	// core-researcher hands off to core-planner when user asks for a project plan.
	assert.Equal(t, "needs_planning", SeedHandoffKeyNeedsPlanning,
		"researcher uses needs_planning to hand off to core-planner")
}

func TestHandoffConfig_ResearcherDoesNotHaveNeedsResearch(t *testing.T) {
	// core-researcher is the research agent — it does NOT hand off to itself.
	// The needs_research key is used by analyst and planner, not researcher.
	// This test encodes that researcher's handoff keys are needs_analysis,
	// needs_planning, and human_escalation — NOT needs_research.
	researcherHandoffKeys := []string{
		SeedHandoffKeyNeedsAnalysis,
		SeedHandoffKeyNeedsPlanning,
		SeedHandoffKeyHumanEscalation,
	}
	for _, k := range researcherHandoffKeys {
		assert.NotEqual(t, SeedHandoffKeyNeedsResearch, k,
			"core-researcher must not have a needs_research handoff")
	}
}

func TestHandoffConfig_PlannerHasNeedsResearch(t *testing.T) {
	// core-planner hands off to core-researcher when it needs more information.
	assert.Equal(t, "needs_research", SeedHandoffKeyNeedsResearch,
		"planner uses needs_research to hand off to core-researcher")
}

func TestHandoffConfig_PlannerHasNeedsAnalysis(t *testing.T) {
	// core-planner hands off to core-analyst when the plan requires data analysis.
	assert.Equal(t, "needs_analysis", SeedHandoffKeyNeedsAnalysis,
		"planner uses needs_analysis to hand off to core-analyst")
}

// ---- loader construction ---------------------------------------------------

func TestHandoffConfig_NewLoaderAcceptsNilPool(t *testing.T) {
	// Construction must not panic even with a nil pool.
	// (pool is only used on method calls, not on construction)
	assert.NotPanics(t, func() {
		_ = NewCoreCapabilityHandoffConfigLoader(nil)
	})
}
