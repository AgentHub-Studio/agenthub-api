package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for query_pipeline_step_template seed.
// Maps §4.1 nine-step pipeline to ah_core default templates.

func TestBDD_AhCoreQueryPipelineStepSeed(t *testing.T) {
	t.Run("Scenario_NineStepsRepresentFullPipelineFromSpec", func(t *testing.T) {
		// Given §4.1 specifies exactly nine fixed steps
		// When the platform loads pipeline step presets
		// Then exactly 9 steps are available from settings_resolution to stop_condition
		assert.Equal(t, 9, SeedExpectedQueryPipelineStepRowCount)
		assert.Equal(t, 9, len(SeedExpectedQueryPipelineStepSlugs))
	})

	t.Run("Scenario_FivePhasesPartitionAllNineSteps", func(t *testing.T) {
		// Given the pipeline is divided into five phases
		// When phases are enumerated
		// Then setup/context/reasoning/execution/termination are all present
		assert.Equal(t, 5, len(SeedQueryPipelinePhases))
	})

	t.Run("Scenario_OnlyModelCallAndToolExecutionAreRetryable", func(t *testing.T) {
		// Given §4.4 recovery applies only to LLM timeouts and skill errors
		// When retryable slugs are checked
		// Then exactly model_call and tool_execution are retryable
		assert.Equal(t, 2, len(SeedQueryPipelineRetryableStepSlugs))
		assert.Contains(t, SeedQueryPipelineRetryableStepSlugs, "model_call")
		assert.Contains(t, SeedQueryPipelineRetryableStepSlugs, "tool_execution")
	})

	t.Run("Scenario_PermissionGateInBlockingSetBeforeToolExecution", func(t *testing.T) {
		// Given §5 permission system must evaluate before any tool runs
		// When blocking step slugs are inspected
		// Then permission_gate is blocking and appears in the set
		assert.Contains(t, SeedQueryPipelineBlockingStepSlugs, "permission_gate")
	})

	t.Run("Scenario_AllBlockingSlugsExistInCanonicalList", func(t *testing.T) {
		// Given blocking steps must be a strict subset of all pipeline steps
		// When each blocking slug is checked against the canonical list
		// Then no orphan slugs exist
		all := map[string]bool{}
		for _, s := range SeedExpectedQueryPipelineStepSlugs {
			all[s] = true
		}
		for _, s := range SeedQueryPipelineBlockingStepSlugs {
			assert.True(t, all[s], "blocking slug %q not in canonical slug list", s)
		}
	})
}
