package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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

// --- EstimateStringTokensForType ---

func TestEstimateStringTokensForType_JSON(t *testing.T) {
	s := `{"key":"value","arr":[1,2,3]}`
	tokens := agentic.EstimateStringTokensForType(s, ".json")
	// JSON uses 2 bytes/token: len(s)/2 + 3
	expected := len(s)/2 + 3
	assert.Equal(t, expected, tokens)
}

func TestEstimateStringTokensForType_JSONL(t *testing.T) {
	s := `{"line":1}`
	tokens := agentic.EstimateStringTokensForType(s, ".jsonl")
	expected := len(s)/2 + 3
	assert.Equal(t, expected, tokens)
}

func TestEstimateStringTokensForType_GeoJSON(t *testing.T) {
	s := `{"type":"Feature"}`
	tokens := agentic.EstimateStringTokensForType(s, ".geojson")
	expected := len(s)/2 + 3
	assert.Equal(t, expected, tokens)
}

func TestEstimateStringTokensForType_NonJSON(t *testing.T) {
	s := "func main() { fmt.Println(hello) }"
	tokens := agentic.EstimateStringTokensForType(s, ".go")
	// Non-JSON uses 4 bytes/token: len(s)/4 + 3
	expected := len(s)/4 + 3
	assert.Equal(t, expected, tokens)
}

func TestEstimateStringTokensForType_EmptyString(t *testing.T) {
	assert.Equal(t, 0, agentic.EstimateStringTokensForType("", ".json"))
	assert.Equal(t, 0, agentic.EstimateStringTokensForType("", ".go"))
}

func TestEstimateStringTokensForType_CaseInsensitive(t *testing.T) {
	s := `{"test":true}`
	tokensLower := agentic.EstimateStringTokensForType(s, ".json")
	tokensUpper := agentic.EstimateStringTokensForType(s, ".JSON")
	assert.Equal(t, tokensLower, tokensUpper)
}

func TestEstimateStringTokensForType_UnknownExtFallsBackTo4(t *testing.T) {
	s := "some content here"
	tokens := agentic.EstimateStringTokensForType(s, ".txt")
	// Should match standard estimation: len/4 + 3
	assert.Equal(t, agentic.EstimateStringTokens(s), tokens)
}

func TestEstimateStringTokens_Long(t *testing.T) {
	s := "This is a longer string that should produce a higher token estimate for testing purposes."
	expected := len(s)/4 + 3
	assert.Equal(t, expected, agentic.EstimateStringTokens(s))
}

// --- GetContextWindowSize ---

func TestGetContextWindowSize_ExactMatch(t *testing.T) {
	assert.Equal(t, 1000000, agentic.GetContextWindowSize("claude-sonnet-4-6", 0))
	assert.Equal(t, 1000000, agentic.GetContextWindowSize("claude-opus-4-6", 0))
	assert.Equal(t, 200000, agentic.GetContextWindowSize("claude-sonnet-4", 0))
	assert.Equal(t, 131072, agentic.GetContextWindowSize("openai/gpt-oss-20b", 0))
	assert.Equal(t, 128000, agentic.GetContextWindowSize("gpt-4o", 0))
	assert.Equal(t, 8192, agentic.GetContextWindowSize("gpt-4", 0))
}

