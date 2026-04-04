package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- TokensToUSDCost ---

func TestTokensToUSDCost_InputOnly(t *testing.T) {
	usage := agentic.CostTokenUsage{InputTokens: 1_000_000}
	cost := agentic.TokensToUSDCost(agentic.CostTier3_15, usage)
	assert.InDelta(t, 3.0, cost, 0.001)
}

func TestTokensToUSDCost_OutputOnly(t *testing.T) {
	usage := agentic.CostTokenUsage{OutputTokens: 1_000_000}
	cost := agentic.TokensToUSDCost(agentic.CostTier3_15, usage)
	assert.InDelta(t, 15.0, cost, 0.001)
}

func TestTokensToUSDCost_Mixed(t *testing.T) {
	usage := agentic.CostTokenUsage{
		InputTokens:  500_000,
		OutputTokens: 200_000,
	}
	// 0.5 * 3 + 0.2 * 15 = 1.5 + 3.0 = 4.5
	cost := agentic.TokensToUSDCost(agentic.CostTier3_15, usage)
	assert.InDelta(t, 4.5, cost, 0.001)
}

func TestTokensToUSDCost_WithCache(t *testing.T) {
	usage := agentic.CostTokenUsage{
		InputTokens:              100_000,
		OutputTokens:             50_000,
		CacheReadInputTokens:     200_000,
		CacheCreationInputTokens: 100_000,
	}
	// 0.1*3 + 0.05*15 + 0.2*0.3 + 0.1*3.75 = 0.3 + 0.75 + 0.06 + 0.375 = 1.485
	cost := agentic.TokensToUSDCost(agentic.CostTier3_15, usage)
	assert.InDelta(t, 1.485, cost, 0.001)
}

func TestTokensToUSDCost_WithWebSearch(t *testing.T) {
	usage := agentic.CostTokenUsage{
		InputTokens:      100_000,
		OutputTokens:     50_000,
		WebSearchRequests: 10,
	}
	// 0.1*3 + 0.05*15 + 10*0.01 = 0.3 + 0.75 + 0.1 = 1.15
	cost := agentic.TokensToUSDCost(agentic.CostTier3_15, usage)
	assert.InDelta(t, 1.15, cost, 0.001)
}

func TestTokensToUSDCost_Zero(t *testing.T) {
	usage := agentic.CostTokenUsage{}
	cost := agentic.TokensToUSDCost(agentic.CostTier3_15, usage)
	assert.Equal(t, 0.0, cost)
}

func TestTokensToUSDCost_OpusTier(t *testing.T) {
	usage := agentic.CostTokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000}
	// 15 + 75 = 90
	cost := agentic.TokensToUSDCost(agentic.CostTier15_75, usage)
	assert.InDelta(t, 90.0, cost, 0.001)
}

// --- GetModelCosts ---

func TestGetModelCosts_KnownSonnet(t *testing.T) {
	costs := agentic.GetModelCosts("claude-sonnet-4-6")
	assert.Equal(t, agentic.CostTier3_15, costs)
}

func TestGetModelCosts_KnownOpus(t *testing.T) {
	costs := agentic.GetModelCosts("claude-opus-4")
	assert.Equal(t, agentic.CostTier15_75, costs)
}

func TestGetModelCosts_KnownHaiku(t *testing.T) {
	costs := agentic.GetModelCosts("claude-haiku-4-5")
	assert.Equal(t, agentic.CostHaiku45, costs)
}

func TestGetModelCosts_Unknown(t *testing.T) {
	costs := agentic.GetModelCosts("gpt-4o")
	assert.Equal(t, agentic.DefaultUnknownModelCost, costs)
}

// --- CalculateUSDCost ---

func TestCalculateUSDCost_KnownModel(t *testing.T) {
	usage := agentic.CostTokenUsage{InputTokens: 1_000_000, OutputTokens: 500_000}
	// 3 + 7.5 = 10.5
	cost := agentic.CalculateUSDCost("claude-sonnet-4-6", usage)
	assert.InDelta(t, 10.5, cost, 0.001)
}

func TestCalculateUSDCost_UnknownModel(t *testing.T) {
	usage := agentic.CostTokenUsage{InputTokens: 1_000_000}
	// Uses default (5_25): 5.0
	cost := agentic.CalculateUSDCost("unknown-model", usage)
	assert.InDelta(t, 5.0, cost, 0.001)
}

// --- FormatModelPricing ---

func TestFormatModelPricing_IntegerPrices(t *testing.T) {
	s := agentic.FormatModelPricing(agentic.CostTier3_15)
	assert.Equal(t, "$3/$15 per Mtok", s)
}

func TestFormatModelPricing_DecimalPrices(t *testing.T) {
	s := agentic.FormatModelPricing(agentic.CostHaiku35)
	assert.Equal(t, "$0.80/$4 per Mtok", s)
}

func TestFormatModelPricing_HighTier(t *testing.T) {
	s := agentic.FormatModelPricing(agentic.CostTier15_75)
	assert.Equal(t, "$15/$75 per Mtok", s)
}

// --- GetModelPricingString ---

func TestGetModelPricingString_Known(t *testing.T) {
	s := agentic.GetModelPricingString("claude-sonnet-4")
	assert.Equal(t, "$3/$15 per Mtok", s)
}

func TestGetModelPricingString_Unknown(t *testing.T) {
	s := agentic.GetModelPricingString("unknown-model")
	assert.Equal(t, "", s)
}

// --- Cost tier values ---

func TestCostTier_FastMode(t *testing.T) {
	assert.Equal(t, float64(30), agentic.CostTier30_150.InputTokens)
	assert.Equal(t, float64(150), agentic.CostTier30_150.OutputTokens)
}

func TestCostTier_AllTiersHaveWebSearch(t *testing.T) {
	tiers := []agentic.ModelCosts{
		agentic.CostTier3_15,
		agentic.CostTier15_75,
		agentic.CostTier5_25,
		agentic.CostTier30_150,
		agentic.CostHaiku35,
		agentic.CostHaiku45,
	}
	for _, tier := range tiers {
		assert.Equal(t, 0.01, tier.WebSearchRequests)
	}
}

func TestKnownModelCosts_AllModels(t *testing.T) {
	expected := []string{
		"claude-3-5-haiku", "claude-haiku-4-5",
		"claude-3-5-sonnet", "claude-3-7-sonnet",
		"claude-sonnet-4", "claude-sonnet-4-5", "claude-sonnet-4-6",
		"claude-opus-4", "claude-opus-4-1", "claude-opus-4-5", "claude-opus-4-6",
	}
	for _, model := range expected {
		_, ok := agentic.KnownModelCosts[model]
		assert.True(t, ok, "missing cost for %s", model)
	}
}
