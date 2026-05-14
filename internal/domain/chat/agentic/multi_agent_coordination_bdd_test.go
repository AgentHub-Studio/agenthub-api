package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_MultiAgentCoordination(t *testing.T) {
	t.Run("Scenario_SequentialStrategyRunsOneAtATime", func(t *testing.T) {
		// Given 3 tasks in sequential strategy,
		// When the coordinator computes NextBatch,
		// Then only the first task is returned — others wait.
		p := CoordinationPlan{Strategy: MultiAgentSequential, Tasks: []CoordinatedTask{
			{TaskID: "step1", Description: "fetch"},
			{TaskID: "step2", Description: "transform"},
			{TaskID: "step3", Description: "store"},
		}}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "step1", Status: MultiAgentTaskPending},
			{TaskID: "step2", Status: MultiAgentTaskPending},
			{TaskID: "step3", Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.Equal(t, []string{"step1"}, next)
	})

	t.Run("Scenario_ParallelStrategyFansOutAll", func(t *testing.T) {
		// Given 3 independent tasks in parallel strategy,
		// When the coordinator computes NextBatch,
		// Then all 3 task IDs are returned — caller spawns them concurrently.
		p := CoordinationPlan{Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{
			{TaskID: "search1", Description: "search a"},
			{TaskID: "search2", Description: "search b"},
			{TaskID: "search3", Description: "search c"},
		}}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "search1", Status: MultiAgentTaskPending},
			{TaskID: "search2", Status: MultiAgentTaskPending},
			{TaskID: "search3", Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.Equal(t, 3, len(next))
	})

	t.Run("Scenario_DAGStrategyRespectsDependencies", func(t *testing.T) {
		// Given a diamond DAG: build → (test, lint) → deploy,
		// When initially nothing has run,
		// Then only build is eligible.
		p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
			{TaskID: "build", Description: "compile"},
			{TaskID: "test", Description: "run tests", DependsOn: []string{"build"}},
			{TaskID: "lint", Description: "lint", DependsOn: []string{"build"}},
			{TaskID: "deploy", Description: "deploy", DependsOn: []string{"test", "lint"}},
		}}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "build", Status: MultiAgentTaskPending},
			{TaskID: "test", DependsOn: []string{"build"}, Status: MultiAgentTaskPending},
			{TaskID: "lint", DependsOn: []string{"build"}, Status: MultiAgentTaskPending},
			{TaskID: "deploy", DependsOn: []string{"test", "lint"}, Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.Equal(t, []string{"build"}, next)
	})

	t.Run("Scenario_DAGUnblocksTwoAfterBuildCompletes", func(t *testing.T) {
		// Given the same diamond and build just completed,
		// When NextBatch runs,
		// Then test AND lint are returned (parallel fan-out).
		p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
			{TaskID: "build", Description: "compile"},
			{TaskID: "test", Description: "test", DependsOn: []string{"build"}},
			{TaskID: "lint", Description: "lint", DependsOn: []string{"build"}},
		}}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "build", Status: MultiAgentTaskCompleted},
			{TaskID: "test", DependsOn: []string{"build"}, Status: MultiAgentTaskPending},
			{TaskID: "lint", DependsOn: []string{"build"}, Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.ElementsMatch(t, []string{"test", "lint"}, next)
	})

	t.Run("Scenario_DependencyCycleDetectedAtPlanLoad", func(t *testing.T) {
		// Given a buggy plan with a → b → a,
		// When NewMultiAgentCoordinator runs,
		// Then it fails fast at construction (never accepted into the
		// runtime).
		p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x", DependsOn: []string{"b"}},
			{TaskID: "b", Description: "y", DependsOn: []string{"a"}},
		}}
		_, err := NewMultiAgentCoordinator(p)
		assert.ErrorIs(t, err, ErrMultiAgentCycleDetected)
	})

	t.Run("Scenario_MaxParallelismCapPreservesBudget", func(t *testing.T) {
		// Given 5 tasks and MaxParallelism=2 with 1 already running,
		// When NextBatch runs,
		// Then only 1 more task is returned (budget = 2 - 1 = 1).
		tasks := []CoordinatedTask{}
		for _, id := range []string{"a", "b", "c", "d", "e"} {
			tasks = append(tasks, CoordinatedTask{TaskID: id, Description: id})
		}
		p := CoordinationPlan{Strategy: MultiAgentParallel, MaxParallelism: 2, Tasks: tasks}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "a", Status: MultiAgentTaskRunning},
			{TaskID: "b", Status: MultiAgentTaskPending},
			{TaskID: "c", Status: MultiAgentTaskPending},
			{TaskID: "d", Status: MultiAgentTaskPending},
			{TaskID: "e", Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.Equal(t, 1, len(next))
	})

	t.Run("Scenario_AbortOnFailureStopsAllProgress", func(t *testing.T) {
		// Given a strict tenant: any failure aborts the plan,
		// When task A fails while B is still pending,
		// Then B is NOT spawned — coordinator returns empty.
		p := CoordinationPlan{
			Strategy:      MultiAgentParallel,
			FailurePolicy: MultiAgentAbortOnFailure,
			Tasks: []CoordinatedTask{
				{TaskID: "a", Description: "x"},
				{TaskID: "b", Description: "y"},
			},
		}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "a", Status: MultiAgentTaskFailed},
			{TaskID: "b", Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.Empty(t, next)
	})

	t.Run("Scenario_ContinueOnFailureKeepsIndependentBranches", func(t *testing.T) {
		// Given a tolerant tenant: keep going despite failures,
		// When task A fails while B is independent,
		// Then B continues.
		p := CoordinationPlan{
			Strategy:      MultiAgentParallel,
			FailurePolicy: MultiAgentContinueOnFailure,
			Tasks: []CoordinatedTask{
				{TaskID: "a", Description: "x"},
				{TaskID: "b", Description: "y"},
			},
		}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "a", Status: MultiAgentTaskFailed},
			{TaskID: "b", Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		assert.Equal(t, []string{"b"}, next)
	})

	t.Run("Scenario_TerminalStateSignalsCompletionToParent", func(t *testing.T) {
		// Given the parent agent polls for completion,
		// When IsTerminalState returns true,
		// Then the parent knows it can collect results (every task
		// completed/failed/skipped).
		c, _ := NewMultiAgentCoordinator(CoordinationPlan{
			Strategy: MultiAgentParallel,
			Tasks:    []CoordinatedTask{{TaskID: "a", Description: "x"}, {TaskID: "b", Description: "y"}},
		})
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "a", Status: MultiAgentTaskCompleted},
			{TaskID: "b", Status: MultiAgentTaskFailed},
		}}
		assert.True(t, c.IsTerminalState(state))
	})

	t.Run("Scenario_SummariseEnablesDashboardAndAudit", func(t *testing.T) {
		// Given an operator dashboard needs at-a-glance state,
		// When SummariseState runs,
		// Then counts per status surface for display + audit logging.
		c, _ := NewMultiAgentCoordinator(CoordinationPlan{
			Strategy: MultiAgentParallel,
			Tasks: []CoordinatedTask{
				{TaskID: "a", Description: "x"}, {TaskID: "b", Description: "y"},
				{TaskID: "c", Description: "z"}, {TaskID: "d", Description: "w"},
			},
		})
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "a", Status: MultiAgentTaskCompleted},
			{TaskID: "b", Status: MultiAgentTaskRunning},
			{TaskID: "c", Status: MultiAgentTaskFailed},
			{TaskID: "d", Status: MultiAgentTaskPending},
		}}
		summary := c.SummariseState(state)
		assert.Equal(t, 1, summary[MultiAgentTaskCompleted])
		assert.Equal(t, 1, summary[MultiAgentTaskRunning])
		assert.Equal(t, 1, summary[MultiAgentTaskFailed])
		assert.Equal(t, 1, summary[MultiAgentTaskPending])
	})

	t.Run("Scenario_SelfDependencyRejectedAtConstruction", func(t *testing.T) {
		// Given a task accidentally lists itself as a dependency,
		// When the plan is validated,
		// Then a self-dependency error fires — never accepted.
		p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x", DependsOn: []string{"a"}},
		}}
		err := p.Validate()
		assert.ErrorIs(t, err, ErrMultiAgentSelfDependency)
	})

	t.Run("Scenario_UnknownDependencyRejectedAtConstruction", func(t *testing.T) {
		// Given a task references a non-existent dep,
		// When the plan is validated,
		// Then unknown-dep error fires — typos caught at plan-load not runtime.
		p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x", DependsOn: []string{"ghost"}},
		}}
		err := p.Validate()
		assert.ErrorIs(t, err, ErrMultiAgentUnknownDependency)
	})

	t.Run("Scenario_ParallelOutputIsDeterministicSortedByTaskID", func(t *testing.T) {
		// Given parallel strategy returns multiple eligible task IDs,
		// When NextBatch returns,
		// Then IDs are sorted — repeatable for audit hashes.
		p := CoordinationPlan{Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{
			{TaskID: "zeta", Description: "z"},
			{TaskID: "alpha", Description: "a"},
			{TaskID: "mu", Description: "m"},
		}}
		c, _ := NewMultiAgentCoordinator(p)
		state := CoordinationState{Tasks: []CoordinatedTask{
			{TaskID: "zeta", Status: MultiAgentTaskPending},
			{TaskID: "alpha", Status: MultiAgentTaskPending},
			{TaskID: "mu", Status: MultiAgentTaskPending},
		}}
		next, _ := c.NextBatch(state)
		require.Equal(t, 3, len(next))
		assert.Equal(t, "alpha", next[0])
		assert.Equal(t, "mu", next[1])
		assert.Equal(t, "zeta", next[2])
	})
}
