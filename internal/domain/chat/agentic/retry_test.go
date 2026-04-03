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

func (s *stubChatModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
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
