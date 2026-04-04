package agentic

import (
	"fmt"
	"math"
)

// LLM model cost calculation and pricing display.
//
// Inspired by Claude Code's modelCost.ts — cost tiers for various
// Claude model families, USD cost calculation from token usage, and
// human-readable pricing format strings. Useful for billing, usage
// dashboards, and cost estimation in multi-model orchestration.

// ModelCosts holds per-million-token pricing for a model.
type ModelCosts struct {
	InputTokens            float64 `json:"inputTokens"`
	OutputTokens           float64 `json:"outputTokens"`
	PromptCacheWriteTokens float64 `json:"promptCacheWriteTokens"`
	PromptCacheReadTokens  float64 `json:"promptCacheReadTokens"`
	WebSearchRequests      float64 `json:"webSearchRequests"`
}

// Standard pricing tiers (USD per million tokens).
var (
	// CostTier3_15 is for Sonnet models: $3 input / $15 output per Mtok.
	CostTier3_15 = ModelCosts{
		InputTokens:            3,
		OutputTokens:           15,
		PromptCacheWriteTokens: 3.75,
		PromptCacheReadTokens:  0.3,
		WebSearchRequests:      0.01,
	}

	// CostTier15_75 is for Opus 4/4.1: $15 input / $75 output per Mtok.
	CostTier15_75 = ModelCosts{
		InputTokens:            15,
		OutputTokens:           75,
		PromptCacheWriteTokens: 18.75,
		PromptCacheReadTokens:  1.5,
		WebSearchRequests:      0.01,
	}

	// CostTier5_25 is for Opus 4.5/4.6: $5 input / $25 output per Mtok.
	CostTier5_25 = ModelCosts{
		InputTokens:            5,
		OutputTokens:           25,
		PromptCacheWriteTokens: 6.25,
		PromptCacheReadTokens:  0.5,
		WebSearchRequests:      0.01,
	}

	// CostTier30_150 is fast-mode Opus 4.6: $30 input / $150 output per Mtok.
	CostTier30_150 = ModelCosts{
		InputTokens:            30,
		OutputTokens:           150,
		PromptCacheWriteTokens: 37.5,
		PromptCacheReadTokens:  3,
		WebSearchRequests:      0.01,
	}

	// CostHaiku35 is for Haiku 3.5: $0.80 input / $4 output per Mtok.
	CostHaiku35 = ModelCosts{
		InputTokens:            0.8,
		OutputTokens:           4,
		PromptCacheWriteTokens: 1,
		PromptCacheReadTokens:  0.08,
		WebSearchRequests:      0.01,
	}

	// CostHaiku45 is for Haiku 4.5: $1 input / $5 output per Mtok.
	CostHaiku45 = ModelCosts{
		InputTokens:            1,
		OutputTokens:           5,
		PromptCacheWriteTokens: 1.25,
		PromptCacheReadTokens:  0.1,
		WebSearchRequests:      0.01,
	}
)

// DefaultUnknownModelCost is used when a model's pricing is not found.
var DefaultUnknownModelCost = CostTier5_25

// KnownModelCosts maps canonical model short names to their cost tiers.
var KnownModelCosts = map[string]ModelCosts{
	"claude-3-5-haiku":   CostHaiku35,
	"claude-haiku-4-5":   CostHaiku45,
	"claude-3-5-sonnet":  CostTier3_15,
	"claude-3-7-sonnet":  CostTier3_15,
	"claude-sonnet-4":    CostTier3_15,
	"claude-sonnet-4-5":  CostTier3_15,
	"claude-sonnet-4-6":  CostTier3_15,
	"claude-opus-4":      CostTier15_75,
	"claude-opus-4-1":    CostTier15_75,
	"claude-opus-4-5":    CostTier5_25,
	"claude-opus-4-6":    CostTier5_25,
}

// CostTokenUsage holds the token counts for cost calculation.
// This is separate from the event-level TokenUsage to allow
// for finer-grained cost inputs (cache write vs read, web search).
type CostTokenUsage struct {
	InputTokens              int64 `json:"inputTokens"`
	OutputTokens             int64 `json:"outputTokens"`
	CacheReadInputTokens     int64 `json:"cacheReadInputTokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cacheCreationInputTokens,omitempty"`
	WebSearchRequests        int64 `json:"webSearchRequests,omitempty"`
}

// TokensToUSDCost calculates the USD cost from token usage and a cost tier.
func TokensToUSDCost(costs ModelCosts, usage CostTokenUsage) float64 {
	mtok := 1_000_000.0
	return float64(usage.InputTokens)/mtok*costs.InputTokens +
		float64(usage.OutputTokens)/mtok*costs.OutputTokens +
		float64(usage.CacheReadInputTokens)/mtok*costs.PromptCacheReadTokens +
		float64(usage.CacheCreationInputTokens)/mtok*costs.PromptCacheWriteTokens +
		float64(usage.WebSearchRequests)*costs.WebSearchRequests
}

// GetModelCosts returns the cost tier for a model. If the model is
// unknown, returns DefaultUnknownModelCost.
func GetModelCosts(model string) ModelCosts {
	if costs, ok := KnownModelCosts[model]; ok {
		return costs
	}
	return DefaultUnknownModelCost
}

// CalculateUSDCost computes the total USD cost for a model and usage.
func CalculateUSDCost(model string, usage CostTokenUsage) float64 {
	costs := GetModelCosts(model)
	return TokensToUSDCost(costs, usage)
}

// formatPrice formats a dollar amount: integers without decimals,
// others with 2 decimal places.
func formatPrice(price float64) string {
	if price == math.Trunc(price) {
		return fmt.Sprintf("$%d", int(price))
	}
	return fmt.Sprintf("$%.2f", price)
}

// FormatModelPricing returns a human-readable pricing string
// like "$3/$15 per Mtok".
func FormatModelPricing(costs ModelCosts) string {
	return fmt.Sprintf("%s/%s per Mtok",
		formatPrice(costs.InputTokens),
		formatPrice(costs.OutputTokens),
	)
}

// GetModelPricingString returns a formatted pricing string for a
// known model, or empty string if the model is not found.
func GetModelPricingString(model string) string {
	costs, ok := KnownModelCosts[model]
	if !ok {
		return ""
	}
	return FormatModelPricing(costs)
}
