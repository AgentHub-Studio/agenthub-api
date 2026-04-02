package agentic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// Summarizer abstracts the LLM call used to compress conversation history.
// The Runner will provide a concrete implementation once the ai/ package is available.
type Summarizer interface {
	// Summarize sends a summarization prompt to the LLM and returns the summary text.
	Summarize(ctx context.Context, prompt string) (string, error)
}

// ContextManager handles token estimation and context window compression.
type ContextManager struct {
	// TailSize is the number of recent messages to preserve when compacting.
	TailSize int
}

// NewContextManager creates a ContextManager with sensible defaults.
func NewContextManager() *ContextManager {
	return &ContextManager{TailSize: 4}
}

// --- Token estimation ---

// EstimateTokens returns a rough token count for a slice of ChatMessages.
// Uses the chars/4 heuristic which works reasonably well for English text
// and provides a conservative estimate for non-Latin scripts.
func EstimateTokens(messages []chat.ChatMessage) int {
	total := 0
	for _, m := range messages {
		total += estimateMessageTokens(m)
	}
	return total
}

// EstimateStringTokens returns a rough token count for a plain string.
func EstimateStringTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	// ~4 chars per token for English, add small overhead for message framing.
	return len(s)/4 + 3
}

// estimateMessageTokens estimates the token count for a single message.
func estimateMessageTokens(m chat.ChatMessage) int {
	tokens := 0
	// Role token overhead (~4 tokens for role + framing).
	tokens += 4
	// Content.
	tokens += len(m.Content) / 4
	// Tool calls JSON.
	if len(m.ToolCalls) > 0 {
		tokens += len(m.ToolCalls) / 4
	}
	return tokens
}

// --- Context window sizes by model ---

// knownContextWindows maps model prefixes to their context window sizes.
var knownContextWindows = map[string]int{
	"claude-opus-4":      200000,
	"claude-sonnet-4":    200000,
	"claude-haiku-4":     200000,
	"claude-3.5-sonnet":  200000,
	"claude-3-opus":      200000,
	"claude-3-sonnet":    200000,
	"claude-3-haiku":     200000,
	"gpt-4o":             128000,
	"gpt-4-turbo":        128000,
	"gpt-4":              8192,
	"gpt-3.5-turbo":      16385,
	"o1":                 200000,
	"o3":                 200000,
}

// GetContextWindowSize returns the context window size for a model.
// Falls back to the value in RunConfig if the model is not recognized.
func GetContextWindowSize(model string, fallback int) int {
	// Try exact match first.
	if size, ok := knownContextWindows[model]; ok {
		return size
	}
	// Try longest prefix match (e.g. "gpt-4o-2024" matches "gpt-4o" not "gpt-4").
	bestSize := 0
	bestLen := 0
	for prefix, size := range knownContextWindows {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix && len(prefix) > bestLen {
			bestSize = size
			bestLen = len(prefix)
		}
	}
	if bestLen > 0 {
		return bestSize
	}
	if fallback > 0 {
		return fallback
	}
	return 200000 // safe default
}

// --- Compaction ---

// ShouldCompact returns true when the estimated token usage exceeds
// the compaction threshold of the context window.
func ShouldCompact(messages []chat.ChatMessage, systemPromptTokens int, cfg RunConfig) bool {
	totalTokens := EstimateTokens(messages) + systemPromptTokens
	threshold := int(float64(cfg.ContextWindowSize) * cfg.CompactThreshold)
	return totalTokens > threshold
}

// CompactResult holds the output of a context compaction.
type CompactResult struct {
	// Messages is the compacted message list (compact_summary + tail).
	Messages []chat.ChatMessage
	// Summary is the generated summary text.
	Summary string
	// OriginalCount is the number of messages before compaction.
	OriginalCount int
	// CompactedCount is the number of messages after compaction.
	CompactedCount int
}

// Compact compresses older messages into a summary, preserving recent tail messages.
// If summarizer is nil or there are too few messages to compact, it returns the
// original messages unchanged.
func (cm *ContextManager) Compact(ctx context.Context, messages []chat.ChatMessage, summarizer Summarizer) (*CompactResult, error) {
	if len(messages) <= cm.TailSize {
		return &CompactResult{
			Messages:       messages,
			OriginalCount:  len(messages),
			CompactedCount: len(messages),
		}, nil
	}

	if summarizer == nil {
		return &CompactResult{
			Messages:       messages,
			OriginalCount:  len(messages),
			CompactedCount: len(messages),
		}, nil
	}

	// Split: old messages to summarize, tail to preserve.
	splitIdx := len(messages) - cm.TailSize
	toSummarize := messages[:splitIdx]
	tail := messages[splitIdx:]

	// Preserve any active tool_call chains in the tail boundary.
	// If the first tail message is a tool_result, include its preceding tool_use too.
	for splitIdx > 0 && tail[0].MessageType == chat.MessageTypeToolResult {
		splitIdx--
		tail = messages[splitIdx:]
	}
	toSummarize = messages[:splitIdx]

	if len(toSummarize) == 0 {
		return &CompactResult{
			Messages:       messages,
			OriginalCount:  len(messages),
			CompactedCount: len(messages),
		}, nil
	}

	// Build summarization prompt.
	prompt := buildSummarizationPrompt(toSummarize)

	summary, err := summarizer.Summarize(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("context: compact: %w", err)
	}

	// Build compacted message list.
	summaryMsg := chat.ChatMessage{
		Role:        "system",
		Content:     summary,
		MessageType: chat.MessageTypeCompactSummary,
	}

	compacted := make([]chat.ChatMessage, 0, 1+len(tail))
	compacted = append(compacted, summaryMsg)
	compacted = append(compacted, tail...)

	return &CompactResult{
		Messages:       compacted,
		Summary:        summary,
		OriginalCount:  len(messages),
		CompactedCount: len(compacted),
	}, nil
}

// buildSummarizationPrompt creates the prompt sent to the LLM for context compression.
func buildSummarizationPrompt(messages []chat.ChatMessage) string {
	var conversation string
	for _, m := range messages {
		switch m.MessageType {
		case chat.MessageTypeToolUse:
			var calls []json.RawMessage
			if len(m.ToolCalls) > 0 {
				_ = json.Unmarshal(m.ToolCalls, &calls)
			}
			conversation += fmt.Sprintf("[%s] (tool_use: %d calls)\n", m.Role, len(calls))
		case chat.MessageTypeToolResult:
			conversation += fmt.Sprintf("[%s] (tool_result)\n", m.Role)
		default:
			content := m.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			conversation += fmt.Sprintf("[%s] %s\n", m.Role, content)
		}
	}

	return fmt.Sprintf(`Summarize the following conversation history. Preserve:
- Key decisions made
- Important facts and data mentioned
- Code snippets or technical details referenced
- Errors encountered and their resolutions
- User preferences expressed

Format: concise bullet points. Do NOT include greetings or filler.

---
%s
---

Summary:`, conversation)
}
