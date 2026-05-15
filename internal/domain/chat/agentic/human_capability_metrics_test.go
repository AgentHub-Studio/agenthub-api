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

func validCapabilitySnapshot() HumanCapabilitySnapshot {
	now := time.Now().UTC()
	return HumanCapabilitySnapshot{
		TenantID: "t", UserID: "alice",
		Period: ReportPeriod{Start: now.Add(-30 * 24 * time.Hour), End: now},
		Metrics: map[CapabilityDimension]CapabilityMetric{
			CapabilityDomainKnowledge: {
				Dimension: CapabilityDomainKnowledge,
				Value: 0.7, TrendDelta: 0.1,
				SampleSize: 50, LastMeasuredAt: now,
			},
			CapabilityDecisionIndependence: {
				Dimension: CapabilityDecisionIndependence,
				Value: 0.6, TrendDelta: 0.08,
				SampleSize: 50, LastMeasuredAt: now,
			},
		},
	}
}

func validCapabilityChange() CapabilityChangeEvent {
	return CapabilityChangeEvent{
		TenantID: "t", UserID: "alice",
		Dimension: CapabilityDomainKnowledge,
		Delta:     0.05,
		Reason:    "User now answers questions about authentication without consulting the agent",
	}
}

func TestCapability_DimensionEnumIsBounded(t *testing.T) {
	for _, d := range AllCapabilityDimensions() {
		assert.True(t, IsValidCapabilityDimension(d))
	}
	assert.False(t, IsValidCapabilityDimension(CapabilityDimension("ego")))
}

func TestCapability_AllDimensionsCount(t *testing.T) {
	// 5 dimensions: domain_knowledge / decision_independence /
	// task_throughput / quality_output / collaboration.
	assert.Equal(t, 5, len(AllCapabilityDimensions()))
}

func TestCapability_TrendEnumIsBounded(t *testing.T) {
	for _, tr := range AllCapabilityTrends() {
		assert.True(t, IsValidCapabilityTrend(tr))
	}
	assert.False(t, IsValidCapabilityTrend(CapabilityTrend("flat")))
}

func TestCapability_ClassifyTrendFromDelta(t *testing.T) {
	// Auto-classification from delta.
	assert.Equal(t, CapabilityTrendImproving, classifyTrend(0.10))
	assert.Equal(t, CapabilityTrendStable, classifyTrend(0.04))
	assert.Equal(t, CapabilityTrendStable, classifyTrend(-0.04))
	assert.Equal(t, CapabilityTrendDegrading, classifyTrend(-0.10))
}

func TestCapability_RecordSnapshot_AutoClassifiesTrend(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	saved, err := store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
	require.NoError(t, err)
	// Both metrics had delta > 0.05 → improving.
	for _, m := range saved.Metrics {
		assert.Equal(t, CapabilityTrendImproving, m.Trend,
			"trend auto-classified from delta")
	}
}

func TestCapability_RecordSnapshot_RejectsRequiredFields(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	for _, missing := range []string{"tenant", "user", "period", "metrics"} {
		s := validCapabilitySnapshot()
		switch missing {
		case "tenant":
			s.TenantID = ""
		case "user":
			s.UserID = ""
		case "period":
			s.Period = ReportPeriod{}
		case "metrics":
			s.Metrics = nil
		}
		_, err := store.RecordSnapshot(context.Background(), s)
		assert.Error(t, err, "missing %s must error", missing)
	}
}

func TestCapability_RecordSnapshot_RejectsValueOutOfRange(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	s := validCapabilitySnapshot()
	bad := s.Metrics[CapabilityDomainKnowledge]
	bad.Value = 1.5
	s.Metrics[CapabilityDomainKnowledge] = bad
	_, err := store.RecordSnapshot(context.Background(), s)
	assert.True(t, errors.Is(err, ErrCapabilityValueOutOfRange))
}

