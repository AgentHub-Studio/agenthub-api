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

// Integration tests for CoreTelemetrySinkLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.

func TestIntegration_CoreTelemetrySink_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedTelemetrySinkSlugs), len(got))
	gotSlugs := map[string]core.CoreTelemetrySink{}
	for _, s := range got {
		gotSlugs[s.Slug] = s
	}
	for _, slug := range core.SeedExpectedTelemetrySinkSlugs {
		assert.Contains(t, gotSlugs, slug)
	}
}

func TestIntegration_CoreTelemetrySink_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTelemetrySink_FindBySlug_ReturnsKnownSink(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	s, found, err := loader.FindBySlug(context.Background(), "otel-collector-traces")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "traces", s.SinkKind)
	assert.Equal(t, "otlp_http", s.Protocol)
	assert.Equal(t, "opentelemetry", s.Vendor)
	assert.False(t, s.RequiresAuth, "OTel collector default = no auth")
	assert.True(t, s.IsOfficial)
}

func TestIntegration_CoreTelemetrySink_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "fake-sink")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreTelemetrySink_LoadByKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadByKind(context.Background(), "traces")
	require.NoError(t, err)
	assert.Equal(t, 3, len(got), "3 trace sinks expected")
	for _, s := range got {
		assert.Equal(t, "traces", s.SinkKind)
	}
}

func TestIntegration_CoreTelemetrySink_LoadByKind_UnknownReturnsEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadByKind(context.Background(), "unknown")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTelemetrySink_AllKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedTelemetrySinkKinds {
		allowed[k] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.SinkKind], "DB sink %q kind %q outside allowlist",
			s.Slug, s.SinkKind)
	}
}

func TestIntegration_CoreTelemetrySink_AllProtocolsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedTelemetrySinkProtocols {
		allowed[p] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.Protocol], "DB sink %q protocol %q outside allowlist",
			s.Slug, s.Protocol)
	}
}

func TestIntegration_CoreTelemetrySink_AllAuthTypesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, a := range core.SeedExpectedTelemetrySinkAuthTypes {
		allowed[a] = true
	}
	for _, s := range got {
		assert.True(t, allowed[s.AuthType], "DB sink %q auth %q outside allowlist",
			s.Slug, s.AuthType)
	}
}

func TestIntegration_CoreTelemetrySink_AllSeedRowsAreOfficial(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.True(t, s.IsOfficial,
			"seed sink %q must be is_official=TRUE — only curated entries", s.Slug)
	}
}

func TestIntegration_CoreTelemetrySink_RequiresAuthHasNonEmptyAuthType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		if s.RequiresAuth {
			assert.NotEqual(t, "none", s.AuthType,
				"sink %q requires_auth=true but auth_type=none", s.Slug)
		}
	}
}

func TestIntegration_CoreTelemetrySink_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, s := range got {
		assert.False(t, seen[s.Slug], "duplicate slug %q", s.Slug)
		seen[s.Slug] = true
	}
}

func TestIntegration_CoreTelemetrySink_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreTelemetrySink_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, s := range got {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedTelemetrySinkSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreTelemetrySink_DocumentationURLForOfficialIsHTTPS(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		// Webhook sinks may not have a vendor doc URL (generic).
		if s.Vendor == "generic" {
			continue
		}
		assert.NotEmpty(t, s.DocumentationURL,
			"non-generic sink %q must have docs URL", s.Slug)
		assert.True(t, strings.HasPrefix(s.DocumentationURL, "https://"),
			"docs URL for %q must be HTTPS (got %q)", s.Slug, s.DocumentationURL)
	}
}

func TestIntegration_CoreTelemetrySink_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000016_seed_telemetry_sinks.up.sql")

	loader := core.NewCoreTelemetrySinkLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.GreaterOrEqual(t, len(s.Description), 30,
			"sink %q description too short (%d chars)", s.Slug, len(s.Description))
	}
}
