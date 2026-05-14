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

const aefMigration = "000030_seed_audit_export_format_templates.up.sql"
const aefMigrationDown = "000030_seed_audit_export_format_templates.down.sql"

func TestIntegration_CoreAuditExportFormatTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAuditExportFormatTemplateRowCount, len(got))
}

func TestIntegration_CoreAuditExportFormatTemplate_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAuditExportFormatTemplate_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "gdpr-data-subject-export")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "json", tmpl.ExportFormat)
	assert.Equal(t, "ed25519", tmpl.SignatureAlgorithm,
		"GDPR DSAR uses ed25519 for non-repudiation")
	assert.Equal(t, "gdpr", tmpl.ComplianceProfile)
	assert.True(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreAuditExportFormatTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreAuditExportFormatTemplate_LoadByComplianceProfile_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	gen, err := loader.LoadByComplianceProfile(context.Background(), "generic_audit")
	require.NoError(t, err)
	assert.Equal(t, 2, len(gen), "two generic templates")

	// Each regulated profile has exactly 1 template.
	for _, p := range []string{"gdpr", "hipaa", "sox", "pci_dss", "soc2", "iso27001"} {
		got, err := loader.LoadByComplianceProfile(context.Background(), p)
		require.NoError(t, err)
		assert.Equal(t, 1, len(got), "profile %q must have exactly 1 template", p)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_LoadByExportFormat_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	zips, err := loader.LoadByExportFormat(context.Background(), "zip_bundle")
	require.NoError(t, err)
	assert.Equal(t, 4, len(zips), "generic-quarterly + hipaa + pci + soc2 use zip_bundle")
}

func TestIntegration_CoreAuditExportFormatTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedAuditExportFormatTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAuditExportFormatTemplate_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewAuditExportFormatTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAuditExportFormatTemplate_AllExportFormatsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, f := range core.SeedExpectedAuditExportFormats {
		allowed[f] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.ExportFormat],
			"template %q has format %q outside FUTURE-005 enum", p.Slug, p.ExportFormat)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_AllSignaturesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedAuditSignatureAlgorithms {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.SignatureAlgorithm],
			"template %q has signature %q outside FUTURE-005 enum", p.Slug, p.SignatureAlgorithm)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_AllProfilesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedAuditExportComplianceProfiles {
		allowed[p] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.ComplianceProfile],
			"template %q has profile %q outside expected set",
			tmpl.Slug, tmpl.ComplianceProfile)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_RetentionMatchesProfileMinimum(t *testing.T) {
	// Cross-row regulatory invariant:
	//   HIPAA security rule = 6 years (≥ 2190 days)
	//   SOX records mgmt    = 7 years (≥ 2555 days)
	//   GDPR Art 30         = ~5 years (≥ 1825 days)
	//   PCI-DSS             = 1 year + 90 days hot (≥ 365)
	//   SOC 2 Type II       = 3 years (≥ 1095 days)
	//   ISO 27001 cert cycle= 3 years (≥ 1095 days)
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	cases := map[string]int{
		"hipaa-phi-access-bundle":       2190,
		"sox-financial-controls-bundle": 2555,
		"gdpr-data-subject-export":      1825,
		"pci-cardholder-data-export":    365,
		"soc2-trust-service-criteria":   1095,
		"iso27001-isms-controls-csv":    1095,
	}
	for slug, minDays := range cases {
		tmpl, found, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		require.True(t, found, "missing %s", slug)
		assert.GreaterOrEqual(t, tmpl.RetentionDays, minDays,
			"template %q retention=%d < required %d days for compliance profile",
			slug, tmpl.RetentionDays, minDays)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_FilenamePatternsContainTenantToken(t *testing.T) {
	// Cross-row invariant: every filename pattern must include the
	// {tenant} token to ensure exports are never tenant-ambiguous.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		assert.Contains(t, tmpl.FilenamePattern, "{tenant}",
			"template %q filename pattern lacks {tenant} token", tmpl.Slug)
		assert.Contains(t, tmpl.FilenamePattern, "{period}",
			"template %q filename pattern lacks {period} token", tmpl.Slug)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_FilenameExtensionMatchesFormat(t *testing.T) {
	// Cross-row invariant: the filename extension must match export_format.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	expectedExt := map[string]string{
		"csv": ".csv", "json": ".json", "pdf": ".pdf",
		"xlsx": ".xlsx", "zip_bundle": ".zip",
	}
	for _, tmpl := range all {
		ext, ok := expectedExt[tmpl.ExportFormat]
		require.True(t, ok)
		assert.True(t, strings.HasSuffix(tmpl.FilenamePattern, ext),
			"template %q (format=%q) pattern %q must end in %q",
			tmpl.Slug, tmpl.ExportFormat, tmpl.FilenamePattern, ext)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_RegulatedTemplatesIncludeRawEvidence(t *testing.T) {
	// Regulators usually need raw evidence + report (not summary alone).
	// Generic-monthly is the only summary-only template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		if tmpl.Slug == "generic-monthly-summary" {
			assert.False(t, tmpl.IncludesRawEvidence,
				"monthly summary deliberately excludes raw evidence")
			continue
		}
		assert.True(t, tmpl.IncludesRawEvidence,
			"template %q must include raw evidence for regulator review", tmpl.Slug)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreAuditExportFormatTemplate_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
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
	assert.Equal(t, "generic-monthly-summary", all[0].Slug)
}

func TestIntegration_CoreAuditExportFormatTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAuditExportFormatTemplateRowCount, len(got))
}

func TestIntegration_CoreAuditExportFormatTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)
	applyMigration(t, pool, migDir, aefMigrationDown)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAuditExportFormatTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedAuditExportFormatTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAuditExportFormatTemplate_RenderFilenameRoundTrip(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, aefMigration)

	loader := core.NewCoreAuditExportFormatTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "soc2-trust-service-criteria")
	require.NoError(t, err)
	rendered := tmpl.RenderFilename(map[string]string{
		"tenant": "acme", "period": "2026-Q2",
	})
	assert.Contains(t, rendered, "acme")
	assert.Contains(t, rendered, "2026-Q2")
	assert.NotContains(t, rendered, "{tenant}")
	assert.NotContains(t, rendered, "{period}")
}
