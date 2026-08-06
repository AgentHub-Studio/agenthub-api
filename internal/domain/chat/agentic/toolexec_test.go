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

func TestStreamingToolExecutor_EmitsAutoCollapseForSearchOrReadTool(t *testing.T) {
	executor := agentic.NewStreamingToolExecutor(agentic.NewSkillRuntimeClient("http://localhost:1"), nil, agentic.DefaultRunConfig())
	ch := make(chan agentic.RunEvent, 100)
	toolCalls := []ai.ToolCall{{ID: "tc_search", Type: "function", Function: ai.ToolFunction{Name: "search_docs", Arguments: "{}"}}}

	executor.ExecuteAll(context.Background(), ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test-tenant",
		SearchOrReadTools: map[string]bool{"search_docs": true},
	}, nil)
	close(ch)

	var result agentic.ToolResultData
	for event := range ch {
		switch event.Type {
		case agentic.EventToolCallStart:
			var payload agentic.ToolCallStartData
			require.NoError(t, json.Unmarshal(event.Data, &payload))
			assert.True(t, payload.AutoCollapse)
		case agentic.EventToolResult:
			require.NoError(t, json.Unmarshal(event.Data, &result))
		}
	}
	assert.True(t, result.AutoCollapse)
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

func TestValidateToolInput_JSONSchemaTypeMismatch(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {"approved": {"type": "boolean"}},
		"required": ["approved"]
	}`)
	result := agentic.ValidateToolInput("strict-tool", json.RawMessage(`{"approved":"yes"}`), schema)
	assert.Contains(t, result, "JSON Schema validation failed")
	assert.Contains(t, result, "approved")
	assert.Contains(t, result, "boolean")
}

func TestValidateToolInput_JSONSchemaValid(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {"approved": {"type": "boolean"}},
		"required": ["approved"]
	}`)
	result := agentic.ValidateToolInput("strict-tool", json.RawMessage(`{"approved":true}`), schema)
	assert.Empty(t, result)
}

func TestValidateToolInput_JSONSchemaRequiredAndTypeMatrix(t *testing.T) {
	for _, schemaType := range toolInputSchemaFuzzTypes {
		field := "field_" + schemaType
		schema := singlePropertyRequiredSchema(t, field, schemaType)

		missing := agentic.ValidateToolInput("strict-tool", json.RawMessage(`{}`), schema)
		assert.Contains(t, missing, "required property")
		assert.Contains(t, missing, field)

		validInput := singlePropertyInput(t, field, matchingSchemaValue(schemaType, "seed"))
		assert.Empty(t, agentic.ValidateToolInput("strict-tool", validInput, schema))

		invalidInput := singlePropertyInput(t, field, mismatchingSchemaValue(schemaType))
		invalid := agentic.ValidateToolInput("strict-tool", invalidInput, schema)
		assert.Contains(t, invalid, "JSON Schema validation failed")
		assert.Contains(t, invalid, field)
		assert.Contains(t, invalid, schemaType)
	}
}

func FuzzValidateToolInputRequiredAndTypeSchema(f *testing.F) {
	f.Add("strict-tool", "approved", "boolean", "true")
	f.Add("strict-tool", "limit", "integer", "42")
	f.Add("strict-tool", "payload", "object", `{"nested":true}`)
	f.Add("strict-tool", "items", "array", `[1,2,3]`)
	f.Add("strict-tool", "nullable", "null", "")
	f.Add("agent", "prompt", "string", "run task")

	f.Fuzz(func(t *testing.T, toolSeed, fieldSeed, typeSeed, valueSeed string) {
		toolName := "schema-fuzz-" + fuzzIdentifier(toolSeed, "tool")
		field := fuzzIdentifier(fieldSeed, "field")
		schemaType := fuzzSchemaType(typeSeed)
		valueSeed = boundedFuzzString(valueSeed, 1024)

		schema := singlePropertyRequiredSchema(t, field, schemaType)

		missing := agentic.ValidateToolInput(toolName, json.RawMessage(`{}`), schema)
		if !strings.Contains(missing, "required property") || !strings.Contains(missing, field) {
			t.Fatalf("missing required property was accepted or misreported: field=%q type=%q result=%q", field, schemaType, missing)
		}

		validInput := singlePropertyInput(t, field, matchingSchemaValue(schemaType, valueSeed))
		if errMsg := agentic.ValidateToolInput(toolName, validInput, schema); errMsg != "" {
			t.Fatalf("valid schema input rejected: field=%q type=%q input=%s result=%q", field, schemaType, string(validInput), errMsg)
		}

		invalidInput := singlePropertyInput(t, field, mismatchingSchemaValue(schemaType))
		invalid := agentic.ValidateToolInput(toolName, invalidInput, schema)
		if !strings.Contains(invalid, "JSON Schema validation failed") ||
			!strings.Contains(invalid, field) ||
			!strings.Contains(invalid, schemaType) {
			t.Fatalf("type mismatch was accepted or misreported: field=%q type=%q input=%s result=%q", field, schemaType, string(invalidInput), invalid)
		}
	})
}

func TestBuildInputSchemaIndex_OmitsEmptySchemas(t *testing.T) {
	idx := agentic.BuildInputSchemaIndex([]agentic.LLMTool{
		{Name: "empty", InputSchema: json.RawMessage(`{}`)},
		{Name: "strict", InputSchema: json.RawMessage(`{"type":"object"}`)},
	})
	assert.NotContains(t, idx, "empty")
	assert.Contains(t, idx, "strict")
}

var toolInputSchemaFuzzTypes = []string{"string", "boolean", "number", "integer", "object", "array", "null"}

