package openairesponses

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestDefaultsEmptyFunctionCallArgumentsToEmptyJSONObject(t *testing.T) {
	p := New("test-key", "")

	req := p.buildRequest([]ai.Message{
		{
			Role: ai.RoleAssistant,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: ai.ToolFunction{
						Name:      "search_documents",
						Arguments: "",
					},
				},
			},
		},
	}, ai.ChatOptions{Model: "gpt-5.4"}, false)

	items, ok := req.Input.([]inputItem)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "function_call", items[0].Type)
	assert.Empty(t, items[0].ID)
	assert.Equal(t, "call_123", items[0].CallID)
	assert.Equal(t, "{}", items[0].Args)
}

func TestBuildRequestPreservesNonEmptyFunctionCallArguments(t *testing.T) {
	p := New("test-key", "")

	req := p.buildRequest([]ai.Message{
		{
			Role: ai.RoleAssistant,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: ai.ToolFunction{
						Name:      "search_documents",
						Arguments: `{"query":"hello"}`,
					},
				},
			},
		},
	}, ai.ChatOptions{Model: "gpt-5.4"}, false)

	items, ok := req.Input.([]inputItem)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].ID)
	assert.Equal(t, "call_456", items[0].CallID)
	assert.Equal(t, `{"query":"hello"}`, items[0].Args)
}

func TestChatMapsIncompleteMaxOutputTokensToLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_901","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	response, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	require.NoError(t, err)
	assert.Equal(t, "length", response.FinishReason)
	assert.Equal(t, "resp_901", response.ResponseID)
	assert.Equal(t, 7, response.Usage.TotalTokens)
}

func TestChatMapsIncompleteContentFilterToError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_902","status":"incomplete","incomplete_details":{"reason":"content_filter"},"output":[]}`))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	assert.EqualError(t, err, "openai-responses: response incomplete: content_filter")
}

func TestChatMapsFailedResponseToError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_903","status":"failed","error":{"code":"server_error","message":"generation failed"},"output":[]}`))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	assert.EqualError(t, err, "openai-responses: server_error: generation failed")
}

func TestChatStreamForwardsReasoningDeltas(t *testing.T) {
	sse := "event: response.reasoning_summary_text.delta\n" +
		"data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"summary step \"}\n\n" +
		"event: response.reasoning_text.delta\n" +
		"data: {\"type\":\"response.reasoning_text.delta\",\"delta\":\"detail step \"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"final answer\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_123\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/responses", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	stream, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	require.NoError(t, err)

	var gotThinking, gotText string
	for chunk := range stream {
		require.NoError(t, chunk.Error)
		gotThinking += chunk.ThinkingDelta
		gotText += chunk.Delta
	}

	assert.Equal(t, "summary step detail step ", gotThinking)
	assert.Equal(t, "final answer", gotText)
}

func TestChatStreamForwardsRefusalDeltas(t *testing.T) {
	sse := "event: response.refusal.delta\n" +
		"data: {\"type\":\"response.refusal.delta\",\"delta\":\"I cannot help with that.\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_345\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	stream, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	require.NoError(t, err)

	var got string
	for chunk := range stream {
		require.NoError(t, chunk.Error)
		got += chunk.Delta
	}

	assert.Equal(t, "I cannot help with that.", got)
}

func TestChatStreamMapsIncompleteMaxTokensToLength(t *testing.T) {
	sse := "event: response.incomplete\n" +
		"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_456\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"output\":[],\"usage\":{\"input_tokens\":3,\"output_tokens\":4,\"total_tokens\":7}}}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	stream, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	require.NoError(t, err)

	var chunks []ai.StreamChunk
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}
	require.Len(t, chunks, 1)
	assert.NoError(t, chunks[0].Error)
	assert.Equal(t, "length", chunks[0].FinishReason)
	assert.Equal(t, "resp_456", chunks[0].ResponseID)
	assert.Equal(t, 7, chunks[0].Usage.TotalTokens)
}

func TestChatStreamMapsIncompleteLegacyMaxTokensToLength(t *testing.T) {
	sse := "event: response.incomplete\n" +
		"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_457\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_tokens\"},\"output\":[]}}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	stream, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	require.NoError(t, err)

	var chunks []ai.StreamChunk
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}
	require.Len(t, chunks, 1)
	assert.NoError(t, chunks[0].Error)
	assert.Equal(t, "length", chunks[0].FinishReason)
	assert.Equal(t, "resp_457", chunks[0].ResponseID)
}

func TestChatStreamMapsIncompleteContentFilterToError(t *testing.T) {
	sse := "event: response.incomplete\n" +
		"data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_789\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"content_filter\"},\"output\":[]}}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := New("test-key", srv.URL)
	stream, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}, ai.ChatOptions{Model: "gpt-5"})
	require.NoError(t, err)

	var chunks []ai.StreamChunk
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}
	require.Len(t, chunks, 1)
	assert.EqualError(t, chunks[0].Error, "openai-responses: response incomplete: content_filter")
}
