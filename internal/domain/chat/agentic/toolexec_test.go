package agentic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
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

// --- TR-01-TASK-07: ExecuteDocumentSearch (P-C179-1) ---

// TestExecuteDocumentSearch_ReturnsJSONResults verifies that ExecuteDocumentSearch
// serialises the search results as JSON.
func TestExecuteDocumentSearch_ReturnsJSONResults(t *testing.T) {
	kb1 := uuid.New()
	client := &mockKnowledgeSearchClient{
		results: []map[string]any{
			{"content": "chunk one", "score": 0.9},
			{"content": "chunk two", "score": 0.8},
		},
	}

	result, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		[]uuid.UUID{kb1},
		json.RawMessage(`{"query":"test query","top_k":2}`),
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "document_search", result.ToolName)
	assert.Nil(t, result.Error)
	// Output should be a valid JSON array.
	var out []map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &out))
	assert.Len(t, out, 2)
}

// TestExecuteDocumentSearch_DefaultsTopKToFive verifies that top_k defaults to 5
// when not provided in the arguments.
func TestExecuteDocumentSearch_DefaultsTopKToFive(t *testing.T) {
	client := &mockKnowledgeSearchClient{}

	_, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"hello"}`),
	)

	require.NoError(t, err)
	assert.Equal(t, 5, client.lastTopK)
}

// TestExecuteDocumentSearch_PropagatesError verifies that search errors are returned.
func TestExecuteDocumentSearch_PropagatesError(t *testing.T) {
	client := &mockKnowledgeSearchClient{err: fmt.Errorf("vector db unavailable")}

	_, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"hello"}`),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vector db unavailable")
}

// mockKnowledgeSearchClient is a test double for knowledge.DocumentSearchClient.
type mockKnowledgeSearchClient struct {
	results  any
	err      error
	lastTopK int
}

func (m *mockKnowledgeSearchClient) Search(_ context.Context, _ string, _ []uuid.UUID, topK int) ([]knowledge.SearchResult, error) {
	m.lastTopK = topK
	if m.err != nil {
		return nil, m.err
	}
	if m.results == nil {
		return nil, nil
	}
	// Marshal+unmarshal to convert []map[string]any → []knowledge.SearchResult.
	b, _ := json.Marshal(m.results)
	var res []knowledge.SearchResult
	_ = json.Unmarshal(b, &res)
	return res, nil
}

// TestFormatToolResult_NoStatusCodeForLLM verifies status_code is stripped from HTTP output.
// P-C176-1: the LLM must not see HTTP status codes in tool results.
func TestFormatToolResult_NoStatusCodeForLLM(t *testing.T) {
	r := agentic.ToolExecResult{
		Output: json.RawMessage(`{"response":{"data":"value"},"status_code":200}`),
	}
	result := agentic.FormatToolResult(r)
	assert.NotContains(t, result, "status_code")
	assert.NotContains(t, result, "200")
	assert.Contains(t, result, "value")
}

// TestFormatToolResult_StripsStatusCodeVariants verifies both snake_case and camelCase variants.
func TestFormatToolResult_StripsStatusCodeVariants(t *testing.T) {
	r := agentic.ToolExecResult{
		Output: json.RawMessage(`{"response":"ok","status_code":201,"statusCode":201}`),
	}
	result := agentic.FormatToolResult(r)
	assert.NotContains(t, result, "status_code")
	assert.NotContains(t, result, "statusCode")
	assert.Contains(t, result, "response")
}

// TestFormatToolResult_NonHTTP_OutputUnchanged verifies non-HTTP outputs (no status_code) pass through.
func TestFormatToolResult_NonHTTP_OutputUnchanged(t *testing.T) {
	r := agentic.ToolExecResult{
		Output: json.RawMessage(`{"users":[{"id":1,"name":"Alice"}]}`),
	}
	result := agentic.FormatToolResult(r)
	assert.JSONEq(t, `{"users":[{"id":1,"name":"Alice"}]}`, result)
}
