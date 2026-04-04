package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- AgentMode ---

func TestAgentMode_Values(t *testing.T) {
	assert.Equal(t, agentic.AgentMode("normal"), agentic.ModeNormal)
	assert.Equal(t, agentic.AgentMode("coordinator"), agentic.ModeCoordinator)
	assert.Equal(t, agentic.AgentMode("worker"), agentic.ModeWorker)
}

// --- ExecutionPhase ---

func TestExecutionPhase_Values(t *testing.T) {
	assert.Equal(t, agentic.TaskPhase("research"), agentic.PhaseResearch)
	assert.Equal(t, agentic.TaskPhase("synthesis"), agentic.PhaseSynthesis)
	assert.Equal(t, agentic.TaskPhase("implementation"), agentic.PhaseImplementation)
	assert.Equal(t, agentic.TaskPhase("verification"), agentic.PhaseVerification)
}

// --- ParallelismStrategy ---

func TestParallelismStrategy_Values(t *testing.T) {
	assert.Equal(t, agentic.ParallelismStrategy("free"), agentic.ParallelFree)
	assert.Equal(t, agentic.ParallelismStrategy("serialize"), agentic.ParallelSerialize)
}

// --- NewCoordinatorModeState ---

func TestNewCoordinatorModeState(t *testing.T) {
	s := agentic.NewCoordinatorModeState(agentic.ModeNormal, agentic.CoordinatorModeConfig{})
	assert.Equal(t, agentic.ModeNormal, s.Mode())
	assert.Equal(t, agentic.PhaseResearch, s.Phase())
	assert.Equal(t, 0, s.WorkerCount())
}

// --- Mode ---

func TestCoordinatorModeState_SetMode(t *testing.T) {
	s := agentic.NewCoordinatorModeState(agentic.ModeNormal, agentic.CoordinatorModeConfig{})
	s.SetMode(agentic.ModeCoordinator)
	assert.True(t, s.IsCoordinator())
	assert.False(t, s.IsWorker())

	s.SetMode(agentic.ModeWorker)
	assert.False(t, s.IsCoordinator())
	assert.True(t, s.IsWorker())
}

// --- Phase ---

func TestCoordinatorModeState_SetPhase(t *testing.T) {
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, agentic.CoordinatorModeConfig{})
	s.SetPhase(agentic.PhaseImplementation)
	assert.Equal(t, agentic.PhaseImplementation, s.Phase())
}

// --- Workers ---

func TestCoordinatorModeState_IncrementWorkers(t *testing.T) {
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, agentic.CoordinatorModeConfig{MaxWorkers: 2})

	assert.True(t, s.IncrementWorkers())
	assert.Equal(t, 1, s.WorkerCount())

	assert.True(t, s.IncrementWorkers())
	assert.Equal(t, 2, s.WorkerCount())

	assert.False(t, s.IncrementWorkers(), "should fail at max workers")
	assert.Equal(t, 2, s.WorkerCount())
}

func TestCoordinatorModeState_DecrementWorkers(t *testing.T) {
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, agentic.CoordinatorModeConfig{})
	s.IncrementWorkers()
	s.IncrementWorkers()
	s.DecrementWorkers()
	assert.Equal(t, 1, s.WorkerCount())

	// Should not go below 0.
	s.DecrementWorkers()
	s.DecrementWorkers()
	assert.Equal(t, 0, s.WorkerCount())
}

// --- GetWorkerTools ---

func TestCoordinatorModeState_GetWorkerTools_AllTools(t *testing.T) {
	config := agentic.CoordinatorModeConfig{
		InternalTools: []string{"admin", "deploy"},
	}
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, config)

	tools := s.GetWorkerTools("any-worker", []string{"read", "write", "admin", "deploy", "search"})
	assert.Equal(t, []string{"read", "write", "search"}, tools)
}

func TestCoordinatorModeState_GetWorkerTools_WithAllowList(t *testing.T) {
	config := agentic.CoordinatorModeConfig{
		Workers: []agentic.WorkerConfig{
			{Name: "reader", AllowedTools: []string{"read", "search", "admin"}},
		},
		InternalTools: []string{"admin"},
	}
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, config)

	tools := s.GetWorkerTools("reader", []string{"read", "write", "admin", "search"})
	assert.Equal(t, []string{"read", "search"}, tools, "should filter internal from allowed list")
}

func TestCoordinatorModeState_GetWorkerTools_WithDenyList(t *testing.T) {
	config := agentic.CoordinatorModeConfig{
		Workers: []agentic.WorkerConfig{
			{Name: "safe-worker", DeniedTools: []string{"write", "delete"}},
		},
	}
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, config)

	tools := s.GetWorkerTools("safe-worker", []string{"read", "write", "delete", "search"})
	assert.Equal(t, []string{"read", "search"}, tools)
}

func TestCoordinatorModeState_GetWorkerTools_UnknownWorker(t *testing.T) {
	config := agentic.CoordinatorModeConfig{
		InternalTools: []string{"admin"},
	}
	s := agentic.NewCoordinatorModeState(agentic.ModeCoordinator, config)

	tools := s.GetWorkerTools("unknown", []string{"read", "admin"})
	assert.Equal(t, []string{"read"}, tools)
}

// --- BuildCoordinatorModePrompt ---

func TestBuildCoordinatorModePrompt(t *testing.T) {
	config := agentic.CoordinatorModeConfig{
		Workers: []agentic.WorkerConfig{
			{Name: "reader", Parallelism: agentic.ParallelFree, MaxTurns: 10},
			{Name: "writer", Parallelism: agentic.ParallelSerialize},
		},
	}

	prompt := agentic.BuildCoordinatorModePrompt(config)
	assert.Contains(t, prompt, "coordinator mode")
	assert.Contains(t, prompt, "Research")
	assert.Contains(t, prompt, "Synthesis")
	assert.Contains(t, prompt, "Implementation")
	assert.Contains(t, prompt, "Verification")
	assert.Contains(t, prompt, "reader")
	assert.Contains(t, prompt, "writer")
	assert.Contains(t, prompt, "max turns: 10")
}

func TestBuildCoordinatorModePrompt_NoWorkers(t *testing.T) {
	prompt := agentic.BuildCoordinatorModePrompt(agentic.CoordinatorModeConfig{})
	assert.Contains(t, prompt, "coordinator mode")
	assert.NotContains(t, prompt, "Available Workers")
}
