package agentic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// HUMAN-001 — Human review guidance.
//
// PDF arXiv:2604.14228v1 §11 (humans need guidance on WHAT to look at
// — raw diffs are noise, attention is the scarce resource); §6.1 (agent
// outputs need stable structure for downstream review).
//
// When an agent produces an artifact (code change, config mutation,
// doc edit, knowledge-base update), the reviewer should know:
//   1. WHAT category of change this is (security / correctness / etc.)
//   2. WHERE to look first (severity-ordered focus points)
//   3. WHY each focus matters (reason)
//   4. WHAT to do (recommended action)
//
// ReviewGuidance is the envelope. Renderers target three surfaces:
//   - PlainText: terminal / log line
//   - Markdown: chat UI / PR description
//   - JSON: SSE stream / audit log
//
// Distinction from GOV-004 PermissionExplanation:
//   - PermissionExplanation explains a runtime DECISION (allow/deny).
//   - ReviewGuidance explains an ARTIFACT (the result of work) and
//     directs human attention before they read the diff.

// ReviewCategory bounded enum classifies the kind of focus point.
// Stable strings — analytics aggregates by category.
type ReviewCategory string

const (
	// ReviewCategorySecurity — security-sensitive change (auth, secrets,
	// permission rules, encryption).
	ReviewCategorySecurity ReviewCategory = "security"
	// ReviewCategoryCorrectness — logic / behavior change that may
	// introduce bugs (algorithm, state mutation, edge cases).
	ReviewCategoryCorrectness ReviewCategory = "correctness"
	// ReviewCategoryStyle — code style / readability change (rename,
	// reformat, comment edit).
	ReviewCategoryStyle ReviewCategory = "style"
	// ReviewCategoryPerformance — performance-relevant change (loop,
	// query, allocation, cache).
	ReviewCategoryPerformance ReviewCategory = "performance"
	// ReviewCategoryTestCoverage — change to tests (added/removed/
	// modified test cases) — flag for "is this still covered?".
	ReviewCategoryTestCoverage ReviewCategory = "test_coverage"
	// ReviewCategoryPolicyCompliance — change that touches a regulated
	// surface (PII handling, audit log, retention policy).
	ReviewCategoryPolicyCompliance ReviewCategory = "policy_compliance"
)

// allReviewCategories is the closed bounded set.
var allReviewCategories = []ReviewCategory{
	ReviewCategorySecurity,
	ReviewCategoryCorrectness,
	ReviewCategoryStyle,
	ReviewCategoryPerformance,
	ReviewCategoryTestCoverage,
	ReviewCategoryPolicyCompliance,
}

// IsValidReviewCategory returns true for the bounded set.
func IsValidReviewCategory(c ReviewCategory) bool {
	for _, v := range allReviewCategories {
		if c == v {
			return true
		}
	}
	return false
}

// AllReviewCategories returns a copy of the bounded set (UI dropdowns).
func AllReviewCategories() []ReviewCategory {
	out := make([]ReviewCategory, len(allReviewCategories))
	copy(out, allReviewCategories)
	return out
}

// ReviewSeverity bounded enum.
type ReviewSeverity string

const (
	ReviewSeverityInfo     ReviewSeverity = "info"
	ReviewSeverityWarn     ReviewSeverity = "warn"
	ReviewSeverityCritical ReviewSeverity = "critical"
)

// IsValidReviewSeverity returns true for the bounded set.
func IsValidReviewSeverity(s ReviewSeverity) bool {
	switch s {
	case ReviewSeverityInfo, ReviewSeverityWarn, ReviewSeverityCritical:
		return true
	}
	return false
}

// ReviewFocusPoint is one direction of attention for the human reviewer.
type ReviewFocusPoint struct {
	// Category classifies the focus. Always in the bounded set.
	Category ReviewCategory `json:"category"`
	// Location identifies the artifact section (e.g. "internal/auth/jwt.go:42-58",
	// "config/secrets.yaml", "kb/docs/payment-flow.md#section-3").
	Location string `json:"location"`
	// Severity classifies the importance.
	Severity ReviewSeverity `json:"severity"`
	// Reason is why this section warrants attention (≤ 500 chars).
	Reason string `json:"reason"`
	// RecommendedAction is what the reviewer should DO (e.g. "verify
	// the new SQL parameter is bound, not interpolated"). Optional.
	RecommendedAction string `json:"recommendedAction,omitempty"`
	// At is when this focus was identified.
	At time.Time `json:"at"`
}

