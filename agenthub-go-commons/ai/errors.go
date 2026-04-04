package ai

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for typed AI error handling.
var (
	// ErrRateLimited is returned when the provider returns HTTP 429.
	ErrRateLimited = errors.New("ai: rate limited")

	// ErrContextLengthExceeded is returned when the prompt exceeds the model's context window.
	ErrContextLengthExceeded = errors.New("ai: context length exceeded")

	// ErrInvalidAPIKey is returned when the provider rejects the API key (HTTP 401/403).
	ErrInvalidAPIKey = errors.New("ai: invalid API key")

	// ErrProviderUnavailable is returned when the provider is temporarily unavailable (HTTP 5xx).
	ErrProviderUnavailable = errors.New("ai: provider unavailable")

	// ErrInvalidRequest is returned when the request itself is invalid (HTTP 400).
	ErrInvalidRequest = errors.New("ai: invalid request")

	// ErrContentFiltered is returned when the response was blocked by content filters.
	ErrContentFiltered = errors.New("ai: content filtered")
)

// APIError wraps a provider API error with its HTTP status code and underlying sentinel error.
type APIError struct {
	StatusCode int
	Message    string
	// RetryAfter is the wait duration before retrying. Negative means the provider
	// did not supply a Retry-After header; callers should use exponential back-off.
	RetryAfter time.Duration
	Err        error
}

func (e *APIError) Error() string {
	if e.RetryAfter >= 0 {
		return fmt.Sprintf("ai: status %d: %s (retry after %s)", e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("ai: status %d: %s", e.StatusCode, e.Message)
}

// Unwrap returns the underlying sentinel error so callers can use errors.Is.
func (e *APIError) Unwrap() error { return e.Err }
