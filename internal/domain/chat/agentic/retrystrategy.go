package agentic

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Comprehensive retry with fast mode fallback and persistent wait.
//
// Inspired by Claude Code's withRetry.ts — handles multiple failure modes
// (429/529 rate limits, auth failures, context overflow, connection stale)
// with dynamic strategy selection based on query source classification.

// RetryCategory classifies how a query source should be retried.
type RetryCategory string

const (
	// RetryCategoryForeground is for user-blocking queries (full retry).
	RetryCategoryForeground RetryCategory = "foreground"
	// RetryCategoryBackground is for non-blocking queries (bail early on overload).
	RetryCategoryBackground RetryCategory = "background"
	// RetryCategoryPersistent is for unattended long-running queries (retry indefinitely).
	RetryCategoryPersistent RetryCategory = "persistent"
)

// RetryErrorKind classifies the type of API error for retry decisions.
type RetryErrorKind string

const (
	RetryErrorRateLimit      RetryErrorKind = "rate_limit"      // 429
	RetryErrorOverloaded     RetryErrorKind = "overloaded"      // 529
	RetryErrorAuth           RetryErrorKind = "auth"            // 401/403
	RetryErrorContextOverflow RetryErrorKind = "context_overflow" // context too large
	RetryErrorConnection     RetryErrorKind = "connection"      // ECONNRESET, etc.
	RetryErrorServer         RetryErrorKind = "server"          // 5xx
	RetryErrorUnknown        RetryErrorKind = "unknown"
)

// RetryableError wraps an error with retry metadata.
type RetryableError struct {
	Err        error
	Kind       RetryErrorKind
	StatusCode int
	RetryAfter time.Duration // server-suggested delay
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("%s error (status %d): %v", e.Kind, e.StatusCode, e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// CannotRetryError indicates the operation cannot be retried.
type CannotRetryError struct {
	OriginalError error
	Reason        string
	Attempts      int
}

func (e *CannotRetryError) Error() string {
	return fmt.Sprintf("cannot retry after %d attempts (%s): %v", e.Attempts, e.Reason, e.OriginalError)
}

func (e *CannotRetryError) Unwrap() error {
	return e.OriginalError
}

// RetryStrategyConfig configures retry behavior.
type RetryStrategyConfig struct {
	MaxRetries        int
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	Category          RetryCategory
	EnableFastFallback bool // if true, switch model on sustained rate limits
	HeartbeatInterval time.Duration // for persistent mode
}

// DefaultRetryStrategyConfig returns defaults for foreground queries.
func DefaultRetryStrategyConfig() RetryStrategyConfig {
	return RetryStrategyConfig{
		MaxRetries:        5,
		BaseDelay:         time.Second,
		MaxDelay:          60 * time.Second,
		Category:          RetryCategoryForeground,
		EnableFastFallback: false,
		HeartbeatInterval: 30 * time.Second,
	}
}

// RetryResult holds the outcome of a retry-wrapped operation.
type RetryResult[T any] struct {
	Value    T
	Attempts int
	Fallback bool // true if fast mode fallback was used
}

// RetryState tracks retry state across attempts.
type RetryState struct {
	mu                    sync.Mutex
	attempts              int
	consecutive529        int
	lastError             error
	fastFallbackTriggered bool
	totalRetryDelay       time.Duration
}

// NewRetryState creates a new retry state, optionally pre-seeded with
// consecutive 529 errors from a previous streaming attempt.
func NewRetryState(initial529Count int) *RetryState {
	return &RetryState{
		consecutive529: initial529Count,
	}
}

// Attempts returns the total number of attempts made.
func (s *RetryState) Attempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts
}

// Consecutive529 returns the current consecutive 529 error count.
func (s *RetryState) Consecutive529() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consecutive529
}

// FastFallbackTriggered returns whether fast mode fallback was activated.
func (s *RetryState) FastFallbackTriggered() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fastFallbackTriggered
}

// TotalRetryDelay returns cumulative time spent waiting for retries.
func (s *RetryState) TotalRetryDelay() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalRetryDelay
}

// RetryStrategy executes an operation with retry logic based on the config.
type RetryStrategy struct {
	config RetryStrategyConfig
	state  *RetryState
}

// NewRetryStrategy creates a retry strategy.
func NewRetryStrategy(config RetryStrategyConfig) *RetryStrategy {
	return &RetryStrategy{
		config: config,
		state:  NewRetryState(0),
	}
}

