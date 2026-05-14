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

const cdtMigration = "000066_seed_complexity_drift_threshold_default_templates.up.sql"
const cdtMigrationDown = "000066_seed_complexity_drift_threshold_default_templates.down.sql"

func TestIntegration_CoreCDT_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCDTTemplateRowCount, len(got))
}

func TestIntegration_CoreCDT_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCDT_FindBySlug_ScopeCreepShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "scope-creep-default")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "scope_creep", tmpl.Signal)
	assert.Equal(t, 50.0, tmpl.WarnAt)
	assert.Equal(t, 100.0, tmpl.CriticalAt)
	assert.Equal(t, "sub_tasks", tmpl.Unit)
	assert.Equal(t, "run", tmpl.AppliesToSubjectKind)
}

func TestIntegration_CoreCDT_FindBySignal_RoundTrip(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	for _, signal := range core.SeedExpectedCDTTemplateSignals {
		tmpl, found, err := loader.FindBySignal(context.Background(), signal)
		require.NoError(t, err)
		require.True(t, found, "signal %q must be seeded", signal)
		assert.Equal(t, signal, tmpl.Signal)
	}
}

func TestIntegration_CoreCDT_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCDT_AllSignalsInHUMAN006Enum(t *testing.T) {
	// Cross-feature invariant: every seeded signal must exist in the
	// HUMAN-006 ComplexityDriftSignal enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedCDTTemplateSignals {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Signal],
			"%s signal %q outside HUMAN-006 enum", t2.Slug, t2.Signal)
	}
}

func TestIntegration_CoreCDT_AllRowsRespectCriticalAboveWarnInvariant(t *testing.T) {
	// Cross-feature invariant: matches HUMAN-006 ComplexityDriftThreshold
	// Validate (critical > warn strict). DB CHECK constraint enforces.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.Greater(t, t2.CriticalAt, t2.WarnAt,
			"%s: critical %v must be > warn %v", t2.Slug, t2.CriticalAt, t2.WarnAt)
		assert.GreaterOrEqual(t, t2.WarnAt, 0.0,
			"%s: warn %v must be >= 0", t2.Slug, t2.WarnAt)
	}
}

func TestIntegration_CoreCDT_DBCheckRejectsCriticalLessThanWarn(t *testing.T) {
	// DB-level CHECK constraint test: explicit INSERT with critical <= warn
	// must fail. Confirms guard rail at storage layer.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.complexity_drift_threshold_default_template
		    (id, slug, signal, warn_at, critical_at, unit, description, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000001', 'bad-row', 'scope_creep',
		     50.0, 10.0, 'sub_tasks', 'bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject critical_at < warn_at")
}

func TestIntegration_CoreCDT_DBCheckRejectsNegativeWarn(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.complexity_drift_threshold_default_template
		    (id, slug, signal, warn_at, critical_at, unit, description, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000002', 'bad-warn', 'scope_creep',
		     -1.0, 10.0, 'sub_tasks', 'bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject warn_at < 0")
}

func TestIntegration_CoreCDT_AllUnitsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedCDTTemplateUnits {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Unit],
			"%s unit %q outside closed set", t2.Slug, t2.Unit)
	}
}

func TestIntegration_CoreCDT_AllSignalsUniqueInDB(t *testing.T) {
	// UNIQUE constraint guarantee.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Signal], "duplicate signal %q", t2.Signal)
		seen[t2.Signal] = true
	}
}

func TestIntegration_CoreCDT_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40,
			"%s description must be substantive", t2.Slug)
	}
}

func TestIntegration_CoreCDT_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
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
	assert.Equal(t, "scope-creep-default", all[0].Slug)
}

func TestIntegration_CoreCDT_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCDTTemplateRowCount, len(got))
}

func TestIntegration_CoreCDT_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)
	applyMigration(t, pool, migDir, cdtMigrationDown)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCDT_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cdtMigration)

	loader := core.NewCoreComplexityDriftThresholdDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedCDTTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
