package agentic

import "strings"

// ModelCapabilities centralises per-model feature detection into a single struct.
// Each capability maps to a `modelSupportsX()` function in Claude Code's codebase.
// The struct is computed once and stored in RunGates to avoid re-evaluating
// string checks on every loop iteration.
//
// Inspired by Claude Code's scattered modelSupportsEffort, modelSupportsThinking,
// modelSupportsAdvisor, modelSupportsStructuredOutputs, modelSupports1M, etc.
type ModelCapabilities struct {
	// SupportsEffort indicates whether the model supports the effort parameter.
	SupportsEffort bool
	// SupportsMaxEffort indicates whether the model supports effort="max" (only Opus 4.6).
	SupportsMaxEffort bool
	// SupportsThinking indicates whether the model supports extended thinking.
	SupportsThinking bool
	// SupportsCacheControl indicates whether the model supports cache_control markers
	// for prompt caching (Anthropic models only).
	SupportsCacheControl bool
	// SupportsVision indicates whether the model can process image inputs.
	SupportsVision bool
	// SupportsToolUse indicates whether the model supports tool/function calling.
	SupportsToolUse bool
	// Supports1MContext indicates whether the model supports 1M token context window.
	Supports1MContext bool
	// SupportsStreaming indicates whether the model supports streaming responses.
	SupportsStreaming bool
	// ContextWindowSize is the default context window for this model.
	ContextWindowSize int
}

// DetectModelCapabilities returns the capabilities for the given model and provider.
func DetectModelCapabilities(model, provider string) ModelCapabilities {
	lower := strings.ToLower(model)
	isAnthropic := strings.ToLower(provider) == "anthropic"
	isClaude := isAnthropic || strings.HasPrefix(lower, "claude")
	contextWindowSize := GetContextWindowSize(model, 0)

	caps := ModelCapabilities{
		// Effort: Claude 4.x models (Opus 4.6, Sonnet 4.6).
		SupportsEffort:    ModelSupportsEffort(model),
		SupportsMaxEffort: ModelSupportsMaxEffort(model),

		// Thinking: Claude 3.5+ Sonnet and Claude 4.x models.
		SupportsThinking: isClaude && (hasAnyPrefix(lower, "claude-sonnet-4", "claude-opus-4") ||
			strings.Contains(lower, "sonnet-3-5") ||
			strings.Contains(lower, "sonnet-3.5")),

		// Cache control: Anthropic provider only.
		SupportsCacheControl: isAnthropic,

		// Vision: most modern models support it.
		SupportsVision: isClaude ||
			hasAnyPrefix(lower, "gpt-4o", "gpt-4-turbo", "gpt-4-vision") ||
			strings.HasPrefix(lower, "gemini"),

		// Tool use: all Claude models, GPT-4+, Gemini.
		SupportsToolUse: isClaude ||
			hasAnyPrefix(lower, "gpt-4", "gpt-3.5-turbo") ||
			strings.HasPrefix(lower, "gemini"),

		// 1M context: models with a resolved 1M window, plus known external variants.
		Supports1MContext: contextWindowSize >= 1000000 ||
			strings.Contains(lower, "gemini-1.5-pro"),

		// Streaming: all major models.
		SupportsStreaming: true,

		// Context window: use the existing detection function.
		ContextWindowSize: contextWindowSize,
	}

	return caps
}

// hasAnyPrefix checks if s starts with any of the given prefixes.
func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
