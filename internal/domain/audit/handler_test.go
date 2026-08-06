package audit_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockAuditSvc satisfies the private auditService interface in audit.Handler.
type mockAuditSvc struct {
	logs       map[uuid.UUID]audit.AuditLog
	lastTenant string
	lastFilter audit.ListFilter
	lastPage   pagination.PageRequest
}

func newMockAuditSvc() *mockAuditSvc {
	return &mockAuditSvc{logs: make(map[uuid.UUID]audit.AuditLog)}
}

func (m *mockAuditSvc) ListAll(_ context.Context, tenantID string, f audit.ListFilter, pr pagination.PageRequest) ([]audit.AuditLog, int, error) {
	m.lastTenant = tenantID
	m.lastFilter = f
	m.lastPage = pr
	items := make([]audit.AuditLog, 0, len(m.logs))
	for _, l := range m.logs {
		items = append(items, l)
	}
	return items, len(items), nil
}

func (m *mockAuditSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (audit.AuditLog, error) {
	l, ok := m.logs[id]
	if !ok {
		return audit.AuditLog{}, audit.ErrNotFound
	}
	return l, nil
}

func (m *mockAuditSvc) Record(_ context.Context, _ string, req audit.RecordRequest) (audit.AuditLog, error) {
	id := uuid.New()
	l := audit.AuditLog{
		ID:         id,
		EntityType: req.EntityType,
		Action:     req.Action,
	}
	m.logs[id] = l
	return l, nil
}

func (m *mockAuditSvc) ApplyRetention(_ context.Context, tenantID string, req audit.AuditRetentionRequest, now time.Time) (audit.AuditRetentionResponse, error) {
	m.lastTenant = tenantID
	retentionDays := req.RetentionDays
	if retentionDays == 0 {
		retentionDays = audit.DefaultAuditRetentionDays
	}
	if retentionDays < audit.MinAuditRetentionDays {
		return audit.AuditRetentionResponse{}, audit.ErrValidation
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	deleted := 0
	for id, l := range m.logs {
		if l.CreatedAt.Before(cutoff) {
			deleted++
			if !req.DryRun {
				delete(m.logs, id)
			}
		}
	}
	return audit.AuditRetentionResponse{
		TenantID:      tenantID,
		RetentionDays: retentionDays,
		Cutoff:        cutoff,
		DryRun:        req.DryRun,
		Deleted:       deleted,
	}, nil
}

func setupAudit() (*chi.Mux, *mockAuditSvc) {
	svc := newMockAuditSvc()
	h := audit.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			ctx = middleware.ContextWithRoles(ctx, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/audit", h.Routes())
	return r, svc
}

func TestAuditHandler_List_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{ID: id, EntityType: "agent", Action: audit.AuditActionCreate}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[audit.AuditLogResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestAuditHandler_List_Empty(t *testing.T) {
	r, _ := setupAudit()
	req := httptest.NewRequest(http.MethodGet, "/api/audit/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditHandler_GetByID_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{ID: id, EntityType: "skill", Action: audit.AuditActionUpdate}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp audit.AuditLogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, id, resp.ID)
}

func TestAuditHandler_PublicSurfacesRedactSensitiveAuditValues(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	const apiKeySecret = "audit-http-api-key-sentinel"
	const authorizationSecret = "Bearer audit-http-authorization-sentinel"
	const refreshTokenSecret = "audit-http-refresh-token-sentinel"
	svc.logs[id] = audit.AuditLog{
		ID:         id,
		EntityType: "agent",
		EntityID:   "agent-redaction",
		Action:     audit.AuditActionUpdate,
		OldValue:   `{"apiKey":"audit-http-api-key-sentinel","before":true}`,
		NewValue:   `{"headers":{"Authorization":"Bearer audit-http-authorization-sentinel"},"after":true}`,
		Metadata:   `{"nested":{"refresh_token":"audit-http-refresh-token-sentinel"},"diagnostic":"Authorization: Bearer audit-http-authorization-sentinel","source":"admin"}`,
		CreatedAt:  time.Now().UTC(),
	}

	tests := []struct {
		name   string
		path   string
		unpack []string
	}{
		{name: "list", path: "/api/audit/"},
		{name: "detail", path: "/api/audit/" + id.String()},
		{name: "json export", path: "/api/audit/export?format=json"},
		{name: "csv export", path: "/api/audit/export?format=csv"},
		{name: "pdf export", path: "/api/audit/export?format=pdf"},
		{name: "zip export", path: "/api/audit/export?format=zip_bundle", unpack: []string{"audit-logs.json", "audit-logs.csv"}},
		{name: "xlsx export", path: "/api/audit/export?format=xlsx", unpack: []string{"xl/worksheets/sheet1.xml"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			bodies := [][]byte{w.Body.Bytes()}
			if len(tc.unpack) > 0 {
				zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
				require.NoError(t, err)
				bodies = make([][]byte, 0, len(tc.unpack))
				for _, name := range tc.unpack {
					bodies = append(bodies, zipEntry(t, zr, name))
				}
			}

			for _, body := range bodies {
				text := string(body)
				assert.NotContains(t, text, apiKeySecret)
				assert.NotContains(t, text, authorizationSecret)
				assert.NotContains(t, text, refreshTokenSecret)
			}
		})
	}
}

func TestAuditHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupAudit()
	req := httptest.NewRequest(http.MethodGet, "/api/audit/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuditHandler_GetByID_InvalidID(t *testing.T) {
	r, _ := setupAudit()
	req := httptest.NewRequest(http.MethodGet, "/api/audit/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuditHandler_ExportJSON_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{
		ID:         id,
		EntityType: "a2a_invoke",
		EntityID:   "agent-123",
		Action:     audit.AuditActionExecute,
		ActorID:    "tenant-a",
		Metadata:   `{"sourceTenant":"tenant-a","targetTenant":"tenant-b"}`,
		CreatedAt:  time.Date(2026, 6, 23, 3, 30, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/export?entityType=a2a_invoke&action=EXECUTE&size=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test-tenant", svc.lastTenant)
	assert.Equal(t, "a2a_invoke", svc.lastFilter.EntityType)
	assert.Equal(t, "EXECUTE", svc.lastFilter.Action)
	assert.Equal(t, 100, svc.lastPage.Size)

	var resp struct {
		TenantID      string                   `json:"tenantId"`
		Format        string                   `json:"format"`
		TotalElements int64                    `json:"totalElements"`
		Records       []audit.AuditLogResponse `json:"records"`
		GeneratedAt   time.Time                `json:"generatedAt"`
		Integrity     struct {
			Algorithm string `json:"algorithm"`
			Checksum  string `json:"checksum"`
		} `json:"integrity"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test-tenant", resp.TenantID)
	assert.Equal(t, "json", resp.Format)
	assert.Equal(t, int64(1), resp.TotalElements)
	require.Len(t, resp.Records, 1)
	assert.Equal(t, id, resp.Records[0].ID)
	assert.False(t, resp.GeneratedAt.IsZero())
	assert.Equal(t, "sha256", resp.Integrity.Algorithm)
	assert.NotEmpty(t, resp.Integrity.Checksum)
}

func TestAuditHandler_ExportCSV_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{
		ID:         id,
		EntityType: "a2a_invoke",
		EntityID:   "agent-456",
		Action:     audit.AuditActionExecute,
		ActorID:    "tenant-a",
		ActorEmail: "admin@test.dev.local",
		Metadata:   `{"sourceTenant":"tenant-a","targetTenant":"tenant-b"}`,
		IPAddress:  "10.0.0.5",
		CreatedAt:  time.Date(2026, 6, 23, 4, 0, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/export?format=csv&entityType=a2a_invoke&size=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "sha256", w.Header().Get("X-Audit-Export-Integrity-Algorithm"))
	assert.NotEmpty(t, w.Header().Get("X-Audit-Export-Integrity-Checksum"))
	assert.Equal(t, "1", w.Header().Get("X-Audit-Export-Total-Elements"))

	rows, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"id", "entityType", "entityId", "action", "actorId", "actorEmail", "ipAddress", "createdAt", "metadata"}, rows[0])
	assert.Equal(t, id.String(), rows[1][0])
	assert.Equal(t, "a2a_invoke", rows[1][1])
	assert.Equal(t, "agent-456", rows[1][2])
	assert.Equal(t, "EXECUTE", rows[1][3])
}

func TestAuditHandler_ExportZipBundle_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{
		ID:         id,
		EntityType: "a2a_invoke",
		EntityID:   "agent-789",
		Action:     audit.AuditActionExecute,
		ActorID:    "tenant-a",
		ActorEmail: "admin@test.dev.local",
		Metadata:   `{"sourceTenant":"tenant-a","targetTenant":"tenant-b"}`,
		IPAddress:  "10.0.0.7",
		CreatedAt:  time.Date(2026, 6, 23, 4, 30, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/export?format=zip_bundle&entityType=a2a_invoke&size=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
	assert.Equal(t, "zip_bundle", w.Header().Get("X-Audit-Export-Format"))
	assert.Equal(t, "sha256", w.Header().Get("X-Audit-Export-Integrity-Algorithm"))
	assert.NotEmpty(t, w.Header().Get("X-Audit-Export-Integrity-Checksum"))
	assert.Equal(t, "1", w.Header().Get("X-Audit-Export-Total-Elements"))

	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)

	manifestBytes := zipEntry(t, zr, "manifest.json")
	_ = zipEntry(t, zr, "audit-logs.json")
	csvBytes := zipEntry(t, zr, "audit-logs.csv")

	var manifest struct {
		TenantID      string `json:"tenantId"`
		Format        string `json:"format"`
		TotalElements int64  `json:"totalElements"`
		Files         []struct {
			Path      string `json:"path"`
			Format    string `json:"format"`
			Bytes     int64  `json:"bytes"`
			Algorithm string `json:"algorithm"`
			Checksum  string `json:"checksum"`
		} `json:"files"`
		Integrity struct {
			Algorithm string `json:"algorithm"`
			Checksum  string `json:"checksum"`
		} `json:"integrity"`
	}
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	assert.Equal(t, "test-tenant", manifest.TenantID)
	assert.Equal(t, "zip_bundle", manifest.Format)
	assert.Equal(t, int64(1), manifest.TotalElements)
	require.Len(t, manifest.Files, 2)
	assert.Equal(t, "audit-logs.json", manifest.Files[0].Path)
	assert.Equal(t, "json", manifest.Files[0].Format)
	assert.Equal(t, "sha256", manifest.Files[0].Algorithm)
	assert.NotEmpty(t, manifest.Files[0].Checksum)
	assert.Equal(t, "audit-logs.csv", manifest.Files[1].Path)
	assert.Equal(t, "csv", manifest.Files[1].Format)
	assert.Equal(t, "sha256", manifest.Files[1].Algorithm)
	assert.NotEmpty(t, manifest.Files[1].Checksum)
	assert.Equal(t, "sha256", manifest.Integrity.Algorithm)
	assert.NotEmpty(t, manifest.Integrity.Checksum)

	rows, err := csv.NewReader(bytes.NewReader(csvBytes)).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, id.String(), rows[1][0])
	assert.Equal(t, "agent-789", rows[1][2])
}

func TestAuditHandler_ExportXLSX_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{
		ID:         id,
		EntityType: "a2a_invoke",
		EntityID:   "agent-999",
		Action:     audit.AuditActionExecute,
		ActorID:    "tenant-a",
		ActorEmail: "admin@test.dev.local",
		Metadata:   `{"sourceTenant":"tenant-a","targetTenant":"tenant-b","note":"a & b"}`,
		IPAddress:  "10.0.0.9",
		CreatedAt:  time.Date(2026, 6, 23, 5, 0, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/export?format=xlsx&entityType=a2a_invoke&size=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
	assert.Equal(t, "xlsx", w.Header().Get("X-Audit-Export-Format"))
	assert.Equal(t, "sha256", w.Header().Get("X-Audit-Export-Integrity-Algorithm"))
	assert.NotEmpty(t, w.Header().Get("X-Audit-Export-Integrity-Checksum"))
	assert.Equal(t, "1", w.Header().Get("X-Audit-Export-Total-Elements"))

	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)

	contentTypes := string(zipEntry(t, zr, "[Content_Types].xml"))
	workbook := string(zipEntry(t, zr, "xl/workbook.xml"))
	sheet := string(zipEntry(t, zr, "xl/worksheets/sheet1.xml"))
	_ = zipEntry(t, zr, "_rels/.rels")
	_ = zipEntry(t, zr, "docProps/core.xml")

	assert.Contains(t, contentTypes, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml")
	assert.Contains(t, workbook, "Audit Logs")
	assert.Contains(t, sheet, "<t>entityId</t>")
	assert.Contains(t, sheet, "<t>"+id.String()+"</t>")
	assert.Contains(t, sheet, "<t>agent-999</t>")
	assert.Contains(t, sheet, "a &amp; b")
}

func TestAuditHandler_ExportPDF_Success(t *testing.T) {
	r, svc := setupAudit()
	id := uuid.New()
	svc.logs[id] = audit.AuditLog{
		ID:         id,
		EntityType: "a2a_invoke",
		EntityID:   "agent-pdf",
		Action:     audit.AuditActionExecute,
		ActorID:    "tenant-a",
		ActorEmail: "admin@test.dev.local",
		Metadata:   `{"sourceTenant":"tenant-a","targetTenant":"tenant-b","note":"a (b) \\ c"}`,
		IPAddress:  "10.0.0.10",
		CreatedAt:  time.Date(2026, 6, 23, 5, 30, 0, 0, time.UTC),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit/export?format=pdf&entityType=a2a_invoke&size=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, "pdf", w.Header().Get("X-Audit-Export-Format"))
	assert.Equal(t, "sha256", w.Header().Get("X-Audit-Export-Integrity-Algorithm"))
	assert.NotEmpty(t, w.Header().Get("X-Audit-Export-Integrity-Checksum"))
	assert.Equal(t, "1", w.Header().Get("X-Audit-Export-Total-Elements"))

	body := w.Body.String()
	assert.True(t, strings.HasPrefix(body, "%PDF-1.4"))
	assert.Contains(t, body, "AgentHub Audit Logs")
	assert.Contains(t, body, id.String())
	assert.Contains(t, body, "agent-pdf")
	assert.Contains(t, body, "a \\(b\\)")
	assert.Contains(t, body, "\\\\\\\\ c")
	assert.Contains(t, body, "%%EOF")
}

func TestAuditHandler_ExportJSON_UnsupportedFormat(t *testing.T) {
	r, _ := setupAudit()
	req := httptest.NewRequest(http.MethodGet, "/api/audit/export?format=xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuditHandler_ApplyRetention_Success(t *testing.T) {
	r, svc := setupAudit()
	oldID := uuid.New()
	recentID := uuid.New()
	now := time.Now().UTC()
	svc.logs[oldID] = audit.AuditLog{
		ID:         oldID,
		EntityType: "a2a_invoke",
		EntityID:   "old-audit",
		Action:     audit.AuditActionExecute,
		CreatedAt:  now.AddDate(-2, 0, 0),
	}
	svc.logs[recentID] = audit.AuditLog{
		ID:         recentID,
		EntityType: "a2a_invoke",
		EntityID:   "recent-audit",
		Action:     audit.AuditActionExecute,
		CreatedAt:  now.AddDate(0, 0, -30),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/audit/retention/apply", strings.NewReader(`{"retentionDays":365}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		TenantID      string    `json:"tenantId"`
		RetentionDays int       `json:"retentionDays"`
		Cutoff        time.Time `json:"cutoff"`
		DryRun        bool      `json:"dryRun"`
		Deleted       int       `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test-tenant", resp.TenantID)
	assert.Equal(t, 365, resp.RetentionDays)
	assert.False(t, resp.Cutoff.IsZero())
	assert.False(t, resp.DryRun)
	assert.Equal(t, 1, resp.Deleted)
	assert.NotContains(t, svc.logs, oldID)
	assert.Contains(t, svc.logs, recentID)
}

func TestAuditHandler_RecordAndRetentionRejectTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("record", func(t *testing.T) {
		r, svc := setupAudit()
		req := httptest.NewRequest(http.MethodPost, "/api/audit/", strings.NewReader(`{"entityType":"agent","action":"CREATE"} {"action":"DELETE"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, svc.logs)
	})

	t.Run("retention", func(t *testing.T) {
		r, svc := setupAudit()
		id := uuid.New()
		svc.logs[id] = audit.AuditLog{ID: id, CreatedAt: time.Now().UTC().AddDate(-2, 0, 0)}
		req := httptest.NewRequest(http.MethodPost, "/api/audit/retention/apply", strings.NewReader(`{"retentionDays":365} {"retentionDays":1}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, svc.logs, id)
	})
}

func zipEntry(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		defer func() { _ = rc.Close() }()
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		return body
	}
	require.Failf(t, "zip entry not found", "entry %s not found", name)
	return nil
}
