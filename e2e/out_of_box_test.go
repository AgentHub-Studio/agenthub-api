//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// Out-of-the-box usability E2E: a freshly provisioned tenant must be able to
// open a chat and talk to an LLM without registering any agent/skill/tool.
//
//	go test -tags=e2e ./e2e/ -run TestE2E_FreshTenant

// pollRunStatus polls the run-status endpoint until the run reaches a terminal
// state (completed/failed/cancelled) or the deadline elapses.
func pollRunStatus(t *testing.T, c *testutil.APIClient, sessionID, runID string) string {
	t.Helper()
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		var st map[string]any
		if c.Get("/api/chat/sessions/"+sessionID+"/run/"+runID+"/status", &st) == http.StatusOK {
			switch s, _ := st["status"].(string); s {
			case "completed", "failed", "cancelled":
				return s
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("run %s did not reach a terminal state within the deadline", runID)
	return ""
}

// TestE2E_FreshTenant_DefaultAgentListed verifies a fresh tenant ships with the
// auto-seeded default assistant, published and ready to use.
func TestE2E_FreshTenant_DefaultAgentListed(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t, cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass, cfg.e2eUserPassword)
	c := tenant.Client(t, cfg.backendURL)

	var page testutil.Page[map[string]any]
	require.Equal(t, http.StatusOK, c.Get("/api/agents?size=50", &page))

	var found map[string]any
	for _, a := range page.Content {
		if a["slug"] == "agenthub-assistant" {
			found = a
			break
		}
	}
	require.NotNil(t, found, "a fresh tenant must ship with the agenthub-assistant default agent")
	assert.Equal(t, "PUBLISHED", found["status"])
}

// TestE2E_FreshTenant_OnboardingStatus verifies the onboarding status endpoint
// reports an incomplete checklist for a fresh tenant, with connect-llm pending.
func TestE2E_FreshTenant_OnboardingStatus(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t, cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass, cfg.e2eUserPassword)
	c := tenant.Client(t, cfg.backendURL)

	var status map[string]any
	require.Equal(t, http.StatusOK, c.Get("/api/core/onboarding/status", &status))

	assert.Equal(t, false, status["completed"], "a fresh tenant has not completed onboarding")
	steps, ok := status["steps"].([]any)
	require.True(t, ok, "status must carry a steps array")
	require.NotEmpty(t, steps, "the onboarding checklist must have steps")

	var connectLLM map[string]any
	for _, s := range steps {
		if m, _ := s.(map[string]any); m["slug"] == "onboarding-connect-llm" {
			connectLLM = m
			break
		}
	}
	require.NotNil(t, connectLLM, "onboarding-connect-llm step must be present")
	assert.Equal(t, false, connectLLM["done"], "connect-llm must not be done on a fresh tenant")
}

// TestE2E_FreshTenant_ChatWithoutAgentId_NoKey is the primary out-of-the-box
// assertion: a fresh tenant can create a chat session WITHOUT picking an agent
// and start a run. With no LLM key configured the run fails — but it must fail
// gracefully, with a friendly assistant message in the chat history rather than
// a raw error or a 410 Gone. This path needs no LLM credentials, so it always runs.
func TestE2E_FreshTenant_ChatWithoutAgentId_NoKey(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t, cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass, cfg.e2eUserPassword)
	c := tenant.Client(t, cfg.backendURL)

	// Create a session WITHOUT an agentId.
	var session map[string]any
	require.Equal(t, http.StatusCreated,
		c.Post("/api/chat/sessions", map[string]any{"title": "OOB no-key"}, &session))
	sessionID := session["id"].(string)
	assert.Nil(t, session["agentId"], "session created without agentId")
	t.Cleanup(func() { c.Delete("/api/chat/sessions/" + sessionID) })

	// Start a run — the router must bind the default assistant; enqueue must
	// succeed (202), NOT fail with 410 Gone.
	var run map[string]any
	require.Equal(t, http.StatusAccepted,
		c.Post("/api/chat/sessions/"+sessionID+"/run", map[string]any{"message": "olá"}, &run),
		"an agentless run must enqueue, not 410")
	runID := run["runId"].(string)

	assert.Equal(t, "failed", pollRunStatus(t, c, sessionID, runID),
		"without an LLM key configured the run fails")

	// The failure must surface as a friendly assistant message — not an empty history.
	var msgs testutil.Page[map[string]any]
	require.Equal(t, http.StatusOK,
		c.Get("/api/chat/sessions/"+sessionID+"/messages?size=50", &msgs))
	var assistantMsg string
	for _, m := range msgs.Content {
		if m["role"] == "assistant" {
			assistantMsg, _ = m["content"].(string)
		}
	}
	require.NotEmpty(t, assistantMsg, "a friendly assistant message must be persisted on failure")
	assert.Contains(t, strings.ToLower(assistantMsg), "chave de api",
		"the failure message must guide the user to configure an API key")
}

// TestE2E_FreshTenant_ChatWithoutAgentId_WithKey verifies the full happy path:
// with an LLM key configured, an agentless chat session runs to completion.
// Requires a real OpenRouter key in E2E_LLM_API_KEY; skipped when absent.
func TestE2E_FreshTenant_ChatWithoutAgentId_WithKey(t *testing.T) {
	apiKey := os.Getenv("E2E_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("E2E_LLM_API_KEY not set — skipping the keyed happy-path run")
	}
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t, cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass, cfg.e2eUserPassword)
	c := tenant.Client(t, cfg.backendURL)

	// Configure the OpenRouter API key — the only manual out-of-the-box step.
	var ignored map[string]any
	putStatus := c.Put("/api/settings/openrouter.apiKey", map[string]any{"value": apiKey}, &ignored)
	require.True(t, putStatus == http.StatusOK || putStatus == http.StatusCreated,
		"configuring the provider key must succeed (got %d)", putStatus)

	var session map[string]any
	require.Equal(t, http.StatusCreated,
		c.Post("/api/chat/sessions", map[string]any{"title": "OOB keyed"}, &session))
	sessionID := session["id"].(string)
	t.Cleanup(func() { c.Delete("/api/chat/sessions/" + sessionID) })

	var run map[string]any
	require.Equal(t, http.StatusAccepted,
		c.Post("/api/chat/sessions/"+sessionID+"/run",
			map[string]any{"message": "Responda apenas com a palavra: pronto"}, &run))
	runID := run["runId"].(string)

	assert.Equal(t, "completed", pollRunStatus(t, c, sessionID, runID),
		"with a valid LLM key configured the agentless run completes")
}
