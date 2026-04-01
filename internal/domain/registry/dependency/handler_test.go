package dependency_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/dependency"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockDepSvc is an in-memory implementation of the dependencyService interface.
type mockDepSvc struct {
	deps map[uuid.UUID]dependency.PackageDependency
}

func newMockDepSvc() *mockDepSvc {
	return &mockDepSvc{deps: make(map[uuid.UUID]dependency.PackageDependency)}
}

func (m *mockDepSvc) List(_ context.Context, packageID uuid.UUID) ([]dependency.DependencyResponse, error) {
	var out []dependency.DependencyResponse
	for _, d := range m.deps {
		if d.PackageID == packageID {
			out = append(out, dependency.ResponseFrom(d))
		}
	}
	return out, nil
}

func (m *mockDepSvc) Add(_ context.Context, packageID uuid.UUID, req dependency.AddDependencyRequest, tenantID string) (dependency.DependencyResponse, error) {
	if req.VersionConstraint == "" {
		return dependency.DependencyResponse{}, &dependency.ValidationError{Field: "versionConstraint", Message: "required"}
	}
	d := dependency.PackageDependency{
		ID:                uuid.New(),
		PackageID:         packageID,
		DependencyID:      req.DependencyID,
		VersionConstraint: req.VersionConstraint,
	}
	m.deps[d.ID] = d
	return dependency.ResponseFrom(d), nil
}

func (m *mockDepSvc) Remove(_ context.Context, packageID, depID uuid.UUID, tenantID string) error {
	for id, d := range m.deps {
		if d.PackageID == packageID && d.ID == depID {
			delete(m.deps, id)
			return nil
		}
	}
	return dependency.ErrNotFound
}

func (m *mockDepSvc) Resolve(_ context.Context, packageID uuid.UUID) (dependency.ResolvedDependency, error) {
	return dependency.ResolvedDependency{PackageID: packageID, Name: "root", Slug: "root"}, nil
}

func setupDepHandler() (*chi.Mux, *mockDepSvc) {
	svc := newMockDepSvc()
	h := dependency.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func withTenantCtx(req *http.Request, tenantID string) *http.Request {
	ctx := tenantpkg.NewContext(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// Tests

func TestDepHandler_List_OK(t *testing.T) {
	r, svc := setupDepHandler()
	pkgID := uuid.New()
	depID := uuid.New()
	svc.deps[depID] = dependency.PackageDependency{ID: depID, PackageID: pkgID, DependencyID: uuid.New(), VersionConstraint: ">=1.0.0"}

	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+pkgID.String()+"/dependencies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var deps []dependency.DependencyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &deps))
	assert.Len(t, deps, 1)
}

func TestDepHandler_List_InvalidPackageID(t *testing.T) {
	r, _ := setupDepHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/bad-id/dependencies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDepHandler_Add_Success(t *testing.T) {
	r, _ := setupDepHandler()
	pkgID := uuid.New()
	body, _ := json.Marshal(dependency.AddDependencyRequest{
		DependencyID:      uuid.New(),
		VersionConstraint: ">=1.0.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/dependencies", bytes.NewReader(body))
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp dependency.DependencyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, pkgID, resp.PackageID)
}

func TestDepHandler_Add_MissingTenant(t *testing.T) {
	r, _ := setupDepHandler()
	pkgID := uuid.New()
	body, _ := json.Marshal(dependency.AddDependencyRequest{DependencyID: uuid.New(), VersionConstraint: ">=1.0.0"})
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/dependencies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDepHandler_Add_BadBody(t *testing.T) {
	r, _ := setupDepHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/dependencies", bytes.NewReader([]byte("not-json")))
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDepHandler_Remove_NoContent(t *testing.T) {
	r, svc := setupDepHandler()
	pkgID := uuid.New()
	depID := uuid.New()
	svc.deps[depID] = dependency.PackageDependency{ID: depID, PackageID: pkgID, DependencyID: uuid.New(), VersionConstraint: ">=1.0.0"}

	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+pkgID.String()+"/dependencies/"+depID.String(), nil)
	req = withTenantCtx(req, "owner")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDepHandler_Remove_NotFound(t *testing.T) {
	r, _ := setupDepHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/packages/"+pkgID.String()+"/dependencies/"+uuid.New().String(), nil)
	req = withTenantCtx(req, "owner")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDepHandler_Resolve_OK(t *testing.T) {
	r, _ := setupDepHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+pkgID.String()+"/dependencies/resolved", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var tree dependency.ResolvedDependency
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tree))
	assert.Equal(t, pkgID, tree.PackageID)
}
