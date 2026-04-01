package installation_test

import (
	"bytes"
	"context"
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

func (m *mockAssetSvc) DownloadURL(_ context.Context, assetID uuid.UUID) (installation.AssetDownloadResponse, error) {
	a, ok := m.assets[assetID]
	if !ok {
		return installation.AssetDownloadResponse{}, installation.ErrNotFound
	}
	return installation.AssetDownloadResponse{URL: "https://example.com/download/" + a.Filename}, nil
}

func setupInstallHandler() (*chi.Mux, *mockAssetSvc) {
	svc := newMockAssetSvc()
	h := installation.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)
	h.RegisterProtectedRoutes(r)
	return r, svc
}

// multipartBody builds a multipart/form-data body with a file field.
func multipartBody(t *testing.T, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
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
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
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
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAssetHandler_UploadAsset_NotMultipart(t *testing.T) {
	r, _ := setupInstallHandler()
	pkgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/packages/"+pkgID.String()+"/assets", bytes.NewReader([]byte("plain text")))
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
