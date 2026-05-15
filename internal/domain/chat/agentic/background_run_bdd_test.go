package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FUTURE-003 — Proactive/background agent loop BDD.
//
// PDF arXiv:2604.14228v1 §12 (agents that initiate actions on their own,
// not just react to user prompts).

func TestBDD_BackgroundAgentRun(t *testing.T) {

	t.Run("Scenario_AgentSchedulesMorningHealthCheck", func(t *testing.T) {
		// Given an admin scheduled a morning health check via cron,
		store := NewInMemoryBackgroundRunStore()
		r, err := store.Schedule(context.Background(), BackgroundAgentRun{
			TenantID: "acme", AgentID: "ops-bot",
			Trigger:        BackgroundTriggerScheduleCron,
			TriggerPayload: "0 9 * * *", // 9am daily
			Description:    "Morning system health check",
			ScheduledAt:    time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		assert.Equal(t, BackgroundRunScheduled, r.Status)
		assert.Equal(t, BackgroundTriggerScheduleCron, r.Trigger)
	})

	t.Run("Scenario_FourTriggerKindsCoverProactiveScenarios", func(t *testing.T) {
		// Given background agents wake from 4 distinct sources,
		expected := map[string]bool{
			"schedule_cron": true, "event_arrived": true,
			"threshold_crossed": true, "absence_timeout": true,
		}
		for _, tr := range AllBackgroundTriggers() {
			assert.True(t, expected[string(tr)])
		}
	})

	t.Run("Scenario_WorkerFetchesDueRunsForExecution", func(t *testing.T) {
		// Given a worker polls FetchDue every minute,
		store := NewInMemoryBackgroundRunStore()
		past := validBgRun()
		past.ScheduledAt = time.Now().Add(-time.Hour)
		pastSaved, _ := store.Schedule(context.Background(), past)

		future := validBgRun()
		future.ScheduledAt = time.Now().Add(time.Hour)
		_, _ = store.Schedule(context.Background(), future)

		due, err := store.FetchDue(context.Background(), "t", time.Now(), 10)
		require.NoError(t, err)
		require.Len(t, due, 1)
		assert.Equal(t, pastSaved.ID, due[0].ID,
			"only past-scheduled runs are due")
	})

	t.Run("Scenario_FetchDueExcludesAlreadyRunningRuns", func(t *testing.T) {
		// Given idempotency: same run shouldn't be picked twice,
		store := NewInMemoryBackgroundRunStore()
		past := validBgRun()
		past.ScheduledAt = time.Now().Add(-time.Hour)
		saved, _ := store.Schedule(context.Background(), past)
		require.NoError(t, store.MarkRunning(context.Background(), saved.ID))

		due, _ := store.FetchDue(context.Background(), "t", time.Now(), 10)
		assert.Empty(t, due, "running runs are claimed — not refetched")
	})

	t.Run("Scenario_TransitionsFollowScheduledRunningTerminalLifecycle", func(t *testing.T) {
		// Given the lifecycle is scheduled → running → completed/failed,
		store := NewInMemoryBackgroundRunStore()
		saved, _ := store.Schedule(context.Background(), validBgRun())
		require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
		require.NoError(t, store.MarkCompleted(context.Background(), saved.ID, "ok"))
		r, _ := store.FindByID(context.Background(), saved.ID)
		assert.Equal(t, BackgroundRunCompleted, r.Status)
	})

	t.Run("Scenario_DoubleCompleteIsRejectedAuditGuarantee", func(t *testing.T) {
		// Given audit requires once-terminal-stays-terminal,
		store := NewInMemoryBackgroundRunStore()
		saved, _ := store.Schedule(context.Background(), validBgRun())
		require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
		require.NoError(t, store.MarkCompleted(context.Background(), saved.ID, "first"))
		err := store.MarkCompleted(context.Background(), saved.ID, "second")
		assert.Error(t, err)
	})

	t.Run("Scenario_CancelWorksForScheduledAndRunningButNotTerminal", func(t *testing.T) {
		store := NewInMemoryBackgroundRunStore()

		// Cancel scheduled.
		s1, _ := store.Schedule(context.Background(), validBgRun())
		assert.NoError(t, store.Cancel(context.Background(), s1.ID))

		// Cancel running.
		s2, _ := store.Schedule(context.Background(), validBgRun())
		require.NoError(t, store.MarkRunning(context.Background(), s2.ID))
		assert.NoError(t, store.Cancel(context.Background(), s2.ID))

		// Cancel terminal → reject.
		s3, _ := store.Schedule(context.Background(), validBgRun())
		require.NoError(t, store.MarkRunning(context.Background(), s3.ID))
		require.NoError(t, store.MarkCompleted(context.Background(), s3.ID, "ok"))
		assert.Error(t, store.Cancel(context.Background(), s3.ID))
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantFetch", func(t *testing.T) {
		store := NewInMemoryBackgroundRunStore()
		past := validBgRun()
		past.ScheduledAt = time.Now().Add(-time.Hour)
		_, _ = store.Schedule(context.Background(), past)

		due, _ := store.FetchDue(context.Background(), "different-tenant", time.Now(), 0)
		assert.Empty(t, due)
	})

	t.Run("Scenario_RunStatusEnumIsBoundedToSixStates", func(t *testing.T) {
		expected := map[string]bool{
			"scheduled": true, "running": true, "completed": true,
			"failed": true, "cancelled": true, "skipped": true,
		}
		for _, s := range AllBackgroundRunStatuses() {
			assert.True(t, expected[string(s)])
		}
		assert.Equal(t, 6, len(AllBackgroundRunStatuses()))
	})

	t.Run("Scenario_CompletedRunStoresResultForAuditTrail", func(t *testing.T) {
		store := NewInMemoryBackgroundRunStore()
		saved, _ := store.Schedule(context.Background(), validBgRun())
		require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
		require.NoError(t, store.MarkCompleted(context.Background(), saved.ID,
			"Health check OK: 12 services healthy, 0 alerts"))
		r, _ := store.FindByID(context.Background(), saved.ID)
		assert.Contains(t, r.Result, "Health check OK")
	})

	t.Run("Scenario_FailedRunStoresErrorMessage", func(t *testing.T) {
		store := NewInMemoryBackgroundRunStore()
		saved, _ := store.Schedule(context.Background(), validBgRun())
		require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
		require.NoError(t, store.MarkFailed(context.Background(), saved.ID,
			"Datadog API rate limit hit"))
		r, _ := store.FindByID(context.Background(), saved.ID)
		assert.Equal(t, BackgroundRunFailed, r.Status)
		assert.Contains(t, r.ErrorMessage, "rate limit")
	})

	t.Run("Scenario_HistogramByStatusEnablesDashboard", func(t *testing.T) {
		// Given oncall dashboard shows "X scheduled / Y running / Z failed",
		store := NewInMemoryBackgroundRunStore()
		s1, _ := store.Schedule(context.Background(), validBgRun())
		_ = store.MarkRunning(context.Background(), s1.ID)
		_ = store.MarkCompleted(context.Background(), s1.ID, "ok")

		s2, _ := store.Schedule(context.Background(), validBgRun())
		_ = store.MarkRunning(context.Background(), s2.ID)
		_ = store.MarkFailed(context.Background(), s2.ID, "fail")

		_, _ = store.Schedule(context.Background(), validBgRun()) // still scheduled

		hist, _ := store.CountByStatus(context.Background(), "t")
		assert.Equal(t, 1, hist[BackgroundRunCompleted])
		assert.Equal(t, 1, hist[BackgroundRunFailed])
		assert.Equal(t, 1, hist[BackgroundRunScheduled])
		assert.Equal(t, 0, hist[BackgroundRunCancelled],
			"stable axes — unused statuses show 0")
	})

	t.Run("Scenario_DescriptionAndPayloadAreBoundedToPreventLogSpam", func(t *testing.T) {
		// Given LLM-generated descriptions / payloads may be long,
		store := NewInMemoryBackgroundRunStore()
		r := validBgRun()
		r.Description = ""
		for i := 0; i < 1000; i++ {
			r.Description += "x"
		}
		saved, _ := store.Schedule(context.Background(), r)
		assert.Equal(t, 300, len(saved.Description))
	})

	t.Run("Scenario_ConcurrentScheduleProducesAllRunsNoLostWrites", func(t *testing.T) {
		// Given multiple workers can schedule runs simultaneously,
		store := NewInMemoryBackgroundRunStore()
		const N = 50
		var wg = make(chan struct{}, N)
		for i := 0; i < N; i++ {
			go func() {
				defer func() { wg <- struct{}{} }()
				_, _ = store.Schedule(context.Background(), validBgRun())
			}()
		}
		for i := 0; i < N; i++ {
			<-wg
		}
		hist, _ := store.CountByStatus(context.Background(), "t")
		assert.Equal(t, N, hist[BackgroundRunScheduled])
	})
}
