package installation_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/installation"
)

// mockAssetRepo is an in-memory implementation of AssetRepository.
type mockAssetRepo struct {
	assets map[uuid.UUID]installation.PackageAsset
}

func newMockAssetRepo() *mockAssetRepo {
	return &mockAssetRepo{assets: make(map[uuid.UUID]installation.PackageAsset)}
}

func (m *mockAssetRepo) ListByPackage(_ context.Context, packageID uuid.UUID, versionID *uuid.UUID) ([]installation.PackageAsset, error) {
	var out []installation.PackageAsset
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

func (m *mockAssetRepo) GetByID(_ context.Context, assetID uuid.UUID) (installation.PackageAsset, error) {
	a, ok := m.assets[assetID]
	if !ok {
		return installation.PackageAsset{}, installation.ErrNotFound
	}
	return a, nil
}

func (m *mockAssetRepo) Create(_ context.Context, a installation.PackageAsset) (installation.PackageAsset, error) {
	a.ID = uuid.New()
	a.CreatedAt = time.Now()
	m.assets[a.ID] = a
	return a, nil
}

// mockStorage is a no-op in-memory StorageBackend.
type mockStorage struct {
	stored map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{stored: make(map[string][]byte)}
}

func (m *mockStorage) Upload(_ context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	m.stored[key] = data
	return "storage://" + key, nil
}

func (m *mockStorage) PresignedURL(_ context.Context, storagePath string, _ string, _ time.Duration) (string, error) {
	if _, ok := m.stored[storagePath]; !ok {
		// Return a fake URL anyway (mimics real object stores)
		return fmt.Sprintf("https://storage.example.com/%s?sig=abc", storagePath), nil
	}
	return fmt.Sprintf("https://storage.example.com/%s?sig=abc", storagePath), nil
}

// Tests

func TestAssetService_UploadAsset_Success(t *testing.T) {
	svc := installation.NewService(newMockAssetRepo(), newMockStorage())
	pkgID := uuid.New()
	content := []byte("fake-archive-content")

	resp, err := svc.UploadAsset(context.Background(), pkgID, nil, "agent.tgz", "application/gzip", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	assert.Equal(t, pkgID, resp.PackageID)
	assert.Equal(t, "agent.tgz", resp.Filename)
	assert.NotEmpty(t, resp.StoragePath)
}

func TestAssetService_UploadAsset_MissingFilename(t *testing.T) {
	svc := installation.NewService(newMockAssetRepo(), newMockStorage())
	pkgID := uuid.New()

	_, err := svc.UploadAsset(context.Background(), pkgID, nil, "", "application/gzip", bytes.NewReader(nil), 0)
	require.Error(t, err)
	var ve *installation.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "filename", ve.Field)
}

func TestAssetService_UploadAsset_WithVersionID(t *testing.T) {
	svc := installation.NewService(newMockAssetRepo(), newMockStorage())
	pkgID := uuid.New()
	versionID := uuid.New()
	content := []byte("versioned-archive")

	resp, err := svc.UploadAsset(context.Background(), pkgID, &versionID, "agent-1.0.0.tgz", "application/gzip", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	require.NotNil(t, resp.VersionID)
	assert.Equal(t, versionID, *resp.VersionID)
}

func TestAssetService_ListAssets_Success(t *testing.T) {
	svc := installation.NewService(newMockAssetRepo(), newMockStorage())
	pkgID := uuid.New()

	for i := 0; i < 3; i++ {
		_, err := svc.UploadAsset(context.Background(), pkgID, nil, fmt.Sprintf("file%d.tgz", i), "application/gzip", bytes.NewReader([]byte("x")), 1)
		require.NoError(t, err)
	}
	// Different package — should not appear
	otherPkg := uuid.New()
	_, err := svc.UploadAsset(context.Background(), otherPkg, nil, "other.tgz", "application/gzip", bytes.NewReader([]byte("x")), 1)
	require.NoError(t, err)

	assets, err := svc.ListAssets(context.Background(), pkgID, nil)
	require.NoError(t, err)
	assert.Len(t, assets, 3)
}

func TestAssetService_ListAssets_FilteredByVersion(t *testing.T) {
	svc := installation.NewService(newMockAssetRepo(), newMockStorage())
	pkgID := uuid.New()
	versionID := uuid.New()
	otherVersionID := uuid.New()

	_, err := svc.UploadAsset(context.Background(), pkgID, &versionID, "v1.tgz", "application/gzip", bytes.NewReader([]byte("x")), 1)
	require.NoError(t, err)
	_, err = svc.UploadAsset(context.Background(), pkgID, &otherVersionID, "v2.tgz", "application/gzip", bytes.NewReader([]byte("x")), 1)
	require.NoError(t, err)

	assets, err := svc.ListAssets(context.Background(), pkgID, &versionID)
	require.NoError(t, err)
	assert.Len(t, assets, 1)
	assert.Equal(t, "v1.tgz", assets[0].Filename)
}

func TestAssetService_DownloadURL_Success(t *testing.T) {
	repo := newMockAssetRepo()
	store := newMockStorage()
	svc := installation.NewService(repo, store)
	pkgID := uuid.New()

	// Manually seed an asset with a known storage path
	assetID := uuid.New()
	store.stored["packages/"+pkgID.String()+"/assets/some-file.tgz"] = []byte("data")
	repo.assets[assetID] = installation.PackageAsset{
		ID:          assetID,
		PackageID:   pkgID,
		Filename:    "some-file.tgz",
		StoragePath: "packages/" + pkgID.String() + "/assets/some-file.tgz",
	}

	resp, err := svc.DownloadURL(context.Background(), assetID)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.URL)
}

func TestAssetService_DownloadURL_NotFound(t *testing.T) {
	svc := installation.NewService(newMockAssetRepo(), newMockStorage())
	_, err := svc.DownloadURL(context.Background(), uuid.New())
	require.ErrorIs(t, err, installation.ErrNotFound)
}
