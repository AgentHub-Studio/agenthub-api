package agentic

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExplainedDiff_IntentEnumIsBounded(t *testing.T) {
	for _, i := range AllHunkIntents() {
		assert.True(t, IsValidHunkIntent(i))
	}
	assert.False(t, IsValidHunkIntent(HunkIntent("unknown")))
	assert.False(t, IsValidHunkIntent(""))
}

func TestExplainedDiff_AllIntentsCount(t *testing.T) {
	// 9 intents covers the common diff purpose categories.
	assert.Equal(t, 9, len(AllHunkIntents()))
}

func TestExplainedDiff_HunkLocation_StringFormatSingleLine(t *testing.T) {
	h := HunkLocation{File: "x.go", StartLine: 5, EndLine: 5}
	assert.Equal(t, "x.go:5", h.String())
}

func TestExplainedDiff_HunkLocation_StringFormatRange(t *testing.T) {
	h := HunkLocation{File: "x.go", StartLine: 5, EndLine: 12}
	assert.Equal(t, "x.go:5-12", h.String())
}

func TestExplainedDiff_NewSetsRequiredFields(t *testing.T) {
	d := NewExplainedDiff("PR-42", "auth fix")
	assert.Equal(t, "PR-42", d.ArtifactID)
	assert.Equal(t, "auth fix", d.Summary)
	assert.False(t, d.CreatedAt.IsZero())
	assert.Empty(t, d.ExplainedHunks)
}

func TestExplainedDiff_AddHunk_AppendsAndChains(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "a.go", StartLine: 1, EndLine: 5},
			Intent:   HunkIntentBugFix, Rationale: "fixed null check",
		}).
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "b.go", StartLine: 10, EndLine: 12},
			Intent:   HunkIntentTestAdded, Rationale: "covers null branch",
		})
	assert.Len(t, d.ExplainedHunks, 2)
}

func TestExplainedDiff_AddHunk_RejectsInvalidIntentSilently(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "a.go", StartLine: 1, EndLine: 1},
			Intent:   HunkIntent("typo"),
		})
	assert.Empty(t, d.ExplainedHunks)
}

func TestExplainedDiff_AddHunk_RejectsEmptyFileSilently(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddHunk(ExplainedHunk{
			Intent: HunkIntentBugFix, Rationale: "x",
		})
	assert.Empty(t, d.ExplainedHunks)
}

func TestExplainedDiff_AddHunk_TruncatesLongRationale(t *testing.T) {
	long := strings.Repeat("x", 800)
	d := NewExplainedDiff("x", "y").
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "a.go", StartLine: 1, EndLine: 1},
			Intent:   HunkIntentBugFix, Rationale: long,
		})
	assert.Equal(t, 500, len(d.ExplainedHunks[0].Rationale))
}

func TestExplainedDiff_AddHunk_PopulatesAtIfZero(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddHunk(ExplainedHunk{
			Location: HunkLocation{File: "a.go", StartLine: 1, EndLine: 1},
			Intent:   HunkIntentBugFix,
		})
	assert.False(t, d.ExplainedHunks[0].At.IsZero())
}

func TestExplainedDiff_AddBugFix_HasCorrectIntent(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddBugFix("auth.go", 42, 50, "null check", "TICKET-1")
	assert.Equal(t, HunkIntentBugFix, d.ExplainedHunks[0].Intent)
	assert.Equal(t, []string{"TICKET-1"}, d.ExplainedHunks[0].LinkedRequirements)
}

func TestExplainedDiff_AddSecurityPatch_HasCorrectIntent(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddSecurityPatch("auth.go", 100, 110, "CVE-2026-X", "CVE-2026-X", "ADR-007")
	assert.Equal(t, HunkIntentSecurityPatch, d.ExplainedHunks[0].Intent)
	assert.Len(t, d.ExplainedHunks[0].LinkedRequirements, 2)
}

func TestExplainedDiff_AddRefactor_HasCorrectIntent(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddRefactor("svc.go", 1, 200, "split into smaller methods")
	assert.Equal(t, HunkIntentRefactor, d.ExplainedHunks[0].Intent)
}

func TestExplainedDiff_AddTestAdded_HasCorrectIntent(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddTestAdded("auth_test.go", 50, 75, "covers MFA flow")
	assert.Equal(t, HunkIntentTestAdded, d.ExplainedHunks[0].Intent)
}

