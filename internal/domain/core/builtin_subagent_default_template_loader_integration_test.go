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

const bsdMigration = "000061_seed_builtin_subagent_default_templates.up.sql"
const bsdMigrationDown = "000061_seed_builtin_subagent_default_templates.down.sql"

func TestIntegration_CoreBSD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBSDTemplateRowCount, len(got))
}

func TestIntegration_CoreBSD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBSD_FindBySlug_ResearcherShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "researcher-baseline")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "researcher", tmpl.Role)
	assert.Equal(t, "documentation-readonly-allowlist", tmpl.DefaultToolsetPolicySlug)
	assert.Equal(t, "extend-parent-rights", tmpl.DefaultInheritanceModeSlug)
	assert.Equal(t, "success-with-artifacts", tmpl.DefaultSummaryShapeSlug)
	assert.False(t, tmpl.RequiresAdminReview)
	assert.True(t, tmpl.IsRecommended)
}

func TestIntegration_CoreBSD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreBSD_LoadByRole_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	for _, role := range core.SeedExpectedBSDTemplateRoles {
		matched, err := loader.LoadByRole(context.Background(), role)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched),
			"role %q must have exactly 1 baseline template (1:1)", role)
	}
}

func TestIntegration_CoreBSD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedBSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreBSD_AdminReviewIsEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.False(t, p.RequiresAdminReview,
			"%s should not require admin review", p.Slug)
	}
}

func TestIntegration_CoreBSD_AllRolesInSUB002Enum(t *testing.T) {
	// Cross-feature invariant: every seeded role must exist in the
	// SUB-002 BuiltinSubagentRole enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, r := range core.SeedExpectedBSDTemplateRoles {
		allowed[r] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.Role],
			"%s role %q outside SUB-002 enum", p.Slug, p.Role)
	}
}

func TestIntegration_CoreBSD_ToolsetRefsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedBSDTemplateToolsetSlugs {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.DefaultToolsetPolicySlug],
			"%s toolset slug %q outside SUB-005 closed set",
			p.Slug, p.DefaultToolsetPolicySlug)
	}
}

func TestIntegration_CoreBSD_InheritanceRefsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedBSDTemplateInheritanceSlugs {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.DefaultInheritanceModeSlug],
			"%s inheritance slug %q outside SUB-006 closed set",
			p.Slug, p.DefaultInheritanceModeSlug)
	}
}

func TestIntegration_CoreBSD_SummaryRefsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedBSDTemplateSummarySlugs {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.DefaultSummaryShapeSlug],
			"%s summary slug %q outside SUB-010 closed set",
			p.Slug, p.DefaultSummaryShapeSlug)
	}
}

func TestIntegration_CoreBSD_ReviewerIsRestrictedToReadOnly(t *testing.T) {
	// Cross-feature invariant: reviewer must not extend parent's write
	// rights — review subagents must not mutate code under review.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "reviewer-baseline")
	require.NoError(t, err)
	assert.Equal(t, "restrict-to-readonly", tmpl.DefaultInheritanceModeSlug)
}

func TestIntegration_CoreBSD_PlannerReturnsPlanOnlySummary(t *testing.T) {
	// Cross-feature invariant: planner must not implement — summary
	// shape must be plan-only (SUB-010 plan shape).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "planner-baseline")
	require.NoError(t, err)
	assert.Equal(t, "plan-only", tmpl.DefaultSummaryShapeSlug)
}

func TestIntegration_CoreBSD_CoderInheritsParentRights(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "coder-baseline")
	require.NoError(t, err)
	assert.Equal(t, "extend-parent-rights", tmpl.DefaultInheritanceModeSlug)
	assert.Equal(t, "code-write-scoped", tmpl.DefaultToolsetPolicySlug)
}

func TestIntegration_CoreBSD_AllSystemPromptsNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.SystemPromptTemplate), 40,
			"%s system_prompt_template", p.Slug)
	}
}

func TestIntegration_CoreBSD_AllSlugsUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreBSD_AllRolesUniqueInDB(t *testing.T) {
	// 1:1 role-to-template mapping must hold at DB level too.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Role], "duplicate role %q", p.Role)
		seen[p.Role] = true
	}
}

func TestIntegration_CoreBSD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
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
	assert.Equal(t, "researcher-baseline", all[0].Slug)
}

func TestIntegration_CoreBSD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBSDTemplateRowCount, len(got))
}

func TestIntegration_CoreBSD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)
	applyMigration(t, pool, migDir, bsdMigrationDown)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBSD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsdMigration)

	loader := core.NewCoreBuiltinSubagentDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedBSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
