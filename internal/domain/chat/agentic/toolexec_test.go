package agentic_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestStreamingToolExecutor_EmitsProgressEvents(t *testing.T) {
	// Start a local HTTP server that responds to skill execution.
	// For unit test, we use a skill client pointing to a non-existent server.
	// The tool will fail, but we can verify the event sequence.
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:1") // will fail

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 1 * time.Second
	config.ConcurrentReadTools = 2

	executor := agentic.NewStreamingToolExecutor(skillClient, nil, config)

	ch := make(chan agentic.RunEvent, 100)

	toolCalls := []ai.ToolCall{
		{
			ID:   "tc_1",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "execute-sql",
				Arguments: `{"query":"SELECT 1"}`,
			},
		},
	}

	results := executor.ExecuteAll(context.Background(), ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	}, nil)
	close(ch)

	// Should have 1 result (with error since server is unreachable).
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error)

	// Collect events.
	var events []agentic.RunEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Should have: tool_call_start, tool_progress(queued), tool_progress(executing), tool_progress(completed)
	types := make([]agentic.RunEventType, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}

	assert.Contains(t, types, agentic.EventToolCallStart)
	assert.Contains(t, types, agentic.EventToolProgress)

	// Verify progress states.
	var states []agentic.ToolState
	for _, ev := range events {
		if ev.Type == agentic.EventToolProgress {
			var pd agentic.ToolProgressData
			require.NoError(t, json.Unmarshal(ev.Data, &pd))
			states = append(states, pd.State)
		}
	}
	assert.Contains(t, states, agentic.ToolStateQueued)
	assert.Contains(t, states, agentic.ToolStateExecuting)
	// Should have completed (even with error, we emit completed not aborted).
	assert.Contains(t, states, agentic.ToolStateCompleted)
}

func TestStreamingToolExecutor_ContextCancelled(t *testing.T) {
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:1")
	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 1 * time.Second

	executor := agentic.NewStreamingToolExecutor(skillClient, nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch := make(chan agentic.RunEvent, 100)
	toolCalls := []ai.ToolCall{
		{ID: "tc_1", Type: "function", Function: ai.ToolFunction{Name: "test", Arguments: "{}"}},
	}

	results := executor.ExecuteAll(ctx, ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	}, nil)
	close(ch)

	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error)
}

func TestStreamingToolExecutor_MultipleTools(t *testing.T) {
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:1")

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 1 * time.Second
	config.ConcurrentReadTools = 3

	executor := agentic.NewStreamingToolExecutor(skillClient, nil, config)

	ch := make(chan agentic.RunEvent, 100)

	toolCalls := []ai.ToolCall{
		{ID: "tc_1", Type: "function", Function: ai.ToolFunction{Name: "tool-a", Arguments: "{}"}},
		{ID: "tc_2", Type: "function", Function: ai.ToolFunction{Name: "tool-b", Arguments: "{}"}},
		{ID: "tc_3", Type: "function", Function: ai.ToolFunction{Name: "tool-c", Arguments: "{}"}},
	}

	results := executor.ExecuteAll(context.Background(), ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	}, nil)
	close(ch)

	// All tools should have results (all failed since server is unreachable).
	// First error triggers abort cascade, so some may be aborted.
	require.Len(t, results, 3)
	for _, r := range results {
		assert.NotNil(t, r.Error, "each result should have an error")
	}
}

// --- ValidateToolInput ---

func TestValidateToolInput_ValidJSON(t *testing.T) {
	result := agentic.ValidateToolInput("some-tool", json.RawMessage(`{"key":"value"}`))
	assert.Empty(t, result)
}

func TestValidateToolInput_EmptyInput(t *testing.T) {
	result := agentic.ValidateToolInput("some-tool", json.RawMessage(nil))
	assert.Empty(t, result)
}

func TestValidateToolInput_InvalidJSON(t *testing.T) {
	result := agentic.ValidateToolInput("some-tool", json.RawMessage(`{not json}`))
	assert.Contains(t, result, "Invalid JSON input")
	assert.Contains(t, result, "some-tool")
}

func TestValidateToolInput_AgentMissingPrompt(t *testing.T) {
	result := agentic.ValidateToolInput("agent", json.RawMessage(`{"tools":["search"]}`))
	assert.Contains(t, result, "prompt")
	assert.Contains(t, result, "missing")
}

func TestValidateToolInput_AgentWithPrompt(t *testing.T) {
	result := agentic.ValidateToolInput("agent", json.RawMessage(`{"prompt":"do something"}`))
	assert.Empty(t, result)
}

func TestValidateToolInput_DocumentSearchMissingQuery(t *testing.T) {
	result := agentic.ValidateToolInput("document_search", json.RawMessage(`{"limit":5}`))
	assert.Contains(t, result, "query")
}

func TestValidateToolInput_DocumentSearchWithQuery(t *testing.T) {
	result := agentic.ValidateToolInput("document_search", json.RawMessage(`{"query":"find docs"}`))
	assert.Empty(t, result)
}

func TestValidateToolInput_MemoryStoreMissingContent(t *testing.T) {
	result := agentic.ValidateToolInput("memory_store", json.RawMessage(`{"category":"fact"}`))
	assert.Contains(t, result, "content")
}

func TestValidateToolInput_MemoryStoreWithContent(t *testing.T) {
	result := agentic.ValidateToolInput("memory_store", json.RawMessage(`{"content":"remember this"}`))
	assert.Empty(t, result)
}

func TestValidateToolInput_UnknownToolPassesThrough(t *testing.T) {
	result := agentic.ValidateToolInput("unknown-tool", json.RawMessage(`{"anything":"goes"}`))
	assert.Empty(t, result)
}

// --- FormatToolError ---

func TestFormatToolError_ShortError(t *testing.T) {
	result := agentic.FormatToolError("short error", 1000)
	assert.Equal(t, "short error", result)
}

func TestFormatToolError_ZeroMaxChars(t *testing.T) {
	result := agentic.FormatToolError("any error", 0)
	assert.Equal(t, "any error", result)
}

func TestFormatToolError_ExactlyAtLimit(t *testing.T) {
	msg := "x" + string(make([]byte, 99)) // 100 chars
	for i := range []byte(msg) {
		_ = i
	}
	msg = strings.Repeat("a", 100)
	result := agentic.FormatToolError(msg, 100)
	assert.Equal(t, msg, result)
}

func TestFormatToolError_LongErrorTruncated(t *testing.T) {
	msg := strings.Repeat("a", 200)
	result := agentic.FormatToolError(msg, 100)
	assert.LessOrEqual(t, len(result), 120) // some overhead for notice
	assert.Contains(t, result, "truncated")
	// Should preserve head (starts with 'a')
	assert.True(t, strings.HasPrefix(result, "a"))
	// Should preserve tail (ends with 'a')
	assert.True(t, strings.HasSuffix(result, "a"))
}

func TestFormatToolError_PreservesHeadAndTail(t *testing.T) {
	head := "ERROR_TYPE: "
	tail := " at stack:trace:line:42"
	middle := strings.Repeat("x", 500)
	msg := head + middle + tail
	result := agentic.FormatToolError(msg, 100)
	// Head should be preserved.
	assert.True(t, strings.HasPrefix(result, "ERROR"), "head should be preserved")
	// Tail should be preserved.
	assert.True(t, strings.HasSuffix(result, "42"), "tail should be preserved")
}
