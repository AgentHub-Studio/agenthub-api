package agentic

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validComplexityDriftThreshold() ComplexityDriftThreshold {
	return ComplexityDriftThreshold{
		Signal:     ComplexityDriftSignalScopeCreep,
		WarnAt:     50.0,
		CriticalAt: 100.0,
	}
}

func validComplexityDriftAlert() ComplexityDriftAlert {
	return ComplexityDriftAlert{
		AlertID:         uuid.New(),
		Signal:          ComplexityDriftSignalScopeCreep,
		Severity:        ComplexityDriftSeverityWarn,
		SubjectSlug:     "run-abc123",
		OwnerTenantSlug: "acme-corp",
		ObservedValue:   60.0,
		ThresholdAt:     50.0,
		Description:     "Run has accumulated 60 sub-tasks beyond the original 10.",
		ObservedAt:      time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC),
	}
}

func TestComplexityDrift_IsValidSignal(t *testing.T) {
	for _, s := range allComplexityDriftSignals {
		assert.True(t, IsValidComplexityDriftSignal(s))
	}
	assert.False(t, IsValidComplexityDriftSignal(ComplexityDriftSignal("nope")))
}

func TestComplexityDrift_AllSignalsReturnsCopy(t *testing.T) {
	s := AllComplexityDriftSignals()
	require.Equal(t, 6, len(s))
	s[0] = "tampered"
	s2 := AllComplexityDriftSignals()
	assert.Equal(t, ComplexityDriftSignalScopeCreep, s2[0])
}

func TestComplexityDrift_IsValidSeverity(t *testing.T) {
	for _, s := range allComplexityDriftSeverities {
		assert.True(t, IsValidComplexityDriftSeverity(s))
	}
	assert.False(t, IsValidComplexityDriftSeverity(ComplexityDriftSeverity("limbo")))
}

func TestComplexityDrift_AllSeveritiesReturnsCopy(t *testing.T) {
	s := AllComplexityDriftSeverities()
	require.Equal(t, 3, len(s))
	s[0] = "tampered"
	s2 := AllComplexityDriftSeverities()
	assert.Equal(t, ComplexityDriftSeverityInfo, s2[0])
}

func TestComplexityDrift_ThresholdValidateBadSignal(t *testing.T) {
	tr := validComplexityDriftThreshold()
	tr.Signal = "nope"
	assert.ErrorIs(t, tr.Validate(), ErrComplexityDriftBadSignal)
}

func TestComplexityDrift_ThresholdValidateNegativeWarn(t *testing.T) {
	tr := validComplexityDriftThreshold()
	tr.WarnAt = -1.0
	assert.ErrorIs(t, tr.Validate(), ErrComplexityDriftNegativeWarn)
}

func TestComplexityDrift_ThresholdValidateBadOrder(t *testing.T) {
	tr := validComplexityDriftThreshold()
	tr.CriticalAt = 10.0 // < WarnAt = 50
	assert.ErrorIs(t, tr.Validate(), ErrComplexityDriftBadThresholdOrder)
}

func TestComplexityDrift_ThresholdValidateEqualWarnCriticalRejected(t *testing.T) {
	tr := validComplexityDriftThreshold()
	tr.CriticalAt = 50.0 // = WarnAt
	assert.ErrorIs(t, tr.Validate(), ErrComplexityDriftBadThresholdOrder)
}

func TestComplexityDrift_SeverityForObservationBelowWarn(t *testing.T) {
	tr := validComplexityDriftThreshold()
	_, ok := tr.SeverityForObservation(49.9)
	assert.False(t, ok)
}

func TestComplexityDrift_SeverityForObservationAtWarn(t *testing.T) {
	tr := validComplexityDriftThreshold()
	sev, ok := tr.SeverityForObservation(50.0)
	require.True(t, ok)
	assert.Equal(t, ComplexityDriftSeverityWarn, sev)
}

