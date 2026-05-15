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

// Integration tests for CoreCapabilityEscalationRuleLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000112 creates capability_escalation_rule table + 9 rows
//
// The ah_core.capability_escalation_rule table is created by migration
// 000112 itself (no prior migration defines it).

const capabilityEscalationRuleMigration = "000112_seed_capability_escalation_rules.up.sql"
const capabilityEscalationRuleMigrationDown = "000112_seed_capability_escalation_rules.down.sql"

func TestIntegration_Capability_LoadEscalationRules_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityEscalationRuleMigration)

	loader := core.NewCoreCapabilityEscalationRuleLoader(pool)
	got, err := loader.LoadCapabilityEscalationRules(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityEscalationRuleCount, len(got),
		"DB row count must match SeedCapabilityEscalationRuleCount (9) after migration 000112")
}

func TestIntegration_Capability_EscalationRuleMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityEscalationRuleMigration)
	// Apply 000112 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityEscalationRuleMigration)

	loader := core.NewCoreCapabilityEscalationRuleLoader(pool)
	got, err := loader.LoadCapabilityEscalationRules(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityEscalationRuleCount, len(got),
		"double-apply of migration 000112 must still produce exactly 9 escalation rule rows (idempotent)")
}

func TestIntegration_Capability_EscalationRuleDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityEscalationRuleMigration)

	loader := core.NewCoreCapabilityEscalationRuleLoader(pool)
	before, err := loader.LoadCapabilityEscalationRules(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced escalation rule rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityEscalationRuleMigrationDown)

	after, err := loader.LoadCapabilityEscalationRules(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 escalation rule rows and drop the table")
}

func TestIntegration_Capability_EscalationRuleNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityEscalationRuleLoader(pool)
	got, err := loader.LoadCapabilityEscalationRules(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadEscalationRulesForAnalyst_ReturnsThree(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityEscalationRuleMigration)

	loader := core.NewCoreCapabilityEscalationRuleLoader(pool)
	got, err := loader.LoadEscalationRulesForAgent(context.Background(), "core-analyst")
	require.NoError(t, err)

	require.Len(t, got, core.SeedAnalystEscalationRuleCount,
		"LoadEscalationRulesForAgent('core-analyst') must return exactly SeedAnalystEscalationRuleCount (3) escalation rule rows")

	// Verify each returned rule belongs to the analyst and is well-formed.
	for _, r := range got {
		assert.Equal(t, "core-analyst", r.AgentSlug,
			"all returned escalation rules must belong to core-analyst")
		assert.NotEmpty(t, r.TriggerCondition, "trigger_condition must not be empty")
		assert.NotEmpty(t, r.ThresholdValue, "threshold_value must not be empty")
		assert.NotEmpty(t, r.Action, "action must not be empty")
		assert.True(t, r.IsActive, "all seeded escalation rules must be active")
	}
}