func TestExplainedDiff_CountByIntent_AlwaysAllIntents(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddBugFix("a.go", 1, 1, "x").
		AddBugFix("b.go", 1, 1, "x").
		AddTestAdded("c_test.go", 1, 1, "x")
	hist := d.CountByIntent()
	for _, i := range AllHunkIntents() {
		_, ok := hist[i]
		assert.True(t, ok, "intent %q must always exist in histogram", i)
	}
	assert.Equal(t, 2, hist[HunkIntentBugFix])
	assert.Equal(t, 1, hist[HunkIntentTestAdded])
	assert.Equal(t, 0, hist[HunkIntentSecurityPatch])
}

func TestExplainedDiff_HunksByFile_GroupsAndSortsByLine(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddBugFix("a.go", 50, 55, "x").
		AddBugFix("a.go", 10, 15, "y").
		AddBugFix("b.go", 1, 5, "z")
	byFile := d.HunksByFile()
	assert.Len(t, byFile["a.go"], 2)
	assert.Equal(t, 10, byFile["a.go"][0].Location.StartLine,
		"hunks within file sorted by StartLine ascending")
	assert.Equal(t, 50, byFile["a.go"][1].Location.StartLine)
	assert.Len(t, byFile["b.go"], 1)
}

func TestExplainedDiff_FilesAffected_SortedUnique(t *testing.T) {
	d := NewExplainedDiff("x", "y").
		AddBugFix("zeta.go", 1, 1, "x").
		AddBugFix("alpha.go", 1, 1, "y").
		AddBugFix("alpha.go", 10, 10, "z")
	files := d.FilesAffected()
	assert.Equal(t, []string{"alpha.go", "zeta.go"}, files)
}

func TestExplainedDiff_HasIntent(t *testing.T) {
	d := NewExplainedDiff("x", "y").AddBugFix("a", 1, 1, "x")
	assert.True(t, d.HasIntent(HunkIntentBugFix))
	assert.False(t, d.HasIntent(HunkIntentSecurityPatch))
}

func TestExplainedDiff_PlainText_Format(t *testing.T) {
	d := NewExplainedDiff("PR-42", "auth fix").
		AddBugFix("a.go", 1, 1, "x").
		AddBugFix("b.go", 5, 5, "y")
	out := d.PlainText()
	assert.Contains(t, out, "[DIFF]")
	assert.Contains(t, out, "PR-42")
	assert.Contains(t, out, "2 hunks across 2 files")
}

func TestExplainedDiff_Markdown_Format(t *testing.T) {
	d := NewExplainedDiff("PR-42", "auth fix").
		AddBugFix("auth.go", 42, 50, "null check", "TICKET-1").
		AddTestAdded("auth_test.go", 100, 150, "covers null branch")
	out := d.Markdown()
	assert.Contains(t, out, "## Explained Diff:")
	assert.Contains(t, out, "`PR-42`")
	assert.Contains(t, out, "#### `auth.go`")
	assert.Contains(t, out, "[bug_fix]")
	assert.Contains(t, out, "Linked: TICKET-1")
	assert.Contains(t, out, "[test_added]")
}

func TestExplainedDiff_Markdown_NoHunksMessage(t *testing.T) {
	d := NewExplainedDiff("PR-1", "trivial fix")
	out := d.Markdown()
	assert.Contains(t, out, "No annotated hunks")
}

func TestExplainedDiff_JSON_RoundTrip(t *testing.T) {
	d := NewExplainedDiff("PR-X", "patch").
		AddSecurityPatch("auth.go", 10, 20, "CVE", "CVE-2026-1")
	jsonStr, err := d.JSON()
	assert.NoError(t, err)
	var rt ExplainedDiff
	assert.NoError(t, json.Unmarshal([]byte(jsonStr), &rt))
	assert.Equal(t, "PR-X", rt.ArtifactID)
	assert.Len(t, rt.ExplainedHunks, 1)
	assert.Equal(t, HunkIntentSecurityPatch, rt.ExplainedHunks[0].Intent)
}

func TestExplainedDiff_AtTimestampPopulated(t *testing.T) {
	before := time.Now()
	d := NewExplainedDiff("x", "y").AddBugFix("a", 1, 1, "x")
	assert.True(t, d.ExplainedHunks[0].At.After(before) || d.ExplainedHunks[0].At.Equal(before))
}
