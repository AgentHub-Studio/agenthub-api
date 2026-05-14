package agentic

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OBS-009 — Quality report persistence.
//
// PDF arXiv:2604.14228v1 Section 11.4 — "deterministic evaluator output
// feeds observability and circuit breakers"; Section 7 — quality reports
// are aggregated across runs to inform retraining and alerting.
//
// OBS-008 introduced the evaluator and the QualityReport envelope
// (in-memory, ephemeral). OBS-009 introduces the persistence layer so
// reports become DURABLE, QUERYABLE, and AGGREGATABLE — a precondition
// for circuit-breaker / alerting / cost-attribution dashboards.
//
// Two separations are enforced:
//
//   - The store NEVER mutates the report payload. Persistence is
//     append-only (StoredQualityReport is immutable after Save).
//   - The store is tenant-isolated by TenantID; queries that omit
//     TenantID return only platform-public data.

// ErrQualityReportNotFound is returned when a lookup misses.
var ErrQualityReportNotFound = errors.New("quality report not found")

// StoredQualityReport wraps a QualityReport with the storage envelope:
// stable storage ID + tenant scope + run/agent correlation + insertion
// timestamp.
type StoredQualityReport struct {
	// ID is the storage-layer identifier (assigned at Save time).
	ID uuid.UUID
	// TenantID scopes the report to a tenant. uuid.Nil = platform-public.
	TenantID uuid.UUID
	// AgentID identifies the generator agent.
	AgentID uuid.UUID
	// RunID correlates back to the originating run trace (OBS-001
	// envelope + OBS-006 subagent tracing). Same string the SSE
	// EventEvaluation payload carries.
	RunID string
	// Report is the evaluator's verdict — value, not pointer, so it
	// cannot be mutated through the store.
	Report QualityReport
	// StoredAt is wall-clock time of insertion.
	StoredAt time.Time
}

// QualityReportStore is the persistence interface for quality reports.
// Implementations must be concurrent-safe (multiple runners may persist
// reports simultaneously).
type QualityReportStore interface {
	// Save persists a report and returns the storage-layer record with
	// ID + StoredAt populated. The store assigns these — the caller
	// must not pre-populate them.
	Save(ctx context.Context, tenantID uuid.UUID, agentID uuid.UUID, runID string, report QualityReport) (StoredQualityReport, error)

	// FindByRunID returns the report for a given run, scoped to a tenant.
	// Returns ErrQualityReportNotFound when nothing matches.
	FindByRunID(ctx context.Context, tenantID uuid.UUID, runID string) (StoredQualityReport, error)

	// ListByAgent returns reports for an agent, newest-first, capped by
	// limit (0 = no cap, return everything).
	ListByAgent(ctx context.Context, tenantID uuid.UUID, agentID uuid.UUID, limit int) ([]StoredQualityReport, error)

	// ListByDecision returns reports matching a decision (e.g. all
	// EvalFail reports for a tenant in a window).
	ListByDecision(ctx context.Context, tenantID uuid.UUID, decision EvaluationDecision, limit int) ([]StoredQualityReport, error)

	// ListSince returns reports newer than the given cutoff.
	ListSince(ctx context.Context, tenantID uuid.UUID, since time.Time, limit int) ([]StoredQualityReport, error)

	// CountByDecision returns a histogram of decisions for a tenant
	// since the cutoff. Useful for circuit-breaker thresholds.
	CountByDecision(ctx context.Context, tenantID uuid.UUID, since time.Time) (map[EvaluationDecision]int, error)

	// AverageScore returns the mean OverallScore for a tenant since
	// the cutoff. Returns (0, false) if no reports exist.
	AverageScore(ctx context.Context, tenantID uuid.UUID, since time.Time) (float64, bool, error)
}

// --- InMemoryQualityReportStore ---

// InMemoryQualityReportStore is the default in-memory implementation.
// Concurrent-safe via sync.RWMutex. Suitable for tests, dev, and
// single-process deployments. Production should use a SQL-backed store
// (separate file, follows same interface).
type InMemoryQualityReportStore struct {
	mu   sync.RWMutex
	rows []StoredQualityReport
}

