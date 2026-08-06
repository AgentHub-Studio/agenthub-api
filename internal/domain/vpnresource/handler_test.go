package vpnresource_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockVPNSvc satisfies the private vpnService interface in vpnresource.Handler.
type mockVPNSvc struct {
	resources    map[uuid.UUID]vpnresource.VpnResource
	lastOvpn     []byte
	lastOvpnSize int64
	lastAuth     []byte
	lastAuthSize int64
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
	v.Description = req.Description
	v.Enabled = req.Enabled
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

func (m *mockVPNSvc) UploadOvpnConfig(_ context.Context, _ string, id uuid.UUID, r io.Reader, size int64) (vpnresource.VpnResource, error) {
	v, ok := m.resources[id]
	if !ok {
		return vpnresource.VpnResource{}, vpnresource.ErrNotFound
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return vpnresource.VpnResource{}, err
	}
	m.lastOvpn = data
	m.lastOvpnSize = size
	return v, nil
}

func (m *mockVPNSvc) UploadAuthFile(_ context.Context, _ string, id uuid.UUID, r io.Reader, size int64) (vpnresource.VpnResource, error) {
	v, ok := m.resources[id]
	if !ok {
		return vpnresource.VpnResource{}, vpnresource.ErrNotFound
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return vpnresource.VpnResource{}, err
	}
	m.lastAuth = data
	m.lastAuthSize = size
	return v, nil
}

func setupVPN() (*chi.Mux, *mockVPNSvc) {
	return setupVPNWithRoles("admin")
}

func setupVPNWithRoles(roles ...string) (*chi.Mux, *mockVPNSvc) {
	svc := newMockVPNSvc()
	h := vpnresource.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			ctx = middleware.ContextWithRoles(ctx, roles...)
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

func TestVPNResourceHandler_Update_Success(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			r, svc := setupVPN()
			id := uuid.New()
			svc.resources[id] = vpnresource.VpnResource{ID: id, Name: "before", Description: "old", Enabled: false}
			body := bytes.NewBufferString(`{"name":"after","description":"updated","enabled":true}`)
			req := httptest.NewRequest(method, "/api/vpn-resources/"+id.String(), body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			var resp vpnresource.VpnResourceResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, id, resp.ID)
			assert.Equal(t, "after", resp.Name)
			assert.Equal(t, "updated", resp.Description)
			assert.True(t, resp.Enabled)
		})
	}
}

func TestVPNResourceHandler_CreateAndUpdateRejectTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupVPN()
		req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/", bytes.NewBufferString(`{"name":"corp-vpn","enabled":true} {"name":"ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, svc.resources)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupVPN()
		id := uuid.New()
		original := vpnresource.VpnResource{ID: id, Name: "unchanged", Enabled: false}
		svc.resources[id] = original
		req := httptest.NewRequest(http.MethodPut, "/api/vpn-resources/"+id.String(), bytes.NewBufferString(`{"name":"corp-vpn","enabled":true} {"name":"ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, original, svc.resources[id])
	})
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

func TestVPNResourceHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupVPNWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/vpn-resources/"},
		{name: "create", method: http.MethodPost, path: "/api/vpn-resources/", body: `{"name":"vpn","enabled":true}`},
		{name: "get", method: http.MethodGet, path: "/api/vpn-resources/" + id},
		{name: "put", method: http.MethodPut, path: "/api/vpn-resources/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/vpn-resources/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/vpn-resources/" + id},
		{name: "test", method: http.MethodPost, path: "/api/vpn-resources/" + id + "/test"},
		{name: "upload config", method: http.MethodPost, path: "/api/vpn-resources/" + id + "/upload-config"},
		{name: "upload auth", method: http.MethodPost, path: "/api/vpn-resources/" + id + "/upload-auth"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
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

func vpnMultipartBody(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &buf, writer.FormDataContentType()
}

func TestVPNResourceHandler_UploadConfig_Success(t *testing.T) {
	r, svc := setupVPN()
	id := uuid.New()
	svc.resources[id] = vpnresource.VpnResource{ID: id, Name: "vpn-a"}
	body, contentType := vpnMultipartBody(t, "corp.ovpn", []byte("client\nremote vpn.example 1194\ndev tun\n"))

	req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/"+id.String()+"/upload-config", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []byte("client\nremote vpn.example 1194\ndev tun\n"), svc.lastOvpn)
	assert.Equal(t, int64(len(svc.lastOvpn)), svc.lastOvpnSize)
}

func TestVPNResourceHandler_UploadAuth_Success(t *testing.T) {
	r, svc := setupVPN()
	id := uuid.New()
	svc.resources[id] = vpnresource.VpnResource{ID: id, Name: "vpn-a"}
	body, contentType := vpnMultipartBody(t, "auth.txt", []byte("user\npass\n"))

	req := httptest.NewRequest(http.MethodPost, "/api/vpn-resources/"+id.String()+"/upload-auth", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []byte("user\npass\n"), svc.lastAuth)
	assert.Equal(t, int64(len(svc.lastAuth)), svc.lastAuthSize)
}
