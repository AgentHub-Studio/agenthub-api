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

const latMigration = "000069_seed_learning_annotation_default_templates.up.sql"
const latMigrationDown = "000069_seed_learning_annotation_default_templates.down.sql"

func TestIntegration_CoreLAT_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedLATTemplateRowCount, len(got))
}

func TestIntegration_CoreLAT_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLAT_FindBySlug_AgentToolBindingShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "agent-tool-binding-pattern")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "pattern", tmpl.Kind)
	assert.Equal(t, "beginner", tmpl.Audience)
	assert.NotEmpty(t, tmpl.Title)
	assert.NotEmpty(t, tmpl.Content)
}

func TestIntegration_CoreLAT_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreLAT_LoadByKind_PatternSubset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	patterns, err := loader.LoadByKind(context.Background(), "pattern")
	require.NoError(t, err)
	// seed has 2 pattern rows
	assert.Equal(t, 2, len(patterns))
	for _, p := range patterns {
		assert.Equal(t, "pattern", p.Kind)
	}
}

func TestIntegration_CoreLAT_LoadByAudience_BeginnerSubset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	beginners, err := loader.LoadByAudience(context.Background(), "beginner")
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range beginners {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedBeginnerLATTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLAT_LoadByAudience_IntermediateSubset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	intermediates, err := loader.LoadByAudience(context.Background(), "intermediate")
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range intermediates {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedIntermediateLATTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLAT_AllSlugsMatchKebabRegex(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.LearningAnnotationTemplateSeedSlugRE.MatchString(t2.Slug),
			"slug %q must match kebab regex", t2.Slug)
	}
}

func TestIntegration_CoreLAT_AllKindsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{
		"pattern": true, "anti_pattern": true, "tip": true,
		"optimization": true, "knowledge_gap": true,
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Kind],
			"%s kind %q outside closed set", t2.Slug, t2.Kind)
	}
}

func TestIntegration_CoreLAT_AllAudiencesInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{
		"beginner": true, "intermediate": true, "advanced": true,
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Audience],
			"%s audience %q outside closed set", t2.Slug, t2.Audience)
	}
}

func TestIntegration_CoreLAT_DBCheckRejectsInvalidKind(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.learning_annotation_template
		    (id, slug, kind, audience, title, content, sort_order)
		VALUES
		    ('cccccccc-0001-0000-0000-000000000001', 'bad-kind',
		     'lesson', 'beginner', 'Bad', 'Bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject kind='lesson'")
}

func TestIntegration_CoreLAT_DBCheckRejectsInvalidAudience(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.learning_annotation_template
		    (id, slug, kind, audience, title, content, sort_order)
		VALUES
		    ('cccccccc-0002-0000-0000-000000000002', 'bad-audience',
		     'tip', 'expert', 'Bad', 'Bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject audience='expert'")
}

func TestIntegration_CoreLAT_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedLATTemplateRowCount, len(got))
}

func TestIntegration_CoreLAT_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)
	applyMigration(t, pool, migDir, latMigrationDown)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLAT_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedLATTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLAT_AllSlugsUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug], "duplicate slug %q", t2.Slug)
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreLAT_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, latMigration)

	loader := core.NewCoreLearningAnnotationDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].Slug, all[i].Slug)
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder)
		}
	}
	assert.Equal(t, "agent-tool-binding-pattern", all[0].Slug)
}
