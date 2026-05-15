package agentic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validLongHorizonTask() LongHorizonTask {
	return LongHorizonTask{
		TenantID: "t", AgentID: "a",
		Title:       "Monitor customer for 30 days",
		Description: "Daily check-ins with weekly summary",
		Phases: []LongHorizonPhase{
			{ID: "phase-1", Name: "Week 1", Goal: "baseline metrics"},
			{ID: "phase-2", Name: "Week 2", Goal: "trend analysis", DependsOn: []string{"phase-1"}},
		},
	}
}

func TestLongHorizon_StatusEnumIsBounded(t *testing.T) {
	for _, s := range AllLongHorizonStatuses() {
		assert.True(t, IsValidLongHorizonStatus(s))
	}
	assert.False(t, IsValidLongHorizonStatus(LongHorizonStatus("done")))
}

func TestLongHorizon_AllStatusesCount(t *testing.T) {
	assert.Equal(t, 5, len(AllLongHorizonStatuses()))
}

func TestLongHorizon_TerminalCheck(t *testing.T) {
	assert.True(t, IsTerminalLongHorizonStatus(LongHorizonCompleted))
	assert.True(t, IsTerminalLongHorizonStatus(LongHorizonAbandoned))
	assert.False(t, IsTerminalLongHorizonStatus(LongHorizonPlanning))
	assert.False(t, IsTerminalLongHorizonStatus(LongHorizonInProgress))
	assert.False(t, IsTerminalLongHorizonStatus(LongHorizonPaused))
}

func TestLongHorizon_PhaseStatusEnumIsBounded(t *testing.T) {
	for _, s := range AllPhaseStatuses() {
		assert.True(t, IsValidPhaseStatus(s))
	}
	assert.False(t, IsValidPhaseStatus(PhaseStatus("ongoing")))
}

func TestLongHorizon_AllPhaseStatusesCount(t *testing.T) {
	assert.Equal(t, 5, len(AllPhaseStatuses()))
}

func TestLongHorizon_Create_AssignsIDAndDefaults(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, err := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.Equal(t, LongHorizonPlanning, saved.Status)
	assert.False(t, saved.CreatedAt.IsZero())
	assert.Equal(t, PhaseStatusPending, saved.Phases[0].Status)
}

func TestLongHorizon_Create_RejectsRequiredFields(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	for _, missing := range []string{"tenant", "agent", "title", "phases"} {
		tk := validLongHorizonTask()
		switch missing {
		case "tenant":
			tk.TenantID = ""
		case "agent":
			tk.AgentID = ""
		case "title":
			tk.Title = ""
		case "phases":
			tk.Phases = nil
		}
		_, err := store.Create(context.Background(), tk)
		assert.Error(t, err, "missing %s must error", missing)
	}
}

func TestLongHorizon_Create_RejectsDuplicatePhaseIDs(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	tk := validLongHorizonTask()
	tk.Phases = []LongHorizonPhase{
		{ID: "p", Name: "first"},
		{ID: "p", Name: "duplicate"},
	}
	_, err := store.Create(context.Background(), tk)
	assert.Error(t, err)
}

func TestLongHorizon_Create_RejectsUnknownDependency(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	tk := validLongHorizonTask()
	tk.Phases = []LongHorizonPhase{
		{ID: "p1", Name: "first", DependsOn: []string{"nonexistent"}},
	}
	_, err := store.Create(context.Background(), tk)
	assert.Error(t, err)
}

func TestLongHorizon_Create_TruncatesLongFields(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	tk := validLongHorizonTask()
	tk.Title = strings.Repeat("x", 500)
	tk.Description = strings.Repeat("y", 2000)
	tk.Phases[0].Goal = strings.Repeat("z", 1000)
	saved, err := store.Create(context.Background(), tk)
	require.NoError(t, err)
	assert.Equal(t, 200, len(saved.Title))
	assert.Equal(t, 1000, len(saved.Description))
	assert.Equal(t, 500, len(saved.Phases[0].Goal))
}

func TestLongHorizon_StartTask_TransitionsCorrectly(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))

	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, LongHorizonInProgress, got.Status)
	assert.False(t, got.StartedAt.IsZero())
}

