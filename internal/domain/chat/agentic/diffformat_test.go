package agentic_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- AdjustHunkLineNumbers ---

func TestAdjustHunkLineNumbers_NoOffset(t *testing.T) {
	hunks := []agentic.DiffHunk{{OldStart: 1, NewStart: 1}}
	result := agentic.AdjustHunkLineNumbers(hunks, 0)
	assert.Equal(t, 1, result[0].OldStart)
}

func TestAdjustHunkLineNumbers_PositiveOffset(t *testing.T) {
	hunks := []agentic.DiffHunk{{OldStart: 5, NewStart: 5, OldLines: 3, NewLines: 4}}
	result := agentic.AdjustHunkLineNumbers(hunks, 10)
	assert.Equal(t, 15, result[0].OldStart)
	assert.Equal(t, 15, result[0].NewStart)
	assert.Equal(t, 3, result[0].OldLines)
}

func TestAdjustHunkLineNumbers_MultipleHunks(t *testing.T) {
	hunks := []agentic.DiffHunk{
		{OldStart: 1, NewStart: 1},
		{OldStart: 20, NewStart: 22},
	}
	result := agentic.AdjustHunkLineNumbers(hunks, 5)
	assert.Equal(t, 6, result[0].OldStart)
	assert.Equal(t, 25, result[1].OldStart)
}

func TestAdjustHunkLineNumbers_Empty(t *testing.T) {
	result := agentic.AdjustHunkLineNumbers(nil, 10)
	assert.Empty(t, result)
}

// --- CountLinesChanged ---

func TestCountLinesChanged_FromHunks(t *testing.T) {
	hunks := []agentic.DiffHunk{{
		Lines: []string{" context", "+added1", "+added2", "-removed", " context"},
	}}
	stats := agentic.CountLinesChanged(hunks, "")
	assert.Equal(t, 2, stats.Additions)
	assert.Equal(t, 1, stats.Removals)
}

func TestCountLinesChanged_NewFile(t *testing.T) {
	stats := agentic.CountLinesChanged(nil, "line1\nline2\nline3\n")
	assert.Equal(t, 3, stats.Additions)
	assert.Equal(t, 0, stats.Removals)
}

func TestCountLinesChanged_EmptyBoth(t *testing.T) {
	stats := agentic.CountLinesChanged(nil, "")
	assert.Equal(t, 0, stats.Additions)
	assert.Equal(t, 0, stats.Removals)
}

func TestCountLinesChanged_MultipleHunks(t *testing.T) {
	hunks := []agentic.DiffHunk{
		{Lines: []string{"+a", "+b"}},
		{Lines: []string{"-c", "+d", "+e"}},
	}
	stats := agentic.CountLinesChanged(hunks, "")
	assert.Equal(t, 4, stats.Additions)
	assert.Equal(t, 1, stats.Removals)
}

// --- FormatUnifiedDiff ---

func TestFormatUnifiedDiff_Empty(t *testing.T) {
	result := agentic.FormatUnifiedDiff("file.go", nil)
	assert.Equal(t, "", result)
}

func TestFormatUnifiedDiff_SingleHunk(t *testing.T) {
	hunks := []agentic.DiffHunk{{
		OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 4,
		Lines: []string{" line1", "-old", "+new1", "+new2", " line3"},
	}}
	result := agentic.FormatUnifiedDiff("src/main.go", hunks)
	assert.Contains(t, result, "--- a/src/main.go")
	assert.Contains(t, result, "+++ b/src/main.go")
	assert.Contains(t, result, "@@ -1,3 +1,4 @@")
	assert.Contains(t, result, "-old")
	assert.Contains(t, result, "+new1")
}

func TestFormatUnifiedDiff_MultipleHunks(t *testing.T) {
	hunks := []agentic.DiffHunk{
		{OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1, Lines: []string{"-a", "+b"}},
		{OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 1, Lines: []string{"-c", "+d"}},
	}
	result := agentic.FormatUnifiedDiff("f.txt", hunks)
	assert.Equal(t, 2, strings.Count(result, "@@ "))
}

// --- ComputeSimpleDiff ---

func TestComputeSimpleDiff_Identical(t *testing.T) {
	hunks := agentic.ComputeSimpleDiff("hello\nworld", "hello\nworld")
	assert.Nil(t, hunks)
}

func TestComputeSimpleDiff_SingleLineChange(t *testing.T) {
	hunks := agentic.ComputeSimpleDiff("hello\nworld", "hello\nearth")
	assert.Len(t, hunks, 1)
	stats := agentic.CountLinesChanged(hunks, "")
	assert.Equal(t, 1, stats.Additions)
	assert.Equal(t, 1, stats.Removals)
}

func TestComputeSimpleDiff_Addition(t *testing.T) {
	hunks := agentic.ComputeSimpleDiff("a\nb", "a\nb\nc")
	assert.Len(t, hunks, 1)
	stats := agentic.CountLinesChanged(hunks, "")
	assert.Equal(t, 1, stats.Additions)
	assert.Equal(t, 0, stats.Removals)
}

func TestComputeSimpleDiff_Removal(t *testing.T) {
	hunks := agentic.ComputeSimpleDiff("a\nb\nc", "a\nc")
	assert.Len(t, hunks, 1)
	stats := agentic.CountLinesChanged(hunks, "")
	assert.Equal(t, 0, stats.Additions)
	assert.Equal(t, 1, stats.Removals)
}

func TestComputeSimpleDiff_HasContext(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8"
	new := "line1\nline2\nline3\nline4\nCHANGED\nline6\nline7\nline8"
	hunks := agentic.ComputeSimpleDiff(old, new)
	assert.Len(t, hunks, 1)
	// Should have context lines around the change.
	contextCount := 0
	for _, l := range hunks[0].Lines {
		if strings.HasPrefix(l, " ") {
			contextCount++
		}
	}
	assert.Greater(t, contextCount, 0)
}

func TestComputeSimpleDiff_CompletelyDifferent(t *testing.T) {
	hunks := agentic.ComputeSimpleDiff("old", "new")
	assert.Len(t, hunks, 1)
	stats := agentic.CountLinesChanged(hunks, "")
	assert.Equal(t, 1, stats.Additions)
	assert.Equal(t, 1, stats.Removals)
}

func TestComputeSimpleDiff_Empty(t *testing.T) {
	hunks := agentic.ComputeSimpleDiff("", "")
	assert.Nil(t, hunks)
}
