//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_TenantProvisioning verifies the full tenant provisioning flow:
// create tenant → Keycloak realm provisioned → JWT accepted by backend.
func TestE2E_TenantProvisioning(t *testing.T) {
	cfg := e2eConfig()

	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	t.Logf("e2e tenant provisioning tenant: %s", tenant.Slug)

	client := tenant.Client(t, cfg.backendURL)

	t.Run("exists endpoint returns true", func(t *testing.T) {
		var result struct {
			Exists bool `json:"exists"`
		}
		status := client.Get("/public/tenants/"+tenant.Slug+"/exists", &result)
		assert.Equal(t, http.StatusOK, status)
		assert.True(t, result.Exists, "newly created tenant must exist")
	})

	t.Run("exists endpoint returns false for unknown slug", func(t *testing.T) {
		noAuth := testutil.NewAPIClient(t, cfg.backendURL, "")
		var result struct {
			Exists bool `json:"exists"`
		}
		status := noAuth.Get("/public/tenants/non-existent-slug-xyz-99999/exists", &result)
		assert.Equal(t, http.StatusOK, status)
		assert.False(t, result.Exists)
	})

	t.Run("JWT accepted by protected endpoint", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		status := client.Get("/api/agents?page=0&size=5", &page)
		assert.Equal(t, http.StatusOK, status, "valid JWT must be accepted")
		assert.GreaterOrEqual(t, page.TotalElements, 0)
	})

	t.Run("POST with invalid slug returns 400", func(t *testing.T) {
		noAuth := testutil.NewAPIClient(t, cfg.backendURL, "")
		var resp map[string]any
		status := noAuth.Post("/public/tenants",
			map[string]any{"name": "Bad", "id": "INVALID SLUG WITH SPACES"},
			&resp)
		assert.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("list tenants contains created tenant", func(t *testing.T) {
		noAuth := testutil.NewAPIClient(t, cfg.backendURL, "")
		var page testutil.Page[map[string]any]
		status := noAuth.Get("/public/tenants?page=0&size=200", &page)
		require.Equal(t, http.StatusOK, status)

		found := false
		for _, item := range page.Content {
			if item["id"] == tenant.Slug {
				found = true
				break
			}
		}
		assert.True(t, found, "tenant list must include the newly created tenant")
	})

	t.Run("no JWT returns 401", func(t *testing.T) {
		noAuth := testutil.NewAPIClient(t, cfg.backendURL, "")
		var resp map[string]any
		status := noAuth.Get("/api/agents", &resp)
		assert.Equal(t, http.StatusUnauthorized, status)
	})
}
