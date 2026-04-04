package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- AccumulatedUsage ---

func TestAccumulatedUsage_AccumulateUsage(t *testing.T) {
	var usage agentic.AccumulatedUsage

	// Turn 1.
	usage.AccumulateUsage(ai.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
		CacheReadTokens:  500,
		CacheCreationTokens: 100,
	})

	assert.Equal(t, 1000, usage.LatestInputTokens, "input should be latest snapshot")
	assert.Equal(t, 200, usage.CumulativeOutputTokens)
	assert.Equal(t, 500, usage.CumulativeCacheReadTokens)
	assert.Equal(t, 100, usage.CumulativeCacheCreationTokens)
	assert.Equal(t, 1200, usage.TotalTokens)

	// Turn 2 — input REPLACED, output ACCUMULATED.
	usage.AccumulateUsage(ai.Usage{
		PromptTokens:     2000,
		CompletionTokens: 300,
		TotalTokens:      2300,
		CacheReadTokens:  800,
		CacheCreationTokens: 0,
	})

	assert.Equal(t, 2000, usage.LatestInputTokens, "input should be replaced")
	assert.Equal(t, 500, usage.CumulativeOutputTokens, "output should accumulate")
	assert.Equal(t, 1300, usage.CumulativeCacheReadTokens, "cache read should accumulate")
	assert.Equal(t, 100, usage.CumulativeCacheCreationTokens, "cache creation should accumulate")
	assert.Equal(t, 3500, usage.TotalTokens)
}

func TestAccumulatedUsage_CacheHitRate(t *testing.T) {
	tests := []struct {
		name     string
		usage    agentic.AccumulatedUsage
		expected float64
	}{
		{
			name:     "zero tokens",
			usage:    agentic.AccumulatedUsage{},
			expected: 0.0,
		},
		{
			name: "all cache hits",
			usage: agentic.AccumulatedUsage{
				LatestInputTokens:         0,
				CumulativeCacheReadTokens: 1000,
			},
			expected: 1.0,
		},
		{
			name: "50% cache hit",
			usage: agentic.AccumulatedUsage{
				LatestInputTokens:         500,
				CumulativeCacheReadTokens: 500,
			},
			expected: 0.5,
		},
		{
			name: "no cache hits",
			usage: agentic.AccumulatedUsage{
				LatestInputTokens:         1000,
				CumulativeCacheReadTokens: 0,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.CacheHitRate()
			assert.InDelta(t, tt.expected, got, 0.001)
		})
	}
}

// --- CacheSafeParamsSnapshot ---

func TestCacheSafeParamsSnapshot_SaveAndGet(t *testing.T) {
	var snap agentic.CacheSafeParamsSnapshot

	assert.Nil(t, snap.Get(), "should be nil initially")

	params := agentic.NewCacheSafeParams("prompt", nil, "anthropic", "claude-sonnet-4", true)
	snap.Save(params)

	got := snap.Get()
	assert.NotNil(t, got)
	assert.True(t, params.Matches(got))
}

func TestCacheSafeParamsSnapshot_Clear(t *testing.T) {
	var snap agentic.CacheSafeParamsSnapshot

	params := agentic.NewCacheSafeParams("prompt", nil, "anthropic", "model", true)
	snap.Save(params)
	assert.NotNil(t, snap.Get())

	snap.Clear()
	assert.Nil(t, snap.Get())
}

// --- extractUserMessage (via ForkedAgentParams) ---

func TestForkedAgentParams_Defaults(t *testing.T) {
	params := agentic.ForkedAgentParams{
		ForkLabel:   "test_fork",
		QuerySource: agentic.SourceMemoryEval,
	}

	assert.Equal(t, "test_fork", params.ForkLabel)
	assert.Equal(t, agentic.SourceMemoryEval, params.QuerySource)
	assert.False(t, params.SkipCacheWrite)
	assert.Nil(t, params.OnMessage)
}

func TestForkedAgentResult_ZeroValue(t *testing.T) {
	var result agentic.ForkedAgentResult

	assert.Equal(t, 0, result.TotalTurns)
	assert.Equal(t, 0.0, result.TotalCost)
	assert.Nil(t, result.Error)
	assert.Equal(t, 0.0, result.TotalUsage.CacheHitRate())
}
