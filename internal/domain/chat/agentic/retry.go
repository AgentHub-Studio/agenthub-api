package agentic

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// FallbackResult is returned by retryStreamWithFallback to indicate which model was used.
type FallbackResult struct {
	Stream       <-chan ai.StreamChunk
	ModelUsed    string // the model that succeeded
	WasFallback  bool   // true if a fallback model was used
}

// retryStreamWithFallback wraps retryStream with model fallback support.
// After exhausting retries on the primary model, it tries each fallback model
// in order (1 attempt per fallback). Returns the stream plus metadata about
// which model was used.
func retryStreamWithFallback(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	config RunConfig,
	onFallback func(from, to string, err error),
) (*FallbackResult, error) {
	// Try primary model with full retries.
	stream, err := retryStream(ctx, model, messages, opts, config.RetryMaxAttempts)
	if err == nil {
		return &FallbackResult{Stream: stream, ModelUsed: opts.Model}, nil
	}

	// No fallbacks configured — return the original error.
	if len(config.ModelFallbacks) == 0 {
		return nil, err
	}

	// Check if the error type qualifies for fallback.
	if !shouldFallback(err, config) {
		return nil, err
	}

	primaryModel := opts.Model
	primaryErr := err

	// Try each fallback model with 1 attempt.
	for _, fallbackModel := range config.ModelFallbacks {
		slog.Warn("falling back to alternative model",
			"from", primaryModel,
			"to", fallbackModel,
			"primaryError", primaryErr,
		)

		if onFallback != nil {
			onFallback(primaryModel, fallbackModel, primaryErr)
		}

		fallbackOpts := opts
		fallbackOpts.Model = fallbackModel

		stream, err := retryStream(ctx, model, messages, fallbackOpts, 1)
		if err == nil {
			return &FallbackResult{
				Stream:      stream,
				ModelUsed:   fallbackModel,
				WasFallback: true,
			}, nil
		}

		slog.Warn("fallback model also failed",
			"model", fallbackModel,
			"error", err,
		)
	}

	return nil, fmt.Errorf("all models failed (primary: %s, fallbacks: %v): %w",
		primaryModel, config.ModelFallbacks, primaryErr)
}

// shouldFallback checks if the error type qualifies for model fallback
// based on the configuration.
func shouldFallback(err error, config RunConfig) bool {
	if err == nil {
		return false
	}

	errType := classifyTransientError(err)
	switch errType {
	case errorTypeRateLimit:
		return config.IsFallbackOnRateLimit()
	case errorTypeOverload:
		return config.IsFallbackOnOverload()
	case errorTypeTimeout:
		return config.IsFallbackOnTimeout()
	default:
		return false
	}
}

// errorType classifies transient errors for fallback routing.
type errorType int

const (
	errorTypeUnknown   errorType = iota
	errorTypeRateLimit           // 429, 529, "rate limit"
	errorTypeOverload            // 502, 503, "overloaded"
	errorTypeTimeout             // "timeout", "connection reset"
)

// classifyTransientError determines the category of a transient error.
func classifyTransientError(err error) errorType {
	if err == nil {
		return errorTypeUnknown
	}
	msg := strings.ToLower(err.Error())

	// Rate limit patterns.
	if strings.Contains(msg, "429") || strings.Contains(msg, "529") || strings.Contains(msg, "rate limit") {
		return errorTypeRateLimit
	}

	// Overload patterns.
	if strings.Contains(msg, "502") || strings.Contains(msg, "503") || strings.Contains(msg, "overloaded") {
		return errorTypeOverload
	}

	// Timeout patterns.
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "connection reset") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "eof") {
		return errorTypeTimeout
	}

	return errorTypeUnknown
}

// retryStream wraps ChatModel.ChatStream with exponential backoff for transient errors.
// It retries up to maxAttempts times (total attempts, not retries). If maxAttempts <= 1
// it calls once without retry.
func retryStream(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	maxAttempts int,
) (<-chan ai.StreamChunk, error) {
	if maxAttempts <= 1 {
		return model.ChatStream(ctx, messages, opts)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stream, err := model.ChatStream(ctx, messages, opts)
		if err == nil {
			return stream, nil
		}

		lastErr = err
		if !isTransientError(err) {
			return nil, err
		}

		if attempt == maxAttempts {
			break
		}

		// Exponential backoff: 1s, 2s, 4s, ...
		backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
		slog.Warn("retrying LLM call after transient error",
			"attempt", attempt,
			"maxAttempts", maxAttempts,
			"backoff", backoff,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// isTransientError returns true for errors that are likely transient and worth retrying.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	// Rate limit and overload status codes.
	for _, code := range []string{"429", "529", "503", "502"} {
		if strings.Contains(msg, code) {
			return true
		}
	}

	// Common transient error patterns.
	transientPatterns := []string{
		"rate limit",
		"overloaded",
		"timeout",
		"connection reset",
		"connection refused",
		"eof",
		"temporary",
	}
	lower := strings.ToLower(msg)
	for _, pat := range transientPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}

	return false
}