func singlePropertyRequiredSchema(t testing.TB, field, schemaType string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type":     "object",
		"required": []string{field},
		"properties": map[string]any{
			field: map[string]any{"type": schemaType},
		},
	})
	require.NoError(t, err)
	return raw
}

func singlePropertyInput(t testing.TB, field string, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{field: value})
	require.NoError(t, err)
	return raw
}

func matchingSchemaValue(schemaType, seed string) any {
	switch schemaType {
	case "string":
		return seed
	case "boolean":
		return len(seed)%2 == 0
	case "number":
		return float64(len(seed)) + 0.25
	case "integer":
		return len(seed)
	case "object":
		return map[string]any{"value": seed}
	case "array":
		return []any{seed}
	case "null":
		return nil
	default:
		return seed
	}
}

func mismatchingSchemaValue(schemaType string) any {
	switch schemaType {
	case "string":
		return true
	case "boolean":
		return "true"
	case "number":
		return "not-a-number"
	case "integer":
		return 1.5
	case "object":
		return []any{"not-object"}
	case "array":
		return map[string]any{"not": "array"}
	case "null":
		return "not-null"
	default:
		return nil
	}
}

func fuzzSchemaType(seed string) string {
	for _, schemaType := range toolInputSchemaFuzzTypes {
		if seed == schemaType {
			return schemaType
		}
	}
	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i])
	}
	return toolInputSchemaFuzzTypes[sum%len(toolInputSchemaFuzzTypes)]
}

func fuzzIdentifier(seed, fallback string) string {
	seed = boundedFuzzString(seed, 128)
	var b strings.Builder
	for i := 0; i < len(seed); i++ {
		ch := seed[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch)
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case ch == '_' || ch == '-':
			b.WriteByte(ch)
		}
		if b.Len() >= 48 {
			break
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

func boundedFuzzString(value string, maxLen int) string {
	if len(value) > maxLen {
		return value[:maxLen]
	}
	return value
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

func TestExecuteDocumentSearch_RejectsConflictingLimitAliasesBeforeSearch(t *testing.T) {
	client := &mockKnowledgeSearchClient{}

	result, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"release notes","top_k":2,"limit":3}`),
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "top_k")
	assert.Contains(t, err.Error(), "limit")
	assert.Zero(t, client.calls, "ambiguous limits must fail before searching")
}

func TestExecuteDocumentSearch_AllowsEquivalentLimitAliases(t *testing.T) {
	client := &mockKnowledgeSearchClient{}

	result, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"release notes","top_k":2,"limit":2}`),
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, client.calls)
	assert.Equal(t, 2, client.lastTopK)
}

func FuzzExecuteDocumentSearch_LimitAliases(f *testing.F) {
	f.Add(0, 0)
	f.Add(2, 3)
	f.Add(2, 2)
	f.Add(-1, 5)

	f.Fuzz(func(t *testing.T, topK, limit int) {
		client := &mockKnowledgeSearchClient{}
		args := json.RawMessage(fmt.Sprintf(`{"query":"release notes","top_k":%d,"limit":%d}`, topK, limit))

		result, err := agentic.ExecuteDocumentSearch(context.Background(), client, nil, args)
		conflicting := topK > 0 && limit > 0 && topK != limit
		if conflicting {
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Zero(t, client.calls)
			return
		}

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, client.calls)
		expected := 5
		if limit > 0 {
			expected = limit
		} else if topK > 0 {
			expected = topK
		}
		assert.Equal(t, expected, client.lastTopK)
	})
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

func TestExecuteDocumentSearch_PropagatesMetadataFilter(t *testing.T) {
	client := &mockKnowledgeSearchClient{}

	_, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"release notes","metadataFilter":{"all":[{"field":"year","op":"in","value":[2025,2026]},{"field":"customer.tier","op":"gte","value":2}]}}`),
	)

	require.NoError(t, err)
	require.NotNil(t, client.lastMetadataFilter)
	sql, args := client.lastMetadataFilter.SQL(0)
	assert.Contains(t, sql, "d.metadata")
	assert.Contains(t, sql, "::numeric >=")
	assert.Equal(t, []any{[]string{"year"}, "2025", "2026", []string{"customer", "tier"}, "2"}, args)
}

func TestExecuteDocumentSearch_RejectsDuplicateMetadataFilterKeyBeforeSearch(t *testing.T) {
	client := &mockKnowledgeSearchClient{}

	_, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"release notes","metadataFilter":{"field":"source","field":"owner","op":"exists"}}`),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate JSON object key")
	assert.Zero(t, client.calls)
}

func TestExecuteDocumentSearch_RejectsOversizedMetadataFilterBeforeSearch(t *testing.T) {
	client := &mockKnowledgeSearchClient{}
	filter := `{"field":"source","op":"eq","value":"` + strings.Repeat("x", knowledge.MaxMetadataFilterBytes) + `"}`

	_, err := agentic.ExecuteDocumentSearch(
		context.Background(),
		client,
		nil,
		json.RawMessage(`{"query":"release notes","metadataFilter":`+filter+`}`),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum size")
	assert.Zero(t, client.calls)
}

// mockKnowledgeSearchClient is a test double for knowledge.DocumentSearchClient.
type mockKnowledgeSearchClient struct {
	results            any
	err                error
	lastTopK           int
	lastMetadataFilter *knowledge.MetadataFilter
	calls              int
}

func (m *mockKnowledgeSearchClient) Search(_ context.Context, _ string, opts knowledge.SearchOptions) ([]knowledge.SearchResult, error) {
	m.calls++
	m.lastTopK = opts.TopK
	m.lastMetadataFilter = opts.MetadataFilter
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
