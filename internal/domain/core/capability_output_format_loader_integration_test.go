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

// Integration tests for CoreCapabilityOutputFormatLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000115 creates capability_output_format table + 9 rows
//
// The ah_core.capability_output_format table is created by migration 000115
// itself (no prior migration defines it).

const capabilityOutputFormatMigration = "000115_seed_capability_output_formats.up.sql"
const capabilityOutputFormatMigrationDown = "000115_seed_capability_output_formats.down.sql"

func TestIntegration_OutputFormat_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOutputFormatMigration)

	loader := core.NewCoreCapabilityOutputFormatLoader(pool)
	got, err := loader.LoadCapabilityOutputFormats(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedOutputFormatCount, len(got),
		"DB row count must match SeedOutputFormatCount (9) after migration 000115")
}

func TestIntegration_OutputFormat_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityOutputFormatLoader(pool)
	got, err := loader.LoadCapabilityOutputFormats(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_OutputFormat_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOutputFormatMigration)
	// Apply 000115 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityOutputFormatMigration)

	loader := core.NewCoreCapabilityOutputFormatLoader(pool)
	got, err := loader.LoadCapabilityOutputFormats(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedOutputFormatCount, len(got),
		"double-apply of migration 000115 must still produce exactly 9 output format rows (idempotent)")
}

func TestIntegration_OutputFormat_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOutputFormatMigration)

	loader := core.NewCoreCapabilityOutputFormatLoader(pool)
	before, err := loader.LoadCapabilityOutputFormats(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed must produce output format rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityOutputFormatMigrationDown)

	after, err := loader.LoadCapabilityOutputFormats(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 output format rows and drop the table")
}

func TestIntegration_OutputFormat_LoadForAgentReturnsThreeFormats(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOutputFormatMigration)

	loader := core.NewCoreCapabilityOutputFormatLoader(pool)
	got, err := loader.LoadOutputFormatsForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, core.SeedResearcherOutputFormatCount,
		"LoadOutputFormatsForAgent('core-researcher') must return exactly SeedResearcherOutputFormatCount (3) rows")

	// Verify each returned row belongs to the researcher and is well-formed.
	for _, f := range got {
		assert.Equal(t, "core-researcher", f.AgentSlug,
			"all returned output format rows must belong to core-researcher")
		assert.NotEmpty(t, f.FormatKey, "format_key must not be empty")
		assert.NotEmpty(t, f.FormatValue, "format_value must not be empty")
		assert.NotEmpty(t, f.Description, "description must not be empty")
		assert.Greater(t, f.DisplayOrder, 0, "display_order must be positive")
	}
}

func TestIntegration_OutputFormat_GetOutputFormatValueReturnsMarkdownForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOutputFormatMigration)

	loader := core.NewCoreCapabilityOutputFormatLoader(pool)
	value, found, err := loader.GetOutputFormatValue(context.Background(), "core-researcher", core.SeedFormatKeyDefault)
	require.NoError(t, err)

	require.True(t, found,
		"GetOutputFormatValue must return found=true for core-researcher default_format after migration 000115")
	assert.Equal(t, core.SeedResearcherDefaultFormat, value,
		"GetOutputFormatValue for core-researcher default_format must return %q", core.SeedResearcherDefaultFormat)
}
