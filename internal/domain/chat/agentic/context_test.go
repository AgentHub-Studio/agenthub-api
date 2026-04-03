package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- mock summarizer ---

type mockSummarizer struct {
	result string
	err    error
}

func (m *mockSummarizer) Summarize(_ context.Context, _ string) (string, error) {
	return m.result, m.err
}

// --- EstimateTokens ---

func TestEstimateTokens_Empty(t *testing.T) {
	tokens := agentic.EstimateTokens(nil)
	assert.Equal(t, 0, tokens)
}

func TestEstimateTokens_SingleTextMessage(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "Hello world", MessageType: chat.MessageTypeText},
	}
	tokens := agentic.EstimateTokens(msgs)
	// 4 (role overhead) + len("Hello world")/4 = 4 + 2 = 6
	assert.Equal(t, 6, tokens)
}

func TestEstimateTokens_MultipleMessages(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "Hello world"},
		{Role: "assistant", Content: "Hi there, how can I help you today?"},
	}
	tokens := agentic.EstimateTokens(msgs)
	// msg1: 4 + 11/4=2 = 6
	// msg2: 4 + 34/4=8 = 12
	assert.Equal(t, 18, tokens)
}

func TestEstimateTokens_WithToolCalls(t *testing.T) {
	toolCalls := json.RawMessage(`[{"id":"tc_1","name":"search","input":{"q":"test"}}]`)
	msgs := []chat.ChatMessage{
		{Role: "assistant", Content: "Let me search", ToolCalls: toolCalls},
	}
	tokens := agentic.EstimateTokens(msgs)
	// 4 + 13/4=3 + len(toolCalls)/4
	expected := 4 + len("Let me search")/4 + len(toolCalls)/4
	assert.Equal(t, expected, tokens)
}

// --- EstimateStringTokens ---

func TestEstimateStringTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, agentic.EstimateStringTokens(""))
}

func TestEstimateStringTokens_Short(t *testing.T) {
	// len("hello")/4 + 3 = 1 + 3 = 4
	assert.Equal(t, 4, agentic.EstimateStringTokens("hello"))
}

func TestEstimateStringTokens_Long(t *testing.T) {
	s := "This is a longer string that should produce a higher token estimate for testing purposes."
	expected := len(s)/4 + 3
	assert.Equal(t, expected, agentic.EstimateStringTokens(s))
}

// --- GetContextWindowSize ---

func TestGetContextWindowSize_ExactMatch(t *testing.T) {
	assert.Equal(t, 200000, agentic.GetContextWindowSize("claude-sonnet-4", 0))
	assert.Equal(t, 128000, agentic.GetContextWindowSize("gpt-4o", 0))
	assert.Equal(t, 8192, agentic.GetContextWindowSize("gpt-4", 0))
}

func TestGetContextWindowSize_PrefixMatch(t *testing.T) {
	assert.Equal(t, 200000, agentic.GetContextWindowSize("claude-sonnet-4-20250514", 0))
	assert.Equal(t, 128000, agentic.GetContextWindowSize("gpt-4o-2024-05-13", 0))
}

func TestGetContextWindowSize_Fallback(t *testing.T) {
	assert.Equal(t, 32000, agentic.GetContextWindowSize("unknown-model", 32000))
}

func TestGetContextWindowSize_DefaultWhenNoFallback(t *testing.T) {
	assert.Equal(t, 200000, agentic.GetContextWindowSize("unknown-model", 0))
}

// --- ShouldCompact ---

func TestShouldCompact_BelowThreshold(t *testing.T) {
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "short"},
	}
	cfg := agentic.DefaultRunConfig()
	assert.False(t, agentic.ShouldCompact(msgs, 100, cfg))
}

func TestShouldCompact_AboveThreshold(t *testing.T) {
	// Create a message with content large enough to exceed threshold.
	// Default: ContextWindowSize=200000, CompactThreshold=0.75 → threshold=150000 tokens
	// We need > 150000 tokens ≈ 600000 chars.
	bigContent := make([]byte, 700000)
	for i := range bigContent {
		bigContent[i] = 'a'
	}
	msgs := []chat.ChatMessage{
		{Role: "user", Content: string(bigContent)},
	}
	cfg := agentic.DefaultRunConfig()
	assert.True(t, agentic.ShouldCompact(msgs, 0, cfg))
}

