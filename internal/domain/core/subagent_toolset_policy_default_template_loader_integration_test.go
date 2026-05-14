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

const stpdMigration = "000051_seed_subagent_toolset_policy_default_templates.up.sql"
const stpdMigrationDown = "000051_seed_subagent_toolset_policy_default_templates.down.sql"

func TestIntegration_CoreSTPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSTPDTemplateRowCount, len(got))
}

func TestIntegration_CoreSTPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSTPD_FindBySlug_DocsShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "documentation-readonly-allowlist")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "explicit_allowlist", tmpl.TargetIsolationMode)
	assert.True(t, tmpl.RequiresAdminReview)
	allowed, err := tmpl.SampleAllowedToolNames()
	require.NoError(t, err)
	assert.Contains(t, allowed, "Read")
	assert.Contains(t, allowed, "document_search")
}

func TestIntegration_CoreSTPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSTPD_LoadByMode_OneToOneMapping(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	for _, mode := range core.SeedExpectedSTPDTemplateModes {
		matched, err := loader.LoadByIsolationMode(context.Background(), mode)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "mode %q must have exactly 1 template (1:1)", mode)
	}
}

func TestIntegration_CoreSTPD_LoadByUseCase(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedSTPDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CoreSTPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedSTPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSTPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewSTPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSTPD_AllModesInSUB005Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, m := range core.SeedExpectedSTPDTemplateModes {
		allowed[m] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetIsolationMode],
			"%s mode %q outside SUB-005 enum", t2.Slug, t2.TargetIsolationMode)
	}
}

func TestIntegration_CoreSTPD_AllowlistModeShipsNonEmptyNames(t *testing.T) {
	// Cross-row invariant: explicit_allowlist template MUST ship
	// non-empty sample_allowed_tool_names (else admin must invent them).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	allowlist, err := loader.LoadByIsolationMode(context.Background(), "explicit_allowlist")
	require.NoError(t, err)
	for _, t2 := range allowlist {
		names, err := t2.SampleAllowedToolNames()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(names), 1, "%s must ship sample names", t2.Slug)
	}
}

func TestIntegration_CoreSTPD_BlocklistModeShipsNonEmptyBlocks(t *testing.T) {
	// Cross-row invariant: parent_minus_blocklist must ship samples.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	blocked, err := loader.LoadByIsolationMode(context.Background(), "parent_minus_blocklist")
	require.NoError(t, err)
	for _, t2 := range blocked {
		names, err := t2.SampleBlockedToolNames()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(names), 1, "%s must ship sample blocks", t2.Slug)
	}
}

func TestIntegration_CoreSTPD_DepthFilteredHasPositiveThreshold(t *testing.T) {
	// Cross-row invariant: depth_filtered template must ship a
	// positive depth threshold (0 means the rule fires at every depth
	// > 0 which is sometimes intended, but the recursion-safety
	// template specifies threshold=1).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "recursion-safety-depth-filtered")
	require.NoError(t, err)
	assert.Equal(t, 1, tmpl.SampleDepthThresholdForAgent)
}

func TestIntegration_CoreSTPD_CategoricalModeShipsPatterns(t *testing.T) {
	// Cross-row invariant: categorical_exclusion must ship at least
	// one prefix OR suffix (SUB-005 Validate would reject empty).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	cat, err := loader.LoadByIsolationMode(context.Background(), "categorical_exclusion")
	require.NoError(t, err)
	for _, t2 := range cat {
		prefixes, err := t2.SampleCategoryPrefixes()
		require.NoError(t, err)
		suffixes, err := t2.SampleCategorySuffixes()
		require.NoError(t, err)
		assert.True(t, len(prefixes) > 0 || len(suffixes) > 0,
			"%s must ship at least one prefix or suffix", t2.Slug)
	}
}

func TestIntegration_CoreSTPD_AdminSurfaceBlocksAdminPrefix(t *testing.T) {
	// Cross-row invariant: admin-surface template uses the "admin_"
	// prefix marker.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "admin-surface-categorical-exclusion")
	require.NoError(t, err)
	prefixes, err := tmpl.SampleCategoryPrefixes()
	require.NoError(t, err)
	assert.Contains(t, prefixes, "admin_")
	suffixes, err := tmpl.SampleCategorySuffixes()
	require.NoError(t, err)
	assert.Contains(t, suffixes, "_dangerous")
}

func TestIntegration_CoreSTPD_NonAllowlistTemplatesHaveEmptyAllowSample(t *testing.T) {
	// Cross-row invariant: only explicit_allowlist mode populates
	// sample_allowed_tool_names; other modes leave it empty so admins
	// don't get misled.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetIsolationMode != "explicit_allowlist" {
			names, err := t2.SampleAllowedToolNames()
			require.NoError(t, err)
			assert.Empty(t, names,
				"%s (mode %s) must have empty sample_allowed_tool_names",
				t2.Slug, t2.TargetIsolationMode)
		}
	}
}

func TestIntegration_CoreSTPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreSTPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreSTPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "documentation-readonly-allowlist", all[0].Slug)
}

func TestIntegration_CoreSTPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSTPDTemplateRowCount, len(got))
}

func TestIntegration_CoreSTPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)
	applyMigration(t, pool, migDir, stpdMigrationDown)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSTPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stpdMigration)

	loader := core.NewCoreSubagentToolsetPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedSTPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
