//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_MultiTenantIsolation verifies that resources created in one tenant
// are not visible or accessible by another tenant.
func TestE2E_MultiTenantIsolation(t *testing.T) {
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
	t.Logf("e2e isolation tenants: A=%s B=%s", tenantA.Slug, tenantB.Slug)

	clientA := tenantA.Client(t, cfg.backendURL)
	clientB := tenantB.Client(t, cfg.backendURL)

	// Tenant A creates an agent.
	var agentA map[string]any
	status := clientA.Post("/api/agents", map[string]any{
		"name":        "agent-exclusive-a",
		"description": "Belongs only to tenant A",
		"status":      "DRAFT",
	}, &agentA)
	require.Equal(t, http.StatusCreated, status, "tenant A must create agent")
	agentID := agentA["id"].(string)
	t.Cleanup(func() { clientA.Delete("/api/agents/" + agentID) })

	t.Run("tenant A sees own agent in list", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		require.Equal(t, http.StatusOK, clientA.Get("/api/agents?page=0&size=50", &page))
		found := false
		for _, a := range page.Content {
			if a["id"] == agentID {
				found = true
				break
			}
		}
		assert.True(t, found, "tenant A must see its own agent")
	})

	t.Run("tenant B does NOT see tenant A agent in list", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		require.Equal(t, http.StatusOK, clientB.Get("/api/agents?page=0&size=50", &page))
		for _, a := range page.Content {
			assert.NotEqual(t, agentID, a["id"], "tenant B must not see tenant A's agent")
		}
	})

	t.Run("tenant B cannot access tenant A agent by ID — 404", func(t *testing.T) {
		var resp map[string]any
		status := clientB.Get("/api/agents/"+agentID, &resp)
		assert.Equal(t, http.StatusNotFound, status,
			"schema isolation: agent from tenant A must not exist in tenant B's schema")
	})

	t.Run("tenant B has its own isolated schema", func(t *testing.T) {
		var agentB map[string]any
		status := clientB.Post("/api/agents", map[string]any{
			"name":   "agent-exclusive-b",
			"status": "DRAFT",
		}, &agentB)
		require.Equal(t, http.StatusCreated, status)
		bID := agentB["id"].(string)
		defer clientB.Delete("/api/agents/" + bID)

		// B sees its own agent
		var fetchedB map[string]any
		assert.Equal(t, http.StatusOK, clientB.Get("/api/agents/"+bID, &fetchedB))

		// A does not see B's agent
		var fromA map[string]any
		assert.Equal(t, http.StatusNotFound, clientA.Get("/api/agents/"+bID, &fromA))
	})
}
