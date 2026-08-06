//go:build integration

package pkg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_RegistryPrivatePackageVisibility(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	applyRegistryMigration(t, ctx, pool, "000007_package_registry_tables.up.sql")
	applyRegistryMigration(t, ctx, pool, "000018_package_registry_semantic_search.up.sql")

	repo := NewRepository(pool)
	svc := NewService(repo)
	private, err := repo.Create(ctx, Package{
		Name:           "Owner private package",
		Slug:           "owner-private-package",
		Tags:           []string{},
		Type:           PackageTypeAgent,
		Visibility:     PackageVisibilityPrivate,
		AuthorTenantID: "owner",
	})
	require.NoError(t, err)
	public, err := repo.Create(ctx, Package{
		Name:           "Public package",
		Slug:           "public-package",
		Tags:           []string{},
		Type:           PackageTypeSkill,
		Visibility:     PackageVisibilityPublic,
		AuthorTenantID: "owner",
	})
	require.NoError(t, err)

	_, err = svc.GetAccessibleByID(ctx, private.ID, "")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = svc.GetAccessibleBySlug(ctx, private.Slug, "other")
	require.ErrorIs(t, err, ErrNotFound)

	ownerPrivate, err := svc.GetAccessibleByID(ctx, private.ID, "owner")
	require.NoError(t, err)
	require.Equal(t, private.ID, ownerPrivate.ID)

	anonymousPublic, err := svc.GetAccessibleBySlug(ctx, public.Slug, "")
	require.NoError(t, err)
	require.Equal(t, public.ID, anonymousPublic.ID)
}
