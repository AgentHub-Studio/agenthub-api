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

// Integration tests for CorePolicyEngineTemplateLoader against real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CorePolicyTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedPolicyTemplateSlugs), len(got))
}

func TestIntegration_CorePolicyTemplate_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePolicyTemplate_FindBySlug_ReturnsKnownTemplate(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "generic-safety")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "generic_safety", tmpl.ComplianceProfile)
	assert.Equal(t, "static_deny", tmpl.EngineKind)
	assert.True(t, tmpl.IsRecommended)
	assert.False(t, tmpl.RequiresAdminReview)

	denied := tmpl.DenyToolsList()
	assert.Contains(t, denied, "shell", "generic safety blocks shell")
	assert.Contains(t, denied, "git-force-push")
}

func TestIntegration_CorePolicyTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "fake")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePolicyTemplate_LoadByComplianceProfile_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadByComplianceProfile(context.Background(), "gdpr")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "gdpr-strict", got[0].Slug)
}

func TestIntegration_CorePolicyTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedPolicyTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CorePolicyTemplate_AdminReviewSetMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbAdmin := []string{}
	for _, tmpl := range got {
		if tmpl.RequiresAdminReview {
			dbAdmin = append(dbAdmin, tmpl.Slug)
		}
	}
	sort.Strings(dbAdmin)
	expected := append([]string{}, core.SeedAdminReviewRequiredPolicyTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbAdmin,
		"DB admin-review set must EXACTLY match SeedAdminReviewRequiredPolicyTemplateSlugs")
}

func TestIntegration_CorePolicyTemplate_AllProfilesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedPolicyTemplateComplianceProfiles {
		allowed[p] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.ComplianceProfile],
			"profile %q outside allowlist", tmpl.ComplianceProfile)
	}
}

func TestIntegration_CorePolicyTemplate_AllEngineKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedPolicyTemplateEngineKinds {
		allowed[k] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.EngineKind],
			"engine_kind %q outside allowlist", tmpl.EngineKind)
	}
}

func TestIntegration_CorePolicyTemplate_AllRegulatedTemplatesIncludeAuditObligation(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		obs := tmpl.ObligationsList()
		assert.Contains(t, obs, "audit_log",
			"template %q must include audit_log obligation (compliance baseline)", tmpl.Slug)
	}
}

func TestIntegration_CorePolicyTemplate_RegulatedTemplatesHaveDenyOrApprovalRules(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		denyCount := len(tmpl.DenyToolsList())
		approvalCount := len(tmpl.RequireApprovalToolsList())
		assert.Greater(t, denyCount+approvalCount, 0,
			"template %q has zero deny + zero approval rules — useless template", tmpl.Slug)
	}
}

func TestIntegration_CorePolicyTemplate_GDPRTemplateBlocksNonEUExport(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "gdpr-strict")
	require.NoError(t, err)
	require.True(t, found)
	denied := tmpl.DenyToolsList()
	assert.Contains(t, denied, "export-data-non-eu",
		"GDPR template must block non-EU export by default")
	obs := tmpl.ObligationsList()
	assert.Contains(t, obs, "gdpr_consent_check",
		"GDPR obligations must include consent check")
}

func TestIntegration_CorePolicyTemplate_HIPAATemplateBlocksPHIExternalShare(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "hipaa-strict")
	require.NoError(t, err)
	require.True(t, found)
	denied := tmpl.DenyToolsList()
	assert.Contains(t, denied, "share-phi-non-baa-vendor",
		"HIPAA template must block PHI share to non-BAA vendors")
}

func TestIntegration_CorePolicyTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CorePolicyTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30,
			"template %q description too short", tmpl.Slug)
	}
}

func TestIntegration_CorePolicyTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CorePolicyTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedPolicyTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CorePolicyTemplate_KebabCaseSlugsInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000018_seed_policy_engine_templates.up.sql")

	loader := core.NewCorePolicyEngineTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.False(t, strings.Contains(tmpl.Slug, "_"),
			"DB slug %q must use kebab-case", tmpl.Slug)
	}
}
