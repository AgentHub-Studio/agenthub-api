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

// Integration tests for CoreCapabilityAgentLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000090 capability skills+tools → 000091 capability agents

const capabilityAgentMigration = "000091_seed_capability_agent_templates.up.sql"
const capabilityAgentMigrationDown = "000091_seed_capability_agent_templates.down.sql"

func TestIntegration_CapabilityAgent_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)      // 000090 — provides skills
	applyMigration(t, pool, migDir, capabilityAgentMigration) // 000091 — provides agents

	loader := core.NewCoreCapabilityAgentLoader(pool)
	got, err := loader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentCount, len(got),
		"DB row count must match SeedCapabilityAgentCount (3) after migration 000091")
}

func TestIntegration_CapabilityAgent_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema is missing.

	loader := core.NewCoreCapabilityAgentLoader(pool)
	got, err := loader.LoadCapabilityAgents(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty, not crash")
}

func TestIntegration_CapabilityAgent_AllAreAssistantType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityAgentMigration)

	loader := core.NewCoreCapabilityAgentLoader(pool)
	got, err := loader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, a := range got {
		assert.Equal(t, core.SeedCapabilityAgentType, a.AgentType,
			"capability agent %q must be ASSISTANT type", a.Slug)
	}
}

func TestIntegration_CapabilityAgent_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityAgentMigration)
	// Apply 000091 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentMigration)

	loader := core.NewCoreCapabilityAgentLoader(pool)
	got, err := loader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentCount, len(got),
		"double-apply of migration 000091 must still produce exactly 3 agents (idempotent)")
}

func TestIntegration_CapabilityAgent_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityAgentMigration)

	// Verify rows exist before down migration.
	loader := core.NewCoreCapabilityAgentLoader(pool)
	before, err := loader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced rows")

	// Apply down migration.
	applyMigration(t, pool, migDir, capabilityAgentMigrationDown)

	after, err := loader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after, "down migration must remove all 3 capability agents")
}

func TestIntegration_CapabilityAgent_FindBySlugResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityAgentMigration)

	loader := core.NewCoreCapabilityAgentLoader(pool)
	agent, found, err := loader.FindCapabilityAgentBySlug(context.Background(), "core-researcher")
	require.NoError(t, err)
	require.True(t, found, "core-researcher must be findable after seed")

	assert.Equal(t, "core-researcher", agent.Slug)
	assert.Equal(t, "ASSISTANT", agent.AgentType)
	assert.True(t, agent.IsActive)
	assert.False(t, agent.EnableManagement, "capability agents must not have management enabled")
	assert.NotEmpty(t, agent.SystemPrompt, "core-researcher must have a system_prompt")
	assert.GreaterOrEqual(t, len(agent.SystemPrompt), 100,
		"system_prompt must be substantive (at least 100 chars)")
}

func TestIntegration_CapabilityAgent_AllSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityAgentMigration)

	loader := core.NewCoreCapabilityAgentLoader(pool)
	got, err := loader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, a := range got {
		dbSlugs = append(dbSlugs, a.Slug)
	}

	for _, expected := range core.SeedCapabilityAgentSlugs {
		assert.Contains(t, dbSlugs, expected,
			"DB must contain canonical slug %q", expected)
	}

	assert.Equal(t, len(core.SeedCapabilityAgentSlugs), len(dbSlugs),
		"DB must have exactly the canonical slugs — no extras")
}

func TestIntegration_CapabilityAgent_SkillBindingsExistAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityAgentMigration)

	// Load agent loader to access skill bindings.
	agentLoader := core.NewCoreAgentLoader(pool)
	bindings, err := agentLoader.LoadSkillBindings(context.Background())
	require.NoError(t, err)

	// Load the capability agents to find their IDs.
	capLoader := core.NewCoreCapabilityAgentLoader(pool)
	agents, err := capLoader.LoadCapabilityAgents(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, len(agents))

	capAgentIDs := map[string]bool{}
	for _, a := range agents {
		capAgentIDs[a.ID.String()] = true
	}

	// Count bindings that belong to capability agents.
	capBindings := 0
	for _, b := range bindings {
		if capAgentIDs[b.AgentID.String()] {
			capBindings++
		}
	}

	assert.Equal(t, core.SeedCapabilityAgentSkillBindingCount, capBindings,
		"exactly 3 agent→skill bindings must exist for capability agents (1 per agent)")
}
