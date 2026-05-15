package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FUTURE-006 — Human capability metrics.
//
// PDF arXiv:2604.14228v1 §12 (Future Directions — track whether the
// platform is HELPING humans grow vs creating dependence; meta-feedback
// for "is this agent making the user more capable, or less?").
//
// Distinct from existing user-tracking abstractions:
//   - FUTURE-001 = facts the agent remembers ABOUT the user
//   - FUTURE-002 = relationship state (trust, rapport)
//   - FUTURE-006 = the human's CAPABILITY trajectory while using the agent
//
// Tracks 5 dimensions per user: domain_knowledge, decision_independence,
// task_throughput, quality_output, collaboration. Each measured in
// [0,1] with a trend signal in [-1,+1] (improving / stable / degrading).
//
// Snapshots are taken periodically (e.g. monthly) by analyzing the user's
// recent behavior. Change events record discrete moments where capability
// shifted (e.g. "user no longer asks how to do X — independence +0.05").

// CapabilityDimension bounded enum.
type CapabilityDimension string

const (
	// CapabilityDomainKnowledge — depth of subject-matter understanding.
	CapabilityDomainKnowledge CapabilityDimension = "domain_knowledge"
	// CapabilityDecisionIndependence — fraction of decisions made
	// without delegating to the agent.
	CapabilityDecisionIndependence CapabilityDimension = "decision_independence"
	// CapabilityTaskThroughput — time-to-completion / volume.
	CapabilityTaskThroughput CapabilityDimension = "task_throughput"
	// CapabilityQualityOutput — error rate / quality scores.
	CapabilityQualityOutput CapabilityDimension = "quality_output"
	// CapabilityCollaboration — effectiveness with other humans/agents.
	CapabilityCollaboration CapabilityDimension = "collaboration"
)

var allCapabilityDimensions = []CapabilityDimension{
	CapabilityDomainKnowledge, CapabilityDecisionIndependence,
	CapabilityTaskThroughput, CapabilityQualityOutput,
	CapabilityCollaboration,
}

// IsValidCapabilityDimension returns true for the bounded set.
func IsValidCapabilityDimension(d CapabilityDimension) bool {
	for _, v := range allCapabilityDimensions {
		if d == v {
			return true
		}
	}
	return false
}

// AllCapabilityDimensions returns a copy.
func AllCapabilityDimensions() []CapabilityDimension {
	out := make([]CapabilityDimension, len(allCapabilityDimensions))
	copy(out, allCapabilityDimensions)
	return out
}

// CapabilityTrend bounded enum for direction signal.
type CapabilityTrend string

const (
	CapabilityTrendImproving CapabilityTrend = "improving"
	CapabilityTrendStable    CapabilityTrend = "stable"
	CapabilityTrendDegrading CapabilityTrend = "degrading"
)

// IsValidCapabilityTrend returns true for the bounded set.
func IsValidCapabilityTrend(t CapabilityTrend) bool {
	switch t {
	case CapabilityTrendImproving, CapabilityTrendStable, CapabilityTrendDegrading:
		return true
	}
	return false
}

// AllCapabilityTrends returns a copy.
func AllCapabilityTrends() []CapabilityTrend {
	return []CapabilityTrend{
		CapabilityTrendImproving, CapabilityTrendStable, CapabilityTrendDegrading,
	}
}

// CapabilityMetric is a single per-dimension measurement.
type CapabilityMetric struct {
	Dimension       CapabilityDimension `json:"dimension"`
	// Value in [0, 1]. 1.0 = max capability.
	Value           float64             `json:"value"`
	// TrendDelta in [-1, +1]. Positive = improving, negative = degrading.
	TrendDelta      float64             `json:"trendDelta"`
	// Trend is the discrete classification (computed from TrendDelta).
	Trend           CapabilityTrend     `json:"trend"`
	// SampleSize indicates how many observations went into this metric.
	SampleSize      int                 `json:"sampleSize"`
	LastMeasuredAt  time.Time           `json:"lastMeasuredAt"`
}

// HumanCapabilitySnapshot is a periodic measurement (e.g. monthly) for one user.
type HumanCapabilitySnapshot struct {
	ID         uuid.UUID                            `json:"id"`
	TenantID   string                               `json:"tenantId"`
	UserID     string                               `json:"userId"`
	Period     ReportPeriod                         `json:"period"`
	Metrics    map[CapabilityDimension]CapabilityMetric `json:"metrics"`
	Notes      string                               `json:"notes,omitempty"`
	RecordedAt time.Time                            `json:"recordedAt"`
}

