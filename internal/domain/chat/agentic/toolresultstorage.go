package agentic

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ContentReplacementState tracks which tool results have been seen and what their
// replacement text is. This enables byte-identical replay across turns for prompt
// cache stability — once a tool result is replaced (or kept), that decision is
// frozen for the rest of the conversation.
//
// Inspired by Claude Code's ContentReplacementState in utils/toolResultStorage.ts.
type ContentReplacementState struct {
	mu           sync.Mutex
	seenIDs      map[string]bool   // Tool use IDs that have been processed.
	replacements map[string]string // Mapping of tool_use_id → replacement text.
}

// NewContentReplacementState creates an empty replacement state.
func NewContentReplacementState() *ContentReplacementState {
	return &ContentReplacementState{
		seenIDs:      make(map[string]bool),
		replacements: make(map[string]string),
	}
}

// ContentReplacementRecord is a serializable record of a replacement decision
// that can be persisted to the session transcript for resume support.
//
// Inspired by Claude Code's ContentReplacementRecord type.
type ContentReplacementRecord struct {
	Kind        string `json:"kind"`        // Always "tool-result".
	ToolUseID   string `json:"toolUseId"`
	Replacement string `json:"replacement"` // The exact string the model saw.
}

// PersistedToolResult holds the metadata about a tool result that was persisted
// to storage because it exceeded the size threshold.
type PersistedToolResult struct {
	// ToolUseID identifies the tool call.
	ToolUseID string `json:"toolUseId"`
	// OriginalSize is the byte length of the original output.
	OriginalSize int `json:"originalSize"`
	// Preview is the first PreviewSize bytes of the original content.
	Preview string `json:"preview"`
	// HasMore indicates whether the output was truncated.
	HasMore bool `json:"hasMore"`
}

const (
	// DefaultMaxResultSizeChars is the default per-tool threshold for persisting
	// large tool results. Results exceeding this are persisted to storage and
	// replaced with a preview in the conversation.
	// Inspired by Claude Code's DEFAULT_MAX_RESULT_SIZE_CHARS (50,000).
	DefaultMaxResultSizeChars = 50000

	// DefaultMaxResultsPerMessageChars is the per-message aggregate budget for
	// tool results. When the sum of all tool results in a single turn exceeds
	// this, the largest results are replaced with previews.
	// Inspired by Claude Code's MAX_TOOL_RESULTS_PER_MESSAGE_CHARS (200,000).
	DefaultMaxResultsPerMessageChars = 200000

	// PreviewSize is the number of characters preserved in the preview when a
	// tool result is persisted.
	PreviewSize = 2048
)

// IsSeen returns true if the tool use ID has been processed before.
func (s *ContentReplacementState) IsSeen(toolUseID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seenIDs[toolUseID]
}

// GetReplacement returns the cached replacement text for a tool use ID.
// Returns ("", false) if the ID has no replacement (either unseen or kept).
func (s *ContentReplacementState) GetReplacement(toolUseID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.replacements[toolUseID]
	return r, ok
}

// MarkSeen marks a tool use ID as seen without replacing it. The decision to
// keep the original content is now frozen — it won't be replaced in future turns.
func (s *ContentReplacementState) MarkSeen(toolUseID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seenIDs[toolUseID] = true
}

// MarkReplaced records a replacement for a tool use ID. Both seenIDs and
// replacements are updated atomically.
func (s *ContentReplacementState) MarkReplaced(toolUseID, replacement string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seenIDs[toolUseID] = true
	s.replacements[toolUseID] = replacement
}

// Records returns all replacement records for transcript persistence.
func (s *ContentReplacementState) Records() []ContentReplacementRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]ContentReplacementRecord, 0, len(s.replacements))
	for id, text := range s.replacements {
		records = append(records, ContentReplacementRecord{
			Kind:        "tool-result",
			ToolUseID:   id,
			Replacement: text,
		})
	}
	return records
}

// ReplacementCount returns the number of replaced tool results.
func (s *ContentReplacementState) ReplacementCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.replacements)
}

// SeenCount returns the number of seen tool use IDs.
func (s *ContentReplacementState) SeenCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seenIDs)
}

// Clone creates a deep copy of the state, suitable for passing to forked
// sub-agents that share the parent's prompt cache prefix.
func (s *ContentReplacementState) Clone() *ContentReplacementState {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := &ContentReplacementState{
		seenIDs:      make(map[string]bool, len(s.seenIDs)),
		replacements: make(map[string]string, len(s.replacements)),
	}
	for k, v := range s.seenIDs {
		clone.seenIDs[k] = v
	}
	for k, v := range s.replacements {
		clone.replacements[k] = v
	}
	return clone
}

