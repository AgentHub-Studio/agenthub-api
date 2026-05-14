package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for the §4.1 nine-step agent query pipeline.

func TestBDD_AgentQueryPipeline(t *testing.T) {
	t.Run("Scenario_NineStepsExecuteInFixedOrder", func(t *testing.T) {
		// Given the §4.1 specification mandating a fixed execution sequence
		// When the pipeline sequence is loaded
		// Then exactly 9 steps exist with orders 1..9 in canonical order
		assert.Equal(t, 9, len(QueryPipelineSequence))
		assert.True(t, IsQueryPipelineSequentiallyOrdered())
	})

	t.Run("Scenario_SetupStepsRunOncePerTurn", func(t *testing.T) {
		// Given settings_resolution and mutable_state_init are one-time setup
		// When the per-iteration flag is inspected for setup steps
		// Then both setup steps are marked as not per-iteration
		r := NewAgentQueryPipelineRegistry()
		setup := r.StepsInPhase(QueryPhaseSetup)
		assert.Equal(t, 2, len(setup))
		for _, s := range setup {
			p, _ := r.Profile(s)
			assert.False(t, p.IsPerIteration, "setup step %q must not be per-iteration", s)
		}
	})

	t.Run("Scenario_ModelCallAndToolExecutionAreRetryable", func(t *testing.T) {
		// Given §4.4 defines a recovery mechanism for transient failures
		// When retryable steps are queried
		// Then only model_call (LLM timeout) and tool_execution (skill errors) are retryable
		r := NewAgentQueryPipelineRegistry()
		retryable := r.RetryableSteps()
		assert.Equal(t, 2, len(retryable))
		assert.Contains(t, retryable, QueryStepModelCall)
		assert.Contains(t, retryable, QueryStepToolExecution)
	})

	t.Run("Scenario_PermissionGateBlocksPipelineBeforeExecution", func(t *testing.T) {
		// Given §5 defines the seven-layer permission system that gates tool use
		// When the permission_gate step profile is inspected
		// Then it can block the pipeline and precedes tool_execution in order
		r := NewAgentQueryPipelineRegistry()
		gateProfile, _ := r.Profile(QueryStepPermissionGate)
		execProfile, _ := r.Profile(QueryStepToolExecution)
		assert.True(t, gateProfile.CanBlock)
		assert.Less(t, gateProfile.StepOrder, execProfile.StepOrder)
	})

	t.Run("Scenario_FivePhasesPartitionAllNineSteps", func(t *testing.T) {
		// Given the pipeline is divided into setup/context/reasoning/execution/termination
		// When steps per phase are counted
		// Then the sum equals 9 and no step belongs to two phases
		r := NewAgentQueryPipelineRegistry()
		phases := []QueryPipelinePhase{
			QueryPhaseSetup, QueryPhaseContext, QueryPhaseReasoning,
			QueryPhaseExecution, QueryPhaseTermination,
		}
		seen := map[AgentQueryPipelineStep]bool{}
		total := 0
		for _, phase := range phases {
			for _, s := range r.StepsInPhase(phase) {
				assert.False(t, seen[s], "step %q appears in multiple phases", s)
				seen[s] = true
				total++
			}
		}
		assert.Equal(t, 9, total)
	})
}
