package agentic

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// HUMAN-005 — Codebase coherence report.
//
// PDF arXiv:2604.14228v1 §11 (over time, agent decisions can drift —
// contradictory choices made by different runs accumulate as silent
// debt; humans need a periodic report that surfaces these); §6.1.
//
// Builds on:
//   - HUMAN-001 ReviewGuidance — repeated focus points on same file = signal
//   - HUMAN-002 ExplainedDiff — contradictory intents on same file = signal
//   - HUMAN-003 DecisionRecord — non-superseded contradictory decisions = drift
//
// CoherenceReport scans an input window and surfaces InconsistencyFindings
// the human reviewer should resolve.
//
// Distinction from GOV-005 GovernanceReport:
//   - GOV-005 = compliance/audit summary (counts + flagged runs)
//   - HUMAN-005 = drift/coherence summary (cross-signal inconsistencies)

// InconsistencyKind bounded enum classifies the detected drift.
type InconsistencyKind string

const (
	// InconsistencyKindContradictoryDecisions — two decisions point at
	// different choices for the same topic without a supersede chain.
	InconsistencyKindContradictoryDecisions InconsistencyKind = "contradictory_decisions"
	// InconsistencyKindRepeatedReviewFocus — same file/section flagged
	// multiple times in review across different artifacts (signals a
	// hot-spot needing structural fix).
	InconsistencyKindRepeatedReviewFocus InconsistencyKind = "repeated_review_focus"
	// InconsistencyKindContradictoryIntents — same file edited multiple
	// times with conflicting intents (e.g. refactor then revert then refactor).
	InconsistencyKindContradictoryIntents InconsistencyKind = "contradictory_intents"
	// InconsistencyKindOrphanedDecisions — decision recorded but never
	// referenced by subsequent diffs (was it actually implemented?).
	InconsistencyKindOrphanedDecisions InconsistencyKind = "orphaned_decisions"
)

// allInconsistencyKinds is the closed bounded set.
var allInconsistencyKinds = []InconsistencyKind{
	InconsistencyKindContradictoryDecisions,
	InconsistencyKindRepeatedReviewFocus,
	InconsistencyKindContradictoryIntents,
	InconsistencyKindOrphanedDecisions,
}

// IsValidInconsistencyKind returns true for the bounded set.
func IsValidInconsistencyKind(k InconsistencyKind) bool {
	for _, v := range allInconsistencyKinds {
		if k == v {
			return true
		}
	}
	return false
}

// AllInconsistencyKinds returns a copy of the bounded set.
func AllInconsistencyKinds() []InconsistencyKind {
	out := make([]InconsistencyKind, len(allInconsistencyKinds))
	copy(out, allInconsistencyKinds)
	return out
}

// InconsistencyFinding is a single detected drift.
type InconsistencyFinding struct {
	// Kind classifies the finding.
	Kind InconsistencyKind `json:"kind"`
	// Subject identifies WHAT drifted (file path, decision topic,
	// review section). Stable string for grouping.
	Subject string `json:"subject"`
	// Evidence is the list of related IDs (decision IDs, diff IDs,
	// guidance IDs) that constitute the drift.
	Evidence []string `json:"evidence"`
	// Description is a one-line human-readable summary (≤ 300 chars).
	Description string `json:"description"`
	// Severity classifies remediation urgency.
	Severity ReviewSeverity `json:"severity"`
	// FirstSeenAt is the wall-clock of the earliest evidence.
	FirstSeenAt time.Time `json:"firstSeenAt"`
	// LastSeenAt is the wall-clock of the most recent evidence.
	LastSeenAt time.Time `json:"lastSeenAt"`
}

// CoherenceReport is the aggregation envelope.
type CoherenceReport struct {
	TenantID    string       `json:"tenantId"`
	Period      ReportPeriod `json:"period"`
	GeneratedAt time.Time    `json:"generatedAt"`

	// Summary counts:
	DecisionsRecorded int `json:"decisionsRecorded"`
	ReviewsAnnotated  int `json:"reviewsAnnotated"`
	DiffsExplained    int `json:"diffsExplained"`

	// Findings is the ordered (severity DESC, then LastSeenAt DESC)
	// list of detected inconsistencies.
	Findings []InconsistencyFinding `json:"findings,omitempty"`
}

// ErrInvalidCoherencePeriod — defensive guard.
var ErrInvalidCoherencePeriod = errors.New("coherence: invalid period")

// ErrEmptyCoherenceTenantID — defensive guard.
var ErrEmptyCoherenceTenantID = errors.New("coherence: tenantId required")

// CoherenceReportInputs is the bundle of pre-filtered data the builder
// scans. Caller filters by tenant + period.
type CoherenceReportInputs struct {
	Decisions []DecisionRecord
	Reviews   []ReviewGuidance
	Diffs     []ExplainedDiff
}