func TestShouldCompact_CustomThreshold(t *testing.T) {
	// Small context window + low threshold.
	cfg := agentic.RunConfig{
		ContextWindowSize: 100,
		CompactThreshold:  0.5, // threshold = 50 tokens
	}
	// Message: 4 + 200/4 = 54 tokens → exceeds 50.
	content := make([]byte, 200)
	for i := range content {
		content[i] = 'x'
	}
	msgs := []chat.ChatMessage{
		{Role: "user", Content: string(content)},
	}
	assert.True(t, agentic.ShouldCompact(msgs, 0, cfg))
}

// --- Compact ---

func TestCompact_TooFewMessages(t *testing.T) {
	cm := agentic.NewContextManager()
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	result, err := cm.Compact(context.Background(), msgs, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.OriginalCount)
	assert.Equal(t, 2, result.CompactedCount)
	assert.Equal(t, msgs, result.Messages)
	assert.Empty(t, result.Summary)
}

func TestCompact_NilSummarizer(t *testing.T) {
	cm := agentic.NewContextManager()
	msgs := make([]chat.ChatMessage, 10)
	for i := range msgs {
		msgs[i] = chat.ChatMessage{Role: "user", Content: "msg"}
	}
	result, err := cm.Compact(context.Background(), msgs, nil)
	require.NoError(t, err)
	assert.Equal(t, 10, result.CompactedCount) // unchanged
}

func TestCompact_Success(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "First question"},
		{Role: "assistant", Content: "First answer"},
		{Role: "user", Content: "Second question"},
		{Role: "assistant", Content: "Second answer"},
		{Role: "user", Content: "Third question"},
		{Role: "assistant", Content: "Third answer"},
		{Role: "user", Content: "Fourth question"},
		{Role: "assistant", Content: "Fourth answer"},
	}

	summarizer := &mockSummarizer{result: "Summary: discussed questions 1-4"}
	result, err := cm.Compact(context.Background(), msgs, summarizer)

	require.NoError(t, err)
	assert.Equal(t, 8, result.OriginalCount)
	assert.Equal(t, 5, result.CompactedCount) // 1 summary + 4 tail
	assert.Equal(t, "Summary: discussed questions 1-4", result.Summary)

	// First message should be the compact summary.
	assert.Equal(t, "system", result.Messages[0].Role)
	assert.Equal(t, chat.MessageTypeCompactSummary, result.Messages[0].MessageType)
	assert.Equal(t, "Summary: discussed questions 1-4", result.Messages[0].Content)

	// Tail should be preserved.
	assert.Equal(t, "Third question", result.Messages[1].Content)
	assert.Equal(t, "Fourth answer", result.Messages[4].Content)
}

func TestCompact_PreservesToolCallChain(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "old message 1"},
		{Role: "user", Content: "old message 2"},
		{Role: "assistant", Content: "I'll use a tool", MessageType: chat.MessageTypeToolUse},
		{Role: "tool", Content: "tool output", MessageType: chat.MessageTypeToolResult},
		{Role: "assistant", Content: "Based on the tool result"},
		{Role: "user", Content: "thanks"},
		{Role: "assistant", Content: "you're welcome"},
		{Role: "user", Content: "bye"},
	}

	summarizer := &mockSummarizer{result: "Summary of old messages"}
	result, err := cm.Compact(context.Background(), msgs, summarizer)

	require.NoError(t, err)
	// The tail starts at index 4 (8-4=4), but msgs[4] is not a tool_result,
	// so no adjustment needed. Summary covers msgs[0:4].
	assert.Equal(t, 8, result.OriginalCount)
	assert.Equal(t, 5, result.CompactedCount) // 1 summary + 4 tail
}

func TestCompact_ToolResultAtTailBoundary(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "old message 1"},
		{Role: "user", Content: "old message 2"},
		{Role: "user", Content: "old message 3"},
		{Role: "assistant", Content: "calling tool", MessageType: chat.MessageTypeToolUse},
		{Role: "tool", Content: "result", MessageType: chat.MessageTypeToolResult},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "thanks"},
		{Role: "assistant", Content: "bye"},
	}

	summarizer := &mockSummarizer{result: "Summary"}
	result, err := cm.Compact(context.Background(), msgs, summarizer)

	require.NoError(t, err)
	// splitIdx starts at 4, but msgs[4] is tool_result → move back to 3 (tool_use).
	// toSummarize = msgs[0:3], tail = msgs[3:]
	assert.Equal(t, 6, result.CompactedCount) // 1 summary + 5 tail
	assert.Equal(t, chat.MessageTypeToolUse, result.Messages[1].MessageType)
}

func TestCompact_SummarizerError(t *testing.T) {
	cm := agentic.NewContextManager()
	msgs := make([]chat.ChatMessage, 10)
	for i := range msgs {
		msgs[i] = chat.ChatMessage{Role: "user", Content: "msg"}
	}

	summarizer := &mockSummarizer{err: errors.New("LLM unavailable")}
	_, err := cm.Compact(context.Background(), msgs, summarizer)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM unavailable")
}

