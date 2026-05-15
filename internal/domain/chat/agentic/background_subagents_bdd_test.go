package agentic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_BackgroundSubagents(t *testing.T) {
	t.Run("Scenario_ParentEnqueuesAsyncResearchAndKeepsReasoning", func(t *testing.T) {
		// Given a parent run wants research done without blocking,
		// When it enqueues a background subagent job,
		// Then the job lands in queued state and the parent may proceed.
		r := NewBackgroundSubagentRegistry()
		j := validBackgroundJob()
		require.NoError(t, r.Enqueue(j))
		got, ok := r.Lookup(j.JobID)
		require.True(t, ok)
		assert.Equal(t, BackgroundSubagentStateQueued, got.State)
	})

	t.Run("Scenario_SchedulerStartsThenJobReportsSuccess", func(t *testing.T) {
		// Given a scheduler picks up the queued job and starts it,
		// When it completes successfully,
		// Then state transitions queued → running → succeeded and the
		// result summary is recorded.
		r := NewBackgroundSubagentRegistry()
		j := validBackgroundJob()
		require.NoError(t, r.Enqueue(j))
		require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
		require.NoError(t, r.Succeed(j.JobID, j.EnqueuedAt.Add(2*time.Minute), "8 citations gathered"))
		got, _ := r.Lookup(j.JobID)
		assert.Equal(t, BackgroundSubagentStateSucceeded, got.State)
		assert.Equal(t, "8 citations gathered", got.ResultSummary)
	})

	t.Run("Scenario_ParentCancelsQueuedJobBeforeStart", func(t *testing.T) {
		// Given the parent abandons interest before the scheduler picked it up,
		// When it cancels the queued job,
		// Then the state becomes cancelled and no work was wasted.
		r := NewBackgroundSubagentRegistry()
		j := validBackgroundJob()
		require.NoError(t, r.Enqueue(j))
		require.NoError(t, r.Cancel(j.JobID, j.EnqueuedAt.Add(time.Second)))
		got, _ := r.Lookup(j.JobID)
		assert.Equal(t, BackgroundSubagentStateCancelled, got.State)
	})

	t.Run("Scenario_ParentCancelsRunningJobMidExecution", func(t *testing.T) {
		// Given a long-running subagent is in flight,
		// When the parent cancels,
		// Then the state becomes cancelled (runner must observe and stop).
		r := NewBackgroundSubagentRegistry()
		j := validBackgroundJob()
		require.NoError(t, r.Enqueue(j))
		require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
		require.NoError(t, r.Cancel(j.JobID, j.EnqueuedAt.Add(time.Minute)))
		got, _ := r.Lookup(j.JobID)
		assert.Equal(t, BackgroundSubagentStateCancelled, got.State)
	})

	t.Run("Scenario_TimeoutDetectorMarksRunningJobAsTimedOut", func(t *testing.T) {
		// Given a watchdog observes the job ran past its budget,
		// When it calls TimeOut,
		// Then state becomes timed_out with a stable error message.
		r := NewBackgroundSubagentRegistry()
		j := validBackgroundJob()
		require.NoError(t, r.Enqueue(j))
		require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
		require.NoError(t, r.TimeOut(j.JobID, j.EnqueuedAt.Add(time.Hour)))
		got, _ := r.Lookup(j.JobID)
		assert.Equal(t, BackgroundSubagentStateTimedOut, got.State)
		assert.Equal(t, "timed out", got.ErrorMsg)
	})

	t.Run("Scenario_TerminalStateIsImmutable", func(t *testing.T) {
		// Given accidental late completions must not overwrite history,
		// When a job is already succeeded and another transition is attempted,
		// Then it is rejected with ErrBackgroundSubagentBadTransition.
		r := NewBackgroundSubagentRegistry()
		j := validBackgroundJob()
		require.NoError(t, r.Enqueue(j))
		require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
		require.NoError(t, r.Succeed(j.JobID, j.EnqueuedAt.Add(time.Minute), "ok"))
		err := r.Fail(j.JobID, j.EnqueuedAt.Add(time.Hour), "late report")
		assert.ErrorIs(t, err, ErrBackgroundSubagentBadTransition)
	})

	t.Run("Scenario_ParentCanAwaitByPollingPendingCount", func(t *testing.T) {
		// Given the parent wants to know when ALL background jobs are done,
		// When it inspects PendingJobsForParent,
		// Then count drops as jobs finish and reaches 0 when all are terminal.
		r := NewBackgroundSubagentRegistry()
		parent := uuid.New()
		j1 := validBackgroundJob()
		j1.ParentRunID = parent
		j2 := validBackgroundJob()
		j2.ParentRunID = parent
		require.NoError(t, r.Enqueue(j1))
		require.NoError(t, r.Enqueue(j2))
		assert.Equal(t, 2, r.PendingJobsForParent(parent))
		require.NoError(t, r.Start(j1.JobID, j1.EnqueuedAt.Add(time.Second)))
		require.NoError(t, r.Succeed(j1.JobID, j1.EnqueuedAt.Add(time.Minute), "ok"))
		assert.Equal(t, 1, r.PendingJobsForParent(parent))
		require.NoError(t, r.Cancel(j2.JobID, j2.EnqueuedAt.Add(time.Second)))
		assert.Equal(t, 0, r.PendingJobsForParent(parent))
	})

	t.Run("Scenario_ListByParentScopesPerRun", func(t *testing.T) {
		// Given two parent runs each spawn their own subagents,
		// When admin lists by one parent,
		// Then only that parent's jobs appear.
		r := NewBackgroundSubagentRegistry()
		parentA := uuid.New()
		parentB := uuid.New()
		j1 := validBackgroundJob()
		j1.ParentRunID = parentA
		j2 := validBackgroundJob()
		j2.ParentRunID = parentB
		require.NoError(t, r.Enqueue(j1))
		require.NoError(t, r.Enqueue(j2))
		assert.Equal(t, 1, len(r.ListByParent(parentA)))
		assert.Equal(t, 1, len(r.ListByParent(parentB)))
	})

	t.Run("Scenario_SchedulerCanDrainQueuedJobs", func(t *testing.T) {
		// Given a scheduler polls for queued jobs,
		// When ListByState(queued) is called,
		// Then it returns only queued jobs sorted by enqueue time.
		r := NewBackgroundSubagentRegistry()
		j1 := validBackgroundJob()
		j2 := validBackgroundJob()
		require.NoError(t, r.Enqueue(j1))
		require.NoError(t, r.Enqueue(j2))
		require.NoError(t, r.Start(j2.JobID, j2.EnqueuedAt.Add(time.Second)))
		got := r.ListByState(BackgroundSubagentStateQueued)
		require.Equal(t, 1, len(got))
		assert.Equal(t, j1.JobID, got[0].JobID)
	})

	t.Run("Scenario_BadSlugSubagentRejectedAtEnqueue", func(t *testing.T) {
		// Given subagent slug must be kebab-case (routability),
		// When a malformed slug is supplied,
		// Then enqueue rejects with ErrBackgroundSubagentBadSlug.
		r := NewBackgroundSubagentRegistry()
		bad := validBackgroundJob()
		bad.SubagentSlug = "Not_A_Slug"
		err := r.Enqueue(bad)
		assert.ErrorIs(t, err, ErrBackgroundSubagentBadSlug)
	})
}
