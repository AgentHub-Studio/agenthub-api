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

func TestE2E_HealthCheck(t *testing.T) {
	client := testutil.NewAPIClient(t, e2eConfig().backendURL, "")
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
	client := testutil.NewAPIClient(t, e2eConfig().backendURL, token)

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
	client := testutil.NewAPIClient(t, e2eConfig().backendURL, token)

	// Create pipeline
	pipelineReq := map[string]any{"name": "Test Pipeline", "description": "E2E test pipeline"}
	var pipeline map[string]any
	status := client.Post("/api/pipelines", pipelineReq, &pipeline)
	require.Equal(t, http.StatusCreated, status, "create pipeline")
	pipelineID := pipeline["id"].(string)
	defer client.Delete(testutil.FormatURL("/api/pipelines/%s", pipelineID))

	// List pipelines
	var page testutil.Page[map[string]any]
	status = client.Get("/api/pipelines?page=0&size=10", &page)
	assert.Equal(t, http.StatusOK, status, "list pipelines")
	assert.GreaterOrEqual(t, page.TotalElements, int64(1))

	// Get pipeline
	var fetched map[string]any
	status = client.Get(testutil.FormatURL("/api/pipelines/%s", pipelineID), &fetched)
	assert.Equal(t, http.StatusOK, status, "get pipeline")
	assert.Equal(t, "Test Pipeline", fetched["name"])

	// Replace nodes via PUT /nodes
	nodes := []map[string]any{
		{"nodeType": "INPUT", "name": "Start", "config": map[string]any{}, "positionX": 0.0, "positionY": 0.0},
		{"nodeType": "OUTPUT", "name": "End", "config": map[string]any{}, "positionX": 300.0, "positionY": 0.0},
	}
	var nodeResps []map[string]any
	status = client.Put(testutil.FormatURL("/api/pipelines/%s/nodes", pipelineID), nodes, &nodeResps)
	require.Equal(t, http.StatusOK, status, "replace nodes")
	require.Len(t, nodeResps, 2)

	startID := nodeResps[0]["id"].(string)
	endID := nodeResps[1]["id"].(string)

	// Replace edges via PUT /edges
	edges := []map[string]any{
		{"sourceNodeId": startID, "targetNodeId": endID, "label": ""},
	}
	var edgeResps []map[string]any
	status = client.Put(testutil.FormatURL("/api/pipelines/%s/edges", pipelineID), edges, &edgeResps)
	assert.Equal(t, http.StatusOK, status, "replace edges")
	assert.Len(t, edgeResps, 1)

	// Get graph in frontend format
	var graph map[string]any
	status = client.Get(testutil.FormatURL("/api/pipelines/%s/graph", pipelineID), &graph)
	assert.Equal(t, http.StatusOK, status, "get graph")
	graphNodes, _ := graph["nodes"].([]any)
	graphEdges, _ := graph["edges"].([]any)
	assert.Len(t, graphNodes, 2, "graph should have 2 nodes")
	assert.Len(t, graphEdges, 1, "graph should have 1 edge")

	// Verify graph node has frontend-compatible format (type, label, position)
	firstNode := graphNodes[0].(map[string]any)
	assert.NotEmpty(t, firstNode["type"], "graph node should have type field")
	assert.NotNil(t, firstNode["position"], "graph node should have position field")

	// Update graph via PUT /graph
	graphReq := map[string]any{
		"nodes": []map[string]any{
			{
				"id": startID, "type": "INPUT", "label": "Start",
				"position": map[string]any{"x": 50.0, "y": 100.0},
				"config":   map[string]any{},
			},
			{
				"id": endID, "type": "OUTPUT", "label": "End",
				"position": map[string]any{"x": 400.0, "y": 100.0},
				"config":   map[string]any{},
			},
		},
		"edges": []map[string]any{
			{"id": "e1", "sourceNodeId": startID, "targetNodeId": endID},
		},
	}
	var updatedGraph map[string]any
	status = client.Put(testutil.FormatURL("/api/pipelines/%s/graph", pipelineID), graphReq, &updatedGraph)
	assert.Equal(t, http.StatusOK, status, "update graph")
	updatedNodes, _ := updatedGraph["nodes"].([]any)
	assert.Len(t, updatedNodes, 2, "updated graph should have 2 nodes")

	// Update pipeline metadata
	updateReq := map[string]any{"name": "Updated Pipeline", "status": "DRAFT"}
	var updated map[string]any
	status = client.Put(testutil.FormatURL("/api/pipelines/%s", pipelineID), updateReq, &updated)
	assert.Equal(t, http.StatusOK, status, "update pipeline")
	assert.Equal(t, "Updated Pipeline", updated["name"])
}

func TestE2E_KnowledgeBaseCRUD(t *testing.T) {
	if os.Getenv("E2E_TESTS") == "" {
		t.Skip("set E2E_TESTS=1 to run e2e tests")
	}

	token := os.Getenv("AUTH_TOKEN")
	client := testutil.NewAPIClient(t, e2eConfig().backendURL, token)

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
