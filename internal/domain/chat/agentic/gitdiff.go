package agentic

import (
	"fmt"
	"strings"
	"sync"
)

// Git diff analysis with line-level granularity.
//
// Inspired by Claude Code's gitDiff.ts — provides structured diff analysis
// with per-file stats, hunk parsing, and volume limits. Two-stage approach:
// quick stats first, detailed hunks on demand.

// GitDiffStats holds aggregate diff statistics.
type GitDiffStats struct {
	FilesCount   int `json:"filesCount"`
	LinesAdded   int `json:"linesAdded"`
	LinesRemoved int `json:"linesRemoved"`
}

// IsEmpty returns true if no changes were detected.
func (s GitDiffStats) IsEmpty() bool {
	return s.FilesCount == 0 && s.LinesAdded == 0 && s.LinesRemoved == 0
}

// Summary returns a human-readable summary.
func (s GitDiffStats) Summary() string {
	if s.IsEmpty() {
		return "no changes"
	}
	return fmt.Sprintf("%d files changed, %d insertions(+), %d deletions(-)",
		s.FilesCount, s.LinesAdded, s.LinesRemoved)
}

// PerFileStats holds diff stats for a single file.
type PerFileStats struct {
	FilePath    string `json:"filePath"`
	Added       int    `json:"added"`
	Removed     int    `json:"removed"`
	IsBinary    bool   `json:"isBinary"`
	IsNew       bool   `json:"isNew"`
	IsDeleted   bool   `json:"isDeleted"`
	IsRenamed   bool   `json:"isRenamed"`
	OldPath     string `json:"oldPath,omitempty"`
}

// DiffHunk represents a single hunk from a unified diff.
type DiffHunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Header   string   `json:"header,omitempty"` // @@ line
	Lines    []string `json:"lines"`            // with +/- prefixes
}

// GitDiffResult holds the complete diff analysis.
type GitDiffResult struct {
	Stats        GitDiffStats           `json:"stats"`
	PerFileStats map[string]PerFileStats `json:"perFileStats"`
	Hunks        map[string][]DiffHunk  `json:"hunks,omitempty"`
}

// Git diff limits.
const (
	GitDiffMaxFiles        = 50
	GitDiffMaxLinesPerFile = 400
	GitDiffMaxFileBytes    = 1024 * 1024 // 1MB
)

// GitTransientState represents a transient git state.
type GitTransientState string

const (
	GitStateNormal     GitTransientState = "normal"
	GitStateMerge      GitTransientState = "merge"
	GitStateRebase     GitTransientState = "rebase"
	GitStateCherryPick GitTransientState = "cherry_pick"
	GitStateRevert     GitTransientState = "revert"
)

// IsTransient returns true if git is in a transient state.
func (s GitTransientState) IsTransient() bool {
	return s != GitStateNormal
}

// GitDiffAnalyzer parses and analyzes git diffs.
type GitDiffAnalyzer struct {
	mu       sync.Mutex
	maxFiles int
	maxLines int
}

// NewGitDiffAnalyzer creates a diff analyzer with default limits.
func NewGitDiffAnalyzer() *GitDiffAnalyzer {
	return &GitDiffAnalyzer{
		maxFiles: GitDiffMaxFiles,
		maxLines: GitDiffMaxLinesPerFile,
	}
}

// ParseNumstat parses `git diff --numstat` output into per-file stats.
// Each line: "added\tremoved\tfilepath" or "-\t-\tbinary_file"
func (a *GitDiffAnalyzer) ParseNumstat(output string) map[string]PerFileStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make(map[string]PerFileStats)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for i, line := range lines {
		if i >= a.maxFiles {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		filePath := parts[2]

		// Handle renames: "old => new" or "{old => new}/path"
		if strings.Contains(filePath, " => ") {
			fs := PerFileStats{FilePath: filePath, IsRenamed: true}
			result[filePath] = fs
			continue
		}

		if parts[0] == "-" && parts[1] == "-" {
			result[filePath] = PerFileStats{
				FilePath: filePath,
				IsBinary: true,
			}
			continue
		}

		added := parseIntSafe(parts[0])
		removed := parseIntSafe(parts[1])

		result[filePath] = PerFileStats{
			FilePath: filePath,
			Added:    added,
			Removed:  removed,
		}
	}

	return result
}

