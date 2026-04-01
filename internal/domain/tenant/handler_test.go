package tenant_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockTenantSvc implements tenant.Service for handler tests.
type mockTenantSvc struct {
	tenants map[string]tenant.TenantResponse
}

func newMockTenantSvc() *mockTenantSvc {
	return &mockTenantSvc{tenants: make(map[string]tenant.TenantResponse)}
}

func (m *mockTenantSvc) Create(_ context.Context, req tenant.CreateTenantRequest) (tenant.TenantResponse, error) {
	if _, ok := m.tenants[req.ID]; ok {
		return tenant.TenantResponse{}, tenant.ErrAlreadyExists
	}
	resp := tenant.TenantResponse{ID: req.ID, Name: req.Name, Status: string(tenant.StatusActive)}
	m.tenants[req.ID] = resp
	return resp, nil
}

func (m *mockTenantSvc) GetByID(_ context.Context, id string) (tenant.TenantResponse, error) {
	t, ok := m.tenants[id]
	if !ok {
		return tenant.TenantResponse{}, tenant.ErrNotFound
	}
	return t, nil
}

func (m *mockTenantSvc) List(_ context.Context, req pagination.PageRequest) (pagination.Page[tenant.TenantResponse], error) {
	items := make([]tenant.TenantResponse, 0, len(m.tenants))
	for _, t := range m.tenants {
		items = append(items, t)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockTenantSvc) Exists(_ context.Context, id string) (bool, error) {
	_, ok := m.tenants[id]
	return ok, nil
}

func (m *mockTenantSvc) UpdateStatus(_ context.Context, id string, status tenant.Status) error {
	t, ok := m.tenants[id]
	if !ok {
		return tenant.ErrNotFound
	}
	t.Status = string(status)
	m.tenants[id] = t
	return nil
}

func setupTenant() (*chi.Mux, *mockTenantSvc) {
	svc := newMockTenantSvc()
	h := tenant.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)
	return r, svc
}

func TestTenantHandler_Create_Success(t *testing.T) {
	r, _ := setupTenant()
	body, _ := json.Marshal(tenant.CreateTenantRequest{ID: "my-company", Name: "My Company"})
	req := httptest.NewRequest(http.MethodPost, "/public/tenants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp tenant.TenantResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "my-company", resp.ID)
}

func TestTenantHandler_Create_AlreadyExists(t *testing.T) {
	r, svc := setupTenant()
	svc.tenants["existing-tenant"] = tenant.TenantResponse{ID: "existing-tenant", Name: "Existing"}

	body, _ := json.Marshal(tenant.CreateTenantRequest{ID: "existing-tenant", Name: "Existing"})
	req := httptest.NewRequest(http.MethodPost, "/public/tenants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTenantHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupTenant()
	req := httptest.NewRequest(http.MethodPost, "/public/tenants", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_List_Empty(t *testing.T) {
	r, _ := setupTenant()
	req := httptest.NewRequest(http.MethodGet, "/public/tenants", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[tenant.TenantResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(0), page.TotalElements)
}

func TestTenantHandler_List_Success(t *testing.T) {
	r, svc := setupTenant()
	svc.tenants["tenant-a"] = tenant.TenantResponse{ID: "tenant-a", Name: "Tenant A"}

	req := httptest.NewRequest(http.MethodGet, "/public/tenants", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTenantHandler_Exists_True(t *testing.T) {
	r, svc := setupTenant()
	svc.tenants["my-tenant"] = tenant.TenantResponse{ID: "my-tenant", Name: "My Tenant"}

	req := httptest.NewRequest(http.MethodGet, "/public/tenants/my-tenant/exists", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp tenant.ExistsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Exists)
}

func TestTenantHandler_Exists_False(t *testing.T) {
	r, _ := setupTenant()
	req := httptest.NewRequest(http.MethodGet, "/public/tenants/nonexistent/exists", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp tenant.ExistsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Exists)
}
