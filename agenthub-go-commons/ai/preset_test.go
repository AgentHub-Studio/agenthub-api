package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// stubModel is a minimal ChatModel for testing.
type stubModel struct{ name string }

func (s *stubModel) Chat(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
	return &ai.ChatResponse{Content: "ok"}, nil
}
func (s *stubModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk)
	close(ch)
	return ch, nil
}
func (s *stubModel) GetProviderName() string { return s.name }

func TestLlmPresetConfig_ToChatOptions(t *testing.T) {
	cfg := ai.LlmPresetConfig{
		Name:       "gpt-4-preset",
		ModelID:    "gpt-4o",
		Provider:   "OPENAI",
		ConfigJSON: json.RawMessage(`{"temperature":0.7,"max_tokens":1024,"system_message":"You are a helpful assistant"}`),
	}
	opts, err := cfg.ToChatOptions()
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", opts.Model)
	assert.InDelta(t, 0.7, opts.Temperature, 1e-9)
	assert.Equal(t, 1024, opts.MaxTokens)
	assert.Equal(t, "You are a helpful assistant", opts.SystemMsg)
}

func TestLlmPresetConfig_ToChatOptions_Empty(t *testing.T) {
	cfg := ai.LlmPresetConfig{ModelID: "gpt-3.5-turbo"}
	opts, err := cfg.ToChatOptions()
	require.NoError(t, err)
	assert.Equal(t, "gpt-3.5-turbo", opts.Model)
	assert.Zero(t, opts.Temperature)
}

func TestProviderRegistry_GetChatModel_ByPresetName(t *testing.T) {
	r := ai.NewProviderRegistry()
	model := &stubModel{name: "openai"}
	r.Register("my-gpt4-preset", model)

	preset := ai.LlmPresetConfig{Name: "my-gpt4-preset", Provider: "OPENAI", ModelID: "gpt-4"}
	got, opts, err := r.GetChatModel(preset)
	require.NoError(t, err)
	assert.Same(t, model, got)
	assert.Equal(t, "gpt-4", opts.Model)
}

func TestProviderRegistry_GetChatModel_ByProviderType(t *testing.T) {
	r := ai.NewProviderRegistry()
	model := &stubModel{name: "OPENAI"}
	r.Register("OPENAI", model)

	preset := ai.LlmPresetConfig{Name: "unknown-preset", Provider: "OPENAI", ModelID: "gpt-4o-mini"}
	got, opts, err := r.GetChatModel(preset)
	require.NoError(t, err)
	assert.Same(t, model, got)
	assert.Equal(t, "gpt-4o-mini", opts.Model)
}

func TestProviderRegistry_GetChatModel_NotFound(t *testing.T) {
	r := ai.NewProviderRegistry()
	preset := ai.LlmPresetConfig{Name: "ghost", Provider: "GROQ", ModelID: "llama3"}
	_, _, err := r.GetChatModel(preset)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ai.ErrProviderNotFound))
}

func TestProviderRegistry_ThreadSafe(t *testing.T) {
	r := ai.NewProviderRegistry()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(i int) {
			r.Register("OPENAI", &stubModel{name: "openai"})
			r.Available()
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestLlmPresetConfig_OverrideCredentials_Present(t *testing.T) {
	cfg := ai.LlmPresetConfig{
		ConfigJSON: json.RawMessage(`{"api_key":"sk-custom","base_url":"https://proxy.example.com/v1"}`),
	}
	apiKey, baseURL := cfg.OverrideCredentials()
	assert.Equal(t, "sk-custom", apiKey)
	assert.Equal(t, "https://proxy.example.com/v1", baseURL)
}

func TestLlmPresetConfig_OverrideCredentials_Absent(t *testing.T) {
	cfg := ai.LlmPresetConfig{
		ConfigJSON: json.RawMessage(`{"temperature":0.5}`),
	}
	apiKey, baseURL := cfg.OverrideCredentials()
	assert.Empty(t, apiKey)
	assert.Empty(t, baseURL)
}

func TestProviderRegistry_GetChatModel_PresetCredentialOverride(t *testing.T) {
	r := ai.NewProviderRegistry()

	ai.RegisterFactory("OPENAI", func(_, _ string) ai.ChatModel {
		return &stubModel{name: "openai-custom"}
	})

	preset := ai.LlmPresetConfig{
		Name:       "tenant-preset",
		Provider:   "OPENAI",
		ModelID:    "gpt-4o",
		ConfigJSON: json.RawMessage(`{"api_key":"sk-tenant-key","base_url":""}`),
	}

	// First call: creates a new instance via factory.
	got1, opts1, err := r.GetChatModel(preset)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", opts1.Model)
	assert.NotNil(t, got1)

	// Second call with same credentials: must return cached instance.
	got2, _, err := r.GetChatModel(preset)
	require.NoError(t, err)
	assert.Same(t, got1, got2, "same credentials should return the cached instance")
}

func TestProviderRegistry_GetChatModel_DifferentCredentialsDifferentInstances(t *testing.T) {
	r := ai.NewProviderRegistry()
	ai.RegisterFactory("ANTHROPIC", func(_, apiKey string) ai.ChatModel {
		return &stubModel{name: "anthropic-" + apiKey}
	})

	presetA := ai.LlmPresetConfig{Provider: "ANTHROPIC", ModelID: "claude-3", ConfigJSON: json.RawMessage(`{"api_key":"key-A"}`)}
	presetB := ai.LlmPresetConfig{Provider: "ANTHROPIC", ModelID: "claude-3", ConfigJSON: json.RawMessage(`{"api_key":"key-B"}`)}

	modelA, _, err := r.GetChatModel(presetA)
	require.NoError(t, err)
	modelB, _, err := r.GetChatModel(presetB)
	require.NoError(t, err)

	// Different API keys -> different instances.
	assert.NotSame(t, modelA, modelB)
}

func TestEnvConfig_Defaults(t *testing.T) {
	// Ensure no panic and sensible defaults when env vars are unset.
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	cfg := ai.EnvConfigFromEnvironment()
	assert.Equal(t, "https://api.openai.com/v1", cfg.OpenAIBaseURL)
	assert.Equal(t, "https://api.anthropic.com", cfg.AnthropicBaseURL)
	assert.Equal(t, "http://localhost:11434", cfg.OllamaBaseURL)
	assert.Equal(t, "https://openrouter.ai/api/v1", cfg.OpenRouterBaseURL)
}
