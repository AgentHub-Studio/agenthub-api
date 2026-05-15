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

// Integration tests for CoreCapabilitySystemPromptTemplateLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000100 creates system_prompt_template table + 3 rows
//
// The ah_core.system_prompt_template table is created by migration 000100
// itself (no prior migration defines it).

const capabilitySystemPromptTemplateMigration = "000100_seed_capability_system_prompt_templates.up.sql"
const capabilitySystemPromptTemplateMigrationDown = "000100_seed_capability_system_prompt_templates.down.sql"

func TestIntegration_Capability_LoadSystemPromptTemplates_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySystemPromptTemplateMigration)

	loader := core.NewCoreCapabilitySystemPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilitySystemPromptTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilitySystemPromptTemplateCount, len(got),
		"DB row count must match SeedCapabilitySystemPromptTemplateCount (3) after migration 000100")
}

func TestIntegration_Capability_SystemPromptTemplateMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySystemPromptTemplateMigration)
	// Apply 000100 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilitySystemPromptTemplateMigration)

	loader := core.NewCoreCapabilitySystemPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilitySystemPromptTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilitySystemPromptTemplateCount, len(got),
		"double-apply of migration 000100 must still produce exactly 3 capability system prompt templates (idempotent)")
}

func TestIntegration_Capability_SystemPromptTemplateDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySystemPromptTemplateMigration)

	loader := core.NewCoreCapabilitySystemPromptTemplateLoader(pool)
	before, err := loader.LoadCapabilitySystemPromptTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability system prompt template rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilitySystemPromptTemplateMigrationDown)

	after, err := loader.LoadCapabilitySystemPromptTemplates(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 3 capability system prompt templates and drop the table")
}

func TestIntegration_Capability_SystemPromptTemplateNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilitySystemPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilitySystemPromptTemplates(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_FindSystemPromptByAgentSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySystemPromptTemplateMigration)

	loader := core.NewCoreCapabilitySystemPromptTemplateLoader(pool)

	// Verify each capability agent resolves to its own system prompt.
	for promptSlug, agentSlug := range core.SeedCapabilitySystemPromptAgentMap {
		got, err := loader.FindSystemPromptTemplateByAgentSlug(context.Background(), agentSlug)
		require.NoError(t, err)

		require.NotNil(t, got,
			"FindSystemPromptTemplateByAgentSlug(%q) must return a template after migration 000100", agentSlug)
		assert.Equal(t, promptSlug, got.Slug,
			"template returned for agent %q must have slug %q", agentSlug, promptSlug)
		assert.Equal(t, agentSlug, got.AgentSlug,
			"returned template agent_slug must equal the queried agent_slug %q", agentSlug)
		assert.True(t, got.IsRecommended,
			"capability system prompt template for agent %q must be is_recommended=true after seed", agentSlug)
		assert.NotEmpty(t, got.Content,
			"capability system prompt template for agent %q must have non-empty content", agentSlug)
	}
}
