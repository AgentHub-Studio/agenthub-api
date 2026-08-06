//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_AgentMemoryLifecycle validates CRUD operations for agent memory entries.
func TestE2E_AgentMemoryLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create an agent to attach memory to
	var agent map[string]any
	status := c.Post("/api/agents", map[string]any{
		"name":        "Memory Test Agent",
		"description": "Agent for memory E2E tests",
	}, &agent)
	require.Equal(t, http.StatusCreated, status)
	agentID := agent["id"].(string)

	memBase := "/api/agents/" + agentID + "/memory"

	// --- Upsert a key ---
	status = c.Put(memBase+"/user-preference", map[string]any{
		"value": "dark-mode",
	}, nil)
	assert.Equal(t, http.StatusOK, status)

	// --- Get by key ---
	var entry map[string]any
	status = c.Get(memBase+"/user-preference", &entry)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "user-preference", entry["key"])

	// --- List memory ---
	var items []map[string]any
	status = c.Get(memBase, &items)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, len(items), 1)

	found := false
	for _, item := range items {
		if item["key"] == "user-preference" {
			found = true
		}
	}
	assert.True(t, found)

	// --- Delete by key ---
	status = c.Delete(memBase + "/user-preference")
	assert.Equal(t, http.StatusNoContent, status)

	// --- Verify gone ---
	var notFound map[string]any
	status = c.Get(memBase+"/user-preference", &notFound)
	assert.Equal(t, http.StatusNotFound, status)

	// --- Clear all ---
	c.Put(memBase+"/key1", map[string]any{"value": "v1"}, nil)
	c.Put(memBase+"/key2", map[string]any{"value": "v2"}, nil)
	status = c.Delete(memBase)
	assert.Equal(t, http.StatusNoContent, status)

	var emptyList []map[string]any
	c.Get(memBase, &emptyList)
	assert.Empty(t, emptyList)
}

// TestE2E_AgentMemoryRecall validates semantic recall endpoint.
func TestE2E_AgentMemoryRecall(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	var agent map[string]any
	status := c.Post("/api/agents", map[string]any{
		"name": "Recall Test Agent",
	}, &agent)
	require.Equal(t, http.StatusCreated, status)
	agentID := agent["id"].(string)

	// Seed semantic memory using the same fixed-size embedding required by pgvector.
	embedding := make([]float32, 1024)
	embedding[0] = 1
	require.Equal(t, http.StatusOK, c.Put("/api/agents/"+agentID+"/memory/pref-lang", map[string]any{
		"value":     "Portuguese",
		"embedding": embedding,
	}, nil))

	var recalled []map[string]any
	status = c.Post("/api/agents/"+agentID+"/memory/recall", map[string]any{
		"embedding": embedding,
		"limit":     3,
	}, &recalled)
	assert.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, recalled)
	assert.Equal(t, "pref-lang", recalled[0]["key"])
}

// TestE2E_SearchReturnsResults validates the global search endpoint.
func TestE2E_SearchReturnsResults(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Seed an agent
	c.Post("/api/agents", map[string]any{
		"name":        "Searchable Agent",
		"description": "This agent should be findable",
	}, nil)

	// Search — returns list of results
	var results any
	status := c.Get("/api/search?q=Searchable", &results)
	assert.Equal(t, http.StatusOK, status)
}

// TestE2E_SearchRequiresQuery validates 400 when q param is missing.
func TestE2E_SearchRequiresQuery(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	var errResp map[string]any
	status := c.Get("/api/search", &errResp)
	assert.Equal(t, http.StatusBadRequest, status)
}
