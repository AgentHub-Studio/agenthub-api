package agentic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// HUMAN-002 — Explainable diffs.
//
// PDF arXiv:2604.14228v1 §11 (raw diffs are opaque — humans need
// intent-level annotation to review effectively); §6.1 (agent outputs
// need stable structure for downstream consumption).
//
// Builds on HUMAN-001 ReviewGuidance (directs attention to where).
// HUMAN-002 annotates EACH HUNK with WHY it changed (intent + rationale)
// and OPTIONALLY links it back to ReviewGuidance focus points or external
// requirements (ticket IDs, ADR references).
//
// The reviewer reads:
//   1. ExplainedDiff.Summary — one-line "what this PR does"
//   2. Per hunk: file:lines + Intent badge + Rationale
//   3. Optional links to focus points / tickets / ADRs
//
// Distinction from neighbouring abstractions:
//   - HUMAN-001 ReviewGuidance directs WHERE to look (severity-ordered).
//   - HUMAN-002 ExplainedDiff annotates WHAT each hunk does (intent-tagged).
//   - GOV-004 PermissionExplanation explains a runtime DECISION.

// HunkIntent bounded enum classifies the purpose of a diff hunk.
// Stable strings — analytics aggregate by intent.
type HunkIntent string

const (
	// HunkIntentBugFix — fixes a defect.
	HunkIntentBugFix HunkIntent = "bug_fix"
	// HunkIntentFeatureAdd — introduces new capability.
	HunkIntentFeatureAdd HunkIntent = "feature_add"
	// HunkIntentRefactor — restructure without behavior change.
	HunkIntentRefactor HunkIntent = "refactor"
	// HunkIntentCleanup — remove dead code, format, simplify.
	HunkIntentCleanup HunkIntent = "cleanup"
	// HunkIntentSecurityPatch — addresses a vulnerability.
	HunkIntentSecurityPatch HunkIntent = "security_patch"
	// HunkIntentDependencyUpdate — bumps an external dependency.
	HunkIntentDependencyUpdate HunkIntent = "dependency_update"
	// HunkIntentTestAdded — new test coverage.
	HunkIntentTestAdded HunkIntent = "test_added"
	// HunkIntentDocUpdate — documentation change.
	HunkIntentDocUpdate HunkIntent = "doc_update"
	// HunkIntentRevert — undoes a previous change.
	HunkIntentRevert HunkIntent = "revert"
)

// allHunkIntents is the closed bounded set.
var allHunkIntents = []HunkIntent{
	HunkIntentBugFix,
	HunkIntentFeatureAdd,
	HunkIntentRefactor,
	HunkIntentCleanup,
	HunkIntentSecurityPatch,
	HunkIntentDependencyUpdate,
	HunkIntentTestAdded,
	HunkIntentDocUpdate,
	HunkIntentRevert,
}

// IsValidHunkIntent returns true for the bounded set.
func IsValidHunkIntent(i HunkIntent) bool {
	for _, v := range allHunkIntents {
		if i == v {
			return true
		}
	}
	return false
}

// AllHunkIntents returns a copy of the bounded set (UI dropdowns).
func AllHunkIntents() []HunkIntent {
	out := make([]HunkIntent, len(allHunkIntents))
	copy(out, allHunkIntents)
	return out
}

// HunkLocation identifies WHERE the hunk lives.
type HunkLocation struct {
	// File is the artifact file path (e.g. "internal/auth/jwt.go").
	File string `json:"file"`
	// StartLine is the first line of the hunk in the new file (1-based).
	StartLine int `json:"startLine"`
	// EndLine is the last line of the hunk in the new file (inclusive).
	EndLine int `json:"endLine"`
}

// String returns "file:start-end" for log/UI display.
func (h HunkLocation) String() string {
	if h.StartLine == h.EndLine {
		return fmt.Sprintf("%s:%d", h.File, h.StartLine)
	}
	return fmt.Sprintf("%s:%d-%d", h.File, h.StartLine, h.EndLine)
}

// ExplainedHunk is one hunk with intent + rationale + optional links.
type ExplainedHunk struct {
	// Location identifies where the hunk lives.
	Location HunkLocation `json:"location"`
	// Intent classifies the purpose. Always in the bounded set.
	Intent HunkIntent `json:"intent"`
	// Rationale is the WHY (≤ 500 chars). Required for any non-trivial intent.
	Rationale string `json:"rationale"`
	// LinkedRequirements is the optional list of ticket IDs / ADR refs /
	// focus point IDs that this hunk addresses.
	LinkedRequirements []string `json:"linkedRequirements,omitempty"`
	// At is when this annotation was made.
	At time.Time `json:"at"`
}

// ExplainedDiff is the full annotated artifact.
type ExplainedDiff struct {
	// ArtifactID identifies the diff (e.g. "PR-42", "agent-config-uuid").
	ArtifactID string `json:"artifactId"`
	// Summary is the one-line description of the entire diff.
	Summary string `json:"summary"`
	// ExplainedHunks is the ordered list of annotated hunks.
	// Default order is input order (preserves chronological flow).
	ExplainedHunks []ExplainedHunk `json:"explainedHunks,omitempty"`
	// CreatedAt is the wall-clock timestamp.
	CreatedAt time.Time `json:"createdAt"`
}

// NewExplainedDiff creates an envelope. ArtifactID + Summary required.
func NewExplainedDiff(artifactID, summary string) *ExplainedDiff {
	return &ExplainedDiff{
		ArtifactID: artifactID,
		Summary:    summary,
		CreatedAt:  time.Now(),
	}
}

