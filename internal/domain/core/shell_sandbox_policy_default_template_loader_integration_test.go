//go:build integration

package core_test

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const sspdMigration = "000049_seed_shell_sandbox_policy_default_templates.up.sql"
const sspdMigrationDown = "000049_seed_shell_sandbox_policy_default_templates.down.sql"

func TestIntegration_CoreSSPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSSPDTemplateRowCount, len(got))
}

func TestIntegration_CoreSSPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSSPD_FindBySlug_WebSafeShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "web-safe-default")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "balanced", tmpl.TargetSafetyPosture)
	assert.Equal(t, "standard_chat", tmpl.TargetUseCase)
	assert.False(t, tmpl.AllowNetwork)
	assert.Equal(t, 60, tmpl.MaxRuntimeSecs)
	roots, err := tmpl.AllowedPathRoots()
	require.NoError(t, err)
	assert.Contains(t, roots, "/var/agenthub/session")
	cmds, err := tmpl.BlockedCommands()
	require.NoError(t, err)
	assert.Contains(t, cmds, "sudo")
}

func TestIntegration_CoreSSPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSSPD_LoadBySafetyPosture(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedSSPDTemplateSafetyPostures {
		matched, err := loader.LoadBySafetyPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "posture %q must have exactly 1 template (1:1)", posture)
	}
}

func TestIntegration_CoreSSPD_LoadByUseCase(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedSSPDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CoreSSPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedSSPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSSPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewSSPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSSPD_AllPosturesInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedSSPDTemplateSafetyPostures {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetSafetyPosture],
			"%s posture %q outside expected set", t2.Slug, t2.TargetSafetyPosture)
	}
}

func TestIntegration_CoreSSPD_AllUseCasesInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedSSPDTemplateUseCases {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetUseCase])
	}
}

func TestIntegration_CoreSSPD_LockedDownHasEmptyAllowedRoots(t *testing.T) {
	// Cross-row invariant: strict posture → no filesystem access.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "locked-down")
	require.NoError(t, err)
	roots, err := tmpl.AllowedPathRoots()
	require.NoError(t, err)
	assert.Empty(t, roots)
	assert.False(t, tmpl.AllowNetwork)
}

func TestIntegration_CoreSSPD_DevAndCICDAllowNetwork(t *testing.T) {
	// Cross-row invariant: engineering / cicd permit network.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	for _, slug := range []string{"dev-workstation", "cicd-runner"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.True(t, tmpl.AllowNetwork, "%s should allow network", slug)
	}
}

func TestIntegration_CoreSSPD_RestrictedPosturesDenyNetwork(t *testing.T) {
	// Cross-row invariant: strict / balanced / conservative deny network.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	for _, slug := range []string{"locked-down", "web-safe-default", "incident-response-readonly"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.False(t, tmpl.AllowNetwork, "%s should deny network", slug)
	}
}

func TestIntegration_CoreSSPD_AllowedRootsAreAbsolute(t *testing.T) {
	// Cross-feature invariant matching PERM-008 ShellSandboxPolicy.Validate:
	// every allowed root MUST be absolute (else the runtime policy
	// validation will fail when applied).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		roots, err := t2.AllowedPathRoots()
		require.NoError(t, err)
		for _, r := range roots {
			assert.True(t, filepath.IsAbs(r),
				"%s root %q must be absolute", t2.Slug, r)
		}
	}
}

func TestIntegration_CoreSSPD_CeilingsNonNegative(t *testing.T) {
	// Cross-feature invariant matching PERM-008 ShellSandboxPolicy.Validate.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.MaxRuntimeSecs, 0, "%s max_runtime_secs", t2.Slug)
		assert.GreaterOrEqual(t, t2.MaxOutputBytes, 0, "%s max_output_bytes", t2.Slug)
	}
}

func TestIntegration_CoreSSPD_SudoBlockedEverywhere(t *testing.T) {
	// Cross-row invariant: every posture blocks sudo (no posture
	// permits privilege escalation).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		cmds, err := t2.BlockedCommands()
		require.NoError(t, err)
		assert.Contains(t, cmds, "sudo", "%s must block sudo", t2.Slug)
	}
}

func TestIntegration_CoreSSPD_CICDLongerThanWebSafe(t *testing.T) {
	// Cross-row priority ladder: CI/CD runtime > web-safe runtime.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	cicd, _, _ := loader.FindBySlug(context.Background(), "cicd-runner")
	web, _, _ := loader.FindBySlug(context.Background(), "web-safe-default")
	assert.Greater(t, cicd.MaxRuntimeSecs, web.MaxRuntimeSecs)
	assert.Greater(t, cicd.MaxOutputBytes, web.MaxOutputBytes)
}

func TestIntegration_CoreSSPD_IncidentReadonlyBlocksMutators(t *testing.T) {
	// Cross-row invariant: incident-response blocks rm, mv, chmod, dd.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "incident-response-readonly")
	require.NoError(t, err)
	cmds, err := tmpl.BlockedCommands()
	require.NoError(t, err)
	for _, m := range []string{"rm", "mv", "chmod", "dd"} {
		assert.Contains(t, cmds, m, "incident-response must block %s", m)
	}
}

func TestIntegration_CoreSSPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreSSPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreSSPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "locked-down", all[0].Slug)
}

func TestIntegration_CoreSSPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSSPDTemplateRowCount, len(got))
}

func TestIntegration_CoreSSPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)
	applyMigration(t, pool, migDir, sspdMigrationDown)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSSPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sspdMigration)

	loader := core.NewCoreShellSandboxPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedSSPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
