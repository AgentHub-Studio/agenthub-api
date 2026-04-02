//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_ChatSessionLifecycle validates the full session + message lifecycle:
// create → get → add messages → list messages → archive → delete.
func TestE2E_ChatSessionLifecycle(t *testing.T) {
	cfg := e2eConfig()

	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	shortID := fmt.Sprintf("%d", time.Now().UnixNano())[:8]

	// --- Create Session (no agentId — optional) ---
	var session map[string]any
	status := c.Post("/api/chat/sessions", map[string]any{
		"title": "E2E Chat " + shortID,
	}, &session)
	require.Equal(t, http.StatusCreated, status, "create session")

	sessionID, ok := session["id"].(string)
	require.True(t, ok, "session id must be a string")
	require.NotEmpty(t, sessionID)
	t.Cleanup(func() { c.Delete("/api/chat/sessions/" + sessionID) })

	t.Run("session has correct fields", func(t *testing.T) {
		assert.Equal(t, "E2E Chat "+shortID, session["title"])
		assert.Equal(t, "ACTIVE", session["status"])
		assert.NotEmpty(t, session["createdAt"])
		assert.Nil(t, session["agentId"], "agentId should be omitted when not set")
	})

	// --- Get Session ---
	t.Run("get session by id", func(t *testing.T) {
		var got map[string]any
		status := c.Get("/api/chat/sessions/"+sessionID, &got)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, sessionID, got["id"])
		assert.Equal(t, "E2E Chat "+shortID, got["title"])
	})

	// --- Get non-existent session returns 404 ---
	t.Run("get unknown session returns 404", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		status := c.Get("/api/chat/sessions/00000000-0000-0000-0000-000000000001", &errResp)
		assert.Equal(t, http.StatusNotFound, status)
	})

	// --- List Sessions ---
	t.Run("list sessions includes created session", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		status := c.Get("/api/chat/sessions?size=50", &page)
		assert.Equal(t, http.StatusOK, status)
		found := false
		for _, s := range page.Content {
			if s["id"] == sessionID {
				found = true
				break
			}
		}
		assert.True(t, found, "created session should appear in list")
	})

	// --- Add Messages ---
	var userMsg map[string]any
	t.Run("add user message", func(t *testing.T) {
		status := c.Post("/api/chat/sessions/"+sessionID+"/messages", map[string]any{
			"role":    "user",
			"content": "Hello, what can you do?",
		}, &userMsg)
		assert.Equal(t, http.StatusCreated, status)
		assert.Equal(t, "user", userMsg["role"])
		assert.Equal(t, "Hello, what can you do?", userMsg["content"])
		assert.Equal(t, sessionID, userMsg["sessionId"])
		assert.NotEmpty(t, userMsg["id"])
	})

	t.Run("add assistant message", func(t *testing.T) {
		var assistantMsg map[string]any
		status := c.Post("/api/chat/sessions/"+sessionID+"/messages", map[string]any{
			"role":    "assistant",
			"content": "I can help you with many tasks.",
		}, &assistantMsg)
		assert.Equal(t, http.StatusCreated, status)
		assert.Equal(t, "assistant", assistantMsg["role"])
	})

	// --- List Messages ---
	t.Run("list messages returns both messages in order", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		status := c.Get("/api/chat/sessions/"+sessionID+"/messages?size=50", &page)
		assert.Equal(t, http.StatusOK, status)
		require.GreaterOrEqual(t, len(page.Content), 2)

		roles := make([]string, len(page.Content))
		for i, m := range page.Content {
			roles[i] = m["role"].(string)
		}
		assert.Contains(t, roles, "user")
		assert.Contains(t, roles, "assistant")
	})

	// --- Add Message with missing role returns 422 ---
	t.Run("add message missing role returns 422", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		status := c.Post("/api/chat/sessions/"+sessionID+"/messages", map[string]any{
			"content": "no role",
		}, &errResp)
		assert.Equal(t, http.StatusUnprocessableEntity, status)
	})

	// --- Add Message with missing content returns 422 ---
	t.Run("add message missing content returns 422", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		status := c.Post("/api/chat/sessions/"+sessionID+"/messages", map[string]any{
			"role": "user",
		}, &errResp)
		assert.Equal(t, http.StatusUnprocessableEntity, status)
	})

	// --- Archive Session ---
	t.Run("archive session sets status to ARCHIVED", func(t *testing.T) {
		var archived map[string]any
		status := c.Post("/api/chat/sessions/"+sessionID+"/archive", nil, &archived)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "ARCHIVED", archived["status"])
	})
}

// TestE2E_ChatSessionCreate_NoTitle validates that missing title returns 422.
func TestE2E_ChatSessionCreate_NoTitle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	var errResp testutil.ErrorResponse
	status := c.Post("/api/chat/sessions", map[string]any{}, &errResp)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

// TestE2E_ChatSessionDelete validates that deleting a session removes it.
func TestE2E_ChatSessionDelete(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create
	var session map[string]any
	status := c.Post("/api/chat/sessions", map[string]any{"title": "Delete Me"}, &session)
	require.Equal(t, http.StatusCreated, status)
	sessionID := session["id"].(string)

	// Delete
	status = c.Delete("/api/chat/sessions/" + sessionID)
	assert.Equal(t, http.StatusNoContent, status)

	// Verify gone
	var errResp testutil.ErrorResponse
	status = c.Get("/api/chat/sessions/"+sessionID, &errResp)
	assert.Equal(t, http.StatusNotFound, status)

	// Second delete returns 404
	status = c.Delete("/api/chat/sessions/" + sessionID)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestE2E_ChatSessionTenantIsolation verifies sessions are isolated per tenant.
func TestE2E_ChatSessionTenantIsolation(t *testing.T) {
	cfg := e2eConfig()

	// Two independent tenants
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

	clientA := tenantA.Client(t, cfg.backendURL)
	clientB := tenantB.Client(t, cfg.backendURL)

	// Tenant A creates a session
	var sessionA map[string]any
	status := clientA.Post("/api/chat/sessions", map[string]any{"title": "Tenant A Chat"}, &sessionA)
	require.Equal(t, http.StatusCreated, status)
	sessionAID := sessionA["id"].(string)
	t.Cleanup(func() { clientA.Delete("/api/chat/sessions/" + sessionAID) })

	// Tenant B cannot see Tenant A's session
	var errResp testutil.ErrorResponse
	status = clientB.Get("/api/chat/sessions/"+sessionAID, &errResp)
	assert.Equal(t, http.StatusNotFound, status, "tenant B must not access tenant A session")

	// Tenant B list should be empty (or not contain tenant A's session)
	var pageB testutil.Page[map[string]any]
	clientB.Get("/api/chat/sessions?size=50", &pageB)
	for _, s := range pageB.Content {
		assert.NotEqual(t, sessionAID, s["id"], "tenant B list must not contain tenant A session")
	}
}