func TestLongHorizon_StartTask_RejectsDoubleStart(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	err := store.StartTask(context.Background(), saved.ID)
	assert.True(t, errors.Is(err, ErrLongHorizonInvalidTransition))
}

func TestLongHorizon_StartPhase_TransitionsCorrectly(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-1"))

	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, PhaseStatusActive, got.Phases[0].Status)
	assert.False(t, got.Phases[0].StartedAt.IsZero())
}

func TestLongHorizon_StartPhase_EnforcesDependencies(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	// Try to start phase-2 before phase-1 completed.
	err := store.StartPhase(context.Background(), saved.ID, "phase-2")
	assert.True(t, errors.Is(err, ErrPhaseDependenciesNotMet))
}

func TestLongHorizon_StartPhase_AllowsAfterDependencyCompleted(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-1"))
	require.NoError(t, store.CompletePhase(context.Background(), saved.ID, "phase-1"))
	// Now phase-2 should start.
	require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-2"))
}

func TestLongHorizon_StartPhase_UnknownReturnsErrPhaseNotFound(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	err := store.StartPhase(context.Background(), saved.ID, "nonexistent")
	assert.True(t, errors.Is(err, ErrPhaseNotFound))
}

func TestLongHorizon_CompletePhase_TransitionsCorrectly(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-1"))
	require.NoError(t, store.CompletePhase(context.Background(), saved.ID, "phase-1"))

	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, PhaseStatusCompleted, got.Phases[0].Status)
	assert.False(t, got.Phases[0].CompletedAt.IsZero())
}

func TestLongHorizon_CompletePhase_RejectsNotActive(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	err := store.CompletePhase(context.Background(), saved.ID, "phase-1")
	assert.Error(t, err, "must be active to complete")
}

func TestLongHorizon_RecordMilestone_AppendsToList(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.RecordMilestone(context.Background(), saved.ID, ProgressMilestone{
		Description: "Customer responded positively",
	}))
	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Len(t, got.Milestones, 1)
	assert.NotEqual(t, uuid.Nil, got.Milestones[0].ID, "ID auto-assigned")
	assert.False(t, got.Milestones[0].RecordedAt.IsZero())
}

func TestLongHorizon_RecordMilestone_RejectsTerminalTask(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.Abandon(context.Background(), saved.ID))
	err := store.RecordMilestone(context.Background(), saved.ID, ProgressMilestone{Description: "x"})
	assert.True(t, errors.Is(err, ErrLongHorizonAlreadyTerminal))
}

func TestLongHorizon_Pause_TransitionsCorrectly(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	require.NoError(t, store.Pause(context.Background(), saved.ID))

	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, LongHorizonPaused, got.Status)
	assert.False(t, got.PausedAt.IsZero())
}

func TestLongHorizon_Pause_RejectsNotInProgress(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	err := store.Pause(context.Background(), saved.ID)
	assert.Error(t, err, "cannot pause from planning")
}

func TestLongHorizon_Resume_TransitionsCorrectly(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	require.NoError(t, store.Pause(context.Background(), saved.ID))
	require.NoError(t, store.Resume(context.Background(), saved.ID))

	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, LongHorizonInProgress, got.Status)
	assert.True(t, got.PausedAt.IsZero(), "PausedAt cleared on resume")
}

func TestLongHorizon_Complete_RequiresAllPhasesTerminal(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	// Phases still pending — cannot complete.
	err := store.Complete(context.Background(), saved.ID)
	assert.Error(t, err)
}

func TestLongHorizon_Complete_TransitionsAfterAllPhasesDone(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), saved.ID))
	require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-1"))
	require.NoError(t, store.CompletePhase(context.Background(), saved.ID, "phase-1"))
	require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-2"))
	require.NoError(t, store.CompletePhase(context.Background(), saved.ID, "phase-2"))
	require.NoError(t, store.Complete(context.Background(), saved.ID))

	got, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, LongHorizonCompleted, got.Status)
}

