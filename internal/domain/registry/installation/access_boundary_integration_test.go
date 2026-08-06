//go:build integration

package installation

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

type integrationStorage struct{}

func (integrationStorage) Upload(context.Context, string, io.Reader, int64, string) (string, error) {
	return "unused", nil
}

func (integrationStorage) PresignedURL(context.Context, string, string, time.Duration) (string, error) {
	return "https://storage.example.test/download", nil
}

func TestIntegration_AssetDownloadBelongsToRequestedPackage(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	applyRegistryMigration(t, ctx, pool, "000007_package_registry_tables.up.sql")

	packageID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO public.package_registry (id, name, slug, type, visibility, author_tenant_id)
		VALUES ($1, 'Owner package', 'owner-package', 'AGENT', 'PRIVATE', 'owner')`, packageID)
	require.NoError(t, err)

	asset, err := NewRepository(pool).Create(ctx, PackageAsset{
		PackageID:   packageID,
		Filename:    "private.tgz",
		ContentType: "application/gzip",
		StoragePath: "packages/owner/private.tgz",
		SizeBytes:   1,
	})
	require.NoError(t, err)

	_, err = NewService(NewRepository(pool), integrationStorage{}).DownloadURL(ctx, uuid.New(), asset.ID)
	require.ErrorIs(t, err, ErrNotFound)

	response, err := NewService(NewRepository(pool), integrationStorage{}).DownloadURL(ctx, packageID, asset.ID)
	require.NoError(t, err)
	require.NotEmpty(t, response.URL)
}

func applyRegistryMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "migrations", "public", name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
}