func TestComplexityDrift_SeverityForObservationAtCritical(t *testing.T) {
	tr := validComplexityDriftThreshold()
	sev, ok := tr.SeverityForObservation(100.0)
	require.True(t, ok)
	assert.Equal(t, ComplexityDriftSeverityCritical, sev)
}

func TestComplexityDrift_SeverityForObservationBetween(t *testing.T) {
	tr := validComplexityDriftThreshold()
	sev, ok := tr.SeverityForObservation(75.0)
	require.True(t, ok)
	assert.Equal(t, ComplexityDriftSeverityWarn, sev)
}

func TestComplexityDrift_AlertValidateEmptyID(t *testing.T) {
	a := validComplexityDriftAlert()
	a.AlertID = uuid.Nil
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftEmptyAlertID)
}

func TestComplexityDrift_AlertValidateBadSignal(t *testing.T) {
	a := validComplexityDriftAlert()
	a.Signal = "nope"
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftBadSignal)
}

func TestComplexityDrift_AlertValidateBadSeverity(t *testing.T) {
	a := validComplexityDriftAlert()
	a.Severity = "limbo"
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftBadSeverity)
}

func TestComplexityDrift_AlertValidateBadSubject(t *testing.T) {
	a := validComplexityDriftAlert()
	a.SubjectSlug = "Bad Slug"
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftBadSubject)
}

func TestComplexityDrift_AlertValidateBadOwner(t *testing.T) {
	a := validComplexityDriftAlert()
	a.OwnerTenantSlug = "Bad Owner"
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftBadOwner)
}

func TestComplexityDrift_AlertValidateEmptyDescription(t *testing.T) {
	a := validComplexityDriftAlert()
	a.Description = ""
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftEmptyDescription)
}

func TestComplexityDrift_AlertValidateEmptyObservedAt(t *testing.T) {
	a := validComplexityDriftAlert()
	a.ObservedAt = time.Time{}
	assert.ErrorIs(t, a.Validate(), ErrComplexityDriftEmptyObservedAt)
}

func TestComplexityDrift_DetectorSetAndLookupThreshold(t *testing.T) {
	d := NewComplexityDriftDetector()
	tr := validComplexityDriftThreshold()
	require.NoError(t, d.SetThreshold(tr))
	got, ok := d.LookupThreshold(tr.Signal)
	require.True(t, ok)
	assert.Equal(t, tr.WarnAt, got.WarnAt)
}

func TestComplexityDrift_DetectorRejectsBadThreshold(t *testing.T) {
	d := NewComplexityDriftDetector()
	bad := validComplexityDriftThreshold()
	bad.CriticalAt = 1.0
	assert.ErrorIs(t, d.SetThreshold(bad), ErrComplexityDriftBadThresholdOrder)
}

func TestComplexityDrift_DetectorEvaluateBelowWarn(t *testing.T) {
	d := NewComplexityDriftDetector()
	_ = d.SetThreshold(validComplexityDriftThreshold())
	_, ok := d.Evaluate(ComplexityDriftSignalScopeCreep, 10.0)
	assert.False(t, ok)
}

func TestComplexityDrift_DetectorEvaluateNoThresholdConfigured(t *testing.T) {
	d := NewComplexityDriftDetector()
	_, ok := d.Evaluate(ComplexityDriftSignalScopeCreep, 999.0)
	assert.False(t, ok, "no threshold => no alert needed")
}

func TestComplexityDrift_DetectorBuildAlertCritical(t *testing.T) {
	d := NewComplexityDriftDetector()
	_ = d.SetThreshold(validComplexityDriftThreshold())
	now := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	alert, err := d.BuildAlert(
		ComplexityDriftSignalScopeCreep, 200.0,
		"run-abc", "acme-corp",
		"Scope creep crossed critical threshold", now,
	)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.Equal(t, ComplexityDriftSeverityCritical, alert.Severity)
	assert.Equal(t, 100.0, alert.ThresholdAt)
}