func TestCompact_AllToolResults_NoSummarization(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4
	// If all messages at the boundary are tool_results, splitIdx reaches 0
	// and toSummarize is empty → return original.
	msgs := []chat.ChatMessage{
		{Role: "tool", Content: "r1", MessageType: chat.MessageTypeToolResult},
		{Role: "tool", Content: "r2", MessageType: chat.MessageTypeToolResult},
		{Role: "tool", Content: "r3", MessageType: chat.MessageTypeToolResult},
		{Role: "tool", Content: "r4", MessageType: chat.MessageTypeToolResult},
		{Role: "tool", Content: "r5", MessageType: chat.MessageTypeToolResult},
	}

	summarizer := &mockSummarizer{result: "should not be called"}
	result, err := cm.Compact(context.Background(), msgs, summarizer)

	require.NoError(t, err)
	assert.Equal(t, 5, result.CompactedCount)
	assert.Equal(t, msgs, result.Messages)
}

// --- NewContextManager ---

func TestNewContextManager_Defaults(t *testing.T) {
	cm := agentic.NewContextManager()
	assert.Equal(t, 4, cm.TailSize)
}

// --- ReactiveCompact ---

func makeMessages(n int, contentLen int) []chat.ChatMessage {
	content := make([]byte, contentLen)
	for i := range content {
		content[i] = 'x'
	}
	msgs := make([]chat.ChatMessage, n)
	for i := 0; i < n; i++ {
		msgs[i] = chat.ChatMessage{
			Role:        "user",
			Content:     string(content),
			MessageType: chat.MessageTypeText,
		}
	}
	return msgs
}

func reactiveCompactConfig(windowSize int) agentic.RunConfig {
	cfg := agentic.DefaultRunConfig()
	cfg.Model = "test-model" // unknown model so GetContextWindowSize uses fallback
	cfg.ContextWindowSize = windowSize
	return cfg
}

func TestReactiveCompact_NoCompactionNeeded(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(200000)

	msgs := makeMessages(5, 100) // ~125 tokens total, well below 75%
	result, err := cm.ReactiveCompact(context.Background(), msgs, 0, cfg, nil)

	require.NoError(t, err)
	assert.Equal(t, len(msgs), result.CompactedCount)
	assert.Equal(t, agentic.CompactStage(""), result.Stage) // no stage applied
}

func TestReactiveCompact_ToolResultTruncation(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	// Create messages where tool result is outside the tail (tailSize=4).
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "old question", MessageType: chat.MessageTypeText},
		{Role: "tool", Content: string(make([]byte, 400)), MessageType: chat.MessageTypeToolResult},
		{Role: "assistant", Content: "old answer", MessageType: chat.MessageTypeText},
		// -- tail boundary (last 4 messages) --
		{Role: "user", Content: "question 2", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "answer 2", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "follow up", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "response", MessageType: chat.MessageTypeText},
	}

	// System tokens push us into 75-80% range.
	result, err := cm.ReactiveCompact(context.Background(), msgs, 610, cfg, nil)

	require.NoError(t, err)
	assert.Equal(t, agentic.StageToolResultTruncation, result.Stage)
	// The tool result at index 1 (outside tail) should be truncated (200 chars + "\n[truncated]").
	assert.LessOrEqual(t, len(result.Messages[1].Content), 220)
}

func TestReactiveCompact_HistorySnip(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	msgs := makeMessages(10, 40) // ~10 * (40/4 + 4) = 140 tokens
	// System tokens push us into 80-85% range (140 + 680 = 820 / 1000 = 82%).
	result, err := cm.ReactiveCompact(context.Background(), msgs, 680, cfg, nil)

	require.NoError(t, err)
	assert.Equal(t, agentic.StageHistorySnip, result.Stage)
	assert.Less(t, result.CompactedCount, result.OriginalCount)
}

func TestReactiveCompact_FullSummarization(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	msgs := makeMessages(10, 40) // ~140 tokens
	summarizer := &mockSummarizer{result: "Summary of conversation"}

	// System tokens push us above 90% (140 + 770 = 910 / 1000 = 91%).
	result, err := cm.ReactiveCompact(context.Background(), msgs, 770, cfg, summarizer)

	require.NoError(t, err)
	assert.Equal(t, agentic.StageFullSummarization, result.Stage)
	assert.Less(t, result.CompactedCount, result.OriginalCount)
}
