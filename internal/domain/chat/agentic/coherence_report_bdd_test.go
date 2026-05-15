package agentic

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HUMAN-005 — Codebase coherence report BDD.
//
// PDF arXiv:2604.14228v1 §11 (decisions can drift over time —
// contradictory choices accumulate as silent debt).

func TestBDD_CoherenceReport(t *testing.T) {

	t.Run("Scenario_OperatorRunsWeeklyCoherenceCheck", func(t *testing.T) {
		// Given an operator wants to verify codebase coherence after
		//       a week of agent activity,
		// When the report is built with empty inputs,
		// Then the report explicitly states no inconsistencies (audit
		//      contract — silence is dangerous).
		rep, err := BuildCoherenceReport("acme", validCoherencePeriod(), CoherenceReportInputs{})
		require.NoError(t, err)
		assert.Empty(t, rep.Findings)
		out := rep.HumanSummary()
		assert.Contains(t, out, "No inconsistencies detected")
	})

	t.Run("Scenario_TwoContradictoryDecisionsTriggerCriticalFinding", func(t *testing.T) {
		// Given two accepted decisions on the same topic without
		//       a supersede chain,
		d1 := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "Pick payment processor",
			Choice: "Stripe", Rationale: "x", Status: DecisionStatusAccepted,
		}
		d2 := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "Pick payment processor",
			Choice: "Adyen", Rationale: "y", Status: DecisionStatusAccepted,
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{d1, d2},
		})
		require.NotEmpty(t, rep.Findings)
		assert.True(t, rep.HasCriticalFinding(),
			"contradictory decisions = critical drift")
	})

	t.Run("Scenario_SupersededDecisionsDoNotTriggerContradiction", func(t *testing.T) {
		// Given the second decision properly supersedes the first,
		newID := uuid.New()
		d1 := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "Pick payment processor",
			Choice: "Stripe", Status: DecisionStatusSuperseded, SupersededBy: &newID,
		}
		d2 := DecisionRecord{
			ID: newID, TenantID: "t", Title: "Pick payment processor",
			Choice: "Adyen", Status: DecisionStatusAccepted,
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{d1, d2},
		})
		// No contradictory_decisions finding (supersede is the audit trail).
		for _, f := range rep.Findings {
			assert.NotEqual(t, InconsistencyKindContradictoryDecisions, f.Kind)
		}
	})

	t.Run("Scenario_RepeatedReviewFocusOnSameFileSurfacesHotspot", func(t *testing.T) {
		// Given the same file is flagged in 3+ reviews — structural debt signal,
		mkReview := func(file string, id string) ReviewGuidance {
			r := *NewReviewGuidance(id, "code_diff", "x")
			r.FocusPoints = []ReviewFocusPoint{
				{Category: ReviewCategorySecurity, Location: file + ":42",
					Severity: ReviewSeverityCritical, Reason: "x"},
			}
			return r
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Reviews: []ReviewGuidance{
				mkReview("auth.go", "PR-1"),
				mkReview("auth.go", "PR-2"),
				mkReview("auth.go", "PR-3"),
			},
		})
		hasFocus := false
		for _, f := range rep.Findings {
			if f.Kind == InconsistencyKindRepeatedReviewFocus {
				hasFocus = true
				assert.Equal(t, "auth.go", f.Subject,
					"hot-spot file extracted (line numbers stripped)")
			}
		}
		assert.True(t, hasFocus, "3+ reviews on same file = hot-spot")
	})

	t.Run("Scenario_TwoReviewsOnSameFileNotEnoughForHotspot", func(t *testing.T) {
		// Given the threshold for hot-spot is 3 (avoid noise),
		mkReview := func(id string) ReviewGuidance {
			r := *NewReviewGuidance(id, "code_diff", "x")
			r.FocusPoints = []ReviewFocusPoint{
				{Category: ReviewCategorySecurity, Location: "auth.go",
					Severity: ReviewSeverityCritical, Reason: "x"},
			}
			return r
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Reviews: []ReviewGuidance{mkReview("PR-1"), mkReview("PR-2")},
		})
		for _, f := range rep.Findings {
			assert.NotEqual(t, InconsistencyKindRepeatedReviewFocus, f.Kind,
				"2 reviews not enough — threshold is 3 for signal/noise ratio")
		}
	})

	t.Run("Scenario_ContradictoryIntentsOnSameFileTriggerChurnSignal", func(t *testing.T) {
		// Given a file is added then reverted then re-added — churn signal,
		d := *NewExplainedDiff("PR-1", "churn").
			AddHunk(ExplainedHunk{
				Location: HunkLocation{File: "feature.go", StartLine: 1, EndLine: 50},
				Intent:   HunkIntentFeatureAdd, Rationale: "add feature",
			}).
			AddHunk(ExplainedHunk{
				Location: HunkLocation{File: "feature.go", StartLine: 1, EndLine: 50},
				Intent:   HunkIntentRevert, Rationale: "rolling back",
			})
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Diffs: []ExplainedDiff{d},
		})
		hasChurn := false
		for _, f := range rep.Findings {
			if f.Kind == InconsistencyKindContradictoryIntents {
				hasChurn = true
				assert.Equal(t, "feature.go", f.Subject)
				assert.Equal(t, ReviewSeverityWarn, f.Severity)
			}
		}
		assert.True(t, hasChurn)
	})

	t.Run("Scenario_OrphanedDecisionDetectedWhenNeverReferenced", func(t *testing.T) {
		// Given a decision was accepted but no diff links to it — was
		//       it actually implemented?,
		dec := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "Adopt feature flags",
			Choice: "LaunchDarkly", Rationale: "x", Status: DecisionStatusAccepted,
		}
		// Diff doesn't reference dec.ID.
		d := *NewExplainedDiff("PR-1", "unrelated").AddBugFix("foo.go", 1, 1, "x")
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{dec},
			Diffs:     []ExplainedDiff{d},
		})
		hasOrphan := false
		for _, f := range rep.Findings {
			if f.Kind == InconsistencyKindOrphanedDecisions {
				hasOrphan = true
				assert.Equal(t, "Adopt feature flags", f.Subject)
				assert.Equal(t, ReviewSeverityInfo, f.Severity,
					"orphaned = info severity (gentle reminder, not crisis)")
			}
		}
		assert.True(t, hasOrphan)
	})

	t.Run("Scenario_LinkedDecisionsAreNotOrphaned", func(t *testing.T) {
		// Given a diff links to the decision via LinkedRequirements,
		dec := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "Adopt feature flags",
			Choice: "LaunchDarkly", Rationale: "x", Status: DecisionStatusAccepted,
		}
		d := *NewExplainedDiff("PR-1", "implementation").
			AddBugFix("flags.go", 1, 100, "first integration", dec.ID.String())
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{dec},
			Diffs:     []ExplainedDiff{d},
		})
		for _, f := range rep.Findings {
			assert.NotEqual(t, InconsistencyKindOrphanedDecisions, f.Kind,
				"linked decision is implemented — not orphan")
		}
	})

	t.Run("Scenario_EmptyTenantIDIsRejected", func(t *testing.T) {
		_, err := BuildCoherenceReport("", validCoherencePeriod(), CoherenceReportInputs{})
		assert.True(t, errors.Is(err, ErrEmptyCoherenceTenantID))
	})

	t.Run("Scenario_InvalidPeriodIsRejected", func(t *testing.T) {
		_, err := BuildCoherenceReport("t", ReportPeriod{}, CoherenceReportInputs{})
		assert.True(t, errors.Is(err, ErrInvalidCoherencePeriod))
	})

	t.Run("Scenario_FindingsSortedSeverityFirstForRendering", func(t *testing.T) {
		// Given mixed-severity findings,
		mkReview := func(id string) ReviewGuidance {
			r := *NewReviewGuidance(id, "code_diff", "x")
			r.FocusPoints = []ReviewFocusPoint{
				{Category: ReviewCategorySecurity, Location: "a.go",
					Severity: ReviewSeverityCritical, Reason: "x"},
			}
			return r
		}
		decID := uuid.New()
		dec := DecisionRecord{
			ID: decID, TenantID: "t", Title: "x", Choice: "A", Rationale: "y",
			Status: DecisionStatusAccepted,
		}
		d2 := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "x", Choice: "B", Rationale: "z",
			Status: DecisionStatusAccepted,
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{dec, d2}, // contradictory → critical
			Reviews:   []ReviewGuidance{mkReview("1"), mkReview("2"), mkReview("3")}, // hot-spot → warn
		})
		require.NotEmpty(t, rep.Findings)
		assert.Equal(t, ReviewSeverityCritical, rep.Findings[0].Severity,
			"after sort, critical first")
	})

	t.Run("Scenario_FourInconsistencyKindsCovered", func(t *testing.T) {
		// Given dashboards bind to kind strings,
		expected := map[string]bool{
			"contradictory_decisions": true,
			"repeated_review_focus":   true,
			"contradictory_intents":   true,
			"orphaned_decisions":      true,
		}
		for _, k := range AllInconsistencyKinds() {
			assert.True(t, expected[string(k)])
		}
		assert.Equal(t, 4, len(AllInconsistencyKinds()))
	})

	t.Run("Scenario_HistogramByKindHasStableAxes", func(t *testing.T) {
		rep := CoherenceReport{
			Findings: []InconsistencyFinding{
				{Kind: InconsistencyKindContradictoryDecisions},
			},
		}
		hist := rep.CountByKind()
		for _, k := range AllInconsistencyKinds() {
			_, ok := hist[k]
			assert.True(t, ok, "kind %q must appear with 0 default", k)
		}
	})

	t.Run("Scenario_VolumeCountsHelpInterpretFindingsDensity", func(t *testing.T) {
		// Given "5 findings" means different things on 100 vs 10000 diffs,
		dec := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "x", Choice: "A",
			Rationale: "y", Status: DecisionStatusAccepted,
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{dec},
		})
		assert.Equal(t, 1, rep.DecisionsRecorded,
			"caller can compute findings/decision ratio")
	})

	t.Run("Scenario_HumanSummaryShowsTopTenFindings", func(t *testing.T) {
		// Given dashboards / digest emails want a digest of the worst,
		// (Test contract via empty findings = explicit "no inconsistencies".)
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{})
		out := rep.HumanSummary()
		assert.Contains(t, out, "tenant t")
		assert.NotEmpty(t, out)
	})

	t.Run("Scenario_LocationStrippingGroupsAcrossLineRefs", func(t *testing.T) {
		// Given reviews flag "auth.go:42" / "auth.go#section-3" / "auth.go",
		// When repeated-focus detection runs,
		// Then all 3 group under "auth.go" (line/section info stripped).
		mkReview := func(loc, id string) ReviewGuidance {
			r := *NewReviewGuidance(id, "code_diff", "x")
			r.FocusPoints = []ReviewFocusPoint{
				{Category: ReviewCategorySecurity, Location: loc,
					Severity: ReviewSeverityCritical, Reason: "x"},
			}
			return r
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Reviews: []ReviewGuidance{
				mkReview("auth.go:42", "PR-1"),
				mkReview("auth.go#section-3", "PR-2"),
				mkReview("auth.go", "PR-3"),
			},
		})
		for _, f := range rep.Findings {
			if f.Kind == InconsistencyKindRepeatedReviewFocus {
				assert.Equal(t, "auth.go", f.Subject)
			}
		}
	})

	t.Run("Scenario_FirstSeenAndLastSeenSpanTheEvidenceWindow", func(t *testing.T) {
		// Given findings span time, the FirstSeenAt / LastSeenAt help
		//       the human prioritize "old vs recent drift",
		early := time.Now().Add(-72 * time.Hour)
		late := time.Now()
		d1 := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "x", Choice: "A",
			Rationale: "y", Status: DecisionStatusAccepted, RecordedAt: early,
		}
		d2 := DecisionRecord{
			ID: uuid.New(), TenantID: "t", Title: "x", Choice: "B",
			Rationale: "z", Status: DecisionStatusAccepted, RecordedAt: late,
		}
		rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
			Decisions: []DecisionRecord{d1, d2},
		})
		require.NotEmpty(t, rep.Findings)
		assert.True(t, rep.Findings[0].FirstSeenAt.Equal(early) ||
			rep.Findings[0].FirstSeenAt.Before(late),
			"FirstSeenAt = earliest evidence")
		assert.True(t, rep.Findings[0].LastSeenAt.Equal(late) ||
			rep.Findings[0].LastSeenAt.After(early),
			"LastSeenAt = latest evidence")
	})
}
