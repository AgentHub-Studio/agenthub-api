package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/anthropic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/ollama"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
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
func (f *settingsChatModelFactory) Build(ctx context.Context, provider string) (ai.ChatModel, error) {
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
		return openai.New(apiKey, baseURL), nil

	case "anthropic":
		apiKey, err := readSettingString(ctx, f.settingsRepo, "claude.apiKey")
		if err != nil || apiKey == "" {
			return nil, fmt.Errorf("chat model: claude.apiKey not configured in settings")
		}
		baseURL, _ := readSettingString(ctx, f.settingsRepo, "claude.baseUrl")
		return anthropic.New(apiKey, baseURL), nil

	case "ollama":
		baseURL, _ := readSettingString(ctx, f.settingsRepo, "ollama.baseUrl")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return ollama.New(baseURL), nil

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
