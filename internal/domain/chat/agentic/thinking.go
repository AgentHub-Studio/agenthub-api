package agentic

import (
	"strings"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// ResolveThinkingConfig determines the thinking configuration for an LLM call
// based on model capabilities, user configuration, and query source.
//
// Rules:
//   - Thinking is disabled for non-capable models (returns nil)
//   - Background sources (compact, memory_eval) disable thinking for speed
//   - If user configured thinking explicitly, use that
//   - Otherwise, use adaptive thinking (model decides)
//
// Inspired by Claude Code's thinking.ts shouldEnableThinkingByDefault() and
// getThinkingForModel().
func ResolveThinkingConfig(caps ModelCapabilities, userConfig *ai.ThinkingConfig, source QuerySource) *ai.ThinkingConfig {
	if !caps.SupportsThinking {
		return nil
	}

	// Background sources: disable thinking for speed.
	if !source.IsForegroundSource() {
		return &ai.ThinkingConfig{Type: ai.ThinkingDisabled}
	}

	// User explicitly configured thinking.
	if userConfig != nil {
		return userConfig
	}

	// Default: adaptive thinking (model decides when to think).
	return &ai.ThinkingConfig{Type: ai.ThinkingAdaptive}
}

// ModelSupportsAdaptiveThinking returns true if the model supports adaptive
// thinking mode (type: "adaptive"). This is a newer feature available on
// Claude 4.x models.
//
// Inspired by Claude Code's modelSupportsAdaptiveThinking.
func ModelSupportsAdaptiveThinking(model string) bool {
	lower := strings.ToLower(model)
	return hasAnyPrefix(lower, "claude-opus-4", "claude-sonnet-4")
}

// GetMaxThinkingTokens returns the recommended thinking budget for a model.
// This is only used when thinking type is "enabled" (not adaptive).
//
// Inspired by Claude Code's getMaxThinkingTokensForModel.
func GetMaxThinkingTokens(model string, maxOutputTokens int) int {
	lower := strings.ToLower(model)

	var budget int
	switch {
	case strings.HasPrefix(lower, "claude-opus-4"):
		budget = 32000
	case strings.HasPrefix(lower, "claude-sonnet-4"):
		budget = 16000
	default:
		budget = 10000
	}

	// Budget must be less than max output tokens.
	if maxOutputTokens > 0 && budget >= maxOutputTokens {
		budget = maxOutputTokens - 1
	}
	if budget < 1024 {
		budget = 1024
	}
	return budget
}

// EstimateThinkingTokens estimates the token count for thinking content.
// Thinking content tends to be denser than regular text, so we use a
// slightly more aggressive ratio.
func EstimateThinkingTokens(thinkingContent string) int {
	if len(thinkingContent) == 0 {
		return 0
	}
	// Thinking is typically more token-dense (~3 chars/token vs ~4 for regular text).
	return len(thinkingContent)/3 + 3
}

// ThinkingAwareChatOptions adjusts ChatOptions to include the resolved thinking
// configuration. When thinking is enabled, temperature must be set to 1.0
// (API requirement for Claude) and max_tokens should account for thinking budget.
func ThinkingAwareChatOptions(opts ai.ChatOptions, thinking *ai.ThinkingConfig) ai.ChatOptions {
	if thinking == nil || thinking.Type == ai.ThinkingDisabled {
		return opts
	}

	opts.Thinking = thinking

	// API requirement: temperature must be 1.0 when thinking is enabled.
	if thinking.Type == ai.ThinkingEnabled || thinking.Type == ai.ThinkingAdaptive {
		opts.Temperature = 1.0
	}

	// Ensure max_tokens includes thinking budget.
	if thinking.Type == ai.ThinkingEnabled && thinking.BudgetTokens > 0 {
		minTokens := thinking.BudgetTokens + 1024
		if opts.MaxTokens < minTokens {
			opts.MaxTokens = minTokens
		}
	}

	return opts
}
