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

const tibMigration = "000074_seed_tool_invocation_budget_default_templates.up.sql"
const tibMigrationDown = "000074_seed_tool_invocation_budget_default_templates.down.sql"

func TestIntegration_CoreTIBTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTIBTemplateRowCount, len(got))
}

func TestIntegration_CoreTIBTemplate_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTIBTemplate_FindBySlug_UnlimitedShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "unlimited")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Unlimited", tmpl.Label)
	assert.Equal(t, 0, tmpl.TotalCap)
	assert.Equal(t, "warn", tmpl.Policy)
	assert.Empty(t, tmpl.CategoryCaps)
}

func TestIntegration_CoreTIBTemplate_FindBySlug_ComplianceAuditIsReadOnly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "compliance-audit")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "deny", tmpl.Policy)
	mutateCap, ok := tmpl.CategoryCaps["mutate"]
	require.True(t, ok, "compliance-audit must have mutate cap")
	assert.Equal(t, 0, mutateCap, "compliance-audit mutate cap must be 0 (read-only)")
}

func TestIntegration_CoreTIBTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreTIBTemplate_LoadByPolicy_Deny(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadByPolicy(context.Background(), "deny")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	for _, tmpl := range got {
		assert.Equal(t, "deny", tmpl.Policy)
	}
}

func TestIntegration_CoreTIBTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTIBTemplateRowCount, len(got))
}

func TestIntegration_CoreTIBTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)
	applyMigration(t, pool, migDir, tibMigrationDown)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTIBTemplate_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedTIBTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTIBTemplate_AllSlugsMatchKebabRegex(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.SeedTIBTemplateSlugRE.MatchString(t2.Slug),
			"slug %q must match kebab regex", t2.Slug)
	}
}

func TestIntegration_CoreTIBTemplate_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
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
	assert.Equal(t, "unlimited", all[0].Slug)
}

func TestIntegration_CoreTIBTemplate_CategoryCapsDeserialiseCorrectly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tibMigration)

	loader := core.NewCoreToolInvocationBudgetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "batch-processing")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, tmpl.CategoryCaps)
	assert.Equal(t, 20, tmpl.CategoryCaps["mutate"])
	assert.Equal(t, 200, tmpl.CategoryCaps["external"])
}
