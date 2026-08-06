package agentic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestStreamingToolExecutor_EmptyArgumentsWithSchemaDoNotReachRuntime(t *testing.T) {
	var requests atomic.Int32
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(runtime.Close)

	executor := agentic.NewStreamingToolExecutor(
		agentic.NewSkillRuntimeClient(runtime.URL),
		nil,
		agentic.DefaultRunConfig(),
	)
	events := make(chan agentic.RunEvent, 8)
	results := executor.ExecuteAll(
		context.Background(),
		events,
		[]ai.ToolCall{{
			ID:   "missing-args",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "execute-sql",
				Arguments: "",
			},
		}},
		agentic.RunInput{SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "schema-test"},
		nil,
		map[string]json.RawMessage{
			"execute-sql": json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
	)

	require.Len(t, results, 1)
	require.NotNil(t, results[0].Error)
	assert.Contains(t, *results[0].Error, "Invalid JSON input")
	assert.Zero(t, requests.Load(), "invalid empty arguments must be rejected before the skill runtime HTTP boundary")

	var toolResult agentic.ToolResultData
	for len(events) > 0 {
		event := <-events
		if event.Type == agentic.EventToolResult {
			require.NoError(t, json.Unmarshal(event.Data, &toolResult))
		}
	}
	require.NotNil(t, toolResult.Error)
	assert.Contains(t, *toolResult.Error, "Invalid JSON input")
}

func TestStreamingToolExecutor_DocumentSearchPropagatesMetadataFilter(t *testing.T) {
	client := &mockKnowledgeSearchClient{}
	executor := agentic.NewStreamingToolExecutor(nil, nil, agentic.DefaultRunConfig()).
		WithDocumentSearch(client, nil)
	events := make(chan agentic.RunEvent, 8)

	results := executor.ExecuteAll(
		context.Background(),
		events,
		[]ai.ToolCall{{
			ID:   "metadata-filter",
			Type: "function",
			Function: ai.ToolFunction{
				Name: "document_search",
				Arguments: `{
					"query":"release notes",
					"metadataFilter":{
						"all":[
							{"field":"year","op":"in","value":[2025,2026]},
							{"field":"customer.region","op":"ilike","value":"br%"}
						]
					}
				}`,
			},
		}},
		agentic.RunInput{SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "metadata-filter-test"},
		nil,
	)

	require.Len(t, results, 1)
	require.Nil(t, results[0].Error)
	require.Equal(t, "document_search", results[0].ToolName)
	require.Equal(t, 1, client.calls, "document_search must execute through the local client")
	require.NotNil(t, client.lastMetadataFilter)
	sql, args := client.lastMetadataFilter.SQL(0)
	assert.Contains(t, sql, "d.metadata")
	assert.Contains(t, sql, "ILIKE")
	assert.Equal(t, []any{[]string{"year"}, "2025", "2026", []string{"customer", "region"}, "br%"}, args)
}

func TestStreamingToolExecutor_DocumentSearchRejectsInvalidMetadataFilterBeforeSearch(t *testing.T) {
	client := &mockKnowledgeSearchClient{}
	executor := agentic.NewStreamingToolExecutor(nil, nil, agentic.DefaultRunConfig()).
		WithDocumentSearch(client, nil)

	results := executor.ExecuteAll(
		context.Background(),
		make(chan agentic.RunEvent, 8),
		[]ai.ToolCall{{
			ID:   "invalid-metadata-filter",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "document_search",
				Arguments: `{"query":"release notes","metadataFilter":{"field":"retired","op":"exists","value":true}}`,
			},
		}},
		agentic.RunInput{SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "metadata-filter-test"},
		nil,
	)

	require.Len(t, results, 1)
	require.NotNil(t, results[0].Error)
	assert.Contains(t, *results[0].Error, "invalid metadataFilter")
	assert.Contains(t, *results[0].Error, "exists does not accept value")
	assert.Zero(t, client.calls, "an invalid filter must be rejected before the local search boundary")
}

func TestStreamingToolExecutor_DocumentSearchRejectsConflictingLimitAliasesBeforeSearch(t *testing.T) {
	client := &mockKnowledgeSearchClient{}
	executor := agentic.NewStreamingToolExecutor(nil, nil, agentic.DefaultRunConfig()).
		WithDocumentSearch(client, nil)

	results := executor.ExecuteAll(
		context.Background(),
		make(chan agentic.RunEvent, 8),
		[]ai.ToolCall{{
			ID:   "conflicting-document-search-limits",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "document_search",
				Arguments: `{"query":"release notes","top_k":2,"limit":3}`,
			},
		}},
		agentic.RunInput{SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "document-search-limit-test"},
		nil,
	)

	require.Len(t, results, 1)
	require.NotNil(t, results[0].Error)
	assert.Contains(t, *results[0].Error, "top_k")
	assert.Contains(t, *results[0].Error, "limit")
	assert.Zero(t, client.calls, "ambiguous limits must be rejected before the local search boundary")
}

func TestStreamingToolExecutor_BlockInterruptCompletesInFlightTool(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runtimeCancelled := make(chan struct{})
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			_, _ = w.Write([]byte(`{"output":{"status":"completed"}}`))
		case <-r.Context().Done():
			close(runtimeCancelled)
		}
	}))
	t.Cleanup(runtime.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = time.Second
	executor := agentic.NewStreamingToolExecutor(agentic.NewSkillRuntimeClient(runtime.URL), nil, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultsCh := make(chan []agentic.ToolExecResult, 1)
	go func() {
		resultsCh <- executor.ExecuteAll(ctx, make(chan agentic.RunEvent, 8), []ai.ToolCall{{
			ID: "block-call", Type: "function", Function: ai.ToolFunction{Name: "destructive_write", Arguments: "{}"},
		}}, agentic.RunInput{
			SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "interrupt-test",
			InterruptBehaviors: map[string]string{"destructive_write": "block"},
		}, nil)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tool request did not start")
	}
	cancel()

	select {
	case <-runtimeCancelled:
		t.Fatal("block interrupt must not cancel an in-flight tool request")
	case <-time.After(75 * time.Millisecond):
	}
	select {
	case results := <-resultsCh:
		t.Fatalf("block interrupt returned before tool completion: %#v", results)
	case <-time.After(75 * time.Millisecond):
	}

	close(release)
	select {
	case results := <-resultsCh:
		require.Len(t, results, 1)
		assert.Nil(t, results[0].Error)
	case <-time.After(time.Second):
		t.Fatal("block interrupt did not finish after tool completion")
	}
}

func TestStreamingToolExecutor_CancelInterruptCancelsInFlightTool(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runtime := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(runtime.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = time.Second
	executor := agentic.NewStreamingToolExecutor(agentic.NewSkillRuntimeClient(runtime.URL), nil, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultsCh := make(chan []agentic.ToolExecResult, 1)
	go func() {
		resultsCh <- executor.ExecuteAll(ctx, make(chan agentic.RunEvent, 8), []ai.ToolCall{{
			ID: "cancel-call", Type: "function", Function: ai.ToolFunction{Name: "read_tool", Arguments: "{}"},
		}}, agentic.RunInput{
			SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "interrupt-test",
		}, nil)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tool request did not start")
	}
	cancel()

	select {
	case results := <-resultsCh:
		require.Len(t, results, 1)
		assert.NotNil(t, results[0].Error)
	case <-time.After(time.Second):
		t.Fatal("cancel interrupt did not return after request cancellation")
	}
	close(release)
}
