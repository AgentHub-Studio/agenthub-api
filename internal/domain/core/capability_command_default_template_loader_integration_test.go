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

// Integration tests for CoreCapabilityCommandLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000008 command table + platform seed → 000092 capability commands
//
// The ah_core.command table is created by migration 000008, not 000001.
// Migration 000092 depends on that table existing.

const capabilityCommandMigration = "000092_seed_capability_command_templates.up.sql"
const capabilityCommandMigrationDown = "000092_seed_capability_command_templates.down.sql"
const platformCommandMigration = "000008_seed_commands.up.sql"

func TestIntegration_CapabilityCommand_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)    // creates table + platform 13 rows
	applyMigration(t, pool, migDir, capabilityCommandMigration)  // adds 5 capability rows

	loader := core.NewCoreCapabilityCommandLoader(pool)
	got, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityCommandCount, len(got),
		"DB row count must match SeedCapabilityCommandCount (5) after migration 000092")
}

func TestIntegration_CapabilityCommand_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema is missing.

	loader := core.NewCoreCapabilityCommandLoader(pool)
	got, err := loader.LoadCapabilityCommands(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityCommand_AllAreCapabilityCategory(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)
	applyMigration(t, pool, migDir, capabilityCommandMigration)

	loader := core.NewCoreCapabilityCommandLoader(pool)
	got, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, cmd := range got {
		assert.Equal(t, core.SeedCapabilityCommandCategory, cmd.Category,
			"command %q must have category %q", cmd.Slug, core.SeedCapabilityCommandCategory)
	}
}

func TestIntegration_CapabilityCommand_AllArePromptHandlerType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)
	applyMigration(t, pool, migDir, capabilityCommandMigration)

	loader := core.NewCoreCapabilityCommandLoader(pool)
	got, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, cmd := range got {
		assert.Equal(t, core.SeedCapabilityCommandHandlerType, cmd.HandlerType,
			"command %q must use handler_type=%q", cmd.Slug, core.SeedCapabilityCommandHandlerType)
		// Also verify prompt_template is non-empty for prompt-type commands.
		assert.NotEmpty(t, cmd.PromptTemplate,
			"command %q with handler_type=prompt must have a non-empty prompt_template", cmd.Slug)
	}
}

func TestIntegration_CapabilityCommand_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)
	applyMigration(t, pool, migDir, capabilityCommandMigration)
	// Apply 000092 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityCommandMigration)

	loader := core.NewCoreCapabilityCommandLoader(pool)
	got, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityCommandCount, len(got),
		"double-apply of migration 000092 must still produce exactly 5 commands (idempotent)")
}

func TestIntegration_CapabilityCommand_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)
	applyMigration(t, pool, migDir, capabilityCommandMigration)

	loader := core.NewCoreCapabilityCommandLoader(pool)
	before, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced rows")

	// Apply down migration.
	applyMigration(t, pool, migDir, capabilityCommandMigrationDown)

	after, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after, "down migration must remove all 5 capability commands")

	// Platform commands must still be intact after removing only capability rows.
	platformLoader := core.NewCoreCommandLoader(pool)
	platform, err := platformLoader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedSlugs), len(platform),
		"platform commands must be unaffected by capability command down migration")
}

func TestIntegration_CapabilityCommand_FindResearchCommand(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)
	applyMigration(t, pool, migDir, capabilityCommandMigration)

	loader := core.NewCoreCapabilityCommandLoader(pool)
	cmd, found, err := loader.FindCapabilityCommandBySlug(context.Background(), core.SeedCapabilityResearchSlug)
	require.NoError(t, err)
	require.True(t, found, "/research must be findable after seed")

	assert.Equal(t, core.SeedCapabilityResearchSlug, cmd.Slug)
	assert.Equal(t, core.SeedCapabilityCommandCategory, cmd.Category)
	assert.Equal(t, core.SeedCapabilityCommandHandlerType, cmd.HandlerType)
	assert.True(t, cmd.IsActive)
	assert.False(t, cmd.DisableModelInvocation,
		"/research must have disable_model_invocation=false — LLM may invoke as meta-tool")
	assert.NotEmpty(t, cmd.PromptTemplate,
		"/research must have a prompt_template for runner expansion")
	assert.NotEmpty(t, cmd.ArgumentHint,
		"/research must have an argument_hint guiding users on what to pass")
}

func TestIntegration_CapabilityCommand_LoaderDoesNotReturnPlatformCommands(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformCommandMigration)
	applyMigration(t, pool, migDir, capabilityCommandMigration)

	loader := core.NewCoreCapabilityCommandLoader(pool)
	got, err := loader.LoadCapabilityCommands(context.Background())
	require.NoError(t, err)

	// Capability loader must return ONLY 'capability' category rows.
	// Platform slugs (help, clear, reset, etc.) must NOT appear here.
	gotSlugs := map[string]bool{}
	for _, cmd := range got {
		gotSlugs[cmd.Slug] = true
	}
	for _, platformSlug := range core.SeedExpectedSlugs {
		assert.False(t, gotSlugs[platformSlug],
			"platform slug %q must NOT be returned by capability command loader", platformSlug)
	}
}
