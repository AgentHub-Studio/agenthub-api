package agentic

import (
	"fmt"
	"sync"
	"time"
)

// LSP diagnostic registry with cross-turn deduplication.
//
// Inspired by Claude Code's LSPDiagnosticRegistry.ts — tracks diagnostics
// from language servers, deduplicates across turns using an LRU cache,
// and applies volume limits to prevent overwhelming the agentic context.

// LSPDiagnosticSeverity maps to LSP specification severity levels.
type LSPDiagnosticSeverity int

const (
	LSPSeverityError   LSPDiagnosticSeverity = 1
	LSPSeverityWarning LSPDiagnosticSeverity = 2
	LSPSeverityInfo    LSPDiagnosticSeverity = 3
	LSPSeverityHint    LSPDiagnosticSeverity = 4
)

// String returns the severity name.
func (s LSPDiagnosticSeverity) String() string {
	switch s {
	case LSPSeverityError:
		return "error"
	case LSPSeverityWarning:
		return "warning"
	case LSPSeverityInfo:
		return "info"
	case LSPSeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// LSPDiagnosticItem represents a single diagnostic from a language server.
type LSPDiagnosticItem struct {
	ServerName string                `json:"serverName"`
	FilePath   string                `json:"filePath"`
	Line       int                   `json:"line"`
	Column     int                   `json:"column"`
	EndLine    int                   `json:"endLine,omitempty"`
	EndColumn  int                   `json:"endColumn,omitempty"`
	Severity   LSPDiagnosticSeverity `json:"severity"`
	Message    string                `json:"message"`
	Code       string                `json:"code,omitempty"`
	Source     string                `json:"source,omitempty"`
}

// PendingLSPDiagnostic groups diagnostics for a file, pending attachment.
type PendingLSPDiagnostic struct {
	ServerName     string              `json:"serverName"`
	FilePath       string              `json:"filePath"`
	Diagnostics    []LSPDiagnosticItem `json:"diagnostics"`
	Timestamp      time.Time           `json:"timestamp"`
	AttachmentSent bool                `json:"attachmentSent"`
}

// LSP registry limits.
const (
	LSPMaxDiagnosticsPerFile = 10
	LSPMaxTotalDiagnostics   = 30
	LSPMaxTrackedFiles       = 500
)

// LSPDiagnosticRegistry tracks diagnostics across turns with deduplication.
type LSPDiagnosticRegistry struct {
	mu       sync.Mutex
	pending  map[string]*PendingLSPDiagnostic // filePath → pending
	sentKeys map[string]time.Time             // dedup key → when sent (LRU-ish)
}

// NewLSPDiagnosticRegistry creates a new registry.
func NewLSPDiagnosticRegistry() *LSPDiagnosticRegistry {
	return &LSPDiagnosticRegistry{
		pending:  make(map[string]*PendingLSPDiagnostic),
		sentKeys: make(map[string]time.Time),
	}
}

// Register adds diagnostics for a file. Replaces any existing pending
// diagnostics for the same file.
func (r *LSPDiagnosticRegistry) Register(serverName, filePath string, diagnostics []LSPDiagnosticItem) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Trim to per-file limit
	if len(diagnostics) > LSPMaxDiagnosticsPerFile {
		diagnostics = diagnostics[:LSPMaxDiagnosticsPerFile]
	}

	r.pending[filePath] = &PendingLSPDiagnostic{
		ServerName:  serverName,
		FilePath:    filePath,
		Diagnostics: diagnostics,
		Timestamp:   time.Now(),
	}

	// Evict oldest tracked files if over capacity
	r.evictSentKeys()
}

// GetPending returns all pending diagnostics that haven't been sent yet,
// applying the total volume limit.
func (r *LSPDiagnosticRegistry) GetPending() []*PendingLSPDiagnostic {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []*PendingLSPDiagnostic
	totalDiags := 0

	for _, pd := range r.pending {
		if pd.AttachmentSent {
			continue
		}

		// Check deduplication
		key := r.dedupKey(pd)
		if _, sent := r.sentKeys[key]; sent {
			continue
		}

		remaining := LSPMaxTotalDiagnostics - totalDiags
		if remaining <= 0 {
			break
		}

		// Trim if necessary
		diags := pd.Diagnostics
		if len(diags) > remaining {
			diags = diags[:remaining]
		}

		copy := *pd
		copy.Diagnostics = diags
		result = append(result, &copy)
		totalDiags += len(diags)
	}

	return result
}

// MarkSent marks pending diagnostics for a file as sent.
func (r *LSPDiagnosticRegistry) MarkSent(filePath string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pd, ok := r.pending[filePath]
	if !ok {
		return
	}

	pd.AttachmentSent = true
	r.sentKeys[r.dedupKey(pd)] = time.Now()
}

// Clear removes all pending diagnostics.
func (r *LSPDiagnosticRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = make(map[string]*PendingLSPDiagnostic)
}

// ClearFile removes pending diagnostics for a specific file.
func (r *LSPDiagnosticRegistry) ClearFile(filePath string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, filePath)
}

// PendingCount returns the number of files with pending diagnostics.
func (r *LSPDiagnosticRegistry) PendingCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, pd := range r.pending {
		if !pd.AttachmentSent {
			count++
		}
	}
	return count
}

// HasErrors returns true if any pending diagnostic has error severity.
func (r *LSPDiagnosticRegistry) HasErrors() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pd := range r.pending {
		if pd.AttachmentSent {
			continue
		}
		for _, d := range pd.Diagnostics {
			if d.Severity == LSPSeverityError {
				return true
			}
		}
	}
	return false
}

// FormatSummary returns a human-readable summary of pending diagnostics.
func (r *LSPDiagnosticRegistry) FormatSummary() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	errors, warnings, infos := 0, 0, 0
	for _, pd := range r.pending {
		if pd.AttachmentSent {
			continue
		}
		for _, d := range pd.Diagnostics {
			switch d.Severity {
			case LSPSeverityError:
				errors++
			case LSPSeverityWarning:
				warnings++
			default:
				infos++
			}
		}
	}

	if errors+warnings+infos == 0 {
		return "no diagnostics"
	}

	return fmt.Sprintf("%d errors, %d warnings, %d info", errors, warnings, infos)
}

// dedupKey generates a deduplication key for a pending diagnostic group.
func (r *LSPDiagnosticRegistry) dedupKey(pd *PendingLSPDiagnostic) string {
	return fmt.Sprintf("%s:%s:%d", pd.ServerName, pd.FilePath, len(pd.Diagnostics))
}

// evictSentKeys removes oldest entries when over LSPMaxTrackedFiles.
func (r *LSPDiagnosticRegistry) evictSentKeys() {
	if len(r.sentKeys) <= LSPMaxTrackedFiles {
		return
	}

	// Simple eviction: remove entries older than 1 hour
	cutoff := time.Now().Add(-time.Hour)
	for k, ts := range r.sentKeys {
		if ts.Before(cutoff) {
			delete(r.sentKeys, k)
		}
	}
}
