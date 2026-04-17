package agentic

import (
	"strings"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// toolChoiceCapableProviders enumerates providers that reliably honour the
// tool_choice parameter in their OpenAI-compatible Chat Completions payload.
// Ollama is excluded because its OpenAI-compatible endpoint ignores
// tool_choice and routinely emits text-only responses even when "any" is
// requested — falling back to ai.ToolChoiceAuto for that provider keeps
// behaviour predictable across agents.
//
// New providers are added as their tool_choice support is verified end-to-end;
// unknown providers default to "auto" (safe), not "required".
var toolChoiceCapableProviders = map[string]bool{
	"openai":          true,
	"openairesponses": true,
	"openrouter":      true,
	"anthropic":       true,
}

// resolveToolChoice translates the agent's configured tool-selection mode
// into an *ai.ToolChoice that the provider layer can act on, taking the
// provider's known capabilities into account.
//
// Behaviour:
//   - Empty or "auto" → nil (no override; provider default applies).
//   - "none" → always respected; providers universally accept "don't call tools".
//   - "any"/"required" → only applied when the provider is capable AND the
//     call actually declares tools. Forcing "any" on a tool-less call is an
//     API error on most providers, so we silently downgrade to auto.
func resolveToolChoice(mode ai.ToolChoiceType, tools []ai.Tool, provider string) *ai.ToolChoice {
	switch mode {
	case "", ai.ToolChoiceAuto:
		return nil
	case ai.ToolChoiceNone:
		return &ai.ToolChoice{Type: ai.ToolChoiceNone}
	case ai.ToolChoiceAny, ai.ToolChoiceTool:
		if len(tools) == 0 {
			return nil // nothing to force
		}
		if !providerSupportsToolChoice(provider) {
			return nil // capability gap — fall back to auto
		}
		return &ai.ToolChoice{Type: mode}
	}
	return nil
}

// providerSupportsToolChoice reports whether the given provider is known to
// honour the tool_choice parameter. Matching is case-insensitive and also
// accepts provider IDs with a trailing slug (e.g. "openrouter/anthropic").
func providerSupportsToolChoice(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		return false
	}
	if toolChoiceCapableProviders[p] {
		return true
	}
	// Strip trailing slugs like "openrouter/anthropic" — the prefix before the
	// first slash carries the actual wire protocol we route through.
	if i := strings.IndexByte(p, '/'); i > 0 {
		return toolChoiceCapableProviders[p[:i]]
	}
	return false
}