// BuildCoherenceReport scans inputs for drift signals and returns a
// CoherenceReport. Pure function.
func BuildCoherenceReport(tenantID string, period ReportPeriod, in CoherenceReportInputs) (CoherenceReport, error) {
	if tenantID == "" {
		return CoherenceReport{}, ErrEmptyCoherenceTenantID
	}
	if !period.IsValid() {
		return CoherenceReport{}, ErrInvalidCoherencePeriod
	}

	rep := CoherenceReport{
		TenantID:          tenantID,
		Period:            period,
		GeneratedAt:       time.Now(),
		DecisionsRecorded: len(in.Decisions),
		ReviewsAnnotated:  len(in.Reviews),
		DiffsExplained:    len(in.Diffs),
	}

	rep.Findings = append(rep.Findings, detectContradictoryDecisions(in.Decisions)...)
	rep.Findings = append(rep.Findings, detectRepeatedReviewFocus(in.Reviews)...)
	rep.Findings = append(rep.Findings, detectContradictoryIntents(in.Diffs)...)
	rep.Findings = append(rep.Findings, detectOrphanedDecisions(in.Decisions, in.Diffs)...)

	// Sort findings: severity DESC then LastSeenAt DESC.
	sevOrder := map[ReviewSeverity]int{
		ReviewSeverityCritical: 0,
		ReviewSeverityWarn:     1,
		ReviewSeverityInfo:     2,
	}
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		ai, aj := sevOrder[rep.Findings[i].Severity], sevOrder[rep.Findings[j].Severity]
		if ai != aj {
			return ai < aj
		}
		return rep.Findings[i].LastSeenAt.After(rep.Findings[j].LastSeenAt)
	})

	return rep, nil
}

// detectContradictoryDecisions surfaces decisions on the same Title
// that are both Accepted and not superseded — clear contradiction.
func detectContradictoryDecisions(decisions []DecisionRecord) []InconsistencyFinding {
	byTitle := map[string][]DecisionRecord{}
	for _, d := range decisions {
		if d.Status == DecisionStatusAccepted && d.SupersededBy == nil {
			byTitle[d.Title] = append(byTitle[d.Title], d)
		}
	}
	var findings []InconsistencyFinding
	for title, group := range byTitle {
		if len(group) < 2 {
			continue
		}
		// Multiple ACCEPTED decisions on the same title without
		// supersede chain = contradiction.
		ids := make([]string, 0, len(group))
		first := group[0].RecordedAt
		last := group[0].RecordedAt
		for _, d := range group {
			ids = append(ids, d.ID.String())
			if d.RecordedAt.Before(first) {
				first = d.RecordedAt
			}
			if d.RecordedAt.After(last) {
				last = d.RecordedAt
			}
		}
		sort.Strings(ids)
		findings = append(findings, InconsistencyFinding{
			Kind:        InconsistencyKindContradictoryDecisions,
			Subject:     title,
			Evidence:    ids,
			Description: fmt.Sprintf("%d accepted decisions on %q without supersede chain", len(group), title),
			Severity:    ReviewSeverityCritical,
			FirstSeenAt: first,
			LastSeenAt:  last,
		})
	}
	return findings
}

// detectRepeatedReviewFocus surfaces files flagged in ≥3 distinct
// reviews across the period — hot-spot signal.
func detectRepeatedReviewFocus(reviews []ReviewGuidance) []InconsistencyFinding {
	byLocation := map[string][]ReviewGuidance{}
	for _, r := range reviews {
		seen := map[string]bool{}
		for _, fp := range r.FocusPoints {
			// Normalize location (strip line numbers).
			file := fp.Location
			if idx := strings.IndexAny(file, ":#"); idx > 0 {
				file = file[:idx]
			}
			if seen[file] {
				continue // dedup per review
			}
			seen[file] = true
			byLocation[file] = append(byLocation[file], r)
		}
	}
	var findings []InconsistencyFinding
	for file, group := range byLocation {
		if len(group) < 3 {
			continue
		}
		ids := make([]string, 0, len(group))
		first := group[0].CreatedAt
		last := group[0].CreatedAt
		for _, r := range group {
			ids = append(ids, r.ArtifactID)
			if r.CreatedAt.Before(first) {
				first = r.CreatedAt
			}
			if r.CreatedAt.After(last) {
				last = r.CreatedAt
			}
		}
		sort.Strings(ids)
		findings = append(findings, InconsistencyFinding{
			Kind:        InconsistencyKindRepeatedReviewFocus,
			Subject:     file,
			Evidence:    ids,
			Description: fmt.Sprintf("%s flagged in %d reviews — structural hot-spot", file, len(group)),
			Severity:    ReviewSeverityWarn,
			FirstSeenAt: first,
			LastSeenAt:  last,
		})
	}
	return findings
}

