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

// Integration tests for CoreCapabilityRateLimitLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000105 creates capability_rate_limit table + 9 rows
//
// The ah_core.capability_rate_limit table is created by migration 000105
// itself (no prior migration defines it).

const capabilityRateLimitMigration = "000105_seed_capability_rate_limits.up.sql"
const capabilityRateLimitMigrationDown = "000105_seed_capability_rate_limits.down.sql"

func TestIntegration_Capability_LoadRateLimits_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityRateLimitMigration)

	loader := core.NewCoreCapabilityRateLimitLoader(pool)
	got, err := loader.LoadCapabilityRateLimits(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityRateLimitCount, len(got),
		"DB row count must match SeedCapabilityRateLimitCount (9) after migration 000105")
}

func TestIntegration_Capability_RateLimitMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityRateLimitMigration)
	// Apply 000105 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityRateLimitMigration)

	loader := core.NewCoreCapabilityRateLimitLoader(pool)
	got, err := loader.LoadCapabilityRateLimits(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityRateLimitCount, len(got),
		"double-apply of migration 000105 must still produce exactly 9 rate limit rows (idempotent)")
}

func TestIntegration_Capability_RateLimitDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityRateLimitMigration)

	loader := core.NewCoreCapabilityRateLimitLoader(pool)
	before, err := loader.LoadCapabilityRateLimits(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rate limit rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityRateLimitMigrationDown)

	after, err := loader.LoadCapabilityRateLimits(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 rate limit rows and drop the table")
}

func TestIntegration_Capability_RateLimitNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityRateLimitLoader(pool)
	got, err := loader.LoadCapabilityRateLimits(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadRateLimitsForAnalyst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityRateLimitMigration)

	loader := core.NewCoreCapabilityRateLimitLoader(pool)
	got, err := loader.LoadRateLimitsForAgent(context.Background(), "core-analyst")
	require.NoError(t, err)

	require.Len(t, got, core.SeedAnalystRateLimitCount,
		"LoadRateLimitsForAgent must return exactly SeedAnalystRateLimitCount (3) rows for core-analyst")

	// Verify the analyst has the highest token budget.
	tokenLimit := -1
	for _, rl := range got {
		assert.Equal(t, "core-analyst", rl.AgentSlug,
			"all returned rows must belong to agent_slug=core-analyst")
		if rl.LimitKey == core.SeedRateLimitKeyTokensPerMinute {
			tokenLimit = rl.LimitValue
		}
	}
	assert.Equal(t, core.SeedAnalystTokensPerMinute, tokenLimit,
		"analyst tokens_per_minute limit value must equal SeedAnalystTokensPerMinute (%d)", core.SeedAnalystTokensPerMinute)
}
