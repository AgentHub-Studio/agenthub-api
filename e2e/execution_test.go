//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_ExecutionLifecycle validates starting an execution and reading its state.
// The execution is started against a published agent; it will immediately enter a
// FAILED or RUNNING state since no real orchestrator is connected in E2E — the test
// focuses on the API contract (create → get → list → cancel).
func TestE2E_ExecutionLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Prerequisite: create and publish an agent
	var agent map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/agents", map[string]any{
		"name":        "E2E Execution Agent",
		"description": "Agent for execution E2E tests",
	}, &agent))
	agentID := agent["id"].(string)
	t.Cleanup(func() { c.Delete("/api/agents/" + agentID) })

	// Publish agent so executions can be started
	var published map[string]any
	c.Post("/api/agents/"+agentID+"/publish", nil, &published)

	// --- Start execution ---
	input, _ := json.Marshal(map[string]any{"query": "hello from e2e"})
	var exec map[string]any
	status := c.Post("/api/executions", map[string]any{
		"agentId": agentID,
		"input":   json.RawMessage(input),
	}, &exec)
	require.Equal(t, http.StatusCreated, status, "start execution")
	execID := exec["id"].(string)
	t.Cleanup(func() { c.Delete("/api/executions/" + execID) })

	t.Run("execution has correct initial fields", func(t *testing.T) {
		assert.NotEmpty(t, execID)
		assert.Equal(t, agentID, exec["agentId"])
		// Status should be one of: PENDING, RUNNING, FAILED (no orchestrator in E2E)
		status, _ := exec["status"].(string)
		assert.Contains(t, []string{"PENDING", "RUNNING", "FAILED", "QUEUED"}, status)
	})

	// --- Get execution by ID ---
	t.Run("get execution by id", func(t *testing.T) {
		var fetched map[string]any
		s := c.Get("/api/executions/"+execID, &fetched)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, execID, fetched["id"])
		assert.Equal(t, agentID, fetched["agentId"])
	})

	// --- List executions for agent ---
	t.Run("list executions for agent", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/executions?agentId="+agentID+"&size=20", &page)
		assert.Equal(t, http.StatusOK, s)
		found := false
		for _, e := range page.Content {
			if e["id"] == execID {
				found = true
				break
			}
		}
		assert.True(t, found, "started execution must appear in list")
	})

	// --- List execution nodes (may be empty if execution hasn't started) ---
	t.Run("list execution nodes returns ok", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/executions/"+execID+"/nodes", &page)
		assert.Equal(t, http.StatusOK, s)
	})

	// --- Cancel execution ---
	t.Run("cancel execution", func(t *testing.T) {
		s := c.Delete("/api/executions/" + execID)
		// Accept 200 (cancelled) or 409 (already in terminal state)
		assert.True(t, s == http.StatusOK || s == http.StatusConflict || s == http.StatusNoContent,
			"cancel must return 2xx or 409, got %d", s)
	})
}

// TestE2E_ExecutionList validates listing and filtering executions.
func TestE2E_ExecutionList(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// List all executions (initially empty for new tenant)
	var page testutil.Page[map[string]any]
	s := c.Get("/api/executions?size=20", &page)
	assert.Equal(t, http.StatusOK, s)
	// Content slice should be initialized (not nil), even if empty
	assert.NotNil(t, page.Content)
}

// TestE2E_ExecutionTenantIsolation verifies executions are isolated per tenant.
func TestE2E_ExecutionTenantIsolation(t *testing.T) {
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

	// Create and publish agent for tenant A
	var agent map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/agents", map[string]any{
		"name": "TenantA Agent",
	}, &agent))
	agentID := agent["id"].(string)
	t.Cleanup(func() { cA.Delete("/api/agents/" + agentID) })
	cA.Post("/api/agents/"+agentID+"/publish", nil, nil)

	// Start execution as tenant A
	input, _ := json.Marshal(map[string]any{"query": "hello"})
	var exec map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/executions", map[string]any{
		"agentId": agentID,
		"input":   json.RawMessage(input),
	}, &exec))
	execID := exec["id"].(string)
	t.Cleanup(func() { cA.Delete("/api/executions/" + execID) })

	// Tenant B must not see tenant A's execution
	var errResp testutil.ErrorResponse
	s := cB.Get("/api/executions/"+execID, &errResp)
	assert.Equal(t, http.StatusNotFound, s, "tenant B must not see tenant A execution")
}
