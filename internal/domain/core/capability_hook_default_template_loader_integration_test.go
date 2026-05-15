//go:build integration

package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreCapabilityHookLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000010 hook table + platform baseline → 000093 capability hooks
//
// The ah_core.hook table is created by migration 000010, not 000001.
// Migration 000093 depends on that table existing.

const capabilityHookMigration = "000093_seed_capability_hook_templates.up.sql"
const capabilityHookMigrationDown = "000093_seed_capability_hook_templates.down.sql"
const platformHookMigration = "000010_seed_hooks.up.sql"

func TestIntegration_CapabilityHook_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)    // creates table + platform 11 rows
	applyMigration(t, pool, migDir, capabilityHookMigration)  // adds 4 capability rows

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityHookCount, len(got),
		"DB row count must match SeedCapabilityHookCount (4) after migration 000093")
}

func TestIntegration_CapabilityHook_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema is missing.

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooks(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityHook_AllArePromptType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, h := range got {
		assert.Equal(t, "prompt", h.HookType,
			"capability hook %q must use hook_type=prompt (zero side effects)", h.Slug)
		assert.NotEmpty(t, h.InjectText,
			"prompt hook %q must have non-empty inject_text", h.Slug)
	}
}

func TestIntegration_CapabilityHook_NoneRequireAdminToDisable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, h := range got {
		assert.False(t, h.RequiresAdminToDisable,
			"capability hook %q must NOT require admin to disable — it is advisory, not safety-critical", h.Slug)
	}
}

func TestIntegration_CapabilityHook_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)
	// Apply 000093 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityHookMigration)

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityHookCount, len(got),
		"double-apply of migration 000093 must still produce exactly 4 hooks (idempotent)")
}

func TestIntegration_CapabilityHook_DownMigrationRemovesCapabilityRowsOnly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)

	capabilityLoader := core.NewCoreCapabilityHookLoader(pool)
	before, err := capabilityLoader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rows")

	// Apply down migration.
	applyMigration(t, pool, migDir, capabilityHookMigrationDown)

	after, err := capabilityLoader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after, "down migration must remove all 4 capability hooks")

	// Platform hooks must still be intact after removing only capability rows.
	platformLoader := core.NewCoreHookLoader(pool)
	platform, err := platformLoader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedHookSlugs), len(platform),
		"platform hooks must be unaffected by capability hook down migration")
}

func TestIntegration_CapabilityHook_LoadByEvent_PostToolUseReturnsTwoHooks(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooksByEvent(context.Background(), "PostToolUse")
	require.NoError(t, err)

	// Two PostToolUse hooks: cite-web-sources and index-doc-citations.
	assert.Equal(t, 2, len(got),
		"PostToolUse must return exactly 2 capability hooks")
	for _, h := range got {
		assert.Equal(t, "PostToolUse", h.Event,
			"LoadCapabilityHooksByEvent must filter strictly to target event")
	}
}

func TestIntegration_CapabilityHook_FindCiteWebSourcesHook(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)

	loader := core.NewCoreCapabilityHookLoader(pool)
	hook, found, err := loader.FindCapabilityHookBySlug(context.Background(), "capability-posttooluse-cite-web-sources")
	require.NoError(t, err)
	require.True(t, found, "cite-web-sources hook must be findable after seed")

	assert.Equal(t, "capability-posttooluse-cite-web-sources", hook.Slug)
	assert.Equal(t, "PostToolUse", hook.Event)
	assert.Equal(t, "prompt", hook.HookType)
	assert.False(t, hook.RequiresAdminToDisable,
		"cite-web-sources is advisory — tenants can disable without admin role")
	assert.NotEmpty(t, hook.Matcher,
		"cite-web-sources must scope to web tools via matcher")
	assert.Contains(t, hook.Matcher, "core-web-search",
		"matcher must include core-web-search tool")
	assert.NotEmpty(t, hook.InjectText,
		"cite-web-sources must have non-empty inject_text with citation directive")
}

func TestIntegration_CapabilityHook_LoaderDoesNotReturnPlatformHooks(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformHookMigration)
	applyMigration(t, pool, migDir, capabilityHookMigration)

	loader := core.NewCoreCapabilityHookLoader(pool)
	got, err := loader.LoadCapabilityHooks(context.Background())
	require.NoError(t, err)

	// Capability loader must return ONLY the 4 capability slugs.
	// Platform slugs (safety-*, lifecycle-*, context-*, coord-*) must NOT appear.
	gotSlugs := map[string]bool{}
	for _, h := range got {
		gotSlugs[h.Slug] = true
	}
	for _, platformSlug := range core.SeedExpectedHookSlugs {
		assert.False(t, gotSlugs[platformSlug],
			"platform hook slug %q must NOT be returned by capability hook loader", platformSlug)
	}
}
