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

const pdadMigration = "000048_seed_permission_denial_alternative_default_templates.up.sql"
const pdadMigrationDown = "000048_seed_permission_denial_alternative_default_templates.down.sql"

func TestIntegration_CorePDAD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPDADTemplateRowCount, len(got))
}

func TestIntegration_CorePDAD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePDAD_FindBySlug_BashShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "bash-to-sandboxed-shell")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "rule_match", tmpl.TargetReason)
	assert.Equal(t, "suggest_alternative_tool", tmpl.TargetRetryHint)
	assert.Equal(t, "Bash", tmpl.DeniedToolName)
	alts, err := tmpl.AlternativeToolNames()
	require.NoError(t, err)
	assert.Contains(t, alts, "shell_sandboxed")
}

func TestIntegration_CorePDAD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePDAD_LoadByReason_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	rule, err := loader.LoadByReason(context.Background(), "rule_match")
	require.NoError(t, err)
	assert.Equal(t, 2, len(rule))
	hook, _ := loader.LoadByReason(context.Background(), "hook_override")
	assert.Equal(t, 1, len(hook))
	pref, _ := loader.LoadByReason(context.Background(), "prefilter_drop")
	assert.Equal(t, 1, len(pref))
	mode, _ := loader.LoadByReason(context.Background(), "mode_block")
	assert.Equal(t, 1, len(mode))
	rate, _ := loader.LoadByReason(context.Background(), "rate_limit")
	assert.Equal(t, 1, len(rate))
}

func TestIntegration_CorePDAD_LoadByRetryHint_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	alt, err := loader.LoadByRetryHint(context.Background(), "suggest_alternative_tool")
	require.NoError(t, err)
	assert.Equal(t, 4, len(alt))
	wait, _ := loader.LoadByRetryHint(context.Background(), "wait_and_retry")
	assert.Equal(t, 1, len(wait))
	conf, _ := loader.LoadByRetryHint(context.Background(), "request_user_confirmation")
	assert.Equal(t, 1, len(conf))
}

func TestIntegration_CorePDAD_LoadByDeniedTool(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	// Bash appears twice (rule_match + mode_block).
	bash, err := loader.LoadByDeniedTool(context.Background(), "Bash")
	require.NoError(t, err)
	assert.Equal(t, 2, len(bash))
	sql, _ := loader.LoadByDeniedTool(context.Background(), "execute-sql")
	assert.Equal(t, 1, len(sql))
}

func TestIntegration_CorePDAD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPDADTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePDAD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPDADTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePDAD_AllReasonsAreInPERM006Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	perm006Reasons := map[string]bool{
		"rule_match": true, "hook_override": true,
		"prefilter_drop": true, "mode_block": true,
		"sandbox_violation": true, "rate_limit": true,
	}
	for _, t2 := range all {
		assert.True(t, perm006Reasons[t2.TargetReason],
			"%s reason %q outside PERM-006 enum", t2.Slug, t2.TargetReason)
	}
}

func TestIntegration_CorePDAD_AllRetryHintsAreInPERM006Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	perm006Hints := map[string]bool{
		"not_retryable": true, "retry_with_different_input": true,
		"request_user_confirmation": true,
		"suggest_alternative_tool":  true, "wait_and_retry": true,
	}
	for _, t2 := range all {
		assert.True(t, perm006Hints[t2.TargetRetryHint],
			"%s retry %q outside PERM-006 enum", t2.Slug, t2.TargetRetryHint)
	}
}

func TestIntegration_CorePDAD_RateLimitHasEmptyAlternates(t *testing.T) {
	// Cross-row invariant: rate-limit templates suggest NO pivot
	// because pivoting defeats the limit.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	rate, err := loader.LoadByReason(context.Background(), "rate_limit")
	require.NoError(t, err)
	for _, t2 := range rate {
		alts, err := t2.AlternativeToolNames()
		require.NoError(t, err)
		assert.Empty(t, alts, "%s rate-limit must have empty alternates", t2.Slug)
	}
}

func TestIntegration_CorePDAD_ModeBlockHasEmptyAlternates(t *testing.T) {
	// Cross-row invariant: mode_block needs user action, not a pivot.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	mode, err := loader.LoadByReason(context.Background(), "mode_block")
	require.NoError(t, err)
	for _, t2 := range mode {
		alts, err := t2.AlternativeToolNames()
		require.NoError(t, err)
		assert.Empty(t, alts, "%s mode_block must have empty alternates", t2.Slug)
	}
}

func TestIntegration_CorePDAD_SuggestAlternativeHasNonEmptyAlternates(t *testing.T) {
	// Cross-row invariant: when retry=suggest_alternative_tool, the
	// alternatives MUST be non-empty (or the hint is hollow).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	suggest, err := loader.LoadByRetryHint(context.Background(), "suggest_alternative_tool")
	require.NoError(t, err)
	for _, t2 := range suggest {
		alts, err := t2.AlternativeToolNames()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(alts), 1,
			"%s suggest_alternative_tool must list ≥1 alternate", t2.Slug)
	}
}

func TestIntegration_CorePDAD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 30, "%s description", t2.Slug)
	}
}

func TestIntegration_CorePDAD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePDAD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
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
	assert.Equal(t, "bash-to-sandboxed-shell", all[0].Slug)
}

func TestIntegration_CorePDAD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPDADTemplateRowCount, len(got))
}

func TestIntegration_CorePDAD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)
	applyMigration(t, pool, migDir, pdadMigrationDown)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePDAD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pdadMigration)

	loader := core.NewCorePermissionDenialAlternativeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPDADTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