func TestGetContextWindowSize_PrefixMatch(t *testing.T) {
	assert.Equal(t, 1000000, agentic.GetContextWindowSize("claude-sonnet-4-6-20260217", 0))
	assert.Equal(t, 200000, agentic.GetContextWindowSize("claude-sonnet-4-20250514", 0))
	assert.Equal(t, 128000, agentic.GetContextWindowSize("llama3.3:70b", 0))
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

// --- SnipOldHistory with tool pair preservation ---

func toolCallID(id string) *string { return &id }

func TestHistorySnip_PreservesToolPairs(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	// Build a history where a tool_result in the kept range depends on
	// a tool_use that would be snipped without adjustment.
	toolCalls := json.RawMessage(`[{"id":"tc_1","type":"function","function":{"name":"search","arguments":"{}"}}]`)

	msgs := []chat.ChatMessage{
		{Role: "user", Content: "old msg 1", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "old msg 2", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "calling tool", MessageType: chat.MessageTypeToolUse, ToolCalls: toolCalls},
		{Role: "tool", Content: "result", MessageType: chat.MessageTypeToolResult, ToolCallID: toolCallID("tc_1")},
		{Role: "assistant", Content: "here is the answer", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "thanks", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "welcome", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "bye", MessageType: chat.MessageTypeText},
	}

	// Push into 80-85% range for StageHistorySnip.
	// msgs tokens ~= 8*(4 + ~10/4) = ~56; keep = tailSize*2 = 8 → all kept normally.
	// Use smaller keep to force a snip. We test via ReactiveCompact.
	// Instead, test the snip directly by pushing usage into the right range.
	// With windowSize=100, msgs~56 tokens + systemTokens to reach 80-85%.
	result, err := cm.ReactiveCompact(context.Background(), msgs, 25, cfg, nil)
	require.NoError(t, err)

	if result.Stage == agentic.StageHistorySnip {
		// Verify no orphaned tool_result: for every tool_result, a matching tool_use must exist.
		for _, m := range result.Messages {
			if m.MessageType == chat.MessageTypeToolResult && m.ToolCallID != nil {
				found := false
				for _, m2 := range result.Messages {
					if m2.MessageType == chat.MessageTypeToolUse && len(m2.ToolCalls) > 0 {
						var tcs []struct {
							ID string `json:"id"`
						}
						if err := json.Unmarshal(m2.ToolCalls, &tcs); err == nil {
							for _, tc := range tcs {
								if tc.ID == *m.ToolCallID {
									found = true
								}
							}
						}
					}
				}
				assert.True(t, found, "tool_result %s has no matching tool_use", *m.ToolCallID)
			}
		}
	}
}

func TestHistorySnip_NoOrphanedToolResults(t *testing.T) {
	// Directly test that snipOldHistory (via HistorySnip stage) pulls in
	// the tool_use when the split point falls between a tool_use and tool_result.
	cm := agentic.NewContextManager() // TailSize = 4
	cfg := reactiveCompactConfig(500)

	toolCalls := json.RawMessage(`[{"id":"tc_A","type":"function","function":{"name":"search","arguments":"{}"}}]`)

	msgs := []chat.ChatMessage{
		// These get snipped:
		{Role: "user", Content: string(make([]byte, 80)), MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: string(make([]byte, 80)), MessageType: chat.MessageTypeText},
		{Role: "user", Content: string(make([]byte, 80)), MessageType: chat.MessageTypeText},
		// tool_use that MUST be preserved if tool_result is in the kept range:
		{Role: "assistant", Content: "tool call", MessageType: chat.MessageTypeToolUse, ToolCalls: toolCalls},
		// tool_result that would be kept:
		{Role: "tool", Content: "output", MessageType: chat.MessageTypeToolResult, ToolCallID: toolCallID("tc_A")},
		// tail messages:
		{Role: "assistant", Content: "answer", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "ok", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "done", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "bye", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "goodbye", MessageType: chat.MessageTypeText},
	}

	// Push into 80-85% range.
	// Approx tokens: 10 msgs * (4 + ~20) = ~240 from messages.
	// Need 80-85% of 500 → 400-425 total → systemTokens ~170.
	result, err := cm.ReactiveCompact(context.Background(), msgs, 170, cfg, nil)
	require.NoError(t, err)

	if result.Stage == agentic.StageHistorySnip {
		// If tool_result tc_A is in the output, tool_use must also be.
		hasToolResult := false
		hasToolUse := false
		for _, m := range result.Messages {
			if m.MessageType == chat.MessageTypeToolResult && m.ToolCallID != nil && *m.ToolCallID == "tc_A" {
				hasToolResult = true
			}
			if m.MessageType == chat.MessageTypeToolUse && len(m.ToolCalls) > 0 {
				var tcs []struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(m.ToolCalls, &tcs)
				for _, tc := range tcs {
					if tc.ID == "tc_A" {
						hasToolUse = true
					}
				}
			}
		}
		if hasToolResult {
			assert.True(t, hasToolUse, "tool_result tc_A kept but matching tool_use was snipped")
		}
	}
}

func TestReactiveCompact_Microcompact(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	msgs := makeMessages(10, 40) // ~140 tokens
	// System tokens push us into 85-90% range (140 + 720 = 860 / 1000 = 86%).
	result, err := cm.ReactiveCompact(context.Background(), msgs, 720, cfg, nil)

	require.NoError(t, err)
	assert.Equal(t, agentic.StageMicrocompact, result.Stage)
	// Non-tail messages should be truncated to 100 chars.
	for i := 0; i < len(result.Messages)-cm.TailSize; i++ {
		assert.LessOrEqual(t, len(result.Messages[i].Content), 103) // 100 + "..."
	}
}

// --- VerifyCompaction ---

func TestVerifyCompaction_NoStage(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	// No stage applied → returns as-is.
	result := &agentic.ReactiveCompactResult{
		CompactResult: agentic.CompactResult{
			Messages: makeMessages(5, 40),
		},
	}
	verified, err := cm.VerifyCompaction(context.Background(), result, 0, cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, result, verified)
}

func TestVerifyCompaction_EffectiveCompaction(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	// Stage 1 applied and result is below 75% → no escalation.
	result := &agentic.ReactiveCompactResult{
		CompactResult: agentic.CompactResult{
			Messages:       makeMessages(5, 40), // ~70 tokens
			OriginalCount:  10,
			CompactedCount: 5,
		},
		Stage: agentic.StageToolResultTruncation,
	}
	// 70 tokens + 0 system = 7% of 1000 → well below 75%.
	verified, err := cm.VerifyCompaction(context.Background(), result, 0, cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, agentic.StageToolResultTruncation, verified.Stage)
}

func TestVerifyCompaction_Escalates(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	// Stage 1 applied but we're still above 75% → escalates.
	// Create messages that push into 80-85% range after Stage 1.
	bigMsgs := makeMessages(10, 320) // ~10 * (4 + 80) = ~840 tokens = 84%
	result := &agentic.ReactiveCompactResult{
		CompactResult: agentic.CompactResult{
			Messages:       bigMsgs,
			OriginalCount:  15,
			CompactedCount: 10,
		},
		Stage: agentic.StageToolResultTruncation,
	}
	// 840 tokens + 0 system = 84% of 1000 → above 75%, escalates to HistorySnip (80-85%).
	verified, err := cm.VerifyCompaction(context.Background(), result, 0, cfg, nil)
	require.NoError(t, err)
	// Should have escalated to a higher stage.
	assert.NotEqual(t, agentic.StageToolResultTruncation, verified.Stage)
}

func TestVerifyCompaction_FullSummarizationNoEscalation(t *testing.T) {
	cm := agentic.NewContextManager()
	cfg := reactiveCompactConfig(1000)

	// Full summarization can't escalate further → returns as-is.
	result := &agentic.ReactiveCompactResult{
		CompactResult: agentic.CompactResult{
			Messages:       makeMessages(5, 400), // still big
			OriginalCount:  20,
			CompactedCount: 5,
		},
		Stage: agentic.StageFullSummarization,
	}
	verified, err := cm.VerifyCompaction(context.Background(), result, 0, cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, agentic.StageFullSummarization, verified.Stage)
}

// --- CompactForPTLRetry ---

func TestCompactForPTLRetry_PreciseTruncation(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4

	// 10 messages, each ~50 chars → ~(50/4 + 4) = ~16 tokens each.
	msgs := makeMessages(10, 50)

	// Error says 200 tokens > 180 → gap of 20 tokens → need to drop ~2 messages (16 each).
	errMsg := "Prompt is too long: 200 tokens > 180"
	result := cm.CompactForPTLRetry(msgs, errMsg)

	require.NotNil(t, result)
	assert.Less(t, len(result), len(msgs))
	// Should have dropped at least 1 message from the head.
	assert.GreaterOrEqual(t, len(msgs)-len(result), 1)
	// Tail (last 4) should still be present.
	assert.Equal(t, msgs[len(msgs)-1].Content, result[len(result)-1].Content)
}

func TestCompactForPTLRetry_FallbackDrops20Percent(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4

	msgs := makeMessages(10, 50)

	// Error with no parseable token counts → falls back to 20%.
	errMsg := "context_length_exceeded: prompt too large"
	result := cm.CompactForPTLRetry(msgs, errMsg)

	require.NotNil(t, result)
	// 20% of 10 = 2 messages dropped.
	assert.Equal(t, 8, len(result))
}

func TestCompactForPTLRetry_TooFewMessages(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4

	// Only 4 messages → can't truncate (need more than TailSize+1).
	msgs := makeMessages(4, 50)
	result := cm.CompactForPTLRetry(msgs, "Prompt is too long: 200 tokens > 180")

	assert.Nil(t, result)
}

func TestCompactForPTLRetry_PreservesToolPairs(t *testing.T) {
	cm := agentic.NewContextManager() // TailSize = 4

	toolCalls := json.RawMessage(`[{"id":"tc_1","type":"function","function":{"name":"search","arguments":"{}"}}]`)

	msgs := []chat.ChatMessage{
		{Role: "user", Content: string(make([]byte, 200)), MessageType: chat.MessageTypeText},
		{Role: "user", Content: string(make([]byte, 200)), MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "tool call", MessageType: chat.MessageTypeToolUse, ToolCalls: toolCalls},
		{Role: "tool", Content: "output", MessageType: chat.MessageTypeToolResult, ToolCallID: toolCallID("tc_1")},
		{Role: "assistant", Content: "answer", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "ok", MessageType: chat.MessageTypeText},
		{Role: "assistant", Content: "done", MessageType: chat.MessageTypeText},
		{Role: "user", Content: "bye", MessageType: chat.MessageTypeText},
	}

	// Unparseable error → 20% fallback → drop 1 message.
	// If drop point falls such that tool_result is orphaned, it should adjust.
	result := cm.CompactForPTLRetry(msgs, "context_length_exceeded")
	require.NotNil(t, result)

	// Verify no orphaned tool_results.
	for _, m := range result {
		if m.MessageType == chat.MessageTypeToolResult && m.ToolCallID != nil {
			found := false
			for _, m2 := range result {
				if m2.MessageType == chat.MessageTypeToolUse && len(m2.ToolCalls) > 0 {
					var tcs []struct {
						ID string `json:"id"`
					}
					if err := json.Unmarshal(m2.ToolCalls, &tcs); err == nil {
						for _, tc := range tcs {
							if tc.ID == *m.ToolCallID {
								found = true
							}
						}
					}
				}
			}
			assert.True(t, found, "tool_result %s has no matching tool_use", *m.ToolCallID)
		}
	}
}

// --- Compact circuit breaker ---

func TestCompactCircuitBreaker_InitiallyClosed(t *testing.T) {
	cm := agentic.NewContextManager()
	assert.False(t, cm.CompactCircuitOpen())
}

func TestCompactCircuitBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactFailure()
	cm.RecordCompactFailure()
	assert.False(t, cm.CompactCircuitOpen())
	cm.RecordCompactFailure() // 3rd consecutive
	assert.True(t, cm.CompactCircuitOpen())
}

