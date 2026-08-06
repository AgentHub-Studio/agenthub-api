package version_test

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
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/version"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockVersionSvc is an in-memory implementation of the versionService interface.
type mockVersionSvc struct {
	data map[string]version.PackageVersion // key: packageID:versionStr
}

func newMockVersionSvc() *mockVersionSvc {
	return &mockVersionSvc{data: make(map[string]version.PackageVersion)}
}

func svcKey(packageID uuid.UUID, v string) string {
	return packageID.String() + ":" + v
}

func (m *mockVersionSvc) ListByPackage(_ context.Context, packageID uuid.UUID) ([]version.VersionResponse, error) {
	var out []version.VersionResponse
	for _, v := range m.data {
		if v.PackageID == packageID {
			out = append(out, version.ResponseFrom(v))
		}
	}
	return out, nil
}

func (m *mockVersionSvc) GetByVersion(_ context.Context, packageID uuid.UUID, versionStr string) (version.VersionResponse, error) {
	v, ok := m.data[svcKey(packageID, versionStr)]
	if !ok {
		return version.VersionResponse{}, version.ErrNotFound
	}
	return version.ResponseFrom(v), nil
}

func (m *mockVersionSvc) Publish(_ context.Context, packageID uuid.UUID, req version.PublishVersionRequest, tenantID string) (version.VersionResponse, error) {
	if req.Version == "" {
		return version.VersionResponse{}, &version.ValidationError{Field: "version", Message: "version is required"}
	}
	v := version.PackageVersion{
		ID:          uuid.New(),
		PackageID:   packageID,
		Version:     req.Version,
		PublishedBy: tenantID,
	}
	m.data[svcKey(packageID, req.Version)] = v
	return version.ResponseFrom(v), nil
}

func (m *mockVersionSvc) Delete(_ context.Context, packageID uuid.UUID, versionStr string, tenantID string) error {
	k := svcKey(packageID, versionStr)
	if _, ok := m.data[k]; !ok {
		return version.ErrNotFound
	}
	delete(m.data, k)
	return nil
}

func setupVersionHandler() (*chi.Mux, *mockVersionSvc) {
	svc := newMockVersionSvc()
	h := version.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

type mockPackageReader struct {
	packages map[uuid.UUID]pkg.PackageResponse
}

func (m *mockPackageReader) GetAccessibleByID(_ context.Context, id uuid.UUID, tenantID string) (pkg.PackageResponse, error) {
	p, ok := m.packages[id]
	if !ok || (p.Visibility != string(pkg.PackageVisibilityPublic) && (tenantID == "" || p.AuthorTenantID != tenantID)) {
		return pkg.PackageResponse{}, pkg.ErrNotFound
	}
	return p, nil
}

func setupVersionHandlerWithPackages(packages map[uuid.UUID]pkg.PackageResponse) (*chi.Mux, *mockVersionSvc) {
	svc := newMockVersionSvc()
	h := version.NewHandler(svc).WithPackageReader(&mockPackageReader{packages: packages})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func withTenantCtx(req *http.Request, tenantID string) *http.Request {
	ctx := tenantpkg.NewContext(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// Tests

func TestVersionHandler_List_OK(t *testing.T) {
	r, svc := setupVersionHandler()
	pkgID := uuid.New()
	svc.data[svcKey(pkgID, "1.0.0")] = version.PackageVersion{ID: uuid.New(), PackageID: pkgID, Version: "1.0.0"}

	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+pkgID.String()+"/versions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var versions []version.VersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &versions))
	assert.Len(t, versions, 1)
}

func TestVersionHandler_List_InvalidPackageID(t *testing.T) {
	r, _ := setupVersionHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/not-a-uuid/versions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVersionHandler_GetByVersion_OK(t *testing.T) {
	r, svc := setupVersionHandler()
	pkgID := uuid.New()
	svc.data[svcKey(pkgID, "2.0.0")] = version.PackageVersion{ID: uuid.New(), PackageID: pkgID, Version: "2.0.0"}

	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+pkgID.String()+"/versions/2.0.0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp version.VersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2.0.0", resp.Version)
}

func TestVersionHandler_GetByVersion_NotFound(t *testing.T) {
	r, _ := setupVersionHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+pkgID.String()+"/versions/9.9.9", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVersionHandler_PrivatePackageReadHidden(t *testing.T) {
	pkgID := uuid.New()
	r, svc := setupVersionHandlerWithPackages(map[uuid.UUID]pkg.PackageResponse{
		pkgID: {
			ID:             pkgID,
			Visibility:     string(pkg.PackageVisibilityPrivate),
			AuthorTenantID: "owner",
		},
	})
	svc.data[svcKey(pkgID, "1.0.0")] = version.PackageVersion{ID: uuid.New(), PackageID: pkgID, Version: "1.0.0"}

	for _, tc := range []struct {
		name     string
		path     string
		tenantID string
		want     int
	}{
		{name: "anonymous list", path: "/api/packages/" + pkgID.String() + "/versions", want: http.StatusNotFound},
		{name: "other tenant version", path: "/api/packages/" + pkgID.String() + "/versions/1.0.0", tenantID: "other", want: http.StatusNotFound},
		{name: "owner version", path: "/api/packages/" + pkgID.String() + "/versions/1.0.0", tenantID: "owner", want: http.StatusOK},
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

func TestVersionHandler_Publish_Success(t *testing.T) {
	r, _ := setupVersionHandler()
	pkgID := uuid.New()
	body, _ := json.Marshal(version.PublishVersionRequest{Version: "1.0.0", Changelog: "first release"})
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/versions", bytes.NewReader(body))
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp version.VersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "1.0.0", resp.Version)
}

func TestVersionHandler_Publish_MissingTenant(t *testing.T) {
	r, _ := setupVersionHandler()
	pkgID := uuid.New()
	body, _ := json.Marshal(version.PublishVersionRequest{Version: "1.0.0"})
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/versions", bytes.NewReader(body))
	// No tenant in context
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestVersionHandler_Publish_BadBody(t *testing.T) {
	r, _ := setupVersionHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/versions", bytes.NewReader([]byte("not-json")))
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVersionHandler_PublishRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	r, svc := setupVersionHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/versions", bytes.NewBufferString(`{"version":"1.0.0","changelog":"first release"} {"version":"ignored"}`))
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.data)
}

func TestVersionHandler_Delete_NoContent(t *testing.T) {
	r, svc := setupVersionHandler()
	pkgID := uuid.New()
	svc.data[svcKey(pkgID, "1.0.0")] = version.PackageVersion{ID: uuid.New(), PackageID: pkgID, Version: "1.0.0"}

	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+pkgID.String()+"/versions/1.0.0", nil)
	req = withTenantCtx(req, "owner")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestVersionHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupVersionHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+pkgID.String()+"/versions/9.9.9", nil)
	req = withTenantCtx(req, "owner")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
