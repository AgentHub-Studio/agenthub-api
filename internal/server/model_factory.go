package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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

type llmProviderSettings struct {
	provider       string
	modelKey       string
	defaultModel   string
	credentialKeys []string
}

var llmProviderResolutionOrder = []llmProviderSettings{
	{
		provider:       "openrouter",
		modelKey:       "openrouter.model",
		defaultModel:   "mistralai/mistral-nemo",
		credentialKeys: []string{"openrouter.apiKey"},
	},
	{
		provider:       "openai",
		modelKey:       "openai.model",
		defaultModel:   "gpt-4o-mini",
		credentialKeys: []string{"openai.apiKey"},
	},
	{
		provider:       "anthropic",
		modelKey:       "claude.model",
		defaultModel:   "claude-sonnet-4-20250514",
		credentialKeys: []string{"claude.apiKey"},
	},
	{
		provider:       "ollama",
		modelKey:       "ollama.model",
		defaultModel:   "llama3.1",
		credentialKeys: []string{"ollama.baseUrl", "ollama.apiKey"},
	},
}

const (
	defaultProviderSettingKey       = "llm.defaultProvider"
	legacyDefaultProviderSettingKey = "general.defaultProvider"
)

// Build returns a ChatModel for the given provider by loading its credentials
// from the tenant's settings. Falls back to the default model when provider is empty.
// The model parameter allows selecting the correct API variant (e.g. OpenAI
// Responses API for gpt-5+ models vs Chat Completions for older models).
func (f *settingsChatModelFactory) Build(ctx context.Context, provider, model string) (ai.ChatModel, error) {
	if provider == "" {
		provider = f.ResolveDefaultProvider(ctx)
	}
	if provider == "" {
		if f.fallback != nil {
			return f.fallback, nil
		}
		return nil, fmt.Errorf("chat model: no provider configured on agent and no fallback available")
	}
	provider = normalizeProviderName(provider)
	if model == "" {
		model = f.ResolveModel(ctx, provider)
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
// from the settings table. If that provider has no credential configured but
// another provider does, the credential wins. This keeps a fresh tenant usable
// after the admin enters a single LLM token, without requiring an additional
// provider/model setting change.
func (f *settingsChatModelFactory) ResolveDefaultProvider(ctx context.Context) string {
	provider := f.resolveConfiguredDefaultProvider(ctx)
	if provider != "" {
		provider = normalizeProviderName(provider)
		if provider == "ollama" || f.hasProviderCredential(ctx, provider) {
			return provider
		}
		if inferred := f.firstConfiguredProvider(ctx); inferred != "" {
			return inferred
		}
		return provider
	}
	return f.firstConfiguredProvider(ctx)
}

func (f *settingsChatModelFactory) resolveConfiguredDefaultProvider(ctx context.Context) string {
	canonical, canonicalAt, canonicalErr := readSettingStringWithUpdatedAt(ctx, f.settingsRepo, defaultProviderSettingKey)
	legacy, legacyAt, legacyErr := readSettingStringWithUpdatedAt(ctx, f.settingsRepo, legacyDefaultProviderSettingKey)

	canonical = strings.TrimSpace(canonical)
	legacy = strings.TrimSpace(legacy)

	if canonicalErr == nil && canonical != "" && legacyErr == nil && legacy != "" {
		if legacyAt.After(canonicalAt) {
			return legacy
		}
		return canonical
	}
	if canonicalErr == nil && canonical != "" {
		return canonical
	}
	if legacyErr == nil && legacy != "" {
		return legacy
	}
	return ""
}

// ResolveModel returns the default model for the given provider from settings.
// For example, for "openai" it reads "openai.model". Returns "" if not configured.
func (f *settingsChatModelFactory) ResolveModel(ctx context.Context, provider string) string {
	cfg, ok := providerSettings(provider)
	if !ok {
		return ""
	}
	model, err := readSettingString(ctx, f.settingsRepo, cfg.modelKey)
	if err == nil && strings.TrimSpace(model) != "" {
		return model
	}
	return cfg.defaultModel
}

func normalizeProviderName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude":
		return "anthropic"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func providerSettings(provider string) (llmProviderSettings, bool) {
	provider = normalizeProviderName(provider)
	for _, cfg := range llmProviderResolutionOrder {
		if cfg.provider == provider {
			return cfg, true
		}
	}
	return llmProviderSettings{}, false
}

func (f *settingsChatModelFactory) firstConfiguredProvider(ctx context.Context) string {
	for _, cfg := range llmProviderResolutionOrder {
		if f.hasProviderCredential(ctx, cfg.provider) {
			return cfg.provider
		}
	}
	return ""
}

func (f *settingsChatModelFactory) hasProviderCredential(ctx context.Context, provider string) bool {
	cfg, ok := providerSettings(provider)
	if !ok {
		return false
	}
	for _, key := range cfg.credentialKeys {
		value, err := readSettingString(ctx, f.settingsRepo, key)
		if err == nil && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return cfg.provider == "ollama" && strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")) != ""
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
	return decodeSettingString(key, s.Value)
}

func readSettingStringWithUpdatedAt(ctx context.Context, repo settings.Repository, key string) (string, time.Time, error) {
	s, err := repo.FindByKey(ctx, key)
	if err != nil {
		return "", time.Time{}, err // includes ErrNotFound
	}
	value, err := decodeSettingString(key, s.Value)
	return value, s.UpdatedAt, err
}

func decodeSettingString(key string, raw json.RawMessage) (string, error) {
	// First unmarshal: `"foo"` → `foo`, or `"\"foo\""` → `"foo"` (still has quotes)
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
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
