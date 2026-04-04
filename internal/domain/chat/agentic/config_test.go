package agentic_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
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

func TestRunConfigFromModelConfig_EffortOverride(t *testing.T) {
	raw := json.RawMessage(`{"effort": "medium"}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.NotNil(t, cfg.Effort)
	assert.Equal(t, ai.EffortMedium, *cfg.Effort)
}

func TestRunConfigFromModelConfig_EffortDefault(t *testing.T) {
	raw := json.RawMessage(`{"model": "gpt-4o"}`)
	cfg := agentic.RunConfigFromModelConfig(raw)
	assert.Nil(t, cfg.Effort, "effort should be nil by default")
}

// --- BuildRunGates ---

func TestBuildRunGates_AnthropicOpus(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.Model = "claude-opus-4-6-20250514"
	cfg.MaxBudgetUSD = 10.0
	cfg.OutputTokenBudget = 5000
	cfg.MaxToolResultsPerTurnChars = 100000
	cfg.ToolResultLimits = map[string]int{"search": 5000}

	gates := agentic.BuildRunGates(cfg, 0, true)

	assert.True(t, gates.Model.SupportsEffort)
	assert.True(t, gates.Model.SupportsMaxEffort)
	assert.True(t, gates.Model.SupportsThinking)
	assert.True(t, gates.Model.SupportsCacheControl)
	assert.True(t, gates.Model.SupportsVision)
	assert.True(t, gates.Model.SupportsToolUse)
	assert.True(t, gates.CacheControl) // anthropic provider
	assert.True(t, gates.HasBudgetLimit)
	assert.True(t, gates.HasOutputTokenBudget)
	assert.True(t, gates.CoordinatorMode) // depth 0 < maxDepth 3, has subtask exec
	assert.True(t, gates.HasToolResultLimits)
	assert.True(t, gates.HasAggregateResultLimit)
	assert.Equal(t, 200000, gates.ContextWindowTokens)
}

func TestBuildRunGates_OpenAI(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.Provider = "openai"
	cfg.Model = "gpt-4o"

	gates := agentic.BuildRunGates(cfg, 0, false)

	assert.False(t, gates.Model.SupportsEffort)
	assert.False(t, gates.Model.SupportsMaxEffort)
	assert.False(t, gates.Model.SupportsThinking)
	assert.False(t, gates.Model.SupportsCacheControl) // not anthropic
	assert.True(t, gates.Model.SupportsVision)        // gpt-4o supports vision
	assert.True(t, gates.Model.SupportsToolUse)
	assert.Nil(t, gates.ResolvedEffort)
	assert.False(t, gates.CacheControl)
	assert.False(t, gates.HasBudgetLimit) // MaxBudgetUSD = 0
	assert.False(t, gates.HasOutputTokenBudget)
	assert.False(t, gates.CoordinatorMode) // no subtask exec
	assert.False(t, gates.HasToolResultLimits)
	assert.True(t, gates.HasAggregateResultLimit) // default 200000 > 0
	assert.Equal(t, 128000, gates.ContextWindowTokens)
}

func TestBuildRunGates_CoordinatorDisabledAtMaxDepth(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.MaxDepth = 3

	gates := agentic.BuildRunGates(cfg, 3, true)
	assert.False(t, gates.CoordinatorMode, "should be disabled when depth >= maxDepth")

	gates = agentic.BuildRunGates(cfg, 2, true)
	assert.True(t, gates.CoordinatorMode, "should be enabled when depth < maxDepth")
}

func TestBuildRunGates_NoSubtaskExec(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	gates := agentic.BuildRunGates(cfg, 0, false)
	assert.False(t, gates.CoordinatorMode, "should be disabled without subtask executor")
}

// --- DetectModelCapabilities ---

func TestDetectModelCapabilities_ClaudeOpus(t *testing.T) {
	caps := agentic.DetectModelCapabilities("claude-opus-4-6-20250514", "anthropic")
	assert.True(t, caps.SupportsEffort)
	assert.True(t, caps.SupportsMaxEffort)
	assert.True(t, caps.SupportsThinking)
	assert.True(t, caps.SupportsCacheControl)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsToolUse)
	assert.True(t, caps.Supports1MContext) // Opus 4.x
	assert.True(t, caps.SupportsStreaming)
	assert.Equal(t, 200000, caps.ContextWindowSize)
}

func TestDetectModelCapabilities_ClaudeSonnet(t *testing.T) {
	caps := agentic.DetectModelCapabilities("claude-sonnet-4-6-20250514", "anthropic")
	assert.True(t, caps.SupportsEffort)
	assert.False(t, caps.SupportsMaxEffort)
	assert.True(t, caps.SupportsThinking)
	assert.True(t, caps.SupportsCacheControl)
	assert.False(t, caps.Supports1MContext) // only Opus
}

func TestDetectModelCapabilities_GPT4o(t *testing.T) {
	caps := agentic.DetectModelCapabilities("gpt-4o", "openai")
	assert.False(t, caps.SupportsEffort)
	assert.False(t, caps.SupportsThinking)
	assert.False(t, caps.SupportsCacheControl)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsToolUse)
	assert.Equal(t, 128000, caps.ContextWindowSize)
}

func TestDetectModelCapabilities_Ollama(t *testing.T) {
	caps := agentic.DetectModelCapabilities("llama3.1:70b", "ollama")
	assert.False(t, caps.SupportsEffort)
	assert.False(t, caps.SupportsThinking)
	assert.False(t, caps.SupportsCacheControl)
	assert.False(t, caps.SupportsVision)
	assert.False(t, caps.SupportsToolUse)
}

func TestBuildRunGates_SonnetEffort(t *testing.T) {
	cfg := agentic.DefaultRunConfig()
	cfg.Model = "claude-sonnet-4-6-20250514"
	high := ai.EffortHigh
	cfg.Effort = &high

	gates := agentic.BuildRunGates(cfg, 0, false)

	assert.True(t, gates.Model.SupportsEffort)
	assert.False(t, gates.Model.SupportsMaxEffort) // only Opus supports max
	assert.NotNil(t, gates.ResolvedEffort)
	assert.Equal(t, ai.EffortHigh, *gates.ResolvedEffort)
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
