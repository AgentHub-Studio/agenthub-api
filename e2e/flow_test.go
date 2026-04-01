//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// apiBaseURL returns the agenthub-api base URL (from env or default).
func apiBaseURL() string {
	if u := os.Getenv("API_URL"); u != "" {
		return u
	}
	return "http://localhost:8081"
}

func TestE2E_HealthCheck(t *testing.T) {
	client := testutil.NewAPIClient(t, apiBaseURL(), "")
	var result map[string]any
	status := client.Get("/health", &result)
	assert.Equal(t, http.StatusOK, status, "health check should return 200")
}

func TestE2E_AgentCRUD(t *testing.T) {
	if os.Getenv("E2E_TESTS") == "" {
		t.Skip("set E2E_TESTS=1 to run e2e tests")
	}
	ctx := context.Background()
	_ = ctx

	token := os.Getenv("AUTH_TOKEN")
	client := testutil.NewAPIClient(t, apiBaseURL(), token)

	// Create agent
	createReq := map[string]any{
		"name":        "Test Agent",
		"description": "E2E test agent",
		"status":      "DRAFT",
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

func TestE2E_PipelineCRUD(t *testing.T) {
	if os.Getenv("E2E_TESTS") == "" {
		t.Skip("set E2E_TESTS=1 to run e2e tests")
	}

	token := os.Getenv("AUTH_TOKEN")
	client := testutil.NewAPIClient(t, apiBaseURL(), token)

	// First create an agent
	agentReq := map[string]any{"name": "Pipeline Test Agent", "description": "For pipeline testing"}
	var agent map[string]any
	require.Equal(t, http.StatusCreated, client.Post("/api/agents", agentReq, &agent))
	agentID := agent["id"].(string)
	defer client.Delete(testutil.FormatURL("/api/agents/%s", agentID))

	// Create pipeline
	pipelineReq := map[string]any{
		"agentId": agentID,
		"name":    "Test Pipeline",
	}
	var pipeline map[string]any
	status := client.Post("/api/pipelines", pipelineReq, &pipeline)
	require.Equal(t, http.StatusCreated, status, "create pipeline")
	pipelineID := pipeline["id"].(string)

	// Add nodes
	inputNode := map[string]any{
		"pipelineId": pipelineID,
		"nodeId":     "input-1",
		"type":       "INPUT",
		"config":     map[string]any{},
	}
	var node map[string]any
	status = client.Post(testutil.FormatURL("/api/pipelines/%s/nodes", pipelineID), inputNode, &node)
	assert.Equal(t, http.StatusCreated, status, "create node")
}

func TestE2E_KnowledgeBaseCRUD(t *testing.T) {
	if os.Getenv("E2E_TESTS") == "" {
		t.Skip("set E2E_TESTS=1 to run e2e tests")
	}

	token := os.Getenv("AUTH_TOKEN")
	client := testutil.NewAPIClient(t, apiBaseURL(), token)

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