// ParseUnifiedDiff parses a unified diff into per-file hunks.
func (a *GitDiffAnalyzer) ParseUnifiedDiff(diff string) map[string][]DiffHunk {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make(map[string][]DiffHunk)
	lines := strings.Split(diff, "\n")

	var currentFile string
	var currentHunk *DiffHunk
	fileCount := 0
	lineCount := 0

	for _, line := range lines {
		// New file header
		if strings.HasPrefix(line, "+++ b/") {
			if fileCount >= a.maxFiles {
				break
			}
			// Flush previous hunk before switching files
			if currentHunk != nil && currentFile != "" {
				result[currentFile] = append(result[currentFile], *currentHunk)
				currentHunk = nil
			}
			currentFile = strings.TrimPrefix(line, "+++ b/")
			fileCount++
			lineCount = 0
			continue
		}

		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Hunk header
		if strings.HasPrefix(line, "@@") {
			if currentFile == "" {
				continue
			}
			hunk := parseHunkHeader(line)
			if hunk != nil {
				if currentHunk != nil {
					result[currentFile] = append(result[currentFile], *currentHunk)
				}
				currentHunk = hunk
				lineCount = 0
			}
			continue
		}

		// Diff lines
		if currentHunk != nil && currentFile != "" {
			if lineCount < a.maxLines {
				if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") {
					currentHunk.Lines = append(currentHunk.Lines, line)
					lineCount++
				}
			}
		}
	}

	// Flush last hunk
	if currentHunk != nil && currentFile != "" {
		result[currentFile] = append(result[currentFile], *currentHunk)
	}

	return result
}

// ComputeStats calculates aggregate stats from per-file stats.
func ComputeStats(perFile map[string]PerFileStats) GitDiffStats {
	stats := GitDiffStats{FilesCount: len(perFile)}
	for _, fs := range perFile {
		stats.LinesAdded += fs.Added
		stats.LinesRemoved += fs.Removed
	}
	return stats
}

// AdjustHunkLineNumbers offsets hunk line numbers for slice-based contexts.
func AdjustHunkLineNumbers(hunks []DiffHunk, offset int) []DiffHunk {
	adjusted := make([]DiffHunk, len(hunks))
	for i, h := range hunks {
		adjusted[i] = h
		adjusted[i].OldStart += offset
		adjusted[i].NewStart += offset
	}
	return adjusted
}

// parseHunkHeader parses "@@ -old,count +new,count @@ optional header".
func parseHunkHeader(line string) *DiffHunk {
	if !strings.HasPrefix(line, "@@") {
		return nil
	}

	// Find the range portion between @@ markers
	end := strings.Index(line[2:], "@@")
	if end < 0 {
		return nil
	}
	rangePart := strings.TrimSpace(line[2 : end+2])

	hunk := &DiffHunk{Header: line}

	parts := strings.Fields(rangePart)
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			parseRange(p[1:], &hunk.OldStart, &hunk.OldLines)
		} else if strings.HasPrefix(p, "+") {
			parseRange(p[1:], &hunk.NewStart, &hunk.NewLines)
		}
	}

	return hunk
}

// parseRange parses "start,count" or "start".
func parseRange(s string, start, count *int) {
	parts := strings.SplitN(s, ",", 2)
	*start = parseIntSafe(parts[0])
	if len(parts) > 1 {
		*count = parseIntSafe(parts[1])
	} else {
		*count = 1
	}
}

// parseIntSafe parses an integer, returning 0 on error.
func parseIntSafe(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
