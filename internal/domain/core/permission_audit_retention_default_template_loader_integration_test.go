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

const pardMigration = "000053_seed_permission_audit_retention_default_templates.up.sql"
const pardMigrationDown = "000053_seed_permission_audit_retention_default_templates.down.sql"

func TestIntegration_CorePARD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPARDTemplateRowCount, len(got))
}

func TestIntegration_CorePARD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePARD_FindBySlug_BalancedShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "balanced-90d-allow-365d-deny")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "balanced", tmpl.CompliancePosture)
	assert.Equal(t, 90, tmpl.AllowTTLDays)
	assert.Equal(t, 365, tmpl.DenyTTLDays)
	assert.False(t, tmpl.RedactInputSnippet)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CorePARD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePARD_LoadByPosture_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedPARDTemplatePostures {
		matched, err := loader.LoadByPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "posture %q must have exactly 1 template (1:1)", posture)
	}
}

func TestIntegration_CorePARD_LoadByUseCase(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedPARDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CorePARD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPARDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePARD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPARDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePARD_AllPosturesInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedPARDTemplatePostures {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.CompliancePosture],
			"%s posture %q outside expected set", t2.Slug, t2.CompliancePosture)
	}
}

func TestIntegration_CorePARD_AllTTLsNonNegative(t *testing.T) {
	// Cross-feature invariant matching PERM-010 PermissionAuditRetentionPolicy.Validate:
	// every TTL >= 0 (else ApplyRetention will error).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.AllowTTLDays, 0, "%s allow_ttl", t2.Slug)
		assert.GreaterOrEqual(t, t2.DenyTTLDays, 0, "%s deny_ttl", t2.Slug)
		assert.GreaterOrEqual(t, t2.ConfirmApprovedTTLDays, 0, "%s confirm_approved_ttl", t2.Slug)
		assert.GreaterOrEqual(t, t2.ConfirmDeniedTTLDays, 0, "%s confirm_denied_ttl", t2.Slug)
		assert.GreaterOrEqual(t, t2.ConfirmEscalatedTTLDays, 0, "%s confirm_escalated_ttl", t2.Slug)
	}
}

func TestIntegration_CorePARD_DenyTTLGTEAllowTTLForFiniteWindows(t *testing.T) {
	// Cross-row invariant: when both TTLs are finite (>0), deny must
	// be retained at least as long as allow (compliance pattern).
	// Forensic_hold uses 0 (forever) so it's excluded from this check.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.AllowTTLDays > 0 && t2.DenyTTLDays > 0 {
			assert.GreaterOrEqual(t, t2.DenyTTLDays, t2.AllowTTLDays,
				"%s deny (%d) must be >= allow (%d)",
				t2.Slug, t2.DenyTTLDays, t2.AllowTTLDays)
		}
	}
}

func TestIntegration_CorePARD_ForensicHoldHasAllZeroTTLs(t *testing.T) {
	// Cross-row invariant: forensic_hold posture → all TTLs are 0
	// (never expire — matches PERM-010 ttlFor=0 → kept forever).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "forensic-hold-never-expires")
	require.NoError(t, err)
	assert.Equal(t, 0, tmpl.AllowTTLDays)
	assert.Equal(t, 0, tmpl.DenyTTLDays)
	assert.Equal(t, 0, tmpl.ConfirmApprovedTTLDays)
	assert.Equal(t, 0, tmpl.ConfirmDeniedTTLDays)
	assert.Equal(t, 0, tmpl.ConfirmEscalatedTTLDays)
}

func TestIntegration_CorePARD_PIIStrictRedactsAllFields(t *testing.T) {
	// Cross-row invariant: pii_strict posture → all 3 redaction flags on.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "strict-pii-redacted-export")
	require.NoError(t, err)
	assert.True(t, tmpl.RedactInputSnippet)
	assert.True(t, tmpl.RedactMatchedRule)
	assert.True(t, tmpl.RedactRunID)
}

func TestIntegration_CorePARD_MinimalAndBalancedHaveNoRedaction(t *testing.T) {
	// Cross-row invariant: minimal and balanced postures → no redaction
	// (operators see full context for debugging / routine audit).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	for _, slug := range []string{"minimal-30day", "balanced-90d-allow-365d-deny"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.False(t, tmpl.RedactInputSnippet, "%s should NOT redact", slug)
		assert.False(t, tmpl.RedactMatchedRule, "%s should NOT redact", slug)
		assert.False(t, tmpl.RedactRunID, "%s should NOT redact", slug)
	}
}

func TestIntegration_CorePARD_RegulatedAndPIIShareTTLsButDifferInRedaction(t *testing.T) {
	// Cross-row invariant: regulated and pii_strict have identical
	// TTL ladders; they differ ONLY in redaction posture.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	reg, _, _ := loader.FindBySlug(context.Background(), "regulated-7y-deny-2y-confirm")
	pii, _, _ := loader.FindBySlug(context.Background(), "strict-pii-redacted-export")
	assert.Equal(t, reg.AllowTTLDays, pii.AllowTTLDays)
	assert.Equal(t, reg.DenyTTLDays, pii.DenyTTLDays)
	assert.Equal(t, reg.ConfirmApprovedTTLDays, pii.ConfirmApprovedTTLDays)
	assert.Equal(t, reg.ConfirmDeniedTTLDays, pii.ConfirmDeniedTTLDays)
	// But redaction differs:
	assert.True(t, pii.RedactMatchedRule)
	assert.False(t, reg.RedactMatchedRule)
}

func TestIntegration_CorePARD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CorePARD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePARD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
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
	assert.Equal(t, "minimal-30day", all[0].Slug)
}

func TestIntegration_CorePARD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPARDTemplateRowCount, len(got))
}

func TestIntegration_CorePARD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)
	applyMigration(t, pool, migDir, pardMigrationDown)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePARD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pardMigration)

	loader := core.NewCorePermissionAuditRetentionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPARDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
