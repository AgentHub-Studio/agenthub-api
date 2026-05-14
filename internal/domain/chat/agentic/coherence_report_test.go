package agentic

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCoherencePeriod() ReportPeriod {
	now := time.Now().UTC()
	return ReportPeriod{Start: now.Add(-24 * time.Hour), End: now}
}

func TestCoherence_KindEnumIsBounded(t *testing.T) {
	for _, k := range AllInconsistencyKinds() {
		assert.True(t, IsValidInconsistencyKind(k))
	}
	assert.False(t, IsValidInconsistencyKind(InconsistencyKind("unknown")))
}

func TestCoherence_AllKindsCount(t *testing.T) {
	// 4 kinds: contradictory_decisions / repeated_review_focus /
	// contradictory_intents / orphaned_decisions.
	assert.Equal(t, 4, len(AllInconsistencyKinds()))
}

func TestCoherence_BuildRejectsEmptyTenant(t *testing.T) {
	_, err := BuildCoherenceReport("", validCoherencePeriod(), CoherenceReportInputs{})
	assert.True(t, errors.Is(err, ErrEmptyCoherenceTenantID))
}

func TestCoherence_BuildRejectsInvalidPeriod(t *testing.T) {
	_, err := BuildCoherenceReport("t", ReportPeriod{}, CoherenceReportInputs{})
	assert.True(t, errors.Is(err, ErrInvalidCoherencePeriod))
}

func TestCoherence_DetectsContradictoryDecisions(t *testing.T) {
	d1 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "Pick database",
		Choice: "Postgres", Rationale: "x", Status: DecisionStatusAccepted,
		RecordedAt: time.Now().Add(-time.Hour),
	}
	d2 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "Pick database",
		Choice: "MySQL", Rationale: "y", Status: DecisionStatusAccepted,
		RecordedAt: time.Now(),
	}
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{d1, d2},
	})
	require.NoError(t, err)
	// May also fire orphan detector since neither decision is linked
	// by any diff. Assert the contradiction finding specifically.
	hasContradiction := false
	for _, f := range rep.Findings {
		if f.Kind == InconsistencyKindContradictoryDecisions {
			hasContradiction = true
			assert.Equal(t, ReviewSeverityCritical, f.Severity)
			assert.Equal(t, "Pick database", f.Subject)
		}
	}
	assert.True(t, hasContradiction)
}

func TestCoherence_SupersededDecisionsAreNotContradictory(t *testing.T) {
	newID := uuid.New()
	d1 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "Pick database",
		Choice: "Postgres", Status: DecisionStatusSuperseded, SupersededBy: &newID,
	}
	d2 := DecisionRecord{
		ID: newID, TenantID: "t", Title: "Pick database",
		Choice: "MySQL", Status: DecisionStatusAccepted,
	}
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{d1, d2},
	})
	require.NoError(t, err)
	for _, f := range rep.Findings {
		assert.NotEqual(t, InconsistencyKindContradictoryDecisions, f.Kind,
			"superseded decisions are not contradictory")
	}
}

func TestCoherence_DetectsRepeatedReviewFocus(t *testing.T) {
	mkReview := func(file string, id string) ReviewGuidance {
		r := *NewReviewGuidance(id, "code_diff", "x")
		r.FocusPoints = []ReviewFocusPoint{
			{Category: ReviewCategorySecurity, Location: file + ":42",
				Severity: ReviewSeverityCritical, Reason: "x"},
		}
		return r
	}
	r1 := mkReview("auth.go", "PR-1")
	r2 := mkReview("auth.go", "PR-2")
	r3 := mkReview("auth.go", "PR-3")
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Reviews: []ReviewGuidance{r1, r2, r3},
	})
	require.NoError(t, err)
	hasFocus := false
	for _, f := range rep.Findings {
		if f.Kind == InconsistencyKindRepeatedReviewFocus {
			hasFocus = true
			assert.Equal(t, "auth.go", f.Subject)
			assert.Len(t, f.Evidence, 3)
		}
	}
	assert.True(t, hasFocus, "3+ reviews on same file = hot-spot finding")
}

func TestCoherence_TwoReviewsAreNotEnoughForRepeatedFocus(t *testing.T) {
	mkReview := func(file string, id string) ReviewGuidance {
		r := *NewReviewGuidance(id, "code_diff", "x")
		r.FocusPoints = []ReviewFocusPoint{
			{Category: ReviewCategorySecurity, Location: file,
				Severity: ReviewSeverityCritical, Reason: "x"},
		}
		return r
	}
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Reviews: []ReviewGuidance{mkReview("a.go", "PR-1"), mkReview("a.go", "PR-2")},
	})
	require.NoError(t, err)
	for _, f := range rep.Findings {
		assert.NotEqual(t, InconsistencyKindRepeatedReviewFocus, f.Kind,
			"2 reviews not enough — threshold is 3+")
	}
}

