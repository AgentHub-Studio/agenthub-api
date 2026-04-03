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
	tail := messages[splitIdx:]

	// Preserve any active tool_call chains in the tail boundary.
	// If the first tail message is a tool_result, include its preceding tool_use too.
	for splitIdx > 0 && tail[0].MessageType == chat.MessageTypeToolResult {
		splitIdx--
		tail = messages[splitIdx:]
	}
	toSummarize := messages[:splitIdx]

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

// CompactStage identifies how aggressively the context was compacted.
type CompactStage string

const (
	StageToolResultTruncation CompactStage = "tool_result_truncation"
	StageHistorySnip          CompactStage = "history_snip"
	StageMicrocompact         CompactStage = "microcompact"
	StageFullSummarization    CompactStage = "full_summarization"
)

// ReactiveCompactResult extends CompactResult with the stage applied.
type ReactiveCompactResult struct {
	CompactResult
	Stage CompactStage
}

// ReactiveCompact applies progressive compaction stages based on context pressure.
// Stage 1 (75% threshold): Truncate old tool results to 200 chars
// Stage 2 (80% threshold): Snip oldest messages beyond tail
// Stage 3 (85% threshold): Microcompact — truncate all non-tail messages
// Stage 4 (90% threshold): Full LLM summarization
func (cm *ContextManager) ReactiveCompact(ctx context.Context, messages []chat.ChatMessage, systemTokens int, cfg RunConfig, summarizer Summarizer) (*ReactiveCompactResult, error) {
	windowSize := GetContextWindowSize(cfg.Model, cfg.ContextWindowSize)
	totalTokens := EstimateTokens(messages) + systemTokens
	usage := float64(totalTokens) / float64(windowSize)

	// Stage 1: Truncate old tool results (75-80%).
	if usage >= 0.75 && usage < 0.80 {
		compacted := truncateOldToolResults(messages, cm.TailSize, 200)
		return &ReactiveCompactResult{
			CompactResult: CompactResult{
				Messages:       compacted,
				OriginalCount:  len(messages),
				CompactedCount: len(compacted),
			},
			Stage: StageToolResultTruncation,
		}, nil
	}

	// Stage 2: History snip — remove oldest messages beyond tail (80-85%).
	if usage >= 0.80 && usage < 0.85 {
		snipped := snipOldHistory(messages, cm.TailSize*2)
		return &ReactiveCompactResult{
			CompactResult: CompactResult{
				Messages:       snipped,
				OriginalCount:  len(messages),
				CompactedCount: len(snipped),
			},
			Stage: StageHistorySnip,
		}, nil
	}

	// Stage 3: Microcompact — aggressively truncate non-tail messages (85-90%).
	if usage >= 0.85 && usage < 0.90 {
		micro := microcompact(messages, cm.TailSize)
		return &ReactiveCompactResult{
			CompactResult: CompactResult{
				Messages:       micro,
				OriginalCount:  len(messages),
				CompactedCount: len(micro),
			},
			Stage: StageMicrocompact,
		}, nil
	}

	// Stage 4: Full summarization (>= 90%).
	if usage >= 0.90 {
		result, err := cm.Compact(ctx, messages, summarizer)
		if err != nil {
			return nil, err
		}
		return &ReactiveCompactResult{
			CompactResult: *result,
			Stage:         StageFullSummarization,
		}, nil
	}

	// No compaction needed.
	return &ReactiveCompactResult{
		CompactResult: CompactResult{
			Messages:       messages,
			OriginalCount:  len(messages),
			CompactedCount: len(messages),
		},
	}, nil
}

// truncateOldToolResults truncates tool_result content in messages outside the tail.
func truncateOldToolResults(messages []chat.ChatMessage, tailSize, maxChars int) []chat.ChatMessage {
	result := make([]chat.ChatMessage, len(messages))
	copy(result, messages)

	boundary := len(result) - tailSize
	if boundary < 0 {
		boundary = 0
	}

	for i := 0; i < boundary; i++ {
		if result[i].MessageType == chat.MessageTypeToolResult && len(result[i].Content) > maxChars {
			result[i].Content = result[i].Content[:maxChars] + "\n[truncated]"
		}
	}
	return result
}

// snipOldHistory keeps only the most recent N messages.
func snipOldHistory(messages []chat.ChatMessage, keep int) []chat.ChatMessage {
	if len(messages) <= keep {
		return messages
	}
	return messages[len(messages)-keep:]
}

// microcompact aggressively truncates all messages outside the tail to 100 chars.
func microcompact(messages []chat.ChatMessage, tailSize int) []chat.ChatMessage {
	result := make([]chat.ChatMessage, len(messages))
	copy(result, messages)

	boundary := len(result) - tailSize
	if boundary < 0 {
		boundary = 0
	}

	for i := 0; i < boundary; i++ {
		if len(result[i].Content) > 100 {
			result[i].Content = result[i].Content[:100] + "..."
		}
		result[i].ToolCalls = nil // strip tool_calls JSON from old messages
	}
	return result
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
