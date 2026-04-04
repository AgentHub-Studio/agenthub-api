package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestEstimateCostUSD_ExactMatch(t *testing.T) {
	usage := ai.Usage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000}
	cost := agentic.EstimateCostUSD("claude-sonnet-4", usage)
	// 3.0 + 15.0 = 18.0
	assert.InDelta(t, 18.0, cost, 0.001)
}

func TestEstimateCostUSD_PrefixMatch(t *testing.T) {
	usage := ai.Usage{PromptTokens: 500_000, CompletionTokens: 200_000}
	cost := agentic.EstimateCostUSD("claude-sonnet-4-20250514", usage)
	// (500000/1M)*3.0 + (200000/1M)*15.0 = 1.5 + 3.0 = 4.5
	assert.InDelta(t, 4.5, cost, 0.001)
}

func TestEstimateCostUSD_UnknownModel(t *testing.T) {
	usage := ai.Usage{PromptTokens: 1000, CompletionTokens: 1000}
	cost := agentic.EstimateCostUSD("unknown-model-xyz", usage)
	assert.Equal(t, 0.0, cost)
}

func TestEstimateCostUSD_ZeroTokens(t *testing.T) {
	cost := agentic.EstimateCostUSD("claude-opus-4", ai.Usage{})
	assert.Equal(t, 0.0, cost)
}

func TestEstimateCostUSD_OpenAIModel(t *testing.T) {
	usage := ai.Usage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000}
	cost := agentic.EstimateCostUSD("gpt-4o", usage)
	// 2.50 + 10.0 = 12.50
	assert.InDelta(t, 12.5, cost, 0.001)
}

func TestEstimateCostUSD_LongestPrefixWins(t *testing.T) {
	// "claude-3-haiku" should match over "claude-3" prefix.
	usage := ai.Usage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000}
	cost := agentic.EstimateCostUSD("claude-3-haiku-20240307", usage)
	// 0.25 + 1.25 = 1.50 (haiku pricing, not opus)
	assert.InDelta(t, 1.50, cost, 0.001)
}

func TestEstimateCostUSD_SmallUsage(t *testing.T) {
	usage := ai.Usage{PromptTokens: 1000, CompletionTokens: 500}
	cost := agentic.EstimateCostUSD("claude-sonnet-4", usage)
	// (1000/1M)*3.0 + (500/1M)*15.0 = 0.003 + 0.0075 = 0.0105
	assert.InDelta(t, 0.0105, cost, 0.0001)
}

func TestEstimateCostUSD_WithCacheTokens(t *testing.T) {
	// Claude Sonnet: input $3/M, output $15/M
	// Cache reads: 10% of input = $0.30/M
	// Cache creation: 125% of input = $3.75/M
	usage := ai.Usage{
		PromptTokens:        1_000_000, // total prompt tokens (includes cache)
		CompletionTokens:    100_000,
		CacheReadTokens:     800_000,   // 80% from cache reads
		CacheCreationTokens: 100_000,   // 10% cache creation
	}
	cost := agentic.EstimateCostUSD("claude-sonnet-4", usage)
	// Regular input: 1M - 800K - 100K = 100K → (100K/1M)*3.0 = 0.30
	// Cache read: (800K/1M)*3.0*0.10 = 0.24
	// Cache creation: (100K/1M)*3.0*1.25 = 0.375
	// Output: (100K/1M)*15.0 = 1.50
	// Total: 0.30 + 0.24 + 0.375 + 1.50 = 2.415
	assert.InDelta(t, 2.415, cost, 0.001)
}

func TestEstimateCostUSD_CacheDiscount_VsNonCache(t *testing.T) {
	// Same total tokens, but with cache should be cheaper.
	noCacheUsage := ai.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}
	noCacheCost := agentic.EstimateCostUSD("claude-opus-4", noCacheUsage)

	cachedUsage := ai.Usage{
		PromptTokens:    1_000_000,
		CacheReadTokens: 900_000,
	}
	cachedCost := agentic.EstimateCostUSD("claude-opus-4", cachedUsage)

	// Cached should be significantly cheaper.
	assert.True(t, cachedCost < noCacheCost, "cached cost %f should be less than non-cached %f", cachedCost, noCacheCost)
}