// ReviewGuidance is the unified attention-directing envelope.
type ReviewGuidance struct {
	// ArtifactID identifies what's being reviewed (e.g. "PR-42",
	// "agent-config-change-uuid", "doc-version-7").
	ArtifactID string `json:"artifactId"`
	// ArtifactKind classifies the artifact (e.g. "code_diff",
	// "config_change", "doc_edit", "agent_definition").
	ArtifactKind string `json:"artifactKind"`
	// Summary is the one-line description of what changed.
	Summary string `json:"summary"`
	// FocusPoints is the ordered list (by severity DESC then At ASC)
	// of attention directives.
	FocusPoints []ReviewFocusPoint `json:"focusPoints,omitempty"`
	// CreatedAt is the wall-clock timestamp.
	CreatedAt time.Time `json:"createdAt"`
}

// NewReviewGuidance creates a guidance envelope. ArtifactID + Summary
// are mandatory — the contract is "no review without a what-and-why".
func NewReviewGuidance(artifactID, artifactKind, summary string) *ReviewGuidance {
	return &ReviewGuidance{
		ArtifactID:   artifactID,
		ArtifactKind: artifactKind,
		Summary:      summary,
		CreatedAt:    time.Now(),
	}
}

// AddFocus appends a focus point. Builder returns *g for chaining.
// Invalid category/severity values are silently dropped.
func (g *ReviewGuidance) AddFocus(p ReviewFocusPoint) *ReviewGuidance {
	if !IsValidReviewCategory(p.Category) {
		return g
	}
	if !IsValidReviewSeverity(p.Severity) {
		return g
	}
	if p.At.IsZero() {
		p.At = time.Now()
	}
	if len(p.Reason) > 500 {
		p.Reason = p.Reason[:497] + "..."
	}
	g.FocusPoints = append(g.FocusPoints, p)
	return g
}

// AddSecurityFocus is a convenience for security-category focus points.
// Defaults to critical severity (security defaults to high attention).
func (g *ReviewGuidance) AddSecurityFocus(location, reason, action string) *ReviewGuidance {
	return g.AddFocus(ReviewFocusPoint{
		Category:          ReviewCategorySecurity,
		Location:          location,
		Severity:          ReviewSeverityCritical,
		Reason:            reason,
		RecommendedAction: action,
	})
}

// AddCorrectnessFocus is a convenience for correctness-category focus.
// Defaults to warn severity.
func (g *ReviewGuidance) AddCorrectnessFocus(location, reason, action string) *ReviewGuidance {
	return g.AddFocus(ReviewFocusPoint{
		Category:          ReviewCategoryCorrectness,
		Location:          location,
		Severity:          ReviewSeverityWarn,
		Reason:            reason,
		RecommendedAction: action,
	})
}

// AddPerformanceFocus is a convenience for performance-category focus.
// Defaults to warn severity.
func (g *ReviewGuidance) AddPerformanceFocus(location, reason, action string) *ReviewGuidance {
	return g.AddFocus(ReviewFocusPoint{
		Category:          ReviewCategoryPerformance,
		Location:          location,
		Severity:          ReviewSeverityWarn,
		Reason:            reason,
		RecommendedAction: action,
	})
}

// AddPolicyComplianceFocus is a convenience for compliance focus.
// Defaults to critical severity (compliance is non-negotiable).
func (g *ReviewGuidance) AddPolicyComplianceFocus(location, reason, action string) *ReviewGuidance {
	return g.AddFocus(ReviewFocusPoint{
		Category:          ReviewCategoryPolicyCompliance,
		Location:          location,
		Severity:          ReviewSeverityCritical,
		Reason:            reason,
		RecommendedAction: action,
	})
}

// AddTestCoverageFocus is a convenience for test-coverage focus.
// Defaults to info severity.
func (g *ReviewGuidance) AddTestCoverageFocus(location, reason, action string) *ReviewGuidance {
	return g.AddFocus(ReviewFocusPoint{
		Category:          ReviewCategoryTestCoverage,
		Location:          location,
		Severity:          ReviewSeverityInfo,
		Reason:            reason,
		RecommendedAction: action,
	})
}

