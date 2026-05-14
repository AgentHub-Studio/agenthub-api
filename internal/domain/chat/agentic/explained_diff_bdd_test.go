package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HUMAN-002 — Explainable diffs BDD.
//
// PDF arXiv:2604.14228v1 §11 (raw diffs are opaque — humans need
// intent-level annotation); §6.1 (agent outputs need stable structure).
//
// These scenarios validate the contract: bounded intents, file-grouped
// rendering, requirement linking, multi-format output.

func TestBDD_ExplainableDiffs(t *testing.T) {

	t.Run("Scenario_AgentAnnotatesHunkWithIntentAndRationale", func(t *testing.T) {
		// Given an agent fixed a bug,
		// When it produces an explained diff,
		// Then each hunk carries WHAT (intent) and WHY (rationale)
		//      — reviewer doesn't have to guess from the diff alone.
		d := NewExplainedDiff("PR-42", "Fix JWT validation null pointer").
			AddBugFix("internal/auth/jwt.go", 42, 58,
				"validateClaims returned nil when typ header missing — caused 500. Now returns explicit error.",
				"BUG-1234")
		assert.Len(t, d.ExplainedHunks, 1)
		assert.Equal(t, HunkIntentBugFix, d.ExplainedHunks[0].Intent)
		assert.Contains(t, d.ExplainedHunks[0].Rationale, "nil")
		assert.Equal(t, []string{"BUG-1234"}, d.ExplainedHunks[0].LinkedRequirements)
	})

	t.Run("Scenario_NineIntentsCoverTheCommonDiffPurposes", func(t *testing.T) {
		// Given dashboards aggregate by intent (PDF §11 — analytics
		//       on PR composition over time),
		expected := map[string]bool{
			"bug_fix":            true,
			"feature_add":        true,
			"refactor":           true,
			"cleanup":            true,
			"security_patch":     true,
			"dependency_update":  true,
			"test_added":         true,
			"doc_update":         true,
			"revert":             true,
		}
		for _, i := range AllHunkIntents() {
			assert.True(t, expected[string(i)], "intent %q not in stable set", i)
		}
		assert.Equal(t, 9, len(AllHunkIntents()))
	})

	t.Run("Scenario_MultipleHunksGroupByFileForRendering", func(t *testing.T) {
		// Given a PR touches 2 files with multiple hunks each,
		// When the markdown is rendered,
		// Then hunks are grouped by file (not by chronological order)
		//      so reviewer reads file-by-file.
		d := NewExplainedDiff("PR-X", "refactor").
			AddRefactor("a.go", 50, 60, "extract method").
			AddRefactor("b.go", 5, 15, "rename type").
			AddRefactor("a.go", 10, 20, "inline helper")

		byFile := d.HunksByFile()
		assert.Len(t, byFile["a.go"], 2, "2 hunks in a.go")
		assert.Equal(t, 10, byFile["a.go"][0].Location.StartLine,
			"sorted by line within file")
		assert.Equal(t, 50, byFile["a.go"][1].Location.StartLine)
		assert.Len(t, byFile["b.go"], 1)
	})

	t.Run("Scenario_LinkedRequirementsTraceToTicketsAndADRs", func(t *testing.T) {
		// Given a security patch links to CVE + ADR,
		// When the diff is rendered,
		// Then the rendered output cites both — auditors can navigate.
		d := NewExplainedDiff("PR-9", "patch CVE-2026-X").
			AddSecurityPatch("auth.go", 10, 20,
				"clamp username to 256 chars to prevent buffer overflow",
				"CVE-2026-X", "ADR-007", "BUG-99")
		md := d.Markdown()
		assert.Contains(t, md, "Linked: CVE-2026-X, ADR-007, BUG-99")
	})

	t.Run("Scenario_HasIntentEnablesReviewRouting", func(t *testing.T) {
		// Given oncall code-review routing: "if HasIntent(security_patch)
		//       → escalate to security team",
		secPR := NewExplainedDiff("PR-1", "x").
			AddSecurityPatch("a.go", 1, 1, "fix")
		bugPR := NewExplainedDiff("PR-2", "y").
			AddBugFix("a.go", 1, 1, "fix")

		assert.True(t, secPR.HasIntent(HunkIntentSecurityPatch))
		assert.False(t, bugPR.HasIntent(HunkIntentSecurityPatch),
			"bug fix is not a security patch — distinct routing")
	})

	t.Run("Scenario_HistogramByIntentDescribesPRComposition", func(t *testing.T) {
		// Given a PR mixes bug fixes + tests + refactor,
		d := NewExplainedDiff("PR-X", "mixed").
			AddBugFix("a.go", 1, 1, "x").
			AddBugFix("b.go", 1, 1, "x").
			AddTestAdded("a_test.go", 1, 1, "x").
			AddRefactor("c.go", 1, 1, "x")

		hist := d.CountByIntent()
		assert.Equal(t, 2, hist[HunkIntentBugFix])
		assert.Equal(t, 1, hist[HunkIntentTestAdded])
		assert.Equal(t, 1, hist[HunkIntentRefactor])
		// Stable axes:
		assert.Equal(t, 0, hist[HunkIntentSecurityPatch],
			"unused intent must appear with 0 — dashboard contract")
	})

	t.Run("Scenario_FilesAffectedReturnsSortedUnique", func(t *testing.T) {
		// Given file-by-file routing or coverage analysis,
		d := NewExplainedDiff("x", "y").
			AddBugFix("zeta.go", 1, 1, "x").
			AddBugFix("alpha.go", 1, 1, "y").
			AddBugFix("alpha.go", 10, 10, "z").
			AddBugFix("middle.go", 1, 1, "w")
		files := d.FilesAffected()
		assert.Equal(t, []string{"alpha.go", "middle.go", "zeta.go"}, files,
			"sorted ascending, deduplicated")
	})

	t.Run("Scenario_BoundedIntentsPreventDriftAcrossDeploys", func(t *testing.T) {
		// Given a typo'd intent ("bugfix" without underscore),
		// When AddHunk is called,
		// Then it's silently dropped — caller bug surfaces in tests.
		d := NewExplainedDiff("x", "y").
			AddHunk(ExplainedHunk{
				Location: HunkLocation{File: "a.go", StartLine: 1, EndLine: 1},
				Intent:   HunkIntent("bugfix"), // missing underscore
				Rationale: "x",
			})
		assert.Empty(t, d.ExplainedHunks)
	})

	t.Run("Scenario_HunksWithoutFileLocationAreDropped", func(t *testing.T) {
		// Given the location is the join key for tracing,
		d := NewExplainedDiff("x", "y").
			AddHunk(ExplainedHunk{
				Intent:    HunkIntentBugFix,
				Rationale: "x",
				// File missing
			})
		assert.Empty(t, d.ExplainedHunks)
	})

	t.Run("Scenario_RationalesAreBoundedToPreventLogSpam", func(t *testing.T) {
		// Given LLM-generated rationales may be long,
		long := strings.Repeat("x", 1500)
		d := NewExplainedDiff("x", "y").
			AddHunk(ExplainedHunk{
				Location:  HunkLocation{File: "a.go", StartLine: 1, EndLine: 1},
				Intent:    HunkIntentBugFix,
				Rationale: long,
			})
		assert.Equal(t, 500, len(d.ExplainedHunks[0].Rationale))
	})

	t.Run("Scenario_NoHunksProducesExplicitFullReviewMessage", func(t *testing.T) {
		// Given silence is dangerous (reviewer wonders "did annotation run?"),
		d := NewExplainedDiff("PR-trivial", "typo")
		md := d.Markdown()
		assert.Contains(t, md, "No annotated hunks",
			"explicit zero-hunks message — never silent")
	})

	t.Run("Scenario_MultipleRendererFormatsForDifferentSurfaces", func(t *testing.T) {
		// Given the explained diff flows to: terminal (PlainText),
		//       PR description (Markdown), audit (JSON),
		d := NewExplainedDiff("PR-42", "auth fix").
			AddBugFix("auth.go", 42, 50, "null check", "BUG-1")

		plain := d.PlainText()
		md := d.Markdown()
		jsonStr, err := d.JSON()
		require.NoError(t, err)

		assert.Contains(t, plain, "[DIFF]")
		assert.Contains(t, plain, "1 hunks across 1 files")

		assert.Contains(t, md, "## Explained Diff:")
		assert.Contains(t, md, "[bug_fix]")
		assert.Contains(t, md, "Linked: BUG-1")

		assert.Contains(t, jsonStr, `"artifactId":"PR-42"`)
		assert.Contains(t, jsonStr, `"intent":"bug_fix"`)
		assert.Contains(t, jsonStr, `"linkedRequirements":["BUG-1"]`)
	})

	t.Run("Scenario_HunkLocationStringHandlesSingleAndRangeLines", func(t *testing.T) {
		// Given UI displays "file:42" or "file:42-58" depending on hunk size,
		single := HunkLocation{File: "x.go", StartLine: 5, EndLine: 5}
		assert.Equal(t, "x.go:5", single.String())

		multi := HunkLocation{File: "x.go", StartLine: 5, EndLine: 12}
		assert.Equal(t, "x.go:5-12", multi.String())
	})

	t.Run("Scenario_AuditTrailLinksDiffToReviewGuidanceViaArtifactID", func(t *testing.T) {
		// Given HUMAN-001 ReviewGuidance and HUMAN-002 ExplainedDiff
		//       both carry ArtifactID — they pair as "where to look"
		//       + "what each hunk does",
		guidance := NewReviewGuidance("PR-42", "code_diff", "auth fix").
			AddSecurityFocus("auth.go:42-58", "JWT signature change", "verify alg")
		diff := NewExplainedDiff("PR-42", "auth fix").
			AddSecurityPatch("auth.go", 42, 58, "fix CVE-2026-X", "CVE-2026-X")

		assert.Equal(t, guidance.ArtifactID, diff.ArtifactID,
			"same ArtifactID = same artifact — pairs across HUMAN-001/002")
	})
}
