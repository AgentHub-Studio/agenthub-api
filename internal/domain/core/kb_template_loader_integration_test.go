//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreKnowledgeBaseTemplateLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CoreKBTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedKBTemplateSlugs), len(got))
}

func TestIntegration_CoreKBTemplate_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreKBTemplate_FindBySlug_ReturnsKnownTemplate(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "faq-template")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "faq", tmpl.TemplateKind)
	assert.Equal(t, "sentence", tmpl.ChunkStrategy)
	assert.Equal(t, 256, tmpl.ChunkSizeTokens, "FAQ uses 256-token chunks")
	assert.True(t, tmpl.IsRecommended)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreKBTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "fake-template")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreKBTemplate_LoadByKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadByKind(context.Background(), "faq")
	require.NoError(t, err)
	require.Len(t, got, 1, "1 faq template expected")
	assert.Equal(t, "faq-template", got[0].Slug)
}

func TestIntegration_CoreKBTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedKBTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CoreKBTemplate_AdminReviewSetMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbAdmin := []string{}
	for _, tmpl := range got {
		if tmpl.RequiresAdminReview {
			dbAdmin = append(dbAdmin, tmpl.Slug)
		}
	}
	sort.Strings(dbAdmin)
	expected := append([]string{}, core.SeedAdminReviewRequiredKBTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbAdmin,
		"DB admin-review set must EXACTLY match SeedAdminReviewRequiredKBTemplateSlugs")
}

func TestIntegration_CoreKBTemplate_AllKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedKBTemplateKinds {
		allowed[k] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.TemplateKind], "kind %q outside allowlist", tmpl.TemplateKind)
	}
}

func TestIntegration_CoreKBTemplate_AllChunkStrategiesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedKBTemplateChunkStrategies {
		allowed[s] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.ChunkStrategy], "strategy %q outside allowlist", tmpl.ChunkStrategy)
	}
}

func TestIntegration_CoreKBTemplate_AllChunkSizesArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.Greater(t, tmpl.ChunkSizeTokens, 0,
			"template %q chunk_size_tokens must be > 0", tmpl.Slug)
		assert.GreaterOrEqual(t, tmpl.ChunkOverlapTokens, 0,
			"template %q chunk_overlap_tokens must be >= 0", tmpl.Slug)
		assert.Less(t, tmpl.ChunkOverlapTokens, tmpl.ChunkSizeTokens,
			"overlap must be < chunk size for %q", tmpl.Slug)
	}
}

func TestIntegration_CoreKBTemplate_RecommendedTopKIsSensible(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, tmpl.RecommendedTopK, 1,
			"template %q top_k must be >= 1", tmpl.Slug)
		assert.LessOrEqual(t, tmpl.RecommendedTopK, 20,
			"template %q top_k > 20 likely too many results", tmpl.Slug)
	}
}

func TestIntegration_CoreKBTemplate_DefaultEmbeddingProviderSlugMatchesEmbeddingCatalog(t *testing.T) {
	// Cross-table integrity: every kb_template's default_embedding_provider_slug
	// must reference an existing ah_core.embedding_provider.slug.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	embeddingLoader := core.NewCoreEmbeddingProviderLoader(pool)
	templateLoader := core.NewCoreKnowledgeBaseTemplateLoader(pool)

	embeddings, _ := embeddingLoader.LoadAll(context.Background())
	templates, _ := templateLoader.LoadAll(context.Background())

	embeddingSlugs := map[string]bool{}
	for _, e := range embeddings {
		embeddingSlugs[e.Slug] = true
	}
	for _, tmpl := range templates {
		assert.True(t, embeddingSlugs[tmpl.DefaultEmbeddingProviderSlug],
			"template %q references embedding %q that does not exist",
			tmpl.Slug, tmpl.DefaultEmbeddingProviderSlug)
	}
}

func TestIntegration_CoreKBTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug], "duplicate slug %q", tmpl.Slug)
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreKBTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30,
			"template %q description too short (%d chars)", tmpl.Slug, len(tmpl.Description))
	}
}

func TestIntegration_CoreKBTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreKBTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedKBTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreKBTemplate_RegulatoryUsesHighestQualityEmbedding(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000015_seed_embedding_providers.up.sql")
	applyMigration(t, pool, migDir, "000017_seed_knowledge_base_templates.up.sql")

	loader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "regulatory-template")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "openai/text-embedding-3-large", tmpl.DefaultEmbeddingProviderSlug,
		"regulatory uses highest-quality embedding (compliance critical)")
	assert.True(t, tmpl.RequiresAdminReview)
}