func TestLongHorizon_Abandon_AllowsFromAnyNonTerminal(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()

	// Abandon from planning.
	t1, _ := store.Create(context.Background(), validLongHorizonTask())
	assert.NoError(t, store.Abandon(context.Background(), t1.ID))

	// Abandon from in_progress.
	t2, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), t2.ID))
	assert.NoError(t, store.Abandon(context.Background(), t2.ID))

	// Abandon from paused.
	t3, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), t3.ID))
	require.NoError(t, store.Pause(context.Background(), t3.ID))
	assert.NoError(t, store.Abandon(context.Background(), t3.ID))

	// Cannot abandon completed or already-abandoned.
	t4, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.Abandon(context.Background(), t4.ID))
	err := store.Abandon(context.Background(), t4.ID)
	assert.True(t, errors.Is(err, ErrLongHorizonAlreadyTerminal))
}

func TestLongHorizon_ListByAgent_FiltersAndSortsNewestFirst(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	for i := 0; i < 3; i++ {
		_, _ = store.Create(context.Background(), validLongHorizonTask())
		time.Sleep(2 * time.Millisecond)
	}
	got, err := store.ListByAgent(context.Background(), "t", "a", "")
	require.NoError(t, err)
	assert.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.True(t, got[i-1].CreatedAt.After(got[i].CreatedAt) || got[i-1].CreatedAt.Equal(got[i].CreatedAt))
	}
}

func TestLongHorizon_ListByAgent_StatusFilter(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	t1, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.Abandon(context.Background(), t1.ID))
	_, _ = store.Create(context.Background(), validLongHorizonTask()) // planning

	planning, _ := store.ListByAgent(context.Background(), "t", "a", LongHorizonPlanning)
	assert.Len(t, planning, 1)

	abandoned, _ := store.ListByAgent(context.Background(), "t", "a", LongHorizonAbandoned)
	assert.Len(t, abandoned, 1)
}

func TestLongHorizon_ListByAgent_TenantIsolation(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	_, _ = store.Create(context.Background(), validLongHorizonTask())
	got, _ := store.ListByAgent(context.Background(), "different", "a", "")
	assert.Empty(t, got)
}

func TestLongHorizon_CountByStatus_AlwaysAllStatuses(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	_, _ = store.Create(context.Background(), validLongHorizonTask())
	t2, _ := store.Create(context.Background(), validLongHorizonTask())
	require.NoError(t, store.StartTask(context.Background(), t2.ID))

	hist, _ := store.CountByStatus(context.Background(), "t")
	for _, s := range AllLongHorizonStatuses() {
		_, ok := hist[s]
		assert.True(t, ok)
	}
	assert.Equal(t, 1, hist[LongHorizonPlanning])
	assert.Equal(t, 1, hist[LongHorizonInProgress])
	assert.Equal(t, 0, hist[LongHorizonCompleted])
}

func TestLongHorizon_FindByID_UnknownReturnsNotFound(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	_, err := store.FindByID(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrLongHorizonTaskNotFound))
}

func TestLongHorizon_ConcurrentCreateIsSafe(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Create(context.Background(), validLongHorizonTask())
		}()
	}
	wg.Wait()
	got, _ := store.ListByAgent(context.Background(), "t", "a", "")
	assert.Len(t, got, 50)
}

func TestLongHorizon_ContextCancelledOperationsError(t *testing.T) {
	store := NewInMemoryLongHorizonTaskStore()
	saved, _ := store.Create(context.Background(), validLongHorizonTask())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Create(ctx, validLongHorizonTask())
	assert.Error(t, err)

	_, err = store.FindByID(ctx, saved.ID)
	assert.Error(t, err)

	_, err = store.ListByAgent(ctx, "t", "a", "")
	assert.Error(t, err)

	err = store.StartTask(ctx, saved.ID)
	assert.Error(t, err)

	err = store.StartPhase(ctx, saved.ID, "phase-1")
	assert.Error(t, err)

	err = store.CompletePhase(ctx, saved.ID, "phase-1")
	assert.Error(t, err)

	err = store.RecordMilestone(ctx, saved.ID, ProgressMilestone{Description: "x"})
	assert.Error(t, err)

	err = store.Pause(ctx, saved.ID)
	assert.Error(t, err)

	err = store.Resume(ctx, saved.ID)
	assert.Error(t, err)

	err = store.Complete(ctx, saved.ID)
	assert.Error(t, err)

	err = store.Abandon(ctx, saved.ID)
	assert.Error(t, err)

	_, err = store.CountByStatus(ctx, "t")
	assert.Error(t, err)
}