// AddStyleFocus is a convenience for style-category focus.
// Defaults to info severity.
func (g *ReviewGuidance) AddStyleFocus(location, reason, action string) *ReviewGuidance {
	return g.AddFocus(ReviewFocusPoint{
		Category:          ReviewCategoryStyle,
		Location:          location,
		Severity:          ReviewSeverityInfo,
		Reason:            reason,
		RecommendedAction: action,
	})
}

// SortBySeverity orders focus points by severity DESC (critical first)
// then At ASC. Renderers call this to lead with the most important.
func (g *ReviewGuidance) SortBySeverity() {
	sevOrder := map[ReviewSeverity]int{
		ReviewSeverityCritical: 0,
		ReviewSeverityWarn:     1,
		ReviewSeverityInfo:     2,
	}
	sort.SliceStable(g.FocusPoints, func(i, j int) bool {
		ai, aj := sevOrder[g.FocusPoints[i].Severity], sevOrder[g.FocusPoints[j].Severity]
		if ai != aj {
			return ai < aj
		}
		return g.FocusPoints[i].At.Before(g.FocusPoints[j].At)
	})
}

// HasCriticalFocus returns true when at least one focus is critical.
// Used by review-routing code to escalate to senior reviewers.
func (g *ReviewGuidance) HasCriticalFocus() bool {
	for _, f := range g.FocusPoints {
		if f.Severity == ReviewSeverityCritical {
			return true
		}
	}
	return false
}

// CountByCategory returns histogram of focus counts grouped by
// category. Always includes ALL bounded categories with 0 default —
// dashboards never need conditional logic.
func (g *ReviewGuidance) CountByCategory() map[ReviewCategory]int {
	hist := map[ReviewCategory]int{}
	for _, c := range allReviewCategories {
		hist[c] = 0
	}
	for _, f := range g.FocusPoints {
		hist[f.Category]++
	}
	return hist
}

// PlainText renders a single-line summary suitable for terminals / log lines.
//
// Format: "[REVIEW] <artifactKind> <artifactID>: <summary> (N focus points, K critical)"
func (g *ReviewGuidance) PlainText() string {
	critical := 0
	for _, f := range g.FocusPoints {
		if f.Severity == ReviewSeverityCritical {
			critical++
		}
	}
	return fmt.Sprintf("[REVIEW] %s %s: %s (%d focus points, %d critical)",
		g.ArtifactKind, g.ArtifactID, g.Summary, len(g.FocusPoints), critical)
}

// Markdown renders a multi-line markdown block suitable for chat UI / PR descriptions.
//
// Format:
//   ## Review Guidance: <artifactKind> `<artifactID>`
//   <summary>
//
//   ### Focus points (N — K critical)
//   - **[<severity>] [<category>]** `<location>` — <reason>
//     - Action: <recommendedAction>
func (g *ReviewGuidance) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Review Guidance: %s `%s`\n\n", g.ArtifactKind, g.ArtifactID)
	fmt.Fprintf(&b, "%s\n", g.Summary)

	if len(g.FocusPoints) == 0 {
		b.WriteString("\n_No specific focus points — review the full artifact._\n")
		return b.String()
	}
	critical := 0
	for _, f := range g.FocusPoints {
		if f.Severity == ReviewSeverityCritical {
			critical++
		}
	}
	fmt.Fprintf(&b, "\n### Focus points (%d — %d critical)\n", len(g.FocusPoints), critical)
	for _, f := range g.FocusPoints {
		fmt.Fprintf(&b, "- **[%s] [%s]** `%s` — %s\n",
			f.Severity, f.Category, f.Location, f.Reason)
		if f.RecommendedAction != "" {
			fmt.Fprintf(&b, "  - Action: %s\n", f.RecommendedAction)
		}
	}
	return b.String()
}

// JSON renders as wire-stable JSON for SSE / audit log.
func (g *ReviewGuidance) JSON() (string, error) {
	raw, err := json.Marshal(g)
	if err != nil {
		return "", fmt.Errorf("review guidance: marshal: %w", err)
	}
	return string(raw), nil
}
