package agentic

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Append-only history logging with lazy paste resolution.
//
// Inspired by Claude Code's history.ts — maintains an append-only log
// of entries with deferred large content resolution. Small pastes are
// stored inline; large pastes use hash references to external storage.

// HistoryInlineThreshold is the max byte size for inline paste storage.
const HistoryInlineThreshold = 1024

// HistoryMaxFlushRetries is the max retries for background flushing.
const HistoryMaxFlushRetries = 5

// HistoryEntry represents a single entry in the history log.
type HistoryEntry struct {
	// Display is the visible text of the entry.
	Display string `json:"display"`
	// Timestamp is when the entry was created.
	Timestamp time.Time `json:"timestamp"`
	// SessionID scopes the entry to a session.
	SessionID string `json:"sessionId,omitempty"`
	// Project identifies the project context.
	Project string `json:"project,omitempty"`
	// PastedContents holds inline or referenced paste data.
	PastedContents []PastedContent `json:"pastedContents,omitempty"`
}

// PastedContent is either inline content or a hash reference.
type PastedContent struct {
	// Inline holds the content directly (for small pastes).
	Inline string `json:"inline,omitempty"`
	// HashRef is a reference to externally stored content (for large pastes).
	HashRef string `json:"hashRef,omitempty"`
	// Label is an optional description.
	Label string `json:"label,omitempty"`
}

// IsReference returns true if this paste is stored externally.
func (p PastedContent) IsReference() bool {
	return p.HashRef != ""
}

// HistoryLog is an append-only in-memory history with background flush support.
type HistoryLog struct {
	mu       sync.Mutex
	entries  []HistoryEntry
	pending  []HistoryEntry
	flusher  HistoryFlusher
	resolver PasteResolver
	removed  map[time.Time]bool
}

// HistoryFlusher persists entries to durable storage.
type HistoryFlusher func(entries []HistoryEntry) error

// PasteResolver resolves hash references to content.
type PasteResolver func(hashRef string) (string, bool)

// NewHistoryLog creates a history log with optional flusher and resolver.
func NewHistoryLog(flusher HistoryFlusher, resolver PasteResolver) *HistoryLog {
	return &HistoryLog{
		flusher:  flusher,
		resolver: resolver,
		removed:  make(map[time.Time]bool),
	}
}

// Append adds an entry to the log. Large pastes are automatically
// converted to hash references.
func (h *HistoryLog) Append(entry HistoryEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Convert large pastes to references.
	for i, p := range entry.PastedContents {
		if p.Inline != "" && len(p.Inline) > HistoryInlineThreshold {
			hash := HashContent(p.Inline)
			entry.PastedContents[i] = PastedContent{
				HashRef: hash,
				Label:   p.Label,
			}
		}
	}

	h.entries = append(h.entries, entry)
	h.pending = append(h.pending, entry)
}

// Remove marks an entry for removal by timestamp.
func (h *HistoryLog) Remove(timestamp time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Fast path: check pending buffer.
	for i, e := range h.pending {
		if e.Timestamp.Equal(timestamp) {
			h.pending = append(h.pending[:i], h.pending[i+1:]...)
			break
		}
	}

	h.removed[timestamp] = true
}

// GetHistory returns entries in newest-first order, deduped by display text.
// If sessionID is provided, session-scoped entries appear first.
func (h *HistoryLog) GetHistory(sessionID string) []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()

	seen := make(map[string]bool)
	var sessionEntries, otherEntries []HistoryEntry

	// Iterate in reverse (newest first).
	for i := len(h.entries) - 1; i >= 0; i-- {
		e := h.entries[i]
		if h.removed[e.Timestamp] {
			continue
		}
		if seen[e.Display] {
			continue
		}
		seen[e.Display] = true

		if sessionID != "" && e.SessionID == sessionID {
			sessionEntries = append(sessionEntries, e)
		} else {
			otherEntries = append(otherEntries, e)
		}
	}

	return append(sessionEntries, otherEntries...)
}

// ResolveEntry resolves all paste references in an entry.
func (h *HistoryLog) ResolveEntry(entry HistoryEntry) HistoryEntry {
	if h.resolver == nil {
		return entry
	}

	resolved := entry
	resolved.PastedContents = make([]PastedContent, len(entry.PastedContents))
	copy(resolved.PastedContents, entry.PastedContents)

	for i, p := range resolved.PastedContents {
		if p.IsReference() {
			if content, ok := h.resolver(p.HashRef); ok {
				resolved.PastedContents[i] = PastedContent{
					Inline: content,
					Label:  p.Label,
				}
			}
		}
	}

	return resolved
}

// Flush writes pending entries to durable storage.
// Returns the number of flushed entries and any error.
func (h *HistoryLog) Flush() (int, error) {
	h.mu.Lock()
	if len(h.pending) == 0 {
		h.mu.Unlock()
		return 0, nil
	}
	pending := make([]HistoryEntry, len(h.pending))
	copy(pending, h.pending)
	h.pending = nil
	h.mu.Unlock()

	if h.flusher == nil {
		return len(pending), nil
	}

	var lastErr error
	for attempt := 0; attempt < HistoryMaxFlushRetries; attempt++ {
		if err := h.flusher(pending); err != nil {
			lastErr = err
			continue
		}
		return len(pending), nil
	}

	// Re-enqueue on failure.
	h.mu.Lock()
	h.pending = append(pending, h.pending...)
	h.mu.Unlock()

	return 0, fmt.Errorf("flush failed after %d retries: %w", HistoryMaxFlushRetries, lastErr)
}

// Len returns the total number of entries (including removed).
func (h *HistoryLog) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.entries)
}

// PendingCount returns the number of unflushed entries.
func (h *HistoryLog) PendingCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.pending)
}

// MarshalJSONL serializes entries as newline-delimited JSON.
func MarshalJSONL(entries []HistoryEntry) (string, error) {
	var b strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return "", fmt.Errorf("marshal entry: %w", err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
