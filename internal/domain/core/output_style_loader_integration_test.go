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

// Integration tests for CoreOutputStyleLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CoreOutputStyle_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedOutputStyleSlugs), len(got),
		"DB row count must match SeedExpectedOutputStyleSlugs canonical count")
	gotSlugs := map[string]core.CoreOutputStyle{}
	for _, s := range got {
		gotSlugs[s.Slug] = s
	}
	for _, slug := range core.SeedExpectedOutputStyleSlugs {
		assert.Contains(t, gotSlugs, slug, "DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreOutputStyle_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got)
}

func TestIntegration_CoreOutputStyle_FindBySlug_ReturnsKnownStyle(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	style, found, err := loader.FindBySlug(context.Background(), "concise")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "concise", style.Slug)
	assert.Equal(t, 150, style.MaxWords, "concise must be capped at 150 words")
	assert.NotEmpty(t, style.PromptTemplate)
	assert.False(t, style.IsDefault, "concise is not platform default")
}

func TestIntegration_CoreOutputStyle_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreOutputStyle_FindDefault_ReturnsConversational(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	def, found, err := loader.FindDefault(context.Background())
	require.NoError(t, err)
	require.True(t, found, "platform default must exist")
	assert.Equal(t, core.SeedDefaultOutputStyleSlug, def.Slug)
	assert.Equal(t, "conversational", def.Slug)
	assert.True(t, def.IsDefault)
}

func TestIntegration_CoreOutputStyle_OnlyOneRowHasIsDefaultTrue(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	defaults := 0
	for _, s := range got {
		if s.IsDefault {
			defaults++
		}
	}
	assert.Equal(t, 1, defaults,
		"EXACTLY ONE row must have is_default=TRUE (schema unique partial index)")
}

func TestIntegration_CoreOutputStyle_PartialUniqueIndexRejectsSecondDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	// Try to insert a SECOND row with is_default=TRUE.
	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.output_style
		    (name, slug, prompt_template, is_default)
		VALUES
		    ('Second Default', 'second-default-test', 'x', TRUE)
	`)
	assert.Error(t, err,
		"partial unique index must REJECT a second is_default=TRUE row")
}

func TestIntegration_CoreOutputStyle_PartialUniqueIndexAllowsManyFalse(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	nonDefault := 0
	for _, s := range got {
		if !s.IsDefault {
			nonDefault++
		}
	}
	// 7 of the 8 seeded styles have is_default=false.
	assert.Equal(t, 7, nonDefault,
		"7 seeded rows must have is_default=FALSE — partial index permits multiple FALSEs")
}

func TestIntegration_CoreOutputStyle_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, s := range got {
		assert.False(t, seen[s.Slug], "duplicate slug %q in DB", s.Slug)
		seen[s.Slug] = true
	}
}

func TestIntegration_CoreOutputStyle_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000011_seed_output_styles.down.sql")

	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing table after down must NOT error")
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreOutputStyle_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, s := range got {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedOutputStyleSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedOutputStyleSlugs")
}

func TestIntegration_CoreOutputStyle_AllFormatsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, f := range core.SeedExpectedOutputStyleFormats {
		allowed[f] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.OutputFormat],
			"DB style %q references format %q outside SeedExpectedOutputStyleFormats",
			s.Slug, s.OutputFormat)
	}
}

func TestIntegration_CoreOutputStyle_AllAudiencesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, a := range core.SeedExpectedOutputStyleAudiences {
		allowed[a] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.Audience],
			"DB style %q references audience %q outside SeedExpectedOutputStyleAudiences",
			s.Slug, s.Audience)
	}
}

func TestIntegration_CoreOutputStyle_AllPromptTemplatesAreInformative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.NotEmpty(t, s.PromptTemplate,
			"style %q must have non-empty prompt_template", s.Slug)
		assert.GreaterOrEqual(t, len(s.PromptTemplate), 50,
			"style %q prompt_template too short (%d chars) — directive must be informative",
			s.Slug, len(s.PromptTemplate))
	}
}

func TestIntegration_CoreOutputStyle_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000011_seed_output_styles.up.sql")

	loader := core.NewCoreOutputStyleLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	prevSort := got[0].SortOrder - 1
	for i, s := range got {
		assert.GreaterOrEqual(t, s.SortOrder, prevSort,
			"style at index %d (sort %d) must have sort >= previous (%d)",
			i, s.SortOrder, prevSort)
		prevSort = s.SortOrder
	}
	// First sorted row must be the default (sort_order=10).
	assert.Equal(t, "conversational", got[0].Slug,
		"first row by sort_order must be conversational (sort=10)")
}