// NewInMemoryQualityReportStore creates an empty in-memory store.
func NewInMemoryQualityReportStore() *InMemoryQualityReportStore {
	return &InMemoryQualityReportStore{rows: []StoredQualityReport{}}
}

// Save persists a report. ID is assigned via uuid.New(); StoredAt is
// set to time.Now(). The input report is COPIED — caller mutations
// after Save do not affect the stored record.
func (s *InMemoryQualityReportStore) Save(
	ctx context.Context,
	tenantID uuid.UUID,
	agentID uuid.UUID,
	runID string,
	report QualityReport,
) (StoredQualityReport, error) {
	if err := ctx.Err(); err != nil {
		return StoredQualityReport{}, err
	}
	rec := StoredQualityReport{
		ID:       uuid.New(),
		TenantID: tenantID,
		AgentID:  agentID,
		RunID:    runID,
		Report:   report, // value copy; QualityReport carries no pointers
		StoredAt: time.Now(),
	}
	s.mu.Lock()
	s.rows = append(s.rows, rec)
	s.mu.Unlock()
	return rec, nil
}

// FindByRunID returns the report for a (tenantID, runID) pair.
func (s *InMemoryQualityReportStore) FindByRunID(
	ctx context.Context,
	tenantID uuid.UUID,
	runID string,
) (StoredQualityReport, error) {
	if err := ctx.Err(); err != nil {
		return StoredQualityReport{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.rows {
		if r.TenantID == tenantID && r.RunID == runID {
			return r, nil
		}
	}
	return StoredQualityReport{}, ErrQualityReportNotFound
}

// ListByAgent returns reports newest-first.
func (s *InMemoryQualityReportStore) ListByAgent(
	ctx context.Context,
	tenantID uuid.UUID,
	agentID uuid.UUID,
	limit int,
) ([]StoredQualityReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := []StoredQualityReport{}
	for _, r := range s.rows {
		if r.TenantID == tenantID && r.AgentID == agentID {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].StoredAt.After(matched[j].StoredAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// ListByDecision returns reports matching a decision, newest-first.
func (s *InMemoryQualityReportStore) ListByDecision(
	ctx context.Context,
	tenantID uuid.UUID,
	decision EvaluationDecision,
	limit int,
) ([]StoredQualityReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := []StoredQualityReport{}
	for _, r := range s.rows {
		if r.TenantID == tenantID && r.Report.Decision == decision {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].StoredAt.After(matched[j].StoredAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// ListSince returns reports stored after `since`, newest-first.
func (s *InMemoryQualityReportStore) ListSince(
	ctx context.Context,
	tenantID uuid.UUID,
	since time.Time,
	limit int,
) ([]StoredQualityReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := []StoredQualityReport{}
	for _, r := range s.rows {
		if r.TenantID == tenantID && r.StoredAt.After(since) {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].StoredAt.After(matched[j].StoredAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// CountByDecision returns a histogram of decisions since the cutoff.
// Always returns a map populated with all 3 decisions (zero counts
// included) so dashboards can render axes without conditional logic.
func (s *InMemoryQualityReportStore) CountByDecision(
	ctx context.Context,
	tenantID uuid.UUID,
	since time.Time,
) (map[EvaluationDecision]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hist := map[EvaluationDecision]int{
		EvalPass: 0,
		EvalWarn: 0,
		EvalFail: 0,
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.rows {
		if r.TenantID == tenantID && r.StoredAt.After(since) {
			hist[r.Report.Decision]++
		}
	}
	return hist, nil
}

// AverageScore returns the mean OverallScore for a tenant since `since`.
// (0, false, nil) when no reports exist in window.
func (s *InMemoryQualityReportStore) AverageScore(
	ctx context.Context,
	tenantID uuid.UUID,
	since time.Time,
) (float64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sum := 0.0
	count := 0
	for _, r := range s.rows {
		if r.TenantID == tenantID && r.StoredAt.After(since) {
			sum += r.Report.OverallScore
			count++
		}
	}
	if count == 0 {
		return 0, false, nil
	}
	return sum / float64(count), true, nil
}