func TestComplexityDrift_DetectorBuildAlertWarn(t *testing.T) {
	d := NewComplexityDriftDetector()
	_ = d.SetThreshold(validComplexityDriftThreshold())
	now := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	alert, err := d.BuildAlert(
		ComplexityDriftSignalScopeCreep, 60.0,
		"run-abc", "acme-corp",
		"Scope creep warn band", now,
	)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.Equal(t, ComplexityDriftSeverityWarn, alert.Severity)
	assert.Equal(t, 50.0, alert.ThresholdAt)
}

func TestComplexityDrift_DetectorBuildAlertNoneNeeded(t *testing.T) {
	d := NewComplexityDriftDetector()
	_ = d.SetThreshold(validComplexityDriftThreshold())
	now := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	alert, err := d.BuildAlert(
		ComplexityDriftSignalScopeCreep, 10.0,
		"run-abc", "acme-corp",
		"Below warn band", now,
	)
	require.NoError(t, err)
	assert.Nil(t, alert)
}

func TestComplexityDrift_DetectorBuildAlertBadDescription(t *testing.T) {
	d := NewComplexityDriftDetector()
	_ = d.SetThreshold(validComplexityDriftThreshold())
	now := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	_, err := d.BuildAlert(
		ComplexityDriftSignalScopeCreep, 200.0,
		"run-abc", "acme-corp", "", now,
	)
	assert.ErrorIs(t, err, ErrComplexityDriftEmptyDescription)
}

func TestComplexityDrift_AllThresholdsSortedBySignal(t *testing.T) {
	d := NewComplexityDriftDetector()
	t1 := validComplexityDriftThreshold()
	t1.Signal = ComplexityDriftSignalChurnSpike
	t2 := validComplexityDriftThreshold()
	t2.Signal = ComplexityDriftSignalCognitiveLoad
	require.NoError(t, d.SetThreshold(t1))
	require.NoError(t, d.SetThreshold(t2))
	list := d.AllThresholds()
	require.Equal(t, 2, len(list))
	assert.Equal(t, ComplexityDriftSignalChurnSpike, list[0].Signal)
}

func TestComplexityDrift_DetectorConcurrentSafe(t *testing.T) {
	d := NewComplexityDriftDetector()
	signals := []ComplexityDriftSignal{
		ComplexityDriftSignalScopeCreep,
		ComplexityDriftSignalDependencyExplosion,
		ComplexityDriftSignalTestDecay,
		ComplexityDriftSignalChurnSpike,
		ComplexityDriftSignalGoalDrift,
		ComplexityDriftSignalCognitiveLoad,
	}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tr := validComplexityDriftThreshold()
			tr.Signal = signals[i%len(signals)]
			tr.WarnAt = float64(i)
			tr.CriticalAt = float64(i + 100)
			_ = d.SetThreshold(tr)
		}(i)
	}
	wg.Wait()
	// All 6 signals should have been set at least once.
	assert.Equal(t, 6, len(d.AllThresholds()))
}

func TestComplexityDrift_SixSignalsCoverMajorDriftKinds(t *testing.T) {
	expected := map[ComplexityDriftSignal]bool{
		ComplexityDriftSignalScopeCreep:          true,
		ComplexityDriftSignalDependencyExplosion: true,
		ComplexityDriftSignalTestDecay:           true,
		ComplexityDriftSignalChurnSpike:          true,
		ComplexityDriftSignalGoalDrift:           true,
		ComplexityDriftSignalCognitiveLoad:       true,
	}
	assert.Equal(t, 6, len(expected))
	for _, s := range allComplexityDriftSignals {
		assert.True(t, expected[s])
	}
}

func TestComplexityDrift_ThreeSeveritiesCoverInfoWarnCritical(t *testing.T) {
	expected := map[ComplexityDriftSeverity]bool{
		ComplexityDriftSeverityInfo:     true,
		ComplexityDriftSeverityWarn:     true,
		ComplexityDriftSeverityCritical: true,
	}
	assert.Equal(t, 3, len(expected))
	for _, s := range allComplexityDriftSeverities {
		assert.True(t, expected[s])
	}
}
