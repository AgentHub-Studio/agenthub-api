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

const srsdMigration = "000057_seed_subagent_return_summary_default_templates.up.sql"
const srsdMigrationDown = "000057_seed_subagent_return_summary_default_templates.down.sql"

func TestIntegration_CoreSRSD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSRSDTemplateRowCount, len(got))
}

func TestIntegration_CoreSRSD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSRSD_FindBySlug_SuccessShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "success-with-artifacts")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "success", tmpl.TargetOutcome)
	assert.False(t, tmpl.RedactFindings)
	assert.False(t, tmpl.RedactArtifacts)
	assert.False(t, tmpl.RedactTranscriptHash)
	assert.False(t, tmpl.RequiresAdminReview)
	findings, err := tmpl.SampleFindings()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(findings), 1)
}

func TestIntegration_CoreSRSD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSRSD_LoadByOutcome(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	// success has 2 templates (routine + audit-redacted).
	success, err := loader.LoadByOutcome(context.Background(), "success")
	require.NoError(t, err)
	assert.Equal(t, 2, len(success))
	partial, _ := loader.LoadByOutcome(context.Background(), "partial")
	assert.Equal(t, 1, len(partial))
	failed, _ := loader.LoadByOutcome(context.Background(), "failed")
	assert.Equal(t, 1, len(failed))
	aborted, _ := loader.LoadByOutcome(context.Background(), "aborted")
	assert.Equal(t, 1, len(aborted))
}

func TestIntegration_CoreSRSD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedSRSDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template (1:1)", uc)
	}
}

func TestIntegration_CoreSRSD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedSRSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSRSD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewSRSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSRSD_AllOutcomesInSUB010Enum(t *testing.T) {
	// Cross-feature invariant: every target_outcome must be in SUB-010
	// SubagentReturnOutcome enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{
		"success": true, "partial": true, "failed": true, "aborted": true,
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetOutcome],
			"%s outcome %q outside SUB-010 enum", t2.Slug, t2.TargetOutcome)
	}
}

func TestIntegration_CoreSRSD_FailedTemplateHasEmptyArtifacts(t *testing.T) {
	// Cross-row invariant: failed-error template ships empty artifacts
	// (failed subagent produced nothing usable).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "failed-error")
	require.NoError(t, err)
	artifacts, err := tmpl.SampleArtifacts()
	require.NoError(t, err)
	assert.Empty(t, artifacts)
}

func TestIntegration_CoreSRSD_AuditTemplateRedactsAllFields(t *testing.T) {
	// Cross-row invariant: audit-with-redaction has all 3 redact flags on.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "audit-with-redaction")
	require.NoError(t, err)
	assert.True(t, tmpl.RedactFindings)
	assert.True(t, tmpl.RedactArtifacts)
	assert.True(t, tmpl.RedactTranscriptHash)
}

func TestIntegration_CoreSRSD_RoutineTemplatesHaveNoRedaction(t *testing.T) {
	// Cross-row invariant: routine templates (success-with-artifacts +
	// partial + failed + aborted) preserve full context — no redaction.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	for _, slug := range []string{"success-with-artifacts", "partial-needs-followup",
		"failed-error", "aborted-by-parent"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.False(t, tmpl.RedactFindings, "%s should NOT redact findings", slug)
		assert.False(t, tmpl.RedactArtifacts, "%s should NOT redact artifacts", slug)
		assert.False(t, tmpl.RedactTranscriptHash, "%s should NOT redact hash", slug)
	}
}

func TestIntegration_CoreSRSD_SampleFieldsArrayBounded(t *testing.T) {
	// Cross-feature invariant: sample arrays MUST respect SUB-010
	// bounded count limits (≤10 findings, ≤20 artifacts, ≤10 next_steps).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		findings, err := t2.SampleFindings()
		require.NoError(t, err)
		assert.LessOrEqual(t, len(findings), 10, "%s sample_findings > SUB-010 max", t2.Slug)
		artifacts, err := t2.SampleArtifacts()
		require.NoError(t, err)
		assert.LessOrEqual(t, len(artifacts), 20, "%s sample_artifacts > SUB-010 max", t2.Slug)
		nextSteps, err := t2.SampleNextSteps()
		require.NoError(t, err)
		assert.LessOrEqual(t, len(nextSteps), 10, "%s sample_next_steps > SUB-010 max", t2.Slug)
	}
}

func TestIntegration_CoreSRSD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreSRSD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreSRSD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
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
	assert.Equal(t, "success-with-artifacts", all[0].Slug)
}

func TestIntegration_CoreSRSD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSRSDTemplateRowCount, len(got))
}

func TestIntegration_CoreSRSD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)
	applyMigration(t, pool, migDir, srsdMigrationDown)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSRSD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, srsdMigration)

	loader := core.NewCoreSubagentReturnSummaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedSRSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