func TestCompactCircuitBreaker_SuccessResets(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactFailure()
	cm.RecordCompactFailure()
	cm.RecordCompactSuccess() // resets
	assert.False(t, cm.CompactCircuitOpen())
	cm.RecordCompactFailure()
	assert.False(t, cm.CompactCircuitOpen()) // only 1 now
}

// --- AutoCompactTracking ---

func TestAutoCompactTracking_InitialState(t *testing.T) {
	cm := agentic.NewContextManager()
	tracking := cm.Tracking()
	assert.False(t, tracking.Compacted)
	assert.Equal(t, 0, tracking.TurnCounter)
	assert.Empty(t, tracking.TurnID)
	assert.Equal(t, 0, tracking.ConsecutiveFailures)
}

func TestAutoCompactTracking_SuccessSetsCompactedAndTurnID(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactSuccess()

	tracking := cm.Tracking()
	assert.True(t, tracking.Compacted)
	assert.NotEmpty(t, tracking.TurnID, "TurnID should be generated on success")
	assert.Equal(t, 0, tracking.TurnCounter)
	assert.Equal(t, 0, tracking.ConsecutiveFailures)
}

func TestAutoCompactTracking_TurnCounterIncrementsAfterCompact(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactSuccess()

	cm.RecordTurnAfterCompact()
	cm.RecordTurnAfterCompact()
	cm.RecordTurnAfterCompact()

	assert.Equal(t, 3, cm.Tracking().TurnCounter)
}

