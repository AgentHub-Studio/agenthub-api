package agentic

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBackgroundJob() BackgroundSubagentJob {
	return BackgroundSubagentJob{
		JobID:           uuid.New(),
		ParentRunID:     uuid.New(),
		OwnerTenantSlug: "acme-corp",
		SubagentSlug:    "researcher-baseline",
		State:           BackgroundSubagentStateQueued,
		EnqueuedAt:      time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC),
	}
}

func TestBackgroundSubagent_IsValidState(t *testing.T) {
	for _, s := range allBackgroundSubagentStates {
		assert.True(t, IsValidBackgroundSubagentState(s))
	}
	assert.False(t, IsValidBackgroundSubagentState(BackgroundSubagentState("nope")))
}

func TestBackgroundSubagent_AllStatesReturnsCopy(t *testing.T) {
	s := AllBackgroundSubagentStates()
	require.Equal(t, 6, len(s))
	s[0] = "tampered"
	s2 := AllBackgroundSubagentStates()
	assert.Equal(t, BackgroundSubagentStateQueued, s2[0])
}

func TestBackgroundSubagent_IsTerminalState(t *testing.T) {
	assert.False(t, IsTerminalBackgroundSubagentState(BackgroundSubagentStateQueued))
	assert.False(t, IsTerminalBackgroundSubagentState(BackgroundSubagentStateRunning))
	assert.True(t, IsTerminalBackgroundSubagentState(BackgroundSubagentStateSucceeded))
	assert.True(t, IsTerminalBackgroundSubagentState(BackgroundSubagentStateFailed))
	assert.True(t, IsTerminalBackgroundSubagentState(BackgroundSubagentStateCancelled))
	assert.True(t, IsTerminalBackgroundSubagentState(BackgroundSubagentStateTimedOut))
}

func TestBackgroundSubagent_ValidateEmptyJobID(t *testing.T) {
	j := validBackgroundJob()
	j.JobID = uuid.Nil
	assert.ErrorIs(t, j.ValidateForEnqueue(), ErrBackgroundSubagentEmptyJobID)
}

func TestBackgroundSubagent_ValidateEmptyParent(t *testing.T) {
	j := validBackgroundJob()
	j.ParentRunID = uuid.Nil
	assert.ErrorIs(t, j.ValidateForEnqueue(), ErrBackgroundSubagentEmptyParent)
}

func TestBackgroundSubagent_ValidateBadOwner(t *testing.T) {
	j := validBackgroundJob()
	j.OwnerTenantSlug = "Bad Owner"
	assert.ErrorIs(t, j.ValidateForEnqueue(), ErrBackgroundSubagentBadOwner)
}

func TestBackgroundSubagent_ValidateBadSlug(t *testing.T) {
	j := validBackgroundJob()
	j.SubagentSlug = "BAD"
	assert.ErrorIs(t, j.ValidateForEnqueue(), ErrBackgroundSubagentBadSlug)
}

func TestBackgroundSubagent_ValidateEmptyEnqueuedAt(t *testing.T) {
	j := validBackgroundJob()
	j.EnqueuedAt = time.Time{}
	assert.ErrorIs(t, j.ValidateForEnqueue(), ErrBackgroundSubagentEmptyEnqueuedAt)
}

func TestBackgroundSubagent_ValidateBadInitialState(t *testing.T) {
	j := validBackgroundJob()
	j.State = BackgroundSubagentStateRunning
	assert.ErrorIs(t, j.ValidateForEnqueue(), ErrBackgroundSubagentBadInitialState)
}

func TestBackgroundSubagent_RegistryEnqueueAndLookup(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	got, ok := r.Lookup(j.JobID)
	require.True(t, ok)
	assert.Equal(t, BackgroundSubagentStateQueued, got.State)
}

func TestBackgroundSubagent_RegistryLookupUnknown(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	_, ok := r.Lookup(uuid.New())
	assert.False(t, ok)
}

func TestBackgroundSubagent_RegistryRejectsDuplicate(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	err := r.Enqueue(j)
	assert.ErrorIs(t, err, ErrBackgroundSubagentDuplicate)
}

func TestBackgroundSubagent_RegistryRejectsBadJob(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	bad := validBackgroundJob()
	bad.JobID = uuid.Nil
	assert.ErrorIs(t, r.Enqueue(bad), ErrBackgroundSubagentEmptyJobID)
}

func TestBackgroundSubagent_StartTransition(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	startAt := j.EnqueuedAt.Add(time.Second)
	require.NoError(t, r.Start(j.JobID, startAt))
	got, _ := r.Lookup(j.JobID)
	assert.Equal(t, BackgroundSubagentStateRunning, got.State)
	assert.Equal(t, startAt, got.StartedAt)
}

func TestBackgroundSubagent_SucceedTransition(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	now := j.EnqueuedAt
	require.NoError(t, r.Start(j.JobID, now.Add(time.Second)))
	require.NoError(t, r.Succeed(j.JobID, now.Add(time.Minute), "5 docs cited"))
	got, _ := r.Lookup(j.JobID)
	assert.Equal(t, BackgroundSubagentStateSucceeded, got.State)
	assert.Equal(t, "5 docs cited", got.ResultSummary)
	assert.Equal(t, now.Add(time.Minute), got.CompletedAt)
}

