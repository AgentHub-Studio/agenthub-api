package audit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockAuditSvc satisfies the private auditService interface in audit.Handler.
type mockAuditSvc struct {
	logs map[uuid.UUID]audit.AuditLog
}

func newMockAuditSvc() *mockAuditSvc {
	return &mockAuditSvc{logs: make(map[uuid.UUID]audit.AuditLog)}
}

func (m *mockAuditSvc) ListAll(_ context.Context, _ string, _ audit.ListFilter, pr pagination.PageRequest) ([]audit.AuditLog, int, error) {
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

func setupAudit() (*chi.Mux, *mockAuditSvc) {
	svc := newMockAuditSvc()
	h := audit.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
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