func TestAutoCompactTracking_TurnCounterNoOpBeforeCompact(t *testing.T) {
	cm := agentic.NewContextManager()
	// No compaction yet — turns should not count.
	cm.RecordTurnAfterCompact()
	cm.RecordTurnAfterCompact()

	assert.Equal(t, 0, cm.Tracking().TurnCounter)
}

func TestAutoCompactTracking_RecompactResetsTurnCounter(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactSuccess()
	cm.RecordTurnAfterCompact()
	cm.RecordTurnAfterCompact()
	assert.Equal(t, 2, cm.Tracking().TurnCounter)

	// Second compaction resets counter and generates new TurnID.
	firstTurnID := cm.Tracking().TurnID
	cm.RecordCompactSuccess()
	assert.Equal(t, 0, cm.Tracking().TurnCounter)
	assert.NotEqual(t, firstTurnID, cm.Tracking().TurnID)
}

func TestAutoCompactTracking_RecompactionInfo_NoCompaction(t *testing.T) {
	cm := agentic.NewContextManager()
	info := cm.Tracking().RecompactionInfo()

	assert.False(t, info.IsRecompactionInChain)
	assert.Equal(t, 0, info.TurnsSincePreviousCompact)
	assert.Empty(t, info.PreviousCompactTurnID)
}

