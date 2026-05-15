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

// Integration tests for CoreCapabilitySuggestedFollowupLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.

const capabilitySuggestedFollowupMigration = "000125_seed_capability_suggested_followups.up.sql"
const capabilitySuggestedFollowupMigrationDown = "000125_seed_capability_suggested_followups.down.sql"

func TestIntegration_CapabilitySuggestedFollowup_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigration)

	loader := core.NewCoreCapabilitySuggestedFollowupLoader(pool)
	got, err := loader.LoadAllSuggestedFollowups(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedSuggestedFollowupCount, len(got))
}

func TestIntegration_CapabilitySuggestedFollowup_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)

	loader := core.NewCoreCapabilitySuggestedFollowupLoader(pool)
	got, err := loader.LoadAllSuggestedFollowups(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CapabilitySuggestedFollowup_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigration)
	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigration)

	loader := core.NewCoreCapabilitySuggestedFollowupLoader(pool)
	got, err := loader.LoadAllSuggestedFollowups(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedSuggestedFollowupCount, len(got))
}

func TestIntegration_CapabilitySuggestedFollowup_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigration)

	loader := core.NewCoreCapabilitySuggestedFollowupLoader(pool)
	before, err := loader.LoadAllSuggestedFollowups(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before)

	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigrationDown)

	after, err := loader.LoadAllSuggestedFollowups(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after)
}

func TestIntegration_CapabilitySuggestedFollowup_LoadForAgent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigration)

	loader := core.NewCoreCapabilitySuggestedFollowupLoader(pool)
	got, err := loader.LoadSuggestedFollowupsForAgent(context.Background(), "core-planner")
	require.NoError(t, err)

	require.Len(t, got, core.SeedSuggestedFollowupPerAgentCount)
	assert.Equal(t, "core-planner", got[0].AgentSlug)
	assert.Equal(t, 1, got[0].DisplayOrder)
	assert.Equal(t, core.SeedPlannerFollowupBreakIntoMilestones, got[0].FollowupSlug)
}

func TestIntegration_CapabilitySuggestedFollowup_GetFactCheckPrompt(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySuggestedFollowupMigration)

	loader := core.NewCoreCapabilitySuggestedFollowupLoader(pool)
	got, found, err := loader.GetSuggestedFollowup(
		context.Background(),
		"core-researcher",
		core.SeedResearcherFollowupFactCheckClaim,
	)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, got)

	assert.Equal(t, core.SeedResearcherFactCheckPrompt, got.PromptText)
	assert.Equal(t, 3, got.DisplayOrder)
}
