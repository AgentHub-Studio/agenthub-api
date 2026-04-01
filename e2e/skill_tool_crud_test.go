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

// TestE2E_SkillToolCrud verifies the full lifecycle of Skill + Tool + binding.
func TestE2E_SkillToolCrud(t *testing.T) {
	cfg := e2eConfig()

	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	shortID := fmt.Sprintf("%d", time.Now().UnixNano())[:8]
	skillSlug := "e2e-skill-" + shortID

	// --- Create Skill ---
	var skill map[string]any
	status := c.Post("/api/skills", map[string]any{
		"name":        "E2E Test Skill",
		"slug":        skillSlug,
		"description": "Created by e2e test",
		"type":        "DATA",
		"category":    "DATA",
		"version":     "1.0.0",
	}, &skill)
	require.Equal(t, http.StatusCreated, status, "create skill")
	skillID := skill["id"].(string)
	t.Cleanup(func() { c.Delete("/api/skills/" + skillID) })

	t.Run("skill has correct fields after creation", func(t *testing.T) {
		assert.Equal(t, skillSlug, skill["slug"])
		assert.Equal(t, "ACTIVE", skill["status"])
	})

	// --- Create Tool ---
	var tool map[string]any
	status = c.Post("/api/tools", map[string]any{
		"name":        "E2E Test Tool",
		"description": "HTTP tool created by e2e test",
		"type":        "HTTP",
		"config": map[string]any{
			"url":    "https://httpbin.org/get",
			"method": "GET",
		},
	}, &tool)
	require.Equal(t, http.StatusCreated, status, "create tool")
	toolID := tool["id"].(string)
	t.Cleanup(func() { c.Delete("/api/tools/" + toolID) })

	t.Run("tool has correct type", func(t *testing.T) {
		assert.NotEmpty(t, toolID)
		assert.Equal(t, "HTTP", tool["type"])
	})

	// --- Bind Tool to Skill ---
	t.Run("bind tool to skill", func(t *testing.T) {
		var binding map[string]any
		status := c.Post("/api/skills/"+skillID+"/tools", map[string]any{
			"toolId":   toolID,
			"priority": 0,
		}, &binding)
		assert.True(t, status == http.StatusOK || status == http.StatusCreated,
			"bind tool: got %d", status)
	})

	// --- List Skills ---
	t.Run("list skills returns Page structure", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		status := c.Get("/api/skills?page=0&size=20", &page)
		assert.Equal(t, http.StatusOK, status)
		assert.GreaterOrEqual(t, page.TotalElements, 1)
		assert.Equal(t, 20, page.Size)

		found := false
		for _, s := range page.Content {
			if s["id"] == skillID {
				found = true
				break
			}
		}
		assert.True(t, found, "skill must appear in list")
	})

	// --- Get Skill by ID ---
	t.Run("get skill by id returns correct fields", func(t *testing.T) {
		var fetched map[string]any
		status := c.Get("/api/skills/"+skillID, &fetched)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, skillID, fetched["id"])
		assert.Equal(t, "1.0.0", fetched["version"])
		assert.Equal(t, "DATA", fetched["category"])
	})

	// --- Get Tool by ID ---
	t.Run("get tool by id returns correct fields", func(t *testing.T) {
		var fetched map[string]any
		status := c.Get("/api/tools/"+toolID, &fetched)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, toolID, fetched["id"])
		assert.Equal(t, "HTTP", fetched["type"])
	})

	// --- Delete Tool ---
	t.Run("delete tool returns 204", func(t *testing.T) {
		status := c.Delete("/api/tools/" + toolID)
		assert.True(t, status == http.StatusOK || status == http.StatusNoContent,
			"delete tool: got %d", status)
		toolID = "" // prevent double-delete in cleanup
	})

	// --- Delete Skill ---
	t.Run("delete skill returns 204", func(t *testing.T) {
		status := c.Delete("/api/skills/" + skillID)
		assert.True(t, status == http.StatusOK || status == http.StatusNoContent,
			"delete skill: got %d", status)
		skillID = "" // prevent double-delete in cleanup
	})

	// --- 404 after delete ---
	t.Run("deleted skill returns 404", func(t *testing.T) {
		// Create a fresh skill to delete
		ephemeralSlug := "e2e-del-" + shortID
		var s map[string]any
		require.Equal(t, http.StatusCreated, c.Post("/api/skills", map[string]any{
			"name": "To Delete", "slug": ephemeralSlug,
			"version": "1.0.0", "type": "DATA", "category": "DATA",
		}, &s))
		sid := s["id"].(string)

		delStatus := c.Delete("/api/skills/" + sid)
		assert.True(t, delStatus == http.StatusOK || delStatus == http.StatusNoContent)

		var notFound map[string]any
		status := c.Get("/api/skills/"+sid, &notFound)
		assert.Equal(t, http.StatusNotFound, status)
	})
}
