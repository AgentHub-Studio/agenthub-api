package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReport(score float64, dec EvaluationDecision) QualityReport {
	return QualityReport{
		EvaluatorName: "heuristic",
		OverallScore:  score,
		Decision:      dec,
		EvaluatedAt:   time.Now(),
	}
}

func TestStore_Save_AssignsIDAndTimestamp(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	rec, err := store.Save(context.Background(), tenantID, agentID, "run-1", newReport(0.9, EvalPass))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rec.ID, "Save must assign ID")
	assert.False(t, rec.StoredAt.IsZero(), "Save must populate StoredAt")
	assert.Equal(t, tenantID, rec.TenantID)
	assert.Equal(t, agentID, rec.AgentID)
	assert.Equal(t, "run-1", rec.RunID)
}

func TestStore_FindByRunID_ReturnsSavedRecord(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	saved, _ := store.Save(context.Background(), tenantID, agentID, "run-find", newReport(0.8, EvalPass))

	got, err := store.FindByRunID(context.Background(), tenantID, "run-find")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
	assert.Equal(t, saved.RunID, got.RunID)
}

func TestStore_FindByRunID_UnknownReturnsErrNotFound(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	_, err := store.FindByRunID(context.Background(), uuid.New(), "missing-run")
	assert.True(t, errors.Is(err, ErrQualityReportNotFound),
		"missing run must return ErrQualityReportNotFound (got %v)", err)
}

func TestStore_FindByRunID_TenantIsolation(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantA := uuid.New()
	tenantB := uuid.New()
	agentID := uuid.New()

	_, _ = store.Save(context.Background(), tenantA, agentID, "shared-runid", newReport(0.9, EvalPass))

	// Tenant B asks for the same RunID — must NOT see tenant A's row.
	_, err := store.FindByRunID(context.Background(), tenantB, "shared-runid")
	assert.True(t, errors.Is(err, ErrQualityReportNotFound),
		"tenant B must not see tenant A's report")
}

func TestStore_ListByAgent_NewestFirst(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	for i := 0; i < 3; i++ {
		_, _ = store.Save(context.Background(), tenantID, agentID,
			"r"+string(rune('a'+i)), newReport(0.9, EvalPass))
		time.Sleep(2 * time.Millisecond) // ensure distinct StoredAt
	}

	got, err := store.ListByAgent(context.Background(), tenantID, agentID, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.True(t, got[i-1].StoredAt.After(got[i].StoredAt) || got[i-1].StoredAt.Equal(got[i].StoredAt),
			"ListByAgent must return newest-first")
	}
}

func TestStore_ListByAgent_RespectsLimit(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	for i := 0; i < 5; i++ {
		_, _ = store.Save(context.Background(), tenantID, agentID, "x", newReport(0.9, EvalPass))
	}
	got, err := store.ListByAgent(context.Background(), tenantID, agentID, 2)
	require.NoError(t, err)
	assert.Len(t, got, 2, "limit=2 must return at most 2")

	gotAll, _ := store.ListByAgent(context.Background(), tenantID, agentID, 0)
	assert.Len(t, gotAll, 5, "limit=0 must return all")
}

func TestStore_ListByDecision_FiltersStrictly(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	_, _ = store.Save(context.Background(), tenantID, agentID, "p1", newReport(0.9, EvalPass))
	_, _ = store.Save(context.Background(), tenantID, agentID, "f1", newReport(0.2, EvalFail))
	_, _ = store.Save(context.Background(), tenantID, agentID, "f2", newReport(0.1, EvalFail))
	_, _ = store.Save(context.Background(), tenantID, agentID, "w1", newReport(0.5, EvalWarn))

	fails, err := store.ListByDecision(context.Background(), tenantID, EvalFail, 0)
	require.NoError(t, err)
	assert.Len(t, fails, 2)
	for _, r := range fails {
		assert.Equal(t, EvalFail, r.Report.Decision)
	}
}

func TestStore_ListSince_FiltersByCutoff(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	_, _ = store.Save(context.Background(), tenantID, agentID, "old", newReport(0.9, EvalPass))
	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)
	_, _ = store.Save(context.Background(), tenantID, agentID, "new", newReport(0.8, EvalPass))

	got, err := store.ListSince(context.Background(), tenantID, cutoff, 0)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "new", got[0].RunID)
}

