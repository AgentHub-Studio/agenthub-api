package installation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/installation"
	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	tenantpkg "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockAssetSvc is an in-memory implementation of the assetService interface.
type mockAssetSvc struct {
	assets map[uuid.UUID]installation.AssetResponse
}

func newMockAssetSvc() *mockAssetSvc {
	return &mockAssetSvc{assets: make(map[uuid.UUID]installation.AssetResponse)}
}

func (m *mockAssetSvc) ListAssets(_ context.Context, packageID uuid.UUID, versionID *uuid.UUID) ([]installation.AssetResponse, error) {
	var out []installation.AssetResponse
	for _, a := range m.assets {
		if a.PackageID != packageID {
			continue
		}
		if versionID != nil {
			if a.VersionID == nil || *a.VersionID != *versionID {
				continue
			}
		}
		out = append(out, a)
	}
	return out, nil
}

func (m *mockAssetSvc) UploadAsset(_ context.Context, packageID uuid.UUID, versionID *uuid.UUID, filename string, contentType string, reader io.Reader, size int64) (installation.AssetResponse, error) {
	if filename == "" {
		return installation.AssetResponse{}, &installation.ValidationError{Field: "filename", Message: "filename is required"}
	}
	a := installation.AssetResponse{
		ID:          uuid.New(),
		PackageID:   packageID,
		VersionID:   versionID,
		Filename:    filename,
		ContentType: contentType,
		StoragePath: "storage://" + filename,
		SizeBytes:   size,
		CreatedAt:   time.Now(),
	}
	m.assets[a.ID] = a
	return a, nil
}

func (m *mockAssetSvc) DownloadURL(_ context.Context, packageID, assetID uuid.UUID) (installation.AssetDownloadResponse, error) {
	a, ok := m.assets[assetID]
	if !ok || a.PackageID != packageID {
		return installation.AssetDownloadResponse{}, installation.ErrNotFound
	}
	return installation.AssetDownloadResponse{URL: "https://example.com/download/" + a.Filename}, nil
}

