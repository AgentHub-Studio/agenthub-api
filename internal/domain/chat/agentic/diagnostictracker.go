package agentic

import (
	"fmt"
	"strings"
	"sync"
)

// Diagnostic tracking for monitoring tool execution health.
//
// Inspired by Claude Code's diagnosticTracking.ts — captures baseline
// diagnostics before edits, then computes only NEW diagnostics. Useful
// for tracking compilation errors, warnings, and other issues as the
// agent modifies code.

// DiagnosticSeverity indicates the severity of a diagnostic.
type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = 0
	SeverityWarning DiagnosticSeverity = 1
	SeverityInfo    DiagnosticSeverity = 2
	SeverityHint    DiagnosticSeverity = 3
)

// String returns the human-readable severity label.
func (s DiagnosticSeverity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Diagnostic represents a single diagnostic issue.
type Diagnostic struct {
	// Severity indicates the issue level.
	Severity DiagnosticSeverity `json:"severity"`
	// Message is the human-readable description.
	Message string `json:"message"`
	// Source identifies the diagnostic provider (e.g., "go vet").
	Source string `json:"source,omitempty"`
	// Code is an optional error code.
	Code string `json:"code,omitempty"`
	// Line is the 1-based line number.
	Line int `json:"line,omitempty"`
	// Column is the 1-based column number.
	Column int `json:"column,omitempty"`
}

// DiagnosticFile groups diagnostics for a single file.
type DiagnosticFile struct {
	// Path is the file path.
	Path string `json:"path"`
	// Diagnostics is the list of issues for this file.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// DiagnosticFetchFunc retrieves current diagnostics for a file.
type DiagnosticFetchFunc func(path string) []Diagnostic

// DiagnosticTracker captures baseline diagnostics and computes new ones.
type DiagnosticTracker struct {
	mu        sync.Mutex
	baselines map[string][]Diagnostic
	fetcher   DiagnosticFetchFunc
}

// NewDiagnosticTracker creates a tracker with the given diagnostic fetcher.
func NewDiagnosticTracker(fetcher DiagnosticFetchFunc) *DiagnosticTracker {
	return &DiagnosticTracker{
		baselines: make(map[string][]Diagnostic),
		fetcher:   fetcher,
	}
}

// CaptureBaseline records the current diagnostics for a file before editing.
func (t *DiagnosticTracker) CaptureBaseline(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fetcher != nil {
		t.baselines[path] = t.fetcher(path)
	} else {
		t.baselines[path] = nil
	}
}

// CaptureBaselineManual records a manually provided baseline.
func (t *DiagnosticTracker) CaptureBaselineManual(path string, diagnostics []Diagnostic) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.baselines[path] = diagnostics
}

// GetNewDiagnostics returns only diagnostics that are NEW since the baseline.
// A diagnostic is considered new if its (severity, message, source, code) tuple
// doesn't appear in the baseline.
func (t *DiagnosticTracker) GetNewDiagnostics(path string) []Diagnostic {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fetcher == nil {
		return nil
	}

	current := t.fetcher(path)
	baseline, hasBaseline := t.baselines[path]
	if !hasBaseline {
		return current
	}

	// Build a set of baseline diagnostic keys.
	baselineSet := make(map[string]int) // key → remaining count
	for _, d := range baseline {
		baselineSet[diagnosticKey(d)]++
	}

	var newDiags []Diagnostic
	for _, d := range current {
		key := diagnosticKey(d)
		if baselineSet[key] > 0 {
			baselineSet[key]--
		} else {
			newDiags = append(newDiags, d)
		}
	}

	return newDiags
}

// GetAllNewDiagnostics returns new diagnostics for all tracked files.
func (t *DiagnosticTracker) GetAllNewDiagnostics() []DiagnosticFile {
	t.mu.Lock()
	paths := make([]string, 0, len(t.baselines))
	for p := range t.baselines {
		paths = append(paths, p)
	}
	t.mu.Unlock()

	var files []DiagnosticFile
	for _, path := range paths {
		newDiags := t.GetNewDiagnostics(path)
		if len(newDiags) > 0 {
			files = append(files, DiagnosticFile{Path: path, Diagnostics: newDiags})
		}
	}
	return files
}

// HasNewErrors returns true if any tracked file has new error-level diagnostics.
func (t *DiagnosticTracker) HasNewErrors() bool {
	files := t.GetAllNewDiagnostics()
	for _, f := range files {
		for _, d := range f.Diagnostics {
			if d.Severity == SeverityError {
				return true
			}
		}
	}
	return false
}

// TrackedFiles returns all files with baselines.
func (t *DiagnosticTracker) TrackedFiles() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	paths := make([]string, 0, len(t.baselines))
	for p := range t.baselines {
		paths = append(paths, p)
	}
	return paths
}

// Reset clears all baselines.
func (t *DiagnosticTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.baselines = make(map[string][]Diagnostic)
}

// FormatSummary produces a human-readable summary of new diagnostics.
func FormatDiagnosticSummary(files []DiagnosticFile) string {
	if len(files) == 0 {
		return "No new diagnostics."
	}

	var b strings.Builder
	totalErrors := 0
	totalWarnings := 0

	for _, f := range files {
		b.WriteString(fmt.Sprintf("## %s\n", f.Path))
		for _, d := range f.Diagnostics {
			switch d.Severity {
			case SeverityError:
				totalErrors++
			case SeverityWarning:
				totalWarnings++
			}

			loc := ""
			if d.Line > 0 {
				loc = fmt.Sprintf(":%d", d.Line)
				if d.Column > 0 {
					loc += fmt.Sprintf(":%d", d.Column)
				}
			}

			b.WriteString(fmt.Sprintf("  [%s]%s %s", d.Severity.String(), loc, d.Message))
			if d.Code != "" {
				b.WriteString(fmt.Sprintf(" (%s)", d.Code))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(fmt.Sprintf("\nTotal: %d error(s), %d warning(s)\n", totalErrors, totalWarnings))
	return b.String()
}

// diagnosticKey generates a deduplication key for a diagnostic.
func diagnosticKey(d Diagnostic) string {
	return fmt.Sprintf("%d|%s|%s|%s", d.Severity, d.Message, d.Source, d.Code)
}
