package agentic

import "github.com/AgentHub-Studio/agenthub-go-commons/ai"

// modelPricing holds per-million-token pricing for known models.
// Prices are in USD per 1M tokens.
type modelPricing struct {
	InputPerM  float64
	OutputPerM float64
}

// knownPricing maps model prefixes to their pricing.
// Source: provider pricing pages as of 2026-04.
// Prices per million tokens (input/output).
var knownPricing = map[string]modelPricing{
	// Anthropic — latest pricing from platform.claude.com/docs/en/about-claude/pricing
	"claude-opus-4-6":   {InputPerM: 5.0, OutputPerM: 25.0},
	"claude-opus-4-5":   {InputPerM: 5.0, OutputPerM: 25.0},
	"claude-opus-4-1":   {InputPerM: 15.0, OutputPerM: 75.0},
	"claude-opus-4":     {InputPerM: 15.0, OutputPerM: 75.0},
	"claude-sonnet-4-6": {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-sonnet-4-5": {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-sonnet-4":   {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-haiku-4-5":  {InputPerM: 1.0, OutputPerM: 5.0},
	"claude-haiku-4":    {InputPerM: 0.80, OutputPerM: 4.0},
	"claude-3.5-haiku":  {InputPerM: 0.80, OutputPerM: 4.0},
	"claude-3.5-sonnet": {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-3-opus":     {InputPerM: 15.0, OutputPerM: 75.0},
	"claude-3-sonnet":   {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-3-haiku":    {InputPerM: 0.25, OutputPerM: 1.25},
	// OpenAI
	"gpt-4o":        {InputPerM: 2.50, OutputPerM: 10.0},
	"gpt-4-turbo":   {InputPerM: 10.0, OutputPerM: 30.0},
	"gpt-4":         {InputPerM: 30.0, OutputPerM: 60.0},
	"gpt-3.5-turbo": {InputPerM: 0.50, OutputPerM: 1.50},
	"o1":            {InputPerM: 15.0, OutputPerM: 60.0},
	"o3":            {InputPerM: 10.0, OutputPerM: 40.0},
	// OpenRouter — OSS/custom models routed via OpenRouter
	"openai/gpt-oss-120b": {InputPerM: 0.50, OutputPerM: 1.50}, // approximate; update when pricing is published
	"openai/gpt-4o":       {InputPerM: 2.50, OutputPerM: 10.0},
	"openai/gpt-4-turbo":  {InputPerM: 10.0, OutputPerM: 30.0},
	"openai/gpt-4":        {InputPerM: 30.0, OutputPerM: 60.0},
	"openai/gpt-3.5":      {InputPerM: 0.50, OutputPerM: 1.50},
	"meta-llama/":         {InputPerM: 0.10, OutputPerM: 0.30},
	"mistralai/":          {InputPerM: 0.20, OutputPerM: 0.60},
	"google/gemini":       {InputPerM: 0.35, OutputPerM: 1.05},
}

// EstimateCostUSD returns the estimated cost in USD for the given usage and model.
// Uses longest prefix match against known pricing. Returns 0 if model is unknown.
// Cache read tokens are billed at 10% of the input price (90% discount).
// Cache creation tokens are billed at 125% of the input price (25% surcharge).
func EstimateCostUSD(model string, usage ai.Usage) float64 {
	pricing := lookupPricing(model)
	if pricing == nil {
		return 0
	}

	// Regular (non-cached) input tokens = prompt tokens minus cache tokens.
	regularInput := usage.PromptTokens - usage.CacheReadTokens - usage.CacheCreationTokens
	if regularInput < 0 {
		regularInput = 0
	}

	inputCost := float64(regularInput) / 1_000_000 * pricing.InputPerM
	cacheReadCost := float64(usage.CacheReadTokens) / 1_000_000 * pricing.InputPerM * 0.10
	cacheCreateCost := float64(usage.CacheCreationTokens) / 1_000_000 * pricing.InputPerM * 1.25
	outputCost := float64(usage.CompletionTokens) / 1_000_000 * pricing.OutputPerM
	return inputCost + cacheReadCost + cacheCreateCost + outputCost
}

// lookupPricing finds pricing by longest prefix match.
func lookupPricing(model string) *modelPricing {
	// Exact match first.
	if p, ok := knownPricing[model]; ok {
		return &p
	}
	// Longest prefix match.
	var best *modelPricing
	bestLen := 0
	for prefix, p := range knownPricing {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix && len(prefix) > bestLen {
			p := p // copy
			best = &p
			bestLen = len(prefix)
		}
	}
	return best
}
