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

// OBS-009 — Quality report persistence BDD.
//
// PDF arXiv:2604.14228v1 Section 11.4 — "deterministic evaluator output
// feeds observability and circuit breakers"; Section 7 — quality reports
// are aggregated across runs.
//
// These scenarios validate the persistence contract: durable, queryable,
// tenant-isolated, append-only, concurrent-safe.

func TestBDD_QualityReportPersistence(t *testing.T) {

	t.Run("Scenario_EveryRunReportIsDurablyStored", func(t *testing.T) {
		// Given the evaluator (OBS-008) just produced a QualityReport,
		// When the runner persists it,
		// Then the report can be retrieved later via run ID — closing
		//      the loop from generation → evaluation → persistence.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		report := QualityReport{
			EvaluatorName: "heuristic",
			OverallScore:  0.82,
			Decision:      EvalPass,
			EvaluatedAt:   time.Now(),
		}
		_, err := store.Save(context.Background(), tenantID, agentID, "run-x", report)
		require.NoError(t, err)

		got, err := store.FindByRunID(context.Background(), tenantID, "run-x")
		require.NoError(t, err)
		assert.Equal(t, EvalPass, got.Report.Decision,
			"persisted report must round-trip identically")
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantLeak", func(t *testing.T) {
		// Given multi-tenancy is the platform's primary security
		//       boundary (CLAUDE.md — multi-tenant via schema isolation),
		// When tenant A saves a report,
		// Then tenant B MUST NOT see it via ANY query path.
		store := NewInMemoryQualityReportStore()
		tenantA := uuid.New()
		tenantB := uuid.New()
		agentID := uuid.New()

		report := QualityReport{
			EvaluatorName: "heuristic",
			OverallScore:  0.5,
			Decision:      EvalWarn,
			EvaluatedAt:   time.Now(),
		}
		_, _ = store.Save(context.Background(), tenantA, agentID, "tenant-a-run", report)

		// Path 1: FindByRunID
		_, err := store.FindByRunID(context.Background(), tenantB, "tenant-a-run")
		assert.True(t, errors.Is(err, ErrQualityReportNotFound),
			"path 1 (FindByRunID): tenant B must not see tenant A's row")

		// Path 2: ListByAgent
		listB, _ := store.ListByAgent(context.Background(), tenantB, agentID, 0)
		assert.Empty(t, listB,
			"path 2 (ListByAgent): tenant B must see empty list")

		// Path 3: ListByDecision
		warnsB, _ := store.ListByDecision(context.Background(), tenantB, EvalWarn, 0)
		assert.Empty(t, warnsB,
			"path 3 (ListByDecision): tenant B must see no warns")

		// Path 4: CountByDecision
		histB, _ := store.CountByDecision(context.Background(), tenantB, time.Time{})
		assert.Equal(t, 0, histB[EvalWarn],
			"path 4 (CountByDecision): tenant B histogram must show 0")

		// Path 5: AverageScore
		_, ok, _ := store.AverageScore(context.Background(), tenantB, time.Time{})
		assert.False(t, ok,
			"path 5 (AverageScore): tenant B must have no average")
	})

	t.Run("Scenario_PersistenceIsAppendOnlyEnforcedByValueCopy", func(t *testing.T) {
		// Given audit/compliance requires that stored reports are
		//       IMMUTABLE — caller cannot retroactively rewrite history,
		// When the caller mutates the original report after Save,
		// Then the stored record is unchanged. The contract is enforced
		//      by passing QualityReport by value — pointer-based stores
		//      would break this guarantee.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		original := QualityReport{
			EvaluatorName: "heuristic",
			OverallScore:  0.9,
			Decision:      EvalPass,
			EvaluatedAt:   time.Now(),
		}
		_, _ = store.Save(context.Background(), tenantID, agentID, "audit", original)

		// Caller mutates the original. Must not affect the store.
		original.Decision = EvalFail
		original.OverallScore = 0.0

		got, _ := store.FindByRunID(context.Background(), tenantID, "audit")
		assert.Equal(t, EvalPass, got.Report.Decision,
			"audit guarantee: post-Save mutation must NOT alter stored record")
	})

	t.Run("Scenario_CountByDecisionAlwaysExposesAllThreeAxes", func(t *testing.T) {
		// Given dashboards render decision histograms with stable axes,
		// When CountByDecision returns a tenant histogram,
		// Then ALL THREE decision keys are present (zero counts
		//      included) — dashboards never need conditional logic to
		//      handle missing axes.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		_, _ = store.Save(context.Background(), tenantID, uuid.New(), "only-pass",
			QualityReport{Decision: EvalPass})

		hist, err := store.CountByDecision(context.Background(), tenantID, time.Time{})
		require.NoError(t, err)
		_, hasPass := hist[EvalPass]
		_, hasWarn := hist[EvalWarn]
		_, hasFail := hist[EvalFail]
		assert.True(t, hasPass, "EvalPass key required")
		assert.True(t, hasWarn, "EvalWarn key required (zero count)")
		assert.True(t, hasFail, "EvalFail key required (zero count)")
	})

	t.Run("Scenario_AverageScoreFeedsCircuitBreakerThreshold", func(t *testing.T) {
		// Given a circuit-breaker policy: "if average tenant score over
		//       last 1h drops below 0.4, alert ops" (PDF Section 11.4),
		// When AverageScore is queried,
		// Then it returns the mean and (true) — alerting code can act.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		for _, s := range []float64{0.2, 0.3, 0.5} {
			_, _ = store.Save(context.Background(), tenantID, agentID, "x",
				QualityReport{OverallScore: s, Decision: EvalFail})
		}

		avg, ok, err := store.AverageScore(context.Background(), tenantID, time.Time{})
		require.NoError(t, err)
		require.True(t, ok)
		assert.InDelta(t, (0.2+0.3+0.5)/3, avg, 0.0001)
		assert.Less(t, avg, 0.4,
			"this average would trigger the example circuit-breaker rule")
	})

	t.Run("Scenario_ListSinceEnablesTimeWindowedQueries", func(t *testing.T) {
		// Given an alerting query "list all reports stored in the last
		//       60 seconds",
		// When ListSince is called with the cutoff,
		// Then only reports stored AFTER the cutoff appear — historical
		//      noise is excluded.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		_, _ = store.Save(context.Background(), tenantID, agentID, "old-1",
			QualityReport{OverallScore: 0.5, Decision: EvalWarn})
		_, _ = store.Save(context.Background(), tenantID, agentID, "old-2",
			QualityReport{OverallScore: 0.5, Decision: EvalWarn})
		cutoff := time.Now()
		time.Sleep(10 * time.Millisecond)
		_, _ = store.Save(context.Background(), tenantID, agentID, "new-1",
			QualityReport{OverallScore: 0.5, Decision: EvalWarn})

		got, err := store.ListSince(context.Background(), tenantID, cutoff, 0)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "new-1", got[0].RunID,
			"only post-cutoff reports must be returned")
	})

	t.Run("Scenario_ListByDecisionEnablesFailureRollup", func(t *testing.T) {
		// Given an oncall dashboard "show me the most recent 10 fails",
		// When ListByDecision(EvalFail, 10) is called,
		// Then only EvalFail rows return, newest-first, capped at 10.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		// 5 fails + 3 passes
		for i := 0; i < 5; i++ {
			_, _ = store.Save(context.Background(), tenantID, agentID, "x",
				QualityReport{OverallScore: 0.1, Decision: EvalFail})
		}
		for i := 0; i < 3; i++ {
			_, _ = store.Save(context.Background(), tenantID, agentID, "y",
				QualityReport{OverallScore: 0.9, Decision: EvalPass})
		}

		fails, _ := store.ListByDecision(context.Background(), tenantID, EvalFail, 10)
		assert.Len(t, fails, 5, "all 5 fails must return (cap 10 not hit)")
		for _, r := range fails {
			assert.Equal(t, EvalFail, r.Report.Decision)
		}

		// Cap test:
		failsCap, _ := store.ListByDecision(context.Background(), tenantID, EvalFail, 2)
		assert.Len(t, failsCap, 2, "cap=2 must trim to 2")
	})

	t.Run("Scenario_ConcurrentRunnersCanPersistSimultaneously", func(t *testing.T) {
		// Given multiple agentic runs may finish at the same instant,
		// When they all call Save concurrently,
		// Then no panics, no lost rows — concurrent-safe.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		var wg sync.WaitGroup
		const N = 50
		for i := 0; i < N; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := store.Save(context.Background(), tenantID, agentID, "c",
					QualityReport{Decision: EvalPass})
				assert.NoError(t, err)
			}()
		}
		wg.Wait()

		got, _ := store.ListByAgent(context.Background(), tenantID, agentID, 0)
		assert.Len(t, got, N,
			"all %d concurrent Saves must result in %d stored rows", N, N)
	})

	t.Run("Scenario_StoredReportCorrelatesBackToRunForTracing", func(t *testing.T) {
		// Given OBS-006 subagent tracing chains spans by RunID,
		// When a report is stored,
		// Then its StoredQualityReport carries RunID exactly — the
		//      tracing layer can join run spans to quality records.
		store := NewInMemoryQualityReportStore()
		tenantID := uuid.New()
		agentID := uuid.New()

		rec, _ := store.Save(context.Background(), tenantID, agentID, "trace-corr-1",
			QualityReport{Decision: EvalPass})
		assert.Equal(t, "trace-corr-1", rec.RunID,
			"RunID is the join key for tracing — must round-trip exact")
	})

	t.Run("Scenario_NotFoundIsExplicitErrorNotEmptyValue", func(t *testing.T) {
		// Given silent missing-data is a recurring source of bugs in
		//       observability code (PDF Section 11 — silent failures),
		// When FindByRunID misses,
		// Then a SENTINEL error returns (ErrQualityReportNotFound) —
		//      callers cannot accidentally treat empty as success.
		store := NewInMemoryQualityReportStore()
		_, err := store.FindByRunID(context.Background(), uuid.New(), "no-such-run")
		assert.True(t, errors.Is(err, ErrQualityReportNotFound),
			"missing report must return ErrQualityReportNotFound (got %v)", err)
	})

	t.Run("Scenario_ContextCancellationIsHonoredAcrossOperations", func(t *testing.T) {
		// Given a cancelled run cleanup loop,
		// When ctx is cancelled before each operation,
		// Then ALL store methods honor cancellation — never block, never
		//      leak goroutines, never silently succeed on stale ctx.
		store := NewInMemoryQualityReportStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ops := []struct {
			name string
			fn   func() error
		}{
			{"Save", func() error {
				_, e := store.Save(ctx, uuid.New(), uuid.New(), "r", QualityReport{Decision: EvalPass})
				return e
			}},
			{"FindByRunID", func() error {
				_, e := store.FindByRunID(ctx, uuid.New(), "r")
				return e
			}},
			{"ListByAgent", func() error {
				_, e := store.ListByAgent(ctx, uuid.New(), uuid.New(), 0)
				return e
			}},
			{"ListByDecision", func() error {
				_, e := store.ListByDecision(ctx, uuid.New(), EvalPass, 0)
				return e
			}},
			{"ListSince", func() error {
				_, e := store.ListSince(ctx, uuid.New(), time.Time{}, 0)
				return e
			}},
			{"CountByDecision", func() error {
				_, e := store.CountByDecision(ctx, uuid.New(), time.Time{})
				return e
			}},
			{"AverageScore", func() error {
				_, _, e := store.AverageScore(ctx, uuid.New(), time.Time{})
				return e
			}},
		}
		for _, op := range ops {
			err := op.fn()
			assert.Error(t, err, "operation %q must honor cancelled ctx", op.name)
		}
	})
}
