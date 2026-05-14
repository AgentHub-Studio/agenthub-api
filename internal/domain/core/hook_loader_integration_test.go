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

// Integration tests for CoreHookLoader against a real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
//   go test -tags=integration ./internal/domain/core/ -run TestIntegration_CoreHook

func TestIntegration_CoreHook_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedHookSlugs), len(got),
		"DB row count must match SeedExpectedHookSlugs canonical count")
	gotSlugs := map[string]core.CoreHook{}
	for _, h := range got {
		gotSlugs[h.Slug] = h
	}
	for _, slug := range core.SeedExpectedHookSlugs {
		assert.Contains(t, gotSlugs, slug, "DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreHook_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got)
}

func TestIntegration_CoreHook_FindBySlug_ReturnsKnownHook(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	hook, found, err := loader.FindBySlug(context.Background(), "safety-pretooluse-confirm-destructive")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "safety-pretooluse-confirm-destructive", hook.Slug)
	assert.Equal(t, "PreToolUse", hook.Event)
	assert.Equal(t, "prompt", hook.HookType)
	assert.True(t, hook.RequiresAdminToDisable, "safety hook must be admin-only-disable")
	assert.NotEmpty(t, hook.InjectText, "prompt hook must have inject text")
	assert.NotEmpty(t, hook.Matcher,
		"destructive-confirm hook must scope via matcher (execute-sql,shell,agenthub_manage)")
}

func TestIntegration_CoreHook_LoadByEvent_FiltersToTargetEvent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadByEvent(context.Background(), "PreToolUse")
	require.NoError(t, err)

	// Two PreToolUse hooks expected (confirm-destructive + redact-secrets).
	assert.GreaterOrEqual(t, len(got), 2,
		"PreToolUse must return at least 2 seeded hooks")
	for _, h := range got {
		assert.Equal(t, "PreToolUse", h.Event,
			"LoadByEvent must filter strictly to target event")
	}
}

func TestIntegration_CoreHook_LoadByEvent_UnknownReturnsEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadByEvent(context.Background(), "EventThatDoesNotExist")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreHook_OrderingPriorityDescThenSortOrder(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	prevPrio := got[0].Priority + 1
	for i, h := range got {
		assert.LessOrEqual(t, h.Priority, prevPrio,
			"hook at index %d (priority %d) must have priority <= previous (%d)",
			i, h.Priority, prevPrio)
		prevPrio = h.Priority
	}

	// Highest priority must be one of the safety hooks.
	assert.Contains(t, got[0].Slug, "safety-",
		"highest-priority hook must be a safety hook")
}

func TestIntegration_CoreHook_AdminOnlyDisable_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbAdminOnly := []string{}
	for _, h := range got {
		if h.RequiresAdminToDisable {
			dbAdminOnly = append(dbAdminOnly, h.Slug)
		}
	}
	sort.Strings(dbAdminOnly)
	expected := append([]string{}, core.SeedAdminOnlyDisableHookSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbAdminOnly,
		"DB admin-only set must EXACTLY match SeedAdminOnlyDisableHookSlugs")
}

func TestIntegration_CoreHook_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, h := range got {
		assert.False(t, seen[h.Slug], "duplicate slug %q in DB", h.Slug)
		seen[h.Slug] = true
	}
}

func TestIntegration_CoreHook_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000010_seed_hooks.down.sql")

	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing table after down must NOT error")
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreHook_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, h := range got {
		dbSlugs = append(dbSlugs, h.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedHookSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedHookSlugs")
}

func TestIntegration_CoreHook_AllSeedHooksUsePromptType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, h := range got {
		assert.Equal(t, "prompt", h.HookType,
			"seed hook %q must use prompt type (got %q) — http/agent/command have side effects",
			h.Slug, h.HookType)
	}
}

func TestIntegration_CoreHook_AllSeedHooksHaveNonEmptyInjectText(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, h := range got {
		assert.NotEmpty(t, h.InjectText,
			"prompt hook %q must have non-empty inject_text", h.Slug)
		assert.GreaterOrEqual(t, len(h.InjectText), 30,
			"hook %q inject_text too short (%d chars) — directive must be informative",
			h.Slug, len(h.InjectText))
	}
}

func TestIntegration_CoreHook_AllEventsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000010_seed_hooks.up.sql")

	loader := core.NewCoreHookLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	expectedSet := map[string]bool{}
	for _, e := range core.SeedExpectedHookEvents {
		expectedSet[e] = true
	}
	for _, h := range got {
		assert.True(t, expectedSet[h.Event],
			"DB hook %q references event %q outside SeedExpectedHookEvents",
			h.Slug, h.Event)
	}
}
