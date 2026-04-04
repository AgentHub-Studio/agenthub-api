package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- NormalizeMessagesForAPI ---

func TestNormalize_Empty(t *testing.T) {
	result := agentic.NormalizeMessagesForAPI(nil)
	assert.Nil(t, result)
}

func TestNormalize_SingleMessage(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	assert.Len(t, result, 1)
	assert.Equal(t, "hello", result[0].Content)
}

// --- Merge consecutive user messages ---

func TestNormalize_MergesConsecutiveUsers(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "first", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "second", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "reply", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Contains(t, result[0].Content, "first")
	assert.Contains(t, result[0].Content, "second")
	assert.Equal(t, "reply", result[1].Content)
}

func TestNormalize_ThreeConsecutiveUsers(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "a", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "b", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "c", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 1)
	assert.Contains(t, result[0].Content, "a")
	assert.Contains(t, result[0].Content, "b")
	assert.Contains(t, result[0].Content, "c")
}

func TestNormalize_AlternatingNotMerged(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "q1", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "a1", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "q2", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	assert.Len(t, result, 3)
}

// --- Merge consecutive assistant messages ---

func TestNormalize_MergesConsecutiveAssistants(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "part 1", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "part 2", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Contains(t, result[1].Content, "part 1")
	assert.Contains(t, result[1].Content, "part 2")
}

// --- Filter orphaned thinking messages ---

func TestNormalize_FiltersOrphanedThinking(t *testing.T) {
	thinkingMeta, _ := json.Marshal(map[string]any{"thinkingContent": "I am thinking..."})
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "", Metadata: thinkingMeta, MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "real reply", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	// Orphaned thinking-only message should be removed.
	require.Len(t, result, 2)
	assert.Equal(t, "user", result[0].Role)
	assert.Contains(t, result[1].Content, "real reply")
}

func TestNormalize_KeepsThinkingWithContent(t *testing.T) {
	thinkingMeta, _ := json.Marshal(map[string]any{"thinkingContent": "reasoning"})
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "visible answer", Metadata: thinkingMeta, MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	// Has content → kept (but thinking stripped from last assistant).
	require.Len(t, result, 2)
	assert.Equal(t, "visible answer", result[1].Content)
}

// --- Strip trailing thinking ---

func TestNormalize_StripsTrailingThinkingFromLastAssistant(t *testing.T) {
	thinkingMeta, _ := json.Marshal(map[string]any{"thinkingContent": "long reasoning"})
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "question", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "answer", Metadata: thinkingMeta, MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 2)
	// Thinking metadata should be stripped from the last assistant.
	assert.Nil(t, result[1].Metadata)
	assert.Equal(t, "answer", result[1].Content)
}

func TestNormalize_KeepsThinkingOnNonLastAssistant(t *testing.T) {
	thinkingMeta, _ := json.Marshal(map[string]any{"thinkingContent": "reasoning"})
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "q1", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "a1", Metadata: thinkingMeta, MessageType: chat.MessageTypeText},
		{Role: "user", Content: "q2", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "a2", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 4)
	// Non-last assistant should keep its metadata.
	assert.NotNil(t, result[1].Metadata)
}

// --- Filter whitespace-only messages ---

func TestNormalize_FiltersWhitespaceOnly(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "   ", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "world", MessageType: chat.MessageTypeText},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Equal(t, "hello", result[0].Content)
	assert.Equal(t, "world", result[1].Content)
}

func TestNormalize_KeepsToolCallsWithWhitespaceContent(t *testing.T) {
	toolCalls, _ := json.Marshal([]map[string]any{{"id": "tc_1", "type": "function"}})
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "", ToolCalls: toolCalls, MessageType: chat.MessageTypeToolUse},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	assert.Len(t, result, 2) // Tool call message kept despite empty content.
}

// --- Sanitize empty tool results ---

func TestNormalize_SanitizesEmptyToolResults(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "run tool", MessageType: chat.MessageTypeText},
		{Role: "tool", Content: "", MessageType: chat.MessageTypeToolResult},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Contains(t, result[1].Content, "no output")
}

