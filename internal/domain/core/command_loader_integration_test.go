//go:build integration

package core_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreCommandLoader against a real Postgres
// (testcontainers pgvector:pg16). These exercise the actual seed
// migration and prove the loader's contract against the real DB —
// not a mock. Build tag `integration` keeps them out of the fast unit
// suite; run via:
//
//   go test -tags=integration ./internal/domain/core/ -run TestIntegration_CoreCommand
//
// The migrations applied here:
//   000001_ah_core_schema.up.sql  — base ah_core schema (tools/skills/agents)
//   000008_seed_commands.up.sql   — command table + 13-row seed (this iteration)

// ah_coreMigrationsDir resolves the path to the ah_core migrations directory
// relative to this test file (internal/domain/core/).
func ah_coreMigrationsDir(t *testing.T) string {
	t.Helper()
	// internal/domain/core/<test> -> ../../../migrations/ah_core
	abs, err := filepath.Abs("../../../migrations/ah_core")
	require.NoError(t, err)
	require.DirExists(t, abs, "ah_core migrations dir must exist")
	return abs
}

// applyMigration loads a migration file from disk and executes it against
// the test pool. We apply only the migrations relevant to commands:
//   - 000001_ah_core_schema.up.sql: prerequisite schema (CREATE SCHEMA + base tables).
//   - 000008_seed_commands.up.sql: the migration under test.
func applyMigration(t *testing.T, pool *pgxpool.Pool, migrationsDir, name string) {
	t.Helper()
	path := filepath.Join(migrationsDir, name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err, "read migration %s", name)
	require.NotEmpty(t, sql, "migration %s must not be empty", name)
	testutil.MustExec(t, pool, string(sql))
}

func TestIntegration_CoreCommand_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)

	// Apply prerequisite schema + the commands seed migration.
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// The seed claims 13 rows. Assert actual DB contents match the
	// canonical Go constant.
	assert.Equal(t, len(core.SeedExpectedSlugs), len(got),
		"DB row count must match SeedExpectedSlugs canonical count")

	// Every expected slug must appear in the DB result.
	gotSlugs := map[string]core.CoreCommand{}
	for _, c := range got {
		gotSlugs[c.Slug] = c
	}
	for _, slug := range core.SeedExpectedSlugs {
		assert.Contains(t, gotSlugs, slug,
			"DB must contain seeded slug %q", slug)
	}

	// All loaded categories must be in the canonical set.
	categorySet := map[string]bool{}
	for _, c := range core.SeedExpectedCategories {
		categorySet[c] = true
	}
	for _, c := range got {
		assert.True(t, categorySet[c.Category],
			"DB row %q has category %q outside the canonical set",
			c.Slug, c.Category)
	}
}

func TestIntegration_CoreCommand_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply the migration — schema is missing on purpose.

	loader := core.NewCoreCommandLoader(pool)
	got, err := loader.LoadAll(context.Background())

	// Contract: missing schema is non-fatal (returns nil, nil).
	require.NoError(t, err,
		"missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got,
		"missing schema must return empty slice, not crash")
}

func TestIntegration_CoreCommand_FindBySlug_ReturnsKnownCommand(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)

	cmd, found, err := loader.FindBySlug(context.Background(), "help")
	require.NoError(t, err)
	require.True(t, found, "/help must be findable after seed")
	assert.Equal(t, "help", cmd.Slug)
	assert.Equal(t, "Help", cmd.Name)
	assert.Equal(t, "session", cmd.Category)
	assert.Equal(t, "builtin", cmd.HandlerType)
	assert.True(t, cmd.IsActive)
	assert.True(t, cmd.DisableModelInvocation,
		"/help is user-invokable only (disable_model_invocation=true)")
}

func TestIntegration_CoreCommand_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)

	_, found, err := loader.FindBySlug(context.Background(), "this-does-not-exist")
	require.NoError(t, err,
		"unknown slug must not error")
	assert.False(t, found,
		"unknown slug must report not-found")
}

func TestIntegration_CoreCommand_OrderingIsStableByCategorySortOrderSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	// Verify the loader's ORDER BY (category, sort_order, slug) produces
	// a stable, monotonic sequence — operators relying on the ordering
	// for UI palette rendering must get deterministic output.
	prevCat := ""
	prevSort := -1
	prevSlug := ""
	for i, c := range got {
		if c.Category != prevCat {
			// New category — sort_order resets within the category but
			// categories themselves must be sorted ascending.
			assert.True(t, c.Category > prevCat || prevCat == "",
				"category at index %d (%q) must be >= previous (%q)",
				i, c.Category, prevCat)
			prevCat = c.Category
			prevSort = -1
			prevSlug = ""
			continue
		}
		// Same category — sort_order monotonic, then slug alphabetical.
		if c.SortOrder == prevSort {
			assert.True(t, c.Slug >= prevSlug,
				"within category %q at sort_order %d, slug %q must be >= %q",
				c.Category, c.SortOrder, c.Slug, prevSlug)
		} else {
			assert.True(t, c.SortOrder > prevSort,
				"sort_order at index %d in category %q must increase: %d > %d",
				i, c.Category, c.SortOrder, prevSort)
		}
		prevSort = c.SortOrder
		prevSlug = c.Slug
	}
}

func TestIntegration_CoreCommand_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// The schema declares slug UNIQUE — verify the DB enforces it AND
	// the loader did not duplicate rows.
	seen := map[string]bool{}
	for _, c := range got {
		assert.False(t, seen[c.Slug], "duplicate slug %q in DB", c.Slug)
		seen[c.Slug] = true
	}
}

func TestIntegration_CoreCommand_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got, "precondition: seed produced rows")

	// Apply the down migration.
	applyMigration(t, pool, migDir, "000008_seed_commands.down.sql")

	// After down, the loader must report empty (schema-missing
	// non-fatal contract kicks in).
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err,
		"missing table after down must NOT error")
	assert.Empty(t, gotAfterDown,
		"down migration must drop the table cleanly")
}

func TestIntegration_CoreCommand_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000008_seed_commands.up.sql")

	loader := core.NewCoreCommandLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, c := range got {
		dbSlugs = append(dbSlugs, c.Slug)
	}
	sort.Strings(dbSlugs)

	expected := append([]string{}, core.SeedExpectedSlugs...)
	sort.Strings(expected)

	// Exact equality — a missing or extra slug breaks the constant
	// guard SeedExpectedSlugs and surfaces the drift.
	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedSlugs (no missing, no extras)")
}
