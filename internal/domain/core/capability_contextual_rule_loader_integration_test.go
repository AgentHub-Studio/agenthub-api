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

// Integration tests for CoreCapabilityContextualRuleLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000114 creates capability_contextual_rule table + 9 rows
//
// The ah_core.capability_contextual_rule table is created by migration 000114
// itself (no prior migration defines it).

const capabilityContextualRuleMigration = "000114_seed_capability_contextual_rules.up.sql"
const capabilityContextualRuleMigrationDown = "000114_seed_capability_contextual_rules.down.sql"

func TestIntegration_ContextualRule_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityContextualRuleMigration)

	loader := core.NewCoreCapabilityContextualRuleLoader(pool)
	got, err := loader.LoadCapabilityContextualRules(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedContextualRuleCount, len(got),
		"DB row count must match SeedContextualRuleCount (9) after migration 000114")
}

func TestIntegration_ContextualRule_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityContextualRuleLoader(pool)
	got, err := loader.LoadCapabilityContextualRules(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_ContextualRule_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityContextualRuleMigration)
	// Apply 000114 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityContextualRuleMigration)

	loader := core.NewCoreCapabilityContextualRuleLoader(pool)
	got, err := loader.LoadCapabilityContextualRules(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedContextualRuleCount, len(got),
		"double-apply of migration 000114 must still produce exactly 9 contextual rule rows (idempotent)")
}

func TestIntegration_ContextualRule_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityContextualRuleMigration)

	loader := core.NewCoreCapabilityContextualRuleLoader(pool)
	before, err := loader.LoadCapabilityContextualRules(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed must produce contextual rule rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityContextualRuleMigrationDown)

	after, err := loader.LoadCapabilityContextualRules(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 contextual rule rows and drop the table")
}

func TestIntegration_ContextualRule_LoadForAgentReturnsThreeRules(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityContextualRuleMigration)

	loader := core.NewCoreCapabilityContextualRuleLoader(pool)
	got, err := loader.LoadContextualRulesForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, core.SeedResearcherContextualRuleCount,
		"LoadContextualRulesForAgent('core-researcher') must return exactly SeedResearcherContextualRuleCount (3) rows")

	// Verify each returned rule belongs to the researcher and is well-formed.
	for _, r := range got {
		assert.Equal(t, "core-researcher", r.AgentSlug,
			"all returned contextual rules must belong to core-researcher")
		assert.NotEmpty(t, r.RuleText, "rule_text must not be empty")
		assert.NotEmpty(t, r.TriggerContext, "trigger_context must not be empty")
		assert.True(t, r.IsActive, "all seeded contextual rules must be active")
	}
}

func TestIntegration_ContextualRule_AllRulesActiveByDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityContextualRuleMigration)

	loader := core.NewCoreCapabilityContextualRuleLoader(pool)
	got, err := loader.LoadCapabilityContextualRules(context.Background())
	require.NoError(t, err)
	require.Len(t, got, core.SeedContextualRuleCount,
		"precondition: all 9 rows must be returned by LoadCapabilityContextualRules")

	for _, r := range got {
		assert.True(t, r.IsActive,
			"contextual rule for agent %q (rule: %q) must be active by default",
			r.AgentSlug, r.RuleText)
	}
}
