package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiAgent_IsValidStrategy(t *testing.T) {
	for _, s := range allMultiAgentStrategies {
		assert.True(t, IsValidMultiAgentStrategy(s))
	}
	assert.False(t, IsValidMultiAgentStrategy(MultiAgentStrategy("nope")))
}

func TestMultiAgent_IsValidTaskStatus(t *testing.T) {
	for _, s := range allMultiAgentTaskStatuses {
		assert.True(t, IsValidMultiAgentTaskStatus(s))
	}
	assert.False(t, IsValidMultiAgentTaskStatus(MultiAgentTaskStatus("nope")))
}

func TestMultiAgent_IsValidFailurePolicy(t *testing.T) {
	for _, p := range allMultiAgentFailurePolicies {
		assert.True(t, IsValidMultiAgentFailurePolicy(p))
	}
	assert.False(t, IsValidMultiAgentFailurePolicy(MultiAgentFailurePolicy("nope")))
}

func TestMultiAgent_TaskValidateEmptyID(t *testing.T) {
	tk := CoordinatedTask{Description: "x"}
	assert.ErrorIs(t, tk.Validate(), ErrMultiAgentEmptyTaskID)
}

func TestMultiAgent_TaskValidateEmptyDescription(t *testing.T) {
	tk := CoordinatedTask{TaskID: "a"}
	assert.ErrorIs(t, tk.Validate(), ErrMultiAgentEmptyTaskDescription)
}

func TestMultiAgent_TaskValidateBadStatus(t *testing.T) {
	tk := CoordinatedTask{TaskID: "a", Description: "x", Status: MultiAgentTaskStatus("nope")}
	assert.ErrorIs(t, tk.Validate(), ErrMultiAgentBadTaskStatus)
}

func TestMultiAgent_TaskValidateSelfDependency(t *testing.T) {
	tk := CoordinatedTask{TaskID: "a", Description: "x", DependsOn: []string{"a"}}
	assert.ErrorIs(t, tk.Validate(), ErrMultiAgentSelfDependency)
}

func TestMultiAgent_PlanValidateBadStrategy(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentStrategy("nope"),
		Tasks: []CoordinatedTask{{TaskID: "a", Description: "x"}}}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentBadStrategy)
}

func TestMultiAgent_PlanValidateBadFailurePolicy(t *testing.T) {
	p := CoordinationPlan{
		Strategy:      MultiAgentSequential,
		FailurePolicy: MultiAgentFailurePolicy("nope"),
		Tasks:         []CoordinatedTask{{TaskID: "a", Description: "x"}},
	}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentBadFailurePolicy)
}

func TestMultiAgent_PlanValidateBadParallelism(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentParallel, MaxParallelism: -1,
		Tasks: []CoordinatedTask{{TaskID: "a", Description: "x"}}}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentBadParallelism)
}

func TestMultiAgent_PlanValidateEmptyPlan(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentSequential}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentEmptyPlan)
}

func TestMultiAgent_PlanValidateDuplicateTaskID(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentSequential, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"}, {TaskID: "a", Description: "y"},
	}}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentDuplicateTaskID)
}

func TestMultiAgent_PlanValidateUnknownDependency(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x", DependsOn: []string{"ghost"}},
	}}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentUnknownDependency)
}

func TestMultiAgent_PlanValidateCycleDetected(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x", DependsOn: []string{"b"}},
		{TaskID: "b", Description: "y", DependsOn: []string{"a"}},
	}}
	assert.ErrorIs(t, p.Validate(), ErrMultiAgentCycleDetected)
}

func TestMultiAgent_PlanValidateNoFalseDAGCycle(t *testing.T) {
	// Diamond DAG: a → b, a → c, b → d, c → d. No cycle.
	p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "1"},
		{TaskID: "b", Description: "2", DependsOn: []string{"a"}},
		{TaskID: "c", Description: "3", DependsOn: []string{"a"}},
		{TaskID: "d", Description: "4", DependsOn: []string{"b", "c"}},
	}}
	assert.NoError(t, p.Validate())
}

func TestMultiAgent_NewCoordinatorRejectsBadPlan(t *testing.T) {
	_, err := NewMultiAgentCoordinator(CoordinationPlan{})
	assert.Error(t, err)
}

func TestMultiAgent_NextBatchSequentialFirstTask(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentSequential, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x", Status: MultiAgentTaskPending},
		{TaskID: "b", Description: "y", Status: MultiAgentTaskPending},
	}}
	c, _ := NewMultiAgentCoordinator(p)
	next, err := c.NextBatch(CoordinationState{Tasks: p.Tasks})
	require.NoError(t, err)
	require.Equal(t, 1, len(next))
	assert.Equal(t, "a", next[0])
}

func TestMultiAgent_NextBatchSequentialReturnsNothingWhileRunning(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentSequential, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"},
		{TaskID: "b", Description: "y"},
	}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x", Status: MultiAgentTaskRunning},
		{TaskID: "b", Description: "y", Status: MultiAgentTaskPending},
	}}
	next, _ := c.NextBatch(state)
	assert.Empty(t, next)
}

func TestMultiAgent_NextBatchParallelAll(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"},
		{TaskID: "b", Description: "y"},
		{TaskID: "c", Description: "z"},
	}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskPending},
		{TaskID: "b", Status: MultiAgentTaskPending},
		{TaskID: "c", Status: MultiAgentTaskPending},
	}}
	next, _ := c.NextBatch(state)
	assert.Equal(t, []string{"a", "b", "c"}, next)
}

