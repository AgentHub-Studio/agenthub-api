package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/anthropic"
)

func TestAnthropicProvider_GetProviderName(t *testing.T) {
	p := anthropic.New("key", "")
	assert.Equal(t, "anthropic", p.GetProviderName())
}

func TestAnthropicProvider_Chat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))

		resp := map[string]any{
			"id":          "msg_01",
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "end_turn",
			"content": []map[string]any{
				{"type": "text", "text": "Hello from Anthropic!"},
			},
			"usage": map[string]any{"input_tokens": 10, "output_tokens": 6},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := anthropic.New("test-key", srv.URL)
	msgs := []ai.Message{{Role: ai.RoleUser, Content: "Hi"}}
	res, err := p.Chat(context.Background(), msgs, ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.NoError(t, err)
	assert.Equal(t, "Hello from Anthropic!", res.Content)
	assert.Equal(t, "stop", res.FinishReason) // Anthropic "end_turn" is mapped to "stop"
	assert.Equal(t, 16, res.Usage.TotalTokens)
}

func TestAnthropicProvider_Chat_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid api key"}}`))
	}))
	defer srv.Close()

	p := anthropic.New("bad-key", srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ai.ErrInvalidAPIKey))
}

func TestAnthropicProvider_ChatStream_Tokens(t *testing.T) {
	sse := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" there\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	p := anthropic.New("key", srv.URL)
	ch, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.NoError(t, err)

	var got string
	for chunk := range ch {
		require.NoError(t, chunk.Error)
		got += chunk.Delta
	}
	assert.Equal(t, "Hi there", got)
}

func TestAnthropicProvider_ChatStream_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"error","error":{"message":"server error"}}`))
	}))
	defer srv.Close()

	p := anthropic.New("key", srv.URL)
	_, err := p.ChatStream(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.Error(t, err)
}

func TestAnthropicProvider_Chat_RateLimited_ReturnsTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limit exceeded"}}`))
	}))
	defer srv.Close()

	p := anthropic.New("key", srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.Error(t, err)

	var ae *ai.APIError
	require.True(t, errors.As(err, &ae), "expected *ai.APIError")
	assert.Equal(t, http.StatusTooManyRequests, ae.StatusCode)
	assert.True(t, errors.Is(err, ai.ErrRateLimited))
}

func TestAnthropicProvider_Chat_Unauthorized_ReturnsInvalidAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid api key"}}`))
	}))
	defer srv.Close()

	p := anthropic.New("bad-key", srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ai.ErrInvalidAPIKey))
}

func TestAnthropicProvider_Chat_ServerError_ReturnsProviderUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"error","error":{"message":"service unavailable"}}`))
	}))
	defer srv.Close()

	p := anthropic.New("key", srv.URL)
	_, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ai.ErrProviderUnavailable))
}

func TestAnthropicProvider_Chat_RetryAfterRateLimit_EventuallySucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limit"}}`))
			return
		}
		resp := map[string]any{
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "end_turn",
			"content":     []map[string]any{{"type": "text", "text": "ok"}},
			"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := anthropic.New("key", srv.URL)
	res, err := p.Chat(context.Background(), []ai.Message{{Role: ai.RoleUser, Content: "Hi"}},
		ai.ChatOptions{Model: "claude-3-5-sonnet-20241022", MaxTokens: 256})
	require.NoError(t, err)
	assert.Equal(t, "ok", res.Content)
	assert.Equal(t, int32(3), calls.Load())
}