// AddHunk appends a hunk explanation. Builder returns *d for chaining.
// Invalid intent / missing location are silently dropped.
func (d *ExplainedDiff) AddHunk(h ExplainedHunk) *ExplainedDiff {
	if !IsValidHunkIntent(h.Intent) {
		return d
	}
	if h.Location.File == "" {
		return d
	}
	if h.At.IsZero() {
		h.At = time.Now()
	}
	if len(h.Rationale) > 500 {
		h.Rationale = h.Rationale[:497] + "..."
	}
	d.ExplainedHunks = append(d.ExplainedHunks, h)
	return d
}

// AddBugFix is a convenience for bug-fix hunks.
func (d *ExplainedDiff) AddBugFix(file string, start, end int, rationale string, links ...string) *ExplainedDiff {
	return d.AddHunk(ExplainedHunk{
		Location:           HunkLocation{File: file, StartLine: start, EndLine: end},
		Intent:             HunkIntentBugFix,
		Rationale:          rationale,
		LinkedRequirements: links,
	})
}

// AddSecurityPatch is a convenience for security-patch hunks.
func (d *ExplainedDiff) AddSecurityPatch(file string, start, end int, rationale string, links ...string) *ExplainedDiff {
	return d.AddHunk(ExplainedHunk{
		Location:           HunkLocation{File: file, StartLine: start, EndLine: end},
		Intent:             HunkIntentSecurityPatch,
		Rationale:          rationale,
		LinkedRequirements: links,
	})
}

// AddRefactor is a convenience for refactor hunks.
func (d *ExplainedDiff) AddRefactor(file string, start, end int, rationale string, links ...string) *ExplainedDiff {
	return d.AddHunk(ExplainedHunk{
		Location:           HunkLocation{File: file, StartLine: start, EndLine: end},
		Intent:             HunkIntentRefactor,
		Rationale:          rationale,
		LinkedRequirements: links,
	})
}

// AddTestAdded is a convenience for test-added hunks.
func (d *ExplainedDiff) AddTestAdded(file string, start, end int, rationale string, links ...string) *ExplainedDiff {
	return d.AddHunk(ExplainedHunk{
		Location:           HunkLocation{File: file, StartLine: start, EndLine: end},
		Intent:             HunkIntentTestAdded,
		Rationale:          rationale,
		LinkedRequirements: links,
	})
}

// CountByIntent returns histogram of hunk counts grouped by intent.
// Always includes ALL bounded intents with 0 default — dashboards have
// stable axes.
func (d *ExplainedDiff) CountByIntent() map[HunkIntent]int {
	hist := map[HunkIntent]int{}
	for _, i := range allHunkIntents {
		hist[i] = 0
	}
	for _, h := range d.ExplainedHunks {
		hist[h.Intent]++
	}
	return hist
}

// HunksByFile groups hunks by file path. Useful for IDE/UI integration
// that renders changes file-by-file. Files sorted ascending.
func (d *ExplainedDiff) HunksByFile() map[string][]ExplainedHunk {
	byFile := map[string][]ExplainedHunk{}
	for _, h := range d.ExplainedHunks {
		byFile[h.Location.File] = append(byFile[h.Location.File], h)
	}
	// Sort hunks within each file by StartLine for stable rendering.
	for f := range byFile {
		sort.SliceStable(byFile[f], func(i, j int) bool {
			return byFile[f][i].Location.StartLine < byFile[f][j].Location.StartLine
		})
	}
	return byFile
}

// FilesAffected returns the sorted unique list of files touched.
func (d *ExplainedDiff) FilesAffected() []string {
	seen := map[string]bool{}
	for _, h := range d.ExplainedHunks {
		seen[h.Location.File] = true
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// HasIntent returns true when at least one hunk has the given intent.
// Useful for routing: if HasIntent(SecurityPatch) → escalate review.
func (d *ExplainedDiff) HasIntent(intent HunkIntent) bool {
	for _, h := range d.ExplainedHunks {
		if h.Intent == intent {
			return true
		}
	}
	return false
}

// PlainText renders a single-line summary suitable for terminals / log lines.
//
// Format: "[DIFF] <artifactID>: <summary> (N hunks across M files)"
func (d *ExplainedDiff) PlainText() string {
	return fmt.Sprintf("[DIFF] %s: %s (%d hunks across %d files)",
		d.ArtifactID, d.Summary, len(d.ExplainedHunks), len(d.FilesAffected()))
}

// Markdown renders a multi-line markdown block — by-file with intent badges.
//
// Format:
//   ## Explained Diff: `<artifactID>`
//   <summary>
//
//   ### Files
//   #### `<file>`
//   - **[<intent>]** `<file>:<lines>` — <rationale>
//     - Linked: <ticket1>, <ticket2>
func (d *ExplainedDiff) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Explained Diff: `%s`\n\n", d.ArtifactID)
	fmt.Fprintf(&b, "%s\n", d.Summary)

	if len(d.ExplainedHunks) == 0 {
		b.WriteString("\n_No annotated hunks — review the raw diff._\n")
		return b.String()
	}

	files := d.FilesAffected()
	byFile := d.HunksByFile()
	fmt.Fprintf(&b, "\n### Files (%d)\n", len(files))
	for _, file := range files {
		fmt.Fprintf(&b, "\n#### `%s`\n", file)
		for _, h := range byFile[file] {
			fmt.Fprintf(&b, "- **[%s]** `%s` — %s\n",
				h.Intent, h.Location.String(), h.Rationale)
			if len(h.LinkedRequirements) > 0 {
				fmt.Fprintf(&b, "  - Linked: %s\n",
					strings.Join(h.LinkedRequirements, ", "))
			}
		}
	}
	return b.String()
}

// JSON renders as wire-stable JSON for SSE / audit log.
func (d *ExplainedDiff) JSON() (string, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("explained diff: marshal: %w", err)
	}
	return string(raw), nil
}