func TestStore_CountByDecision_IncludesAllThreeKeys(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	_, _ = store.Save(context.Background(), tenantID, agentID, "a", newReport(0.9, EvalPass))
	_, _ = store.Save(context.Background(), tenantID, agentID, "b", newReport(0.9, EvalPass))
	_, _ = store.Save(context.Background(), tenantID, agentID, "c", newReport(0.2, EvalFail))

	hist, err := store.CountByDecision(context.Background(), tenantID, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 2, hist[EvalPass])
	assert.Equal(t, 1, hist[EvalFail])
	// EvalWarn must be present with 0 count — dashboards need stable axis.
	zero, exists := hist[EvalWarn]
	assert.True(t, exists, "EvalWarn key must exist even with 0 count")
	assert.Equal(t, 0, zero)
}

func TestStore_AverageScore_ComputedCorrectly(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	for _, score := range []float64{0.4, 0.6, 0.8} {
		_, _ = store.Save(context.Background(), tenantID, agentID, "r", newReport(score, EvalPass))
	}
	avg, ok, err := store.AverageScore(context.Background(), tenantID, time.Time{})
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, 0.6, avg, 0.0001) // (0.4+0.6+0.8)/3 = 0.6
}

func TestStore_AverageScore_EmptyReturnsFalse(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	avg, ok, err := store.AverageScore(context.Background(), uuid.New(), time.Time{})
	require.NoError(t, err)
	assert.False(t, ok, "empty window must return ok=false")
	assert.Equal(t, 0.0, avg)
}

func TestStore_TenantIsolationAcrossAllQueries(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantA := uuid.New()
	tenantB := uuid.New()
	agentID := uuid.New()

	_, _ = store.Save(context.Background(), tenantA, agentID, "a-run", newReport(0.9, EvalPass))
	_, _ = store.Save(context.Background(), tenantA, agentID, "a-run-2", newReport(0.2, EvalFail))

	listB, _ := store.ListByAgent(context.Background(), tenantB, agentID, 0)
	assert.Empty(t, listB, "tenant B must see no rows in ListByAgent")

	failsB, _ := store.ListByDecision(context.Background(), tenantB, EvalFail, 0)
	assert.Empty(t, failsB, "tenant B must see no fails")

	histB, _ := store.CountByDecision(context.Background(), tenantB, time.Time{})
	assert.Equal(t, 0, histB[EvalPass]+histB[EvalWarn]+histB[EvalFail],
		"tenant B histogram must be all zeros")

	_, okB, _ := store.AverageScore(context.Background(), tenantB, time.Time{})
	assert.False(t, okB, "tenant B AverageScore must be unavailable")
}

func TestStore_ContextCancelled_OperationsReturnError(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Save(ctx, uuid.New(), uuid.New(), "r", newReport(0.9, EvalPass))
	assert.Error(t, err)

	_, err = store.FindByRunID(ctx, uuid.New(), "r")
	assert.Error(t, err)

	_, err = store.ListByAgent(ctx, uuid.New(), uuid.New(), 0)
	assert.Error(t, err)
}

func TestStore_SavedReportIsImmutableAfterSave(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	original := newReport(0.7, EvalPass)
	original.Dimensions = map[string]float64{"length": 0.7}
	saved, _ := store.Save(context.Background(), tenantID, agentID, "imm", original)

	// Mutate the original after Save — value copy must shield the store.
	original.Decision = EvalFail
	original.OverallScore = 0.0

	got, err := store.FindByRunID(context.Background(), tenantID, "imm")
	require.NoError(t, err)
	assert.Equal(t, EvalPass, got.Report.Decision,
		"mutation of original report after Save must NOT affect stored record")
	assert.InDelta(t, 0.7, got.Report.OverallScore, 0.0001)
	assert.Equal(t, saved.ID, got.ID)
}

func TestStore_ConcurrentSaveIsSafe(t *testing.T) {
	store := NewInMemoryQualityReportStore()
	tenantID := uuid.New()
	agentID := uuid.New()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rep := newReport(0.5, EvalPass)
			_, err := store.Save(context.Background(), tenantID, agentID,
				"concurrent-"+string(rune('a'+(i%26))), rep)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	got, err := store.ListByAgent(context.Background(), tenantID, agentID, 0)
	require.NoError(t, err)
	assert.Len(t, got, 100, "100 concurrent Saves must produce 100 rows")
}
