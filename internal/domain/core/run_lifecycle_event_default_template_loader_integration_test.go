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

const rletMigration = "000070_seed_run_lifecycle_event_default_templates.up.sql"
const rletMigrationDown = "000070_seed_run_lifecycle_event_default_templates.down.sql"

func TestIntegration_CoreRLET_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedRLETRowCount, len(got))
}

func TestIntegration_CoreRLET_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreRLET_FindBySlug_RunStartedAuditShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "run-started-audit-log")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "RunStarted", tmpl.HookEvent)
	assert.Equal(t, "audit", tmpl.HandlerKind)
	assert.True(t, tmpl.EnabledByDefault)
	assert.NotEmpty(t, tmpl.Description)
}

func TestIntegration_CoreRLET_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreRLET_LoadByHookEvent_RunComplete(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	results, err := loader.LoadByHookEvent(context.Background(), "RunComplete")
	require.NoError(t, err)
	// seed has 2 RunComplete rows
	assert.Equal(t, 2, len(results))
	for _, r := range results {
		assert.Equal(t, "RunComplete", r.HookEvent)
	}
}

func TestIntegration_CoreRLET_LoadByHandlerKind_AuditSubset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	audits, err := loader.LoadByHandlerKind(context.Background(), "audit")
	require.NoError(t, err)
	// seed has 2 audit rows
	assert.Equal(t, 2, len(audits))
	for _, r := range audits {
		assert.Equal(t, "audit", r.HandlerKind)
	}
}

func TestIntegration_CoreRLET_LoadEnabledByDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	enabled, err := loader.LoadEnabledByDefault(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, e := range enabled {
		got = append(got, e.Slug)
		assert.True(t, e.EnabledByDefault)
	}
	expected := append([]string{}, core.SeedRLETEnabledByDefaultSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreRLET_AllSlugsMatchKebabRegex(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.RLETSlugRE.MatchString(t2.Slug),
			"slug %q must match kebab regex", t2.Slug)
	}
}

func TestIntegration_CoreRLET_AllHookEventsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{
		"RunStarted": true, "RunComplete": true,
		"ContextWindowAlert": true, "KnowledgeBaseQueried": true,
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.HookEvent],
			"%s hook_event %q outside closed set", t2.Slug, t2.HookEvent)
	}
}

func TestIntegration_CoreRLET_AllHandlerKindsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{"webhook": true, "notification": true, "audit": true}
	for _, t2 := range all {
		assert.True(t, allowed[t2.HandlerKind],
			"%s handler_kind %q outside closed set", t2.Slug, t2.HandlerKind)
	}
}

func TestIntegration_CoreRLET_DBCheckRejectsInvalidHookEvent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.run_lifecycle_event_template
		    (id, slug, hook_event, handler_kind, description, sort_order)
		VALUES
		    ('eeeeeeee-0001-0000-0000-000000000001', 'bad-hook-event',
		     'SessionStart', 'webhook', 'bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject hook_event='SessionStart' (not in web-adapted set)")
}

func TestIntegration_CoreRLET_DBCheckRejectsInvalidHandlerKind(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.run_lifecycle_event_template
		    (id, slug, hook_event, handler_kind, description, sort_order)
		VALUES
		    ('eeeeeeee-0002-0000-0000-000000000002', 'bad-handler-kind',
		     'RunStarted', 'email', 'bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject handler_kind='email'")
}

func TestIntegration_CoreRLET_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedRLETRowCount, len(got))
}

func TestIntegration_CoreRLET_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)
	applyMigration(t, pool, migDir, rletMigrationDown)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreRLET_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedRLETSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreRLET_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
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
	assert.Equal(t, "run-started-audit-log", all[0].Slug)
}

func TestIntegration_CoreRLET_HandlerConfigIsValidJSON(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, rletMigration)

	loader := core.NewCoreRunLifecycleEventDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.NotNil(t, t2.HandlerConfig,
			"%s handler_config must not be nil", t2.Slug)
	}
}
