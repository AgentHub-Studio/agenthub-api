//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_LLMPresetLifecycle validates full LLM preset CRUD: create → get → list → set-default → delete.
func TestE2E_LLMPresetLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// --- Create preset ---
	var preset map[string]any
	status := c.Post("/api/llm-config-presets", map[string]any{
		"name":          "E2E OpenAI Preset",
		"description":   "Created by e2e test",
		"provider":      "openai",
		"model":         "gpt-4o-mini",
		"maxTokens":     2048,
		"contextWindow": 128000,
		"temperature":   0.7,
		"configJson": map[string]any{
			"top_p":             0.9,
			"frequency_penalty": 0.2,
		},
		"isDefault": false,
	}, &preset)
	require.Equal(t, http.StatusCreated, status, "create LLM preset")
	presetID, ok := preset["id"].(string)
	require.True(t, ok, "created preset must have a string id")
	t.Cleanup(func() { c.Delete("/api/llm-config-presets/" + presetID) })

	t.Run("preset has correct fields", func(t *testing.T) {
		assert.Equal(t, "E2E OpenAI Preset", preset["name"])
		assert.Equal(t, "openai", preset["provider"])
		assert.Equal(t, "gpt-4o-mini", preset["model"])
		assert.Equal(t, float64(2048), preset["maxTokens"])
		assert.Equal(t, float64(128000), preset["contextWindow"])
		assert.Equal(t, 0.7, preset["temperature"])
		assert.Equal(t, map[string]any{
			"top_p":             0.9,
			"frequency_penalty": 0.2,
		}, preset["configJson"])
		assert.NotEmpty(t, presetID)
	})

	// --- Get by ID ---
	t.Run("get preset by id", func(t *testing.T) {
		var fetched map[string]any
		s := c.Get("/api/llm-config-presets/"+presetID, &fetched)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, presetID, fetched["id"])
		assert.Equal(t, "openai", fetched["provider"])
		assert.Equal(t, float64(128000), fetched["contextWindow"])
		assert.Equal(t, map[string]any{
			"top_p":             0.9,
			"frequency_penalty": 0.2,
		}, fetched["configJson"])
	})

	// --- List ---
	t.Run("list presets contains created preset", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/llm-config-presets?size=50", &page)
		assert.Equal(t, http.StatusOK, s)
		found := false
		for _, p := range page.Content {
			if p["id"] == presetID {
				found = true
				break
			}
		}
		assert.True(t, found, "created preset must appear in list")
	})

	// --- List by provider ---
	t.Run("list presets by provider", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/llm-config-presets/by-provider/openai?size=50", &page)
		assert.Equal(t, http.StatusOK, s)
		for _, p := range page.Content {
			assert.Equal(t, "openai", p["provider"])
		}
	})

	// --- Update ---
	t.Run("update preset fields and config", func(t *testing.T) {
		var updated map[string]any
		s := c.Put("/api/llm-config-presets/"+presetID, map[string]any{
			"description":   "Updated by e2e test",
			"maxTokens":     4096,
			"contextWindow": 200000,
			"temperature":   0.3,
			"configJson": map[string]any{
				"top_p":            0.8,
				"presence_penalty": 0.1,
			},
		}, &updated)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "Updated by e2e test", updated["description"])
		assert.Equal(t, float64(4096), updated["maxTokens"])
		assert.Equal(t, float64(200000), updated["contextWindow"])
		assert.Equal(t, 0.3, updated["temperature"])
		assert.Equal(t, map[string]any{
			"top_p":            0.8,
			"presence_penalty": 0.1,
		}, updated["configJson"])

		var fetched map[string]any
		require.Equal(t, http.StatusOK, c.Get("/api/llm-config-presets/"+presetID, &fetched))
		assert.Equal(t, updated["configJson"], fetched["configJson"])
	})

	// --- Set as default ---
	t.Run("set preset as default", func(t *testing.T) {
		s := c.Put("/api/llm-config-presets/"+presetID+"/default", nil, nil)
		require.Equal(t, http.StatusNoContent, s)

		var fetched map[string]any
		require.Equal(t, http.StatusOK, c.Get("/api/llm-config-presets/"+presetID, &fetched))
		assert.Equal(t, true, fetched["isDefault"])
	})

	// --- Delete ---
	t.Run("delete preset", func(t *testing.T) {
		s := c.Delete("/api/llm-config-presets/" + presetID)
		assert.Equal(t, http.StatusNoContent, s)

		var notFound testutil.ErrorResponse
		assert.Equal(t, http.StatusNotFound, c.Get("/api/llm-config-presets/"+presetID, &notFound))
	})
}

