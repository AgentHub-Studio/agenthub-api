//go:build e2e

package e2e

import (
	"bytes"
	"io"
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
// and the local disposable stack routes it to the published default assistant.
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

	// The disposable stack has no RabbitMQ, so this endpoint streams synchronously
	// using its fake OpenAI-compatible fallback rather than returning an async 202.
	req, err := http.NewRequest(http.MethodPost,
		cfg.backendURL+"/api/chat/sessions/"+sessionID+"/run",
		bytes.NewBufferString(`{"message":"olá"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "event: run_complete")

	var routed map[string]any
	require.Equal(t, http.StatusOK, c.Get("/api/chat/sessions/"+sessionID, &routed))
	assert.NotNil(t, routed["agentId"], "agentless run must bind the default assistant")
}

// TestE2E_FreshTenant_OpenRouterRequiredModel_WithKey verifies the full happy path:
// with an LLM key configured, a chat run completes through the model required
// by the canonical functional test plan. Requires a real OpenRouter key in
// E2E_LLM_API_KEY; skipped when absent.
func TestE2E_FreshTenant_OpenRouterRequiredModel_WithKey(t *testing.T) {
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

	const requiredOpenRouterModel = "openai/gpt-oss-120b"
	var agent map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/agents", map[string]any{
		"name":         "E2E OpenRouter Required Model",
		"slug":         "e2e-openrouter-required-model",
		"description":  "Disposable agent for the keyed OpenRouter contract.",
		"systemPrompt": "Reply with exactly the requested answer.",
		"modelConfig": map[string]any{
			"provider": "openrouter",
			"model":    requiredOpenRouterModel,
		},
	}, &agent))
	agentID := agent["id"].(string)
	t.Cleanup(func() { c.Delete("/api/agents/" + agentID) })
	require.Equal(t, http.StatusOK, c.Post("/api/agents/"+agentID+"/publish", nil, nil))

	var persistedAgent map[string]any
	require.Equal(t, http.StatusOK, c.Get("/api/agents/"+agentID, &persistedAgent))
	persistedModelConfig, ok := persistedAgent["modelConfig"].(map[string]any)
	require.True(t, ok, "the selected agent must expose its sanitized model config")
	assert.Equal(t, "openrouter", persistedModelConfig["provider"])
	assert.Equal(t, requiredOpenRouterModel, persistedModelConfig["model"])

	var session map[string]any
	require.Equal(t, http.StatusCreated,
		c.Post("/api/chat/sessions", map[string]any{
			"title":   "OOB keyed",
			"agentId": agentID,
		}, &session))
	sessionID := session["id"].(string)
	assert.Equal(t, agentID, session["agentId"], "the run must be bound to the configured OpenRouter agent")
	t.Cleanup(func() { c.Delete("/api/chat/sessions/" + sessionID) })

	// The disposable E2E stack deliberately has no RabbitMQ. The public
	// contract is therefore the synchronous SSE stream, which proves that the
	// real provider response completed instead of merely accepting a queued run.
	req, err := http.NewRequest(http.MethodPost,
		cfg.backendURL+"/api/chat/sessions/"+sessionID+"/run",
		bytes.NewBufferString(`{"message":"Responda apenas com a palavra: pronto"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	stream := strings.ToLower(string(body))
	assert.Contains(t, stream, "event: text_delta")
	assert.Contains(t, stream, "pronto")
	assert.Contains(t, stream, "event: run_complete")
}
