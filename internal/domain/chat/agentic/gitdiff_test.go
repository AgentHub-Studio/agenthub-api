package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- GitDiffStats ---

func TestGitDiffStats_IsEmpty(t *testing.T) {
	assert.True(t, agentic.GitDiffStats{}.IsEmpty())
	assert.False(t, agentic.GitDiffStats{FilesCount: 1}.IsEmpty())
	assert.False(t, agentic.GitDiffStats{LinesAdded: 1}.IsEmpty())
}

func TestGitDiffStats_Summary(t *testing.T) {
	assert.Equal(t, "no changes", agentic.GitDiffStats{}.Summary())

	s := agentic.GitDiffStats{FilesCount: 3, LinesAdded: 10, LinesRemoved: 5}
	assert.Contains(t, s.Summary(), "3 files changed")
	assert.Contains(t, s.Summary(), "10 insertions(+)")
	assert.Contains(t, s.Summary(), "5 deletions(-)")
}

// --- GitTransientState ---

func TestGitTransientState(t *testing.T) {
	assert.False(t, agentic.GitStateNormal.IsTransient())
	assert.True(t, agentic.GitStateMerge.IsTransient())
	assert.True(t, agentic.GitStateRebase.IsTransient())
	assert.True(t, agentic.GitStateCherryPick.IsTransient())
	assert.True(t, agentic.GitStateRevert.IsTransient())
}

// --- ParseNumstat ---

func TestGitDiffAnalyzer_ParseNumstat(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	output := "10\t5\tsrc/main.go\n3\t0\tsrc/util.go\n"

	stats := a.ParseNumstat(output)
	require.Len(t, stats, 2)

	main := stats["src/main.go"]
	assert.Equal(t, 10, main.Added)
	assert.Equal(t, 5, main.Removed)
	assert.False(t, main.IsBinary)

	util := stats["src/util.go"]
	assert.Equal(t, 3, util.Added)
	assert.Equal(t, 0, util.Removed)
}

func TestGitDiffAnalyzer_ParseNumstat_Binary(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	output := "-\t-\timage.png\n"

	stats := a.ParseNumstat(output)
	require.Len(t, stats, 1)
	assert.True(t, stats["image.png"].IsBinary)
}

func TestGitDiffAnalyzer_ParseNumstat_Rename(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	output := "0\t0\told.go => new.go\n"

	stats := a.ParseNumstat(output)
	require.Len(t, stats, 1)
	for _, s := range stats {
		assert.True(t, s.IsRenamed)
	}
}

func TestGitDiffAnalyzer_ParseNumstat_Empty(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	stats := a.ParseNumstat("")
	assert.Empty(t, stats)
}

// --- ParseUnifiedDiff ---

func TestGitDiffAnalyzer_ParseUnifiedDiff(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	diff := `--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {
-    println("hello")
+    fmt.Println("hello")
 }
`

	hunks := a.ParseUnifiedDiff(diff)
	require.Len(t, hunks, 1)
	require.Contains(t, hunks, "main.go")

	h := hunks["main.go"]
	require.Len(t, h, 1)
	assert.Equal(t, 1, h[0].OldStart)
	assert.Equal(t, 3, h[0].OldLines)
	assert.Equal(t, 1, h[0].NewStart)
	assert.Equal(t, 4, h[0].NewLines)
	assert.GreaterOrEqual(t, len(h[0].Lines), 4)
}

func TestGitDiffAnalyzer_ParseUnifiedDiff_MultipleFiles(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	diff := `--- a/a.go
+++ b/a.go
@@ -1,2 +1,3 @@
 package a
+// comment
 var x = 1
--- a/b.go
+++ b/b.go
@@ -1,1 +1,2 @@
 package b
+var y = 2
`

	hunks := a.ParseUnifiedDiff(diff)
	assert.Len(t, hunks, 2)
	assert.Contains(t, hunks, "a.go")
	assert.Contains(t, hunks, "b.go")
}

func TestGitDiffAnalyzer_ParseUnifiedDiff_Empty(t *testing.T) {
	a := agentic.NewGitDiffAnalyzer()
	hunks := a.ParseUnifiedDiff("")
	assert.Empty(t, hunks)
}

// --- ComputeStats ---

func TestComputeStats(t *testing.T) {
	perFile := map[string]agentic.PerFileStats{
		"a.go": {Added: 10, Removed: 3},
		"b.go": {Added: 5, Removed: 2},
	}

	stats := agentic.ComputeStats(perFile)
	assert.Equal(t, 2, stats.FilesCount)
	assert.Equal(t, 15, stats.LinesAdded)
	assert.Equal(t, 5, stats.LinesRemoved)
}

func TestComputeStats_Empty(t *testing.T) {
	stats := agentic.ComputeStats(nil)
	assert.True(t, stats.IsEmpty())
}

// --- AdjustHunkLineNumbers ---

func TestAdjustHunkLineNumbers(t *testing.T) {
	hunks := []agentic.DiffHunk{
		{OldStart: 10, NewStart: 12},
		{OldStart: 20, NewStart: 25},
	}

	adjusted := agentic.AdjustHunkLineNumbers(hunks, 5)
	assert.Equal(t, 15, adjusted[0].OldStart)
	assert.Equal(t, 17, adjusted[0].NewStart)
	assert.Equal(t, 25, adjusted[1].OldStart)
	assert.Equal(t, 30, adjusted[1].NewStart)

	// Original unchanged
	assert.Equal(t, 10, hunks[0].OldStart)
}

// --- Constants ---

func TestGitDiff_Constants(t *testing.T) {
	assert.Equal(t, 50, agentic.GitDiffMaxFiles)
	assert.Equal(t, 400, agentic.GitDiffMaxLinesPerFile)
	assert.Equal(t, 1024*1024, agentic.GitDiffMaxFileBytes)
}
