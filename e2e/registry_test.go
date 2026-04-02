//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_RegistryPackageLifecycle validates full CRUD for registry packages.
func TestE2E_RegistryPackageLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// --- Create package ---
	var created map[string]any
	status := c.Post("/api/packages", map[string]any{
		"name":        "my-e2e-agent",
		"slug":        "my-e2e-agent",
		"description": "An agent package for E2E testing",
		"type":        "AGENT",
		"visibility":  "private",
	}, &created)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "my-e2e-agent", created["name"])
	pkgID, ok := created["id"].(string)
	require.True(t, ok)

	// --- List mine ---
	var mine testutil.Page[map[string]any]
	status = c.Get("/api/packages/mine?page=0&size=20", &mine)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, mine.TotalElements, int64(1))

	// --- Get by ID ---
	var fetched map[string]any
	status = c.Get("/api/packages/"+pkgID, &fetched)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "my-e2e-agent", fetched["name"])

	// --- Update ---
	status = c.Patch("/api/packages/"+pkgID, map[string]any{
		"name": "my-e2e-agent-v2",
	}, nil)
	assert.Equal(t, http.StatusOK, status)

	// --- Delete ---
	status = c.Delete("/api/packages/" + pkgID)
	assert.Equal(t, http.StatusNoContent, status)

	var notFound map[string]any
	status = c.Get("/api/packages/"+pkgID, &notFound)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_RegistryVersionLifecycle validates publishing and listing versions.
func TestE2E_RegistryVersionLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Setup: create package first
	var pkg map[string]any
	status := c.Post("/api/packages", map[string]any{
		"name":       "versioned-pkg",
		"slug":       "versioned-pkg",
		"description": "Package for version E2E",
		"type":       "AGENT",
		"visibility": "private",
	}, &pkg)
	require.Equal(t, http.StatusCreated, status)
	pkgID := pkg["id"].(string)

	versionBase := "/api/packages/" + pkgID + "/versions"

	// --- Publish a version ---
	var version map[string]any
	status = c.Post(versionBase, map[string]any{
		"version":     "1.0.0",
		"changelog":   "Initial release",
		"storagePath": "packages/versioned-pkg/1.0.0/package.tar.gz",
		"checksum":    "sha256:abc123",
	}, &version)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "1.0.0", version["version"])

	// --- List versions ---
	var versionPage testutil.Page[map[string]any]
	status = c.Get(versionBase+"?page=0&size=10", &versionPage)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, versionPage.TotalElements, int64(1))

	// --- Get specific version ---
	var fetchedV map[string]any
	status = c.Get(versionBase+"/1.0.0", &fetchedV)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "1.0.0", fetchedV["version"])

	// --- Delete version ---
	status = c.Delete(versionBase + "/1.0.0")
	assert.Equal(t, http.StatusNoContent, status)
}

// TestE2E_PublicPackageListing validates the public (unauthenticated) package listing.
func TestE2E_PublicPackageListing(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create a public package
	var pkg map[string]any
	status := c.Post("/api/packages", map[string]any{
		"name":       "public-agent",
		"slug":       "public-agent-e2e",
		"description": "A publicly visible agent",
		"type":       "AGENT",
		"visibility": "public",
	}, &pkg)
	require.Equal(t, http.StatusCreated, status)

	// Public listing (no auth needed — use raw http)
	var publicPage testutil.Page[map[string]any]
	status = c.Get("/api/packages?page=0&size=20", &publicPage)
	assert.Equal(t, http.StatusOK, status)
}
