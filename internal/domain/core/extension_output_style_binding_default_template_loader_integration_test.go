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

const eosbdMigration = "000043_seed_extension_output_style_binding_default_templates.up.sql"
const eosbdMigrationDown = "000043_seed_extension_output_style_binding_default_templates.down.sql"

func TestIntegration_CoreEOSBD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedEOSBDTemplateRowCount, len(got))
}

func TestIntegration_CoreEOSBD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEOSBD_FindBySlug_PlatformDefaultShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "platform-conversational-default")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "platform", tmpl.TargetScope)
	assert.Equal(t, "markdown", tmpl.TargetFormat)
	assert.Equal(t, "conversational", tmpl.StyleSlug)
	assert.Equal(t, 10, tmpl.DefaultPriority)
}

func TestIntegration_CoreEOSBD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreEOSBD_LoadByScope_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	// platform: conversational + plain = 2.
	platform, err := loader.LoadByScope(context.Background(), "platform")
	require.NoError(t, err)
	assert.Equal(t, 2, len(platform))

	// tenant: technical + html_sanitized = 2.
	tenant, _ := loader.LoadByScope(context.Background(), "tenant")
	assert.Equal(t, 2, len(tenant))

	agent, _ := loader.LoadByScope(context.Background(), "agent")
	assert.Equal(t, 1, len(agent))

	explicit, _ := loader.LoadByScope(context.Background(), "explicit")
	assert.Equal(t, 1, len(explicit))
}

func TestIntegration_CoreEOSBD_LoadByFormat_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	// markdown: platform-conversational + tenant-technical + explicit-debug = 3.
	md, err := loader.LoadByFormat(context.Background(), "markdown")
	require.NoError(t, err)
	assert.Equal(t, 3, len(md))

	json, _ := loader.LoadByFormat(context.Background(), "json")
	assert.Equal(t, 1, len(json))

	plain, _ := loader.LoadByFormat(context.Background(), "plain")
	assert.Equal(t, 1, len(plain))

	html, _ := loader.LoadByFormat(context.Background(), "html_sanitized")
	assert.Equal(t, 1, len(html))
}

func TestIntegration_CoreEOSBD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedEOSBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreEOSBD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewEOSBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreEOSBD_AllScopesAreInEXT009Enum(t *testing.T) {
	// Cross-feature invariant: every scope MUST be in EXT-009 enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedEOSBDTemplateScopes {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetScope],
			"%s scope %q outside EXT-009 enum", p.Slug, p.TargetScope)
	}
}

func TestIntegration_CoreEOSBD_AllFormatsAreInEXT009Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, f := range core.SeedExpectedEOSBDTemplateFormats {
		allowed[f] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetFormat])
	}
}

func TestIntegration_CoreEOSBD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedEOSBDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreEOSBD_AllScopesHaveExample(t *testing.T) {
	// Cross-row invariant: seed includes ≥1 example per EXT-009 scope.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	covered := map[string]bool{}
	for _, p := range all {
		covered[p.TargetScope] = true
	}
	for _, scope := range core.SeedExpectedEOSBDTemplateScopes {
		assert.True(t, covered[scope],
			"scope %q must have at least one example", scope)
	}
}

func TestIntegration_CoreEOSBD_ExplicitHasHighestPriority(t *testing.T) {
	// Cross-template ladder invariant: explicit-scope priority > agent
	// > tenant > platform (mirrors EXT-009 cascade order).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	explicit, _, _ := loader.FindBySlug(context.Background(), "explicit-debug-verbose")
	agent, _, _ := loader.FindBySlug(context.Background(), "agent-extractor-json")
	tenant, _, _ := loader.FindBySlug(context.Background(), "tenant-technical-default")
	platform, _, _ := loader.FindBySlug(context.Background(), "platform-conversational-default")

	assert.Greater(t, explicit.DefaultPriority, agent.DefaultPriority)
	assert.Greater(t, agent.DefaultPriority, tenant.DefaultPriority)
	assert.Greater(t, tenant.DefaultPriority, platform.DefaultPriority)
}

func TestIntegration_CoreEOSBD_HTMLSanitizedRequiresAdminReview(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "tenant-html-sanitized-ui")
	require.NoError(t, err)
	assert.True(t, tmpl.RequiresAdminReview)
	assert.Equal(t, "html_sanitized", tmpl.TargetFormat)
}

func TestIntegration_CoreEOSBD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreEOSBD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreEOSBD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
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
	assert.Equal(t, "platform-conversational-default", all[0].Slug)
}

func TestIntegration_CoreEOSBD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedEOSBDTemplateRowCount, len(got))
}

func TestIntegration_CoreEOSBD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)
	applyMigration(t, pool, migDir, eosbdMigrationDown)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEOSBD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, eosbdMigration)

	loader := core.NewCoreExtensionOutputStyleBindingDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedEOSBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
