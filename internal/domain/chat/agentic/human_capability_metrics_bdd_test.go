package agentic

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FUTURE-006 — Human capability metrics BDD.
//
// PDF arXiv:2604.14228v1 §12 (track whether the platform is HELPING
// humans grow vs creating dependence).

func TestBDD_HumanCapabilityMetrics(t *testing.T) {

	t.Run("Scenario_AdminMonitorsUserCapabilityTrajectoryOver30Days", func(t *testing.T) {
		// Given an admin wants to verify the platform is helping users
		//       grow (PDF §12: meta-feedback that agent helps, not
		//       creates dependence),
		store := NewInMemoryCapabilityMetricsStore()
		now := time.Now().UTC()
		saved, err := store.RecordSnapshot(context.Background(), HumanCapabilitySnapshot{
			TenantID: "acme", UserID: "alice",
			Period: ReportPeriod{Start: now.Add(-30 * 24 * time.Hour), End: now},
			Metrics: map[CapabilityDimension]CapabilityMetric{
				CapabilityDomainKnowledge: {
					Dimension: CapabilityDomainKnowledge,
					Value: 0.75, TrendDelta: 0.15, // strong improvement
					SampleSize: 100, LastMeasuredAt: now,
				},
				CapabilityDecisionIndependence: {
					Dimension: CapabilityDecisionIndependence,
					Value: 0.65, TrendDelta: 0.12,
					SampleSize: 100, LastMeasuredAt: now,
				},
			},
			Notes: "Alice now answers tier-1 support questions independently",
		})
		require.NoError(t, err)
		// Trends auto-classified as improving (delta > 0.05).
		for _, m := range saved.Metrics {
			assert.Equal(t, CapabilityTrendImproving, m.Trend)
		}
	})

	t.Run("Scenario_FiveDimensionsCoverHolisticCapabilityModel", func(t *testing.T) {
		// Given capability is multi-faceted (PDF §12 enumerates these),
		expected := map[string]bool{
			"domain_knowledge":       true,
			"decision_independence":  true,
			"task_throughput":        true,
			"quality_output":         true,
			"collaboration":          true,
		}
		for _, d := range AllCapabilityDimensions() {
			assert.True(t, expected[string(d)])
		}
	})

	t.Run("Scenario_TrendAutoClassifiedFromDeltaWithThreshold", func(t *testing.T) {
		// Given the auto-classifier uses ±0.05 threshold,
		assert.Equal(t, CapabilityTrendStable, classifyTrend(0.04))
		assert.Equal(t, CapabilityTrendImproving, classifyTrend(0.06))
		assert.Equal(t, CapabilityTrendStable, classifyTrend(-0.04))
		assert.Equal(t, CapabilityTrendDegrading, classifyTrend(-0.06))
	})

	t.Run("Scenario_DegradingTrendSignalsDependenceBuilding", func(t *testing.T) {
		// Given the platform contract is "help users grow",
		// When a degrading trend appears (delta < -0.05),
		// Then it signals the user may be becoming dependent — admin
		//      should investigate.
		store := NewInMemoryCapabilityMetricsStore()
		now := time.Now().UTC()
		_, err := store.RecordSnapshot(context.Background(), HumanCapabilitySnapshot{
			TenantID: "t", UserID: "user-degrading",
			Period: ReportPeriod{Start: now.Add(-30 * 24 * time.Hour), End: now},
			Metrics: map[CapabilityDimension]CapabilityMetric{
				CapabilityDecisionIndependence: {
					Dimension: CapabilityDecisionIndependence,
					Value: 0.30, TrendDelta: -0.20, // degrading
					SampleSize: 100, LastMeasuredAt: now,
				},
			},
		})
		require.NoError(t, err)
		latest, _ := store.FindLatestSnapshotForUser(context.Background(), "t", "user-degrading")
		assert.Equal(t, CapabilityTrendDegrading,
			latest.Metrics[CapabilityDecisionIndependence].Trend,
			"strong negative delta = degrading (dependence building)")
	})

	t.Run("Scenario_ChangeEventCapturesDiscreteCapabilityShift", func(t *testing.T) {
		// Given a clear moment where capability shifted (e.g. user
		//       solved a problem they used to delegate),
		store := NewInMemoryCapabilityMetricsStore()
		saved, err := store.RecordChange(context.Background(), CapabilityChangeEvent{
			TenantID: "t", UserID: "alice",
			Dimension: CapabilityDomainKnowledge,
			Delta:     0.05,
			Reason:    "User wrote complete OAuth integration without asking the agent for boilerplate",
		})
		require.NoError(t, err)
		assert.Equal(t, 0.05, saved.Delta)
		assert.NotEmpty(t, saved.Reason)
	})

	t.Run("Scenario_RequiredFieldsEnforceMeaningfulMetrics", func(t *testing.T) {
		// Given metrics without context are useless (audit + analysis),
		store := NewInMemoryCapabilityMetricsStore()
		_, err := store.RecordChange(context.Background(), CapabilityChangeEvent{
			Dimension: CapabilityDomainKnowledge, Delta: 0.05,
			// Missing TenantID, UserID, Reason
		})
		assert.Error(t, err)
	})

	t.Run("Scenario_ValueAndDeltaAreBoundedToAvoidNonsense", func(t *testing.T) {
		// Given Value [0,1] and Delta [-1,+1] are normalized scales,
		store := NewInMemoryCapabilityMetricsStore()
		s := validCapabilitySnapshot()
		bad := s.Metrics[CapabilityDomainKnowledge]
		bad.Value = 1.5
		s.Metrics[CapabilityDomainKnowledge] = bad
		_, err := store.RecordSnapshot(context.Background(), s)
		assert.Error(t, err, "value > 1.0 must error")
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantQuery", func(t *testing.T) {
		store := NewInMemoryCapabilityMetricsStore()
		_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
		_, err := store.FindLatestSnapshotForUser(context.Background(), "different", "alice")
		assert.Error(t, err)
	})

	t.Run("Scenario_LatestSnapshotIsUsedForAggregateTrends", func(t *testing.T) {
		// Given users may have many snapshots over time, only the LATEST
		//       represents current capability state,
		store := NewInMemoryCapabilityMetricsStore()
		_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())
		time.Sleep(5 * time.Millisecond)
		_, _ = store.RecordSnapshot(context.Background(), validCapabilitySnapshot())

		hist, _ := store.AggregateTrendsByDimension(context.Background(), "t")
		// 1 user × 1 latest snapshot = 1 entry (not 2).
		assert.Equal(t, 1, hist[CapabilityDomainKnowledge][CapabilityTrendImproving])
	})

	t.Run("Scenario_AggregateProducesStableAxesForDashboards", func(t *testing.T) {
		// Given dashboards show "X improving / Y stable / Z degrading"
		//       per dimension,
		store := NewInMemoryCapabilityMetricsStore()
		hist, _ := store.AggregateTrendsByDimension(context.Background(), "t")
		// All 5 dimensions × 3 trends present even with zero data.
		for _, dim := range AllCapabilityDimensions() {
			for _, tr := range AllCapabilityTrends() {
				_, ok := hist[dim][tr]
				assert.True(t, ok)
			}
		}
	})

	t.Run("Scenario_ListChangesShowsRecentEventsForUser", func(t *testing.T) {
		// Given the admin reviews recent capability changes for a user,
		store := NewInMemoryCapabilityMetricsStore()
		for i := 0; i < 5; i++ {
			_, _ = store.RecordChange(context.Background(), validCapabilityChange())
		}
		got, _ := store.ListChangesForUser(context.Background(), "t", "alice", time.Time{}, 3)
		assert.Len(t, got, 3, "limit applied")
	})

	t.Run("Scenario_NewestFirstOrderingForRecencyDashboard", func(t *testing.T) {
		store := NewInMemoryCapabilityMetricsStore()
		for i := 0; i < 3; i++ {
			_, _ = store.RecordChange(context.Background(), validCapabilityChange())
			time.Sleep(2 * time.Millisecond)
		}
		got, _ := store.ListChangesForUser(context.Background(), "t", "alice", time.Time{}, 0)
		require.Len(t, got, 3)
		for i := 1; i < len(got); i++ {
			assert.True(t,
				got[i-1].OccurredAt.After(got[i].OccurredAt) ||
					got[i-1].OccurredAt.Equal(got[i].OccurredAt))
		}
	})

	t.Run("Scenario_LongReasonsAndNotesAreBoundedToPreventLogSpam", func(t *testing.T) {
		store := NewInMemoryCapabilityMetricsStore()
		s := validCapabilitySnapshot()
		s.Notes = ""
		for i := 0; i < 2000; i++ {
			s.Notes += "x"
		}
		saved, _ := store.RecordSnapshot(context.Background(), s)
		assert.Equal(t, 1000, len(saved.Notes), "notes capped at 1000 chars")
	})

	t.Run("Scenario_MapKeyMustEqualMetricDimension", func(t *testing.T) {
		// Given the map structure is map[Dimension]Metric — key/value
		//       Dimension must match (defensive: caller bug guard),
		store := NewInMemoryCapabilityMetricsStore()
		s := validCapabilitySnapshot()
		mismatched := s.Metrics[CapabilityDomainKnowledge]
		mismatched.Dimension = CapabilityCollaboration
		s.Metrics[CapabilityDomainKnowledge] = mismatched
		_, err := store.RecordSnapshot(context.Background(), s)
		assert.Error(t, err, "map key/metric dimension mismatch caught")
	})
}
