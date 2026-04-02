package agentic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestSkillRuntimeClient_Execute_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/skills/document-search/execute", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		input := body["input"].(map[string]any)
		assert.Equal(t, "how to reset password", input["query"])
		ctx := body["context"].(map[string]any)
		assert.Equal(t, "test-tenant", ctx["tenantId"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output":    map[string]any{"results": []string{"doc1", "doc2"}},
			"latencyMs": 150,
		})
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	result, err := client.Execute(
		context.Background(),
		"document-search",
		json.RawMessage(`{"query":"how to reset password"}`),
		"test-tenant", "agent-1", "session-1",
	)

	require.NoError(t, err)
	assert.Nil(t, result.Error)
	assert.GreaterOrEqual(t, result.LatencyMs, int64(150))
	assert.Contains(t, string(result.Output), "doc1")
}

func TestSkillRuntimeClient_Execute_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	result, err := client.Execute(
		context.Background(), "bad-skill",
		json.RawMessage(`{}`), "t", "", "",
	)

	require.NoError(t, err) // HTTP errors are returned in result, not as Go errors.
	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "internal server error")
}

func TestSkillRuntimeClient_Execute_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"skill not found"}`))
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	result, err := client.Execute(
		context.Background(), "unknown",
		json.RawMessage(`{}`), "t", "", "",
	)

	require.NoError(t, err)
	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "skill not found")
}

func TestSkillRuntimeClient_Execute_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate slow response — but context will be cancelled before it returns.
		select {}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := agentic.NewSkillRuntimeClient(server.URL)
	_, err := client.Execute(ctx, "slow-skill", nil, "t", "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestSkillRuntimeClient_Execute_NilInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		input := body["input"].(map[string]any)
		assert.Empty(t, input)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"output": "ok"})
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	result, err := client.Execute(context.Background(), "test", nil, "t", "", "")

	require.NoError(t, err)
	assert.Nil(t, result.Error)
}

func TestSkillRuntimeClient_Execute_NonMapInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"output": "ok"})
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	// Send a string instead of a map — should be wrapped.
	result, err := client.Execute(
		context.Background(), "test",
		json.RawMessage(`"just a string"`), "t", "", "",
	)

	require.NoError(t, err)
	assert.Nil(t, result.Error)
}

func TestSkillRuntimeClient_ExecuteParallel(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output":    map[string]any{"ok": true},
			"latencyMs": 10,
		})
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	calls := []agentic.ToolCall{
		{ID: "tc_1", Slug: "search", Input: json.RawMessage(`{"q":"a"}`), TenantID: "t"},
		{ID: "tc_2", Slug: "search", Input: json.RawMessage(`{"q":"b"}`), TenantID: "t"},
		{ID: "tc_3", Slug: "search", Input: json.RawMessage(`{"q":"c"}`), TenantID: "t"},
	}

	results := client.ExecuteParallel(context.Background(), calls, 2)

	assert.Len(t, results, 3)
	assert.Equal(t, int32(3), callCount.Load())
	for _, r := range results {
		assert.Nil(t, r.Error)
		assert.Contains(t, string(r.Output), "ok")
	}
}

func TestSkillRuntimeClient_ExecuteParallel_WithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		input := body["input"].(map[string]any)
		if input["fail"] == true {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad request"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"output": "ok"})
	}))
	defer server.Close()

	client := agentic.NewSkillRuntimeClient(server.URL)
	calls := []agentic.ToolCall{
		{ID: "tc_1", Slug: "ok-skill", Input: json.RawMessage(`{"fail":false}`), TenantID: "t"},
		{ID: "tc_2", Slug: "bad-skill", Input: json.RawMessage(`{"fail":true}`), TenantID: "t"},
	}

	results := client.ExecuteParallel(context.Background(), calls, 3)

	assert.Len(t, results, 2)
	assert.Nil(t, results[0].Error)
	require.NotNil(t, results[1].Error)
	assert.Contains(t, *results[1].Error, "bad request")
}

func TestNewSkillRuntimeClient_DefaultURL(t *testing.T) {
	client := agentic.NewSkillRuntimeClient("")
	// Can't directly access baseURL, but we can verify it doesn't panic.
	assert.NotNil(t, client)
}
