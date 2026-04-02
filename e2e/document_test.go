//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_DocumentUpload validates the full document lifecycle:
// create knowledge base → upload document → verify PENDING status → delete.
func TestE2E_DocumentUpload(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)

	shortID := fmt.Sprintf("%d", time.Now().UnixNano())[:8]

	// --- Create Knowledge Base ---
	var kb map[string]any
	status := c.Post("/api/knowledge-bases", map[string]any{
		"name":        "E2E KB " + shortID,
		"description": "Created by e2e document test",
	}, &kb)
	require.Equal(t, http.StatusCreated, status, "create knowledge base")
	kbID := kb["id"].(string)
	t.Cleanup(func() { c.Delete("/api/knowledge-bases/" + kbID) })

	t.Run("knowledge base has correct status", func(t *testing.T) {
		assert.Equal(t, "ACTIVE", kb["status"])
		assert.NotEmpty(t, kb["id"])
	})

	// --- Upload document via multipart ---
	t.Run("upload PDF document returns PENDING", func(t *testing.T) {
		body, contentType := buildMultipartUpload(t, "test-document.pdf", "application/pdf",
			[]byte("%PDF-1.4 fake pdf content for e2e testing"))

		req, err := http.NewRequest(http.MethodPost,
			cfg.backendURL+"/api/knowledge-bases/"+kbID+"/documents", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", tenant.BearerToken())
		req.Header.Set("Content-Type", contentType)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "upload should return 201")

		var doc map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))

		assert.NotEmpty(t, doc["id"])
		assert.Equal(t, "test-document.pdf", doc["fileName"])
		assert.Equal(t, "application/pdf", doc["contentType"])
		assert.Equal(t, "PENDING", doc["status"],
			"newly uploaded document should be PENDING until extractor processes it")
		assert.Equal(t, kbID, doc["knowledgeBaseId"])

		docID := doc["id"].(string)
		t.Cleanup(func() { c.Delete("/api/knowledge-bases/" + kbID + "/documents/" + docID) })

		// --- Get document ---
		t.Run("get document by id returns same data", func(t *testing.T) {
			var fetched map[string]any
			status := c.Get("/api/knowledge-bases/"+kbID+"/documents/"+docID, &fetched)
			assert.Equal(t, http.StatusOK, status)
			assert.Equal(t, docID, fetched["id"])
			assert.Equal(t, "PENDING", fetched["status"])
		})

		// --- List documents ---
		t.Run("list documents shows uploaded document", func(t *testing.T) {
			var page testutil.Page[map[string]any]
			status := c.Get("/api/knowledge-bases/"+kbID+"/documents?size=50", &page)
			assert.Equal(t, http.StatusOK, status)
			found := false
			for _, d := range page.Content {
				if d["id"] == docID {
					found = true
					break
				}
			}
			assert.True(t, found, "uploaded document must appear in list")
		})
	})
}

// TestE2E_DocumentUpload_UnknownKB validates that uploading to a non-existent KB returns 404.
func TestE2E_DocumentUpload_UnknownKB(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)

	body, contentType := buildMultipartUpload(t, "file.txt", "text/plain", []byte("hello"))

	req, err := http.NewRequest(http.MethodPost,
		cfg.backendURL+"/api/knowledge-bases/00000000-0000-0000-0000-000000000001/documents", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", tenant.BearerToken())
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_DocumentTenantIsolation verifies documents are isolated per tenant.
func TestE2E_DocumentTenantIsolation(t *testing.T) {
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

	// Tenant A creates a KB
	var kb map[string]any
	require.Equal(t, http.StatusCreated, cA.Post("/api/knowledge-bases", map[string]any{
		"name": "Isolated KB",
	}, &kb))
	kbID := kb["id"].(string)
	t.Cleanup(func() { cA.Delete("/api/knowledge-bases/" + kbID) })

	// Tenant B cannot access Tenant A's KB
	var errResp testutil.ErrorResponse
	status := cB.Get("/api/knowledge-bases/"+kbID, &errResp)
	assert.Equal(t, http.StatusNotFound, status, "tenant B must not see tenant A knowledge base")
}

// ── helpers ──────────────────────────────────────────────────────────────────

// buildMultipartUpload creates a multipart form body with a single "file" field.
func buildMultipartUpload(t *testing.T, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", contentType)

	part, err := w.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	return &buf, w.FormDataContentType()
}
