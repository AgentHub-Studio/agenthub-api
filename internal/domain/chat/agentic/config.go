package agentic

import (
	"encoding/json"
	"time"
)

// RunConfig holds tuneable parameters for the agentic Runner loop.
type RunConfig struct {
	// MaxIterations is the safety brake — the loop will stop after this many
	// LLM→tool→LLM iterations even if the model has not emitted a stop signal.
	MaxIterations int `json:"maxIterations"`

	// MaxTokensPerCall caps the max_tokens parameter sent to the LLM per call.
	MaxTokensPerCall int `json:"maxTokensPerCall"`

	// ContextWindowSize is the total context window of the model (e.g. 200000 for Claude).
	ContextWindowSize int `json:"contextWindowSize"`

	// CompactThreshold triggers context compression when usage exceeds this
	// fraction of ContextWindowSize (0.0–1.0). Default 0.75.
	CompactThreshold float64 `json:"compactThreshold"`

	// ToolTimeout is the maximum duration for a single tool execution.
	ToolTimeout time.Duration `json:"toolTimeout"`

	// TotalTimeout is the maximum duration for the entire run.
	TotalTimeout time.Duration `json:"totalTimeout"`

	// ConcurrentReadTools is the max number of read-only tools executed in parallel.
	ConcurrentReadTools int `json:"concurrentReadTools"`

	// StreamBufferSize is the capacity of the RunEvent channel.
	StreamBufferSize int `json:"streamBufferSize"`

	// Provider is the LLM provider name (e.g. "anthropic", "openai", "ollama").
	Provider string `json:"provider"`

	// Model is the LLM model identifier (e.g. "claude-sonnet-4-20250514").
	Model string `json:"model"`

	// Temperature for the LLM call (0.0–2.0).
	Temperature float64 `json:"temperature"`
}

// DefaultRunConfig returns sensible defaults for a Claude-class model.
func DefaultRunConfig() RunConfig {
	return RunConfig{
		MaxIterations:       25,
		MaxTokensPerCall:    4096,
		ContextWindowSize:   200000,
		CompactThreshold:    0.75,
		ToolTimeout:         30 * time.Second,
		TotalTimeout:        5 * time.Minute,
		ConcurrentReadTools: 3,
		StreamBufferSize:    64,
		Provider:            "anthropic",
		Model:               "claude-sonnet-4-20250514",
		Temperature:         0.7,
	}
}

// modelConfig mirrors the JSON shape stored in agent.model_config.
type modelConfig struct {
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	Temperature      *float64 `json:"temperature"`
	MaxTokens        *int     `json:"maxTokens"`
	ContextWindow    *int     `json:"contextWindow"`
	MaxIterations    *int     `json:"maxIterations"`
	CompactThreshold *float64 `json:"compactThreshold"`
}

// RunConfigFromModelConfig creates a RunConfig by overlaying agent-specific
// model_config JSON on top of the defaults.
func RunConfigFromModelConfig(raw json.RawMessage) RunConfig {
	cfg := DefaultRunConfig()
	if len(raw) == 0 {
		return cfg
	}
	var mc modelConfig
	if err := json.Unmarshal(raw, &mc); err != nil {
		return cfg
	}
	if mc.Provider != "" {
		cfg.Provider = mc.Provider
	}
	if mc.Model != "" {
		cfg.Model = mc.Model
	}
	if mc.Temperature != nil {
		cfg.Temperature = *mc.Temperature
	}
	if mc.MaxTokens != nil {
		cfg.MaxTokensPerCall = *mc.MaxTokens
	}
	if mc.ContextWindow != nil {
		cfg.ContextWindowSize = *mc.ContextWindow
	}
	if mc.MaxIterations != nil {
		cfg.MaxIterations = *mc.MaxIterations
	}
	if mc.CompactThreshold != nil {
		cfg.CompactThreshold = *mc.CompactThreshold
	}
	return cfg
}
