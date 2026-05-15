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

// Integration tests for CoreCapabilityOnboardingChecklistLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000106 creates capability_onboarding_checklist table + 5 rows
//
// The ah_core.capability_onboarding_checklist table is created by migration
// 000106 itself (no prior migration defines it).

const capabilityOnboardingChecklistMigration = "000106_seed_capability_onboarding_checklist.up.sql"
const capabilityOnboardingChecklistMigrationDown = "000106_seed_capability_onboarding_checklist.down.sql"

func TestIntegration_Capability_LoadOnboardingChecklist_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOnboardingChecklistMigration)

	loader := core.NewCoreCapabilityOnboardingChecklistLoader(pool)
	got, err := loader.LoadCapabilityOnboardingChecklist(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityOnboardingChecklistCount, len(got),
		"DB row count must match SeedCapabilityOnboardingChecklistCount (5) after migration 000106")
}

func TestIntegration_Capability_OnboardingChecklistMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOnboardingChecklistMigration)
	// Apply 000106 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityOnboardingChecklistMigration)

	loader := core.NewCoreCapabilityOnboardingChecklistLoader(pool)
	got, err := loader.LoadCapabilityOnboardingChecklist(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityOnboardingChecklistCount, len(got),
		"double-apply of migration 000106 must still produce exactly 5 onboarding checklist rows (idempotent)")
}

func TestIntegration_Capability_OnboardingChecklistDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOnboardingChecklistMigration)

	loader := core.NewCoreCapabilityOnboardingChecklistLoader(pool)
	before, err := loader.LoadCapabilityOnboardingChecklist(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced onboarding checklist rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityOnboardingChecklistMigrationDown)

	after, err := loader.LoadCapabilityOnboardingChecklist(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 5 onboarding checklist rows and drop the table")
}

func TestIntegration_Capability_OnboardingChecklistNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityOnboardingChecklistLoader(pool)
	got, err := loader.LoadCapabilityOnboardingChecklist(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadBlockingOnboardingSteps_ReturnsThree(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityOnboardingChecklistMigration)

	loader := core.NewCoreCapabilityOnboardingChecklistLoader(pool)
	got, err := loader.LoadBlockingOnboardingSteps(context.Background())
	require.NoError(t, err)

	require.Len(t, got, core.SeedOnboardingBlockingStepCount,
		"LoadBlockingOnboardingSteps must return exactly SeedOnboardingBlockingStepCount (3) rows")

	// All returned rows must have is_blocking=TRUE and step_order 1-3.
	for _, item := range got {
		assert.True(t, item.IsBlocking,
			"LoadBlockingOnboardingSteps must only return rows with is_blocking=TRUE, got slug=%q", item.Slug)
		assert.True(t, item.StepOrder >= 1 && item.StepOrder <= core.SeedOnboardingBlockingStepCount,
			"blocking step step_order must be in [1,%d], got %d for slug=%q",
			core.SeedOnboardingBlockingStepCount, item.StepOrder, item.Slug)
	}
}
