package agentic

import "github.com/AgentHub-Studio/agenthub-go-commons/ai"

// modelPricing holds per-million-token pricing for known models.
// Prices are in USD per 1M tokens.
type modelPricing struct {
	InputPerM  float64
	OutputPerM float64
}

// knownPricing maps model prefixes to their pricing.
// Source: provider pricing pages as of 2025-05.
var knownPricing = map[string]modelPricing{
	// Anthropic
	"claude-opus-4":     {InputPerM: 15.0, OutputPerM: 75.0},
	"claude-sonnet-4":   {InputPerM: 3.0, OutputPerM: 15.0},
	"claude-haiku-4":    {InputPerM: 0.80, OutputPerM: 4.0},
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
}

// EstimateCostUSD returns the estimated cost in USD for the given usage and model.
// Uses longest prefix match against known pricing. Returns 0 if model is unknown.
func EstimateCostUSD(model string, usage ai.Usage) float64 {
	pricing := lookupPricing(model)
	if pricing == nil {
		return 0
	}
	inputCost := float64(usage.PromptTokens) / 1_000_000 * pricing.InputPerM
	outputCost := float64(usage.CompletionTokens) / 1_000_000 * pricing.OutputPerM
	return inputCost + outputCost
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
