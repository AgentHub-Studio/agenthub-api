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

func validBgRun() BackgroundAgentRun {
	return BackgroundAgentRun{
		TenantID: "t", AgentID: "a",
		Trigger:        BackgroundTriggerScheduleCron,
		TriggerPayload: "0 9 * * *",
		Description:    "Morning health check",
		ScheduledAt:    time.Now(),
	}
}

func TestBgRun_TriggerEnumIsBounded(t *testing.T) {
	for _, tr := range AllBackgroundTriggers() {
		assert.True(t, IsValidBackgroundTrigger(tr))
	}
	assert.False(t, IsValidBackgroundTrigger(BackgroundTrigger("manual")))
}

func TestBgRun_AllTriggersCount(t *testing.T) {
	// 4 triggers: schedule_cron / event_arrived / threshold_crossed / absence_timeout.
	assert.Equal(t, 4, len(AllBackgroundTriggers()))
}

func TestBgRun_StatusEnumIsBounded(t *testing.T) {
	for _, s := range AllBackgroundRunStatuses() {
		assert.True(t, IsValidBackgroundRunStatus(s))
	}
	assert.False(t, IsValidBackgroundRunStatus(BackgroundRunStatus("paused")))
}

func TestBgRun_AllStatusesCount(t *testing.T) {
	// 6 statuses.
	assert.Equal(t, 6, len(AllBackgroundRunStatuses()))
}

func TestBgRun_TerminalStatusCheck(t *testing.T) {
	terminal := []BackgroundRunStatus{
		BackgroundRunCompleted, BackgroundRunFailed,
		BackgroundRunCancelled, BackgroundRunSkipped,
	}
	for _, s := range terminal {
		assert.True(t, IsTerminalBackgroundRunStatus(s))
	}
	assert.False(t, IsTerminalBackgroundRunStatus(BackgroundRunScheduled))
	assert.False(t, IsTerminalBackgroundRunStatus(BackgroundRunRunning))
}

func TestBgRun_Schedule_AssignsIDAndStatus(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	r, err := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, r.ID)
	assert.Equal(t, BackgroundRunScheduled, r.Status)
}

func TestBgRun_Schedule_RejectsRequiredFields(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	for _, missing := range []string{"tenant", "agent", "trigger", "description", "scheduledAt"} {
		r := validBgRun()
		switch missing {
		case "tenant":
			r.TenantID = ""
		case "agent":
			r.AgentID = ""
		case "trigger":
			r.Trigger = ""
		case "description":
			r.Description = ""
		case "scheduledAt":
			r.ScheduledAt = time.Time{}
		}
		_, err := store.Schedule(context.Background(), r)
		assert.Error(t, err, "missing %s must error", missing)
	}
}

func TestBgRun_Schedule_RejectsInvalidTrigger(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	r := validBgRun()
	r.Trigger = BackgroundTrigger("manual")
	_, err := store.Schedule(context.Background(), r)
	assert.True(t, errors.Is(err, ErrInvalidBackgroundTrigger))
}

func TestBgRun_Schedule_TruncatesLongFields(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	r := validBgRun()
	r.Description = strings.Repeat("x", 500)
	r.TriggerPayload = strings.Repeat("y", 2000)
	saved, _ := store.Schedule(context.Background(), r)
	assert.Equal(t, 300, len(saved.Description))
	assert.Equal(t, 1000, len(saved.TriggerPayload))
}

func TestBgRun_FetchDue_ReturnsScheduledBeforeNow(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()

	// Past + future runs.
	past := validBgRun()
	past.ScheduledAt = time.Now().Add(-time.Hour)
	pastSaved, _ := store.Schedule(context.Background(), past)

	future := validBgRun()
	future.ScheduledAt = time.Now().Add(time.Hour)
	_, _ = store.Schedule(context.Background(), future)

	due, err := store.FetchDue(context.Background(), "t", time.Now(), 0)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, pastSaved.ID, due[0].ID)
}

func TestBgRun_FetchDue_RespectsLimit(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	for i := 0; i < 5; i++ {
		r := validBgRun()
		r.ScheduledAt = time.Now().Add(-time.Duration(i) * time.Minute)
		_, _ = store.Schedule(context.Background(), r)
	}
	due, _ := store.FetchDue(context.Background(), "t", time.Now(), 2)
	assert.Len(t, due, 2)
}

func TestBgRun_FetchDue_TenantIsolation(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	past := validBgRun()
	past.ScheduledAt = time.Now().Add(-time.Hour)
	_, _ = store.Schedule(context.Background(), past)

	due, _ := store.FetchDue(context.Background(), "different-tenant", time.Now(), 0)
	assert.Empty(t, due)
}

