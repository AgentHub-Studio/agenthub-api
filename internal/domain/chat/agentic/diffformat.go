package agentic

import (
	"fmt"
	"strings"
)

// Diff formatting, line change counting, and simple diff computation.
//
// Inspired by Claude Code's diff.ts — line change counting
// (additions/removals), unified diff rendering, and basic line-by-line
// diff computation. Complements the DiffHunk type and
// AdjustHunkLineNumbers already defined in gitdiff.go.

const (
	// DiffContextLines is the default number of context lines around changes.
	DiffContextLines = 3
)

// DiffStats holds aggregate line change statistics.
type DiffStats struct {
	Additions int `json:"additions"`
	Removals  int `json:"removals"`
}

// CountLinesChanged counts additions and removals across all hunks.
// If hunks is empty and newFileContent is provided, all lines in
// newFileContent are counted as additions (new file case).
func CountLinesChanged(hunks []DiffHunk, newFileContent string) DiffStats {
	if len(hunks) == 0 && newFileContent != "" {
		lines := strings.Split(newFileContent, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return DiffStats{Additions: len(lines)}
	}

	var additions, removals int
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if strings.HasPrefix(line, "+") {
				additions++
			} else if strings.HasPrefix(line, "-") {
				removals++
			}
		}
	}
	return DiffStats{Additions: additions, Removals: removals}
}

// FormatUnifiedDiff renders hunks as a unified diff string.
func FormatUnifiedDiff(filePath string, hunks []DiffHunk) string {
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- a/")
	sb.WriteString(filePath)
	sb.WriteByte('\n')
	sb.WriteString("+++ b/")
	sb.WriteString(filePath)
	sb.WriteByte('\n')

	for _, hunk := range hunks {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n",
			hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines)
		for _, line := range hunk.Lines {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// ComputeSimpleDiff creates diff hunks from old and new content by
// comparing lines. This is a basic line-by-line diff (not
// Myers/patience) — suitable for small edits and display purposes.
func ComputeSimpleDiff(oldContent, newContent string) []DiffHunk {
	if oldContent == newContent {
		return nil
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	prefixLen := 0
	minLen := len(oldLines)
	if len(newLines) < minLen {
		minLen = len(newLines)
	}
	for prefixLen < minLen && oldLines[prefixLen] == newLines[prefixLen] {
		prefixLen++
	}

	suffixLen := 0
	for suffixLen < minLen-prefixLen &&
		oldLines[len(oldLines)-1-suffixLen] == newLines[len(newLines)-1-suffixLen] {
		suffixLen++
	}

	oldChanged := oldLines[prefixLen : len(oldLines)-suffixLen]
	newChanged := newLines[prefixLen : len(newLines)-suffixLen]

	if len(oldChanged) == 0 && len(newChanged) == 0 {
		return nil
	}

	contextStart := prefixLen - DiffContextLines
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd := len(oldLines) - suffixLen + DiffContextLines
	if contextEnd > len(oldLines) {
		contextEnd = len(oldLines)
	}
	contextEndNew := len(newLines) - suffixLen + DiffContextLines
	if contextEndNew > len(newLines) {
		contextEndNew = len(newLines)
	}

	var lines []string
	for i := contextStart; i < prefixLen; i++ {
		lines = append(lines, " "+oldLines[i])
	}
	for _, l := range oldChanged {
		lines = append(lines, "-"+l)
	}
	for _, l := range newChanged {
		lines = append(lines, "+"+l)
	}
	for i := len(oldLines) - suffixLen; i < contextEnd; i++ {
		lines = append(lines, " "+oldLines[i])
	}

	return []DiffHunk{{
		OldStart: contextStart + 1,
		OldLines: (len(oldLines) - suffixLen - contextStart) + (contextEnd - (len(oldLines) - suffixLen)),
		NewStart: contextStart + 1,
		NewLines: (prefixLen - contextStart) + len(newChanged) + (contextEndNew - (len(newLines) - suffixLen)),
		Lines:    lines,
	}}
}