func TestCapability_RecordSnapshot_RejectsDeltaOutOfRange(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	s := validCapabilitySnapshot()
	bad := s.Metrics[CapabilityDomainKnowledge]
	bad.TrendDelta = 1.5
	s.Metrics[CapabilityDomainKnowledge] = bad
	_, err := store.RecordSnapshot(context.Background(), s)
	assert.True(t, errors.Is(err, ErrCapabilityDeltaOutOfRange))
}

func TestCapability_RecordSnapshot_RejectsMapKeyMismatch(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	s := validCapabilitySnapshot()
	mismatch := s.Metrics[CapabilityDomainKnowledge]
	mismatch.Dimension = CapabilityCollaboration // mismatch with map key
	s.Metrics[CapabilityDomainKnowledge] = mismatch
	_, err := store.RecordSnapshot(context.Background(), s)
	assert.Error(t, err, "map key must equal metric.Dimension")
}

func TestCapability_RecordSnapshot_TruncatesLongNotes(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	s := validCapabilitySnapshot()
	s.Notes = strings.Repeat("x", 2000)
	saved, _ := store.RecordSnapshot(context.Background(), s)
	assert.Equal(t, 1000, len(saved.Notes))
}

func TestCapability_FindSnapshotByID_RoundTrips(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	saved, _ := store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
	got, err := store.FindSnapshotByID(context.Background(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.UserID, got.UserID)
}

func TestCapability_FindSnapshotByID_UnknownReturnsNotFound(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	_, err := store.FindSnapshotByID(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrCapabilitySnapshotNotFound))
}

func TestCapability_FindLatestSnapshotForUser_ReturnsNewest(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	first, _ := store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
	time.Sleep(5 * time.Millisecond)
	second, _ := store.RecordSnapshot(context.Background(), validCapabilitySnapshot())

	got, err := store.FindLatestSnapshotForUser(context.Background(), "t", "alice")
	require.NoError(t, err)
	assert.Equal(t, second.ID, got.ID, "newest returned")
	_ = first
}

func TestCapability_FindLatestSnapshotForUser_TenantIsolation(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
	_, err := store.FindLatestSnapshotForUser(context.Background(), "different", "alice")
	assert.True(t, errors.Is(err, ErrCapabilitySnapshotNotFound))
}

func TestCapability_RecordChange_AssignsIDAndTimestamp(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	saved, err := store.RecordChange(context.Background(), validCapabilityChange())
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, saved.ID)
	assert.False(t, saved.OccurredAt.IsZero())
}

func TestCapability_RecordChange_RejectsRequiredFields(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	for _, missing := range []string{"tenant", "user", "dimension", "reason"} {
		e := validCapabilityChange()
		switch missing {
		case "tenant":
			e.TenantID = ""
		case "user":
			e.UserID = ""
		case "dimension":
			e.Dimension = ""
		case "reason":
			e.Reason = ""
		}
		_, err := store.RecordChange(context.Background(), e)
		assert.Error(t, err, "missing %s must error", missing)
	}
}

func TestCapability_RecordChange_RejectsDeltaOutOfRange(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	e := validCapabilityChange()
	e.Delta = 2.0
	_, err := store.RecordChange(context.Background(), e)
	assert.True(t, errors.Is(err, ErrCapabilityDeltaOutOfRange))
}

func TestCapability_RecordChange_TruncatesLongReason(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	e := validCapabilityChange()
	e.Reason = strings.Repeat("x", 800)
	saved, _ := store.RecordChange(context.Background(), e)
	assert.Equal(t, 500, len(saved.Reason))
}

func TestCapability_ListChangesForUser_NewestFirst(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	for i := 0; i < 3; i++ {
		_, _ = store.RecordChange(context.Background(), validCapabilityChange())
		time.Sleep(2 * time.Millisecond)
	}
	got, err := store.ListChangesForUser(context.Background(), "t", "alice", time.Time{}, 0)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.True(t, got[i-1].OccurredAt.After(got[i].OccurredAt) ||
			got[i-1].OccurredAt.Equal(got[i].OccurredAt))
	}
}

