//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_VPNResourceLifecycle validates full CRUD for VPN resources.
func TestE2E_VPNResourceLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// --- Create ---
	var created map[string]any
	status := c.Post("/api/vpn-resources", map[string]any{
		"name":        "corp-vpn",
		"description": "Corporate VPN resource for E2E test",
		"enabled":     true,
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "corp-vpn", created["name"])
	id, ok := created["id"].(string)
	require.True(t, ok)

	// --- Get by ID ---
	var fetched map[string]any
	status = c.Get("/api/vpn-resources/"+id, &fetched)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "corp-vpn", fetched["name"])

	// --- List ---
	var page testutil.Page[map[string]any]
	status = c.Get("/api/vpn-resources?page=0&size=20", &page)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, page.TotalElements, 1)

	// --- Update ---
	var updated map[string]any
	status = c.Put("/api/vpn-resources/"+id, map[string]any{
		"name":    "corp-vpn-updated",
		"enabled": false,
	}, &updated)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "corp-vpn-updated", updated["name"])

	// --- Delete ---
	status = c.Delete("/api/vpn-resources/" + id)
	assert.Equal(t, http.StatusNoContent, status)

	// --- Confirm gone ---
	var notFound map[string]any
	status = c.Get("/api/vpn-resources/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_VPNResourceTenantIsolation validates cross-tenant isolation.
func TestE2E_VPNResourceTenantIsolation(t *testing.T) {
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
	cA.Post("/api/vpn-resources", map[string]any{"name": "tenant-a-vpn", "enabled": true}, &created)
	id := created["id"].(string)

	var notFound map[string]any
	status := cB.Get("/api/vpn-resources/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_DatasourceLifecycle validates full CRUD for datasources.
func TestE2E_DatasourceLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// --- Create ---
	var created map[string]any
	status := c.Post("/api/datasources", map[string]any{
		"name":     "prod-postgres",
		"type":     "POSTGRESQL",
		"host":     "pg.internal",
		"port":     5432,
		"database": "appdb",
		"dbUser":   "appuser",
		"dbPassword": "s3cret",
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "prod-postgres", created["name"])
	id, ok := created["id"].(string)
	require.True(t, ok)

	// --- Get by ID ---
	var fetched map[string]any
	status = c.Get("/api/datasources/"+id, &fetched)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "POSTGRESQL", fetched["type"])

	// --- List ---
	var page testutil.Page[map[string]any]
	status = c.Get("/api/datasources?page=0&size=20", &page)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, page.TotalElements, 1)

	// --- Update ---
	var updated map[string]any
	status = c.Put("/api/datasources/"+id, map[string]any{
		"name": "prod-postgres-updated",
	}, &updated)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "prod-postgres-updated", updated["name"])

	// --- Delete ---
	status = c.Delete("/api/datasources/" + id)
	assert.Equal(t, http.StatusNoContent, status)

	var notFound map[string]any
	status = c.Get("/api/datasources/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_DatasourceTenantIsolation validates cross-tenant isolation.
func TestE2E_DatasourceTenantIsolation(t *testing.T) {
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
	cA.Post("/api/datasources", map[string]any{
		"name":       "tenant-a-db",
		"type":       "MYSQL",
		"host":       "mysql.internal",
		"port":       3306,
		"database":   "mydb",
		"dbUser":     "user",
		"dbPassword": "pass",
	}, &created)
	id := created["id"].(string)

	var notFound map[string]any
	status := cB.Get("/api/datasources/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}