// ReconstructContentReplacementState rebuilds the state from persisted records,
// typically during session resume. It also processes message history to mark
// all existing tool use IDs as seen (frozen).
//
// Inspired by Claude Code's reconstructContentReplacementState.
func ReconstructContentReplacementState(records []ContentReplacementRecord) *ContentReplacementState {
	state := NewContentReplacementState()
	for _, rec := range records {
		state.seenIDs[rec.ToolUseID] = true
		if rec.Replacement != "" {
			state.replacements[rec.ToolUseID] = rec.Replacement
		}
	}
	return state
}

// BuildLargeResultPreview creates the replacement message for a tool result that
// was persisted to storage. The preview includes a truncated beginning of the
// content wrapped in XML tags for easy parsing.
//
// Inspired by Claude Code's buildLargeToolResultMessage.
func BuildLargeResultPreview(toolUseID string, originalSize int, preview string) string {
	return fmt.Sprintf(
		"<persisted-output>\nOutput too large (%d chars). Tool use ID: %s\n\nPreview (first %d chars):\n%s\n...\n</persisted-output>",
		originalSize, toolUseID, len(preview), preview,
	)
}

// toolResultCandidate represents a tool result eligible for budget enforcement.
type toolResultCandidate struct {
	index   int
	id      string
	size    int
	content string
}

// EnforceToolResultBudget applies per-tool and per-message budgets to tool results.
// It classifies each result into three categories:
//   - mustReapply: previously replaced → re-apply cached replacement (zero I/O)
//   - frozen: previously seen but not replaced → locked in (can't replace)
//   - fresh: never seen → eligible for replacement decisions
//
// Fresh results exceeding perToolLimit are replaced individually. If the aggregate
// of all non-replaced results exceeds messageBudget, the largest fresh results
// are replaced until the budget is met.
//
// Inspired by Claude Code's enforceToolResultBudget in utils/toolResultStorage.ts.
func EnforceToolResultBudget(
	results []ToolExecResult,
	toolUseIDs []string,
	state *ContentReplacementState,
	perToolLimit int,
	messageBudget int,
) []ToolExecResult {
	if state == nil || len(results) == 0 {
		return results
	}
	if perToolLimit <= 0 {
		perToolLimit = DefaultMaxResultSizeChars
	}
	if messageBudget <= 0 {
		messageBudget = DefaultMaxResultsPerMessageChars
	}

	out := make([]ToolExecResult, len(results))
	copy(out, results)

	var fresh []toolResultCandidate
	totalChars := 0

	for i, r := range out {
		if r.Error != nil || len(r.Output) == 0 {
			continue
		}
		id := ""
		if i < len(toolUseIDs) {
			id = toolUseIDs[i]
		}
		if id == "" {
			continue
		}

		outputStr := string(r.Output)
		size := len(outputStr)

		// Check if this was previously replaced → re-apply.
		if replacement, ok := state.GetReplacement(id); ok {
			out[i].Output = json.RawMessage(replacement)
			totalChars += len(replacement)
			continue
		}

		// Check if previously seen but not replaced → frozen.
		if state.IsSeen(id) {
			totalChars += size
			continue
		}

		// Fresh result — check per-tool limit first.
		if size > perToolLimit {
			preview := outputStr
			if len(preview) > PreviewSize {
				preview = preview[:PreviewSize]
			}
			replacement := BuildLargeResultPreview(id, size, preview)
			state.MarkReplaced(id, replacement)
			out[i].Output = json.RawMessage(replacement)
			totalChars += len(replacement)
			continue
		}

		// Under per-tool limit — mark seen and add to fresh candidates.
		fresh = append(fresh, toolResultCandidate{
			index:   i,
			id:      id,
			size:    size,
			content: outputStr,
		})
		totalChars += size
		state.MarkSeen(id)
	}

	// Check aggregate message budget.
	if totalChars <= messageBudget {
		return out
	}

	// Sort fresh candidates by size descending (replace largest first).
	// Simple insertion sort — typically few candidates.
	for i := 1; i < len(fresh); i++ {
		for j := i; j > 0 && fresh[j].size > fresh[j-1].size; j-- {
			fresh[j], fresh[j-1] = fresh[j-1], fresh[j]
		}
	}

	// Greedily replace largest fresh results until under budget.
	for _, c := range fresh {
		if totalChars <= messageBudget {
			break
		}
		preview := c.content
		if len(preview) > PreviewSize {
			preview = preview[:PreviewSize]
		}
		replacement := BuildLargeResultPreview(c.id, c.size, preview)
		state.MarkReplaced(c.id, replacement)
		out[c.index].Output = json.RawMessage(replacement)
		totalChars -= c.size
		totalChars += len(replacement)
	}

	return out
}