func TestBackgroundSubagent_FailTransition(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
	require.NoError(t, r.Fail(j.JobID, j.EnqueuedAt.Add(time.Minute), "llm 429"))
	got, _ := r.Lookup(j.JobID)
	assert.Equal(t, BackgroundSubagentStateFailed, got.State)
	assert.Equal(t, "llm 429", got.ErrorMsg)
}

func TestBackgroundSubagent_CancelFromQueued(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	require.NoError(t, r.Cancel(j.JobID, j.EnqueuedAt.Add(time.Second)))
	got, _ := r.Lookup(j.JobID)
	assert.Equal(t, BackgroundSubagentStateCancelled, got.State)
}

func TestBackgroundSubagent_CancelFromRunning(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
	require.NoError(t, r.Cancel(j.JobID, j.EnqueuedAt.Add(time.Minute)))
	got, _ := r.Lookup(j.JobID)
	assert.Equal(t, BackgroundSubagentStateCancelled, got.State)
}

func TestBackgroundSubagent_TimeOutFromRunning(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
	require.NoError(t, r.TimeOut(j.JobID, j.EnqueuedAt.Add(time.Hour)))
	got, _ := r.Lookup(j.JobID)
	assert.Equal(t, BackgroundSubagentStateTimedOut, got.State)
	assert.Equal(t, "timed out", got.ErrorMsg)
}

func TestBackgroundSubagent_RejectInvalidTransitionQueuedToSucceeded(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	err := r.Succeed(j.JobID, j.EnqueuedAt.Add(time.Minute), "x")
	assert.ErrorIs(t, err, ErrBackgroundSubagentBadTransition)
}

func TestBackgroundSubagent_RejectTimeOutFromQueued(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	err := r.TimeOut(j.JobID, j.EnqueuedAt.Add(time.Hour))
	assert.ErrorIs(t, err, ErrBackgroundSubagentBadTransition)
}

func TestBackgroundSubagent_RejectTransitionFromTerminal(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	require.NoError(t, r.Start(j.JobID, j.EnqueuedAt.Add(time.Second)))
	require.NoError(t, r.Succeed(j.JobID, j.EnqueuedAt.Add(time.Minute), "ok"))
	err := r.Fail(j.JobID, j.EnqueuedAt.Add(time.Hour), "late")
	assert.ErrorIs(t, err, ErrBackgroundSubagentBadTransition)
}

func TestBackgroundSubagent_RejectStartUnknownJob(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	err := r.Start(uuid.New(), time.Now())
	assert.ErrorIs(t, err, ErrBackgroundSubagentNotFound)
}

func TestBackgroundSubagent_RejectZeroTimestamp(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j := validBackgroundJob()
	require.NoError(t, r.Enqueue(j))
	err := r.Start(j.JobID, time.Time{})
	assert.ErrorIs(t, err, ErrBackgroundSubagentEmptyTimestamp)
}

func TestBackgroundSubagent_ListByParentSortedByEnqueuedAt(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	parent := uuid.New()
	j1 := validBackgroundJob()
	j1.ParentRunID = parent
	j1.EnqueuedAt = time.Date(2026, 5, 12, 12, 0, 5, 0, time.UTC)
	j2 := validBackgroundJob()
	j2.ParentRunID = parent
	j2.EnqueuedAt = time.Date(2026, 5, 12, 12, 0, 1, 0, time.UTC)
	require.NoError(t, r.Enqueue(j1))
	require.NoError(t, r.Enqueue(j2))
	list := r.ListByParent(parent)
	require.Equal(t, 2, len(list))
	assert.Equal(t, j2.JobID, list[0].JobID)
}

func TestBackgroundSubagent_ListByStateFilters(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	j1 := validBackgroundJob()
	j2 := validBackgroundJob()
	require.NoError(t, r.Enqueue(j1))
	require.NoError(t, r.Enqueue(j2))
	require.NoError(t, r.Start(j2.JobID, j2.EnqueuedAt.Add(time.Second)))
	queued := r.ListByState(BackgroundSubagentStateQueued)
	running := r.ListByState(BackgroundSubagentStateRunning)
	assert.Equal(t, 1, len(queued))
	assert.Equal(t, 1, len(running))
}

func TestBackgroundSubagent_PendingJobsForParent(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	parent := uuid.New()
	j1 := validBackgroundJob()
	j1.ParentRunID = parent
	j2 := validBackgroundJob()
	j2.ParentRunID = parent
	j3 := validBackgroundJob()
	j3.ParentRunID = parent
	require.NoError(t, r.Enqueue(j1))
	require.NoError(t, r.Enqueue(j2))
	require.NoError(t, r.Enqueue(j3))
	require.NoError(t, r.Start(j2.JobID, j2.EnqueuedAt.Add(time.Second)))
	require.NoError(t, r.Succeed(j2.JobID, j2.EnqueuedAt.Add(time.Minute), "x"))
	require.NoError(t, r.Cancel(j3.JobID, j3.EnqueuedAt.Add(time.Second)))
	// j1 still queued (non-terminal); j2 succeeded; j3 cancelled.
	assert.Equal(t, 1, r.PendingJobsForParent(parent))
}

func TestBackgroundSubagent_ConcurrentEnqueueSafe(t *testing.T) {
	r := NewBackgroundSubagentRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Enqueue(validBackgroundJob())
		}()
	}
	wg.Wait()
	assert.Equal(t, 50, r.Size())
}
