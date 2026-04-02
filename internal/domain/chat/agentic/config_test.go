package agentic_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestDefaultRunConfig(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	assert.Equal(t, 25, cfg.MaxIterations)
	assert.Equal(t, 4096, cfg.MaxTokensPerCall)
	assert.Equal(t, 200000, cfg.ContextWindowSize)
	assert.Equal(t, 0.75, cfg.CompactThreshold)
	assert.Equal(t, 30*time.Second, cfg.ToolTimeout)
	assert.Equal(t, 5*time.Minute, cfg.TotalTimeout)
	assert.Equal(t, 3, cfg.ConcurrentReadTools)
	assert.Equal(t, 64, cfg.StreamBufferSize)
	assert.Equal(t, "anthropic", cfg.Provider)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.Model)
	assert.Equal(t, 0.7, cfg.Temperature)
}

func TestRunConfigFromModelConfig_OverridesDefaults(t *testing.T) {
	raw := json.RawMessage(`{
		"provider": "openai",
		"model": "gpt-4o",
		"temperature": 0.3,
		"maxTokens": 8192,
		"contextWindow": 128000,
		"maxIterations": 10,
		"compactThreshold": 0.5
	}`)

	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, "openai", cfg.Provider)
	assert.Equal(t, "gpt-4o", cfg.Model)
	assert.Equal(t, 0.3, cfg.Temperature)
	assert.Equal(t, 8192, cfg.MaxTokensPerCall)
	assert.Equal(t, 128000, cfg.ContextWindowSize)
	assert.Equal(t, 10, cfg.MaxIterations)
	assert.Equal(t, 0.5, cfg.CompactThreshold)
	// Non-overridden values should remain default.
	assert.Equal(t, 30*time.Second, cfg.ToolTimeout)
	assert.Equal(t, 64, cfg.StreamBufferSize)
}

func TestRunConfigFromModelConfig_EmptyJSON(t *testing.T) {
	cfg := agentic.RunConfigFromModelConfig(nil)
	assert.Equal(t, agentic.DefaultRunConfig(), cfg)
}

func TestRunConfigFromModelConfig_InvalidJSON(t *testing.T) {
	cfg := agentic.RunConfigFromModelConfig(json.RawMessage(`not-json`))
	assert.Equal(t, agentic.DefaultRunConfig(), cfg)
}

func TestRunConfigFromModelConfig_PartialOverride(t *testing.T) {
	raw := json.RawMessage(`{"model": "claude-opus-4-20250514"}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, "claude-opus-4-20250514", cfg.Model)
	assert.Equal(t, "anthropic", cfg.Provider) // default kept
	assert.Equal(t, 0.7, cfg.Temperature)      // default kept
}

func TestRunConfigFromModelConfig_ZeroTemperature(t *testing.T) {
	raw := json.RawMessage(`{"temperature": 0.0}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, 0.0, cfg.Temperature)
}
