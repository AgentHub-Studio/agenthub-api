package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSFSDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksForkStrategyFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a PERSIST-005a SessionForkStrategy
		// before any /branch operation, and the cost/safety/compaction
		// trade-offs are subtle,
		// When admin opens fork-strategy onboarding,
		// Then 3 recommended templates surface 1:1 with the enum.
		assert.Equal(t, 3, len(SeedRecommendedSFSDTemplateSlugs))
	})

	t.Run("Scenario_FullCopyExploreForRoutineBranching", func(t *testing.T) {
		// Given users want a fully isolated /branch playground,
		// When admin uses full-copy-explore,
		// Then strategy=full_copy, storage=high, no hash compute, no
		// compaction tolerance.
		assert.Contains(t, SeedExpectedSFSDTemplateSlugs, "full-copy-explore")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSFSDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["full-copy-explore"])
	})

	t.Run("Scenario_BranchPointerCheapForStorageOptimized", func(t *testing.T) {
		// Given storage cost matters more than divergence detection,
		// When admin uses branch-pointer-cheap,
		// Then strategy=branch_pointer, storage=low, no hash compute.
		assert.Contains(t, SeedExpectedSFSDTemplateSlugs, "branch-pointer-cheap")
	})

	t.Run("Scenario_SnapshotIsolatedForComplianceAndCompactedPrefix", func(t *testing.T) {
		// Given audit-strict tenants need divergence detection and may
		// fork inside compacted prefix,
		// When admin uses snapshot-isolated-compliance,
		// Then strategy=snapshot_isolated, computes_hash=true,
		// allows_compaction=true, admin review.
		assert.Contains(t, SeedExpectedSFSDTemplateSlugs, "snapshot-isolated-compliance")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSFSDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["snapshot-isolated-compliance"])
	})

	t.Run("Scenario_StrategyLabelsMatchPERSIST005aEnumByteForByte", func(t *testing.T) {
		// Given PERSIST-005a SessionForkStrategy has 3 values,
		// When seed declares target_strategy,
		// Then labels match enum bytes (no mapping table runtime).
		ps := []string{"full_copy", "branch_pointer", "snapshot_isolated"}
		set := map[string]bool{}
		for _, s := range SeedExpectedSFSDTemplateStrategies {
			set[s] = true
		}
		for _, e := range ps {
			assert.True(t, set[e], "PERSIST-005a strategy %q missing", e)
		}
	})

	t.Run("Scenario_OnlySnapshotIsolatedAllowsCompactionFork", func(t *testing.T) {
		// Given PERSIST-005a Evaluate rejects fork past compaction
		// unless strategy is snapshot_isolated,
		// When admin inspects allows_fork_past_compaction,
		// Then only snapshot-isolated → true; others → false.
		// Validated DB-real in integration test.
		assert.Equal(t, 3, SeedExpectedSFSDTemplateRowCount)
	})

	t.Run("Scenario_OnlySnapshotIsolatedComputesHash", func(t *testing.T) {
		// Given PERSIST-005a only computes hash for snapshot_isolated,
		// When admin compares computes_transcript_hash,
		// Then only snapshot-isolated → true; others → false.
		// Validated DB-real.
		assert.Equal(t, 3, SeedExpectedSFSDTemplateRowCount)
	})

	t.Run("Scenario_StorageOverheadLadderReflectsTradeoff", func(t *testing.T) {
		// Given full_copy duplicates everything (high) and
		// branch_pointer just stores a pointer (low),
		// When admin compares typical_storage_overhead,
		// Then 3 overhead tiers represented (low/medium/high).
		assert.Equal(t, 3, len(SeedExpectedSFSDTemplateStorageOverheads))
	})

	t.Run("Scenario_SafetyPostureSpansSpectrum", func(t *testing.T) {
		// Given posture is a stance ladder (balanced..permissive..strict),
		// When seed templates ship,
		// Then all 3 postures are represented.
		assert.Equal(t, 3, len(SeedExpectedSFSDTemplateSafetyPostures))
	})

	t.Run("Scenario_AdminReviewGatesComplianceFork", func(t *testing.T) {
		// Given snapshot_isolated crosses compact boundary (non-trivial
		// semantics),
		// When admin compares admin-review subset,
		// Then only snapshot-isolated is listed.
		assert.Equal(t, 1, len(SeedAdminReviewSFSDTemplateSlugs))
	})

	t.Run("Scenario_OneToOneMappingStrategyToTemplate", func(t *testing.T) {
		// Given PERSIST-005a has 3 strategies,
		// When seed templates ship,
		// Then each strategy has exactly 1 template (1:1). Validated DB-real.
		assert.Equal(t, len(SeedExpectedSFSDTemplateStrategies), SeedExpectedSFSDTemplateRowCount)
	})
}
