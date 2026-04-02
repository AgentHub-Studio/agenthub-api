//go:build e2e

package e2e

import (
	"net/http"
	"testing"

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

	// --- Create webhook ---
	var wh map[string]any
	status := c.Post("/api/webhooks", map[string]any{
		"name":       "E2E Webhook",
		"url":        "https://webhook.site/test-e2e",
		"events":     []string{"execution.completed", "execution.failed"},
		"enabled":    true,
		"retryCount": 3,
	}, &wh)
	require.Equal(t, http.StatusCreated, status, "create webhook")
	whID := wh["id"].(string)
	t.Cleanup(func() { c.Delete("/api/webhooks/" + whID) })

	t.Run("webhook has correct fields", func(t *testing.T) {
		assert.Equal(t, "E2E Webhook", wh["name"])
		assert.Equal(t, "https://webhook.site/test-e2e", wh["url"])
		assert.Equal(t, true, wh["enabled"])
		assert.NotEmpty(t, wh["token"], "webhook must have an auto-generated token")
		assert.NotEmpty(t, whID)
	})

	// --- Get by ID ---
	t.Run("get webhook by id", func(t *testing.T) {
		var fetched map[string]any
		s := c.Get("/api/webhooks/"+whID, &fetched)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, whID, fetched["id"])
		assert.Equal(t, "E2E Webhook", fetched["name"])
	})

	// --- List ---
	t.Run("list webhooks contains created webhook", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/webhooks?size=50", &page)
		assert.Equal(t, http.StatusOK, s)
		found := false
		for _, w := range page.Content {
			if w["id"] == whID {
				found = true
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
			"enabled": false,
		}, &updated)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "https://webhook.site/test-e2e-updated", updated["url"])
		assert.Equal(t, false, updated["enabled"])
	})

	// --- List deliveries (empty for a new webhook) ---
	t.Run("list deliveries returns empty page for new webhook", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/webhooks/"+whID+"/deliveries?size=20", &page)
		assert.Equal(t, http.StatusOK, s)
		assert.EqualValues(t, 0, page.TotalElements)
	})
}

// TestE2E_WebhookIngest validates that a public ingest endpoint accepts payloads.
func TestE2E_WebhookIngest(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create webhook to get a valid token
	var wh map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/webhooks", map[string]any{
		"name":    "Ingest Test Webhook",
		"url":     "https://webhook.site/ingest-test",
		"events":  []string{"custom.event"},
		"enabled": true,
	}, &wh))
	whID := wh["id"].(string)
	token := wh["token"].(string)
	t.Cleanup(func() { c.Delete("/api/webhooks/" + whID) })

	// Ingest event via public endpoint using the webhook token
	t.Run("ingest event via public endpoint returns 200", func(t *testing.T) {
		var result map[string]any
		// Public endpoint — no auth required, use token in URL
		s := c.Post("/api/webhooks/"+token+"/ingest", map[string]any{
			"event": "custom.event",
			"data":  map[string]any{"key": "value"},
		}, &result)
		// Accept 200 or 204 — backend may or may not return a body
		assert.True(t, s == http.StatusOK || s == http.StatusAccepted || s == http.StatusNoContent,
			"ingest must return 2xx, got %d", s)
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
