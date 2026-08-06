package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

type streamFallbackInternalMockModel struct {
	chatStreamErr error
	chatResponse  *ai.ChatResponse
	chatErr       error
	chatCalls     int
	streamCalls   int
}

func (m *streamFallbackInternalMockModel) Chat(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
	m.chatCalls++
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return m.chatResponse, nil
}

func (m *streamFallbackInternalMockModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	m.streamCalls++
	if m.chatStreamErr != nil {
		return nil, m.chatStreamErr
	}
	ch := make(chan ai.StreamChunk, 1)
	ch <- ai.StreamChunk{Delta: "streamed", FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func (m *streamFallbackInternalMockModel) GetProviderName() string { return "mock" }

func TestRetryStreamWithNonStreamingFallback_UsesChatWhenStreamingEndpointIsUnavailable(t *testing.T) {
	model := &streamFallbackInternalMockModel{
		chatStreamErr: errors.New("provider returned HTTP 404 for streaming endpoint"),
		chatResponse: &ai.ChatResponse{
			Content:      "fallback response",
			FinishReason: "stop",
			Model:        "mock-model",
			Usage:        ai.Usage{TotalTokens: 9},
		},
	}

	result, err := retryStreamWithNonStreamingFallback(
		context.Background(),
		model,
		[]ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		ai.ChatOptions{Model: "mock-model"},
		StreamFallbackConfig{Enabled: true, TimeoutMs: 1000, Max529Retries: 3},
		SourceMainLoop,
	)

	require.NoError(t, err)
	require.True(t, result.UsedNonStreaming)
	require.Equal(t, "mock-model", result.Model)
	require.Equal(t, 1, model.streamCalls)
	require.Equal(t, 1, model.chatCalls)

	chunk := <-result.Stream
	require.Equal(t, "fallback response", chunk.Delta)
	final := <-result.Stream
	require.Equal(t, "stop", final.FinishReason)
	require.NotNil(t, final.Usage)
	require.Equal(t, 9, final.Usage.TotalTokens)
}

func TestRetryStreamWithNonStreamingFallback_RejectsEmptyNonStreamingResponse(t *testing.T) {
	model := &streamFallbackInternalMockModel{
		chatStreamErr: errors.New("provider returned HTTP 404 for streaming endpoint"),
	}

	_, err := retryStreamWithNonStreamingFallback(
		context.Background(),
		model,
		[]ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		ai.ChatOptions{Model: "mock-model"},
		StreamFallbackConfig{Enabled: true, TimeoutMs: 1000},
		SourceMainLoop,
	)

	require.Error(t, err)
	require.ErrorContains(t, err, "non-streaming fallback returned no response")
	require.Equal(t, 1, model.streamCalls)
	require.Equal(t, 1, model.chatCalls)
}