func TestCoherence_DetectsContradictoryIntents(t *testing.T) {
	d := *NewExplainedDiff("PR-1", "x").
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "x.go", StartLine: 1, EndLine: 5},
			Intent:   HunkIntentFeatureAdd, Rationale: "add",
		}).
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "x.go", StartLine: 1, EndLine: 5},
			Intent:   HunkIntentRevert, Rationale: "undo",
		})
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Diffs: []ExplainedDiff{d},
	})
	require.NoError(t, err)
	hasConflict := false
	for _, f := range rep.Findings {
		if f.Kind == InconsistencyKindContradictoryIntents {
			hasConflict = true
			assert.Equal(t, "x.go", f.Subject)
		}
	}
	assert.True(t, hasConflict)
}

func TestCoherence_DetectsOrphanedDecisions(t *testing.T) {
	dec := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "Adopt React",
		Choice: "React 18", Rationale: "team", Status: DecisionStatusAccepted,
		RecordedAt: time.Now(),
	}
	// Diff exists but doesn't link to the decision.
	d := *NewExplainedDiff("PR-1", "x").AddBugFix("foo.js", 1, 1, "x")
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{dec},
		Diffs:     []ExplainedDiff{d},
	})
	require.NoError(t, err)
	hasOrphan := false
	for _, f := range rep.Findings {
		if f.Kind == InconsistencyKindOrphanedDecisions {
			hasOrphan = true
			assert.Equal(t, "Adopt React", f.Subject)
			assert.Equal(t, ReviewSeverityInfo, f.Severity)
		}
	}
	assert.True(t, hasOrphan, "decision not linked by any diff = orphaned")
}

func TestCoherence_LinkedDecisionsAreNotOrphaned(t *testing.T) {
	dec := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "Adopt React",
		Choice: "React 18", Rationale: "team", Status: DecisionStatusAccepted,
	}
	d := *NewExplainedDiff("PR-1", "x").
		AddBugFix("foo.js", 1, 1, "x", dec.ID.String())
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{dec},
		Diffs:     []ExplainedDiff{d},
	})
	require.NoError(t, err)
	for _, f := range rep.Findings {
		assert.NotEqual(t, InconsistencyKindOrphanedDecisions, f.Kind)
	}
}

func TestCoherence_FindingsSortedSeverityFirst(t *testing.T) {
	rep := CoherenceReport{
		Findings: []InconsistencyFinding{
			{Kind: InconsistencyKindOrphanedDecisions, Severity: ReviewSeverityInfo, LastSeenAt: time.Now()},
			{Kind: InconsistencyKindContradictoryDecisions, Severity: ReviewSeverityCritical, LastSeenAt: time.Now()},
			{Kind: InconsistencyKindRepeatedReviewFocus, Severity: ReviewSeverityWarn, LastSeenAt: time.Now()},
		},
	}
	// Mimic the build sort.
	d1 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "x",
		Choice: "A", Rationale: "x", Status: DecisionStatusAccepted,
	}
	d2 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "x",
		Choice: "B", Rationale: "y", Status: DecisionStatusAccepted,
	}
	mkReview := func(file string, id string) ReviewGuidance {
		r := *NewReviewGuidance(id, "code_diff", "x")
		r.FocusPoints = []ReviewFocusPoint{
			{Category: ReviewCategorySecurity, Location: file,
				Severity: ReviewSeverityCritical, Reason: "x"},
		}
		return r
	}
	got, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{d1, d2},
		Reviews:   []ReviewGuidance{mkReview("a", "PR-1"), mkReview("a", "PR-2"), mkReview("a", "PR-3")},
	})
	_ = rep // unused alias
	require.NoError(t, err)
	require.NotEmpty(t, got.Findings)
	assert.Equal(t, ReviewSeverityCritical, got.Findings[0].Severity,
		"after sort, critical leads")
}

func TestCoherence_HasCriticalFinding(t *testing.T) {
	rep := CoherenceReport{
		Findings: []InconsistencyFinding{
			{Severity: ReviewSeverityWarn},
		},
	}
	assert.False(t, rep.HasCriticalFinding())

	rep.Findings = append(rep.Findings, InconsistencyFinding{Severity: ReviewSeverityCritical})
	assert.True(t, rep.HasCriticalFinding())
}

