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

// Integration tests for CoreLLMModelLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CoreLLMModel_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedLLMModelSlugs), len(got),
		"DB count must match SeedExpectedLLMModelSlugs canonical count")
	gotSlugs := map[string]core.CoreLLMModel{}
	for _, m := range got {
		gotSlugs[m.Slug] = m
	}
	for _, s := range core.SeedExpectedLLMModelSlugs {
		assert.Contains(t, gotSlugs, s, "DB must contain seeded slug %q", s)
	}
}

func TestIntegration_CoreLLMModel_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error")
	assert.Empty(t, got)
}

func TestIntegration_CoreLLMModel_FindBySlug_ReturnsKnownModel(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "anthropic/claude-haiku-4-5")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "anthropic/claude-haiku-4-5", m.Slug)
	assert.Equal(t, "anthropic", m.Provider)
	assert.Equal(t, "claude-haiku-4-5-20251001", m.ModelName)
	assert.Equal(t, 200000, m.ContextWindow)
	assert.True(t, m.SupportsTools)
	assert.True(t, m.SupportsVision)
	assert.True(t, m.IsRecommended)
}

func TestIntegration_CoreLLMModel_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "fake/model")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreLLMModel_LoadByProvider_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadByProvider(context.Background(), "anthropic")
	require.NoError(t, err)
	assert.Equal(t, 2, len(got), "anthropic provider has 2 models")
	for _, m := range got {
		assert.Equal(t, "anthropic", m.Provider)
	}
}

func TestIntegration_CoreLLMModel_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRecommended := []string{}
	for _, m := range got {
		dbRecommended = append(dbRecommended, m.Slug)
	}
	sort.Strings(dbRecommended)
	expected := append([]string{}, core.SeedRecommendedLLMModelSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRecommended,
		"DB recommended set must EXACTLY match SeedRecommendedLLMModelSlugs")
}

func TestIntegration_CoreLLMModel_AllProvidersAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedLLMModelProviders {
		allowed[p] = true
	}
	for _, m := range got {
		assert.True(t, allowed[m.Provider],
			"DB model %q has provider %q outside allowlist", m.Slug, m.Provider)
	}
}

func TestIntegration_CoreLLMModel_AllSeedModelsSupportTools(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		assert.True(t, m.SupportsTools,
			"seed model %q must support tools — agentic loop requires tool_calls", m.Slug)
	}
}

func TestIntegration_CoreLLMModel_AllSeedModelsSupportStreaming(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		assert.True(t, m.SupportsStreaming,
			"seed model %q must support streaming — SSE delivery requires it", m.Slug)
	}
}

func TestIntegration_CoreLLMModel_OllamaCostIsZero(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadByProvider(context.Background(), "ollama")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	for _, m := range got {
		assert.Equal(t, 0.0, m.CostPer1MInputTokensUSD,
			"ollama model %q must have 0 input cost (local)", m.Slug)
		assert.Equal(t, 0.0, m.CostPer1MOutputTokensUSD,
			"ollama model %q must have 0 output cost (local)", m.Slug)
	}
}

func TestIntegration_CoreLLMModel_PaidModelsHaveNonZeroCost(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		if m.Provider == "ollama" {
			continue
		}
		assert.Greater(t, m.CostPer1MInputTokensUSD, 0.0,
			"non-ollama model %q must have non-zero input cost (cost-aware policy)", m.Slug)
		assert.Greater(t, m.CostPer1MOutputTokensUSD, 0.0,
			"non-ollama model %q must have non-zero output cost", m.Slug)
		assert.GreaterOrEqual(t, m.CostPer1MOutputTokensUSD, m.CostPer1MInputTokensUSD,
			"output cost must be >= input cost for %q (LLM economics invariant — flagship models charge more for output, commodity models tie)", m.Slug)
	}
}

func TestIntegration_CoreLLMModel_AllContextWindowsArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		assert.Greater(t, m.ContextWindow, 0,
			"model %q context_window must be > 0", m.Slug)
		assert.GreaterOrEqual(t, m.ContextWindow, 4096,
			"model %q context_window too small for agentic use (< 4K)", m.Slug)
	}
}

func TestIntegration_CoreLLMModel_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		assert.NotEmpty(t, m.Description,
			"model %q must have description for UI picker", m.Slug)
		assert.GreaterOrEqual(t, len(m.Description), 30,
			"model %q description too short (%d chars)", m.Slug, len(m.Description))
	}
}

func TestIntegration_CoreLLMModel_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, m := range got {
		assert.False(t, seen[m.Slug], "duplicate slug %q in DB", m.Slug)
		seen[m.Slug] = true
	}
}

func TestIntegration_CoreLLMModel_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000014_seed_llm_models.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "post-down LoadAll must NOT error")
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreLLMModel_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, m := range got {
		dbSlugs = append(dbSlugs, m.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedLLMModelSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedLLMModelSlugs")
}

func TestIntegration_CoreLLMModel_SlugProviderPrefixMatchesProviderColumn(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		idx := strings.Index(m.Slug, "/")
		require.Greater(t, idx, 0, "slug %q must contain /", m.Slug)
		prefix := m.Slug[:idx]
		assert.Equal(t, prefix, m.Provider,
			"slug prefix %q must match provider column %q for %q",
			prefix, m.Provider, m.Slug)
	}
}

func TestIntegration_CoreLLMModel_DefaultTemperatureWithinSensibleRange(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		assert.GreaterOrEqual(t, m.DefaultTemperature, 0.0,
			"model %q default_temperature must be >= 0", m.Slug)
		assert.LessOrEqual(t, m.DefaultTemperature, 2.0,
			"model %q default_temperature must be <= 2.0", m.Slug)
	}
}

func TestIntegration_CoreLLMModel_DefaultMaxTokensFitInContextWindow(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000014_seed_llm_models.up.sql")

	loader := core.NewCoreLLMModelLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range got {
		assert.LessOrEqual(t, m.DefaultMaxTokens, m.ContextWindow,
			"model %q default_max_tokens (%d) must fit in context_window (%d)",
			m.Slug, m.DefaultMaxTokens, m.ContextWindow)
	}
}
