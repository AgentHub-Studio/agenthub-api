package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ComplexityDrift(t *testing.T) {
	t.Run("Scenario_AdminConfiguresScopeCreepThreshold", func(t *testing.T) {
		// Given admin wants alerts when scope creep crosses 50/100,
		// When SetThreshold is called,
		// Then LookupThreshold returns the configured bands.
		d := NewComplexityDriftDetector()
		require.NoError(t, d.SetThreshold(validComplexityDriftThreshold()))
		got, ok := d.LookupThreshold(ComplexityDriftSignalScopeCreep)
		require.True(t, ok)
		assert.Equal(t, 50.0, got.WarnAt)
		assert.Equal(t, 100.0, got.CriticalAt)
	})

	t.Run("Scenario_ObservationBelowWarnEmitsNoAlert", func(t *testing.T) {
		// Given a run accumulates only 10 sub-tasks (below warn=50),
		// When BuildAlert is invoked,
		// Then no alert is built (nil).
		d := NewComplexityDriftDetector()
		_ = d.SetThreshold(validComplexityDriftThreshold())
		alert, err := d.BuildAlert(
			ComplexityDriftSignalScopeCreep, 10.0,
			"run-1", "acme-corp", "x",
			time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		assert.Nil(t, alert)
	})

	t.Run("Scenario_ObservationInWarnBandEmitsWarn", func(t *testing.T) {
		// Given scope creep reaches 60 (warn=50, critical=100),
		// When BuildAlert is invoked,
		// Then an alert with Severity=warn is built.
		d := NewComplexityDriftDetector()
		_ = d.SetThreshold(validComplexityDriftThreshold())
		alert, err := d.BuildAlert(
			ComplexityDriftSignalScopeCreep, 60.0,
			"run-1", "acme-corp", "Scope creep approaching budget",
			time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		require.NotNil(t, alert)
		assert.Equal(t, ComplexityDriftSeverityWarn, alert.Severity)
		assert.Equal(t, 50.0, alert.ThresholdAt)
	})

	t.Run("Scenario_ObservationAboveCriticalEmitsCritical", func(t *testing.T) {
		// Given scope creep reaches 150 (critical=100),
		// When BuildAlert is invoked,
		// Then an alert with Severity=critical is built.
		d := NewComplexityDriftDetector()
		_ = d.SetThreshold(validComplexityDriftThreshold())
		alert, err := d.BuildAlert(
			ComplexityDriftSignalScopeCreep, 150.0,
			"run-1", "acme-corp", "Scope creep critical",
			time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		require.NotNil(t, alert)
		assert.Equal(t, ComplexityDriftSeverityCritical, alert.Severity)
		assert.Equal(t, 100.0, alert.ThresholdAt)
	})

	t.Run("Scenario_NoThresholdConfiguredMeansNoAlert", func(t *testing.T) {
		// Given a tenant has not configured a threshold for goal_drift,
		// When BuildAlert is invoked,
		// Then no alert fires (silence is the default — passive observer).
		d := NewComplexityDriftDetector()
		alert, err := d.BuildAlert(
			ComplexityDriftSignalGoalDrift, 999.0,
			"run-1", "acme-corp", "no threshold",
			time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		assert.Nil(t, alert)
	})

	t.Run("Scenario_CriticalThresholdMustBeAboveWarn", func(t *testing.T) {
		// Given misconfigured thresholds (critical<=warn) make no sense,
		// When SetThreshold is called with critical<=warn,
		// Then registration is rejected.
		d := NewComplexityDriftDetector()
		bad := validComplexityDriftThreshold()
		bad.CriticalAt = 30.0 // < WarnAt=50
		err := d.SetThreshold(bad)
		assert.ErrorIs(t, err, ErrComplexityDriftBadThresholdOrder)
	})

	t.Run("Scenario_AdminListsAllConfiguredThresholdsSorted", func(t *testing.T) {
		// Given admin wants a deterministic UI listing,
		// When AllThresholds is called,
		// Then output is sorted by signal name.
		d := NewComplexityDriftDetector()
		t1 := validComplexityDriftThreshold()
		t1.Signal = ComplexityDriftSignalChurnSpike
		t2 := validComplexityDriftThreshold()
		t2.Signal = ComplexityDriftSignalCognitiveLoad
		require.NoError(t, d.SetThreshold(t1))
		require.NoError(t, d.SetThreshold(t2))
		all := d.AllThresholds()
		require.Equal(t, 2, len(all))
		assert.Equal(t, ComplexityDriftSignalChurnSpike, all[0].Signal)
	})

	t.Run("Scenario_AlertValidationGuardsAgainstMalformedRecords", func(t *testing.T) {
		// Given alerts are persisted/forwarded to Slack/pager,
		// When malformed fields slip in,
		// Then Validate rejects.
		a := validComplexityDriftAlert()
		a.SubjectSlug = "Bad Slug"
		assert.ErrorIs(t, a.Validate(), ErrComplexityDriftBadSubject)
	})

	t.Run("Scenario_PassiveObserverDoesNotBlockRunFlow", func(t *testing.T) {
		// Given HUMAN-006 is a passive observer (not GOV-003 checkpoint),
		// When an alert fires,
		// Then BuildAlert returns the alert but no runtime control-flow
		// hook is invoked (caller decides whether to act).
		d := NewComplexityDriftDetector()
		_ = d.SetThreshold(validComplexityDriftThreshold())
		alert, err := d.BuildAlert(
			ComplexityDriftSignalScopeCreep, 200.0,
			"run-1", "acme-corp", "critical",
			time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC),
		)
		require.NoError(t, err)
		require.NotNil(t, alert)
		// No side-effect: caller just receives the alert struct.
		assert.Equal(t, ComplexityDriftSeverityCritical, alert.Severity)
	})

	t.Run("Scenario_SixSignalsCoverWhatHumansCareAbout", func(t *testing.T) {
		// Given humans worry about scope/dependencies/tests/churn/goal/load,
		// When admin lists supported signals,
		// Then exactly 6 are bounded.
		assert.Equal(t, 6, len(AllComplexityDriftSignals()))
	})
}
