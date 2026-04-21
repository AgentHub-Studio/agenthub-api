package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/anthropic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/ollama"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openairesponses"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openrouter"
)

// settingsChatModelFactory builds a ChatModel by reading provider credentials
// from the per-tenant settings table. Credentials are fetched at request time
// so that changes in settings take effect immediately without restarting.
type settingsChatModelFactory struct {
	settingsRepo settings.Repository
	fallback     ai.ChatModel // used when no provider is specified on the agent
}

// Build returns a ChatModel for the given provider by loading its credentials
// from the tenant's settings. Falls back to the default model when provider is empty.
// The model parameter allows selecting the correct API variant (e.g. OpenAI
// Responses API for gpt-5+ models vs Chat Completions for older models).
func (f *settingsChatModelFactory) Build(ctx context.Context, provider, model string) (ai.ChatModel, error) {
	if provider == "" {
		if f.fallback != nil {
			return f.fallback, nil
		}
		return nil, fmt.Errorf("chat model: no provider configured on agent and no fallback available")
	}

	switch provider {
	case "openai":
		apiKey, err := readSettingString(ctx, f.settingsRepo, "openai.apiKey")
		if err != nil || apiKey == "" {
			return nil, fmt.Errorf("chat model: openai.apiKey not configured in settings")
		}
		baseURL, _ := readSettingString(ctx, f.settingsRepo, "openai.baseUrl")
		// Models gpt-5+ use the Responses API instead of Chat Completions.
		if requiresResponsesAPI(model) {
			return openairesponses.New(apiKey, baseURL), nil
		}
		return openai.New(apiKey, baseURL), nil

	case "anthropic":
		apiKey, err := readSettingString(ctx, f.settingsRepo, "claude.apiKey")
		if err != nil || apiKey == "" {
			return nil, fmt.Errorf("chat model: claude.apiKey not configured in settings")
		}
		baseURL, _ := readSettingString(ctx, f.settingsRepo, "claude.baseUrl")
		return anthropic.New(apiKey, baseURL), nil

	case "ollama":
		// Resolution order: per-tenant setting → OLLAMA_BASE_URL env → localhost.
		// The env fallback makes it possible to point all tenants at a shared
		// ollama deployment without writing a setting row per tenant, which is
		// especially useful for self-hosted dev clusters.
		baseURL, _ := readSettingString(ctx, f.settingsRepo, "ollama.baseUrl")
		if baseURL == "" {
			baseURL = os.Getenv("OLLAMA_BASE_URL")
		}
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		// Optional — blank for local Ollama, required for Ollama Cloud
		// (https://ollama.com/v1) and other authenticated deployments.
		apiKey, _ := readSettingString(ctx, f.settingsRepo, "ollama.apiKey")
		return ollama.New(baseURL, apiKey), nil

	case "openrouter":
		apiKey, err := readSettingString(ctx, f.settingsRepo, "openrouter.apiKey")
		if err != nil || apiKey == "" {
			return nil, fmt.Errorf("chat model: openrouter.apiKey not configured in settings")
		}
		baseURL, _ := readSettingString(ctx, f.settingsRepo, "openrouter.baseUrl")
		return openrouter.New(apiKey, baseURL, "agenthub"), nil

	default:
		return nil, fmt.Errorf("chat model: unsupported provider %q", provider)
	}
}

// ResolveDefaultProvider returns the tenant's configured default LLM provider
// from the settings table. Used to fall back when an agent's model_config has
// no explicit provider.
func (f *settingsChatModelFactory) ResolveDefaultProvider(ctx context.Context) string {
	provider, err := readSettingString(ctx, f.settingsRepo, "llm.defaultProvider")
	if err != nil || provider == "" {
		return ""
	}
	return provider
}

// ResolveModel returns the default model for the given provider from settings.
// For example, for "openai" it reads "openai.model". Returns "" if not configured.
func (f *settingsChatModelFactory) ResolveModel(ctx context.Context, provider string) string {
	var key string
	switch provider {
	case "openai":
		key = "openai.model"
	case "anthropic":
		key = "claude.model"
	case "ollama":
		key = "ollama.model"
	case "openrouter":
		key = "openrouter.model"
	default:
		return ""
	}
	model, err := readSettingString(ctx, f.settingsRepo, key)
	if err != nil || model == "" {
		return ""
	}
	return model
}

// requiresResponsesAPI returns true for OpenAI models that must use the
// Responses API (POST /v1/responses) instead of Chat Completions.
// GPT-5 family models and newer use the Responses API exclusively.
func requiresResponsesAPI(model string) bool {
	m := strings.ToLower(model)
	// gpt-5, gpt-5.2, gpt-5.4, gpt-5.4-pro, etc.
	if strings.HasPrefix(m, "gpt-5") {
		return true
	}
	// o3, o3-mini, o3-pro, o4-mini, etc. (reasoning models)
	if strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") {
		return true
	}
	return false
}

// readSettingString reads a string value from the settings table.
// The frontend stores values as JSON.stringify(originalValue), so a string "foo"
// arrives as the JSON literal `"foo"` (value field), which the backend stores as-is.
// Some clients double-encode, producing `"\"foo\""`. We handle both cases.
func readSettingString(ctx context.Context, repo settings.Repository, key string) (string, error) {
	s, err := repo.FindByKey(ctx, key)
	if err != nil {
		return "", err // includes ErrNotFound
	}
	// First unmarshal: `"foo"` → `foo`, or `"\"foo\""` → `"foo"` (still has quotes)
	var v string
	if err := json.Unmarshal(s.Value, &v); err != nil {
		return "", fmt.Errorf("settings: unmarshal %q: %w", key, err)
	}
	// Second unmarshal: handle double-encoded values like `"\"foo\""` → `foo`
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		var inner string
		if err := json.Unmarshal([]byte(v), &inner); err == nil {
			return inner, nil
		}
	}
	return v, nil
}