// NewRetryStrategyWithState creates a retry strategy with pre-existing state.
func NewRetryStrategyWithState(config RetryStrategyConfig, state *RetryState) *RetryStrategy {
	return &RetryStrategy{
		config: config,
		state:  state,
	}
}

// State returns the retry state.
func (r *RetryStrategy) State() *RetryState {
	return r.state
}

// Execute runs the operation with retry logic. The operation receives the
// current attempt number (0-based). Returns the result or a CannotRetryError.
func (r *RetryStrategy) Execute(ctx context.Context, op func(ctx context.Context, attempt int) error) error {
	maxAttempts := r.config.MaxRetries + 1
	if r.config.Category == RetryCategoryPersistent {
		maxAttempts = math.MaxInt32
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		r.state.mu.Lock()
		r.state.attempts = attempt + 1
		r.state.mu.Unlock()

		err := op(ctx, attempt)
		if err == nil {
			r.state.mu.Lock()
			r.state.consecutive529 = 0
			r.state.lastError = nil
			r.state.mu.Unlock()
			return nil
		}

		// Check context cancellation
		if ctx.Err() != nil {
			return &CannotRetryError{
				OriginalError: err,
				Reason:        "context cancelled",
				Attempts:      attempt + 1,
			}
		}

		// Classify the error
		var retryErr *RetryableError
		if !errors.As(err, &retryErr) {
			return &CannotRetryError{
				OriginalError: err,
				Reason:        "non-retryable error",
				Attempts:      attempt + 1,
			}
		}

		// Decide retry based on error kind and category
		if !r.shouldRetry(retryErr) {
			return &CannotRetryError{
				OriginalError: err,
				Reason:        fmt.Sprintf("%s error not retryable for %s category", retryErr.Kind, r.config.Category),
				Attempts:      attempt + 1,
			}
		}

		// Track 529s
		r.state.mu.Lock()
		if retryErr.Kind == RetryErrorOverloaded {
			r.state.consecutive529++
		} else {
			r.state.consecutive529 = 0
		}
		r.state.lastError = err
		r.state.mu.Unlock()

		// Fast fallback check
		if r.config.EnableFastFallback {
			r.state.mu.Lock()
			if r.state.consecutive529 >= 3 && !r.state.fastFallbackTriggered {
				r.state.fastFallbackTriggered = true
			}
			r.state.mu.Unlock()
		}

		// Calculate and apply delay
		delay := r.calculateDelay(attempt, retryErr.RetryAfter)
		r.state.mu.Lock()
		r.state.totalRetryDelay += delay
		r.state.mu.Unlock()

		select {
		case <-ctx.Done():
			return &CannotRetryError{
				OriginalError: err,
				Reason:        "context cancelled during backoff",
				Attempts:      attempt + 1,
			}
		case <-time.After(delay):
		}
	}

	r.state.mu.Lock()
	lastErr := r.state.lastError
	attempts := r.state.attempts
	r.state.mu.Unlock()

	return &CannotRetryError{
		OriginalError: lastErr,
		Reason:        "max retries exhausted",
		Attempts:      attempts,
	}
}

// shouldRetry determines if the error is retryable given the category.
func (r *RetryStrategy) shouldRetry(err *RetryableError) bool {
	switch err.Kind {
	case RetryErrorAuth:
		return false // never retry auth errors
	case RetryErrorContextOverflow:
		return false // caller must reduce context
	case RetryErrorRateLimit:
		return true
	case RetryErrorOverloaded:
		// Background queries bail immediately on overload
		return r.config.Category != RetryCategoryBackground
	case RetryErrorConnection:
		return true
	case RetryErrorServer:
		return true
	default:
		return r.config.Category == RetryCategoryPersistent
	}
}

// calculateDelay returns the backoff delay with jitter, respecting server hints.
func (r *RetryStrategy) calculateDelay(attempt int, serverHint time.Duration) time.Duration {
	base := float64(r.config.BaseDelay) * math.Pow(2, float64(attempt))
	jitter := base * 0.2 * rand.Float64()
	delay := time.Duration(base + jitter)

	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	if serverHint > 0 && serverHint > delay {
		delay = serverHint
	}

	return delay
}

// ClassifyHTTPError maps an HTTP status code to a RetryErrorKind.
func ClassifyHTTPError(statusCode int) RetryErrorKind {
	switch {
	case statusCode == 401 || statusCode == 403:
		return RetryErrorAuth
	case statusCode == 429:
		return RetryErrorRateLimit
	case statusCode == 529:
		return RetryErrorOverloaded
	case statusCode >= 500 && statusCode < 600:
		return RetryErrorServer
	default:
		return RetryErrorUnknown
	}
}