func TestBgRun_FetchDue_ExcludesNonScheduled(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	r := validBgRun()
	r.ScheduledAt = time.Now().Add(-time.Hour)
	saved, _ := store.Schedule(context.Background(), r)
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))

	due, _ := store.FetchDue(context.Background(), "t", time.Now(), 0)
	assert.Empty(t, due, "running runs are not 'due' anymore")
}

func TestBgRun_MarkRunning_TransitionsCorrectly(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))

	r, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, BackgroundRunRunning, r.Status)
	assert.False(t, r.ExecutedAt.IsZero())
}

func TestBgRun_MarkRunning_RejectsAlreadyRunning(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	err := store.MarkRunning(context.Background(), saved.ID)
	assert.Error(t, err, "double-mark-running rejected")
}

func TestBgRun_MarkCompleted_TransitionsAndStoresResult(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	require.NoError(t, store.MarkCompleted(context.Background(), saved.ID, "all clear"))

	r, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, BackgroundRunCompleted, r.Status)
	assert.Equal(t, "all clear", r.Result)
	assert.False(t, r.CompletedAt.IsZero())
}

func TestBgRun_MarkCompleted_RejectsNotRunning(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	err := store.MarkCompleted(context.Background(), saved.ID, "x")
	assert.Error(t, err, "cannot complete a scheduled-but-not-running run")
}

func TestBgRun_MarkCompleted_AlreadyTerminalRejects(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	require.NoError(t, store.MarkCompleted(context.Background(), saved.ID, "ok"))
	err := store.MarkCompleted(context.Background(), saved.ID, "again")
	assert.True(t, errors.Is(err, ErrBackgroundRunAlreadyTerminal))
}

func TestBgRun_MarkFailed_TransitionsAndStoresError(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	require.NoError(t, store.MarkFailed(context.Background(), saved.ID, "tool quota exceeded"))

	r, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, BackgroundRunFailed, r.Status)
	assert.Contains(t, r.ErrorMessage, "quota")
}

func TestBgRun_Cancel_TransitionsScheduledRun(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.Cancel(context.Background(), saved.ID))

	r, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, BackgroundRunCancelled, r.Status)
}

func TestBgRun_Cancel_TransitionsRunningRun(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	require.NoError(t, store.Cancel(context.Background(), saved.ID))

	r, _ := store.FindByID(context.Background(), saved.ID)
	assert.Equal(t, BackgroundRunCancelled, r.Status)
}

func TestBgRun_Cancel_AlreadyTerminalRejects(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	require.NoError(t, store.MarkCompleted(context.Background(), saved.ID, "x"))
	err := store.Cancel(context.Background(), saved.ID)
	assert.True(t, errors.Is(err, ErrBackgroundRunAlreadyTerminal))
}

func TestBgRun_CountByStatus_AlwaysAllStatuses(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())
	require.NoError(t, store.MarkRunning(context.Background(), saved.ID))
	require.NoError(t, store.MarkCompleted(context.Background(), saved.ID, "x"))

	hist, _ := store.CountByStatus(context.Background(), "t")
	for _, s := range AllBackgroundRunStatuses() {
		_, ok := hist[s]
		assert.True(t, ok)
	}
	assert.Equal(t, 1, hist[BackgroundRunCompleted])
	assert.Equal(t, 0, hist[BackgroundRunFailed])
}

func TestBgRun_FindByID_UnknownReturnsNotFound(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	_, err := store.FindByID(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrBackgroundRunNotFound))
}

func TestBgRun_ConcurrentScheduleIsSafe(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Schedule(context.Background(), validBgRun())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	hist, _ := store.CountByStatus(context.Background(), "t")
	assert.Equal(t, 50, hist[BackgroundRunScheduled])
}

func TestBgRun_ContextCancelledOperationsError(t *testing.T) {
	store := NewInMemoryBackgroundRunStore()
	saved, _ := store.Schedule(context.Background(), validBgRun())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Schedule(ctx, validBgRun())
	assert.Error(t, err)

	_, err = store.FindByID(ctx, saved.ID)
	assert.Error(t, err)

	_, err = store.FetchDue(ctx, "t", time.Now(), 0)
	assert.Error(t, err)

	err = store.MarkRunning(ctx, saved.ID)
	assert.Error(t, err)

	err = store.MarkCompleted(ctx, saved.ID, "x")
	assert.Error(t, err)

	err = store.MarkFailed(ctx, saved.ID, "x")
	assert.Error(t, err)

	err = store.Cancel(ctx, saved.ID)
	assert.Error(t, err)

	_, err = store.CountByStatus(ctx, "t")
	assert.Error(t, err)
}
