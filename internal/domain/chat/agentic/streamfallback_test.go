package agentic_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- FallbackTriggeredError ---

func TestFallbackTriggeredError_Message(t *testing.T) {
	err := &agentic.FallbackTriggeredError{
		OriginalModel: "claude-opus-4-6",
		FallbackModel: "claude-sonnet-4-20250514",
	}
	assert.Contains(t, err.Error(), "claude-opus-4-6")
	assert.Contains(t, err.Error(), "claude-sonnet-4-20250514")
	assert.Contains(t, err.Error(), "fallback triggered")
}

func TestFallbackTriggeredError_IsError(t *testing.T) {
	var err error = &agentic.FallbackTriggeredError{
		OriginalModel: "opus",
		FallbackModel: "sonnet",
	}
	var fte *agentic.FallbackTriggeredError
	assert.True(t, errors.As(err, &fte))
	assert.Equal(t, "opus", fte.OriginalModel)
}

// --- DefaultStreamFallbackConfig ---

func TestDefaultStreamFallbackConfig(t *testing.T) {
	cfg := agentic.DefaultStreamFallbackConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 300_000, cfg.TimeoutMs)
	assert.Equal(t, 3, cfg.Max529Retries)
}

// --- EvaluateModelFallback ---

func TestEvaluateModelFallback_BelowThreshold(t *testing.T) {
	decision := agentic.EvaluateModelFallback(2, 3, "opus", []string{"sonnet"})
	assert.False(t, decision.ShouldFallback)
	assert.Empty(t, decision.FallbackModel)
}

func TestEvaluateModelFallback_AtThreshold_NoFallbacks(t *testing.T) {
	decision := agentic.EvaluateModelFallback(3, 3, "opus", nil)
	assert.False(t, decision.ShouldFallback)
}

func TestEvaluateModelFallback_AtThreshold_WithFallback(t *testing.T) {
	decision := agentic.EvaluateModelFallback(3, 3, "opus", []string{"sonnet", "haiku"})
	assert.True(t, decision.ShouldFallback)
	assert.Equal(t, "sonnet", decision.FallbackModel)
	assert.Contains(t, decision.Reason, "529")
	assert.Contains(t, decision.Reason, "opus")
}

func TestEvaluateModelFallback_AboveThreshold(t *testing.T) {
	decision := agentic.EvaluateModelFallback(5, 3, "opus", []string{"sonnet"})
	assert.True(t, decision.ShouldFallback)
}

func TestEvaluateModelFallback_ZeroThreshold(t *testing.T) {
	decision := agentic.EvaluateModelFallback(0, 0, "opus", []string{"sonnet"})
	assert.True(t, decision.ShouldFallback)
}

// --- StreamFallbackResult field tests ---

func TestStreamFallbackResult_Fields(t *testing.T) {
	result := agentic.StreamFallbackResult{
		Model:             "sonnet",
		UsedNonStreaming:  true,
		UsedFallbackModel: true,
	}
	assert.Equal(t, "sonnet", result.Model)
	assert.True(t, result.UsedNonStreaming)
	assert.True(t, result.UsedFallbackModel)
}

// --- retryStreamWithNonStreamingFallback (via exported wrapper would be ideal,
//     but since it's unexported we test the public types and EvaluateModelFallback) ---

// Test the complete model fallback flow using EvaluateModelFallback as the public API.
func TestModelFallback_FullScenario(t *testing.T) {
	primary := "claude-opus-4-6"
	fallbacks := []string{"claude-sonnet-4-20250514", "claude-haiku-4-5-20251001"}

	// Simulate consecutive 529 errors.
	consecutive529 := 0
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		consecutive529++
		decision := agentic.EvaluateModelFallback(consecutive529, maxRetries, primary, fallbacks)
		if i < maxRetries-1 {
			assert.False(t, decision.ShouldFallback, "should not fallback on attempt %d", i)
		} else {
			assert.True(t, decision.ShouldFallback, "should fallback on attempt %d", i)
			assert.Equal(t, "claude-sonnet-4-20250514", decision.FallbackModel)
		}
	}
}

// --- StreamFallbackConfig validation ---

func TestStreamFallbackConfig_Disabled(t *testing.T) {
	cfg := agentic.StreamFallbackConfig{
		Enabled:       false,
		TimeoutMs:     60_000,
		Max529Retries: 3,
	}
	assert.False(t, cfg.Enabled)
}

func TestStreamFallbackConfig_RemoteTimeout(t *testing.T) {
	cfg := agentic.StreamFallbackConfig{
		Enabled:       true,
		TimeoutMs:     120_000, // Remote: 2 minutes
		Max529Retries: 3,
	}
	assert.Equal(t, 120_000, cfg.TimeoutMs)
}

// --- FallbackTriggeredError type assertion ---

func TestFallbackTriggeredError_TypeAssertion(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &agentic.FallbackTriggeredError{
		OriginalModel: "opus",
		FallbackModel: "sonnet",
	})

	var fte *agentic.FallbackTriggeredError
	require.True(t, errors.As(err, &fte))
	assert.Equal(t, "opus", fte.OriginalModel)
	assert.Equal(t, "sonnet", fte.FallbackModel)
}
