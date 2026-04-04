package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Paste content reference tracking.
//
// Inspired by Claude Code's pastedContent.ts — stores pasted text/images
// by reference (hash) to avoid duplicating large content in the message
// history. References are formatted as special markers and expanded when
// building the LLM prompt.

// PasteContentType distinguishes text pastes from image pastes.
type PasteContentType string

const (
	PasteTypeText  PasteContentType = "text"
	PasteTypeImage PasteContentType = "image"
)

// StoredPastedContent holds a pasted content entry, either inline or by hash reference.
type StoredPastedContent struct {
	// ID is a unique identifier for this paste.
	ID string `json:"id"`
	// Type is text or image.
	Type PasteContentType `json:"type"`
	// Content is the full text (for inline storage) or empty (for hash storage).
	Content string `json:"content,omitempty"`
	// Hash is the SHA-256 hash of Content (used as reference key).
	Hash string `json:"hash"`
	// MimeType is the MIME type (for images).
	MimeType string `json:"mimeType,omitempty"`
	// Label is an optional display label.
	Label string `json:"label,omitempty"`
	// Lines is the line count of text content.
	Lines int `json:"lines,omitempty"`
}

// pasteRefPattern matches paste references in message content.
// Format: {{paste:TYPE:HASH}} or {{paste:TYPE:HASH:LABEL}}
var pasteRefPattern = regexp.MustCompile(`\{\{paste:(text|image):([a-f0-9]{64})(?::([^}]*))?\}\}`)

// FormatPastedTextRef creates a paste reference marker for text content.
func FormatPastedTextRef(hash, label string) string {
	if label != "" {
		return fmt.Sprintf("{{paste:text:%s:%s}}", hash, label)
	}
	return fmt.Sprintf("{{paste:text:%s}}", hash)
}

// FormatImageRef creates a paste reference marker for image content.
func FormatImageRef(hash, label string) string {
	if label != "" {
		return fmt.Sprintf("{{paste:image:%s:%s}}", hash, label)
	}
	return fmt.Sprintf("{{paste:image:%s}}", hash)
}

// PasteReference is a parsed reference found in content.
type PasteReference struct {
	// Full is the complete matched string (e.g., "{{paste:text:abc123}}").
	Full string
	// Type is text or image.
	Type PasteContentType
	// Hash is the content hash.
	Hash string
	// Label is the optional display label.
	Label string
}

// ParseReferences extracts all paste references from content.
func ParseReferences(content string) []PasteReference {
	matches := pasteRefPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	refs := make([]PasteReference, 0, len(matches))
	for _, m := range matches {
		ref := PasteReference{
			Full: m[0],
			Type: PasteContentType(m[1]),
			Hash: m[2],
		}
		if len(m) > 3 {
			ref.Label = m[3]
		}
		refs = append(refs, ref)
	}
	return refs
}

// HashContent returns the SHA-256 hex digest of content.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// PasteStore is a thread-safe store for pasted content, keyed by hash.
type PasteStore struct {
	mu      sync.RWMutex
	entries map[string]StoredPastedContent
}

// NewPasteStore creates an empty paste store.
func NewPasteStore() *PasteStore {
	return &PasteStore{
		entries: make(map[string]StoredPastedContent),
	}
}

// Store adds content to the store, returning its hash and reference marker.
func (ps *PasteStore) Store(content string, typ PasteContentType, label string) (hash string, ref string) {
	hash = HashContent(content)

	ps.mu.Lock()
	if _, exists := ps.entries[hash]; !exists {
		entry := StoredPastedContent{
			ID:      hash[:12], // short ID for display
			Type:    typ,
			Content: content,
			Hash:    hash,
			Label:   label,
		}
		if typ == PasteTypeText {
			entry.Lines = strings.Count(content, "\n") + 1
		}
		ps.entries[hash] = entry
	}
	ps.mu.Unlock()

	if typ == PasteTypeImage {
		ref = FormatImageRef(hash, label)
	} else {
		ref = FormatPastedTextRef(hash, label)
	}
	return
}

// StoreImage adds an image entry with MIME type.
func (ps *PasteStore) StoreImage(content, mimeType, label string) (hash string, ref string) {
	hash = HashContent(content)

	ps.mu.Lock()
	if _, exists := ps.entries[hash]; !exists {
		ps.entries[hash] = StoredPastedContent{
			ID:       hash[:12],
			Type:     PasteTypeImage,
			Content:  content,
			Hash:     hash,
			MimeType: mimeType,
			Label:    label,
		}
	}
	ps.mu.Unlock()

	ref = FormatImageRef(hash, label)
	return
}

// Get retrieves stored content by hash.
func (ps *PasteStore) Get(hash string) (StoredPastedContent, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	entry, ok := ps.entries[hash]
	return entry, ok
}

// ExpandReferences replaces all paste references in content with actual content.
// Unknown references are left as-is.
func (ps *PasteStore) ExpandReferences(content string) string {
	refs := ParseReferences(content)
	if len(refs) == 0 {
		return content
	}

	result := content
	for _, ref := range refs {
		entry, ok := ps.Get(ref.Hash)
		if !ok {
			continue
		}

		var replacement string
		switch ref.Type {
		case PasteTypeText:
			if ref.Label != "" {
				replacement = fmt.Sprintf("[Pasted: %s]\n%s", ref.Label, entry.Content)
			} else {
				replacement = entry.Content
			}
		case PasteTypeImage:
			if ref.Label != "" {
				replacement = fmt.Sprintf("[Image: %s]", ref.Label)
			} else {
				replacement = "[Image]"
			}
		}

		result = strings.Replace(result, ref.Full, replacement, 1)
	}
	return result
}

// Size returns the number of stored entries.
func (ps *PasteStore) Size() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.entries)
}

// Hashes returns all stored hashes.
func (ps *PasteStore) Hashes() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	hashes := make([]string, 0, len(ps.entries))
	for h := range ps.entries {
		hashes = append(hashes, h)
	}
	return hashes
}
