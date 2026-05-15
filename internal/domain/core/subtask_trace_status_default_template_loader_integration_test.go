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

const stsdMigration = "000059_seed_subtask_trace_status_default_templates.up.sql"
const stsdMigrationDown = "000059_seed_subtask_trace_status_default_templates.down.sql"

func TestIntegration_CoreSTSD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSTSDTemplateRowCount, len(got))
}

func TestIntegration_CoreSTSD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSTSD_FindBySlug_CompletedShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "completed-routine-trace")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "completed", tmpl.TargetStatus)
	assert.True(t, tmpl.PropagateCostToParent)
	assert.False(t, tmpl.EmitErrorEnvelope)
	assert.False(t, tmpl.RequiresAdminReview)
	events, err := tmpl.ExpectedEventSequence()
	require.NoError(t, err)
	assert.Contains(t, events, "subtask_start")
	assert.Contains(t, events, "subtask_complete")
}

func TestIntegration_CoreSTSD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSTSD_LoadByStatus_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	for _, status := range core.SeedExpectedSTSDTemplateStatuses {
		matched, err := loader.LoadByStatus(context.Background(), status)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "status %q must have exactly 1 template (1:1)", status)
	}
}

func TestIntegration_CoreSTSD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedSTSDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CoreSTSD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedSTSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSTSD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewSTSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSTSD_AllStatusesInOBS006Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{
		"completed": true, "failed": true, "killed": true,
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetStatus],
			"%s status %q outside OBS-006 enum", t2.Slug, t2.TargetStatus)
	}
}

func TestIntegration_CoreSTSD_EventSequenceStartsWithSubtaskStart(t *testing.T) {
	// Cross-feature invariant: every trace MUST start with subtask_start
	// (matches OBS-006 SubtaskExecutor.Execute behavior).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		events, err := t2.ExpectedEventSequence()
		require.NoError(t, err)
		require.NotEmpty(t, events, "%s must have non-empty sequence", t2.Slug)
		assert.Equal(t, "subtask_start", events[0],
			"%s sequence must start with subtask_start", t2.Slug)
	}
}

func TestIntegration_CoreSTSD_EventSequenceEndsWithSubtaskComplete(t *testing.T) {
	// Cross-feature invariant: every trace MUST end with subtask_complete.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		events, err := t2.ExpectedEventSequence()
		require.NoError(t, err)
		require.NotEmpty(t, events)
		last := events[len(events)-1]
		assert.Equal(t, "subtask_complete", last,
			"%s sequence must end with subtask_complete", t2.Slug)
	}
}

func TestIntegration_CoreSTSD_AllEventTypesInExpectedSet(t *testing.T) {
	// Cross-feature invariant: every event type used in templates must
	// be a canonical event name AgentHub emits.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, e := range core.SeedExpectedSTSDTemplateEventTypes {
		allowed[e] = true
	}
	for _, t2 := range all {
		events, err := t2.ExpectedEventSequence()
		require.NoError(t, err)
		for _, e := range events {
			assert.True(t, allowed[e],
				"%s uses unknown event type %q", t2.Slug, e)
		}
	}
}

func TestIntegration_CoreSTSD_FailedAndKilledEmitErrorEnvelope(t *testing.T) {
	// Cross-row invariant: failed + killed → emit_error_envelope=true.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	for _, slug := range []string{"failed-error-context-trace", "killed-budget-or-depth-trace"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.True(t, tmpl.EmitErrorEnvelope, "%s should emit error envelope", slug)
	}
}

func TestIntegration_CoreSTSD_CompletedDoesNotEmitError(t *testing.T) {
	// Cross-row invariant: completed → no error envelope.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "completed-routine-trace")
	require.NoError(t, err)
	assert.False(t, tmpl.EmitErrorEnvelope)
}

func TestIntegration_CoreSTSD_KillTraceMinimalSequence(t *testing.T) {
	// Cross-row invariant: depth-kill can happen before first LLM call,
	// so the kill template sequence is just [subtask_start, subtask_complete].
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "killed-budget-or-depth-trace")
	require.NoError(t, err)
	events, err := tmpl.ExpectedEventSequence()
	require.NoError(t, err)
	assert.Equal(t, []string{"subtask_start", "subtask_complete"}, events)
	// typical_min_turns=0 since depth-kill happens before any LLM turn.
	assert.Equal(t, 0, tmpl.TypicalMinTurns)
}

func TestIntegration_CoreSTSD_TypicalMinTurnsNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.TypicalMinTurns, 0, "%s typical_min_turns", t2.Slug)
	}
}

func TestIntegration_CoreSTSD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreSTSD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreSTSD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
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
	assert.Equal(t, "completed-routine-trace", all[0].Slug)
}

func TestIntegration_CoreSTSD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSTSDTemplateRowCount, len(got))
}

func TestIntegration_CoreSTSD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)
	applyMigration(t, pool, migDir, stsdMigrationDown)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSTSD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, stsdMigration)

	loader := core.NewCoreSubtaskTraceStatusDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedSTSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
