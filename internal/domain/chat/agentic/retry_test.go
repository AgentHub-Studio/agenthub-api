package agentic

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
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
	calls       int
	modelsUsed  []string
	results     []stubResult
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
	stream, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3)
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
	stream, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3)
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, 2, model.calls)
}

func TestRetryStream_NonTransientError(t *testing.T) {
	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("invalid API key")},
		},
	}
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3)
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
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 3)
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
	_, err := retryStream(context.Background(), model, nil, ai.ChatOptions{}, 1)
	require.Error(t, err)
	assert.Equal(t, 1, model.calls)
}

func TestRetryStream_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	model := &stubChatModel{
		results: []stubResult{
			{err: fmt.Errorf("429 rate limit")},
		},
	}
	_, err := retryStream(ctx, model, nil, ai.ChatOptions{}, 3)
	require.Error(t, err)
	// Should fail with context error, not retry
	assert.Equal(t, 1, model.calls)
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
			{err: fmt.Errorf("503 service unavail")},  // first fallback fails
			{err: nil},                                 // second fallback succeeds
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
			{err: nil},                           // fallback succeeds
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
