package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreEmbeddingProvider represents a platform-managed embedding provider
// catalog entry. Tenants opt-in by selecting a slug for their KB's
// embedding_provider value. ah_core stores the catalog; API keys live
// in tenant settings.
//
// Inspired by PDF arXiv:2604.14228v1 §4 (RAG pipeline) + CLAUDE.md
// (agenthub-embedding service E5-Large 1024dim default).
type CoreEmbeddingProvider struct {
	ID                  uuid.UUID
	Slug                string
	DisplayName         string
	Description         string
	Provider            string
	ModelName           string
	Dimensions          int
	MaxInputTokens      int
	SupportsBatch       bool
	CostPer1MTokensUSD  float64
	IsRecommended       bool
	IsActive            bool
	SortOrder           int
}

// CoreEmbeddingProviderLoader loads platform-managed embedding catalog.
// Like other core loaders, non-fatal when schema missing.
type CoreEmbeddingProviderLoader struct {
	pool *pgxpool.Pool
}

// NewCoreEmbeddingProviderLoader creates a CoreEmbeddingProviderLoader.
func NewCoreEmbeddingProviderLoader(pool *pgxpool.Pool) *CoreEmbeddingProviderLoader {
	return &CoreEmbeddingProviderLoader{pool: pool}
}

// LoadAll returns all active providers, ordered by sort_order then slug.
func (l *CoreEmbeddingProviderLoader) LoadAll(ctx context.Context) ([]CoreEmbeddingProvider, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description,
		       provider, model_name, dimensions, max_input_tokens,
		       supports_batch, cost_per_1m_tokens_usd,
		       is_recommended, is_active, sort_order
		  FROM ah_core.embedding_provider
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.embedding_provider not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query embedding_provider: %w", err)
	}
	defer rows.Close()

	var providers []CoreEmbeddingProvider
	for rows.Next() {
		var p CoreEmbeddingProvider
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.DisplayName, &p.Description,
			&p.Provider, &p.ModelName, &p.Dimensions, &p.MaxInputTokens,
			&p.SupportsBatch, &p.CostPer1MTokensUSD,
			&p.IsRecommended, &p.IsActive, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan embedding_provider: %w", err)
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate embedding_provider: %w", err)
	}
	return providers, nil
}

// FindBySlug returns one provider by slug.
func (l *CoreEmbeddingProviderLoader) FindBySlug(ctx context.Context, slug string) (CoreEmbeddingProvider, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreEmbeddingProvider{}, false, err
	}
	for _, p := range all {
		if p.Slug == slug {
			return p, true, nil
		}
	}
	return CoreEmbeddingProvider{}, false, nil
}

// LoadByDimensions returns providers matching a specific dimension count
// — useful when the tenant's pgvector column is fixed-size and they
// need a compatible provider.
func (l *CoreEmbeddingProviderLoader) LoadByDimensions(ctx context.Context, dimensions int) ([]CoreEmbeddingProvider, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreEmbeddingProvider
	for _, p := range all {
		if p.Dimensions == dimensions {
			matched = append(matched, p)
		}
	}
	return matched, nil
}

// LoadRecommended returns providers flagged is_recommended.
func (l *CoreEmbeddingProviderLoader) LoadRecommended(ctx context.Context) ([]CoreEmbeddingProvider, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreEmbeddingProvider
	for _, p := range all {
		if p.IsRecommended {
			matched = append(matched, p)
		}
	}
	return matched, nil
}

// SeedExpectedEmbeddingProviderSlugs is the canonical list of slugs the
// seed migration 000015_seed_embedding_providers installs.
var SeedExpectedEmbeddingProviderSlugs = []string{
	"local/e5-large",
	"openai/text-embedding-3-small",
	"openai/text-embedding-3-large",
	"huggingface/bge-small-en-v1.5",
	"ollama/nomic-embed-text",
}

// SeedExpectedEmbeddingProviders is the closed set of providers seeded.
var SeedExpectedEmbeddingProviders = []string{
	"local",
	"openai",
	"huggingface",
	"ollama",
}

// SeedRecommendedEmbeddingProviderSlugs is the curated suggested subset.
var SeedRecommendedEmbeddingProviderSlugs = []string{
	"local/e5-large",
	"openai/text-embedding-3-small",
	"ollama/nomic-embed-text",
}

// SeedDefaultEmbeddingProviderSlug is the platform default — used when
// tenant doesn't pick one explicitly. Matches CLAUDE.md (agenthub-embedding
// E5-Large 1024dim is the historical AgentHub default).
const SeedDefaultEmbeddingProviderSlug = "local/e5-large"

// SeedExpectedEmbeddingDimensions is the closed set of dimension values
// the seed uses. Used for compatibility validation against pgvector
// column sizes.
var SeedExpectedEmbeddingDimensions = []int{384, 768, 1024, 1536, 3072}