func TestCoherence_CountByKind_AlwaysAllKinds(t *testing.T) {
	rep := CoherenceReport{
		Findings: []InconsistencyFinding{
			{Kind: InconsistencyKindContradictoryDecisions},
			{Kind: InconsistencyKindContradictoryDecisions},
			{Kind: InconsistencyKindOrphanedDecisions},
		},
	}
	hist := rep.CountByKind()
	for _, k := range AllInconsistencyKinds() {
		_, ok := hist[k]
		assert.True(t, ok, "kind %q axis must always exist", k)
	}
	assert.Equal(t, 2, hist[InconsistencyKindContradictoryDecisions])
	assert.Equal(t, 1, hist[InconsistencyKindOrphanedDecisions])
	assert.Equal(t, 0, hist[InconsistencyKindRepeatedReviewFocus])
}

func TestCoherence_JSON_RoundTrip(t *testing.T) {
	rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{
			{ID: uuid.New(), TenantID: "t", Title: "x", Choice: "A", Rationale: "y",
				Status: DecisionStatusAccepted},
		},
	})
	jsonStr, err := rep.JSON()
	require.NoError(t, err)
	var rt CoherenceReport
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &rt))
	assert.Equal(t, rep.TenantID, rt.TenantID)
	assert.Equal(t, rep.DecisionsRecorded, rt.DecisionsRecorded)
}

func TestCoherence_HumanSummary_NoFindingsExplicit(t *testing.T) {
	rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{})
	out := rep.HumanSummary()
	assert.Contains(t, out, "No inconsistencies detected",
		"silence is dangerous — explicit zero")
}

func TestCoherence_HumanSummary_ListsFindingsTopTen(t *testing.T) {
	d1 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "x", Choice: "A", Rationale: "y",
		Status: DecisionStatusAccepted,
	}
	d2 := DecisionRecord{
		ID: uuid.New(), TenantID: "t", Title: "x", Choice: "B", Rationale: "z",
		Status: DecisionStatusAccepted,
	}
	rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{d1, d2},
	})
	out := rep.HumanSummary()
	assert.Contains(t, out, "Top findings")
	assert.Contains(t, out, "[critical]")
	assert.Contains(t, out, "contradictory_decisions")
}

func TestCoherence_VolumeCountsArePopulated(t *testing.T) {
	rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Decisions: []DecisionRecord{
			{ID: uuid.New(), TenantID: "t", Title: "x", Choice: "A", Rationale: "y", Status: DecisionStatusAccepted},
		},
		Reviews: []ReviewGuidance{*NewReviewGuidance("PR-1", "x", "y")},
		Diffs:   []ExplainedDiff{*NewExplainedDiff("PR-1", "x")},
	})
	assert.Equal(t, 1, rep.DecisionsRecorded)
	assert.Equal(t, 1, rep.ReviewsAnnotated)
	assert.Equal(t, 1, rep.DiffsExplained)
}

func TestCoherence_NoSilentDataLossWithEmptyInputs(t *testing.T) {
	rep, err := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{})
	require.NoError(t, err)
	assert.Equal(t, 0, rep.DecisionsRecorded)
	assert.Empty(t, rep.Findings)
	assert.False(t, rep.HasCriticalFinding())
}

func TestCoherence_LocationStrippingNormalizesFileName(t *testing.T) {
	// Reviews with "auth.go:42" and "auth.go#section-3" must be grouped
	// under "auth.go" for repeated-focus detection.
	r1 := *NewReviewGuidance("PR-1", "x", "y")
	r1.FocusPoints = []ReviewFocusPoint{
		{Category: ReviewCategorySecurity, Location: "auth.go:42", Severity: ReviewSeverityCritical, Reason: "x"},
	}
	r2 := *NewReviewGuidance("PR-2", "x", "y")
	r2.FocusPoints = []ReviewFocusPoint{
		{Category: ReviewCategorySecurity, Location: "auth.go#section-3", Severity: ReviewSeverityCritical, Reason: "x"},
	}
	r3 := *NewReviewGuidance("PR-3", "x", "y")
	r3.FocusPoints = []ReviewFocusPoint{
		{Category: ReviewCategorySecurity, Location: "auth.go", Severity: ReviewSeverityCritical, Reason: "x"},
	}
	rep, _ := BuildCoherenceReport("t", validCoherencePeriod(), CoherenceReportInputs{
		Reviews: []ReviewGuidance{r1, r2, r3},
	})
	hasFocus := false
	for _, f := range rep.Findings {
		if f.Kind == InconsistencyKindRepeatedReviewFocus {
			hasFocus = true
			assert.Equal(t, "auth.go", f.Subject)
		}
	}
	assert.True(t, hasFocus, "location stripping groups across line/section refs")
}

func TestCoherence_HumanSummaryFormatsPeriod(t *testing.T) {
	rep, _ := BuildCoherenceReport("acme", validCoherencePeriod(), CoherenceReportInputs{})
	out := rep.HumanSummary()
	assert.Contains(t, out, "Coherence report — tenant acme")
	assert.True(t, strings.Contains(out, "Period:"))
}
