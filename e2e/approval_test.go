//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_ApprovalCRUD validates the full approval lifecycle:
// create → get → list → pending-count → approve → verify resolved.
func TestE2E_ApprovalCRUD(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Prerequisite: a published agent + execution to anchor the approval
	var agent map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/agents", map[string]any{
		"name":        "Approval E2E Agent",
		"description": "Agent for approval E2E tests",
	}, &agent))
	agentID := agent["id"].(string)
	t.Cleanup(func() { c.Delete("/api/agents/" + agentID) })

	// --- pending-count starts at zero ---
	t.Run("pending count is zero for new tenant", func(t *testing.T) {
		var resp map[string]any
		s := c.Get("/api/approvals/pending-count", &resp)
		assert.Equal(t, http.StatusOK, s)
		count, _ := resp["count"].(float64)
		assert.Equal(t, float64(0), count)
	})

	// --- Create approval ---
	createReq := map[string]any{
		"executionId": "00000000-0000-0000-0000-000000000001",
		"nodeId":      "node-deploy",
		"title":       "Deploy to production?",
		"description": "Please review and approve the deployment",
		"details":     "Version 1.2.3 — changelog: ...",
	}
	var approval map[string]any
	status := c.Post("/api/approvals", createReq, &approval)
	require.Equal(t, http.StatusCreated, status, "create approval")
	require.NotNil(t, approval["id"], "created approval must have id")

	approvalID := approval["id"].(string)
	t.Cleanup(func() { c.Delete("/api/approvals/" + approvalID) })

	t.Run("create approval fields", func(t *testing.T) {
		assert.Equal(t, "Deploy to production?", approval["title"])
		assert.Equal(t, "PENDING", approval["status"])
		assert.Equal(t, "node-deploy", approval["nodeId"])
		channels, _ := approval["notifyChannels"].([]any)
		assert.NotNil(t, channels, "notifyChannels must not be null")
	})

	// --- Get by ID ---
	t.Run("get approval by id", func(t *testing.T) {
		var fetched map[string]any
		s := c.Get("/api/approvals/"+approvalID, &fetched)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, approvalID, fetched["id"])
		assert.Equal(t, "PENDING", fetched["status"])
	})

	// --- List approvals ---
	t.Run("list approvals returns new approval", func(t *testing.T) {
		var page testutil.Page[map[string]any]
		s := c.Get("/api/approvals?page=0&size=20", &page)
		assert.Equal(t, http.StatusOK, s)
		assert.GreaterOrEqual(t, int64(page.TotalElements), int64(1))
		found := false
		for _, a := range page.Content {
			if a["id"] == approvalID {
				found = true
				break
			}
		}
		assert.True(t, found, "created approval must appear in list")
	})

	// --- Pending count increased ---
	t.Run("pending count reflects new approval", func(t *testing.T) {
		var resp map[string]any
		s := c.Get("/api/approvals/pending-count", &resp)
		assert.Equal(t, http.StatusOK, s)
		count, _ := resp["count"].(float64)
		assert.GreaterOrEqual(t, count, float64(1))
	})

	// --- Respond: approve ---
	t.Run("approve the pending approval", func(t *testing.T) {
		comment := "LGTM, deploy approved"
		var responded map[string]any
		s := c.Post("/api/approvals/"+approvalID+"/respond", map[string]any{
			"approved": true,
			"comment":  comment,
		}, &responded)
		assert.Equal(t, http.StatusOK, s)
		assert.Equal(t, "APPROVED", responded["status"])
		assert.NotNil(t, responded["respondedBy"])
		assert.NotNil(t, responded["respondedAt"])
		assert.Equal(t, comment, responded["comment"])
	})

	// --- Second respond must conflict ---
	t.Run("second respond returns 409", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Post("/api/approvals/"+approvalID+"/respond", map[string]any{
			"approved": false,
		}, &errResp)
		assert.Equal(t, http.StatusConflict, s, "already resolved approval must return 409")
	})

	// --- Get non-existent ---
	t.Run("get unknown approval returns 404", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Get("/api/approvals/00000000-0000-0000-0000-000000000099", &errResp)
		assert.Equal(t, http.StatusNotFound, s)
	})

	// --- Invalid ID format ---
	t.Run("invalid id returns 400", func(t *testing.T) {
		var errResp testutil.ErrorResponse
		s := c.Get("/api/approvals/not-a-uuid", &errResp)
		assert.Equal(t, http.StatusBadRequest, s)
	})

	// --- Delete ---
	t.Run("delete approval removes it", func(t *testing.T) {
		// Create a fresh approval to delete
		var toDelete map[string]any
		require.Equal(t, http.StatusCreated, c.Post("/api/approvals", map[string]any{
			"executionId": "00000000-0000-0000-0000-000000000010",
			"nodeId":      "node-to-delete",
			"title":       "Approval to be deleted",
		}, &toDelete))
		deleteID := toDelete["id"].(string)

		s := c.Delete("/api/approvals/" + deleteID)
		assert.Equal(t, http.StatusNoContent, s)

		// Must be gone
		var errResp testutil.ErrorResponse
		s = c.Get("/api/approvals/"+deleteID, &errResp)
		assert.Equal(t, http.StatusNotFound, s)
	})

	t.Run("delete unknown approval returns 404", func(t *testing.T) {
		s := c.Delete("/api/approvals/00000000-0000-0000-0000-000000000099")
		assert.Equal(t, http.StatusNotFound, s)
	})
}

