package vpnresource_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockVPNSvc satisfies the private vpnService interface in vpnresource.Handler.
type mockVPNSvc struct {
	resources map[uuid.UUID]vpnresource.VpnResource
}

func newMockVPNSvc() *mockVPNSvc {
	return &mockVPNSvc{resources: make(map[uuid.UUID]vpnresource.VpnResource)}
}

func (m *mockVPNSvc) ListAll(_ context.Context, _ string, pr pagination.PageRequest) ([]vpnresource.VpnResource, int, error) {
	items := make([]vpnresource.VpnResource, 0, len(m.resources))
	for _, v := range m.resources {
		items = append(items, v)
	}
	return items, len(items), nil
}

func (m *mockVPNSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (vpnresource.VpnResource, error) {
	v, ok := m.resources[id]
	if !ok {
		return vpnresource.VpnResource{}, vpnresource.ErrNotFound
	}
	return v, nil
}

func (m *mockVPNSvc) Create(_ context.Context, _ string, req vpnresource.CreateRequest) (vpnresource.VpnResource, error) {
	id := uuid.New()
	v := vpnresource.VpnResource{
		ID:      id,
		Name:    req.Name,
		Enabled: req.Enabled,
	}
	m.resources[id] = v
	return v, nil
}

func (m *mockVPNSvc) Update(_ context.Context, _ string, id uuid.UUID, req vpnresource.CreateRequest) (vpnresource.VpnResource, error) {
	v, ok := m.resources[id]
	if !ok {
		return vpnresource.VpnResource{}, vpnresource.ErrNotFound
	}
	v.Name = req.Name
	m.resources[id] = v
	return v, nil
}

func (m *mockVPNSvc) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.resources[id]; !ok {
		return vpnresource.ErrNotFound
	}
	delete(m.resources, id)
	return nil
}

func (m *mockVPNSvc) TestConnection(_ context.Context, _ string, id uuid.UUID) (vpnresource.TestConnectionResponse, error) {
	if _, ok := m.resources[id]; !ok {
		return vpnresource.TestConnectionResponse{}, vpnresource.ErrNotFound
	}
	return vpnresource.TestConnectionResponse{Connected: true, Message: "OK"}, nil
}

func setupVPN() (*chi.Mux, *mockVPNSvc) {
	svc := newMockVPNSvc()
	h := vpnresource.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/vpn-resources", h.Routes())
	return r, svc
}

func TestVPNResourceHandler_List_Success(t *testing.T) {
	r, svc := setupVPN()
	id := uuid.New()
	svc.resources[id] = vpnresource.VpnResource{ID: id, Name: "corp-vpn", Enabled: true}

	req := httptest.NewRequest(http.MethodGet, "/api/vpn-resources/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[vpnresource.VpnResourceResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestVPNResourceHandler_Create_Success(t *testing.T) {
	r, _ := setupVPN()
	body, _ := json.Marshal(vpnresource.CreateRequest{Name: "my-vpn", Enabled: true})
	req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp vpnresource.VpnResourceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "my-vpn", resp.Name)
}

func TestVPNResourceHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupVPN()
	req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVPNResourceHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupVPN()
	req := httptest.NewRequest(http.MethodGet, "/api/vpn-resources/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVPNResourceHandler_Delete_Success(t *testing.T) {
	r, svc := setupVPN()
	id := uuid.New()
	svc.resources[id] = vpnresource.VpnResource{ID: id, Name: "to-delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/vpn-resources/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestVPNResourceHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupVPN()
	req := httptest.NewRequest(http.MethodDelete, "/api/vpn-resources/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVPNResourceHandler_TestConnection_Success(t *testing.T) {
	r, svc := setupVPN()
	id := uuid.New()
	svc.resources[id] = vpnresource.VpnResource{ID: id, Name: "vpn-a"}

	req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/"+id.String()+"/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp vpnresource.TestConnectionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Connected)
}

func TestVPNResourceHandler_TestConnection_NotFound(t *testing.T) {
	r, _ := setupVPN()
	req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/"+uuid.New().String()+"/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
