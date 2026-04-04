package agentic_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- RetryCategory values ---

func TestRetryCategory_Values(t *testing.T) {
	assert.Equal(t, agentic.RetryCategory("foreground"), agentic.RetryCategoryForeground)
	assert.Equal(t, agentic.RetryCategory("background"), agentic.RetryCategoryBackground)
	assert.Equal(t, agentic.RetryCategory("persistent"), agentic.RetryCategoryPersistent)
}

// --- RetryErrorKind values ---

func TestRetryErrorKind_Values(t *testing.T) {
	assert.Equal(t, agentic.RetryErrorKind("rate_limit"), agentic.RetryErrorRateLimit)
	assert.Equal(t, agentic.RetryErrorKind("overloaded"), agentic.RetryErrorOverloaded)
	assert.Equal(t, agentic.RetryErrorKind("auth"), agentic.RetryErrorAuth)
	assert.Equal(t, agentic.RetryErrorKind("context_overflow"), agentic.RetryErrorContextOverflow)
	assert.Equal(t, agentic.RetryErrorKind("connection"), agentic.RetryErrorConnection)
}

// --- RetryableError ---

func TestRetryableError(t *testing.T) {
	err := &agentic.RetryableError{
		Err:        errors.New("too many requests"),
		Kind:       agentic.RetryErrorRateLimit,
		StatusCode: 429,
		RetryAfter: 5 * time.Second,
	}
	assert.Contains(t, err.Error(), "rate_limit")
	assert.Contains(t, err.Error(), "429")
	assert.Equal(t, "too many requests", errors.Unwrap(err).Error())
}

// --- CannotRetryError ---

func TestCannotRetryError(t *testing.T) {
	orig := errors.New("auth failed")
	err := &agentic.CannotRetryError{
		OriginalError: orig,
		Reason:        "auth error",
		Attempts:      3,
	}
	assert.Contains(t, err.Error(), "3 attempts")
	assert.Contains(t, err.Error(), "auth error")
	assert.Equal(t, orig, errors.Unwrap(err))
}

// --- DefaultRetryStrategyConfig ---

func TestDefaultRetryStrategyConfig(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, time.Second, cfg.BaseDelay)
	assert.Equal(t, 60*time.Second, cfg.MaxDelay)
	assert.Equal(t, agentic.RetryCategoryForeground, cfg.Category)
}

// --- RetryStrategy.Execute ---

func TestRetryStrategy_Execute_Success(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	r := agentic.NewRetryStrategy(cfg)

	err := r.Execute(context.Background(), func(ctx context.Context, attempt int) error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, r.State().Attempts())
}

func TestRetryStrategy_Execute_NonRetryableError(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	r := agentic.NewRetryStrategy(cfg)

	err := r.Execute(context.Background(), func(ctx context.Context, attempt int) error {
		return errors.New("plain error")
	})

	require.Error(t, err)
	var cannotRetry *agentic.CannotRetryError
	assert.True(t, errors.As(err, &cannotRetry))
	assert.Equal(t, "non-retryable error", cannotRetry.Reason)
}

