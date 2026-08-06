//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_WebhookLifecycle validates full webhook CRUD: create → get → list → update → deliveries → delete.
func TestE2E_WebhookLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)
	const (
		webhookSecret        = "e2e-webhook-write-only-secret"
		updatedWebhookSecret = "e2e-webhook-updated-write-only-secret"
	)
	assertSecretRedacted := func(t *testing.T, response map[string]any, secret string) {
		t.Helper()
		assert.NotContains(t, response, "secret")
		body, err := json.Marshal(response)
		require.NoError(t, err)
		assert.NotContains(t, string(body), secret)
	}

	// --- Create webhook ---
	var wh map[string]any
	status := c.Post("/api/webhooks", map[string]any{
		"name":       "E2E Webhook",
		"url":        "https://webhook.site/test-e2e",
		"events":     []string{"execution.completed", "execution.failed"},
		"secret":     webhookSecret,
		"enabled":    true,
		"retryCount": 3,
	}, &wh)
	require.Equal(t, http.StatusCreated, status, "create webhook")
	whID, ok := wh["id"].(string)
	require.True(t, ok, "created webhook must have a string id")
	t.Cleanup(func() { c.Delete("/api/webhooks/" + whID) })

	t.Run("webhook has correct fields", func(t *testing.T) {
		assert.Equal(t, "E2E Webhook", wh["name"])
		assert.Equal(t, "https://webhook.site/test-e2e", wh["url"])
		assert.Equal(t, true, wh["enabled"])
		assert.NotEmpty(t, wh["token"], "webhook must have an auto-generated token")
		assertSecretRedacted(t, wh, webhookSecret)
		assert.NotEmpty(t, whID)
	})

	// --- Get by ID ---
	t.Run("get webhook by id", func(t *testing.T) {
		var fetched map[string]any
		s := c.Get("/api/webhooks/"+whID, &fetched)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, whID, fetched["id"])
		assert.Equal(t, "E2E Webhook", fetched["name"])
		assertSecretRedacted(t, fetched, webhookSecret)
	})

	// --- List ---
	t.Run("list webhooks contains created webhook", func(t *testing.T) {
		var webhooks []map[string]any
		s := c.Get("/api/webhooks", &webhooks)
		assert.Equal(t, http.StatusOK, s)
		found := false
		for _, w := range webhooks {
			if w["id"] == whID {
				found = true
				assertSecretRedacted(t, w, webhookSecret)
				break
			}
		}
		assert.True(t, found, "created webhook must appear in list")
	})

	// --- Update ---
	t.Run("update webhook url", func(t *testing.T) {
		var updated map[string]any
		s := c.Put("/api/webhooks/"+whID, map[string]any{
			"url":     "https://webhook.site/test-e2e-updated",
			"secret":  updatedWebhookSecret,
			"enabled": false,
		}, &updated)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "https://webhook.site/test-e2e-updated", updated["url"])
		assert.Equal(t, false, updated["enabled"])
		assertSecretRedacted(t, updated, updatedWebhookSecret)
	})

	// --- List deliveries (empty for a new webhook) ---
	t.Run("list deliveries returns empty page for new webhook", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/webhooks/"+whID+"/deliveries?size=20", &page)
		assert.Equal(t, http.StatusOK, s)
		assert.EqualValues(t, 0, page.TotalElements)
	})

	// --- Delete ---
	t.Run("delete webhook", func(t *testing.T) {
		require.Equal(t, http.StatusNoContent, c.Delete("/api/webhooks/"+whID))

		var notFound testutil.ErrorResponse
		assert.Equal(t, http.StatusNotFound, c.Get("/api/webhooks/"+whID, &notFound))
	})
}

// TestE2E_WebhookIngest validates that the public ingest endpoint requires a
// valid signature without requiring a bearer token and filters disallowed events.
func TestE2E_WebhookIngest(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	const webhookSecret = "e2e-public-ingest-secret"

	// Create webhook to get a valid token. The URL is never contacted because
	// the exercised requests fail signature validation or are event-filtered.
	var wh map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/webhooks", map[string]any{
		"name":       "Ingest Test Webhook",
		"url":        "https://webhook.site/ingest-test",
		"events":     []string{"custom.event"},
		"secret":     webhookSecret,
		"enabled":    true,
		"retryCount": 1,
	}, &wh))
	whID, ok := wh["id"].(string)
	require.True(t, ok, "created webhook must have a string id")
	token, ok := wh["token"].(string)
	require.True(t, ok, "created webhook must return a token")
	t.Cleanup(func() { c.Delete("/api/webhooks/" + whID) })

	postPublic := func(t *testing.T, eventType, signature string) (int, map[string]any) {
		t.Helper()
		payload, err := json.Marshal(map[string]any{"key": "value"})
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, cfg.backendURL+"/api/webhooks/"+token+"/ingest", bytes.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gitlab-Token", signature)
		req.Header.Set("X-Gitlab-Event", eventType)

		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		result := map[string]any{}
		if len(body) > 0 {
			require.NoError(t, json.Unmarshal(body, &result))
		}
		return resp.StatusCode, result
	}

	t.Run("public request rejects an invalid signature without bearer auth", func(t *testing.T) {
		status, _ := postPublic(t, "custom.event", "invalid-signature")
		assert.Equal(t, http.StatusUnauthorized, status)
	})

	t.Run("public request reports a valid but filtered event", func(t *testing.T) {
		status, result := postPublic(t, "ignored.event", webhookSecret)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "filtered", result["status"])
	})

	t.Run("rejected and filtered requests create no delivery", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		require.Equal(t, http.StatusOK, c.Get("/api/webhooks/"+whID+"/deliveries?size=20", &page))
		assert.EqualValues(t, 0, page.TotalElements)
	})
}

// TestE2E_WebhookTenantIsolation verifies webhooks are isolated per tenant.
func TestE2E_WebhookTenantIsolation(t *testing.T) {
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

	var wh map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/webhooks", map[string]any{
		"name":   "TenantA Webhook",
		"url":    "https://webhook.site/tenant-a",
		"events": []string{"execution.completed"},
	}, &wh))
	whID := wh["id"].(string)
	t.Cleanup(func() { cA.Delete("/api/webhooks/" + whID) })

	// Tenant B must not see tenant A's webhook
	var errResp testutil.ErrorResponse
	s := cB.Get("/api/webhooks/"+whID, &errResp)
	assert.Equal(t, http.StatusNotFound, s, "tenant B must not see tenant A webhook")
}
