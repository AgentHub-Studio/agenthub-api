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

const psrdMigration = "000037_seed_path_scoped_rule_default_templates.up.sql"
const psrdMigrationDown = "000037_seed_path_scoped_rule_default_templates.down.sql"

func TestIntegration_CorePSRD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPSRDTemplateRowCount, len(got))
}

func TestIntegration_CorePSRD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePSRD_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "no-secrets-in-go-files")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "file", tmpl.TargetScope)
	assert.Equal(t, "**/*.go", tmpl.PathGlob)
	assert.Equal(t, 80, tmpl.DefaultPriority)
}

func TestIntegration_CorePSRD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePSRD_LoadByScope_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	files, err := loader.LoadByScope(context.Background(), "file")
	require.NoError(t, err)
	assert.Equal(t, 3, len(files), "no-secrets-go + no-pii-yaml + docs-frontmatter")

	tools, _ := loader.LoadByScope(context.Background(), "tool")
	assert.Equal(t, 2, len(tools), "execute-sql + shell-rm-rf")

	dirs, _ := loader.LoadByScope(context.Background(), "directory")
	assert.Equal(t, 1, len(dirs))

	agents, _ := loader.LoadByScope(context.Background(), "agent")
	assert.Equal(t, 1, len(agents))

	globals, _ := loader.LoadByScope(context.Background(), "global")
	assert.Equal(t, 1, len(globals))
}

func TestIntegration_CorePSRD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPSRDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePSRD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPSRDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePSRD_AllScopesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedPSRDTemplateScopes {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetScope],
			"template %q has scope %q outside expected set", p.Slug, p.TargetScope)
	}
}

func TestIntegration_CorePSRD_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedPSRDTemplateTenantKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForTenantKind])
	}
}

func TestIntegration_CorePSRD_GlobalRuleHasHighestPriority(t *testing.T) {
	// Cross-row invariant: prompt injection (global) has the highest
	// default_priority — most critical safety boundary.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	maxPriority := 0
	maxSlug := ""
	for _, p := range all {
		if p.DefaultPriority > maxPriority {
			maxPriority = p.DefaultPriority
			maxSlug = p.Slug
		}
	}
	assert.Equal(t, "global-no-prompt-injection", maxSlug,
		"prompt injection global rule must have highest priority")
	assert.Equal(t, 100, maxPriority)
}

func TestIntegration_CorePSRD_AllPriorityValuesArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.Positive(t, p.DefaultPriority)
	}
}

func TestIntegration_CorePSRD_AllPathGlobsAreNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.NotEmpty(t, strings.TrimSpace(p.PathGlob),
			"template %q must have non-empty path_glob", p.Slug)
	}
}

func TestIntegration_CorePSRD_AllRuleContentsAreInformative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.RuleContent), 30,
			"template %q rule_content must be informative ≥30 chars", p.Slug)
	}
}

func TestIntegration_CorePSRD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CorePSRD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CorePSRD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
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
	assert.Equal(t, "no-secrets-in-go-files", all[0].Slug,
		"first by sort_order=10")
}

func TestIntegration_CorePSRD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPSRDTemplateRowCount, len(got))
}

func TestIntegration_CorePSRD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)
	applyMigration(t, pool, migDir, psrdMigrationDown)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePSRD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, psrdMigration)

	loader := core.NewCorePathScopedRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPSRDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