func TestMultiAgent_NextBatchParallelRespectsMaxParallelism(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentParallel, MaxParallelism: 2,
		Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x"},
			{TaskID: "b", Description: "y"},
			{TaskID: "c", Description: "z"},
		}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskPending},
		{TaskID: "b", Status: MultiAgentTaskPending},
		{TaskID: "c", Status: MultiAgentTaskPending},
	}}
	next, _ := c.NextBatch(state)
	assert.Equal(t, 2, len(next))
}

func TestMultiAgent_NextBatchParallelBudgetAccountsRunning(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentParallel, MaxParallelism: 2,
		Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x"},
			{TaskID: "b", Description: "y"},
			{TaskID: "c", Description: "z"},
		}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskRunning},
		{TaskID: "b", Status: MultiAgentTaskPending},
		{TaskID: "c", Status: MultiAgentTaskPending},
	}}
	next, _ := c.NextBatch(state)
	// 1 running, max=2, so budget=1.
	assert.Equal(t, 1, len(next))
}

func TestMultiAgent_NextBatchDAGRespectsDeps(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"},
		{TaskID: "b", Description: "y", DependsOn: []string{"a"}},
	}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", DependsOn: []string{}, Status: MultiAgentTaskPending},
		{TaskID: "b", DependsOn: []string{"a"}, Status: MultiAgentTaskPending},
	}}
	next, _ := c.NextBatch(state)
	// Only "a" is eligible — b waits for a.
	assert.Equal(t, []string{"a"}, next)
}

func TestMultiAgent_NextBatchDAGUnblocksOnDepCompletion(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentDAG, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"},
		{TaskID: "b", Description: "y", DependsOn: []string{"a"}},
	}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", DependsOn: []string{}, Status: MultiAgentTaskCompleted},
		{TaskID: "b", DependsOn: []string{"a"}, Status: MultiAgentTaskPending},
	}}
	next, _ := c.NextBatch(state)
	assert.Equal(t, []string{"b"}, next)
}

func TestMultiAgent_NextBatchEmptyWhenAllTerminal(t *testing.T) {
	p := CoordinationPlan{Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"},
	}}
	c, _ := NewMultiAgentCoordinator(p)
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskCompleted},
	}}
	next, _ := c.NextBatch(state)
	assert.Empty(t, next)
}

func TestMultiAgent_NextBatchAbortOnFailureReturnsEmpty(t *testing.T) {
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
}

func TestMultiAgent_NextBatchContinueOnFailureSpawnsOthers(t *testing.T) {
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
}

func TestMultiAgent_IsTerminalStateTrueWhenAllDone(t *testing.T) {
	c, _ := NewMultiAgentCoordinator(CoordinationPlan{
		Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{{TaskID: "a", Description: "x"}},
	})
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskCompleted},
	}}
	assert.True(t, c.IsTerminalState(state))
}

func TestMultiAgent_IsTerminalStateFalseWhileRunning(t *testing.T) {
	c, _ := NewMultiAgentCoordinator(CoordinationPlan{
		Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{{TaskID: "a", Description: "x"}},
	})
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskRunning},
	}}
	assert.False(t, c.IsTerminalState(state))
}

func TestMultiAgent_IsTerminalStateAcceptsFailedAndSkipped(t *testing.T) {
	c, _ := NewMultiAgentCoordinator(CoordinationPlan{
		Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x"}, {TaskID: "b", Description: "y"},
		},
	})
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskFailed},
		{TaskID: "b", Status: MultiAgentTaskSkipped},
	}}
	assert.True(t, c.IsTerminalState(state))
}

func TestMultiAgent_SummariseStateCountsPerStatus(t *testing.T) {
	c, _ := NewMultiAgentCoordinator(CoordinationPlan{
		Strategy: MultiAgentParallel, Tasks: []CoordinatedTask{
			{TaskID: "a", Description: "x"}, {TaskID: "b", Description: "y"},
			{TaskID: "c", Description: "z"},
		},
	})
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Status: MultiAgentTaskCompleted},
		{TaskID: "b", Status: MultiAgentTaskCompleted},
		{TaskID: "c", Status: MultiAgentTaskFailed},
	}}
	summary := c.SummariseState(state)
	assert.Equal(t, 2, summary[MultiAgentTaskCompleted])
	assert.Equal(t, 1, summary[MultiAgentTaskFailed])
}

func TestMultiAgent_PlanReturnsCopy(t *testing.T) {
	original := CoordinationPlan{
		Strategy: MultiAgentSequential,
		Tasks:    []CoordinatedTask{{TaskID: "a", Description: "x"}},
	}
	c, _ := NewMultiAgentCoordinator(original)
	got := c.Plan()
	got.Tasks[0].TaskID = "tampered"
	got2 := c.Plan()
	assert.Equal(t, "a", got2.Tasks[0].TaskID)
}

func TestMultiAgent_SetClockNilIsNoop(t *testing.T) {
	c, _ := NewMultiAgentCoordinator(CoordinationPlan{
		Strategy: MultiAgentSequential,
		Tasks:    []CoordinatedTask{{TaskID: "a", Description: "x"}},
	})
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	c.SetClock(func() time.Time { return stamp })
	c.SetClock(nil)
	c.mu.RLock()
	got := c.now()
	c.mu.RUnlock()
	assert.Equal(t, stamp, got)
}

func TestMultiAgent_FindTaskByID(t *testing.T) {
	state := CoordinationState{Tasks: []CoordinatedTask{
		{TaskID: "a", Description: "x"},
		{TaskID: "b", Description: "y"},
	}}
	tk, ok := state.FindTask("b")
	require.True(t, ok)
	assert.Equal(t, "y", tk.Description)
	_, notOk := state.FindTask("nope")
	assert.False(t, notOk)
}
