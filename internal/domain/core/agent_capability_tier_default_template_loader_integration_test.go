//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const capTierMigration = "000078_seed_agent_capability_tier_default_templates.up.sql"
const capTierMigrationDown = "000078_seed_agent_capability_tier_default_templates.down.sql"

func TestIntegration_CoreCapabilityTier_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCapabilityTierRowCount, len(got))
}

func TestIntegration_CoreCapabilityTier_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCapabilityTier_FindBySlug_StandardIsDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	tier, found, err := loader.FindBySlug(context.Background(), "standard")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "default", tier.PermissionMode)
	assert.Equal(t, "standard", tier.ToolAccessLevel)
	assert.Equal(t, "standard", tier.GovernanceLevel)
	assert.False(t, tier.AllowsBackgroundRun)
	assert.False(t, tier.RequiresHumanCheckpoint)
}

func TestIntegration_CoreCapabilityTier_FindBySlug_GovernedRequiresCheckpoint(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	tier, found, err := loader.FindBySlug(context.Background(), "governed")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, tier.RequiresHumanCheckpoint,
		"governed tier must require human checkpoint (§13 human_authority design value)")
	assert.Equal(t, "human_authority", tier.DesignValueProfile)
	assert.Equal(t, "strict", tier.GovernanceLevel)
}

func TestIntegration_CoreCapabilityTier_FindBySlug_ReadOnlyHasRestrictedContext(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	tier, found, err := loader.FindBySlug(context.Background(), "read-only")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "read-only", tier.ToolAccessLevel)
	assert.Less(t, tier.ContextWindowFraction, 1.0,
		"read-only tier uses a restricted context window fraction")
	assert.Equal(t, "safety", tier.DesignValueProfile)
}

func TestIntegration_CoreCapabilityTier_FindBySlug_AutonomousHasBypassPermissions(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	tier, found, err := loader.FindBySlug(context.Background(), "autonomous")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "bypassPermissions", tier.PermissionMode)
	assert.True(t, tier.AllowsBackgroundRun)
	assert.Equal(t, "full", tier.ToolAccessLevel)
}

func TestIntegration_CoreCapabilityTier_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCapabilityTier_LoadByGovernanceLevel_Strict(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	got, err := loader.LoadByGovernanceLevel(context.Background(), "strict")
	require.NoError(t, err)
	// governed + background
	assert.Equal(t, 2, len(got), "2 strict-governance tiers expected")
	for _, t2 := range got {
		assert.Equal(t, "strict", t2.GovernanceLevel)
	}
}

func TestIntegration_CoreCapabilityTier_LoadBackgroundCapable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	got, err := loader.LoadBackgroundCapable(context.Background())
	require.NoError(t, err)
	// background + autonomous
	assert.Equal(t, 2, len(got), "2 background-capable tiers expected")
	for _, t2 := range got {
		assert.True(t, t2.AllowsBackgroundRun)
	}
}

func TestIntegration_CoreCapabilityTier_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCapabilityTierRowCount, len(got))
}

func TestIntegration_CoreCapabilityTier_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)
	applyMigration(t, pool, migDir, capTierMigrationDown)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCapabilityTier_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, t2 := range all {
		dbSlugs = append(dbSlugs, t2.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedCapabilityTierSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreCapabilityTier_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)

	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].SortOrder, all[i].SortOrder)
	}
	assert.Equal(t, "read-only", all[0].Slug, "read-only sorts first")
}

func TestIntegration_CoreCapabilityTier_AllGovernanceLevelsInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, g := range core.SeedCapabilityTierGovernanceLevels {
		allowed[g] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.GovernanceLevel],
			"tier %q has unknown governance_level %q", t2.Slug, t2.GovernanceLevel)
	}
}

func TestIntegration_CoreCapabilityTier_ContextWindowFractionInRange(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.ContextWindowFraction, 0.0)
		assert.LessOrEqual(t, t2.ContextWindowFraction, 1.0)
	}
}

func TestIntegration_CoreCapabilityTier_RecommendedForIsNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTierMigration)

	loader := core.NewCoreAgentCapabilityTierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, t2 := range all {
		assert.NotEmpty(t, t2.RecommendedFor,
			"tier %q must have at least one recommended_for entry", t2.Slug)
	}
}
