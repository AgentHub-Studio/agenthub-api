//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_MCPServerConfigLifecycle validates full CRUD for MCP server configs.
func TestE2E_MCPServerConfigLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// --- Create stdio config ---
	var created map[string]any
	status := c.Post("/api/mcp-server-configs", map[string]any{
		"name":          "filesystem-mcp",
		"transportType": "stdio",
		"command":       "/usr/local/bin/mcp-filesystem",
		"args":          []string{"--root", "/tmp"},
		"autoStart":     true,
		"enabled":       true,
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "filesystem-mcp", created["name"])
	assert.Equal(t, "stdio", created["transportType"])
	id, ok := created["id"].(string)
	require.True(t, ok, "id should be a string")

	// --- Get by ID ---
	var fetched map[string]any
	status = c.Get("/api/mcp-server-configs/"+id, &fetched)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "filesystem-mcp", fetched["name"])

	// --- List returns pagination envelope ---
	var page testutil.Page[map[string]any]
	status = c.Get("/api/mcp-server-configs?page=0&size=20", &page)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, page.TotalElements, int64(1))

	found := false
	for _, item := range page.Content {
		if item["id"] == id {
			found = true
		}
	}
	assert.True(t, found, "created config should appear in list")

	// --- Update ---
	var updated map[string]any
	status = c.Put("/api/mcp-server-configs/"+id, map[string]any{
		"name": "filesystem-mcp-updated",
	}, &updated)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "filesystem-mcp-updated", updated["name"])

	// --- Delete ---
	status = c.Delete("/api/mcp-server-configs/" + id)
	assert.Equal(t, http.StatusNoContent, status)

	// --- Confirm gone ---
	var notFound map[string]any
	status = c.Get("/api/mcp-server-configs/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_MCPServerConfigHTTPTransport validates creation of an HTTP transport config.
func TestE2E_MCPServerConfigHTTPTransport(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	var created map[string]any
	status := c.Post("/api/mcp-server-configs", map[string]any{
		"name":          "remote-http-mcp",
		"transportType": "http",
		"httpBaseUrl":   "https://mcp.example.com",
		"autoStart":     false,
		"enabled":       true,
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "http", created["transportType"])
	assert.Equal(t, "https://mcp.example.com", created["httpBaseUrl"])

	id := created["id"].(string)
	c.Delete("/api/mcp-server-configs/" + id)
}

// TestE2E_MCPServerConfigTenantIsolation validates that tenant B cannot see tenant A's configs.
func TestE2E_MCPServerConfigTenantIsolation(t *testing.T) {
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

	// Tenant A creates a config
	var created map[string]any
	status := cA.Post("/api/mcp-server-configs", map[string]any{
		"name":          "tenant-a-only",
		"transportType": "stdio",
		"command":       "/bin/true",
		"autoStart":     false,
		"enabled":       true,
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	id := created["id"].(string)

	// Tenant B cannot access it by ID
	var notFound map[string]any
	status = cB.Get("/api/mcp-server-configs/"+id, &notFound)
	assert.Equal(t, http.StatusNotFound, status)

	// Tenant B's list does not include tenant A's config
	var pageB testutil.Page[map[string]any]
	cB.Get("/api/mcp-server-configs", &pageB)
	for _, item := range pageB.Content {
		assert.NotEqual(t, id, item["id"], "tenant B should not see tenant A's MCP config")
	}
}
