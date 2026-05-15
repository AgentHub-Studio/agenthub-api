//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreEmbeddingProviderLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CoreEmbeddingProvider_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedEmbeddingProviderSlugs), len(got))
	gotSlugs := map[string]core.CoreEmbeddingProvider{}
	for _, p := range got {
		gotSlugs[p.Slug] = p
	}
	for _, s := range core.SeedExpectedEmbeddingProviderSlugs {
		assert.Contains(t, gotSlugs, s)
	}
}

func TestIntegration_CoreEmbeddingProvider_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error")
	assert.Empty(t, got)
}

func TestIntegration_CoreEmbeddingProvider_FindBySlug_ReturnsKnownProvider(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	p, found, err := loader.FindBySlug(context.Background(), "local/e5-large")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "local", p.Provider)
	assert.Equal(t, 1024, p.Dimensions, "E5-Large is 1024-dim per CLAUDE.md")
	assert.True(t, p.IsRecommended)
	assert.Equal(t, 0.0, p.CostPer1MTokensUSD, "local provider cost = 0")
}

func TestIntegration_CoreEmbeddingProvider_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "fake/model")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreEmbeddingProvider_LoadByDimensions_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadByDimensions(context.Background(), 1024)
	require.NoError(t, err)
	require.NotEmpty(t, got, "1024-dim providers exist (E5-Large)")
	for _, p := range got {
		assert.Equal(t, 1024, p.Dimensions)
	}
}

func TestIntegration_CoreEmbeddingProvider_LoadByDimensions_UnknownReturnsEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadByDimensions(context.Background(), 99999)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEmbeddingProvider_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, p := range got {
		dbRec = append(dbRec, p.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedEmbeddingProviderSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec,
		"DB recommended set must EXACTLY match SeedRecommendedEmbeddingProviderSlugs")
}

func TestIntegration_CoreEmbeddingProvider_AllProvidersAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedEmbeddingProviders {
		allowed[p] = true
	}
	for _, p := range got {
		assert.True(t, allowed[p.Provider],
			"DB provider %q for %q outside allowlist", p.Provider, p.Slug)
	}
}

func TestIntegration_CoreEmbeddingProvider_AllDimensionsArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, p := range got {
		assert.Greater(t, p.Dimensions, 0,
			"provider %q dimensions must be > 0", p.Slug)
		assert.LessOrEqual(t, p.Dimensions, 4096,
			"provider %q dimensions suspiciously large (%d)", p.Slug, p.Dimensions)
	}
}

func TestIntegration_CoreEmbeddingProvider_LocalAndOllamaAreFree(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, p := range got {
		if p.Provider == "local" || p.Provider == "ollama" {
			assert.Equal(t, 0.0, p.CostPer1MTokensUSD,
				"%q is %s — must be free (no external cost)", p.Slug, p.Provider)
		}
	}
}

func TestIntegration_CoreEmbeddingProvider_PaidProvidersHavePositiveCost(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, p := range got {
		if p.Provider == "openai" {
			assert.Greater(t, p.CostPer1MTokensUSD, 0.0,
				"openai provider %q must have positive cost", p.Slug)
		}
	}
}

func TestIntegration_CoreEmbeddingProvider_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, p := range got {
		assert.False(t, seen[p.Slug], "duplicate slug %q in DB", p.Slug)
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreEmbeddingProvider_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, p := range got {
		assert.GreaterOrEqual(t, len(p.Description), 30,
			"provider %q description too short (%d chars)", p.Slug, len(p.Description))
	}
}

func TestIntegration_CoreEmbeddingProvider_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreEmbeddingProvider_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, p := range got {
		dbSlugs = append(dbSlugs, p.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedEmbeddingProviderSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreEmbeddingProvider_SlugProviderPrefixMatchesProviderColumn(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, p := range got {
		idx := strings.Index(p.Slug, "/")
		require.Greater(t, idx, 0)
		assert.Equal(t, p.Slug[:idx], p.Provider,
			"slug prefix must match provider column for %q", p.Slug)
	}
}

func TestIntegration_CoreEmbeddingProvider_DefaultExistsAndIsRecommended(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	def, found, err := loader.FindBySlug(context.Background(), core.SeedDefaultEmbeddingProviderSlug)
	require.NoError(t, err)
	require.True(t, found, "default %q must exist in DB", core.SeedDefaultEmbeddingProviderSlug)
	assert.True(t, def.IsRecommended, "default must also be recommended")
}

func TestIntegration_CoreEmbeddingProvider_MaxInputTokensIsPositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")

	loader := core.NewCoreEmbeddingProviderLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, p := range got {
		assert.Greater(t, p.MaxInputTokens, 0,
			"provider %q max_input_tokens must be > 0", p.Slug)
	}
}
