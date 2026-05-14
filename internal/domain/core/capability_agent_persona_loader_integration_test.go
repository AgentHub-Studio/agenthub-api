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

// Integration tests for CoreCapabilityAgentPersonaLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000104 creates capability_agent_persona table + 3 rows
//
// The ah_core.capability_agent_persona table is created by migration 000104
// itself (no prior migration defines it).

const capabilityAgentPersonaMigration = "000104_seed_capability_agent_personas.up.sql"
const capabilityAgentPersonaMigrationDown = "000104_seed_capability_agent_personas.down.sql"

func TestIntegration_Capability_LoadAgentPersonas_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPersonaMigration)

	loader := core.NewCoreCapabilityAgentPersonaLoader(pool)
	got, err := loader.LoadCapabilityAgentPersonas(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentPersonaCount, len(got),
		"DB row count must match SeedCapabilityAgentPersonaCount (3) after migration 000104")
}

func TestIntegration_Capability_AgentPersonaMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPersonaMigration)
	// Apply 000104 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentPersonaMigration)

	loader := core.NewCoreCapabilityAgentPersonaLoader(pool)
	got, err := loader.LoadCapabilityAgentPersonas(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentPersonaCount, len(got),
		"double-apply of migration 000104 must still produce exactly 3 capability agent persona rows (idempotent)")
}

func TestIntegration_Capability_AgentPersonaDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPersonaMigration)

	loader := core.NewCoreCapabilityAgentPersonaLoader(pool)
	before, err := loader.LoadCapabilityAgentPersonas(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability agent persona rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentPersonaMigrationDown)

	after, err := loader.LoadCapabilityAgentPersonas(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 3 capability agent persona rows and drop the table")
}

func TestIntegration_Capability_AgentPersonaNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentPersonaLoader(pool)
	got, err := loader.LoadCapabilityAgentPersonas(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_FindPersonaByAgentSlug_Researcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPersonaMigration)

	loader := core.NewCoreCapabilityAgentPersonaLoader(pool)
	persona, err := loader.FindPersonaByAgentSlug(context.Background(), "core-researcher")
	require.NoError(t, err)
	require.NotNil(t, persona,
		"FindPersonaByAgentSlug must return a non-nil persona for agent_slug=core-researcher")

	assert.Equal(t, core.SeedResearcherPersonaSlug, persona.Slug,
		"researcher persona slug must equal SeedResearcherPersonaSlug")
	assert.Equal(t, "core-researcher", persona.AgentSlug,
		"researcher persona agent_slug must be 'core-researcher'")
	assert.Equal(t, core.SeedResearcherTone, persona.Tone,
		"researcher persona tone must equal SeedResearcherTone ('curious')")
	assert.Len(t, persona.Traits, core.SeedResearcherTraitCount,
		"researcher persona must have exactly SeedResearcherTraitCount (4) traits")
	assert.True(t, persona.IsRecommended,
		"researcher persona must be marked is_recommended=TRUE")
}
