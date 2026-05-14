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

const macpdMigration = "000058_seed_multi_agent_coordination_plan_default_templates.up.sql"
const macpdMigrationDown = "000058_seed_multi_agent_coordination_plan_default_templates.down.sql"

func TestIntegration_CoreMACPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMACPDTemplateRowCount, len(got))
}

func TestIntegration_CoreMACPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMACPD_FindBySlug_SequentialShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "sequential-pipeline")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "sequential", tmpl.TargetStrategy)
	assert.Equal(t, "abort_on_failure", tmpl.TargetFailurePolicy)
	assert.Equal(t, 1, tmpl.MaxParallelism)
	assert.False(t, tmpl.HasDagDependencies)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreMACPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreMACPD_LoadByStrategy(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	// sequential: 1, parallel: 1, pipeline: 1, dag: 2.
	seq, err := loader.LoadByStrategy(context.Background(), "sequential")
	require.NoError(t, err)
	assert.Equal(t, 1, len(seq))
	par, _ := loader.LoadByStrategy(context.Background(), "parallel")
	assert.Equal(t, 1, len(par))
	pip, _ := loader.LoadByStrategy(context.Background(), "pipeline")
	assert.Equal(t, 1, len(pip))
	dag, _ := loader.LoadByStrategy(context.Background(), "dag")
	assert.Equal(t, 2, len(dag))
}

func TestIntegration_CoreMACPD_LoadByFailurePolicy(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	for _, policy := range core.SeedExpectedMACPDTemplateFailurePolicies {
		matched, err := loader.LoadByFailurePolicy(context.Background(), policy)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(matched), 1, "policy %q must have ≥1 template", policy)
	}
}

func TestIntegration_CoreMACPD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedMACPDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template (1:1)", uc)
	}
}

func TestIntegration_CoreMACPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedMACPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreMACPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewMACPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreMACPD_AllStrategiesInSUB011Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedMACPDTemplateStrategies {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetStrategy],
			"%s strategy %q outside SUB-011 enum", t2.Slug, t2.TargetStrategy)
	}
}

func TestIntegration_CoreMACPD_AllFailurePoliciesInSUB011Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedMACPDTemplateFailurePolicies {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetFailurePolicy],
			"%s policy %q outside SUB-011 enum", t2.Slug, t2.TargetFailurePolicy)
	}
}

func TestIntegration_CoreMACPD_MaxParallelismNonNegative(t *testing.T) {
	// Cross-feature invariant: matches SUB-011 ErrMultiAgentBadParallelism.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.MaxParallelism, 0, "%s max_parallelism", t2.Slug)
	}
}

func TestIntegration_CoreMACPD_DAGStrategyImpliesHasDagDependencies(t *testing.T) {
	// Cross-row invariant: strategy=dag → has_dag_dependencies=true;
	// sequential/parallel → false. Pipeline is interesting (sequential
	// flow with explicit data hand-off) — we mark it true since the
	// template ships 2 tasks with extract → summarize dependency.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetStrategy == "dag" || t2.TargetStrategy == "pipeline" {
			assert.True(t, t2.HasDagDependencies,
				"%s (strategy=%s) should have dag dependencies", t2.Slug, t2.TargetStrategy)
		} else {
			assert.False(t, t2.HasDagDependencies,
				"%s (strategy=%s) should NOT have dag dependencies", t2.Slug, t2.TargetStrategy)
		}
	}
}

func TestIntegration_CoreMACPD_SequentialAndPipelineHaveMaxParallelismOne(t *testing.T) {
	// Cross-feature invariant: sequential and pipeline strategies
	// inherently run one task at a time (SUB-011 NextBatch returns max
	// 1 task for these). Encode that in max_parallelism=1.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	for _, slug := range []string{"sequential-pipeline", "pipeline-extract-summarize"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.Equal(t, 1, tmpl.MaxParallelism,
			"%s (strategy=%s) must have max_parallelism=1", slug, tmpl.TargetStrategy)
	}
}

func TestIntegration_CoreMACPD_OnlyDAGRequiresAdminReview(t *testing.T) {
	// Cross-row invariant: DAG templates → admin review; others routine.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetStrategy == "dag" {
			assert.True(t, t2.RequiresAdminReview, "%s (dag) should require review", t2.Slug)
		} else {
			assert.False(t, t2.RequiresAdminReview, "%s (%s) should be routine", t2.Slug, t2.TargetStrategy)
		}
	}
}

func TestIntegration_CoreMACPD_SampleTaskCountNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.SampleTaskCount, 0, "%s sample_task_count", t2.Slug)
	}
}

func TestIntegration_CoreMACPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreMACPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreMACPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
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
	assert.Equal(t, "sequential-pipeline", all[0].Slug)
}

func TestIntegration_CoreMACPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMACPDTemplateRowCount, len(got))
}

func TestIntegration_CoreMACPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)
	applyMigration(t, pool, migDir, macpdMigrationDown)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMACPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, macpdMigration)

	loader := core.NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedMACPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
