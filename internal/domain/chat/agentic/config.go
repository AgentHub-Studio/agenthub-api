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

	// MaxBudgetUSD is the maximum total cost in USD for the entire run.
	// Zero means no budget limit.
	MaxBudgetUSD float64 `json:"maxBudgetUsd"`

	// MaxToolResultChars is the maximum character length for a single tool result.
	// Results exceeding this are truncated with a note. Zero means no truncation.
	MaxToolResultChars int `json:"maxToolResultChars"`

	// RetryMaxAttempts is the maximum number of retries for transient LLM errors.
	RetryMaxAttempts int `json:"retryMaxAttempts"`

	// MaxDepth is the maximum recursion depth for sub-agent spawning.
	// The root agent runs at depth 0. Default 3.
	MaxDepth int `json:"maxDepth"`

	// Provider is the LLM provider name (e.g. "anthropic", "openai", "ollama").
	Provider string `json:"provider"`

	// Model is the LLM model identifier (e.g. "claude-sonnet-4-20250514").
	Model string `json:"model"`

	// Temperature for the LLM call (0.0–2.0).
	Temperature float64 `json:"temperature"`

	// ModelFallbacks is an ordered list of fallback models to try when the
	// primary model fails with transient errors (rate limit, overload, timeout).
	ModelFallbacks []string `json:"modelFallbacks,omitempty"`

	// FallbackOnRateLimit enables fallback on 429/529 errors. Default true.
	FallbackOnRateLimit *bool `json:"fallbackOnRateLimit,omitempty"`
	// FallbackOnOverload enables fallback on 502/503 errors. Default true.
	FallbackOnOverload *bool `json:"fallbackOnOverload,omitempty"`
	// FallbackOnTimeout enables fallback on timeout errors. Default true.
	FallbackOnTimeout *bool `json:"fallbackOnTimeout,omitempty"`

	// ToolCacheCapacity is the max number of entries in the per-run tool result
	// LRU cache. Cacheable tools (read-only, idempotent) have their results cached
	// to avoid redundant re-execution. Default 64. Set to 0 to disable caching.
	ToolCacheCapacity int `json:"toolCacheCapacity"`
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
		MaxBudgetUSD:        0, // no limit by default
		MaxToolResultChars:  50000,
		RetryMaxAttempts:    3,
		MaxDepth:            3,
		Provider:            "anthropic",
		Model:               "claude-sonnet-4-20250514",
		Temperature:       0.7,
		ToolCacheCapacity: 64,
	}
}

// modelConfig mirrors the JSON shape stored in agent.model_config.
type modelConfig struct {
	Provider           string   `json:"provider"`
	Model              string   `json:"model"`
	Temperature        *float64 `json:"temperature"`
	MaxTokens          *int     `json:"maxTokens"`
	ContextWindow      *int     `json:"contextWindow"`
	MaxIterations      *int     `json:"maxIterations"`
	CompactThreshold   *float64 `json:"compactThreshold"`
	MaxBudgetUSD       *float64 `json:"maxBudgetUsd"`
	MaxToolResultChars *int     `json:"maxToolResultChars"`
	RetryMaxAttempts   *int     `json:"retryMaxAttempts"`
	MaxDepth           *int     `json:"maxDepth"`
	ModelFallbacks     []string `json:"modelFallbacks,omitempty"`
	FallbackOnRateLimit *bool   `json:"fallbackOnRateLimit,omitempty"`
	FallbackOnOverload  *bool   `json:"fallbackOnOverload,omitempty"`
	FallbackOnTimeout   *bool   `json:"fallbackOnTimeout,omitempty"`
	ToolCacheCapacity   *int    `json:"toolCacheCapacity,omitempty"`
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
	if mc.MaxBudgetUSD != nil {
		cfg.MaxBudgetUSD = *mc.MaxBudgetUSD
	}
	if mc.MaxToolResultChars != nil {
		cfg.MaxToolResultChars = *mc.MaxToolResultChars
	}
	if mc.RetryMaxAttempts != nil {
		cfg.RetryMaxAttempts = *mc.RetryMaxAttempts
	}
	if mc.MaxDepth != nil {
		cfg.MaxDepth = *mc.MaxDepth
	}
	if len(mc.ModelFallbacks) > 0 {
		cfg.ModelFallbacks = mc.ModelFallbacks
	}
	if mc.FallbackOnRateLimit != nil {
		cfg.FallbackOnRateLimit = mc.FallbackOnRateLimit
	}
	if mc.FallbackOnOverload != nil {
		cfg.FallbackOnOverload = mc.FallbackOnOverload
	}
	if mc.FallbackOnTimeout != nil {
		cfg.FallbackOnTimeout = mc.FallbackOnTimeout
	}
	if mc.ToolCacheCapacity != nil {
		cfg.ToolCacheCapacity = *mc.ToolCacheCapacity
	}
	return cfg
}

// IsFallbackOnRateLimit returns whether fallback is enabled for rate limit errors.
func (c RunConfig) IsFallbackOnRateLimit() bool {
	if c.FallbackOnRateLimit == nil {
		return true // default enabled
	}
	return *c.FallbackOnRateLimit
}

// IsFallbackOnOverload returns whether fallback is enabled for overload errors.
func (c RunConfig) IsFallbackOnOverload() bool {
	if c.FallbackOnOverload == nil {
		return true
	}
	return *c.FallbackOnOverload
}

// IsFallbackOnTimeout returns whether fallback is enabled for timeout errors.
func (c RunConfig) IsFallbackOnTimeout() bool {
	if c.FallbackOnTimeout == nil {
		return true
	}
	return *c.FallbackOnTimeout
}
