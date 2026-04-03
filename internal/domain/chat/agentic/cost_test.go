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
