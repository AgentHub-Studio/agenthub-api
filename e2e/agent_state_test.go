//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_AgentPublishArchiveClone validates agent state transitions and clone.
func TestE2E_AgentPublishArchiveClone(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create agent
	var agent map[string]any
	status := c.Post("/api/agents", map[string]any{
		"name":        "E2E State Agent",
		"description": "For state transition tests",
		"systemPrompt": "You are an E2E state-transition test agent.",
		"modelConfig": map[string]any{
			"provider": "openai",
			"model":    "gpt-4o",
		},
	}, &agent)
	require.Equal(t, http.StatusCreated, status)
	agentID := agent["id"].(string)
	t.Cleanup(func() { c.Delete("/api/agents/" + agentID) })

	assert.Equal(t, "DRAFT", agent["status"], "new agent must start as DRAFT")

	// --- Publish ---
	t.Run("publish draft agent", func(t *testing.T) {
		var published map[string]any
		s := c.Post("/api/agents/"+agentID+"/publish", nil, &published)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "PUBLISHED", published["status"])
	})

	// --- Archive ---
	t.Run("archive published agent", func(t *testing.T) {
		var archived map[string]any
		s := c.Post("/api/agents/"+agentID+"/archive", nil, &archived)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "ARCHIVED", archived["status"])
	})

	// --- Clone from archived agent ---
	t.Run("clone agent creates new DRAFT", func(t *testing.T) {
		var cloned map[string]any
		s := c.Post("/api/agents/"+agentID+"/clone", map[string]any{
			"name": "E2E State Agent Clone",
		}, &cloned)
		assert.Equal(t, http.StatusCreated, s)
		clonedID, _ := cloned["id"].(string)
		assert.NotEmpty(t, clonedID)
		assert.NotEqual(t, agentID, clonedID, "clone must have a new ID")
		assert.Equal(t, "DRAFT", cloned["status"], "cloned agent starts as DRAFT")
		t.Cleanup(func() { c.Delete("/api/agents/" + clonedID) })
	})
}

// TestE2E_KnowledgeBaseActivatePause validates KB status transitions.
func TestE2E_KnowledgeBaseActivatePause(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create KB
	var kb map[string]any
	status := c.Post("/api/knowledge-bases", map[string]any{
		"name":        "E2E Status KB",
		"description": "For activate/pause tests",
	}, &kb)
	require.Equal(t, http.StatusCreated, status)
	kbID := kb["id"].(string)
	t.Cleanup(func() { c.Delete("/api/knowledge-bases/" + kbID) })

	assert.Equal(t, "ACTIVE", kb["status"], "new knowledge base must start as ACTIVE")

	// --- Pause ---
	t.Run("pause active knowledge base", func(t *testing.T) {
		var paused map[string]any
		s := c.Post("/api/knowledge-bases/"+kbID+"/pause", nil, &paused)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "PAUSED", paused["status"])
	})

	// --- Activate ---
	t.Run("activate paused knowledge base", func(t *testing.T) {
		var activated map[string]any
		s := c.Post("/api/knowledge-bases/"+kbID+"/activate", nil, &activated)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "ACTIVE", activated["status"])
	})
}
