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

const sessionChannelMigration = "000083_seed_session_persistence_channel_default_templates.up.sql"
const sessionChannelMigrationDown = "000083_seed_session_persistence_channel_default_templates.down.sql"

func TestIntegration_CoreSessionPersistenceChannel_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSessionPersistenceChannelRowCount, len(got))
}

func TestIntegration_CoreSessionPersistenceChannel_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSessionPersistenceChannel_FindBySlug_SessionTranscripts(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	ch, found, err := loader.FindBySlug(context.Background(), "session_transcripts")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "jsonl", ch.StorageFormat)
	assert.True(t, ch.IsAppendOnly)
	assert.True(t, ch.IsProjectScoped)
	assert.True(t, ch.IsAlwaysActive)
}

func TestIntegration_CoreSessionPersistenceChannel_FindBySlug_GlobalPromptHistoryNotProjectScoped(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	ch, found, err := loader.FindBySlug(context.Background(), "global_prompt_history")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, ch.IsProjectScoped, "global_prompt_history is user-level, not project-scoped")
	assert.True(t, ch.IsAlwaysActive)
	assert.True(t, ch.IsAppendOnly)
}

func TestIntegration_CoreSessionPersistenceChannel_FindBySlug_SubagentSidechainsConditional(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	ch, found, err := loader.FindBySlug(context.Background(), "subagent_sidechains")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, ch.IsAlwaysActive, "subagent_sidechains only active when subagents run")
	assert.True(t, ch.IsProjectScoped)
	assert.True(t, ch.IsAppendOnly)
}

func TestIntegration_CoreSessionPersistenceChannel_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSessionPersistenceChannel_LoadAlwaysActive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	got, err := loader.LoadAlwaysActive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	for _, ch := range got {
		assert.True(t, ch.IsAlwaysActive)
	}
}

func TestIntegration_CoreSessionPersistenceChannel_LoadProjectScoped(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	got, err := loader.LoadProjectScoped(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	for _, ch := range got {
		assert.True(t, ch.IsProjectScoped)
	}
}

func TestIntegration_CoreSessionPersistenceChannel_AllChannelsAreAppendOnly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, ch := range all {
		assert.True(t, ch.IsAppendOnly, "channel %q must be append-only (§9.1 design)", ch.Slug)
	}
}

func TestIntegration_CoreSessionPersistenceChannel_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSessionPersistenceChannelRowCount, len(got))
}

func TestIntegration_CoreSessionPersistenceChannel_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)
	applyMigration(t, pool, migDir, sessionChannelMigrationDown)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSessionPersistenceChannel_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, sessionChannelMigration)

	loader := core.NewCoreSessionPersistenceChannelDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, ch := range all {
		dbSlugs = append(dbSlugs, ch.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedSessionPersistenceChannelSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
