package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/google/uuid"
)

// Summarizer abstracts the LLM call used to compress conversation history.
// The Runner will provide a concrete implementation once the ai/ package is available.
type Summarizer interface {
	// Summarize sends a summarization prompt to the LLM and returns the summary text.
	Summarize(ctx context.Context, prompt string) (string, error)
}

// AutoCompactTracking tracks the state of the auto-compaction lifecycle across
// turns. It enables analytics correlation, turn counting since last compaction,
// and a circuit breaker to stop retrying when context is irrecoverably over limit.
//
// Inspired by Claude Code's AutoCompactTrackingState in services/compact/autoCompact.ts.
type AutoCompactTracking struct {
	// Compacted indicates whether at least one compaction has occurred in this chain.
	Compacted bool `json:"compacted"`
	// TurnCounter counts turns elapsed since the last successful compaction.
	// Reset to 0 on each new compaction. Enables analytics to measure how many
	// turns the agent runs before needing to compact again.
	TurnCounter int `json:"turnCounter"`
	// TurnID is a unique UUID generated on each successful compaction event.
	// Allows analytics to correlate post-compact turns with the compaction that
	// triggered them (e.g. RecompactionInfo).
	TurnID string `json:"turnId"`
	// ConsecutiveFailures tracks how many compactions in a row have failed or
	// had no effect. Acts as a circuit breaker to avoid wasting API calls on
	// irrecoverable context overflow (e.g. a single huge system prompt).
	ConsecutiveFailures int `json:"consecutiveFailures"`
}

// RecompactionInfo provides context about the compaction chain for analytics.
// Inspired by Claude Code's RecompactionInfo in services/compact/compact.ts.
type RecompactionInfo struct {
	// IsRecompactionInChain is true when a previous compaction already happened.
	IsRecompactionInChain bool `json:"isRecompactionInChain"`
	// TurnsSincePreviousCompact is the number of turns since the last compaction.
	// -1 means no previous compaction in this chain.
	TurnsSincePreviousCompact int `json:"turnsSincePreviousCompact"`
	// PreviousCompactTurnID is the TurnID of the most recent compaction.
	PreviousCompactTurnID string `json:"previousCompactTurnId,omitempty"`
}

// RecompactionInfo returns analytics context about the current compaction chain.
func (t AutoCompactTracking) RecompactionInfo() RecompactionInfo {
	return RecompactionInfo{
		IsRecompactionInChain:     t.Compacted,
		TurnsSincePreviousCompact: t.TurnCounter,
		PreviousCompactTurnID:     t.TurnID,
	}
}

// RecompactionInfoPtr returns analytics context, handling nil pointer receivers.
// Returns -1 for TurnsSincePreviousCompact when nil (no tracking state).
func RecompactionInfoFromPtr(t *AutoCompactTracking) RecompactionInfo {
	if t == nil {
		return RecompactionInfo{TurnsSincePreviousCompact: -1}
	}
	return t.RecompactionInfo()
}

// ContextManager handles token estimation and context window compression.
type ContextManager struct {
	// TailSize is the number of recent messages to preserve when compacting.
	TailSize int
	// tracking holds the auto-compact lifecycle state.
	tracking AutoCompactTracking
}

// maxConsecutiveCompactFailures stops auto-compaction after this many
// consecutive failures to avoid wasting API calls when context is
// irrecoverably over the limit (e.g. a single huge system prompt).
const maxConsecutiveCompactFailures = 3

// NewContextManager creates a ContextManager with sensible defaults.
func NewContextManager() *ContextManager {
	return &ContextManager{TailSize: 4}
}

// Tracking returns the current auto-compact tracking state.
func (cm *ContextManager) Tracking() AutoCompactTracking {
	return cm.tracking
}

// RecordCompactSuccess resets the failure counter, sets the compacted flag,
// generates a new TurnID, and resets TurnCounter to 0.
func (cm *ContextManager) RecordCompactSuccess() {
	cm.tracking.Compacted = true
	cm.tracking.TurnID = uuid.New().String()
	cm.tracking.TurnCounter = 0
	cm.tracking.ConsecutiveFailures = 0
}

// RecordCompactFailure increments the consecutive failure counter.
func (cm *ContextManager) RecordCompactFailure() {
	cm.tracking.ConsecutiveFailures++
}

// RecordTurnAfterCompact increments the turn counter if a compaction has
// occurred. Should be called after each successful LLM turn.
func (cm *ContextManager) RecordTurnAfterCompact() {
	if cm.tracking.Compacted {
		cm.tracking.TurnCounter++
	}
}

// CompactCircuitOpen returns true when auto-compaction should be skipped
// because too many consecutive compactions have failed.
func (cm *ContextManager) CompactCircuitOpen() bool {
	return cm.tracking.ConsecutiveFailures >= maxConsecutiveCompactFailures
}

