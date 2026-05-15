//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreMCPServerLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CoreMCPServer_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedMCPServerSlugs), len(got),
		"DB row count must match SeedExpectedMCPServerSlugs canonical count")
	gotSlugs := map[string]core.CoreMCPServer{}
	for _, s := range got {
		gotSlugs[s.Slug] = s
	}
	for _, slug := range core.SeedExpectedMCPServerSlugs {
		assert.Contains(t, gotSlugs, slug, "DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreMCPServer_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got)
}

func TestIntegration_CoreMCPServer_FindBySlug_ReturnsKnownServer(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	server, found, err := loader.FindBySlug(context.Background(), "github")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "github", server.Slug)
	assert.Equal(t, "http", server.TransportType)
	assert.Equal(t, "development", server.Category)
	assert.Equal(t, "github", server.Vendor)
	assert.Equal(t, "oauth2", server.AuthType)
	assert.True(t, server.RequiresAuth)
	assert.True(t, server.IsOfficial)
	assert.Contains(t, server.HTTPBaseURL, "api.github.com")
}

func TestIntegration_CoreMCPServer_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreMCPServer_LoadByCategory_FiltersToTargetCategory(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadByCategory(context.Background(), "development")
	require.NoError(t, err)

	// 3 development servers expected: github + gitlab + postgres-readonly.
	assert.Equal(t, 3, len(got),
		"development category must return exactly 3 seeded servers")
	for _, s := range got {
		assert.Equal(t, "development", s.Category,
			"LoadByCategory must filter strictly to target category")
	}
}

func TestIntegration_CoreMCPServer_LoadByCategory_UnknownReturnsEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadByCategory(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMCPServer_AllCategoriesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedMCPServerCategories {
		allowed[c] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.Category],
			"DB server %q references category %q outside SeedExpectedMCPServerCategories",
			s.Slug, s.Category)
	}
}

func TestIntegration_CoreMCPServer_AllAuthTypesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, a := range core.SeedExpectedMCPServerAuthTypes {
		allowed[a] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.AuthType],
			"DB server %q references auth_type %q outside allowlist",
			s.Slug, s.AuthType)
	}
}

func TestIntegration_CoreMCPServer_AllTransportsAreHTTP(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.Equal(t, "http", s.TransportType,
			"DB server %q must use http transport (got %q) — web product invariant",
			s.Slug, s.TransportType)
	}
}

func TestIntegration_CoreMCPServer_AllSeedRowsAreOfficial(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.True(t, s.IsOfficial,
			"seed row %q must be is_official=TRUE — only curate vendor-published servers",
			s.Slug)
	}
}

func TestIntegration_CoreMCPServer_AuthRequiredFlag_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbAuthRequired := []string{}
	for _, s := range got {
		if s.RequiresAuth {
			dbAuthRequired = append(dbAuthRequired, s.Slug)
		}
	}
	sort.Strings(dbAuthRequired)
	expected := append([]string{}, core.SeedAuthRequiredMCPServerSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbAuthRequired,
		"DB requires_auth set must EXACTLY match SeedAuthRequiredMCPServerSlugs")
}

func TestIntegration_CoreMCPServer_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, s := range got {
		assert.False(t, seen[s.Slug], "duplicate slug %q in DB", s.Slug)
		seen[s.Slug] = true
	}
}

func TestIntegration_CoreMCPServer_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.down.sql")

	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing table after down must NOT error")
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreMCPServer_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, s := range got {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedMCPServerSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedMCPServerSlugs")
}

func TestIntegration_CoreMCPServer_AuthRequiredHasNonEmptyAuthType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		if s.RequiresAuth {
			assert.NotEqual(t, "none", s.AuthType,
				"server %q requires_auth=TRUE but auth_type=none — UI cannot show picker",
				s.Slug)
		}
	}
}

func TestIntegration_CoreMCPServer_NoAuthHasAuthTypeNone(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		if !s.RequiresAuth {
			assert.Equal(t, "none", s.AuthType,
				"server %q requires_auth=FALSE must have auth_type=none — picker contract", s.Slug)
		}
	}
}

func TestIntegration_CoreMCPServer_DocumentationURLPresentForOfficialServers(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		if s.IsOfficial {
			assert.NotEmpty(t, s.DocumentationURL,
				"official server %q must have documentation_url for trust badge", s.Slug)
			assert.True(t, strings.HasPrefix(s.DocumentationURL, "https://"),
				"documentation_url for %q must be HTTPS (got %q)", s.Slug, s.DocumentationURL)
		}
	}
}

func TestIntegration_CoreMCPServer_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000012_seed_mcp_servers.up.sql")

	loader := core.NewCoreMCPServerLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	prevSort := got[0].SortOrder - 1
	for i, s := range got {
		assert.GreaterOrEqual(t, s.SortOrder, prevSort,
			"server at index %d (sort %d) must have sort >= previous (%d)",
			i, s.SortOrder, prevSort)
		prevSort = s.SortOrder
	}
}