// TestE2E_LLMPresetTenantIsolation verifies presets are isolated per tenant.
func TestE2E_LLMPresetTenantIsolation(t *testing.T) {
	cfg := e2eConfig()
	tenantA := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	tenantB := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	cA := tenantA.Client(t, cfg.backendURL)
	cB := tenantB.Client(t, cfg.backendURL)

	// Create preset for tenant A
	var preset map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/llm-config-presets", map[string]any{
		"name":          "TenantA Preset",
		"provider":      "anthropic",
		"model":         "claude-3-haiku-20240307",
		"maxTokens":     1024,
		"contextWindow": 200000,
		"temperature":   0.5,
		"configJson": map[string]any{
			"top_k": 40,
		},
	}, &preset))
	presetID, ok := preset["id"].(string)
	require.True(t, ok, "created preset must have a string id")
	t.Cleanup(func() { cA.Delete("/api/llm-config-presets/" + presetID) })

	// Tenant B must not see tenant A's preset
	var errResp testutil.ErrorResponse
	s := cB.Get("/api/llm-config-presets/"+presetID, &errResp)
	assert.Equal(t, http.StatusNotFound, s, "tenant B must not see tenant A preset")

	var page testutil.Page[map[string]any]
	require.Equal(t, http.StatusOK, cB.Get("/api/llm-config-presets?size=50", &page))
	for _, listed := range page.Content {
		assert.NotEqual(t, presetID, listed["id"], "tenant B list must not include tenant A preset")
	}
}

// TestE2E_SettingsUpsertAndGet validates settings key-value storage.
func TestE2E_SettingsUpsertAndGet(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	const testKey = "e2e.test.setting"
	t.Cleanup(func() { c.Delete("/api/settings/" + testKey) })

	// --- Upsert ---
	value, _ := json.Marshal("e2e-value")
	var upserted map[string]any
	s := c.Put("/api/settings/"+testKey, map[string]any{
		"value":       json.RawMessage(value),
		"description": "Created by e2e test",
	}, &upserted)
	require.Equal(t, http.StatusOK, s, "upsert setting")
	assert.Equal(t, testKey, upserted["key"])

	// --- Get ---
	t.Run("get setting by key", func(t *testing.T) {
		var fetched map[string]any
		s := c.Get("/api/settings/"+testKey, &fetched)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, testKey, fetched["key"])
	})

	// --- List ---
	t.Run("list settings contains upserted key", func(t *testing.T) {
		var settings []map[string]any
		s := c.Get("/api/settings", &settings)
		assert.Equal(t, http.StatusOK, s)
		found := false
		for _, sv := range settings {
			if sv["key"] == testKey {
				found = true
				break
			}
		}
		assert.True(t, found, "upserted setting must appear in list")
	})

	// --- Delete ---
	t.Run("delete setting", func(t *testing.T) {
		s := c.Delete("/api/settings/" + testKey)
		assert.Equal(t, http.StatusNoContent, s)

		var notFound testutil.ErrorResponse
		s = c.Get("/api/settings/"+testKey, &notFound)
		assert.Equal(t, http.StatusNotFound, s)
	})
}

// TestE2E_SettingsNestedSecretsAreRedacted verifies that nested values never
// expose credentials through the settings HTTP responses.
func TestE2E_SettingsNestedSecretsAreRedacted(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	const (
		settingKey       = "e2e.provider.bundle"
		clientSecret     = "e2e-nested-client-secret"
		authorizationVal = "Bearer e2e-nested-authorization"
	)
	t.Cleanup(func() { c.Delete("/api/settings/" + settingKey) })

	assertRedacted := func(t *testing.T, response map[string]any) {
		t.Helper()
		value, ok := response["value"].(map[string]any)
		require.True(t, ok, "setting response must contain object value")

		credentials, ok := value["credentials"].(map[string]any)
		require.True(t, ok, "setting value must preserve credentials object")
		assert.Equal(t, "***", credentials["clientSecret"])
		assert.NotEqual(t, clientSecret, credentials["clientSecret"])

		headers, ok := credentials["headers"].(map[string]any)
		require.True(t, ok, "setting value must preserve headers object")
		assert.Equal(t, "***", headers["Authorization"])
		assert.NotEqual(t, authorizationVal, headers["Authorization"])
		assert.Equal(t, "e2e-trace-id", headers["X-Trace"])
	}

	requestValue := map[string]any{
		"provider": "custom",
		"credentials": map[string]any{
			"clientSecret": clientSecret,
			"headers": map[string]any{
				"Authorization": authorizationVal,
				"X-Trace":       "e2e-trace-id",
			},
		},
	}

	var upserted map[string]any
	require.Equal(t, http.StatusOK, c.Put("/api/settings/"+settingKey, map[string]any{
		"value":       requestValue,
		"description": "Nested setting secret redaction E2E",
	}, &upserted))
	assertRedacted(t, upserted)

	var fetched map[string]any
	require.Equal(t, http.StatusOK, c.Get("/api/settings/"+settingKey, &fetched))
	assertRedacted(t, fetched)

	var listed []map[string]any
	require.Equal(t, http.StatusOK, c.Get("/api/settings", &listed))
	for _, setting := range listed {
		if setting["key"] == settingKey {
			assertRedacted(t, setting)
			return
		}
	}
	t.Fatal("nested secret setting must be listed")
}
