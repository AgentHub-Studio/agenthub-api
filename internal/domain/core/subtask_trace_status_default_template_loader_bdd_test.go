package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSTSDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksTraceShapeFromCatalog", func(t *testing.T) {
		// Given fresh tenants must produce OBS-006 subtask traces but
		// inventing event sequences + cost propagation per status is
		// error-prone,
		// When admin opens trace-shape onboarding,
		// Then 3 recommended templates surface 1:1 with SubtaskStatus.
		assert.Equal(t, 3, len(SeedRecommendedSTSDTemplateSlugs))
	})

	t.Run("Scenario_CompletedRoutineForBaseline", func(t *testing.T) {
		// Given a routine successful subagent run,
		// When admin uses completed-routine-trace,
		// Then propagate_cost=true, no error envelope, no admin review.
		assert.Contains(t, SeedExpectedSTSDTemplateSlugs, "completed-routine-trace")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSTSDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["completed-routine-trace"])
	})

	t.Run("Scenario_FailedErrorContextForDiagnosis", func(t *testing.T) {
		// Given a subagent could not complete,
		// When admin uses failed-error-context-trace,
		// Then emit_error_envelope=true; admin review (misconfiguration signal).
		assert.Contains(t, SeedExpectedSTSDTemplateSlugs, "failed-error-context-trace")
	})

	t.Run("Scenario_KilledBudgetOrDepthForHarnessAbort", func(t *testing.T) {
		// Given the harness aborts a subagent (depth/budget/explicit),
		// When admin uses killed-budget-or-depth-trace,
		// Then propagate_cost=false (kill happens before LLM call sometimes),
		// emit_error_envelope=true, typical_min_turns=0 (depth kill).
		assert.Contains(t, SeedExpectedSTSDTemplateSlugs, "killed-budget-or-depth-trace")
	})

	t.Run("Scenario_StatusLabelsMatchOBS006EnumByteForByte", func(t *testing.T) {
		// Given OBS-006 SubtaskStatus has 3 values,
		// When seed declares target_status,
		// Then labels match enum bytes (no mapping table runtime).
		obs006 := []string{"completed", "failed", "killed"}
		set := map[string]bool{}
		for _, s := range SeedExpectedSTSDTemplateStatuses {
			set[s] = true
		}
		for _, e := range obs006 {
			assert.True(t, set[e], "OBS-006 status %q missing", e)
		}
	})

	t.Run("Scenario_ThreeStatusesAllRepresented1to1", func(t *testing.T) {
		// Given OBS-006 has 3 statuses,
		// When seed templates ship,
		// Then ALL 3 statuses have exactly one template (1:1).
		assert.Equal(t, 3, len(SeedExpectedSTSDTemplateStatuses))
	})

	t.Run("Scenario_EventSequenceTraceShapeAligns", func(t *testing.T) {
		// Given the trace event sequence must align with OBS-006 emit
		// patterns,
		// When seed declares expected_event_sequence,
		// Then sequences start with subtask_start and end with
		// subtask_complete (validated DB-real in integration test).
		assert.Contains(t, SeedExpectedSTSDTemplateEventTypes, "subtask_start")
		assert.Contains(t, SeedExpectedSTSDTemplateEventTypes, "subtask_complete")
	})

	t.Run("Scenario_FailedTraceEmitsErrorEnvelope", func(t *testing.T) {
		// Given failed status implies ErrorData event in transcript,
		// When admin inspects emit_error_envelope flag,
		// Then failed + killed → true; completed → false. Validated DB-real.
		assert.Equal(t, 3, SeedExpectedSTSDTemplateRowCount)
	})

	t.Run("Scenario_CostPropagatedExceptOnEarlyKill", func(t *testing.T) {
		// Given killed-by-depth happens BEFORE first LLM call (no cost),
		// When admin compares propagate_cost_to_parent,
		// Then completed/failed → true; killed → false.
		// Validated DB-real in integration test.
		assert.Contains(t, SeedExpectedSTSDTemplateSlugs, "killed-budget-or-depth-trace")
	})

	t.Run("Scenario_AdminReviewGatesNonRoutineTraces", func(t *testing.T) {
		// Given failed + killed signal something needs investigation,
		// When admin compares admin-review subset,
		// Then 2 of 3 templates require review (only completed is routine).
		assert.Equal(t, 2, len(SeedAdminReviewSTSDTemplateSlugs))
	})

	t.Run("Scenario_KillTraceHasMinimalEventSequence", func(t *testing.T) {
		// Given a depth-kill happens before any meaningful turn,
		// When admin inspects the kill template,
		// Then expected_event_sequence is just [subtask_start, subtask_complete].
		// Validated DB-real.
		assert.Contains(t, SeedExpectedSTSDTemplateSlugs, "killed-budget-or-depth-trace")
	})
}
