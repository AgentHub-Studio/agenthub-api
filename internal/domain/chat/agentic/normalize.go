package agentic

import (
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// NormalizeMessagesForAPI prepares chat messages for the LLM API by applying
// a multi-pass normalization pipeline. This ensures API invariants are met:
// - No consecutive same-role messages (merged)
// - No orphaned thinking-only assistant messages
// - No trailing thinking on the last assistant message
// - No whitespace-only messages
// - Tool result content sanitized
//
// Inspired by Claude Code's normalizeMessagesForAPI in utils/messages.ts.
func NormalizeMessagesForAPI(messages []chat.ChatMessage) []chat.ChatMessage {
	if len(messages) == 0 {
		return messages
	}

	result := messages

	// Pass 1: Merge consecutive same-role user messages.
	result = mergeConsecutiveUserMessages(result)

	// Pass 2: Filter orphaned thinking-only assistant messages.
	result = filterOrphanedThinkingMessages(result)

	// Pass 3: Strip trailing thinking from last assistant message.
	result = stripTrailingThinkingFromLastAssistant(result)

	// Pass 4: Filter whitespace-only messages.
	result = filterWhitespaceOnlyMessages(result)

	// Pass 5: Sanitize empty tool results.
	result = sanitizeEmptyToolResults(result)

	// Pass 6: Ensure alternating user/assistant pattern where possible.
	result = mergeConsecutiveAssistantMessages(result)

	return result
}

// mergeConsecutiveUserMessages combines consecutive user messages into one.
// The Anthropic API (and Bedrock) requires alternating user/assistant messages.
// Consecutive user messages can happen when the system injects nudge messages
// or when context compaction leaves adjacent user messages.
func mergeConsecutiveUserMessages(messages []chat.ChatMessage) []chat.ChatMessage {
	if len(messages) <= 1 {
		return messages
	}

	result := make([]chat.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "user" || len(result) == 0 || result[len(result)-1].Role != "user" {
			result = append(result, msg)
			continue
		}

		// Merge into previous user message.
		prev := &result[len(result)-1]
		if prev.Content != "" && msg.Content != "" {
			prev.Content = prev.Content + "\n\n" + msg.Content
		} else if msg.Content != "" {
			prev.Content = msg.Content
		}
	}
	return result
}

// mergeConsecutiveAssistantMessages combines consecutive assistant messages into one.
// This can happen when streaming produces multiple assistant message fragments or
// when tool_use messages are followed by text-only assistant messages.
func mergeConsecutiveAssistantMessages(messages []chat.ChatMessage) []chat.ChatMessage {
	if len(messages) <= 1 {
		return messages
	}

	result := make([]chat.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "assistant" || len(result) == 0 || result[len(result)-1].Role != "assistant" {
			result = append(result, msg)
			continue
		}

		prev := &result[len(result)-1]
		if prev.Content != "" && msg.Content != "" {
			prev.Content = prev.Content + "\n\n" + msg.Content
		} else if msg.Content != "" {
			prev.Content = msg.Content
		}
		// Merge tool calls.
		if len(msg.ToolCalls) > 0 && len(prev.ToolCalls) > 0 {
			// Both have tool calls — this is unusual, keep the newer ones.
			prev.ToolCalls = msg.ToolCalls
		} else if len(msg.ToolCalls) > 0 {
			prev.ToolCalls = msg.ToolCalls
		}
	}
	return result
}

// filterOrphanedThinkingMessages removes assistant messages that contain only
// thinking content (no visible text or tool calls). These can occur when the
// model produces a thinking block but then the response is interrupted.
//
// A "thinking-only" message is identified by having the metadata key "thinking_only"
// set to true, or by having empty content and no tool calls but having metadata
// with a "thinking" key.
//
// Inspired by Claude Code's filterOrphanedThinkingOnlyMessages.
func filterOrphanedThinkingMessages(messages []chat.ChatMessage) []chat.ChatMessage {
	result := make([]chat.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "assistant" && isThinkingOnlyMessage(msg) {
			continue
		}
		result = append(result, msg)
	}
	return result
}

// isThinkingOnlyMessage returns true if the assistant message contains only
// thinking content with no visible output.
func isThinkingOnlyMessage(msg chat.ChatMessage) bool {
	if msg.Role != "assistant" {
		return false
	}
	// Has visible content or tool calls → not thinking-only.
	if strings.TrimSpace(msg.Content) != "" {
		return false
	}
	if len(msg.ToolCalls) > 0 {
		return false
	}
	// Check metadata for thinking marker.
	if len(msg.Metadata) > 0 {
		return hasMetadataKey(msg.Metadata, "thinkingContent")
	}
	return false
}

// hasMetadataKey checks if a JSON metadata blob contains a specific key with
// a non-empty value.
func hasMetadataKey(metadata []byte, key string) bool {
	if len(metadata) == 0 {
		return false
	}
	// Simple string search — avoids full JSON parse for performance.
	// Works for flat metadata objects.
	return strings.Contains(string(metadata), `"`+key+`"`)
}

// stripTrailingThinkingFromLastAssistant removes thinking content from the
// metadata of the last assistant message. This prevents the API from receiving
// stale thinking that could confuse the next turn.
//
// Inspired by Claude Code's filterTrailingThinkingFromLastAssistant.
func stripTrailingThinkingFromLastAssistant(messages []chat.ChatMessage) []chat.ChatMessage {
	if len(messages) == 0 {
		return messages
	}

	// Find last assistant message.
	lastIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastIdx = i
			break
		}
	}
	if lastIdx < 0 {
		return messages
	}

	// If the last assistant has thinking metadata but also has content,
	// strip the thinking metadata to reduce token usage.
	msg := messages[lastIdx]
	if !hasMetadataKey(msg.Metadata, "thinkingContent") {
		return messages
	}
	if strings.TrimSpace(msg.Content) == "" {
		return messages // No content to keep — filterOrphaned should handle this.
	}

	// Create a copy with thinking stripped from metadata.
	result := make([]chat.ChatMessage, len(messages))
	copy(result, messages)
	result[lastIdx].Metadata = stripThinkingFromMetadata(msg.Metadata)
	return result
}