func setupInstallHandler() (*chi.Mux, *mockAssetSvc) {
	svc := newMockAssetSvc()
	h := installation.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterReadRoutes(r)
	h.RegisterProtectedRoutes(r)
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

func setupInstallHandlerWithPackages(packages map[uuid.UUID]pkg.PackageResponse) (*chi.Mux, *mockAssetSvc) {
	svc := newMockAssetSvc()
	h := installation.NewHandler(svc).WithPackageReader(&mockPackageReader{packages: packages})
	r := chi.NewRouter()
	h.RegisterReadRoutes(r)
	h.RegisterProtectedRoutes(r)
	return r, svc
}

func withTenantCtx(req *http.Request, tenantID string) *http.Request {
	return req.WithContext(tenantpkg.NewContext(req.Context(), tenantID))
}

// multipartBody builds a multipart/form-data body with a file field.
func multipartBody(t *testing.T, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	return multipartBodyWithFields(t, filename, contentType, content, nil)
}

func multipartBodyWithFields(t *testing.T, filename, contentType string, content []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for name, value := range fields {
		require.NoError(t, w.WriteField(name, value))
	}
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

// Tests

func TestAssetHandler_ListAssets_OK(t *testing.T) {
	r, svc := setupInstallHandler()
	pkgID := uuid.New()
	assetID := uuid.New()
	svc.assets[assetID] = installation.AssetResponse{ID: assetID, PackageID: pkgID, Filename: "file.tgz"}

	req := httptest.NewRequest(http.MethodGet, "/api/packages/"+pkgID.String()+"/assets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAssetHandler_ListAssets_InvalidPackageID(t *testing.T) {
	r, _ := setupInstallHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/packages/bad-id/assets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssetHandler_UploadAsset_Success(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	body, ct := multipartBody(t, "agent.tgz", "application/gzip", []byte("fake-content"))

	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/assets", body)
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAssetHandler_UploadAsset_PreservesVersionID(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	versionID := uuid.New()
	body, ct := multipartBodyWithFields(t, "agent.tgz", "application/gzip", []byte("fake-content"), map[string]string{
		"versionId": versionID.String(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/assets", body)
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp installation.AssetResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.NotNil(t, resp.VersionID)
	assert.Equal(t, versionID, *resp.VersionID)
}

func TestAssetHandler_UploadAsset_InvalidPackageID(t *testing.T) {
	r, _ := setupInstallHandler()
	body, ct := multipartBody(t, "file.tgz", "application/gzip", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/api/packages/bad-id/assets", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssetHandler_UploadAsset_MissingFilePart(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	// Send a multipart body with no "file" part
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("versionId", uuid.New().String())
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/assets", &buf)
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssetHandler_UploadAsset_NotMultipart(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/assets", bytes.NewReader([]byte("plain text")))
	req = withTenantCtx(req, "owner")
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssetHandler_Download_OK(t *testing.T) {
	r, svc := setupInstallHandler()
	pkgID := uuid.New()
	assetID := uuid.New()
	svc.assets[assetID] = installation.AssetResponse{
		ID:          assetID,
		PackageID:   pkgID,
		Filename:    "agent.tgz",
		StoragePath: "storage://agent.tgz",
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/packages/%s/assets/%s/download", pkgID, assetID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAssetHandler_Download_NotFound(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/packages/%s/assets/%s/download", pkgID, uuid.New()), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAssetHandler_Download_InvalidAssetID(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/packages/%s/assets/not-a-uuid/download", pkgID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssetHandler_PrivatePackageAccessAndOwnerUpload(t *testing.T) {
	pkgID := uuid.New()
	publicPackageID := uuid.New()
	r, svc := setupInstallHandlerWithPackages(map[uuid.UUID]pkg.PackageResponse{
		pkgID: {
			ID:             pkgID,
			Visibility:     string(pkg.PackageVisibilityPrivate),
			AuthorTenantID: "owner",
		},
		publicPackageID: {
			ID:             publicPackageID,
			Visibility:     string(pkg.PackageVisibilityPublic),
			AuthorTenantID: "owner",
		},
	})
	assetID := uuid.New()
	svc.assets[assetID] = installation.AssetResponse{ID: assetID, PackageID: pkgID, Filename: "private.tgz"}

	for _, tc := range []struct {
		name     string
		method   string
		path     string
		tenantID string
		want     int
	}{
		{name: "anonymous list", method: http.MethodGet, path: "/api/packages/" + pkgID.String() + "/assets", want: http.StatusNotFound},
		{name: "other list", method: http.MethodGet, path: "/api/packages/" + pkgID.String() + "/assets", tenantID: "other", want: http.StatusNotFound},
		{name: "owner list", method: http.MethodGet, path: "/api/packages/" + pkgID.String() + "/assets", tenantID: "owner", want: http.StatusOK},
		{name: "other download", method: http.MethodGet, path: fmt.Sprintf("/api/packages/%s/assets/%s/download", pkgID, assetID), tenantID: "other", want: http.StatusNotFound},
		{name: "owner download", method: http.MethodGet, path: fmt.Sprintf("/api/packages/%s/assets/%s/download", pkgID, assetID), tenantID: "owner", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.tenantID != "" {
				req = withTenantCtx(req, tc.tenantID)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.want, w.Code)
		})
	}

	body, contentType := multipartBody(t, "private.tgz", "application/gzip", []byte("data"))
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/assets", body)
	req = withTenantCtx(req, "other")
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	body, contentType = multipartBody(t, "public.tgz", "application/gzip", []byte("data"))
	req = httptest.NewRequest(http.MethodPost, "/api/packages/"+publicPackageID.String()+"/assets", body)
	req = withTenantCtx(req, "other")
	req.Header.Set("Content-Type", contentType)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
