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

const sfsdMigration = "000060_seed_session_fork_strategy_default_templates.up.sql"
const sfsdMigrationDown = "000060_seed_session_fork_strategy_default_templates.down.sql"

func TestIntegration_CoreSFSD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSFSDTemplateRowCount, len(got))
}

func TestIntegration_CoreSFSD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSFSD_FindBySlug_FullCopyShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "full-copy-explore")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "full_copy", tmpl.TargetStrategy)
	assert.False(t, tmpl.ComputesTranscriptHash)
	assert.False(t, tmpl.AllowsForkPastCompaction)
	assert.Equal(t, "high", tmpl.TypicalStorageOverhead)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreSFSD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSFSD_LoadByStrategy_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	for _, strategy := range core.SeedExpectedSFSDTemplateStrategies {
		matched, err := loader.LoadByStrategy(context.Background(), strategy)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "strategy %q must have exactly 1 template (1:1)", strategy)
	}
}

func TestIntegration_CoreSFSD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedSFSDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CoreSFSD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedSFSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSFSD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewSFSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSFSD_AllStrategiesInPERSIST005aEnum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedSFSDTemplateStrategies {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetStrategy],
			"%s strategy %q outside PERSIST-005a enum", t2.Slug, t2.TargetStrategy)
	}
}

func TestIntegration_CoreSFSD_OnlySnapshotIsolatedComputesHash(t *testing.T) {
	// Cross-feature invariant: matches PERSIST-005a Evaluate behavior
	// (hash computed only for snapshot_isolated strategy).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetStrategy == "snapshot_isolated" {
			assert.True(t, t2.ComputesTranscriptHash,
				"%s (snapshot_isolated) must compute hash", t2.Slug)
		} else {
			assert.False(t, t2.ComputesTranscriptHash,
				"%s (%s) must NOT compute hash", t2.Slug, t2.TargetStrategy)
		}
	}
}

func TestIntegration_CoreSFSD_OnlySnapshotIsolatedAllowsCompactionFork(t *testing.T) {
	// Cross-feature invariant: PERSIST-005a Evaluate rejects fork past
	// compaction unless strategy is snapshot_isolated.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetStrategy == "snapshot_isolated" {
			assert.True(t, t2.AllowsForkPastCompaction)
		} else {
			assert.False(t, t2.AllowsForkPastCompaction,
				"%s (%s) must NOT allow fork past compaction",
				t2.Slug, t2.TargetStrategy)
		}
	}
}

func TestIntegration_CoreSFSD_StorageOverheadInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{"low": true, "medium": true, "high": true}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TypicalStorageOverhead])
	}
}

func TestIntegration_CoreSFSD_FullCopyHasHighestStorage(t *testing.T) {
	// Cross-row invariant: full_copy is most storage-heavy.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "full-copy-explore")
	require.NoError(t, err)
	assert.Equal(t, "high", tmpl.TypicalStorageOverhead)
}

func TestIntegration_CoreSFSD_BranchPointerHasLowestStorage(t *testing.T) {
	// Cross-row invariant: branch_pointer is cheapest.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "branch-pointer-cheap")
	require.NoError(t, err)
	assert.Equal(t, "low", tmpl.TypicalStorageOverhead)
}

func TestIntegration_CoreSFSD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreSFSD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreSFSD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "full-copy-explore", all[0].Slug)
}

func TestIntegration_CoreSFSD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSFSDTemplateRowCount, len(got))
}

func TestIntegration_CoreSFSD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)
	applyMigration(t, pool, migDir, sfsdMigrationDown)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSFSD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sfsdMigration)

	loader := core.NewCoreSessionForkStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedSFSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
