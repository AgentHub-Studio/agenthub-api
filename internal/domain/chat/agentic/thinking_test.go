package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- ResolveThinkingConfig ---

func TestResolveThinking_NonCapableModel(t *testing.T) {
	caps := agentic.ModelCapabilities{SupportsThinking: false}
	result := agentic.ResolveThinkingConfig(caps, nil, agentic.SourceMainLoop)
	assert.Nil(t, result)
}

func TestResolveThinking_BackgroundSource_Disabled(t *testing.T) {
	caps := agentic.ModelCapabilities{SupportsThinking: true}
	result := agentic.ResolveThinkingConfig(caps, nil, agentic.SourceCompact)
	assert.NotNil(t, result)
	assert.Equal(t, ai.ThinkingDisabled, result.Type)
}

func TestResolveThinking_MemoryEvalSource_Disabled(t *testing.T) {
	caps := agentic.ModelCapabilities{SupportsThinking: true}
	result := agentic.ResolveThinkingConfig(caps, nil, agentic.SourceMemoryEval)
	assert.NotNil(t, result)
	assert.Equal(t, ai.ThinkingDisabled, result.Type)
}

func TestResolveThinking_UserConfig_Honored(t *testing.T) {
	caps := agentic.ModelCapabilities{SupportsThinking: true}
	userCfg := &ai.ThinkingConfig{Type: ai.ThinkingEnabled, BudgetTokens: 8000}
	result := agentic.ResolveThinkingConfig(caps, userCfg, agentic.SourceMainLoop)
	assert.Equal(t, ai.ThinkingEnabled, result.Type)
	assert.Equal(t, 8000, result.BudgetTokens)
}

func TestResolveThinking_DefaultAdaptive(t *testing.T) {
	caps := agentic.ModelCapabilities{SupportsThinking: true}
	result := agentic.ResolveThinkingConfig(caps, nil, agentic.SourceMainLoop)
	assert.NotNil(t, result)
	assert.Equal(t, ai.ThinkingAdaptive, result.Type)
}

func TestResolveThinking_Subtask_Enabled(t *testing.T) {
	caps := agentic.ModelCapabilities{SupportsThinking: true}
	result := agentic.ResolveThinkingConfig(caps, nil, agentic.SourceSubtask)
	assert.NotNil(t, result)
	assert.Equal(t, ai.ThinkingAdaptive, result.Type)
}

// --- ModelSupportsAdaptiveThinking ---

func TestModelSupportsAdaptive_Opus4(t *testing.T) {
	assert.True(t, agentic.ModelSupportsAdaptiveThinking("claude-opus-4-6-20250514"))
}

func TestModelSupportsAdaptive_Sonnet4(t *testing.T) {
	assert.True(t, agentic.ModelSupportsAdaptiveThinking("claude-sonnet-4-20250514"))
}

func TestModelSupportsAdaptive_Haiku(t *testing.T) {
	assert.False(t, agentic.ModelSupportsAdaptiveThinking("claude-haiku-4-5-20251001"))
}

func TestModelSupportsAdaptive_GPT(t *testing.T) {
	assert.False(t, agentic.ModelSupportsAdaptiveThinking("gpt-4o"))
}

// --- GetMaxThinkingTokens ---

func TestGetMaxThinkingTokens_Opus(t *testing.T) {
	budget := agentic.GetMaxThinkingTokens("claude-opus-4-6", 0)
	assert.Equal(t, 32000, budget)
}

func TestGetMaxThinkingTokens_Sonnet(t *testing.T) {
	budget := agentic.GetMaxThinkingTokens("claude-sonnet-4-20250514", 0)
	assert.Equal(t, 16000, budget)
}

func TestGetMaxThinkingTokens_Default(t *testing.T) {
	budget := agentic.GetMaxThinkingTokens("claude-3-5-sonnet-20241022", 0)
	assert.Equal(t, 10000, budget)
}

func TestGetMaxThinkingTokens_CappedByMaxOutput(t *testing.T) {
	// Budget of 32000 should be capped to maxOutput - 1.
	budget := agentic.GetMaxThinkingTokens("claude-opus-4-6", 8000)
	assert.Equal(t, 7999, budget)
}

func TestGetMaxThinkingTokens_MinFloor(t *testing.T) {
	// Very low maxOutput should still give at least 1024.
	budget := agentic.GetMaxThinkingTokens("claude-opus-4-6", 500)
	assert.Equal(t, 1024, budget)
}

// --- EstimateThinkingTokens ---

func TestEstimateThinkingTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, agentic.EstimateThinkingTokens(""))
}

func TestEstimateThinkingTokens_ShortText(t *testing.T) {
	// "Let me think about this task." = 29 chars → 29/3 + 3 = 12
	tokens := agentic.EstimateThinkingTokens("Let me think about this task.")
	assert.Equal(t, 12, tokens)
}

// --- ThinkingAwareChatOptions ---

func TestThinkingAwareOpts_NilThinking(t *testing.T) {
	opts := ai.ChatOptions{Model: "test", Temperature: 0.7, MaxTokens: 4096}
	result := agentic.ThinkingAwareChatOptions(opts, nil)
	assert.Equal(t, 0.7, result.Temperature) // unchanged
	assert.Nil(t, result.Thinking)
}

func TestThinkingAwareOpts_DisabledThinking(t *testing.T) {
	opts := ai.ChatOptions{Model: "test", Temperature: 0.7}
	result := agentic.ThinkingAwareChatOptions(opts, &ai.ThinkingConfig{Type: ai.ThinkingDisabled})
	assert.Equal(t, 0.7, result.Temperature) // unchanged
}

func TestThinkingAwareOpts_AdaptiveSetsTempTo1(t *testing.T) {
	opts := ai.ChatOptions{Model: "test", Temperature: 0.7}
	result := agentic.ThinkingAwareChatOptions(opts, &ai.ThinkingConfig{Type: ai.ThinkingAdaptive})
	assert.Equal(t, 1.0, result.Temperature)
	assert.NotNil(t, result.Thinking)
}

func TestThinkingAwareOpts_EnabledAdjustsMaxTokens(t *testing.T) {
	opts := ai.ChatOptions{Model: "test", MaxTokens: 4096}
	thinking := &ai.ThinkingConfig{Type: ai.ThinkingEnabled, BudgetTokens: 16000}
	result := agentic.ThinkingAwareChatOptions(opts, thinking)
	assert.Equal(t, 1.0, result.Temperature)
	assert.Equal(t, 17024, result.MaxTokens) // 16000 + 1024
}

func TestThinkingAwareOpts_EnabledMaxTokensAlreadySufficient(t *testing.T) {
	opts := ai.ChatOptions{Model: "test", MaxTokens: 32000}
	thinking := &ai.ThinkingConfig{Type: ai.ThinkingEnabled, BudgetTokens: 16000}
	result := agentic.ThinkingAwareChatOptions(opts, thinking)
	assert.Equal(t, 32000, result.MaxTokens) // already sufficient
}
