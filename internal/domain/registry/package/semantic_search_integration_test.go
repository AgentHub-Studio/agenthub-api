//go:build integration

package pkg

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	fastembed "github.com/AgentHub-Studio/agenthub-api/internal/infra/embedding/fastembed"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_RegistrySemanticSearch_RanksAndRequeuesOnPublish(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	applyRegistryMigration(t, ctx, pool, "000007_package_registry_tables.up.sql")
	applyRegistryMigration(t, ctx, pool, "000018_package_registry_semantic_search.up.sql")

	repo := NewRepository(pool).WithQueryEmbedder(fastRegistryQueryEmbedder{})
	target, err := repo.Create(ctx, Package{
		Name:           "On-call Runbook",
		Slug:           "on-call-runbook",
		Description:    "Procedures for production outage recovery",
		Tags:           []string{"sre", "incident"},
		Type:           PackageTypeSkill,
		Visibility:     PackageVisibilityPublic,
		AuthorTenantID: "tenant-a",
	})
	require.NoError(t, err)
	distractor, err := repo.Create(ctx, Package{
		Name:           "Data Catalog",
		Slug:           "data-catalog",
		Description:    "Browse datasets and ownership metadata",
		Tags:           []string{"analytics"},
		Type:           PackageTypeTool,
		Visibility:     PackageVisibilityPublic,
		AuthorTenantID: "tenant-a",
	})
	require.NoError(t, err)

	assertRegistryEmbeddingStatus(t, ctx, pool, target.ID.String(), "PENDING", false)
	storeRegistryEmbedding(t, ctx, pool, target)
	storeRegistryEmbedding(t, ctx, pool, distractor)

	results, total, err := repo.Search(ctx, "production incident recovery", nil, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.NotEmpty(t, results)
	require.Equal(t, target.ID, results[0].ID)
	require.Positive(t, results[0].Relevance)

	lexicalResults, lexicalTotal, err := NewRepository(pool).Search(
		ctx,
		"On-call",
		nil,
		pagination.PageRequest{Page: 0, Size: 20},
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), lexicalTotal)
	require.Equal(t, target.ID, lexicalResults[0].ID)
	require.Equal(t, 0.35, lexicalResults[0].Relevance)

	require.NoError(t, repo.UpdateLatestVersion(ctx, target.ID, "1.0.0"))
	assertRegistryEmbeddingStatus(t, ctx, pool, target.ID.String(), "PENDING", false)
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

func storeRegistryEmbedding(t *testing.T, ctx context.Context, pool *pgxpool.Pool, p Package) {
	t.Helper()
	vector, err := fastembed.New().Embed(p.Name + "\n" + p.Description + "\n" + p.Slug)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		UPDATE public.package_registry
		   SET embedding = $2::vector,
		       embedding_model = $3,
		       embedding_source_hash = 'integration-test',
		       embedding_status = 'COMPLETED',
		       embedded_at = NOW()
		 WHERE id = $1`, p.ID, float32SliceToVector(vector), fastembed.ModelName)
	require.NoError(t, err)
}

type fastRegistryQueryEmbedder struct{}

func (fastRegistryQueryEmbedder) Embed(_ context.Context, text string) (QueryEmbedding, error) {
	vector, err := fastembed.New().Embed(text)
	if err != nil {
		return QueryEmbedding{}, err
	}
	return QueryEmbedding{Vector: vector, Model: fastembed.ModelName}, nil
}

func assertRegistryEmbeddingStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, wantStatus string, wantEmbedding bool) {
	t.Helper()
	var status string
	var hasEmbedding bool
	err := pool.QueryRow(ctx, `
		SELECT embedding_status, embedding IS NOT NULL
		  FROM public.package_registry
		 WHERE id = $1`, id).Scan(&status, &hasEmbedding)
	require.NoError(t, err)
	require.Equal(t, wantStatus, status)
	require.Equal(t, wantEmbedding, hasEmbedding)
}
