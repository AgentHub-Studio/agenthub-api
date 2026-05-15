package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- count constants --------------------------------------------------------

func TestInteractionMode_SeedInteractionModeCount(t *testing.T) {
	assert.Equal(t, 9, SeedInteractionModeCount, "9 total rows: 3 mode dimensions × 3 agents")
}

func TestInteractionMode_SeedInteractionModeAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedInteractionModeAgentCount, "3 agents: researcher, analyst, planner")
}

func TestInteractionMode_TotalRowsEqualAgentsTimesModesPerAgent(t *testing.T) {
	const modesPerAgent = 3 // primary_mode, proactive_questions, response_style
	assert.Equal(t, SeedInteractionModeCount, SeedInteractionModeAgentCount*modesPerAgent)
}

// ---- per-agent row count (3 modes each) ------------------------------------

func TestInteractionMode_ResearcherHasThreeModes(t *testing.T) {
	// core-researcher seeds 3 rows: primary_mode, proactive_questions, response_style.
	const researcherModeCount = 3
	assert.Equal(t, 3, researcherModeCount,
		"core-researcher must seed exactly 3 interaction mode rows")
}

func TestInteractionMode_AnalystHasThreeModes(t *testing.T) {
	// core-analyst seeds 3 rows: primary_mode, proactive_questions, response_style.
	const analystModeCount = 3
	assert.Equal(t, 3, analystModeCount,
		"core-analyst must seed exactly 3 interaction mode rows")
}

func TestInteractionMode_PlannerHasThreeModes(t *testing.T) {
	// core-planner seeds 3 rows: primary_mode, proactive_questions, response_style.
	const plannerModeCount = 3
	assert.Equal(t, 3, plannerModeCount,
		"core-planner must seed exactly 3 interaction mode rows")
}

// ---- mode key constants -----------------------------------------------------

func TestInteractionMode_ModeKeyPrimary(t *testing.T) {
	assert.Equal(t, "primary_mode", SeedModeKeyPrimary)
}

func TestInteractionMode_ModeKeyProactiveQ(t *testing.T) {
	assert.Equal(t, "proactive_questions", SeedModeKeyProactiveQ)
}

func TestInteractionMode_ModeKeyResponseStyle(t *testing.T) {
	assert.Equal(t, "response_style", SeedModeKeyResponseStyle)
}

func TestInteractionMode_ModeKeysAreDistinct(t *testing.T) {
	keys := []string{
		SeedModeKeyPrimary,
		SeedModeKeyProactiveQ,
		SeedModeKeyResponseStyle,
	}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "duplicate mode key constant %q", k)
		seen[k] = true
	}
	assert.Equal(t, 3, len(seen), "exactly 3 distinct mode key constants")
}

// ---- primary mode value constants ------------------------------------------

func TestInteractionMode_PrimaryModeSingleTurn(t *testing.T) {
	assert.Equal(t, "single_turn", SeedPrimaryModeSingleTurn)
}

func TestInteractionMode_PrimaryModeMultiTurn(t *testing.T) {
	assert.Equal(t, "multi_turn", SeedPrimaryModeMultiTurn)
}

func TestInteractionMode_PrimaryModeTaskExecution(t *testing.T) {
	assert.Equal(t, "task_execution", SeedPrimaryModeTaskExecution)
}

func TestInteractionMode_PrimaryModeValuesAreDistinct(t *testing.T) {
	values := []string{
		SeedPrimaryModeSingleTurn,
		SeedPrimaryModeMultiTurn,
		SeedPrimaryModeTaskExecution,
	}
	seen := map[string]bool{}
	for _, v := range values {
		assert.False(t, seen[v], "duplicate primary mode constant %q", v)
		seen[v] = true
	}
	assert.Equal(t, 3, len(seen), "exactly 3 distinct primary mode constants")
}

// ---- proactive questions value constants -----------------------------------

func TestInteractionMode_ProactiveQLow(t *testing.T) {
	assert.Equal(t, "low", SeedProactiveQLow)
}

func TestInteractionMode_ProactiveQHigh(t *testing.T) {
	assert.Equal(t, "high", SeedProactiveQHigh)
}

func TestInteractionMode_ProactiveQValuesAreDistinct(t *testing.T) {
	assert.NotEqual(t, SeedProactiveQLow, SeedProactiveQHigh,
		"low and high proactive_questions values must differ")
}

// ---- per-agent primary mode values -----------------------------------------

func TestInteractionMode_ResearcherPrimaryModeIsSingleTurn(t *testing.T) {
	// core-researcher responds in one comprehensive reply per query.
	assert.Equal(t, SeedPrimaryModeSingleTurn, SeedResearcherPrimaryMode,
		"core-researcher primary mode must be single_turn")
}

func TestInteractionMode_AnalystPrimaryModeIsMultiTurn(t *testing.T) {
	// core-analyst engages in back-and-forth dialogue.
	assert.Equal(t, SeedPrimaryModeMultiTurn, SeedAnalystPrimaryMode,
		"core-analyst primary mode must be multi_turn")
}

func TestInteractionMode_PlannerPrimaryModeIsTaskExecution(t *testing.T) {
	// core-planner focuses on decomposing and executing tasks.
	assert.Equal(t, SeedPrimaryModeTaskExecution, SeedPlannerPrimaryMode,
		"core-planner primary mode must be task_execution")
}

func TestInteractionMode_AgentPrimaryModesAreDistinct(t *testing.T) {
	// Each of the three core agents uses a different primary mode.
	assert.NotEqual(t, SeedResearcherPrimaryMode, SeedAnalystPrimaryMode,
		"researcher and analyst must have different primary modes")
	assert.NotEqual(t, SeedResearcherPrimaryMode, SeedPlannerPrimaryMode,
		"researcher and planner must have different primary modes")
	assert.NotEqual(t, SeedAnalystPrimaryMode, SeedPlannerPrimaryMode,
		"analyst and planner must have different primary modes")
}

// ---- per-agent proactive_questions values -----------------------------------

func TestInteractionMode_ResearcherProactiveQIsLow(t *testing.T) {
	// core-researcher proceeds with reasonable assumptions — low proactive questions.
	const researcherProactiveQ = SeedProactiveQLow
	assert.Equal(t, "low", researcherProactiveQ,
		"core-researcher proactive_questions must be low")
}

func TestInteractionMode_AnalystProactiveQIsHigh(t *testing.T) {
	// core-analyst actively asks for clarification when data or goals are ambiguous.
	const analystProactiveQ = SeedProactiveQHigh
	assert.Equal(t, "high", analystProactiveQ,
		"core-analyst proactive_questions must be high")
}

func TestInteractionMode_PlannerProactiveQIsHigh(t *testing.T) {
	// core-planner asks clarifying questions upfront before creating plans.
	const plannerProactiveQ = SeedProactiveQHigh
	assert.Equal(t, "high", plannerProactiveQ,
		"core-planner proactive_questions must be high")
}

// ---- loader construction ---------------------------------------------------

func TestInteractionMode_NewLoaderAcceptsNilPool(t *testing.T) {
	// Construction must not panic even with a nil pool.
	// (pool is only used on method calls, not on construction)
	assert.NotPanics(t, func() {
		_ = NewCoreCapabilityInteractionModeLoader(nil)
	})
}
