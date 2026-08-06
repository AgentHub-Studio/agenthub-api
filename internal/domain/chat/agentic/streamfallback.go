package agentic

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// FallbackTriggeredError is returned when the primary model fails with
// consecutive capacity errors (529) and a fallback model is configured.
// The caller should retry with the fallback model.
//
// Inspired by Claude Code's FallbackTriggeredError in withRetry.ts.
type FallbackTriggeredError struct {
	// OriginalModel is the model that was requested.
	OriginalModel string
	// FallbackModel is the model to switch to.
	FallbackModel string
}

func (e *FallbackTriggeredError) Error() string {
	return fmt.Sprintf("model fallback triggered: %s -> %s", e.OriginalModel, e.FallbackModel)
}

// StreamFallbackConfig controls the streaming-to-non-streaming fallback behavior.
//
// Inspired by Claude Code's getNonstreamingFallbackTimeoutMs and retry options.
type StreamFallbackConfig struct {
	// Enabled controls whether non-streaming fallback is attempted when streaming fails.
	Enabled bool
	// TimeoutMs is the maximum duration for the non-streaming request.
	// Default: 300_000 (5 minutes) for local, 120_000 (2 minutes) for remote.
	TimeoutMs int
	// Max529Retries is the number of consecutive 529 errors before triggering
	// a model fallback (if FallbackModel is set). Default: 3.
	Max529Retries int
}

// DefaultStreamFallbackConfig returns sensible defaults for local environments.
func DefaultStreamFallbackConfig() StreamFallbackConfig {
	return StreamFallbackConfig{
		Enabled:       true,
		TimeoutMs:     300_000,
		Max529Retries: 3,
	}
}

// StreamFallbackResult holds the outcome of a stream-with-fallback attempt.
type StreamFallbackResult struct {
	// Stream is the result stream (from streaming or non-streaming path).
	Stream <-chan ai.StreamChunk
	// Model is the model that actually served the request. Empty means primary.
	Model string
	// UsedNonStreaming indicates the response came from the non-streaming API.
	UsedNonStreaming bool
	// UsedFallbackModel indicates a fallback model was used instead of the primary.
	UsedFallbackModel bool
}

// retryStreamWithNonStreamingFallback attempts a streaming request first. If it
// fails with a streaming-specific error (e.g. 404 on streaming endpoint, connection
// reset during stream), it retries using the non-streaming API with a timeout.
//
// If consecutive 529 errors exceed Max529Retries and a fallback model is configured,
// it returns a FallbackTriggeredError for the caller to handle.
//
// Inspired by Claude Code's executeNonStreamingRequest fallback in claude.ts.
func retryStreamWithNonStreamingFallback(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	retryOpts StreamFallbackConfig,
	source QuerySource,
) (StreamFallbackResult, error) {
	// First, try the streaming path with retries.
	stream, err := retryStream(ctx, model, messages, opts, 3, source)
	if err == nil {
		return StreamFallbackResult{Stream: stream}, nil
	}

	result, _, fallbackErr := retryNonStreamingAfterStreamFailure(ctx, model, messages, opts, retryOpts, err)
	return result, fallbackErr
}

// retryNonStreamingAfterStreamFailure converts an already exhausted streaming
// attempt to a non-streaming request when the error is stream-specific. The
// attempted result distinguishes an ineligible error from a failed fallback.
func retryNonStreamingAfterStreamFailure(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	retryOpts StreamFallbackConfig,
	streamErr error,
) (result StreamFallbackResult, attempted bool, err error) {

	// If non-streaming fallback is disabled, return the error.
	if !retryOpts.Enabled {
		return StreamFallbackResult{}, false, streamErr
	}

	// Don't fallback for errors that won't be helped by switching to non-streaming.
	if isPromptTooLong(streamErr) || isAbortError(streamErr) || isMediaSizeError(streamErr) {
		return StreamFallbackResult{}, false, streamErr
	}

	// Check if this is a streaming-specific error worth retrying non-streaming.
	if !isStreamingFallbackEligible(streamErr) {
		return StreamFallbackResult{}, false, streamErr
	}

	slog.Warn("streaming failed, falling back to non-streaming API",
		"model", opts.Model,
		"error", streamErr,
		"timeout_ms", retryOpts.TimeoutMs,
	)

	// Create a timeout context for the non-streaming fallback.
	timeout := time.Duration(retryOpts.TimeoutMs) * time.Millisecond
	nsCtx, nsCancel := context.WithTimeout(ctx, timeout)
	defer nsCancel()

	// Use the Chat (non-streaming) API.
	response, nsErr := model.Chat(nsCtx, messages, opts)
	if nsErr == nil && response == nil {
		nsErr = fmt.Errorf("non-streaming fallback returned no response")
	}
	if nsErr != nil {
		// Return the original streaming error if non-streaming also fails.
		slog.Warn("non-streaming fallback also failed",
			"model", opts.Model,
			"streamError", streamErr,
			"nonStreamError", nsErr,
		)
		return StreamFallbackResult{}, true, fmt.Errorf("streaming and non-streaming both failed: streaming=%v, non-streaming=%w", streamErr, nsErr)
	}

	// Convert the non-streaming response into a single-chunk stream.
	// StreamChunk uses Delta (not Content) and ToolCallDelta (single, not slice).
	ch := make(chan ai.StreamChunk, 2)
	ch <- ai.StreamChunk{
		Delta: response.Content,
	}
	// Emit a final chunk with finish reason and usage.
	usage := response.Usage
	ch <- ai.StreamChunk{
		FinishReason: response.FinishReason,
		Usage:        &usage,
	}
	close(ch)

	return StreamFallbackResult{
		Stream:           ch,
		Model:            response.Model,
		UsedNonStreaming: true,
	}, true, nil
}

// isStreamingFallbackEligible returns true if the error suggests a streaming-specific
// issue that might be resolved by using the non-streaming API.
//
// Inspired by Claude Code's streaming fallback trigger conditions.
func isStreamingFallbackEligible(err error) bool {
	if err == nil {
		return false
	}

	// 404 on streaming endpoint (provider may not support streaming for this model).
	if isHTTPStatusError(err, "404") {
		return true
	}

	// Connection-level errors during stream (reset, broken pipe).
	if isStaleConnectionError(err) {
		return true
	}

	// Overload errors may work better non-streaming (simpler server-side processing).
	if isOverloadError(err) {
		return true
	}

	return false
}

// isHTTPStatusError checks if an error message contains a specific HTTP status code.
func isHTTPStatusError(err error, statusCode string) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), statusCode)
}

// containsAny checks if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// ModelFallbackDecision represents the outcome of evaluating whether to switch models.
type ModelFallbackDecision struct {
	// ShouldFallback is true when the caller should retry with FallbackModel.
	ShouldFallback bool
	// FallbackModel is the model to switch to (empty if ShouldFallback is false).
	FallbackModel string
	// Reason explains why fallback was triggered.
	Reason string
}

// EvaluateModelFallback checks whether consecutive 529 errors warrant a model fallback.
// Called by the runner's main loop after each failed LLM call.
//
// Inspired by Claude Code's 529 fallback logic in withRetry.ts.
func EvaluateModelFallback(consecutive529 int, max529Retries int, primaryModel string, fallbackModels []string) ModelFallbackDecision {
	if consecutive529 < max529Retries || len(fallbackModels) == 0 {
		return ModelFallbackDecision{}
	}

	return ModelFallbackDecision{
		ShouldFallback: true,
		FallbackModel:  fallbackModels[0],
		Reason:         fmt.Sprintf("exceeded %d consecutive 529 errors on %s", max529Retries, primaryModel),
	}
}
