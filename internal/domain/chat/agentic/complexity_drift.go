package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HUMAN-006 — Complexity/drift alerts.
//
// PDF arXiv:2604.14228v1 §10 (Human-in-the-loop signals). The platform
// watches a small set of "things are going wrong" signals during long
// agentic runs and emits alerts when an observation crosses a tenant-
// declared threshold. Humans see the alert in the UI / Slack / pager
// channel and decide whether to intervene (rewind, pause, re-scope).
//
// Distinct from neighbouring features:
//   - OBS-* are passive traces — every event recorded.
//   - GOV-003 Checkpoint = explicit pause-for-approval mid-run.
//   - HUMAN-004 Understanding = pre-action verification.
//   - HUMAN-006 (this) = passive observer that emits *alerts* when
//     thresholds cross. No control flow blocked; the human reads it
//     async and chooses to act.

// ComplexityDriftSignal bounded enum classifies what kind of drift the
// alert is about.
type ComplexityDriftSignal string

const (
	ComplexityDriftSignalScopeCreep           ComplexityDriftSignal = "scope_creep"
	ComplexityDriftSignalDependencyExplosion  ComplexityDriftSignal = "dependency_explosion"
	ComplexityDriftSignalTestDecay            ComplexityDriftSignal = "test_decay"
	ComplexityDriftSignalChurnSpike           ComplexityDriftSignal = "churn_spike"
	ComplexityDriftSignalGoalDrift            ComplexityDriftSignal = "goal_drift"
	ComplexityDriftSignalCognitiveLoad        ComplexityDriftSignal = "cognitive_load"
)

var allComplexityDriftSignals = []ComplexityDriftSignal{
	ComplexityDriftSignalScopeCreep,
	ComplexityDriftSignalDependencyExplosion,
	ComplexityDriftSignalTestDecay,
	ComplexityDriftSignalChurnSpike,
	ComplexityDriftSignalGoalDrift,
	ComplexityDriftSignalCognitiveLoad,
}

// IsValidComplexityDriftSignal returns true for the bounded set.
func IsValidComplexityDriftSignal(s ComplexityDriftSignal) bool {
	for _, v := range allComplexityDriftSignals {
		if s == v {
			return true
		}
	}
	return false
}

// AllComplexityDriftSignals returns a defensive copy.
func AllComplexityDriftSignals() []ComplexityDriftSignal {
	out := make([]ComplexityDriftSignal, len(allComplexityDriftSignals))
	copy(out, allComplexityDriftSignals)
	return out
}

// ComplexityDriftSeverity bounded enum.
type ComplexityDriftSeverity string

const (
	ComplexityDriftSeverityInfo     ComplexityDriftSeverity = "info"
	ComplexityDriftSeverityWarn     ComplexityDriftSeverity = "warn"
	ComplexityDriftSeverityCritical ComplexityDriftSeverity = "critical"
)

var allComplexityDriftSeverities = []ComplexityDriftSeverity{
	ComplexityDriftSeverityInfo,
	ComplexityDriftSeverityWarn,
	ComplexityDriftSeverityCritical,
}

// IsValidComplexityDriftSeverity returns true for the bounded set.
func IsValidComplexityDriftSeverity(s ComplexityDriftSeverity) bool {
	for _, v := range allComplexityDriftSeverities {
		if s == v {
			return true
		}
	}
	return false
}

// AllComplexityDriftSeverities returns a defensive copy.
func AllComplexityDriftSeverities() []ComplexityDriftSeverity {
	out := make([]ComplexityDriftSeverity, len(allComplexityDriftSeverities))
	copy(out, allComplexityDriftSeverities)
	return out
}

// ComplexityDriftThreshold declares two cut-off values per signal: the
// warn band and the critical band. ObservedValue > WarnAt → warn;
// ObservedValue > CriticalAt → critical. CriticalAt must be > WarnAt.
type ComplexityDriftThreshold struct {
	Signal     ComplexityDriftSignal
	WarnAt     float64
	CriticalAt float64
}

// Validate enforces signal bounded + ordering invariants.
func (t ComplexityDriftThreshold) Validate() error {
	if !IsValidComplexityDriftSignal(t.Signal) {
		return fmt.Errorf("%w: %q", ErrComplexityDriftBadSignal, t.Signal)
	}
	if t.WarnAt < 0 {
		return ErrComplexityDriftNegativeWarn
	}
	if t.CriticalAt <= t.WarnAt {
		return fmt.Errorf("%w: critical=%v must be > warn=%v",
			ErrComplexityDriftBadThresholdOrder, t.CriticalAt, t.WarnAt)
	}
	return nil
}

// SeverityForObservation classifies an observation against the threshold.
// Returns ("", false) if below WarnAt.
func (t ComplexityDriftThreshold) SeverityForObservation(value float64) (ComplexityDriftSeverity, bool) {
	if value >= t.CriticalAt {
		return ComplexityDriftSeverityCritical, true
	}
	if value >= t.WarnAt {
		return ComplexityDriftSeverityWarn, true
	}
	return "", false
}

// ComplexityDriftAlert is one emitted alert record.
type ComplexityDriftAlert struct {
	AlertID         uuid.UUID
	Signal          ComplexityDriftSignal
	Severity        ComplexityDriftSeverity
	SubjectSlug     string // kebab-case identifier of run/agent/project
	OwnerTenantSlug string
	ObservedValue   float64
	ThresholdAt     float64 // the threshold that fired (warn or critical)
	Description     string  // human-readable explanation
	ObservedAt      time.Time
}

var complexityDriftSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Validate enforces invariants. Severity must be bounded; subject +
// owner must be kebab; ObservedAt must be set.
func (a ComplexityDriftAlert) Validate() error {
	if a.AlertID == uuid.Nil {
		return ErrComplexityDriftEmptyAlertID
	}
	if !IsValidComplexityDriftSignal(a.Signal) {
		return fmt.Errorf("%w: %q", ErrComplexityDriftBadSignal, a.Signal)
	}
	if !IsValidComplexityDriftSeverity(a.Severity) {
		return fmt.Errorf("%w: %q", ErrComplexityDriftBadSeverity, a.Severity)
	}
	if !complexityDriftSlugRE.MatchString(a.SubjectSlug) {
		return fmt.Errorf("%w: %q", ErrComplexityDriftBadSubject, a.SubjectSlug)
	}
	if !complexityDriftSlugRE.MatchString(a.OwnerTenantSlug) {
		return fmt.Errorf("%w: %q", ErrComplexityDriftBadOwner, a.OwnerTenantSlug)
	}
	if strings.TrimSpace(a.Description) == "" {
		return ErrComplexityDriftEmptyDescription
	}
	if a.ObservedAt.IsZero() {
		return ErrComplexityDriftEmptyObservedAt
	}
	return nil
}

// ComplexityDriftDetector holds thresholds and emits alerts when
// observations cross them. Thread-safe.
type ComplexityDriftDetector struct {
	mu         sync.RWMutex
	thresholds map[ComplexityDriftSignal]ComplexityDriftThreshold
}

// NewComplexityDriftDetector creates an empty detector.
func NewComplexityDriftDetector() *ComplexityDriftDetector {
	return &ComplexityDriftDetector{
		thresholds: map[ComplexityDriftSignal]ComplexityDriftThreshold{},
	}
}

// SetThreshold registers / overwrites a threshold for a signal.
func (d *ComplexityDriftDetector) SetThreshold(t ComplexityDriftThreshold) error {
	if err := t.Validate(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.thresholds[t.Signal] = t
	return nil
}

// LookupThreshold returns the threshold for a signal, if registered.
func (d *ComplexityDriftDetector) LookupThreshold(signal ComplexityDriftSignal) (ComplexityDriftThreshold, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	t, ok := d.thresholds[signal]
	return t, ok
}

// AllThresholds returns all configured thresholds sorted by signal.
func (d *ComplexityDriftDetector) AllThresholds() []ComplexityDriftThreshold {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]ComplexityDriftThreshold, 0, len(d.thresholds))
	for _, t := range d.thresholds {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Signal < out[j].Signal })
	return out
}

// Evaluate classifies an observation. Returns (severity, alertNeeded).
// alertNeeded=false when observation is below the warn band OR when
// no threshold is configured for the signal.
func (d *ComplexityDriftDetector) Evaluate(signal ComplexityDriftSignal, value float64) (ComplexityDriftSeverity, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	t, ok := d.thresholds[signal]
	if !ok {
		return "", false
	}
	return t.SeverityForObservation(value)
}

// BuildAlert composes a fully-populated alert given an observation.
// Returns nil if the observation does NOT cross any threshold (no
// alert needed) or the signal has no configured threshold.
func (d *ComplexityDriftDetector) BuildAlert(
	signal ComplexityDriftSignal, value float64,
	subjectSlug, ownerTenantSlug, description string,
	observedAt time.Time,
) (*ComplexityDriftAlert, error) {
	severity, needed := d.Evaluate(signal, value)
	if !needed {
		return nil, nil
	}
	d.mu.RLock()
	t, ok := d.thresholds[signal]
	d.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	thresholdAt := t.WarnAt
	if severity == ComplexityDriftSeverityCritical {
		thresholdAt = t.CriticalAt
	}
	alert := &ComplexityDriftAlert{
		AlertID:         uuid.New(),
		Signal:          signal,
		Severity:        severity,
		SubjectSlug:     subjectSlug,
		OwnerTenantSlug: ownerTenantSlug,
		ObservedValue:   value,
		ThresholdAt:     thresholdAt,
		Description:     description,
		ObservedAt:      observedAt,
	}
	if err := alert.Validate(); err != nil {
		return nil, err
	}
	return alert, nil
}

// Sentinel errors.
var (
	ErrComplexityDriftBadSignal           = errors.New("complexity drift: invalid signal")
	ErrComplexityDriftBadSeverity         = errors.New("complexity drift: invalid severity")
	ErrComplexityDriftBadThresholdOrder   = errors.New("complexity drift: critical threshold must be > warn threshold")
	ErrComplexityDriftNegativeWarn        = errors.New("complexity drift: warn threshold must be >= 0")
	ErrComplexityDriftEmptyAlertID        = errors.New("complexity drift: alert id required")
	ErrComplexityDriftBadSubject          = errors.New("complexity drift: subject slug must be kebab-case")
	ErrComplexityDriftBadOwner            = errors.New("complexity drift: owner tenant slug must be kebab-case")
	ErrComplexityDriftEmptyDescription    = errors.New("complexity drift: description required")
	ErrComplexityDriftEmptyObservedAt     = errors.New("complexity drift: observed_at required")
)