func TestAutoCompactTracking_RecompactionInfo_AfterCompact(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactSuccess()
	cm.RecordTurnAfterCompact()
	cm.RecordTurnAfterCompact()

	info := cm.Tracking().RecompactionInfo()
	assert.True(t, info.IsRecompactionInChain)
	assert.Equal(t, 2, info.TurnsSincePreviousCompact)
	assert.NotEmpty(t, info.PreviousCompactTurnID)
}

func TestAutoCompactTracking_RecompactionInfoFromPtr_Nil(t *testing.T) {
	info := agentic.RecompactionInfoFromPtr(nil)
	assert.False(t, info.IsRecompactionInChain)
	assert.Equal(t, -1, info.TurnsSincePreviousCompact)
}

func TestAutoCompactTracking_ResetTracking(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactSuccess()
	cm.RecordTurnAfterCompact()
	assert.True(t, cm.Tracking().Compacted)

	cm.ResetTracking()
	tracking := cm.Tracking()
	assert.False(t, tracking.Compacted)
	assert.Equal(t, 0, tracking.TurnCounter)
	assert.Empty(t, tracking.TurnID)
	assert.Equal(t, 0, tracking.ConsecutiveFailures)
}

func TestAutoCompactTracking_FailurePreservesOtherState(t *testing.T) {
	cm := agentic.NewContextManager()
	cm.RecordCompactSuccess()
	turnID := cm.Tracking().TurnID
	cm.RecordTurnAfterCompact()

	cm.RecordCompactFailure()
	tracking := cm.Tracking()
	// Failure should only increment failures, not touch other fields.
	assert.True(t, tracking.Compacted)
	assert.Equal(t, turnID, tracking.TurnID)
	assert.Equal(t, 1, tracking.TurnCounter)
	assert.Equal(t, 1, tracking.ConsecutiveFailures)
}