// stripThinkingFromMetadata removes the "thinkingContent" key from metadata JSON.
// Returns nil if metadata becomes empty.
func stripThinkingFromMetadata(metadata []byte) []byte {
	if len(metadata) == 0 {
		return nil
	}
	s := string(metadata)
	// Remove "thinkingContent":"..." pattern.
	// This is a simple approach that works for our structured metadata format.
	if idx := strings.Index(s, `"thinkingContent"`); idx >= 0 {
		// Find the key-value pair boundaries.
		// For now, set metadata to nil to avoid complex JSON manipulation.
		// The thinking content is only used for display, not for API calls.
		return nil
	}
	return metadata
}

// filterWhitespaceOnlyMessages removes messages with only whitespace content
// and no tool calls. These serve no purpose in the conversation.
func filterWhitespaceOnlyMessages(messages []chat.ChatMessage) []chat.ChatMessage {
	result := make([]chat.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 && msg.MessageType != chat.MessageTypeToolResult {
			// Skip whitespace-only messages (except tool results which may be empty).
			continue
		}
		result = append(result, msg)
	}
	return result
}

// sanitizeEmptyToolResults ensures tool_result messages have non-empty content.
// The API may reject empty tool results. Empty results get a descriptive placeholder.
//
// Inspired by Claude Code's empty result handling.
func sanitizeEmptyToolResults(messages []chat.ChatMessage) []chat.ChatMessage {
	result := make([]chat.ChatMessage, len(messages))
	copy(result, messages)

	for i := range result {
		if result[i].MessageType != chat.MessageTypeToolResult {
			continue
		}
		if strings.TrimSpace(result[i].Content) == "" {
			result[i].Content = "[Tool executed successfully with no output]"
		}
	}
	return result
}

// NormalizeAIMessagesForAPI applies the same normalization pipeline to ai.Message
// slices (used when preparing messages for the LLM call directly).
func NormalizeAIMessagesForAPI(messages []ai.Message) []ai.Message {
	if len(messages) == 0 {
		return messages
	}

	result := messages

	// Merge consecutive same-role messages.
	result = mergeConsecutiveAIMessages(result)

	// Filter whitespace-only messages.
	result = filterWhitespaceOnlyAIMessages(result)

	return result
}

// mergeConsecutiveAIMessages merges consecutive messages with the same role.
func mergeConsecutiveAIMessages(messages []ai.Message) []ai.Message {
	if len(messages) <= 1 {
		return messages
	}

	result := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		if len(result) == 0 || result[len(result)-1].Role != msg.Role {
			result = append(result, msg)
			continue
		}
		prev := &result[len(result)-1]
		if prev.Content != "" && msg.Content != "" {
			prev.Content = prev.Content + "\n\n" + msg.Content
		} else if msg.Content != "" {
			prev.Content = msg.Content
		}
		// Merge tool calls for assistant messages.
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			prev.ToolCalls = append(prev.ToolCalls, msg.ToolCalls...)
		}
	}
	return result
}

// filterWhitespaceOnlyAIMessages removes ai.Messages with only whitespace.
func filterWhitespaceOnlyAIMessages(messages []ai.Message) []ai.Message {
	result := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 && msg.ToolCallID == "" {
			continue
		}
		result = append(result, msg)
	}
	return result
}
