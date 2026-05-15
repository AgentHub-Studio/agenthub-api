package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryPipelineSequence_HasNineSteps(t *testing.T) {
	assert.Equal(t, 9, len(QueryPipelineSequence))
}

func TestIsQueryPipelineSequentiallyOrdered_ReturnsTrue(t *testing.T) {
	assert.True(t, IsQueryPipelineSequentiallyOrdered())
}

func TestQueryPipelineSequence_AllStepOrdersDistinct(t *testing.T) {
	seen := map[int]AgentQueryPipelineStep{}
	for _, step := range QueryPipelineSequence {
		p := queryPipelineStepProfiles[step]
		_, exists := seen[p.StepOrder]
		assert.False(t, exists, "duplicate StepOrder %d for step %q", p.StepOrder, step)
		seen[p.StepOrder] = step
	}
}

func TestQueryPipelineSequence_OrdersOneToNine(t *testing.T) {
	for i, step := range QueryPipelineSequence {
		p := queryPipelineStepProfiles[step]
		assert.Equal(t, i+1, p.StepOrder, "step %q should have order %d", step, i+1)
	}
}

func TestAgentQueryPipelineRegistry_Profile_KnownStep(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	p, ok := r.Profile(QueryStepModelCall)
	require.True(t, ok)
	assert.Equal(t, QueryStepModelCall, p.Step)
	assert.Equal(t, 5, p.StepOrder)
	assert.Equal(t, QueryPhaseReasoning, p.Phase)
	assert.True(t, p.CanBlock)
	assert.True(t, p.IsRetryable)
	assert.True(t, p.IsPerIteration)
}

func TestAgentQueryPipelineRegistry_Profile_UnknownStep(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	_, ok := r.Profile("not_a_real_step")
	assert.False(t, ok)
}

func TestAgentQueryPipelineRegistry_AllSteps_Length(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.AllSteps()
	assert.Equal(t, 9, len(steps))
}

func TestAgentQueryPipelineRegistry_AllSteps_DefensiveCopy(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.AllSteps()
	original := steps[0]
	steps[0] = "mutated"
	steps2 := r.AllSteps()
	assert.Equal(t, original, steps2[0], "AllSteps must return a defensive copy")
}

func TestAgentQueryPipelineRegistry_BlockingSteps(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	blocking := r.BlockingSteps()
	expected := []AgentQueryPipelineStep{
		QueryStepPreModelShapers,
		QueryStepModelCall,
		QueryStepPermissionGate,
		QueryStepStopCondition,
	}
	assert.Equal(t, expected, blocking)
}

func TestAgentQueryPipelineRegistry_RetryableSteps(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	retryable := r.RetryableSteps()
	expected := []AgentQueryPipelineStep{
		QueryStepModelCall,
		QueryStepToolExecution,
	}
	assert.Equal(t, expected, retryable)
}

func TestAgentQueryPipelineRegistry_StepsInPhase_Setup(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.StepsInPhase(QueryPhaseSetup)
	expected := []AgentQueryPipelineStep{
		QueryStepSettingsResolution,
		QueryStepMutableStateInit,
	}
	assert.Equal(t, expected, steps)
}

func TestAgentQueryPipelineRegistry_StepsInPhase_Context(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.StepsInPhase(QueryPhaseContext)
	expected := []AgentQueryPipelineStep{
		QueryStepContextAssembly,
		QueryStepPreModelShapers,
	}
	assert.Equal(t, expected, steps)
}

func TestAgentQueryPipelineRegistry_StepsInPhase_Reasoning(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.StepsInPhase(QueryPhaseReasoning)
	assert.Equal(t, []AgentQueryPipelineStep{QueryStepModelCall}, steps)
}

func TestAgentQueryPipelineRegistry_StepsInPhase_Execution(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.StepsInPhase(QueryPhaseExecution)
	expected := []AgentQueryPipelineStep{
		QueryStepToolUseDispatch,
		QueryStepPermissionGate,
		QueryStepToolExecution,
	}
	assert.Equal(t, expected, steps)
}

func TestAgentQueryPipelineRegistry_StepsInPhase_Termination(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.StepsInPhase(QueryPhaseTermination)
	assert.Equal(t, []AgentQueryPipelineStep{QueryStepStopCondition}, steps)
}

func TestAgentQueryPipelineRegistry_StepsInPhase_Unknown(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	steps := r.StepsInPhase("not_a_phase")
	assert.Empty(t, steps)
}

func TestQueryPipelineStepProfiles_SetupStepsNotPerIteration(t *testing.T) {
	for _, step := range []AgentQueryPipelineStep{QueryStepSettingsResolution, QueryStepMutableStateInit} {
		p := queryPipelineStepProfiles[step]
		assert.False(t, p.IsPerIteration, "setup step %q must not be per-iteration", step)
	}
}

func TestQueryPipelineStepProfiles_NonSetupStepsArePerIteration(t *testing.T) {
	setup := map[AgentQueryPipelineStep]bool{
		QueryStepSettingsResolution: true,
		QueryStepMutableStateInit:   true,
	}
	for _, step := range QueryPipelineSequence {
		if setup[step] {
			continue
		}
		p := queryPipelineStepProfiles[step]
		assert.True(t, p.IsPerIteration, "step %q must be per-iteration", step)
	}
}

func TestQueryPipelineStepProfiles_SettingsResolution(t *testing.T) {
	p := queryPipelineStepProfiles[QueryStepSettingsResolution]
	assert.Equal(t, 1, p.StepOrder)
	assert.Equal(t, QueryPhaseSetup, p.Phase)
	assert.False(t, p.CanBlock)
	assert.False(t, p.IsRetryable)
	assert.False(t, p.IsPerIteration)
}

func TestQueryPipelineStepProfiles_PermissionGate(t *testing.T) {
	p := queryPipelineStepProfiles[QueryStepPermissionGate]
	assert.Equal(t, 7, p.StepOrder)
	assert.Equal(t, QueryPhaseExecution, p.Phase)
	assert.True(t, p.CanBlock)
	assert.False(t, p.IsRetryable)
	assert.True(t, p.IsPerIteration)
}

func TestQueryPipelineStepProfiles_ToolExecutionRetryableNotBlocking(t *testing.T) {
	p := queryPipelineStepProfiles[QueryStepToolExecution]
	assert.True(t, p.IsRetryable)
	assert.False(t, p.CanBlock)
}

func TestQueryPipelineStepProfiles_AllPhasesHaveAtLeastOneStep(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	phases := []QueryPipelinePhase{
		QueryPhaseSetup, QueryPhaseContext, QueryPhaseReasoning,
		QueryPhaseExecution, QueryPhaseTermination,
	}
	for _, phase := range phases {
		steps := r.StepsInPhase(phase)
		assert.NotEmpty(t, steps, "phase %q must have at least one step", phase)
	}
}

func TestQueryPipelineStepProfiles_TotalStepsCoverAllPhases(t *testing.T) {
	r := NewAgentQueryPipelineRegistry()
	total := 0
	phases := []QueryPipelinePhase{
		QueryPhaseSetup, QueryPhaseContext, QueryPhaseReasoning,
		QueryPhaseExecution, QueryPhaseTermination,
	}
	for _, phase := range phases {
		total += len(r.StepsInPhase(phase))
	}
	assert.Equal(t, 9, total)
}
