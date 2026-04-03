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
