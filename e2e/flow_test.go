//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

func TestE2E_HealthCheck(t *testing.T) {
	client := testutil.NewAPIClient(t, e2eConfig().backendURL, "")
	var result map[string]any
	status := client.Get("/health", &result)
	assert.Equal(t, http.StatusOK, status, "health check should return 200")
}

func TestE2E_AgentCRUD(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	client := tenant.Client(t, cfg.backendURL)

	// Create agent
	createReq := map[string]any{
		"name":        "Test Agent",
		"description": "E2E test agent",
	}
	var created map[string]any
	status := client.Post("/api/agents", createReq, &created)
	require.Equal(t, http.StatusCreated, status, "create agent")
	require.NotNil(t, created["id"], "created agent should have id")

	agentID := created["id"].(string)

	// Get agent
	var fetched map[string]any
	status = client.Get(testutil.FormatURL("/api/agents/%s", agentID), &fetched)
	assert.Equal(t, http.StatusOK, status, "get agent")
	assert.Equal(t, "Test Agent", fetched["name"], "agent name should match")

	// List agents
	var page testutil.Page[map[string]any]
	status = client.Get("/api/agents?page=0&size=20", &page)
	assert.Equal(t, http.StatusOK, status, "list agents")
	assert.GreaterOrEqual(t, page.TotalElements, 1, "should have at least one agent")

	// Update agent
	updateReq := map[string]any{"name": "Updated Agent", "description": "Updated"}
	var updated map[string]any
	status = client.Put(testutil.FormatURL("/api/agents/%s", agentID), updateReq, &updated)
	assert.Equal(t, http.StatusOK, status, "update agent")

	// Delete agent
	status = client.Delete(testutil.FormatURL("/api/agents/%s", agentID))
	assert.Equal(t, http.StatusNoContent, status, "delete agent")

	// Verify deleted
	var notFound testutil.ErrorResponse
	status = client.Get(testutil.FormatURL("/api/agents/%s", agentID), &notFound)
	assert.Equal(t, http.StatusNotFound, status, "deleted agent should return 404")
}

func TestE2E_PipelineReadOnlyContract(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	client := tenant.Client(t, cfg.backendURL)

	// Pipelines are legacy read-only compatibility routes. The agentic runner is
	// the supported execution path, so writes must not be accepted.
	var page testutil.Page[map[string]any]
	status := client.Get("/api/pipelines?page=0&size=10", &page)
	assert.Equal(t, http.StatusOK, status, "list pipelines")

	req, err := http.NewRequest(http.MethodPost, cfg.backendURL+"/api/pipelines", strings.NewReader(`{"name":"legacy write"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+client.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "pipeline writes must remain disabled")
}

func TestE2E_KnowledgeBaseCRUD(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	client := tenant.Client(t, cfg.backendURL)

	// Create KB
	kbReq := map[string]any{"name": "Test KB", "description": "E2E test knowledge base"}
	var kb map[string]any
	status := client.Post("/api/knowledge-bases", kbReq, &kb)
	require.Equal(t, http.StatusCreated, status, "create knowledge base")

	kbID := kb["id"].(string)

	// List KBs
	var page testutil.Page[map[string]any]
	status = client.Get("/api/knowledge-bases?page=0&size=10", &page)
	assert.Equal(t, http.StatusOK, status, "list knowledge bases")
	assert.GreaterOrEqual(t, page.TotalElements, 1)

	// Delete
	status = client.Delete(testutil.FormatURL("/api/knowledge-bases/%s", kbID))
	assert.Equal(t, http.StatusNoContent, status, "delete knowledge base")
}
