package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// providerHTTPClient is the shared HTTP client for provider model queries.
var providerHTTPClient = &http.Client{Timeout: 15 * time.Second}

// ModelInfo represents a single model available from a provider.
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ProviderInfo describes an available AI provider.
type ProviderInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// availableProviders is the static list of supported AI providers.
var availableProviders = []ProviderInfo{
	{ID: "openai", Name: "OpenAI", Description: "GPT-4, GPT-3.5 and DALL-E models"},
	{ID: "anthropic", Name: "Anthropic", Description: "Claude 3.5 and Claude 4 models"},
	{ID: "ollama", Name: "Ollama", Description: "Local open-source models via Ollama"},
	{ID: "openrouter", Name: "OpenRouter", Description: "Multi-model gateway with 100+ models"},
}

// ListProviders returns the supported AI providers.
func ListProviders() []ProviderInfo {
	return availableProviders
}

// ListOpenAIModels fetches available GPT models using the provided API key.
func ListOpenAIModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai: status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID})
	}
	return models, nil
}

// ListAnthropicModels returns the hardcoded list of Claude models
// (Anthropic doesn't have a public /models endpoint).
func ListAnthropicModels(_ context.Context, _ string) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "claude-opus-4-6", Name: "Claude Opus 4.6"},
		{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5"},
		{ID: "claude-opus-4-5", Name: "Claude Opus 4.5"},
		{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5"},
	}, nil
}

// ListOllamaModels fetches available models from a local Ollama instance.
func ListOllamaModels(ctx context.Context, baseURL string) ([]ModelInfo, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama: decode: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, ModelInfo{ID: m.Name, Name: m.Name})
	}
	return models, nil
}

// ListOpenRouterModels fetches available models from OpenRouter.
func ListOpenRouterModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := providerHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter: status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openrouter: decode: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID, Name: m.Name})
	}
	return models, nil
}

// SetProviderHTTPClient replaces the shared HTTP client used by provider functions.
// Pass nil to restore the default. Used in tests.
func SetProviderHTTPClient(c *http.Client) {
	if c == nil {
		providerHTTPClient = &http.Client{Timeout: 15 * time.Second}
	} else {
		providerHTTPClient = c
	}
}
