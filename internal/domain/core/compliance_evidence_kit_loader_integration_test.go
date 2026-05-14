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

func TestIntegration_CoreEvidenceKit_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedComplianceEvidenceKitSlugs), len(got))
}

func TestIntegration_CoreEvidenceKit_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEvidenceKit_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	kit, found, err := loader.FindBySlug(context.Background(), "gdpr-quarterly-audit")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "gdpr", kit.ComplianceProfile)
	assert.Equal(t, 90, kit.CollectionPeriodDays)
	assert.True(t, kit.IncludesUserConsentLogs, "GDPR requires consent logs")
	assert.True(t, kit.RequiresAdminSignoff)
	assert.GreaterOrEqual(t, kit.RetentionDays, 365, "GDPR retention floor")
}

func TestIntegration_CoreEvidenceKit_LoadByComplianceProfile_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadByComplianceProfile(context.Background(), "gdpr")
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
}

func TestIntegration_CoreEvidenceKit_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, kit := range got {
		dbRec = append(dbRec, kit.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedComplianceEvidenceKitSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CoreEvidenceKit_AdminSignoffSetMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbAdmin := []string{}
	for _, kit := range got {
		if kit.RequiresAdminSignoff {
			dbAdmin = append(dbAdmin, kit.Slug)
		}
	}
	sort.Strings(dbAdmin)
	expected := append([]string{}, core.SeedAdminSignoffComplianceEvidenceKitSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbAdmin)
}

func TestIntegration_CoreEvidenceKit_AllProfilesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedComplianceEvidenceKitProfiles {
		allowed[p] = true
	}
	for _, kit := range got {
		assert.True(t, allowed[kit.ComplianceProfile])
	}
}

func TestIntegration_CoreEvidenceKit_AllExportFormatsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, f := range core.SeedExpectedComplianceEvidenceKitFormats {
		allowed[f] = true
	}
	for _, kit := range got {
		for _, format := range kit.ExportFormatsList() {
			assert.True(t, allowed[format],
				"kit %q has format %q outside allowlist", kit.Slug, format)
		}
	}
}

func TestIntegration_CoreEvidenceKit_GDPRMeetsRetentionFloor(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, kit := range got {
		if kit.ComplianceProfile == "gdpr" {
			assert.GreaterOrEqual(t, kit.RetentionDays, 365,
				"GDPR kits must retain ≥365 days (regulatory floor)")
		}
	}
}

func TestIntegration_CoreEvidenceKit_AllRegulatedKitsRequireAdminSignoff(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	regulatedProfiles := map[string]bool{
		"gdpr": true, "hipaa": true, "sox": true,
		"pci_dss": true, "iso27001": true,
	}
	for _, kit := range got {
		if !regulatedProfiles[kit.ComplianceProfile] {
			continue
		}
		assert.True(t, kit.RequiresAdminSignoff,
			"regulated kit %q (profile %q) must require admin signoff",
			kit.Slug, kit.ComplianceProfile)
	}
}

func TestIntegration_CoreEvidenceKit_RegulatedKitsAutoDeliverViaWebhook(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, kit := range got {
		if !kit.RequiresAdminSignoff {
			continue
		}
		assert.NotEmpty(t, kit.TargetWebhookTemplateSlug,
			"regulated kit %q must have target webhook for auto-delivery", kit.Slug)
	}
}

func TestIntegration_CoreEvidenceKit_TargetWebhookExistsInWebhookCatalog(t *testing.T) {
	// Cross-table integrity: target_webhook_template_slug must reference
	// existing ah_core.webhook_endpoint_template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	kitLoader := core.NewCoreComplianceEvidenceKitLoader(pool)
	webhookLoader := core.NewCoreWebhookEndpointTemplateLoader(pool)

	kits, _ := kitLoader.LoadAll(context.Background())
	webhooks, _ := webhookLoader.LoadAll(context.Background())

	webhookSlugs := map[string]bool{}
	for _, w := range webhooks {
		webhookSlugs[w.Slug] = true
	}
	for _, k := range kits {
		if k.TargetWebhookTemplateSlug == "" {
			continue
		}
		assert.True(t, webhookSlugs[k.TargetWebhookTemplateSlug],
			"kit %q references webhook %q that does not exist",
			k.Slug, k.TargetWebhookTemplateSlug)
	}
}

func TestIntegration_CoreEvidenceKit_AllCollectionPeriodsArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, kit := range got {
		assert.Greater(t, kit.CollectionPeriodDays, 0)
		assert.Greater(t, kit.RetentionDays, 0)
		assert.GreaterOrEqual(t, kit.RetentionDays, kit.CollectionPeriodDays,
			"retention must be ≥ collection period")
	}
}

func TestIntegration_CoreEvidenceKit_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, kit := range got {
		assert.False(t, seen[kit.Slug])
		seen[kit.Slug] = true
	}
}

func TestIntegration_CoreEvidenceKit_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, kit := range got {
		assert.GreaterOrEqual(t, len(kit.Description), 30)
	}
}

func TestIntegration_CoreEvidenceKit_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreEvidenceKit_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000024_seed_compliance_evidence_kits.up.sql")

	loader := core.NewCoreComplianceEvidenceKitLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, kit := range got {
		dbSlugs = append(dbSlugs, kit.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedComplianceEvidenceKitSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
