package pkg_test

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

	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockPkgSvc is an in-memory implementation of the packageService interface.
type mockPkgSvc struct {
	data map[uuid.UUID]pkg.Package
}

func newMockPkgSvc() *mockPkgSvc {
	return &mockPkgSvc{data: make(map[uuid.UUID]pkg.Package)}
}

func (m *mockPkgSvc) ListPublic(_ context.Context, req pagination.PageRequest) (pagination.Page[pkg.PackageResponse], error) {
	var items []pkg.PackageResponse
	for _, p := range m.data {
		if p.Visibility == pkg.PackageVisibilityPublic {
			items = append(items, pkg.ResponseFrom(p))
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockPkgSvc) GetByID(_ context.Context, id uuid.UUID) (pkg.PackageResponse, error) {
	p, ok := m.data[id]
	if !ok {
		return pkg.PackageResponse{}, pkg.ErrNotFound
	}
	return pkg.ResponseFrom(p), nil
}

func (m *mockPkgSvc) GetBySlug(_ context.Context, slug string) (pkg.PackageResponse, error) {
	for _, p := range m.data {
		if p.Slug == slug {
			return pkg.ResponseFrom(p), nil
		}
	}
	return pkg.PackageResponse{}, pkg.ErrNotFound
}

func (m *mockPkgSvc) GetAccessibleByID(_ context.Context, id uuid.UUID, tenantID string) (pkg.PackageResponse, error) {
	p, ok := m.data[id]
	if !ok || (p.Visibility != pkg.PackageVisibilityPublic && (tenantID == "" || p.AuthorTenantID != tenantID)) {
		return pkg.PackageResponse{}, pkg.ErrNotFound
	}
	return pkg.ResponseFrom(p), nil
}

func (m *mockPkgSvc) GetAccessibleBySlug(_ context.Context, slug, tenantID string) (pkg.PackageResponse, error) {
	for _, p := range m.data {
		if p.Slug == slug && (p.Visibility == pkg.PackageVisibilityPublic || (tenantID != "" && p.AuthorTenantID == tenantID)) {
			return pkg.ResponseFrom(p), nil
		}
	}
	return pkg.PackageResponse{}, pkg.ErrNotFound
}

func (m *mockPkgSvc) ListByTenant(_ context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[pkg.PackageResponse], error) {
	var items []pkg.PackageResponse
	for _, p := range m.data {
		if p.AuthorTenantID == tenantID {
			items = append(items, pkg.ResponseFrom(p))
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockPkgSvc) Create(_ context.Context, req pkg.CreatePackageRequest, tenantID string) (pkg.PackageResponse, error) {
	if req.Name == "" {
		return pkg.PackageResponse{}, &pkg.ValidationError{Field: "name", Message: "name is required"}
	}
	p := pkg.Package{
		ID:             uuid.New(),
		Name:           req.Name,
		Slug:           req.Slug,
		Type:           pkg.PackageType(req.Type),
		Visibility:     pkg.PackageVisibilityPrivate,
		AuthorTenantID: tenantID,
	}
	if req.Visibility == "PUBLIC" {
		p.Visibility = pkg.PackageVisibilityPublic
	}
	m.data[p.ID] = p
	return pkg.ResponseFrom(p), nil
}

func (m *mockPkgSvc) Update(_ context.Context, id uuid.UUID, req pkg.UpdatePackageRequest, tenantID string) (pkg.PackageResponse, error) {
	p, ok := m.data[id]
	if !ok {
		return pkg.PackageResponse{}, pkg.ErrNotFound
	}
	if p.AuthorTenantID != tenantID {
		return pkg.PackageResponse{}, &pkg.ForbiddenError{Message: "not the owner"}
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	m.data[id] = p
	return pkg.ResponseFrom(p), nil
}

func (m *mockPkgSvc) Delete(_ context.Context, id uuid.UUID, tenantID string) error {
	p, ok := m.data[id]
	if !ok {
		return pkg.ErrNotFound
	}
	if p.AuthorTenantID != tenantID {
		return &pkg.ForbiddenError{Message: "not the owner"}
	}
	delete(m.data, id)
	return nil
}

func (m *mockPkgSvc) Search(_ context.Context, query string, pkgType *string, req pagination.PageRequest) (pagination.Page[pkg.PackageResponse], error) {
	var items []pkg.PackageResponse
	for _, p := range m.data {
		if p.Visibility == pkg.PackageVisibilityPublic {
			items = append(items, pkg.ResponseFrom(p))
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func setupPkgHandler() (*chi.Mux, *mockPkgSvc) {
	svc := newMockPkgSvc()
	h := pkg.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)
	h.RegisterReadRoutes(r)
	h.RegisterProtectedRoutes(r)
	return r, svc
}

func withTenantCtx(req *http.Request, tenantID string) *http.Request {
	ctx := tenantpkg.NewContext(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// Tests

func TestPkgHandler_ListPublic_OK(t *testing.T) {
	r, svc := setupPkgHandler()
	id := uuid.New()
	svc.data[id] = pkg.Package{ID: id, Name: "A", Visibility: pkg.PackageVisibilityPublic, AuthorTenantID: "t1"}

	req := httptest.NewRequest(http.MethodGet, "/api/packages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[pkg.PackageResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestPkgHandler_GetByID_OK(t *testing.T) {
	r, svc := setupPkgHandler()
	id := uuid.New()
	svc.data[id] = pkg.Package{ID: id, Name: "B", Visibility: pkg.PackageVisibilityPublic, AuthorTenantID: "t1"}

	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPkgHandler_GetPrivatePackageRequiresOwner(t *testing.T) {
	r, svc := setupPkgHandler()
	id := uuid.New()
	svc.data[id] = pkg.Package{
		ID:             id,
		Name:           "Private package",
		Slug:           "private-package",
		Visibility:     pkg.PackageVisibilityPrivate,
		AuthorTenantID: "owner",
	}

	for _, tc := range []struct {
		name     string
		path     string
		tenantID string
		want     int
	}{
		{name: "anonymous ID", path: "/api/packages/" + id.String(), want: http.StatusNotFound},
		{name: "other tenant ID", path: "/api/packages/" + id.String(), tenantID: "other", want: http.StatusNotFound},
		{name: "owner ID", path: "/api/packages/" + id.String(), tenantID: "owner", want: http.StatusOK},
		{name: "anonymous slug", path: "/api/packages/slug/private-package", want: http.StatusNotFound},
		{name: "owner slug", path: "/api/packages/slug/private-package", tenantID: "owner", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.tenantID != "" {
				req = withTenantCtx(req, tc.tenantID)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tc.want, w.Code)
		})
	}
}

func TestPkgHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupPkgHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPkgHandler_GetByID_InvalidID(t *testing.T) {
	r, _ := setupPkgHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPkgHandler_Search_CanonicalRegistryRoute(t *testing.T) {
	r, svc := setupPkgHandler()
	id := uuid.New()
	svc.data[id] = pkg.Package{
		ID:             id,
		Name:           "Document Search",
		Visibility:     pkg.PackageVisibilityPublic,
		AuthorTenantID: "t1",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/registry/search?q=document", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[pkg.PackageResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestPkgHandler_Create_Success(t *testing.T) {
	r, _ := setupPkgHandler()
	body, _ := json.Marshal(pkg.CreatePackageRequest{
		Name:       "My Package",
		Slug:       "my-package",
		Type:       "AGENT",
		Visibility: "PUBLIC",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/packages", bytes.NewReader(body))
	req = withTenantCtx(req, "tenant-1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp pkg.PackageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Package", resp.Name)
}

func TestPkgHandler_Create_MissingTenant(t *testing.T) {
	r, _ := setupPkgHandler()
	body, _ := json.Marshal(pkg.CreatePackageRequest{Name: "X", Slug: "x", Type: "AGENT"})
	req := httptest.NewRequest(http.MethodPost, "/api/packages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No tenant context injected
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPkgHandler_Create_BadBody(t *testing.T) {
	r, _ := setupPkgHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/packages", bytes.NewReader([]byte("not-json")))
	req = withTenantCtx(req, "t1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPkgHandler_CreateAndUpdateRejectTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupPkgHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/packages", bytes.NewBufferString(`{"name":"first","slug":"first","type":"AGENT"} {"name":"ignored"}`))
		req = withTenantCtx(req, "tenant-1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, svc.data)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupPkgHandler()
		id := uuid.New()
		original := pkg.Package{ID: id, Name: "unchanged", AuthorTenantID: "tenant-1"}
		svc.data[id] = original
		req := httptest.NewRequest(http.MethodPatch, "/api/packages/"+id.String(), bytes.NewBufferString(`{"name":"changed"} {"name":"ignored"}`))
		req = withTenantCtx(req, "tenant-1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, original, svc.data[id])
	})
}

func TestPkgHandler_Delete_NoContent(t *testing.T) {
	r, svc := setupPkgHandler()
	id := uuid.New()
	svc.data[id] = pkg.Package{ID: id, Name: "D", AuthorTenantID: "owner"}

	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+id.String(), nil)
	req = withTenantCtx(req, "owner")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPkgHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupPkgHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+uuid.New().String(), nil)
	req = withTenantCtx(req, "owner")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPkgHandler_Delete_Forbidden(t *testing.T) {
	r, svc := setupPkgHandler()
	id := uuid.New()
	svc.data[id] = pkg.Package{ID: id, Name: "D", AuthorTenantID: "real-owner"}

	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+id.String(), nil)
	req = withTenantCtx(req, "intruder")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