// --- Time-based tool result eviction ---

func TestEvictStaleToolResults_BelowThreshold(t *testing.T) {
	cfg := agentic.DefaultTimeBasedEvictionConfig()
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "hello", MessageType: "text"},
		{Role: "assistant", Content: "result", MessageType: "tool_result"},
	}
	result := agentic.EvictStaleToolResults(msgs, 30*time.Minute, cfg)
	assert.Equal(t, msgs, result) // no changes — under threshold
}

func TestEvictStaleToolResults_EvictsOldResults(t *testing.T) {
	cfg := agentic.TimeBasedEvictionConfig{
		GapThreshold: 60 * time.Minute,
		KeepRecent:   2,
	}
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "q1", MessageType: "text"},
		{Role: "user", Content: "result1 very long", MessageType: "tool_result"},
		{Role: "user", Content: "result2 very long", MessageType: "tool_result"},
		{Role: "user", Content: "result3 recent", MessageType: "tool_result"},
		{Role: "user", Content: "result4 recent", MessageType: "tool_result"},
	}
	result := agentic.EvictStaleToolResults(msgs, 2*time.Hour, cfg)
	require.Len(t, result, 5)

	// First two tool results should be cleared.
	assert.Contains(t, result[1].Content, "tool result cleared")
	assert.Contains(t, result[2].Content, "tool result cleared")
	// Last two should be preserved.
	assert.Equal(t, "result3 recent", result[3].Content)
	assert.Equal(t, "result4 recent", result[4].Content)
}

func TestEvictStaleToolResults_NotEnoughToEvict(t *testing.T) {
	cfg := agentic.TimeBasedEvictionConfig{
		GapThreshold: 60 * time.Minute,
		KeepRecent:   5,
	}
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "result1", MessageType: "tool_result"},
		{Role: "user", Content: "result2", MessageType: "tool_result"},
	}
	result := agentic.EvictStaleToolResults(msgs, 2*time.Hour, cfg)
	assert.Equal(t, msgs, result) // only 2 results, keep=5 → no eviction
}

// --- TruncateToTokens ---

func TestTruncateToTokens_FitsWithinBudget(t *testing.T) {
	content := "short text"
	result := agentic.TruncateToTokens(content, 100)
	assert.Equal(t, content, result) // no truncation needed
}

func TestTruncateToTokens_TruncatesLongContent(t *testing.T) {
	// Create content that exceeds 10 tokens (40 chars)
	content := strings.Repeat("a", 200) // ~50 tokens
	result := agentic.TruncateToTokens(content, 10)
	assert.Less(t, len(result), len(content))
	assert.Contains(t, result, "[... content truncated for compaction")
}

func TestTruncateToTokens_PreservesBeginning(t *testing.T) {
	content := "HEADER: important instructions\n" + strings.Repeat("x", 500)
	result := agentic.TruncateToTokens(content, 20)
	assert.True(t, strings.HasPrefix(result, "HEADER: important"))
}

func TestEvictStaleToolResults_DoesNotMutateOriginal(t *testing.T) {
	cfg := agentic.TimeBasedEvictionConfig{
		GapThreshold: 60 * time.Minute,
		KeepRecent:   0,
	}
	msgs := []chat.ChatMessage{
		{Role: "user", Content: "original content", MessageType: "tool_result"},
	}
	result := agentic.EvictStaleToolResults(msgs, 2*time.Hour, cfg)
	// Original should be untouched.
	assert.Equal(t, "original content", msgs[0].Content)
	// Result should be cleared.
	assert.Contains(t, result[0].Content, "tool result cleared")
}
