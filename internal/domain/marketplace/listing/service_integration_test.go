//go:build integration

package listing_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	pkg "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_ListingServiceBindsCreationToPackageOwner(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	applyMarketplaceMigration(t, ctx, pool, "000005_llm_config_preset.up.sql")
	applyMarketplaceMigration(t, ctx, pool, "000007_package_registry_tables.up.sql")
	applyMarketplaceMigration(t, ctx, pool, "000018_package_registry_semantic_search.up.sql")
	applyMarketplaceMigration(t, ctx, pool, "000001_marketplace_listing.up.sql")
	applyMarketplaceMigration(t, ctx, pool, "000008_fix_column_mismatches.up.sql")
	applyMarketplaceMigration(t, ctx, pool, "000009_marketplace_listing_created_at.up.sql")

	pkgRepo := pkg.NewRepository(pool)
	ownerPackage, err := pkgRepo.Create(ctx, pkg.Package{
		Name:           "Owner package",
		Slug:           "owner-package",
		Tags:           []string{},
		Type:           pkg.PackageTypeAgent,
		Visibility:     pkg.PackageVisibilityPublic,
		AuthorTenantID: "owner-tenant",
	})
	require.NoError(t, err)

	listingRepo := listing.NewRepository(pool)
	svc := listing.NewService(listingRepo, pkgRepo)
	req := listing.CreateListingRequest{
		PackageID: ownerPackage.ID,
		Name:      "Owner marketplace entry",
		Slug:      "owner-marketplace-entry",
		Type:      "AGENT",
	}

	_, err = svc.Create(ctx, "other-tenant", req)
	require.ErrorIs(t, err, listing.ErrForbidden)
	assertMarketplaceListingCount(t, ctx, pool, 0)

	created, err := svc.Create(ctx, "owner-tenant", req)
	require.NoError(t, err)
	assert.Equal(t, ownerPackage.ID, created.PackageID)
	assert.Equal(t, "owner-tenant", created.TenantID)
	assertMarketplaceListingCount(t, ctx, pool, 1)

	privatePackage, err := pkgRepo.Create(ctx, pkg.Package{
		Name:           "Private package",
		Slug:           "private-package",
		Tags:           []string{},
		Type:           pkg.PackageTypeAgent,
		Visibility:     pkg.PackageVisibilityPrivate,
		AuthorTenantID: "owner-tenant",
	})
	require.NoError(t, err)

	_, err = svc.Create(ctx, "other-tenant", listing.CreateListingRequest{
		PackageID: privatePackage.ID, Name: "Private entry", Slug: "private-entry", Type: "AGENT",
	})
	require.ErrorIs(t, err, listing.ErrPackageNotFound)
	assertMarketplaceListingCount(t, ctx, pool, 1)

	_, err = svc.Create(ctx, "owner-tenant", listing.CreateListingRequest{
		PackageID: privatePackage.ID, Name: "Private entry", Slug: "private-entry", Type: "AGENT",
	})
	require.ErrorIs(t, err, listing.ErrPackageNotPublic)
	assertMarketplaceListingCount(t, ctx, pool, 1)

	_, err = svc.Create(ctx, "owner-tenant", listing.CreateListingRequest{
		PackageID: uuid.New(), Name: "Missing package", Slug: "missing-package", Type: "AGENT",
	})
	require.ErrorIs(t, err, listing.ErrPackageNotFound)
	assertMarketplaceListingCount(t, ctx, pool, 1)

	page, err := svc.ListAll(ctx, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)

	_, err = pkgRepo.Update(ctx, ownerPackage.ID, ownerPackage.Name, ownerPackage.Description, string(pkg.PackageVisibilityPrivate), ownerPackage.Tags)
	require.NoError(t, err)

	page, err = svc.ListAll(ctx, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Zero(t, page.TotalElements)
	assert.Empty(t, page.Content)

	_, err = svc.GetByID(ctx, created.ID)
	require.ErrorIs(t, err, listing.ErrNotFound)
	_, err = svc.GetBySlug(ctx, created.Slug)
	require.ErrorIs(t, err, listing.ErrNotFound)
}

func applyMarketplaceMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "migrations", "public", name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
}

func assertMarketplaceListingCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()
	var got int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.marketplace_listing").Scan(&got))
	assert.Equal(t, want, got)
}
