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

const ctxAssemblyMigration = "000082_seed_context_assembly_source_default_templates.up.sql"
const ctxAssemblyMigrationDown = "000082_seed_context_assembly_source_default_templates.down.sql"

func TestIntegration_CoreContextAssemblySource_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedContextAssemblySourceRowCount, len(got))
}

func TestIntegration_CoreContextAssemblySource_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreContextAssemblySource_FindBySlug_SystemPromptIsFirst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	src, found, err := loader.FindBySlug(context.Background(), "system_prompt")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, src.SourceOrder)
	assert.Equal(t, "prompt_construction", src.Domain)
	assert.True(t, src.IsAlwaysIncluded)
	assert.False(t, src.IsMemoized)
	assert.False(t, src.IsAsynchronous)
}

func TestIntegration_CoreContextAssemblySource_FindBySlug_AutoMemoryIsAsync(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	src, found, err := loader.FindBySlug(context.Background(), "auto_memory")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 5, src.SourceOrder)
	assert.Equal(t, "memory", src.Domain)
	assert.True(t, src.IsAsynchronous)
	assert.False(t, src.IsAlwaysIncluded, "auto_memory is conditional")
}

func TestIntegration_CoreContextAssemblySource_FindBySlug_EnvironmentInfoIsMemoized(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	src, found, err := loader.FindBySlug(context.Background(), "environment_info")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 2, src.SourceOrder)
	assert.True(t, src.IsMemoized, "environment_info is memoized once per session")
	assert.True(t, src.IsAlwaysIncluded)
}

func TestIntegration_CoreContextAssemblySource_FindBySlug_CompactSummariesIsConditional(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	src, found, err := loader.FindBySlug(context.Background(), "compact_summaries")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 9, src.SourceOrder)
	assert.False(t, src.IsAlwaysIncluded, "compact_summaries only present after compaction")
	assert.Equal(t, "conversation", src.Domain)
}

func TestIntegration_CoreContextAssemblySource_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreContextAssemblySource_LoadAlwaysIncluded(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadAlwaysIncluded(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 6, len(got))
	for _, src := range got {
		assert.True(t, src.IsAlwaysIncluded)
	}
}

func TestIntegration_CoreContextAssemblySource_LoadAsync(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadAsync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	for _, src := range got {
		assert.True(t, src.IsAsynchronous)
	}
}

func TestIntegration_CoreContextAssemblySource_LoadInDomain_Conversation(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadInDomain(context.Background(), "conversation")
	require.NoError(t, err)
	assert.Equal(t, 3, len(got))
	for _, src := range got {
		assert.Equal(t, "conversation", src.Domain)
	}
}

func TestIntegration_CoreContextAssemblySource_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedContextAssemblySourceRowCount, len(got))
}

func TestIntegration_CoreContextAssemblySource_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)
	applyMigration(t, pool, migDir, ctxAssemblyMigrationDown)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreContextAssemblySource_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, s := range all {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedContextAssemblySourceSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreContextAssemblySource_SourceOrderIsStrictlyIncreasing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxAssemblyMigration)

	loader := core.NewCoreContextAssemblySourceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 9, len(all))

	for i := 1; i < len(all); i++ {
		assert.Less(t, all[i-1].SourceOrder, all[i].SourceOrder,
			"source_order must be strictly increasing: %q vs %q", all[i-1].Slug, all[i].Slug)
	}
	assert.Equal(t, 1, all[0].SourceOrder)
	assert.Equal(t, 9, all[8].SourceOrder)
}
