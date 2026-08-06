//go:build integration

package installation_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/installation"
	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_InstallationRejectsPrivatePackage(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	applyInstallationMigration(t, ctx, pool, "000005_llm_config_preset.up.sql")
	applyInstallationMigration(t, ctx, pool, "000007_package_registry_tables.up.sql")
	applyInstallationMigration(t, ctx, pool, "000018_package_registry_semantic_search.up.sql")
	applyInstallationMigration(t, ctx, pool, "000003_tenant_package_installation.up.sql")
	applyInstallationMigration(t, ctx, pool, "000012_installation_hydrated.up.sql")

	pkgRepo := pkg.NewRepository(pool)
	privatePackage, err := pkgRepo.Create(ctx, pkg.Package{
		Name:           "Private installation package",
		Slug:           "private-installation-package",
		Tags:           []string{},
		Type:           pkg.PackageTypeAgent,
		Visibility:     pkg.PackageVisibilityPrivate,
		AuthorTenantID: "owner-tenant",
	})
	require.NoError(t, err)
	publicPackage, err := pkgRepo.Create(ctx, pkg.Package{
		Name:           "Public installation package",
		Slug:           "public-installation-package",
		Tags:           []string{},
		Type:           pkg.PackageTypeAgent,
		Visibility:     pkg.PackageVisibilityPublic,
		AuthorTenantID: "owner-tenant",
	})
	require.NoError(t, err)

	svc := installation.NewService(installation.NewRepository(pool)).WithPackageReader(pkgRepo)
	_, err = svc.Install(ctx, "consumer-tenant", installation.InstallRequest{
		PackageID: privatePackage.ID, PackageVersion: "1.0.0",
	})
	require.ErrorIs(t, err, installation.ErrPackageUnavailable)
	assertInstallationCount(t, ctx, pool, 0)

	_, err = svc.Install(ctx, "consumer-tenant", installation.InstallRequest{
		PackageID: uuid.New(), PackageVersion: "1.0.0",
	})
	require.ErrorIs(t, err, installation.ErrPackageUnavailable)
	assertInstallationCount(t, ctx, pool, 0)

	created, err := svc.Install(ctx, "consumer-tenant", installation.InstallRequest{
		PackageID: publicPackage.ID, PackageVersion: "1.0.0",
	})
	require.NoError(t, err)
	assert.Equal(t, publicPackage.ID, created.PackageID)
	assertInstallationCount(t, ctx, pool, 1)
}

func applyInstallationMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "migrations", "public", name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
}

func assertInstallationCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()
	var got int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.tenant_package_installation").Scan(&got))
	assert.Equal(t, want, got)
}