// TestE2E_ApprovalReject validates the reject flow.
func TestE2E_ApprovalReject(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	var approval map[string]any
	require.Equal(t, http.StatusCreated, c.Post("/api/approvals", map[string]any{
		"executionId": "00000000-0000-0000-0000-000000000002",
		"nodeId":      "node-critical",
		"title":       "Dangerous operation — reject?",
	}, &approval))
	approvalID := approval["id"].(string)

	var responded map[string]any
	s := c.Post("/api/approvals/"+approvalID+"/respond", map[string]any{
		"approved": false,
		"comment":  "Not ready for production",
	}, &responded)
	assert.Equal(t, http.StatusOK, s)
	assert.Equal(t, "REJECTED", responded["status"])
	assert.NotNil(t, responded["respondedBy"])
}

// TestE2E_ApprovalTenantIsolation ensures approvals are isolated per tenant.
func TestE2E_ApprovalTenantIsolation(t *testing.T) {
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

	// Create approval as tenant A
	var approval map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/approvals", map[string]any{
		"executionId": "00000000-0000-0000-0000-000000000003",
		"nodeId":      "node-1",
		"title":       "TenantA exclusive approval",
	}, &approval))
	approvalID := approval["id"].(string)

	// Tenant B must not see tenant A's approval
	var errResp testutil.ErrorResponse
	s := cB.Get("/api/approvals/"+approvalID, &errResp)
	assert.Equal(t, http.StatusNotFound, s, "tenant B must not see tenant A approval")

	// Tenant B's list must not include tenant A's approval
	var page testutil.Page[map[string]any]
	cB.Get("/api/approvals?size=50", &page)
	for _, a := range page.Content {
		assert.NotEqual(t, approvalID, a["id"], "tenant B list must not include tenant A approval")
	}

	// Tenant B pending count must be 0
	var countResp map[string]any
	cB.Get("/api/approvals/pending-count", &countResp)
	count, _ := countResp["count"].(float64)
	assert.Equal(t, float64(0), count, "tenant B pending count must be 0")
}

// TestE2E_ApprovalPagination validates pagination behavior on the list endpoint.
func TestE2E_ApprovalPagination(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	// Create 5 approvals
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusCreated, c.Post("/api/approvals", map[string]any{
			"executionId": "00000000-0000-0000-0000-00000000000" + string(rune('1'+i)),
			"nodeId":      "node-" + string(rune('a'+i)),
			"title":       "Approval " + string(rune('A'+i)),
		}, nil))
	}

	// Page 0 size 3 → 3 items
	var page1 testutil.Page[map[string]any]
	s := c.Get("/api/approvals?page=0&size=3", &page1)
	assert.Equal(t, http.StatusOK, s)
	assert.Equal(t, 3, len(page1.Content))
	assert.GreaterOrEqual(t, int64(page1.TotalElements), int64(5))

	// Page 1 size 3 → remaining items
	var page2 testutil.Page[map[string]any]
	s = c.Get("/api/approvals?page=1&size=3", &page2)
	assert.Equal(t, http.StatusOK, s)
	assert.GreaterOrEqual(t, len(page2.Content), 2)
}
