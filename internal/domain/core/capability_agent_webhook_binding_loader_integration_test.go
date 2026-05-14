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

// Integration tests for CoreCapabilityAgentWebhookBindingLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000099 creates capability_agent_webhook_binding table + 3 rows
//
// The ah_core.capability_agent_webhook_binding table is created by migration
// 000099 itself (no prior migration defines it).

const capabilityAgentWebhookBindingMigration = "000099_seed_capability_agent_webhook_bindings.up.sql"
const capabilityAgentWebhookBindingMigrationDown = "000099_seed_capability_agent_webhook_bindings.down.sql"

func TestIntegration_Capability_LoadAgentWebhookBindings_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentWebhookBindingMigration)

	loader := core.NewCoreCapabilityAgentWebhookBindingLoader(pool)
	got, err := loader.LoadCapabilityAgentWebhookBindings(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentWebhookBindingCount, len(got),
		"DB row count must match SeedCapabilityAgentWebhookBindingCount (3) after migration 000099")
}

func TestIntegration_Capability_AgentWebhookBindingMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentWebhookBindingMigration)
	// Apply 000099 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentWebhookBindingMigration)

	loader := core.NewCoreCapabilityAgentWebhookBindingLoader(pool)
	got, err := loader.LoadCapabilityAgentWebhookBindings(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentWebhookBindingCount, len(got),
		"double-apply of migration 000099 must still produce exactly 3 capability agent–webhook bindings (idempotent)")
}

func TestIntegration_Capability_AgentWebhookBindingDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentWebhookBindingMigration)

	loader := core.NewCoreCapabilityAgentWebhookBindingLoader(pool)
	before, err := loader.LoadCapabilityAgentWebhookBindings(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability agent–webhook binding rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentWebhookBindingMigrationDown)

	after, err := loader.LoadCapabilityAgentWebhookBindings(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 3 capability agent–webhook bindings and drop the table")
}

func TestIntegration_Capability_AgentWebhookBindingNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentWebhookBindingLoader(pool)
	got, err := loader.LoadCapabilityAgentWebhookBindings(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadWebhookBindingsForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentWebhookBindingMigration)

	loader := core.NewCoreCapabilityAgentWebhookBindingLoader(pool)
	got, err := loader.LoadWebhookBindingsForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, 1,
		"core-researcher must have exactly 1 webhook binding after migration 000099")
	assert.Equal(t, "core-researcher", got[0].AgentSlug,
		"returned binding must have AgentSlug=core-researcher")
	assert.Equal(t, "capability-research-complete", got[0].WebhookSlug,
		"core-researcher must be bound to capability-research-complete webhook")
	assert.True(t, got[0].IsActive,
		"core-researcher webhook binding must be is_active=true after seed")
}
