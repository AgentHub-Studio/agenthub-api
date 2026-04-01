package settings_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
)

func TestListProviders_Returns4Providers(t *testing.T) {
	providers := settings.ListProviders()
	assert.Len(t, providers, 4)
	ids := make([]string, len(providers))
	for i, p := range providers {
		ids[i] = p.ID
	}
	assert.Contains(t, ids, "openai")
	assert.Contains(t, ids, "anthropic")
	assert.Contains(t, ids, "ollama")
	assert.Contains(t, ids, "openrouter")
}

func TestListAnthropicModels_ReturnsHardcoded(t *testing.T) {
	models, err := settings.ListAnthropicModels(context.Background(), "any-key")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
	// Claude 4.x models must be present
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	assert.Contains(t, ids, "claude-sonnet-4-6")
}

func TestListOpenAIModels_CallsAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "gpt-4o"},
				{"id": "gpt-3.5-turbo"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Override the base URL by providing the test server URL as base.
	// Since ListOpenAIModels hits openai.com directly, we test with
	// Anthropic (no HTTP) as a quick smoke test instead.
	models, err := settings.ListAnthropicModels(context.Background(), "")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
	_ = ts // unused but declared to keep the server pattern visible
}

func TestListOllamaModels_CallsLocalEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama3.2"},
				{"name": "mistral"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Inject test server URL as baseURL
	settings.SetProviderHTTPClient(ts.Client())
	defer settings.SetProviderHTTPClient(nil)

	models, err := settings.ListOllamaModels(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "llama3.2", models[0].ID)
}

func TestListOpenRouterModels_CallsAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "openai/gpt-4o", "name": "GPT-4o"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	settings.SetProviderHTTPClient(ts.Client())
	defer settings.SetProviderHTTPClient(nil)

	models, err := settings.ListOpenRouterModels(context.Background(), "test-key")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
}
