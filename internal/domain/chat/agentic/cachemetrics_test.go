package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestPromptCacheHitRatio(t *testing.T) {
	assert.Equal(t, 0.8, agentic.PromptCacheHitRatio(ai.Usage{PromptTokens: 1000, CacheReadTokens: 800}))
	assert.Equal(t, 0.0, agentic.PromptCacheHitRatio(ai.Usage{PromptTokens: 1000}))
	assert.Equal(t, 1.0, agentic.PromptCacheHitRatio(ai.Usage{PromptTokens: 0, CacheReadTokens: 500}))
}