func TestNormalize_KeepsNonEmptyToolResults(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "run", MessageType: chat.MessageTypeText},
		{Role: "tool", Content: "result data", MessageType: chat.MessageTypeToolResult},
	}
	result := agentic.NormalizeMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Equal(t, "result data", result[1].Content)
}

// --- NormalizeAIMessagesForAPI ---

func TestNormalizeAI_MergesConsecutiveSameRole(t *testing.T) {
	msgs := []ai.Message{
		{Role: "user", Content: "part 1"},
		{Role: "user", Content: "part 2"},
		{Role: "assistant", Content: "reply"},
	}
	result := agentic.NormalizeAIMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Contains(t, result[0].Content, "part 1")
	assert.Contains(t, result[0].Content, "part 2")
}

func TestNormalizeAI_MergesToolCalls(t *testing.T) {
	msgs := []ai.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "thinking", ToolCalls: []ai.ToolCall{{ID: "tc_1"}}},
		{Role: "assistant", Content: "", ToolCalls: []ai.ToolCall{{ID: "tc_2"}}},
	}
	result := agentic.NormalizeAIMessagesForAPI(msgs)
	require.Len(t, result, 2)
	assert.Len(t, result[1].ToolCalls, 2)
}

func TestNormalizeAI_FiltersWhitespaceOnly(t *testing.T) {
	msgs := []ai.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "  \n  "},
		{Role: "user", Content: "world"},
	}
	result := agentic.NormalizeAIMessagesForAPI(msgs)
	require.Len(t, result, 2)
}

func TestNormalizeAI_KeepsToolCallID(t *testing.T) {
	msgs := []ai.Message{
		{Role: "user", Content: "go"},
		{Role: "tool", Content: "", ToolCallID: "tc_1"},
	}
	result := agentic.NormalizeAIMessagesForAPI(msgs)
	assert.Len(t, result, 2) // Tool result with ID kept despite empty content.
}

func TestNormalizeAI_Empty(t *testing.T) {
	result := agentic.NormalizeAIMessagesForAPI(nil)
	assert.Nil(t, result)
}

// --- Full pipeline integration ---

func TestNormalize_FullPipeline(t *testing.T) {
	thinkingMeta, _ := json.Marshal(map[string]any{"thinkingContent": "hmm"})
	toolCalls, _ := json.Marshal([]map[string]any{{"id": "tc_1", "type": "function"}})

	msgs := []chat.ChatMessage{
		{Role: "user", Content: "first query", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "addendum", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "", Metadata: thinkingMeta, MessageType: chat.MessageTypeText}, // Orphaned thinking
		{Role: "assistant", Content: "I'll help", ToolCalls: toolCalls, MessageType: chat.MessageTypeToolUse},
		{Role: "tool", Content: "", MessageType: chat.MessageTypeToolResult}, // Empty result
		{Role: "assistant", Content: "   ", MessageType: chat.MessageTypeText}, // Whitespace
		{Role: "assistant", Content: "final answer", Metadata: thinkingMeta, MessageType: chat.MessageTypeText},
	}

	result := agentic.NormalizeMessagesForAPI(msgs)

	// Expected:
	// 1. "first query\n\naddendum" (merged users)
	// 2. "I'll help" with tool_calls (orphaned thinking removed, whitespace removed, assistants merged)
	// 3. tool result "[Tool executed successfully with no output]"
	// 4. "final answer" (trailing thinking stripped)

	require.Len(t, result, 4)
	assert.Contains(t, result[0].Content, "first query")
	assert.Contains(t, result[0].Content, "addendum")
	assert.Equal(t, "user", result[0].Role)

	assert.Equal(t, "assistant", result[1].Role)
	assert.Contains(t, result[1].Content, "I'll help")

	assert.Equal(t, "tool", result[2].Role)
	assert.Contains(t, result[2].Content, "no output")

	assert.Equal(t, "assistant", result[3].Role)
	assert.Contains(t, result[3].Content, "final answer")
	assert.Nil(t, result[3].Metadata, "trailing thinking should be stripped")
}