// detectContradictoryIntents surfaces files with conflicting intent
// pairs (e.g. revert + feature_add on same file = churn).
func detectContradictoryIntents(diffs []ExplainedDiff) []InconsistencyFinding {
	type intentEvidence struct {
		intent     HunkIntent
		artifactID string
		at         time.Time
	}
	byFile := map[string][]intentEvidence{}
	for _, d := range diffs {
		for _, h := range d.ExplainedHunks {
			byFile[h.Location.File] = append(byFile[h.Location.File], intentEvidence{
				intent: h.Intent, artifactID: d.ArtifactID, at: h.At,
			})
		}
	}
	conflictPairs := map[HunkIntent]HunkIntent{
		HunkIntentRevert:      HunkIntentFeatureAdd,
		HunkIntentFeatureAdd:  HunkIntentRevert,
		HunkIntentRefactor:    HunkIntentRevert,
	}
	var findings []InconsistencyFinding
	for file, evs := range byFile {
		intents := map[HunkIntent]bool{}
		ids := map[string]bool{}
		first := evs[0].at
		last := evs[0].at
		for _, e := range evs {
			intents[e.intent] = true
			ids[e.artifactID] = true
			if e.at.Before(first) {
				first = e.at
			}
			if e.at.After(last) {
				last = e.at
			}
		}
		hasConflict := false
		for a, b := range conflictPairs {
			if intents[a] && intents[b] {
				hasConflict = true
				break
			}
		}
		if !hasConflict {
			continue
		}
		idList := make([]string, 0, len(ids))
		for id := range ids {
			idList = append(idList, id)
		}
		sort.Strings(idList)
		findings = append(findings, InconsistencyFinding{
			Kind:        InconsistencyKindContradictoryIntents,
			Subject:     file,
			Evidence:    idList,
			Description: fmt.Sprintf("%s has conflicting intents (e.g. revert + feature_add) — churn signal", file),
			Severity:    ReviewSeverityWarn,
			FirstSeenAt: first,
			LastSeenAt:  last,
		})
	}
	return findings
}

// detectOrphanedDecisions surfaces decisions never referenced by any
// diff's LinkedRequirements — was the decision actually implemented?
func detectOrphanedDecisions(decisions []DecisionRecord, diffs []ExplainedDiff) []InconsistencyFinding {
	referenced := map[string]bool{}
	for _, d := range diffs {
		for _, h := range d.ExplainedHunks {
			for _, link := range h.LinkedRequirements {
				referenced[link] = true
			}
		}
	}
	var findings []InconsistencyFinding
	for _, dec := range decisions {
		if dec.Status != DecisionStatusAccepted {
			continue
		}
		if !referenced[dec.ID.String()] {
			findings = append(findings, InconsistencyFinding{
				Kind:        InconsistencyKindOrphanedDecisions,
				Subject:     dec.Title,
				Evidence:    []string{dec.ID.String()},
				Description: fmt.Sprintf("Decision %q (id %s) accepted but never referenced by any diff",
					dec.Title, dec.ID.String()),
				Severity:    ReviewSeverityInfo,
				FirstSeenAt: dec.RecordedAt,
				LastSeenAt:  dec.RecordedAt,
			})
		}
	}
	return findings
}

// HasCriticalFinding returns true when at least one finding is critical.
func (r CoherenceReport) HasCriticalFinding() bool {
	for _, f := range r.Findings {
		if f.Severity == ReviewSeverityCritical {
			return true
		}
	}
	return false
}

// CountByKind returns histogram (always all 4 kinds with 0 default).
func (r CoherenceReport) CountByKind() map[InconsistencyKind]int {
	hist := map[InconsistencyKind]int{}
	for _, k := range allInconsistencyKinds {
		hist[k] = 0
	}
	for _, f := range r.Findings {
		hist[f.Kind]++
	}
	return hist
}

// JSON renders wire-stable JSON.
func (r CoherenceReport) JSON() (string, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("coherence: marshal: %w", err)
	}
	return string(raw), nil
}

// HumanSummary renders multi-line summary for operator dashboard.
func (r CoherenceReport) HumanSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Coherence report — tenant %s\n", r.TenantID)
	fmt.Fprintf(&b, "Period: %s → %s\n",
		r.Period.Start.UTC().Format(time.RFC3339),
		r.Period.End.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "\nVolume: %d decisions / %d reviews / %d diffs\n",
		r.DecisionsRecorded, r.ReviewsAnnotated, r.DiffsExplained)

	if len(r.Findings) == 0 {
		b.WriteString("\nNo inconsistencies detected — codebase coherent for this window.\n")
		return b.String()
	}

	hist := r.CountByKind()
	fmt.Fprintf(&b, "\nFindings: %d total\n", len(r.Findings))
	for _, k := range allInconsistencyKinds {
		fmt.Fprintf(&b, "  %s: %d\n", k, hist[k])
	}

	b.WriteString("\nTop findings (severity-ordered):\n")
	max := len(r.Findings)
	if max > 10 {
		max = 10
	}
	for _, f := range r.Findings[:max] {
		fmt.Fprintf(&b, "  - [%s] [%s] %s — %s\n",
			f.Severity, f.Kind, f.Subject, f.Description)
	}
	return b.String()
}
