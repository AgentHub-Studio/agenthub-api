package agentic

import (
	"encoding/json"
	"math"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// Post-compact re-injection budget constants. After compaction, tools, skills,
// and file context are re-announced to the LLM within a token budget to prevent
// the model from losing awareness of its capabilities.
// Inspired by Claude Code's POST_COMPACT_* constants in services/compact/compact.ts.
const (
	// PostCompactTokenBudget is the total token budget for all re-injected content.
	PostCompactTokenBudget = 50_000
	// PostCompactMaxTokensPerSkill caps the re-injected content per skill description.
	PostCompactMaxTokensPerSkill = 5_000
	// PostCompactSkillsTokenBudget is the aggregate budget for all re-injected skills.
	PostCompactSkillsTokenBudget = 25_000
	// PostCompactMaxFilesToRestore is the max number of recently-read files re-injected.
	PostCompactMaxFilesToRestore = 5
	// PostCompactMaxTokensPerFile caps the re-injected content per file.
	PostCompactMaxTokensPerFile = 5_000
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

	// LLMCallTimeout is the maximum duration for a single LLM call (including streaming).
	// P-C102-1: prevents a stalled provider from blocking the goroutine indefinitely.
	// Default: 5 minutes. Set to 0 to disable (not recommended for production).
	LLMCallTimeout time.Duration `json:"llmCallTimeout"`

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

	// MaxToolResultsPerTurnChars is the aggregate character budget for all tool results
	// in a single turn. Subsequent results are truncated to "[budget exceeded]" once
	// the aggregate exceeds this limit. Zero means no aggregate limit.
	MaxToolResultsPerTurnChars int `json:"maxToolResultsPerTurnChars,omitempty"`

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

	// MaxTokensPerTurn caps total tokens (prompt+completion) for a single turn.
	// Zero means no per-turn limit (only the global MaxBudgetUSD applies).
	MaxTokensPerTurn int `json:"maxTokensPerTurn,omitempty"`

	// BudgetEscalation enables automatic budget increase on subsequent turns.
	// When true, the per-turn limit scales as: base * EscalationFactor^turnIndex.
	BudgetEscalation bool `json:"budgetEscalation,omitempty"`

	// EscalationFactor is the multiplier applied per turn when BudgetEscalation is true.
	// Default 1.5.
	EscalationFactor float64 `json:"escalationFactor,omitempty"`

	// StallCheckInterval is how often the stall detector checks each tool (default 15s).
	StallCheckInterval time.Duration `json:"stallCheckInterval"`

	// StallThreshold is the duration without output after which a tool is considered
	// stalled (default 45s). Set to 0 to disable stall detection.
	StallThreshold time.Duration `json:"stallThreshold"`

	// DenialEscalationThreshold is the number of consecutive denials of the same
	// tool before the Runner injects an escalation hint into the LLM context.
	// Default 3. Set to 0 to disable escalation.
	DenialEscalationThreshold int `json:"denialEscalationThreshold"`

	// ModelFallbacks is an ordered list of fallback models to try when the
	// primary model fails with transient errors (rate limit, overload, timeout).
	ModelFallbacks []string `json:"modelFallbacks,omitempty"`

	// FallbackOnRateLimit enables fallback on 429/529 errors. Default true.
	FallbackOnRateLimit *bool `json:"fallbackOnRateLimit,omitempty"`
	// FallbackOnOverload enables fallback on 502/503 errors. Default true.
	FallbackOnOverload *bool `json:"fallbackOnOverload,omitempty"`
	// FallbackOnTimeout enables fallback on timeout errors. Default true.
	FallbackOnTimeout *bool `json:"fallbackOnTimeout,omitempty"`

	// Thinking configures extended thinking / chain-of-thought for the LLM.
	// Nil means disabled. Inspired by Claude Code's ThinkingConfig.
	Thinking *ai.ThinkingConfig `json:"thinking,omitempty"`

	// Effort controls reasoning effort level for the LLM.
	// Nil means no effort parameter is sent (API defaults to "high").
	// "max" is only valid for Opus 4.6; it's downgraded to "high" for other models.
	// Inspired by Claude Code's effort.ts.
	Effort *ai.EffortLevel `json:"effort,omitempty"`

	// OutputTokenBudget is the target number of output tokens for budget-based
	// continuation. When set, the runner nudges the LLM to keep working until
	// the budget is reached or diminishing returns are detected.
	// Zero means no output budget (normal stop behavior).
	// Inspired by Claude Code's tokenBudget.ts.
	OutputTokenBudget int `json:"outputTokenBudget,omitempty"`

	// ToolResultLimits maps tool names to per-tool max result char limits.
	// Tools not listed here use MaxToolResultChars as default.
	// Inspired by Claude Code's per-tool maxResultSizeChars.
	ToolResultLimits map[string]int `json:"toolResultLimits,omitempty"`

	// ToolCacheCapacity is the max number of entries in the per-run tool result
	// LRU cache. Cacheable tools (read-only, idempotent) have their results cached
	// to avoid redundant re-execution. Default 64. Set to 0 to disable caching.
	ToolCacheCapacity int `json:"toolCacheCapacity"`

	// MaxHistoryMessages is the maximum number of messages loaded from history.
	// When the session has more messages than this limit, only the most recent N
	// are used (sliding window). Default 200. Zero means no limit.
	// DX-01-M (ACT-F3-06): prevents enormous prompts for long-running sessions.
	MaxHistoryMessages int `json:"maxHistoryMessages"`
}

// RunGates captures immutable, pre-computed boolean flags and derived values
// snapshotted once at the start of a run. This prevents re-evaluating conditions
// on every loop iteration and ensures consistent behavior throughout a single run
// even if external state (feature flags, config) changes.
//
// Inspired by Claude Code's QueryConfig (query/config.ts):
// "Immutable values snapshotted once at query() entry. Separating these from
// the per-iteration State struct makes future step() extraction tractable."
type RunGates struct {
	// Model contains the detected capabilities for the selected model.
	Model ModelCapabilities
	// ResolvedEffort is the pre-resolved effort level for this run (nil if unsupported).
	ResolvedEffort *ai.EffortLevel
	// CacheControl indicates whether to send cache_control markers to the provider.
	CacheControl bool
	// HasBudgetLimit indicates whether a cost budget is in effect.
	HasBudgetLimit bool
	// HasOutputTokenBudget indicates whether output token budget tracking is enabled.
	HasOutputTokenBudget bool
	// CoordinatorMode indicates whether sub-agent coordination is enabled for this run.
	CoordinatorMode bool
	// HasToolResultLimits indicates whether per-tool result limits are configured.
	HasToolResultLimits bool
	// HasAggregateResultLimit indicates whether per-turn aggregate result limit is set.
	HasAggregateResultLimit bool
	// ContextWindowTokens is the resolved context window size in tokens.
	ContextWindowTokens int
}

// BuildRunGates creates an immutable RunGates snapshot from the RunConfig and run input.
func BuildRunGates(cfg RunConfig, currentDepth int, hasSubtaskExec bool) RunGates {
	caps := DetectModelCapabilities(cfg.Model, cfg.Provider)
	return RunGates{
		Model:                   caps,
		ResolvedEffort:          ResolveEffortLevel(cfg),
		CacheControl:            caps.SupportsCacheControl,
		HasBudgetLimit:          cfg.MaxBudgetUSD > 0,
		HasOutputTokenBudget:    cfg.OutputTokenBudget > 0,
		CoordinatorMode:         hasSubtaskExec && currentDepth < cfg.MaxDepth,
		HasToolResultLimits:     len(cfg.ToolResultLimits) > 0,
		HasAggregateResultLimit: cfg.MaxToolResultsPerTurnChars > 0,
		ContextWindowTokens:     caps.ContextWindowSize,
	}
}

// DefaultRunConfig returns sensible defaults for a Claude-class model.
func DefaultRunConfig() RunConfig {
	return RunConfig{
		MaxIterations:       25,
		MaxTokensPerCall:    4096,
		ContextWindowSize:   200000,
		CompactThreshold:    0.75,
		ToolTimeout:         30 * time.Second,
		TotalTimeout:        5 * time.Minute, // agents can override via totalTimeoutSeconds in model config
		LLMCallTimeout:      5 * time.Minute,  // P-C102-1: per-call timeout; configurable via LLM_CALL_TIMEOUT_SECS
		ConcurrentReadTools: 3,
		StreamBufferSize:    64,
		MaxBudgetUSD:        0, // no limit by default
		MaxToolResultChars:         50000,
		MaxToolResultsPerTurnChars: 200000,
		RetryMaxAttempts:    5,
		MaxDepth:            3,
		Provider:            "anthropic",
		Model:               "claude-sonnet-4-20250514",
		Temperature:                0.7,
		StallCheckInterval:         15 * time.Second,
		StallThreshold:             45 * time.Second,
		ToolCacheCapacity:          64,
		DenialEscalationThreshold: 3,
		MaxHistoryMessages:        200,
	}
}

// modelConfig mirrors the JSON shape stored in agent.model_config.
type modelConfig struct {
	Provider            string             `json:"provider"`
	Model               string             `json:"model"`
	Temperature         *float64           `json:"temperature"`
	MaxTokens           *int               `json:"maxTokens"`
	ContextWindow       *int               `json:"contextWindow"`
	MaxIterations       *int               `json:"maxIterations"`
	CompactThreshold    *float64           `json:"compactThreshold"`
	MaxBudgetUSD        *float64           `json:"maxBudgetUsd"`
	MaxToolResultChars  *int               `json:"maxToolResultChars"`
	RetryMaxAttempts    *int               `json:"retryMaxAttempts"`
	MaxDepth            *int               `json:"maxDepth"`
	MaxTokensPerTurn    *int               `json:"maxTokensPerTurn,omitempty"`
	BudgetEscalation    *bool              `json:"budgetEscalation,omitempty"`
	EscalationFactor    *float64           `json:"escalationFactor,omitempty"`
	ModelFallbacks      []string           `json:"modelFallbacks,omitempty"`
	FallbackOnRateLimit *bool              `json:"fallbackOnRateLimit,omitempty"`
	FallbackOnOverload  *bool              `json:"fallbackOnOverload,omitempty"`
	FallbackOnTimeout   *bool              `json:"fallbackOnTimeout,omitempty"`
	Thinking            *ai.ThinkingConfig `json:"thinking,omitempty"`
	Effort              *ai.EffortLevel    `json:"effort,omitempty"`
	ToolCacheCapacity   *int               `json:"toolCacheCapacity,omitempty"`
	// TotalTimeoutSeconds overrides the default 5-minute run timeout.
	// Useful for large local models that need more time per inference pass.
	TotalTimeoutSeconds *int `json:"totalTimeoutSeconds,omitempty"`
	// LLMCallTimeoutSeconds overrides the per-LLM-call timeout (P-C102-1).
	// Default: 300 (5 minutes). Set to 0 to disable.
	LLMCallTimeoutSeconds *int `json:"llmCallTimeoutSeconds,omitempty"`
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
	if mc.MaxTokensPerTurn != nil {
		cfg.MaxTokensPerTurn = *mc.MaxTokensPerTurn
	}
	if mc.BudgetEscalation != nil {
		cfg.BudgetEscalation = *mc.BudgetEscalation
	}
	if mc.EscalationFactor != nil {
		cfg.EscalationFactor = *mc.EscalationFactor
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
	if mc.Thinking != nil {
		cfg.Thinking = mc.Thinking
	}
	if mc.Effort != nil {
		cfg.Effort = mc.Effort
	}
	if mc.ToolCacheCapacity != nil {
		cfg.ToolCacheCapacity = *mc.ToolCacheCapacity
	}
	if mc.TotalTimeoutSeconds != nil && *mc.TotalTimeoutSeconds > 0 {
		cfg.TotalTimeout = time.Duration(*mc.TotalTimeoutSeconds) * time.Second
	}
	if mc.LLMCallTimeoutSeconds != nil && *mc.LLMCallTimeoutSeconds >= 0 {
		cfg.LLMCallTimeout = time.Duration(*mc.LLMCallTimeoutSeconds) * time.Second
	}
	return cfg
}

// EffectiveTurnBudget calculates the token budget for a given turn.
// If MaxTokensPerTurn is 0, returns 0 (no limit).
// When BudgetEscalation is enabled, scales the base budget by EscalationFactor^turnIndex.
func (c RunConfig) EffectiveTurnBudget(turnIndex int) int {
	if c.MaxTokensPerTurn <= 0 {
		return 0
	}
	if !c.BudgetEscalation || turnIndex <= 0 {
		return c.MaxTokensPerTurn
	}
	factor := c.EscalationFactor
	if factor <= 0 {
		factor = 1.5
	}
	// base * factor^turnIndex
	escalated := float64(c.MaxTokensPerTurn) * math.Pow(factor, float64(turnIndex))
	return int(escalated)
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