// CapabilityChangeEvent records a discrete capability shift.
type CapabilityChangeEvent struct {
	ID         uuid.UUID           `json:"id"`
	TenantID   string              `json:"tenantId"`
	UserID     string              `json:"userId"`
	Dimension  CapabilityDimension `json:"dimension"`
	// Delta in [-1, +1].
	Delta      float64             `json:"delta"`
	Reason     string              `json:"reason"`
	OccurredAt time.Time           `json:"occurredAt"`
}

// Sentinels.
var (
	ErrCapabilitySnapshotNotFound = errors.New("capability metrics: snapshot not found")
	ErrInvalidCapabilityDimension = errors.New("capability metrics: invalid dimension")
	ErrInvalidCapabilityTrend     = errors.New("capability metrics: invalid trend")
	ErrCapabilityValueOutOfRange  = errors.New("capability metrics: value out of [0, 1] range")
	ErrCapabilityDeltaOutOfRange  = errors.New("capability metrics: delta out of [-1, +1] range")
)

// classifyTrend converts a numerical delta to a discrete trend.
// Threshold: ±0.05 = stable, beyond = improving/degrading.
func classifyTrend(delta float64) CapabilityTrend {
	const threshold = 0.05
	switch {
	case delta > threshold:
		return CapabilityTrendImproving
	case delta < -threshold:
		return CapabilityTrendDegrading
	default:
		return CapabilityTrendStable
	}
}

// validateMetric checks bounds + dimension.
func validateMetric(m CapabilityMetric) error {
	if !IsValidCapabilityDimension(m.Dimension) {
		return fmt.Errorf("%w: %q", ErrInvalidCapabilityDimension, m.Dimension)
	}
	if m.Value < 0 || m.Value > 1 {
		return fmt.Errorf("%w: %f", ErrCapabilityValueOutOfRange, m.Value)
	}
	if m.TrendDelta < -1 || m.TrendDelta > 1 {
		return fmt.Errorf("%w: %f", ErrCapabilityDeltaOutOfRange, m.TrendDelta)
	}
	return nil
}

// validateSnapshot checks required fields + bounds.
func validateSnapshot(s HumanCapabilitySnapshot) error {
	if s.TenantID == "" {
		return errors.New("capability metrics: tenantId required")
	}
	if s.UserID == "" {
		return errors.New("capability metrics: userId required")
	}
	if !s.Period.IsValid() {
		return errors.New("capability metrics: period invalid")
	}
	if len(s.Metrics) == 0 {
		return errors.New("capability metrics: at least one metric required")
	}
	for dim, m := range s.Metrics {
		if dim != m.Dimension {
			return fmt.Errorf("capability metrics: map key %q != metric.Dimension %q", dim, m.Dimension)
		}
		if err := validateMetric(m); err != nil {
			return err
		}
	}
	return nil
}

func validateChangeEvent(e CapabilityChangeEvent) error {
	if e.TenantID == "" {
		return errors.New("capability metrics: tenantId required")
	}
	if e.UserID == "" {
		return errors.New("capability metrics: userId required")
	}
	if !IsValidCapabilityDimension(e.Dimension) {
		return fmt.Errorf("%w: %q", ErrInvalidCapabilityDimension, e.Dimension)
	}
	if e.Delta < -1 || e.Delta > 1 {
		return fmt.Errorf("%w: %f", ErrCapabilityDeltaOutOfRange, e.Delta)
	}
	if e.Reason == "" {
		return errors.New("capability metrics: reason required for change event")
	}
	return nil
}

// CapabilityMetricsStore is the persistence interface.
type CapabilityMetricsStore interface {
	RecordSnapshot(ctx context.Context, s HumanCapabilitySnapshot) (HumanCapabilitySnapshot, error)
	FindSnapshotByID(ctx context.Context, id uuid.UUID) (HumanCapabilitySnapshot, error)
	FindLatestSnapshotForUser(ctx context.Context, tenantID, userID string) (HumanCapabilitySnapshot, error)
	RecordChange(ctx context.Context, e CapabilityChangeEvent) (CapabilityChangeEvent, error)
	ListChangesForUser(ctx context.Context, tenantID, userID string, since time.Time, limit int) ([]CapabilityChangeEvent, error)
	AggregateTrendsByDimension(ctx context.Context, tenantID string) (map[CapabilityDimension]map[CapabilityTrend]int, error)
}

// --- InMemoryCapabilityMetricsStore ---

type InMemoryCapabilityMetricsStore struct {
	mu        sync.Mutex
	snapshots map[uuid.UUID]HumanCapabilitySnapshot
	changes   []CapabilityChangeEvent
}

