//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_AgentVersionLifecycle validates the full agent version workflow:
// list empty → create draft → publish → conflict guard → rollback.
func TestE2E_AgentVersionLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create the parent agent.
	var ag map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/agents", map[string]any{
		"name":        "Version Lifecycle Agent",
		"description": "Agent for version lifecycle E2E",
	}, &ag))
	agentID := ag["id"].(string)
	t.Cleanup(func() { c.Delete("/api/agents/" + agentID) })

	// --- List versions (empty) ---
	t.Run("list versions is empty initially", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/agents/"+agentID+"/versions?page=0&size=10", &page)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, 0, page.TotalElements)
	})

	// --- Get draft (none yet) ---
	t.Run("get draft returns 404 when no draft exists", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Get("/api/agents/"+agentID+"/versions/draft", &errResp)
		assert.Equal(t, http.StatusNotFound, s)
	})

	// --- Get latest published (none yet) ---
	t.Run("get latest published returns 404 when no published version", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Get("/api/agents/"+agentID+"/versions/latest-published", &errResp)
		assert.Equal(t, http.StatusNotFound, s)
	})

	// --- Create draft ---
	var draftVersion map[string]any
	t.Run("create draft version", func(t *testing.T) {
		s := c.Post("/api/agents/"+agentID+"/versions", map[string]any{
			"description":    "Initial draft",
			"definitionJson": map[string]any{"systemPrompt": "Version one system prompt"},
			"configJson":     map[string]any{"provider": "openrouter", "model": "baseline-model"},
		}, &draftVersion)
		require.Equal(t, http.StatusCreated, s, "create draft must return 201")
		require.NotNil(t, draftVersion["id"], "draft must have id")
		assert.Equal(t, agentID, draftVersion["agentId"])
		assert.Equal(t, "DRAFT", draftVersion["status"])
		assert.NotNil(t, draftVersion["createdAt"])
	})

	// --- Conflict: second draft must fail ---
	t.Run("second draft creation returns 409", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Post("/api/agents/"+agentID+"/versions", map[string]any{
			"description": "Should conflict",
		}, &errResp)
		assert.Equal(t, http.StatusConflict, s)
	})

	// --- Get draft ---
	t.Run("get draft returns the active draft", func(t *testing.T) {
		var v map[string]any
		s := c.Get("/api/agents/"+agentID+"/versions/draft", &v)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, draftVersion["id"], v["id"])
		assert.Equal(t, "DRAFT", v["status"])
	})

	// --- Update draft ---
	versionID := draftVersion["id"].(string)
	t.Run("update draft description", func(t *testing.T) {
		newDesc := "Updated description"
		var updated map[string]any
		s := c.Put("/api/agents/"+agentID+"/versions/by-id/"+versionID, map[string]any{
			"description": newDesc,
		}, &updated)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, newDesc, updated["description"])
	})

	// --- List versions (now has 1) ---
	t.Run("list versions returns draft", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/agents/"+agentID+"/versions", &page)
		assert.Equal(t, http.StatusOK, s)
		assert.GreaterOrEqual(t, page.TotalElements, 1)
		found := false
		for _, v := range page.Content {
			if v["id"] == versionID {
				found = true
			}
		}
		assert.True(t, found, "draft must appear in list")
	})

	// --- Publish ---
	t.Run("publish draft version", func(t *testing.T) {
		var published map[string]any
		s := c.Post("/api/agents/"+agentID+"/versions/"+versionID+"/publish", nil, &published)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "PUBLISHED", published["status"])
	})

	// --- Get latest published ---
	t.Run("get latest published returns the just-published version", func(t *testing.T) {
		var v map[string]any
		s := c.Get("/api/agents/"+agentID+"/versions/latest-published", &v)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, versionID, v["id"])
		assert.Equal(t, "PUBLISHED", v["status"])
	})

	// --- Attempt to update published version (immutable) ---
	t.Run("update published version returns 409", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Put("/api/agents/"+agentID+"/versions/by-id/"+versionID, map[string]any{
			"description": "should fail",
		}, &errResp)
		assert.Equal(t, http.StatusConflict, s)
	})

	// --- Attempt to publish again (immutable) ---
	t.Run("publish already-published version returns 409", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Post("/api/agents/"+agentID+"/versions/"+versionID+"/publish", nil, &errResp)
		assert.Equal(t, http.StatusConflict, s)
	})

	// --- Create and publish a later version before rolling back to version 1. ---
	var secondVersion map[string]any
	t.Run("create and publish second version", func(t *testing.T) {
		s := c.Post("/api/agents/"+agentID+"/versions", map[string]any{
			"description":    "Second published version",
			"definitionJson": map[string]any{"systemPrompt": "Version two system prompt"},
			"configJson":     map[string]any{"provider": "openrouter", "model": "later-model"},
		}, &secondVersion)
		require.Equal(t, http.StatusCreated, s)
		secondVersionID := secondVersion["id"].(string)

		var published map[string]any
		s = c.Post("/api/agents/"+agentID+"/versions/"+secondVersionID+"/publish", nil, &published)
		require.Equal(t, http.StatusOK, s)
		assert.Equal(t, "PUBLISHED", published["status"])
	})

	t.Run("rollback restores first published snapshot", func(t *testing.T) {
		var rollback map[string]any
		s := c.Post("/api/agents/"+agentID+"/versions/"+versionID+"/rollback", nil, &rollback)
		require.Equal(t, http.StatusOK, s)
		assert.Equal(t, "PUBLISHED", rollback["status"])
		assert.Equal(t, "Rollback to version 1", rollback["description"])

		var restored map[string]any
		require.Equal(t, http.StatusOK, c.Get("/api/agents/"+agentID, &restored))
		assert.Equal(t, "Version one system prompt", restored["systemPrompt"])
		modelConfig, ok := restored["modelConfig"].(map[string]any)
		require.True(t, ok, "restored modelConfig must be an object")
		assert.Equal(t, "baseline-model", modelConfig["model"])
		assert.EqualValues(t, 3, restored["currentVersion"])
	})
}

// TestE2E_AgentVersionTenantIsolation verifies that agent versions are isolated per tenant.
func TestE2E_AgentVersionTenantIsolation(t *testing.T) {
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

	// Create agent and draft version in tenant A.
	var agA map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/agents", map[string]any{
		"name": "TenantA Version Agent",
	}, &agA))
	agentAID := agA["id"].(string)
	t.Cleanup(func() { cA.Delete("/api/agents/" + agentAID) })

	require.Equal(t, http.StatusCreated, cA.Post("/api/agents/"+agentAID+"/versions", map[string]any{
		"description": "Tenant A private draft",
	}, nil))

	// Tenant B must not see tenant A's versions.
	var page testutil.Page[map[string]any]
	s := cB.Get("/api/agents/"+agentAID+"/versions", &page)
	// Either 404 (agent not found) or empty list — both are acceptable isolation behaviors.
	assert.True(t, s == http.StatusNotFound || len(page.Content) == 0,
		"tenant B must not access tenant A's agent versions; got status %d", s)
}