// ResetTracking clears the tracking state. Used when reactive-compact recovery
// takes over to avoid double-tracking.
func (cm *ContextManager) ResetTracking() {
	cm.tracking = AutoCompactTracking{}
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

// EstimateStringTokensForType returns a rough token count using file-type-aware
// ratios. JSON content uses ~2 chars/token (many single-char tokens like braces,
// colons, commas). Other content uses ~4 chars/token.
// Inspired by Claude Code's roughTokenCountEstimationForFileType in tokens.ts.
func EstimateStringTokensForType(s string, fileExt string) int {
	if len(s) == 0 {
		return 0
	}
	bytesPerToken := 4
	switch strings.ToLower(fileExt) {
	case ".json", ".jsonl", ".jsonc", ".geojson":
		bytesPerToken = 2
	}
	return len(s)/bytesPerToken + 3
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

// VerifyCompaction checks whether the compacted result actually reduced token usage
// below the threshold that triggered it. If not, it escalates to the next stage.
// This prevents compaction loops where the same stage is applied repeatedly without effect.
func (cm *ContextManager) VerifyCompaction(ctx context.Context, result *ReactiveCompactResult, systemTokens int, cfg RunConfig, summarizer Summarizer) (*ReactiveCompactResult, error) {
	if result.Stage == "" {
		return result, nil // no compaction was applied
	}

	windowSize := GetContextWindowSize(cfg.Model, cfg.ContextWindowSize)
	newTokens := EstimateTokens(result.Messages) + systemTokens
	newUsage := float64(newTokens) / float64(windowSize)

	// Check if we're still above the stage's lower bound.
	var threshold float64
	switch result.Stage {
	case StageToolResultTruncation:
		threshold = 0.75
	case StageHistorySnip:
		threshold = 0.80
	case StageMicrocompact:
		threshold = 0.85
	case StageFullSummarization:
		return result, nil // can't escalate beyond full summarization
	}

	if newUsage < threshold {
		return result, nil // compaction was effective
	}

	// Escalate: apply the next stage on the already-compacted messages.
	return cm.ReactiveCompact(ctx, result.Messages, systemTokens, cfg, summarizer)
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

// snipOldHistory keeps only the most recent N messages, adjusting the cut
// point to preserve tool_use/tool_result pairs (API invariant).
func snipOldHistory(messages []chat.ChatMessage, keep int) []chat.ChatMessage {
	if len(messages) <= keep {
		return messages
	}
	startIdx := len(messages) - keep
	startIdx = adjustSplitForToolPairs(messages, startIdx)
	return messages[startIdx:]
}

// adjustSplitForToolPairs adjusts a split index backward to avoid orphaning
// tool_result messages. If the first kept message is a tool_result, it pulls
// the index back to include the preceding tool_use (assistant) message.
// Inspired by Claude Code's adjustIndexToPreserveAPIInvariants.
func adjustSplitForToolPairs(messages []chat.ChatMessage, startIdx int) int {
	if startIdx <= 0 || startIdx >= len(messages) {
		return startIdx
	}

	// Collect tool_call_ids from tool_result messages in the kept range
	// that need matching tool_use messages.
	neededIDs := map[string]bool{}
	for i := startIdx; i < len(messages); i++ {
		if messages[i].MessageType == chat.MessageTypeToolResult && messages[i].ToolCallID != nil {
			neededIDs[*messages[i].ToolCallID] = true
		}
	}

	if len(neededIDs) == 0 {
		return startIdx
	}

	// Check if matching tool_use messages are already in the kept range.
	for i := startIdx; i < len(messages); i++ {
		if messages[i].MessageType == chat.MessageTypeToolUse && len(messages[i].ToolCalls) > 0 {
			// Parse tool call IDs from this message.
			var tcs []struct{ ID string `json:"id"` }
			if err := json.Unmarshal(messages[i].ToolCalls, &tcs); err == nil {
				for _, tc := range tcs {
					delete(neededIDs, tc.ID)
				}
			}
		}
	}

	if len(neededIDs) == 0 {
		return startIdx
	}

	// Look backward for assistant messages containing the needed tool_use IDs.
	for i := startIdx - 1; i >= 0 && len(neededIDs) > 0; i-- {
		if messages[i].MessageType == chat.MessageTypeToolUse && len(messages[i].ToolCalls) > 0 {
			var tcs []struct{ ID string `json:"id"` }
			if err := json.Unmarshal(messages[i].ToolCalls, &tcs); err == nil {
				for _, tc := range tcs {
					if neededIDs[tc.ID] {
						delete(neededIDs, tc.ID)
						if i < startIdx {
							startIdx = i
						}
					}
				}
			}
		}
	}

	return startIdx
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

// TimeBasedEvictionConfig controls when idle sessions should proactively
// clear old tool results. When the server-side prompt cache has expired
// (e.g. after 60 minutes idle), the full prefix will be rewritten anyway,
// so clearing old results shrinks the rewritten payload.
// Inspired by Claude Code's timeBasedMCConfig.
type TimeBasedEvictionConfig struct {
	// GapThreshold is the duration since the last turn that triggers eviction.
	// Default: 60 minutes (safe choice — server's 1h cache TTL is guaranteed expired).
	GapThreshold time.Duration
	// KeepRecent is the number of most-recent tool results to preserve.
	// Default: 5.
	KeepRecent int
}

// DefaultTimeBasedEvictionConfig returns sensible defaults matching
// Claude Code's time-based microcompact configuration.
func DefaultTimeBasedEvictionConfig() TimeBasedEvictionConfig {
	return TimeBasedEvictionConfig{
		GapThreshold: 60 * time.Minute,
		KeepRecent:   5,
	}
}

// EvictStaleToolResults clears old tool_result content when the session has
// been idle for longer than cfg.GapThreshold. Keeps only the most recent
// cfg.KeepRecent tool results intact; older ones get their content replaced
// with a placeholder. Returns the messages unmodified if the gap is below
// threshold or there aren't enough tool results to evict.
func EvictStaleToolResults(messages []chat.ChatMessage, timeSinceLastTurn time.Duration, cfg TimeBasedEvictionConfig) []chat.ChatMessage {
	if timeSinceLastTurn < cfg.GapThreshold {
		return messages
	}

	// Count tool_result messages.
	var toolResultIndices []int
	for i, m := range messages {
		if m.MessageType == chat.MessageTypeToolResult {
			toolResultIndices = append(toolResultIndices, i)
		}
	}

	// Nothing to evict if within keep budget.
	if len(toolResultIndices) <= cfg.KeepRecent {
		return messages
	}

	// Copy messages to avoid mutating the original slice.
	result := make([]chat.ChatMessage, len(messages))
	copy(result, messages)

	// Clear content of older tool results, keeping the most recent N.
	evictCount := len(toolResultIndices) - cfg.KeepRecent
	for j := 0; j < evictCount; j++ {
		idx := toolResultIndices[j]
		result[idx].Content = "[tool result cleared: session idle > " + cfg.GapThreshold.String() + "]"
	}

	return result
}

// CompactForPTLRetry truncates oldest messages to recover from a prompt-too-long error.
// If the error message contains parseable token counts (e.g. "210000 tokens > 200000"),
// it uses the exact gap to calculate how many messages to drop. Otherwise it falls back
// to dropping 20% of messages. Returns nil if there are too few messages to truncate.
// Inspired by Claude Code's truncateHeadForPTLRetry.
func (cm *ContextManager) CompactForPTLRetry(messages []chat.ChatMessage, errMsg string) []chat.ChatMessage {
	if len(messages) <= cm.TailSize+1 {
		return nil // too few messages to truncate
	}

	tokenGap := getPromptTooLongTokenGap(errMsg)
	var dropCount int

	if tokenGap > 0 {
		// Precise: accumulate token counts from oldest messages until reaching the gap.
		acc := 0
		for i := 0; i < len(messages)-cm.TailSize; i++ {
			acc += estimateMessageTokens(messages[i])
			dropCount++
			if acc >= tokenGap {
				break
			}
		}
	} else {
		// Fallback: drop 20% of messages when error format is unrecognised.
		dropCount = len(messages) / 5
		if dropCount < 1 {
			dropCount = 1
		}
	}

	// Don't drop more than available (keep tail).
	maxDrop := len(messages) - cm.TailSize
	if dropCount > maxDrop {
		dropCount = maxDrop
	}
	if dropCount < 1 {
		return nil
	}

	sliced := messages[dropCount:]

	// Fix tool_result orphaning at the new boundary.
	startIdx := adjustSplitForToolPairs(messages, dropCount)
	if startIdx != dropCount {
		sliced = messages[startIdx:]
	}

	return sliced
}

// TruncateToTokens truncates content to fit within a token budget using the
// chars/4 heuristic. Preserves the beginning of the content (instruction headers)
// and appends a truncation marker. Returns content unchanged if it fits.
// Inspired by Claude Code's truncateToTokens in services/compact/compact.ts.
func TruncateToTokens(content string, maxTokens int) string {
	if EstimateStringTokens(content) <= maxTokens {
		return content
	}
	const marker = "\n\n[... content truncated for compaction]"
	charBudget := maxTokens*4 - len(marker)
	if charBudget < 0 {
		charBudget = 0
	}
	if charBudget >= len(content) {
		return content
	}
	return content[:charBudget] + marker
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