func TestCapability_ListChangesForUser_RespectsLimit(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	for i := 0; i < 5; i++ {
		_, _ = store.RecordChange(context.Background(), validCapabilityChange())
	}
	got, _ := store.ListChangesForUser(context.Background(), "t", "alice", time.Time{}, 2)
	assert.Len(t, got, 2)
}

func TestCapability_ListChangesForUser_FiltersBySinceCutoff(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	_, _ = store.RecordChange(context.Background(), validCapabilityChange())
	cutoff := time.Now()
	time.Sleep(10 * time.Millisecond)
	_, _ = store.RecordChange(context.Background(), validCapabilityChange())

	got, _ := store.ListChangesForUser(context.Background(), "t", "alice", cutoff, 0)
	assert.Len(t, got, 1)
}

func TestCapability_ListChangesForUser_TenantIsolation(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	_, _ = store.RecordChange(context.Background(), validCapabilityChange())
	got, _ := store.ListChangesForUser(context.Background(), "different", "alice", time.Time{}, 0)
	assert.Empty(t, got)
}

func TestCapability_AggregateTrendsByDimension_AlwaysAllDimensionsAndTrends(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())

	hist, err := store.AggregateTrendsByDimension(context.Background(), "t")
	require.NoError(t, err)
	for _, dim := range AllCapabilityDimensions() {
		_, ok := hist[dim]
		assert.True(t, ok, "dimension %q axis must exist", dim)
		for _, tr := range AllCapabilityTrends() {
			_, ok := hist[dim][tr]
			assert.True(t, ok, "trend %q axis must exist for %q", tr, dim)
		}
	}
}

func TestCapability_AggregateTrendsByDimension_UsesLatestSnapshotPerUser(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	// Older snapshot — should be ignored.
	_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
	time.Sleep(5 * time.Millisecond)
	// Newer snapshot — should be used.
	_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())

	hist, _ := store.AggregateTrendsByDimension(context.Background(), "t")
	// Count = 1 user × 1 trend (improving) = 1 (not 2 because we use latest only).
	assert.Equal(t, 1, hist[CapabilityDomainKnowledge][CapabilityTrendImproving])
}

func TestCapability_AggregateTrendsByDimension_TenantIsolation(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
	hist, _ := store.AggregateTrendsByDimension(context.Background(), "different-tenant")
	for _, dim := range AllCapabilityDimensions() {
		for _, tr := range AllCapabilityTrends() {
			assert.Equal(t, 0, hist[dim][tr])
		}
	}
}

func TestCapability_ConcurrentRecordIsSafe(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.RecordChange(context.Background(), validCapabilityChange())
		}()
	}
	wg.Wait()
	got, _ := store.ListChangesForUser(context.Background(), "t", "alice", time.Time{}, 0)
	assert.Len(t, got, 50)
}

func TestCapability_ContextCancelledOperationsError(t *testing.T) {
	store := NewInMemoryCapabilityMetricsStore()
	saved, _ := store.RecordSnapshot(context.Background(), validCapabilitySnapshot())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.RecordSnapshot(ctx, validCapabilitySnapshot())
	assert.Error(t, err)

	_, err = store.FindSnapshotByID(ctx, saved.ID)
	assert.Error(t, err)

	_, err = store.FindLatestSnapshotForUser(ctx, "t", "alice")
	assert.Error(t, err)

	_, err = store.RecordChange(ctx, validCapabilityChange())
	assert.Error(t, err)

	_, err = store.ListChangesForUser(ctx, "t", "alice", time.Time{}, 0)
	assert.Error(t, err)

	_, err = store.AggregateTrendsByDimension(ctx, "t")
	assert.Error(t, err)
}
