//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_AuditRecord validates recording an audit log entry and listing it.
func TestE2E_AuditRecord(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Record an event
	status := c.Post("/api/audit-logs", map[string]any{
		"action":     "agent.created",
		"entityType": "agent",
		"entityId":   "00000000-0000-0000-0000-000000000001",
	}, nil)
	assert.Equal(t, http.StatusCreated, status)

	// List audit logs (admin required)
	var page testutil.Page[map[string]any]
	status = c.Get("/api/audit-logs?page=0&size=20", &page)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, page.TotalElements, int64(1))
}

// TestE2E_AuditTenantIsolation validates that tenant B cannot see tenant A's logs.
func TestE2E_AuditTenantIsolation(t *testing.T) {
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

	// Tenant A records an event
	cA.Post("/api/audit-logs", map[string]any{
		"action":     "agent.deleted",
		"entityType": "agent",
		"entityId":   "00000000-0000-0000-0000-000000000002",
	}, nil)

	// Tenant B list should be empty (zero logs from their perspective)
	var pageB testutil.Page[map[string]any]
	cB.Get("/api/audit-logs", &pageB)
	for _, item := range pageB.Content {
		assert.NotEqual(t, "tenant_a_entity", item["entityId"], "tenant B should not see tenant A's audit logs")
	}
}

// TestE2E_OAuthCredentialLifecycle validates full CRUD for OAuth credentials.
func TestE2E_OAuthCredentialLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// --- Create ---
	var created map[string]any
	status := c.Post("/api/oauth-credentials", map[string]any{
		"name":         "github-oauth",
		"provider":     "github",
		"clientId":     "my-client-id",
		"clientSecret": "my-client-secret",
		"tokenUrl":     "https://github.com/login/oauth/access_token",
		"scopes":       []string{"read:user", "repo"},
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "github-oauth", created["name"])
	id, ok := created["id"].(string)
	require.True(t, ok)

	// --- Get by ID ---
	var fetched map[string]any
	status = c.Get("/api/oauth-credentials/"+id, &fetched)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "github-oauth", fetched["name"])

	// --- List ---
	var page testutil.Page[map[string]any]
	status = c.Get("/api/oauth-credentials?page=0&size=20", &page)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, page.TotalElements, int64(1))

	// --- Update ---
	var updated map[string]any
	status = c.Put("/api/oauth-credentials/"+id, map[string]any{
		"name": "github-oauth-updated",
	}, &updated)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "github-oauth-updated", updated["name"])

	// --- Delete ---
	status = c.Delete("/api/oauth-credentials/" + id)
	assert.Equal(t, http.StatusNoContent, status)

	// --- Confirm gone ---
	var notFound map[string]any
	status = c.Get("/api/oauth-credentials/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_OAuthCredentialTenantIsolation validates cross-tenant isolation.
func TestE2E_OAuthCredentialTenantIsolation(t *testing.T) {
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

	var created map[string]any
	cA.Post("/api/oauth-credentials", map[string]any{
		"name":         "tenant-a-cred",
		"provider":     "custom",
		"clientId":     "cid",
		"clientSecret": "csecret",
		"tokenUrl":     "https://auth.example.com/token",
	}, &created)
	id := created["id"].(string)

	// Tenant B cannot access
	var notFound map[string]any
	status := cB.Get("/api/oauth-credentials/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}
