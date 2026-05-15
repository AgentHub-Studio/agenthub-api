package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FUTURE-004 — Long-horizon task support BDD.
//
// PDF arXiv:2604.14228v1 §12 (tasks spanning days/weeks across many runs).

func TestBDD_LongHorizonTask(t *testing.T) {

	t.Run("Scenario_30DayCustomerMonitoringTaskWithWeeklyPhases", func(t *testing.T) {
		// Given a 30-day customer monitoring engagement,
		store := NewInMemoryLongHorizonTaskStore()
		tk, err := store.Create(context.Background(), LongHorizonTask{
			TenantID: "acme", AgentID: "support-bot",
			Title: "Monitor enterprise customer for 30 days",
			Description: "Daily check-ins, weekly summaries, escalation if NPS drops",
			Phases: []LongHorizonPhase{
				{ID: "week-1", Name: "Week 1: Baseline", Goal: "Establish baseline NPS"},
				{ID: "week-2", Name: "Week 2: Trends", DependsOn: []string{"week-1"}},
				{ID: "week-3", Name: "Week 3: Optimization", DependsOn: []string{"week-2"}},
				{ID: "week-4", Name: "Week 4: Renewal Prep", DependsOn: []string{"week-3"}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, LongHorizonPlanning, tk.Status)
		assert.Len(t, tk.Phases, 4)
	})

	t.Run("Scenario_PhasesEnforceDependencyOrdering", func(t *testing.T) {
		// Given phase-2 depends on phase-1,
		store := NewInMemoryLongHorizonTaskStore()
		saved, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.StartTask(context.Background(), saved.ID))

		// When agent tries to start phase-2 before phase-1 completed,
		// Then it errors — dependency contract enforced.
		err := store.StartPhase(context.Background(), saved.ID, "phase-2")
		assert.Error(t, err)
	})

	t.Run("Scenario_DependencyMetUnlocksDependentPhase", func(t *testing.T) {
		// Given phase-1 was completed,
		store := NewInMemoryLongHorizonTaskStore()
		saved, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.StartTask(context.Background(), saved.ID))
		require.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-1"))
		require.NoError(t, store.CompletePhase(context.Background(), saved.ID, "phase-1"))

		// When phase-2 starts,
		// Then no error — dependency was met.
		assert.NoError(t, store.StartPhase(context.Background(), saved.ID, "phase-2"))
	})

	t.Run("Scenario_TaskCanBePausedAndResumed", func(t *testing.T) {
		// Given a long-horizon task is in progress,
		store := NewInMemoryLongHorizonTaskStore()
		saved, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.StartTask(context.Background(), saved.ID))

		// When pause + resume cycle,
		require.NoError(t, store.Pause(context.Background(), saved.ID))
		paused, _ := store.FindByID(context.Background(), saved.ID)
		assert.Equal(t, LongHorizonPaused, paused.Status)

		require.NoError(t, store.Resume(context.Background(), saved.ID))
		resumed, _ := store.FindByID(context.Background(), saved.ID)
		assert.Equal(t, LongHorizonInProgress, resumed.Status)
		assert.True(t, resumed.PausedAt.IsZero(), "PausedAt cleared")
	})

	t.Run("Scenario_MilestonesTrackIncrementalProgress", func(t *testing.T) {
		// Given the agent records milestones throughout the task,
		store := NewInMemoryLongHorizonTaskStore()
		saved, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.RecordMilestone(context.Background(), saved.ID,
			ProgressMilestone{Description: "Customer agreed to weekly cadence"}))
		require.NoError(t, store.RecordMilestone(context.Background(), saved.ID,
			ProgressMilestone{Description: "First check-in completed"}))

		got, _ := store.FindByID(context.Background(), saved.ID)
		assert.Len(t, got.Milestones, 2)
	})

	t.Run("Scenario_CompleteRequiresAllPhasesTerminal", func(t *testing.T) {
		// Given some phases are still pending,
		store := NewInMemoryLongHorizonTaskStore()
		saved, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.StartTask(context.Background(), saved.ID))

		// When agent tries to mark task complete,
		// Then error — at least one phase isn't terminal.
		err := store.Complete(context.Background(), saved.ID)
		assert.Error(t, err)
	})

	t.Run("Scenario_AbandonAllowedFromAnyNonTerminal", func(t *testing.T) {
		// Given the user wants to abandon the task,
		// Then abandon works from planning, in_progress, or paused
		//      — but NOT from completed or already-abandoned.
		store := NewInMemoryLongHorizonTaskStore()

		fromPlanning, _ := store.Create(context.Background(), validLongHorizonTask())
		assert.NoError(t, store.Abandon(context.Background(), fromPlanning.ID))

		fromProgress, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.StartTask(context.Background(), fromProgress.ID))
		assert.NoError(t, store.Abandon(context.Background(), fromProgress.ID))

		alreadyAbandoned := fromPlanning
		err := store.Abandon(context.Background(), alreadyAbandoned.ID)
		assert.Error(t, err, "cannot abandon already-terminal task")
	})

	t.Run("Scenario_FivePhaseStatusesCoverLifecycle", func(t *testing.T) {
		// Stable wire strings.
		expected := map[string]bool{
			"pending": true, "active": true, "completed": true,
			"skipped": true, "blocked": true,
		}
		for _, s := range AllPhaseStatuses() {
			assert.True(t, expected[string(s)])
		}
	})

	t.Run("Scenario_FiveTaskStatusesCoverLifecycle", func(t *testing.T) {
		expected := map[string]bool{
			"planning": true, "in_progress": true, "paused": true,
			"completed": true, "abandoned": true,
		}
		for _, s := range AllLongHorizonStatuses() {
			assert.True(t, expected[string(s)])
		}
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantList", func(t *testing.T) {
		store := NewInMemoryLongHorizonTaskStore()
		_, _ = store.Create(context.Background(), validLongHorizonTask())
		got, _ := store.ListByAgent(context.Background(), "different-tenant", "a", "")
		assert.Empty(t, got)
	})

	t.Run("Scenario_HistogramByStatusEnablesDashboard", func(t *testing.T) {
		// Given dashboards show "X planning / Y in progress / Z completed",
		store := NewInMemoryLongHorizonTaskStore()
		_, _ = store.Create(context.Background(), validLongHorizonTask())
		t2, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.StartTask(context.Background(), t2.ID))
		require.NoError(t, store.Abandon(context.Background(), t2.ID))

		hist, _ := store.CountByStatus(context.Background(), "t")
		assert.Equal(t, 1, hist[LongHorizonPlanning])
		assert.Equal(t, 1, hist[LongHorizonAbandoned])
		assert.Equal(t, 0, hist[LongHorizonCompleted], "stable axes")
	})

	t.Run("Scenario_DuplicatePhaseIDsRejected", func(t *testing.T) {
		store := NewInMemoryLongHorizonTaskStore()
		tk := validLongHorizonTask()
		tk.Phases = []LongHorizonPhase{
			{ID: "p", Name: "first"},
			{ID: "p", Name: "duplicate"},
		}
		_, err := store.Create(context.Background(), tk)
		assert.Error(t, err, "duplicate phase IDs would break dependency resolution")
	})

	t.Run("Scenario_DependencyOnUnknownPhaseRejected", func(t *testing.T) {
		store := NewInMemoryLongHorizonTaskStore()
		tk := validLongHorizonTask()
		tk.Phases = []LongHorizonPhase{
			{ID: "p1", Name: "first", DependsOn: []string{"ghost"}},
		}
		_, err := store.Create(context.Background(), tk)
		assert.Error(t, err, "dangling dependency caught at create time")
	})

	t.Run("Scenario_TerminalTaskRejectsMilestones", func(t *testing.T) {
		// Given audit requires terminal-tasks-stay-terminal,
		store := NewInMemoryLongHorizonTaskStore()
		saved, _ := store.Create(context.Background(), validLongHorizonTask())
		require.NoError(t, store.Abandon(context.Background(), saved.ID))
		err := store.RecordMilestone(context.Background(), saved.ID,
			ProgressMilestone{Description: "x"})
		assert.Error(t, err)
	})

	t.Run("Scenario_ListByAgentNewestFirstForRecencyDashboard", func(t *testing.T) {
		// Given dashboards show most recent tasks first,
		store := NewInMemoryLongHorizonTaskStore()
		for i := 0; i < 3; i++ {
			_, _ = store.Create(context.Background(), validLongHorizonTask())
		}
		got, _ := store.ListByAgent(context.Background(), "t", "a", "")
		require.Len(t, got, 3)
		for i := 1; i < len(got); i++ {
			assert.True(t, got[i-1].CreatedAt.After(got[i].CreatedAt) ||
				got[i-1].CreatedAt.Equal(got[i].CreatedAt),
				"newest first")
		}
	})
}
