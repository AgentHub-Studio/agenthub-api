//go:build integration

package pkg

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// TestIntegration_RegistrySemanticSearch_UsesWorkerEmbeddingService is run by
// the Docker harness after the embedding worker has indexed a package with the
// official AgentHub embedding service.
func TestIntegration_RegistrySemanticSearch_UsesWorkerEmbeddingService(t *testing.T) {
	dsn := os.Getenv("REGISTRY_TEST_DSN")
	serviceURL := os.Getenv("REGISTRY_TEST_EMBEDDING_URL")
	if dsn == "" || serviceURL == "" {
		t.Skip("REGISTRY_TEST_DSN and REGISTRY_TEST_EMBEDDING_URL are required")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewRepository(pool).WithQueryEmbedder(NewHTTPQueryEmbedder(serviceURL))
	results, total, err := repo.Search(
		context.Background(),
		"production incident recovery",
		nil,
		pagination.PageRequest{Page: 0, Size: 20},
	)
	require.NoError(t, err)
	require.Positive(t, total)
	require.NotEmpty(t, results)
	require.True(t, strings.HasPrefix(results[0].Slug, "incident-recovery-"))
	require.Positive(t, results[0].Relevance)
}
