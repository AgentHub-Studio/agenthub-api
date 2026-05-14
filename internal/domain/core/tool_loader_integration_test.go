//go:build integration

package core_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreToolLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// BACKFILL — migration 000002_seed_tools predates the
// testcontainers integration-test pattern (see commands/rules/hooks/
// output_styles/mcp_servers); this file closes the audit gap.

func TestIntegration_CoreTool_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedToolSlugs), len(got),
		"DB row count must match SeedExpectedToolSlugs canonical count")

	gotSlugs := map[string]core.CoreTool{}
	for _, tl := range got {
		gotSlugs[tl.Slug] = tl
	}
	for _, slug := range core.SeedExpectedToolSlugs {
		assert.Contains(t, gotSlugs, slug, "DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreTool_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got)
}

func TestIntegration_CoreTool_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tl := range got {
		assert.False(t, seen[tl.Slug], "duplicate slug %q in DB", tl.Slug)
		seen[tl.Slug] = true
	}
}

func TestIntegration_CoreTool_AllRowsAreHTTPType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tl := range got {
		assert.Equal(t, "HTTP", tl.Type,
			"DB tool %q must use HTTP type (got %q) — backend-proxy invariant",
			tl.Slug, tl.Type)
	}
}

func TestIntegration_CoreTool_AllRowsUseCorePrefix(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tl := range got {
		assert.True(t, len(tl.Slug) > len(core.SeedExpectedToolSlugPrefix),
			"slug %q too short for prefix check", tl.Slug)
		assert.Equal(t, core.SeedExpectedToolSlugPrefix, tl.Slug[:len(core.SeedExpectedToolSlugPrefix)],
			"slug %q must start with %q (namespace contract)", tl.Slug, core.SeedExpectedToolSlugPrefix)
	}
}

func TestIntegration_CoreTool_AllConfigsHaveRequiredKeys(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tl := range got {
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(tl.Config, &cfg),
			"tool %q config must be valid JSON", tl.Slug)
		for _, requiredKey := range core.SeedRequiredToolConfigKeys {
			_, ok := cfg[requiredKey]
			assert.True(t, ok,
				"tool %q config missing required key %q", tl.Slug, requiredKey)
		}
		// useCallerToken must be the boolean true — backend-proxy contract.
		uct, _ := cfg["useCallerToken"].(bool)
		assert.True(t, uct,
			"tool %q must have useCallerToken=true (got %v)",
			tl.Slug, cfg["useCallerToken"])
	}
}

func TestIntegration_CoreTool_AllConfigKeysAreInAllowlist(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedToolConfigKeys {
		allowed[k] = true
	}
	for _, tl := range got {
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(tl.Config, &cfg))
		for k := range cfg {
			assert.True(t, allowed[k],
				"tool %q has config key %q outside allowlist %v",
				tl.Slug, k, core.SeedExpectedToolConfigKeys)
		}
	}
}

func TestIntegration_CoreTool_ConfigURLStartsWithApiPath(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tl := range got {
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(tl.Config, &cfg))
		url, _ := cfg["url"].(string)
		assert.True(t, len(url) >= 4 && url[:4] == "/api",
			"tool %q url %q must start with /api (relative path resolved against BACKEND_BASE_URL)",
			tl.Slug, url)
	}
}

func TestIntegration_CoreTool_ConfigMethodIsValidHTTPVerb(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	validVerbs := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	}
	for _, tl := range got {
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(tl.Config, &cfg))
		method, _ := cfg["method"].(string)
		assert.True(t, validVerbs[method],
			"tool %q method %q must be a valid HTTP verb",
			tl.Slug, method)
	}
}

func TestIntegration_CoreTool_LoadAll_FiltersInactive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)

	// Soft-disable one row.
	_, err = conn.Exec(context.Background(),
		`UPDATE ah_core.tool SET is_active = FALSE WHERE slug = 'core-list-agents'`)
	require.NoError(t, err)
	conn.Release()

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	// Disabled row must NOT be in result.
	for _, tl := range got {
		assert.NotEqual(t, "core-list-agents", tl.Slug,
			"disabled row must NOT be returned by LoadAll (is_active=true filter)")
	}
	assert.Equal(t, len(core.SeedExpectedToolSlugs)-1, len(got),
		"with 1 row disabled, result must be N-1")
}

func TestIntegration_CoreTool_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	// Apply twice — ON CONFLICT (slug) DO NOTHING must protect idempotency.
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedToolSlugs), len(got),
		"second-apply must not duplicate rows — ON CONFLICT DO NOTHING contract")
}

func TestIntegration_CoreTool_OrderingIsStableAcrossCalls(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)

	// The loader does ORDER BY name — but Postgres collation is locale-
	// aware so the byte-comparison contract is meaningless here. The
	// REAL contract is STABILITY: two calls return the same order (so
	// pagination / UI list stay deterministic).
	first, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, first)

	second, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, len(first), len(second))

	for i := range first {
		assert.Equal(t, first[i].Slug, second[i].Slug,
			"row %d slug differs across calls — ORDER BY contract must be stable", i)
	}
}

func TestIntegration_CoreTool_DownMigrationDropsRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000002_seed_tools.down.sql")

	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "post-down LoadAll must NOT error")
	// down migration should remove the seeded rows. Either drops the
	// table (in which case loader returns nil non-fatally) or DELETEs
	// the rows. Either is acceptable — the contract is "no seed rows
	// after down".
	for _, tl := range gotAfterDown {
		assert.False(t, len(tl.Slug) >= 5 && tl.Slug[:5] == "core-",
			"down migration must remove all seed rows; saw %q", tl.Slug)
	}
}

func TestIntegration_CoreTool_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, tl := range got {
		dbSlugs = append(dbSlugs, tl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedToolSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedToolSlugs")
}

func TestIntegration_CoreTool_DescriptionIsAlwaysPresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tl := range got {
		assert.NotEmpty(t, tl.Description,
			"tool %q must have a description for the agent's tool catalog",
			tl.Slug)
		assert.GreaterOrEqual(t, len(tl.Description), 10,
			"tool %q description too short (%d chars) — agent needs informative summary",
			tl.Slug, len(tl.Description))
	}
}

func TestIntegration_CoreTool_BodyTemplateOnlyOnWriteVerbs(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")

	loader := core.NewCoreToolLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	writeVerbs := map[string]bool{"POST": true, "PUT": true, "PATCH": true}
	for _, tl := range got {
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(tl.Config, &cfg))
		method, _ := cfg["method"].(string)
		_, hasBody := cfg["bodyTemplate"]
		if hasBody {
			assert.True(t, writeVerbs[method],
				"tool %q has bodyTemplate but method %q is not a write verb",
				tl.Slug, method)
		}
	}
}
