//go:build integration

package review_test

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
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/review"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_ReviewDeleteRequiresRouteListingMatch(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	applyReviewMigration(t, ctx, pool, "000005_llm_config_preset.up.sql")
	applyReviewMigration(t, ctx, pool, "000001_marketplace_listing.up.sql")
	applyReviewMigration(t, ctx, pool, "000008_fix_column_mismatches.up.sql")
	applyReviewMigration(t, ctx, pool, "000009_marketplace_listing_created_at.up.sql")
	applyReviewMigration(t, ctx, pool, "000002_marketplace_rating.up.sql")

	listingRepo := listing.NewRepository(pool)
	listingA := createReviewListing(t, ctx, listingRepo, "listing-a")
	listingB := createReviewListing(t, ctx, listingRepo, "listing-b")
	reviewRepo := review.NewRepository(pool)
	svc := review.NewService(reviewRepo, listingRepo)

	created, err := svc.Create(ctx, listingA.ID, "reviewer-tenant", review.CreateRequest{Rating: 5, Comment: "belongs to A"})
	require.NoError(t, err)

	err = svc.Delete(ctx, listingB.ID, created.ID, "reviewer-tenant")
	require.ErrorIs(t, err, review.ErrNotFound)

	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.marketplace_rating WHERE id = $1", created.ID).Scan(&count))
	assert.Equal(t, 1, count)

	stats, err := reviewRepo.GetRatingStats(ctx, listingA.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.Count)
}

func createReviewListing(t *testing.T, ctx context.Context, repo *listing.Repository, slug string) listing.Listing {
	t.Helper()
	created, err := repo.Create(ctx, listing.Listing{
		ID:        uuid.New(),
		TenantID:  "owner-tenant",
		PackageID: uuid.New(),
		Name:      slug,
		Slug:      slug,
		Type:      listing.PackageTypeAgent,
		Status:    listing.StatusActive,
	})
	require.NoError(t, err)
	return created
}

func applyReviewMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "migrations", "public", name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
}
