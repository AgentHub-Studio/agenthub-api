package installation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/installation"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockInstallSvc is an in-memory implementation of the installationService interface.
type mockInstallSvc struct {
	data       map[uuid.UUID]installation.Installation
	installErr error
}

func newMockInstallSvc() *mockInstallSvc {
	return &mockInstallSvc{data: make(map[uuid.UUID]installation.Installation)}
}

func (m *mockInstallSvc) ListByTenant(_ context.Context, tenantID string, req pagination.PageRequest) (pagination.Page[installation.InstallResponse], error) {
	var items []installation.InstallResponse
	for _, i := range m.data {
		if i.TenantID == tenantID {
			items = append(items, installation.ResponseFrom(i))
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockInstallSvc) Install(_ context.Context, tenantID string, req installation.InstallRequest) (installation.InstallResponse, error) {
	if m.installErr != nil {
		return installation.InstallResponse{}, m.installErr
	}
	if req.PackageID == uuid.Nil {
		return installation.InstallResponse{}, installation.ErrNotFound
	}
	i := installation.Installation{
		ID:             uuid.New(),
		TenantID:       tenantID,
		PackageID:      req.PackageID,
		PackageVersion: req.PackageVersion,
		Status:         installation.InstallStatusInstalled,
		InstalledAt:    time.Now(),
	}
	m.data[i.ID] = i
	return installation.ResponseFrom(i), nil
}

func (m *mockInstallSvc) Uninstall(_ context.Context, id uuid.UUID, tenantID string) error {
	i, ok := m.data[id]
	if !ok {
		return installation.ErrNotFound
	}
	if i.TenantID != tenantID {
		return installation.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func setupInstallationHandler() (*chi.Mux, *mockInstallSvc) {
	svc := newMockInstallSvc()
	h := installation.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func withTenant(req *http.Request, tenantID string) *http.Request {
	ctx := tenantpkg.NewContext(req.Context(), tenantID)
	return req.WithContext(ctx)
}

// Tests

func TestInstallHandler_List_OK(t *testing.T) {
	r, svc := setupInstallationHandler()
	svc.data[uuid.New()] = installation.Installation{
		ID: uuid.New(), TenantID: "t1", PackageID: uuid.New(), PackageVersion: "1.0.0", Status: installation.InstallStatusInstalled,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/installations", nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[installation.InstallResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestInstallHandler_List_Empty(t *testing.T) {
	r, _ := setupInstallationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/marketplace/installations", nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[installation.InstallResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.True(t, page.Empty)
}

func TestInstallHandler_Install_Success(t *testing.T) {
	r, _ := setupInstallationHandler()
	body, _ := json.Marshal(installation.InstallRequest{
		PackageID:      uuid.New(),
		PackageVersion: "1.0.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/installations", bytes.NewReader(body))
	req = withTenant(req, "t1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp installation.InstallResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, installation.InstallStatusInstalled, resp.Status)
}

func TestInstallHandler_Install_BadBody(t *testing.T) {
	r, _ := setupInstallationHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/installations", bytes.NewReader([]byte("not-json")))
	req = withTenant(req, "t1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInstallHandler_Install_HidesPrivatePackage(t *testing.T) {
	r, svc := setupInstallationHandler()
	svc.installErr = installation.ErrPackageUnavailable
	body, err := json.Marshal(installation.InstallRequest{PackageID: uuid.New(), PackageVersion: "1.0.0"})
	require.NoError(t, err)
	req := withTenant(httptest.NewRequest(http.MethodPost, "/api/marketplace/installations", bytes.NewReader(body)), "other-tenant")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, svc.data)
}

func TestInstallHandler_InstallRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	r, svc := setupInstallationHandler()
	body, err := json.Marshal(installation.InstallRequest{
		PackageID:      uuid.New(),
		PackageVersion: "1.0.0",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/marketplace/installations", bytes.NewReader(append(body, []byte(` {"packageVersion":"ignored"}`)...)))
	req = withTenant(req, "t1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.data)
}

func TestInstallHandler_Uninstall_NoContent(t *testing.T) {
	r, svc := setupInstallationHandler()
	id := uuid.New()
	svc.data[id] = installation.Installation{ID: id, TenantID: "t1", PackageID: uuid.New(), Status: installation.InstallStatusInstalled}

	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/installations/"+id.String(), nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestInstallHandler_Uninstall_NotFound(t *testing.T) {
	r, _ := setupInstallationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/installations/"+uuid.New().String(), nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInstallHandler_Uninstall_InvalidID(t *testing.T) {
	r, _ := setupInstallationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/marketplace/installations/bad-id", nil)
	req = withTenant(req, "t1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