func NewInMemoryCapabilityMetricsStore() *InMemoryCapabilityMetricsStore {
	return &InMemoryCapabilityMetricsStore{
		snapshots: map[uuid.UUID]HumanCapabilitySnapshot{},
	}
}

// RecordSnapshot persists a snapshot. Defaults Trend if not set.
func (s *InMemoryCapabilityMetricsStore) RecordSnapshot(ctx context.Context, snap HumanCapabilitySnapshot) (HumanCapabilitySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return HumanCapabilitySnapshot{}, err
	}
	// Auto-classify trend from delta if Trend not specified.
	for dim, m := range snap.Metrics {
		if m.Trend == "" {
			m.Trend = classifyTrend(m.TrendDelta)
		}
		snap.Metrics[dim] = m
	}
	if err := validateSnapshot(snap); err != nil {
		return HumanCapabilitySnapshot{}, err
	}
	if len(snap.Notes) > 1000 {
		snap.Notes = snap.Notes[:997] + "..."
	}
	snap.ID = uuid.New()
	snap.RecordedAt = time.Now()
	s.mu.Lock()
	s.snapshots[snap.ID] = snap
	s.mu.Unlock()
	return snap, nil
}

// FindSnapshotByID returns a snapshot by ID.
func (s *InMemoryCapabilityMetricsStore) FindSnapshotByID(ctx context.Context, id uuid.UUID) (HumanCapabilitySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return HumanCapabilitySnapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.snapshots[id]
	if !ok {
		return HumanCapabilitySnapshot{}, ErrCapabilitySnapshotNotFound
	}
	return snap, nil
}

// FindLatestSnapshotForUser returns the most recent snapshot.
func (s *InMemoryCapabilityMetricsStore) FindLatestSnapshotForUser(ctx context.Context, tenantID, userID string) (HumanCapabilitySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return HumanCapabilitySnapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest HumanCapabilitySnapshot
	found := false
	for _, snap := range s.snapshots {
		if snap.TenantID != tenantID || snap.UserID != userID {
			continue
		}
		if !found || snap.RecordedAt.After(latest.RecordedAt) {
			latest = snap
			found = true
		}
	}
	if !found {
		return HumanCapabilitySnapshot{}, ErrCapabilitySnapshotNotFound
	}
	return latest, nil
}

// RecordChange persists a change event.
func (s *InMemoryCapabilityMetricsStore) RecordChange(ctx context.Context, e CapabilityChangeEvent) (CapabilityChangeEvent, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityChangeEvent{}, err
	}
	if err := validateChangeEvent(e); err != nil {
		return CapabilityChangeEvent{}, err
	}
	if len(e.Reason) > 500 {
		e.Reason = e.Reason[:497] + "..."
	}
	e.ID = uuid.New()
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	s.mu.Lock()
	s.changes = append(s.changes, e)
	s.mu.Unlock()
	return e, nil
}

// ListChangesForUser returns change events newest-first.
func (s *InMemoryCapabilityMetricsStore) ListChangesForUser(ctx context.Context, tenantID, userID string, since time.Time, limit int) ([]CapabilityChangeEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := []CapabilityChangeEvent{}
	for _, e := range s.changes {
		if e.TenantID != tenantID || e.UserID != userID {
			continue
		}
		if !since.IsZero() && e.OccurredAt.Before(since) {
			continue
		}
		matched = append(matched, e)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].OccurredAt.After(matched[j].OccurredAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// AggregateTrendsByDimension returns dimension → trend → count for tenant.
// Always returns ALL 5 dimensions × 3 trends with 0 default — dashboards
// have stable axes.
func (s *InMemoryCapabilityMetricsStore) AggregateTrendsByDimension(ctx context.Context, tenantID string) (map[CapabilityDimension]map[CapabilityTrend]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := map[CapabilityDimension]map[CapabilityTrend]int{}
	for _, dim := range allCapabilityDimensions {
		out[dim] = map[CapabilityTrend]int{
			CapabilityTrendImproving: 0,
			CapabilityTrendStable:    0,
			CapabilityTrendDegrading: 0,
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// For each user, find their latest snapshot — use that snapshot's trends.
	latest := map[string]HumanCapabilitySnapshot{}
	for _, snap := range s.snapshots {
		if snap.TenantID != tenantID {
			continue
		}
		key := snap.UserID
		if existing, ok := latest[key]; !ok || snap.RecordedAt.After(existing.RecordedAt) {
			latest[key] = snap
		}
	}
	for _, snap := range latest {
		for dim, m := range snap.Metrics {
			out[dim][m.Trend]++
		}
	}
	return out, nil
}