func TestRetryStrategy_Execute_RetryThenSucceed(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = 10 * time.Millisecond
	r := agentic.NewRetryStrategy(cfg)

	attempt := 0
	err := r.Execute(context.Background(), func(ctx context.Context, a int) error {
		attempt++
		if attempt < 3 {
			return &agentic.RetryableError{
				Err:        errors.New("rate limited"),
				Kind:       agentic.RetryErrorRateLimit,
				StatusCode: 429,
			}
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, r.State().Attempts())
}

func TestRetryStrategy_Execute_MaxRetriesExhausted(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	cfg.MaxRetries = 2
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = time.Millisecond
	r := agentic.NewRetryStrategy(cfg)

	err := r.Execute(context.Background(), func(ctx context.Context, a int) error {
		return &agentic.RetryableError{
			Err:        errors.New("server error"),
			Kind:       agentic.RetryErrorServer,
			StatusCode: 500,
		}
	})

	require.Error(t, err)
	var cannotRetry *agentic.CannotRetryError
	assert.True(t, errors.As(err, &cannotRetry))
	assert.Equal(t, "max retries exhausted", cannotRetry.Reason)
}

func TestRetryStrategy_Execute_AuthNotRetried(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	r := agentic.NewRetryStrategy(cfg)

	err := r.Execute(context.Background(), func(ctx context.Context, a int) error {
		return &agentic.RetryableError{
			Err:        errors.New("unauthorized"),
			Kind:       agentic.RetryErrorAuth,
			StatusCode: 401,
		}
	})

	require.Error(t, err)
	assert.Equal(t, 1, r.State().Attempts())
}

func TestRetryStrategy_Execute_ContextCancelled(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	cfg.BaseDelay = time.Second
	r := agentic.NewRetryStrategy(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := r.Execute(ctx, func(ctx context.Context, a int) error {
		return &agentic.RetryableError{
			Err:  errors.New("rate limited"),
			Kind: agentic.RetryErrorRateLimit,
		}
	})

	require.Error(t, err)
	var cannotRetry *agentic.CannotRetryError
	assert.True(t, errors.As(err, &cannotRetry))
	assert.Contains(t, cannotRetry.Reason, "context cancelled")
}

// --- Background category bails on overload ---

func TestRetryStrategy_Background_BailsOnOverload(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	cfg.Category = agentic.RetryCategoryBackground
	r := agentic.NewRetryStrategy(cfg)

	err := r.Execute(context.Background(), func(ctx context.Context, a int) error {
		return &agentic.RetryableError{
			Err:        errors.New("overloaded"),
			Kind:       agentic.RetryErrorOverloaded,
			StatusCode: 529,
		}
	})

	require.Error(t, err)
	assert.Equal(t, 1, r.State().Attempts())
}

// --- Fast fallback triggered ---

func TestRetryStrategy_FastFallback(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	cfg.EnableFastFallback = true
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = time.Millisecond
	cfg.MaxRetries = 5
	r := agentic.NewRetryStrategy(cfg)

	attempt := 0
	_ = r.Execute(context.Background(), func(ctx context.Context, a int) error {
		attempt++
		if attempt <= 4 {
			return &agentic.RetryableError{
				Err:        errors.New("overloaded"),
				Kind:       agentic.RetryErrorOverloaded,
				StatusCode: 529,
			}
		}
		return nil
	})

	assert.True(t, r.State().FastFallbackTriggered())
}

// --- RetryState pre-seeded ---

func TestRetryState_PreSeeded(t *testing.T) {
	state := agentic.NewRetryState(5)
	assert.Equal(t, 5, state.Consecutive529())
}

// --- ClassifyHTTPError ---

func TestClassifyHTTPError(t *testing.T) {
	assert.Equal(t, agentic.RetryErrorAuth, agentic.ClassifyHTTPError(401))
	assert.Equal(t, agentic.RetryErrorAuth, agentic.ClassifyHTTPError(403))
	assert.Equal(t, agentic.RetryErrorRateLimit, agentic.ClassifyHTTPError(429))
	assert.Equal(t, agentic.RetryErrorOverloaded, agentic.ClassifyHTTPError(529))
	assert.Equal(t, agentic.RetryErrorServer, agentic.ClassifyHTTPError(500))
	assert.Equal(t, agentic.RetryErrorServer, agentic.ClassifyHTTPError(503))
	assert.Equal(t, agentic.RetryErrorUnknown, agentic.ClassifyHTTPError(400))
}

// --- TotalRetryDelay tracked ---

func TestRetryStrategy_TotalRetryDelay(t *testing.T) {
	cfg := agentic.DefaultRetryStrategyConfig()
	cfg.BaseDelay = 5 * time.Millisecond
	cfg.MaxDelay = 10 * time.Millisecond
	cfg.MaxRetries = 2
	r := agentic.NewRetryStrategy(cfg)

	attempt := 0
	_ = r.Execute(context.Background(), func(ctx context.Context, a int) error {
		attempt++
		if attempt < 3 {
			return &agentic.RetryableError{
				Err:  errors.New("fail"),
				Kind: agentic.RetryErrorConnection,
			}
		}
		return nil
	})

	assert.Greater(t, r.State().TotalRetryDelay(), time.Duration(0))
}
