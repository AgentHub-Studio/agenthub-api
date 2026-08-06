package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
)

// stubChatModel is a minimal ChatModel for retry tests (internal package).
type stubChatModel struct {
	calls   int
	results []stubResult
}

type stubResult struct {
	err error
}

func (s *stubChatModel) Chat(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
	return nil, nil
}

func (s *stubChatModel) ChatStream(_ context.Context, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	idx := s.calls
	s.calls++
	if idx < len(s.results) && s.results[idx].err != nil {
		return nil, s.results[idx].err
	}
	ch := make(chan ai.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (s *stubChatModel) GetProviderName() string { return "stub" }

// modelTrackingChatModel tracks which models were requested.
type modelTrackingChatModel struct {
	calls      int
	modelsUsed []string
	results    []stubResult
}

func (s *modelTrackingChatModel) Chat(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
	return nil, nil
}

func (s *modelTrackingChatModel) ChatStream(_ context.Context, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	idx := s.calls
	s.calls++
	s.modelsUsed = append(s.modelsUsed, opts.Model)
	if idx < len(s.results) && s.results[idx].err != nil {
		return nil, s.results[idx].err
	}
	ch := make(chan ai.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (s *modelTrackingChatModel) GetProviderName() string { return "stub" }

// --- retryStream tests ---

func TestRetryStream_SuccessFirstAttempt(t *testing.T) {
	model := &stubChatModel{}
	stream, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStream_TransientThenSuccess(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("status 429: rate limit exceeded")},
			{err: nil}, // success
		},
	}
	stream, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, 2, model.calls)
}

func TestRetryStream_OpenAICompatibleRateLimitThenSSESuccess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		var request struct {
			Stream bool `json:"stream"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.True(t, request.Stream)

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit"}}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"recovered\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	stream, err := retryStream(
		context.Background(),
		openai.New("test-key", server.URL),
		[]ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		ai.ChatOptions{Model: "test-model"},
		2,
		SourceMainLoop,
	)
	require.NoError(t, err)

	var output string
	for chunk := range stream {
		require.NoError(t, chunk.Error)
		output += chunk.Delta
	}

	assert.Equal(t, "recovered", output)
	assert.Equal(t, int32(2), calls.Load(), "the runner retry must issue a second streaming provider request")
}

func TestRetryStream_NonTransientError(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("invalid API key")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key")
	assert.Equal(t, 1, model.calls) // no retry
}

func TestRetryStream_AllAttemptsExhausted(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("503 service unavailable")},
			{err: fmt.Errorf("503 service unavailable")},
			{err: fmt.Errorf("503 service unavailable")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 attempts failed")
	assert.Equal(t, 3, model.calls)
}

func TestRetryStream_NoRetryWhenMaxAttemptsOne(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 1, SourceMainLoop)
	require.Error(t, err)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStream_AbortErrorNoRetry(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("context canceled")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Equal(t, 1, model.calls) // no retry
}

func TestRetryStream_MediaSizeErrorNoRetry(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("image exceeds maximum size limit")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image exceeds")
	assert.Equal(t, 1, model.calls) // no retry
}

func TestRetryStream_PromptTooLongNoRetry(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("prompt_too_long: 200000 tokens > 180000")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt_too_long")
	assert.Equal(t, 1, model.calls) // no retry
}

func TestRetryStream_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},
		},
	}
	_, err := retryStream(ctx, model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.Error(t, err)
	// Should fail with context error, not retry
	assert.Equal(t, 1, model.calls)
}

func TestIsPromptTooLong(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"prompt_too_long", fmt.Errorf("error: prompt_too_long"), true},
		{"prompt is too long", fmt.Errorf("Prompt is too long for this model"), true},
		{"context_length_exceeded", fmt.Errorf("context_length_exceeded"), true},
		{"maximum context length", fmt.Errorf("maximum context length exceeded"), true},
		{"unrelated error", fmt.Errorf("invalid API key"), false},
		{"transient error", fmt.Errorf("429 rate limit"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isPromptTooLong(tt.err))
		})
	}
}

func TestParsePromptTooLongTokenCounts(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		actual int
		limit  int
	}{
		{"standard format", "Prompt is too long: 210000 tokens > 200000", 210000, 200000},
		{"lowercase", "prompt is too long: 150000 tokens > 128000", 150000, 128000},
		{"singular token", "Prompt is too long: 50000 token > 32000", 50000, 32000},
		{"extra text around", "error: Prompt is too long — 300000 tokens > 200000 tokens limit", 300000, 200000},
		{"no match", "invalid API key", 0, 0},
		{"rate limit", "429 rate limited", 0, 0},
		{"empty", "", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts := parsePromptTooLongTokenCounts(tt.errMsg)
			assert.Equal(t, tt.actual, counts.ActualTokens)
			assert.Equal(t, tt.limit, counts.LimitTokens)
		})
	}
}

func TestGetPromptTooLongTokenGap(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		gap    int
	}{
		{"10k over", "Prompt is too long: 210000 tokens > 200000", 10000},
		{"22k over", "prompt is too long: 150000 tokens > 128000", 22000},
		{"no match", "invalid API key", 0},
		{"equal (no gap)", "Prompt is too long: 200000 tokens > 200000", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.gap, getPromptTooLongTokenGap(tt.errMsg))
		})
	}
}

func TestRetryStreamWithFallback_PrimarySuccess(t *testing.T) {
	model := &stubChatModel{}
	cfg := RunConfig{ModelFallbacks: []string{"fallback1"}, RetryMaxAttempts: 3}
	result, err := retryStreamWithFallbackSource(context.Background(), model, nil, ai.ChatOptions{Model: "primary"}, cfg, SourceMainLoop, nil)
	require.NoError(t, err)
	require.NotNil(t, result.Stream)
	assert.False(t, result.WasFallback) // primary model used
	assert.Equal(t, 1, model.calls)
}

func TestRetryStreamWithFallback_FallbackOnRateLimit(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit exceeded")}, // primary fails with rate limit
			{err: nil}, // fallback1 succeeds
		},
	}
	cfg := RunConfig{ModelFallbacks: []string{"fallback1"}, RetryMaxAttempts: 1}
	result, err := retryStreamWithFallbackSource(context.Background(), model, nil, ai.ChatOptions{Model: "primary"}, cfg, SourceMainLoop, nil)
	require.NoError(t, err)
	require.NotNil(t, result.Stream)
	assert.True(t, result.WasFallback)
	assert.Equal(t, "fallback1", result.ModelUsed) // reports which fallback was used
	assert.Equal(t, 2, model.calls)
}

func TestRetryStreamWithFallback_NoFallbackOnPromptTooLong(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("prompt_too_long")},
		},
	}
	cfg := RunConfig{ModelFallbacks: []string{"fallback1"}, RetryMaxAttempts: 1}
	_, err := retryStreamWithFallbackSource(context.Background(), model, nil, ai.ChatOptions{Model: "primary"}, cfg, SourceMainLoop, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt_too_long")
	assert.Equal(t, 1, model.calls) // no fallback attempted
}

func TestRetryStreamWithFallback_NoFallbackModels(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("invalid API key")},
		},
	}
	cfg := RunConfig{RetryMaxAttempts: 1}
	_, err := retryStreamWithFallbackSource(context.Background(), model, nil, ai.ChatOptions{}, cfg, SourceMainLoop, nil)
	require.Error(t, err)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStreamWithFallbackSource_RecoversWithNonStreamingAndKeepsStream(t *testing.T) {
	model := &streamFallbackInternalMockModel{
		chatStreamErr: fmt.Errorf("provider returned HTTP 404 for streaming endpoint"),
		chatResponse: &ai.ChatResponse{
			Content:      "recovered without changing the public stream",
			FinishReason: "stop",
			Model:        "primary",
			Usage:        ai.Usage{TotalTokens: 11},
		},
	}

	result, err := retryStreamWithFallbackSource(
		context.Background(),
		model,
		[]ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		ai.ChatOptions{Model: "primary"},
		RunConfig{RetryMaxAttempts: 1},
		SourceMainLoop,
		nil,
	)

	require.NoError(t, err)
	require.True(t, result.UsedNonStreaming)
	require.False(t, result.WasFallback)
	require.Equal(t, "primary", result.ModelUsed)
	require.Equal(t, 1, model.streamCalls)
	require.Equal(t, 1, model.chatCalls)

	chunk := <-result.Stream
	require.Equal(t, "recovered without changing the public stream", chunk.Delta)
	final := <-result.Stream
	require.Equal(t, "stop", final.FinishReason)
	require.NotNil(t, final.Usage)
	require.Equal(t, 11, final.Usage.TotalTokens)
}

func TestRetryStreamWithFallback_AllFallbacksFail(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("529 overloaded")},
			{err: fmt.Errorf("529 overloaded")},
			{err: fmt.Errorf("529 overloaded")},
		},
	}
	cfg := RunConfig{ModelFallbacks: []string{"fb1", "fb2"}, RetryMaxAttempts: 1}
	_, err := retryStreamWithFallbackSource(context.Background(), model, nil, ai.ChatOptions{}, cfg, SourceMainLoop, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all models failed")
	assert.Equal(t, 3, model.calls)
}

func TestRetryStream_BackgroundSourceNoRetryOnOverload(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("529 overloaded")},
		},
	}
	// Background source (compact) should NOT retry on 529.
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceCompact)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "529")
	assert.Equal(t, 1, model.calls) // no retry for background source
}

func TestRetryStream_Consecutive529Limit(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("529 overloaded")},
			{err: fmt.Errorf("529 overloaded")},
			{err: fmt.Errorf("529 overloaded")},
			{err: fmt.Errorf("529 overloaded")}, // won't reach this
		},
	}
	// With maxAttempts=5 and max529Retries=3, should give up after 3 consecutive 529s.
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 5, SourceMainLoop)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "consecutive 529")
	assert.Equal(t, 3, model.calls) // stopped at 3, not 5
}

func TestRetryStream_Consecutive529ResetsOnNon529(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("529 overloaded")},
			{err: fmt.Errorf("503 service unavailable")}, // resets consecutive counter
			{err: fmt.Errorf("529 overloaded")},
			{err: nil}, // success
		},
	}
	stream, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 5, SourceMainLoop)
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, 4, model.calls)
}

func TestRetryStream_ForegroundSourceRetriesOnOverload(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("529 overloaded")},
			{err: nil}, // second attempt succeeds
		},
	}
	// Foreground source (main_loop) SHOULD retry on 529.
	stream, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3, SourceMainLoop)
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, 2, model.calls)
}

func TestQuerySource_IsForeground(t *testing.T) {
	assert.True(t, SourceMainLoop.IsForegroundSource())
	assert.True(t, SourceSubtask.IsForegroundSource())
	assert.True(t, SourceBudgetNudge.IsForegroundSource())
	assert.False(t, SourceCompact.IsForegroundSource())
	assert.False(t, SourceMemoryEval.IsForegroundSource())
	assert.False(t, QuerySource("unknown").IsForegroundSource())
}

func TestIsOverloadError(t *testing.T) {
	assert.True(t, isOverloadError(fmt.Errorf("529 overloaded")))
	assert.True(t, isOverloadError(fmt.Errorf("service overloaded, try again")))
	assert.False(t, isOverloadError(fmt.Errorf("429 rate limit")))
	assert.False(t, isOverloadError(fmt.Errorf("invalid API key")))
	assert.False(t, isOverloadError(nil))
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected time.Duration
	}{
		{"no match", "invalid API key", 0},
		{"retry-after integer", "retry-after: 5", 5 * time.Second},
		{"retry_after underscore", "retry_after: 3", 3 * time.Second},
		{"Retry-After capitalized", "Retry-After: 10", 10 * time.Second},
		{"fractional seconds", "retry-after: 2.5", time.Duration(2.5 * float64(time.Second))},
		{"capped at 60s", "retry-after: 120", 60 * time.Second},
		{"zero value", "retry-after: 0", 0},
		{"negative value", "retry-after: -5", 0},
		{"embedded in message", "status 429: rate limited, retry-after: 8 seconds", 8 * time.Second},
		{"retry_after with space", "retry_after:  15", 15 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseRetryAfter(tt.errMsg))
		})
	}
}

func TestResolveToolResultLimit(t *testing.T) {
	limits := map[string]int{
		"document_search": 100000,
		"execute_sql":     10000,
	}
	globalMax := 50000

	// Tool with explicit override.
	assert.Equal(t, 100000, resolveToolResultLimit("document_search", limits, globalMax))
	assert.Equal(t, 10000, resolveToolResultLimit("execute_sql", limits, globalMax))

	// Tool without override — falls back to global.
	assert.Equal(t, 50000, resolveToolResultLimit("unknown_tool", limits, globalMax))

	// Nil map — falls back to global.
	assert.Equal(t, 50000, resolveToolResultLimit("any_tool", nil, globalMax))
}

func TestFilterUnresolvedToolUses_NoOrphans(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "I'll search", ToolCalls: []ai.ToolCall{{ID: "tc1"}}},
		{Role: ai.RoleTool, ToolCallID: "tc1", Content: "result"},
		{Role: ai.RoleAssistant, Content: "done"},
	}
	filtered := filterUnresolvedToolUses(messages)
	assert.Equal(t, len(messages), len(filtered))
}

func TestFilterUnresolvedToolUses_OrphanRemoved(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "I'll search", ToolCalls: []ai.ToolCall{{ID: "tc1"}}},
		// No tool_result for tc1 — orphaned.
		{Role: ai.RoleUser, Content: "try again"},
		{Role: ai.RoleAssistant, Content: "ok"},
	}
	filtered := filterUnresolvedToolUses(messages)
	// The orphaned assistant message should be removed.
	assert.Equal(t, 3, len(filtered))
	for _, m := range filtered {
		if m.Role == ai.RoleAssistant {
			assert.Empty(t, m.ToolCalls)
		}
	}
}

func TestFilterUnresolvedToolUses_MixedResolvedUnresolved(t *testing.T) {
	// An assistant message with 2 tool_calls, one resolved one not.
	// Since not ALL are unresolved, the message should be kept.
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "searching", ToolCalls: []ai.ToolCall{
			{ID: "tc1"},
			{ID: "tc2"},
		}},
		{Role: ai.RoleTool, ToolCallID: "tc1", Content: "result1"},
		// tc2 has no result — but the assistant message has tc1 resolved.
		{Role: ai.RoleAssistant, Content: "done"},
	}
	filtered := filterUnresolvedToolUses(messages)
	// Message kept because tc1 is resolved (not ALL unresolved).
	assert.Equal(t, 4, len(filtered))
}

func TestFilterUnresolvedToolUses_EmptyMessages(t *testing.T) {
	var messages []ai.Message
	filtered := filterUnresolvedToolUses(messages)
	assert.Nil(t, filtered)
}

func TestFilterUnresolvedToolUses_NoToolCalls(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "hi there"},
	}
	filtered := filterUnresolvedToolUses(messages)
	assert.Equal(t, 2, len(filtered))
}

func TestModelSupportsThinking(t *testing.T) {
	assert.True(t, modelSupportsThinking("claude-opus-4-6"))
	assert.True(t, modelSupportsThinking("claude-sonnet-4-6"))
	assert.True(t, modelSupportsThinking("claude-haiku-4-5-20251001"))
	assert.True(t, modelSupportsThinking("claude-opus-4-1"))
	assert.False(t, modelSupportsThinking("gpt-4o"))
	assert.False(t, modelSupportsThinking("claude-3-opus"))
}

func TestModelSupportsAdaptiveThinking(t *testing.T) {
	assert.True(t, modelSupportsAdaptiveThinking("claude-opus-4-6"))
	assert.True(t, modelSupportsAdaptiveThinking("claude-sonnet-4-6"))
	assert.False(t, modelSupportsAdaptiveThinking("claude-opus-4-1"))
	assert.False(t, modelSupportsAdaptiveThinking("claude-sonnet-4-5"))
	assert.False(t, modelSupportsAdaptiveThinking("gpt-4o"))
}

func TestResolveThinkingConfig(t *testing.T) {
	// Nil config → nil.
	cfg := RunConfig{Model: "claude-opus-4-6", MaxTokensPerCall: 4096}
	assert.Nil(t, resolveThinkingConfig(cfg))

	// Disabled → nil.
	cfg.Thinking = &ai.ThinkingConfig{Type: ai.ThinkingDisabled}
	assert.Nil(t, resolveThinkingConfig(cfg))

	// Unsupported model → nil.
	cfg.Model = "gpt-4o"
	cfg.Thinking = &ai.ThinkingConfig{Type: ai.ThinkingAdaptive}
	assert.Nil(t, resolveThinkingConfig(cfg))

	// Adaptive on Opus 4.6 → adaptive.
	cfg.Model = "claude-opus-4-6"
	cfg.Thinking = &ai.ThinkingConfig{Type: ai.ThinkingAdaptive}
	result := resolveThinkingConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, ai.ThinkingAdaptive, result.Type)

	// Adaptive on Opus 4.1 (no adaptive support) → fallback to enabled.
	cfg.Model = "claude-opus-4-1"
	result = resolveThinkingConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, ai.ThinkingEnabled, result.Type)
	assert.Greater(t, result.BudgetTokens, 0)

	// Enabled with explicit budget.
	cfg.Model = "claude-sonnet-4-6"
	cfg.Thinking = &ai.ThinkingConfig{Type: ai.ThinkingEnabled, BudgetTokens: 2000}
	result = resolveThinkingConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, ai.ThinkingEnabled, result.Type)
	assert.Equal(t, 2000, result.BudgetTokens)

	// Budget exceeds max_tokens → capped.
	cfg.Thinking = &ai.ThinkingConfig{Type: ai.ThinkingEnabled, BudgetTokens: 10000}
	result = resolveThinkingConfig(cfg)
	require.NotNil(t, result)
	assert.Equal(t, cfg.MaxTokensPerCall-1, result.BudgetTokens)
}

func TestIsMediaSizeError(t *testing.T) {
	assert.True(t, isMediaSizeError(fmt.Errorf("image exceeds maximum size")))
	assert.True(t, isMediaSizeError(fmt.Errorf("image dimensions exceed limit")))
	assert.True(t, isMediaSizeError(fmt.Errorf("maximum of 100 PDF pages allowed")))
	assert.False(t, isMediaSizeError(fmt.Errorf("invalid API key")))
	assert.False(t, isMediaSizeError(nil))
}

func TestIsAbortError(t *testing.T) {
	assert.True(t, isAbortError(fmt.Errorf("context canceled")))
	assert.True(t, isAbortError(fmt.Errorf("context deadline exceeded")))
	assert.True(t, isAbortError(fmt.Errorf("operation was aborted")))
	assert.False(t, isAbortError(fmt.Errorf("429 rate limit")))
	assert.False(t, isAbortError(nil))
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		connection bool
	}{
		{"nil", nil, false},
		{"econnreset", fmt.Errorf("ECONNRESET"), true},
		{"connection reset", fmt.Errorf("connection reset by peer"), true},
		{"econnrefused", fmt.Errorf("ECONNREFUSED"), true},
		{"connection refused", fmt.Errorf("connection refused"), true},
		{"epipe", fmt.Errorf("EPIPE: broken pipe"), true},
		{"broken pipe", fmt.Errorf("broken pipe"), true},
		{"etimedout", fmt.Errorf("ETIMEDOUT"), true},
		{"no such host", fmt.Errorf("no such host"), true},
		{"rate limit", fmt.Errorf("429 rate limit"), false},
		{"auth error", fmt.Errorf("invalid API key"), false},
		{"server error", fmt.Errorf("500 internal"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.connection, isConnectionError(tt.err))
		})
	}
}

func TestIsStaleConnectionError(t *testing.T) {
	assert.True(t, isStaleConnectionError(fmt.Errorf("ECONNRESET")))
	assert.True(t, isStaleConnectionError(fmt.Errorf("connection reset by peer")))
	assert.True(t, isStaleConnectionError(fmt.Errorf("EPIPE")))
	assert.True(t, isStaleConnectionError(fmt.Errorf("broken pipe")))
	assert.False(t, isStaleConnectionError(fmt.Errorf("ECONNREFUSED")))
	assert.False(t, isStaleConnectionError(fmt.Errorf("ETIMEDOUT")))
	assert.False(t, isStaleConnectionError(fmt.Errorf("429 rate limit")))
	assert.False(t, isStaleConnectionError(nil))
}

func TestClassifyAPIError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected APIErrorClass
	}{
		{"nil", nil, ErrorClassNone},
		{"rate limit", fmt.Errorf("status 429: rate limited"), ErrorClassRateLimit},
		{"overload", fmt.Errorf("529 overloaded"), ErrorClassServerOverload},
		{"timeout", fmt.Errorf("connection timeout"), ErrorClassTimeout},
		{"econnreset", fmt.Errorf("ECONNRESET"), ErrorClassStaleConnection},
		{"epipe", fmt.Errorf("broken pipe"), ErrorClassStaleConnection},
		{"connection refused", fmt.Errorf("ECONNREFUSED"), ErrorClassConnection},
		{"auth 401", fmt.Errorf("status 401: unauthorized"), ErrorClassAuth},
		{"invalid key", fmt.Errorf("invalid API key"), ErrorClassAuth},
		{"prompt too long", fmt.Errorf("prompt_too_long"), ErrorClassPromptTooLong},
		{"server 500", fmt.Errorf("500 internal server error"), ErrorClassServerError},
		{"media size", fmt.Errorf("image exceeds maximum size"), ErrorClassMediaSize},
		{"aborted", fmt.Errorf("context canceled"), ErrorClassAborted},
		{"unknown", fmt.Errorf("something weird happened"), ErrorClassUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyAPIError(tt.err))
		})
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{"nil", nil, false},
		{"rate limit 429", fmt.Errorf("status 429: rate limited"), true},
		{"overloaded 529", fmt.Errorf("529 overloaded"), true},
		{"503", fmt.Errorf("503 service unavailable"), true},
		{"502", fmt.Errorf("502 bad gateway"), true},
		{"timeout", fmt.Errorf("connection timeout"), true},
		{"connection reset", fmt.Errorf("connection reset by peer"), true},
		{"EOF", fmt.Errorf("unexpected EOF"), true},
		{"rate limit text", fmt.Errorf("Rate Limit exceeded"), true},
		{"invalid key", fmt.Errorf("invalid API key"), false},
		{"not found", fmt.Errorf("model not found"), false},
		{"auth error", fmt.Errorf("authentication failed"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.transient, isTransientError(tt.err))
		})
	}
}

// --- Context overflow tests ---

func TestParseContextOverflow_MatchesAnthropicFormat(t *testing.T) {
	err := "input length and max_tokens exceed context limit: 180000 + 16384 > 200000"
	counts := parseContextOverflow(err)
	assert.Equal(t, 180000, counts.InputTokens)
	assert.Equal(t, 16384, counts.MaxTokens)
	assert.Equal(t, 200000, counts.ContextLimit)
}

func TestParseContextOverflow_NoMatch(t *testing.T) {
	counts := parseContextOverflow("prompt is too long: 300000 tokens > 200000")
	assert.Equal(t, ContextOverflowCounts{}, counts)
}

func TestComputeAdjustedMaxTokens_Normal(t *testing.T) {
	err := "input length and max_tokens exceed context limit: 180000 + 16384 > 200000"
	adjusted := computeAdjustedMaxTokens(err)
	// 200000 - 180000 - 1000 = 19000
	assert.Equal(t, 19000, adjusted)
}

func TestComputeAdjustedMaxTokens_ClampsToMinimum(t *testing.T) {
	err := "input length and max_tokens exceed context limit: 198000 + 4096 > 200000"
	adjusted := computeAdjustedMaxTokens(err)
	// 200000 - 198000 - 1000 = 1000 < 3000 → clamped to 3000
	assert.Equal(t, 3000, adjusted)
}

func TestComputeAdjustedMaxTokens_NoMatch(t *testing.T) {
	adjusted := computeAdjustedMaxTokens("some other error")
	assert.Equal(t, 0, adjusted)
}

func TestIsContextOverflow(t *testing.T) {
	assert.True(t, isContextOverflow(fmt.Errorf("input length and max_tokens exceed context limit: 180000 + 16384 > 200000")))
	assert.False(t, isContextOverflow(fmt.Errorf("prompt is too long")))
	assert.False(t, isContextOverflow(nil))
}

// --- retryStreamWithFallback tests ---

func TestRetryStreamWithFallback_PrimarySucceeds(t *testing.T) {
	model := &modelTrackingChatModel{}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4"}

	result, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4", result.ModelUsed)
	assert.False(t, result.WasFallback)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStreamWithFallback_FallbackAfterRateLimit(t *testing.T) {
	model := &modelTrackingChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit exceeded")}, // primary fails
			{err: nil}, // fallback succeeds
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4"}

	var fallbackFrom, fallbackTo string
	result, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config,
		func(from, to string, _ error) {
			fallbackFrom = from
			fallbackTo = to
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "claude-haiku-4", result.ModelUsed)
	assert.True(t, result.WasFallback)
	assert.Equal(t, "claude-sonnet-4", fallbackFrom)
	assert.Equal(t, "claude-haiku-4", fallbackTo)
	assert.Equal(t, []string{"claude-sonnet-4", "claude-haiku-4"}, model.modelsUsed)
}

func TestRetryStreamWithFallback_FallbackChain(t *testing.T) {
	model := &modelTrackingChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},      // primary fails
			{err: fmt.Errorf("503 service unavail")}, // first fallback fails
			{err: nil},                               // second fallback succeeds
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4", "gpt-4o-mini"}

	result, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.NoError(t, err)
	assert.Equal(t, "gpt-4o-mini", result.ModelUsed)
	assert.True(t, result.WasFallback)
	assert.Equal(t, []string{"claude-sonnet-4", "claude-haiku-4", "gpt-4o-mini"}, model.modelsUsed)
}

func TestRetryStreamWithFallback_AllModelsFail(t *testing.T) {
	model := &modelTrackingChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},
			{err: fmt.Errorf("429 rate limit")},
			{err: fmt.Errorf("429 rate limit")},
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4", "gpt-4o-mini"}

	_, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all models failed")
	assert.Contains(t, err.Error(), "claude-sonnet-4")
}

func TestRetryStreamWithFallback_NoFallbacksConfigured(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	// No fallbacks configured.

	_, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.Error(t, err)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStreamWithFallback_NonTransientError_NoFallback(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("invalid API key")},
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4"}

	_, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key")
	assert.Equal(t, 1, model.calls) // no fallback attempted
}

func TestRetryStreamWithFallback_FallbackDisabledForRateLimit(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4"}
	falseVal := false
	config.FallbackOnRateLimit = &falseVal

	_, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.Error(t, err)
	assert.Equal(t, 1, model.calls) // no fallback because rate limit fallback is disabled
}

func TestRetryStreamWithFallback_FallbackDisabledForOverload(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("503 service unavailable")},
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbacks = []string{"claude-haiku-4"}
	falseVal := false
	config.FallbackOnOverload = &falseVal

	_, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.Error(t, err)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStreamWithFallback_PrimaryRetriesThenFallback(t *testing.T) {
	model := &modelTrackingChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")}, // primary attempt 1
			{err: fmt.Errorf("429 rate limit")}, // primary attempt 2
			{err: nil},                          // fallback succeeds
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 2
	config.ModelFallbacks = []string{"claude-haiku-4"}

	result, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "claude-sonnet-4"}, config, nil)

	require.NoError(t, err)
	assert.Equal(t, "claude-haiku-4", result.ModelUsed)
	assert.True(t, result.WasFallback)
	// 2 retries on primary + 1 on fallback = 3 calls
	assert.Equal(t, 3, model.calls)
}

func TestRetryStreamWithFallback_FallbackChainRespectsStepAttempts(t *testing.T) {
	model := &modelTrackingChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},          // primary
			{err: fmt.Errorf("503 service unavailable")}, // fallback attempt 1
			{err: nil}, // fallback attempt 2
		},
	}
	config := DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.ModelFallbackChain = []ModelFallbackStep{{Model: "always-ok", MaxRetries: 2}}
	config.ModelFallbacks = []string{"legacy-ignored"}

	result, err := retryStreamWithFallback(context.Background(), model, nil,
		ai.ChatOptions{Model: "always-rate-limit"}, config, nil)

	require.NoError(t, err)
	assert.Equal(t, "always-ok", result.ModelUsed)
	assert.Equal(t, []string{"always-rate-limit", "always-ok", "always-ok"}, model.modelsUsed)
}

// --- classifyTransientError tests ---

func TestClassifyTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected errorType
	}{
		{"nil", nil, errorTypeUnknown},
		{"429", fmt.Errorf("429 rate limit"), errorTypeRateLimit},
		{"529", fmt.Errorf("529 overloaded"), errorTypeRateLimit},
		{"rate limit text", fmt.Errorf("Rate Limit exceeded"), errorTypeRateLimit},
		{"502", fmt.Errorf("502 bad gateway"), errorTypeOverload},
		{"503", fmt.Errorf("503 service unavailable"), errorTypeOverload},
		{"overloaded text", fmt.Errorf("server overloaded"), errorTypeOverload},
		{"timeout", fmt.Errorf("connection timeout"), errorTypeTimeout},
		{"connection reset", fmt.Errorf("connection reset by peer"), errorTypeTimeout},
		{"eof", fmt.Errorf("unexpected EOF"), errorTypeTimeout},
		{"auth error", fmt.Errorf("invalid API key"), errorTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyTransientError(tt.err))
		})
	}
}

// --- Config fallback defaults ---

func TestRunConfig_FallbackDefaults(t *testing.T) {
	cfg := DefaultRunConfig()
	assert.True(t, cfg.IsFallbackOnRateLimit())
	assert.True(t, cfg.IsFallbackOnOverload())
	assert.True(t, cfg.IsFallbackOnTimeout())
}

func TestRunConfig_FallbackExplicitDisable(t *testing.T) {
	cfg := DefaultRunConfig()
	falseVal := false
	cfg.FallbackOnRateLimit = &falseVal
	cfg.FallbackOnOverload = &falseVal
	cfg.FallbackOnTimeout = &falseVal
	assert.False(t, cfg.IsFallbackOnRateLimit())
	assert.False(t, cfg.IsFallbackOnOverload())
	assert.False(t, cfg.IsFallbackOnTimeout())
}

func TestRunConfigFromModelConfig_Fallbacks(t *testing.T) {
	raw := []byte(`{
		"model": "claude-sonnet-4",
		"modelFallbacks": ["claude-haiku-4", "gpt-4o-mini"],
		"fallbackOnRateLimit": false
	}`)
	cfg := RunConfigFromModelConfig(raw)
	assert.Equal(t, []string{"claude-haiku-4", "gpt-4o-mini"}, cfg.ModelFallbacks)
	assert.False(t, cfg.IsFallbackOnRateLimit())
	assert.True(t, cfg.IsFallbackOnOverload()) // default
}
