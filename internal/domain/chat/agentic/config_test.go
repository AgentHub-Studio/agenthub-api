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

func TestDefaultRunConfig_NewFields(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	assert.Equal(t, 0.0, cfg.MaxBudgetUSD, "default budget should be 0 (no limit)")
	assert.Equal(t, 50000, cfg.MaxToolResultChars)
	assert.Equal(t, 3, cfg.RetryMaxAttempts)
}

func TestRunConfigFromModelConfig_NewFieldsOverride(t *testing.T) {
	raw := json.RawMessage(`{
		"maxBudgetUsd": 5.0,
		"maxToolResultChars": 10000,
		"retryMaxAttempts": 5
	}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, 5.0, cfg.MaxBudgetUSD)
	assert.Equal(t, 10000, cfg.MaxToolResultChars)
	assert.Equal(t, 5, cfg.RetryMaxAttempts)
}

func TestRunConfigFromModelConfig_NewFieldsDefaults(t *testing.T) {
	raw := json.RawMessage(`{"model": "gpt-4o"}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, 0.0, cfg.MaxBudgetUSD, "should keep default")
	assert.Equal(t, 50000, cfg.MaxToolResultChars, "should keep default")
	assert.Equal(t, 3, cfg.RetryMaxAttempts, "should keep default")
}

// --- Token Budget Per-Turn tests ---

func TestEffectiveTurnBudget_NoLimit(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	assert.Equal(t, 0, cfg.EffectiveTurnBudget(0))
	assert.Equal(t, 0, cfg.EffectiveTurnBudget(5))
}

func TestEffectiveTurnBudget_FixedBudget(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.MaxTokensPerTurn = 8000
	assert.Equal(t, 8000, cfg.EffectiveTurnBudget(0))
	assert.Equal(t, 8000, cfg.EffectiveTurnBudget(1))
	assert.Equal(t, 8000, cfg.EffectiveTurnBudget(5))
}

func TestEffectiveTurnBudget_Escalation(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.MaxTokensPerTurn = 4000
	cfg.BudgetEscalation = true
	cfg.EscalationFactor = 1.5

	// Turn 0: 4000 (no escalation on first turn).
	assert.Equal(t, 4000, cfg.EffectiveTurnBudget(0))
	// Turn 1: 4000 * 1.5^1 = 6000.
	assert.Equal(t, 6000, cfg.EffectiveTurnBudget(1))
	// Turn 2: 4000 * 1.5^2 = 9000.
	assert.Equal(t, 9000, cfg.EffectiveTurnBudget(2))
	// Turn 3: 4000 * 1.5^3 = 13500.
	assert.Equal(t, 13500, cfg.EffectiveTurnBudget(3))
}

func TestEffectiveTurnBudget_EscalationDefaultFactor(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.MaxTokensPerTurn = 4000
	cfg.BudgetEscalation = true
	// EscalationFactor is 0 → default 1.5.

	assert.Equal(t, 4000, cfg.EffectiveTurnBudget(0))
	assert.Equal(t, 6000, cfg.EffectiveTurnBudget(1))
}

func TestEffectiveTurnBudget_EscalationDisabled(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.MaxTokensPerTurn = 4000
	cfg.BudgetEscalation = false
	cfg.EscalationFactor = 2.0

	// Factor ignored when escalation is disabled.
	assert.Equal(t, 4000, cfg.EffectiveTurnBudget(0))
	assert.Equal(t, 4000, cfg.EffectiveTurnBudget(5))
}

func TestRunConfigFromModelConfig_TokenBudgetPerTurn(t *testing.T) {
	raw := json.RawMessage(`{
		"maxTokensPerTurn": 10000,
		"budgetEscalation": true,
		"escalationFactor": 2.0
	}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, 10000, cfg.MaxTokensPerTurn)
	assert.True(t, cfg.BudgetEscalation)
	assert.Equal(t, 2.0, cfg.EscalationFactor)
}

func TestRunConfigFromModelConfig_TokenBudgetDefaults(t *testing.T) {
	raw := json.RawMessage(`{"model": "gpt-4o"}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, 0, cfg.MaxTokensPerTurn)
	assert.False(t, cfg.BudgetEscalation)
	assert.Equal(t, 0.0, cfg.EscalationFactor)
}
