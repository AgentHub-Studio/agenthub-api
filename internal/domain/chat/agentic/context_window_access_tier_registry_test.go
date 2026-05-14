package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for ContextWindowAccessTierRegistry — §7.1 / Figure 6.
// All tests are prefixed FEAT038.

func TestFEAT038_NewRegistryIsNotNil(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	require.NotNil(t, r)
}

func TestFEAT038_CountIsSix(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	assert.Equal(t, 6, r.Count())
	assert.Equal(t, SeedContextWindowAccessTierCount, r.Count())
}

func TestFEAT038_AllTiersReturnsDefensiveCopy(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	a := r.AllTiers()
	b := r.AllTiers()
	require.Equal(t, 6, len(a))
	// Verify it is a copy: mutating the slice should not affect subsequent calls.
	a[0] = nil
	assert.NotNil(t, b[0])
}

func TestFEAT038_AllTiersAreInMutabilityAscendingOrder(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.AllTiers()
	for i := 1; i < len(tiers); i++ {
		assert.Greater(t, tiers[i].MutabilityRank, tiers[i-1].MutabilityRank,
			"tier at index %d should have higher MutabilityRank than index %d", i, i-1)
	}
}

func TestFEAT038_FindBySlugKnownTiers(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()

	knownSlugs := []string{
		"read_only",
		"hot_reload",
		"sys_write",
		"append",
		"model_trigger",
		"lazy_load",
	}
	for _, slug := range knownSlugs {
		p, ok := r.FindContextWindowAccessTierBySlug(slug)
		assert.True(t, ok, "expected to find tier: %s", slug)
		assert.NotNil(t, p)
		assert.Equal(t, ContextWindowAccessTierID(slug), p.TierID)
	}
}

func TestFEAT038_FindBySlugUnknownReturnsNilFalse(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	p, ok := r.FindContextWindowAccessTierBySlug("nonexistent_tier")
	assert.False(t, ok)
	assert.Nil(t, p)
}

func TestFEAT038_TierByMutabilityRankAllSixValid(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	for rank := 1; rank <= 6; rank++ {
		p, ok := r.TierByMutabilityRank(rank)
		assert.True(t, ok, "expected rank %d to be found", rank)
		assert.Equal(t, rank, p.MutabilityRank)
	}
}

func TestFEAT038_TierByMutabilityRankOutOfRangeReturnsFalse(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	_, ok0 := r.TierByMutabilityRank(0)
	_, ok7 := r.TierByMutabilityRank(7)
	assert.False(t, ok0)
	assert.False(t, ok7)
}

func TestFEAT038_LeastMutableTierIsReadOnly(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tier := r.LeastMutableTier()
	require.NotNil(t, tier)
	assert.Equal(t, ContextAccessTierReadOnly, tier.TierID)
	assert.Equal(t, 1, tier.MutabilityRank)
}

func TestFEAT038_MostMutableTierIsLazyLoad(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tier := r.MostMutableTier()
	require.NotNil(t, tier)
	assert.Equal(t, ContextAccessTierLazyLoad, tier.TierID)
	assert.Equal(t, 6, tier.MutabilityRank)
}

func TestFEAT038_TiersWrittenBySystemIsSysWriteOnly(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.TiersWrittenBySystem()
	require.Len(t, tiers, 1)
	assert.Equal(t, ContextAccessTierSysWrite, tiers[0].TierID)
}

func TestFEAT038_TiersWrittenByModelAreModelTriggerAndLazyLoad(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.TiersWrittenByModel()
	require.Len(t, tiers, 2)
	ids := []ContextWindowAccessTierID{tiers[0].TierID, tiers[1].TierID}
	assert.Contains(t, ids, ContextAccessTierModelTrigger)
	assert.Contains(t, ids, ContextAccessTierLazyLoad)
}

func TestFEAT038_AppendOnlyTiersAreAppendAndModelTrigger(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.AppendOnlyTiers()
	require.Len(t, tiers, 2)
	ids := []ContextWindowAccessTierID{tiers[0].TierID, tiers[1].TierID}
	assert.Contains(t, ids, ContextAccessTierAppend)
	assert.Contains(t, ids, ContextAccessTierModelTrigger)
}

func TestFEAT038_LazyResolvedTiersIsLazyLoadOnly(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.LazyResolvedTiers()
	require.Len(t, tiers, 1)
	assert.Equal(t, ContextAccessTierLazyLoad, tiers[0].TierID)
}

func TestFEAT038_IsMutableAtRuntimeBoundary(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()

	// Ranks 1 and 2 are NOT mutable at runtime.
	assert.False(t, r.IsMutableAtRuntime(ContextAccessTierReadOnly))
	assert.False(t, r.IsMutableAtRuntime(ContextAccessTierHotReload))

	// Ranks 3–6 ARE mutable at runtime.
	assert.True(t, r.IsMutableAtRuntime(ContextAccessTierSysWrite))
	assert.True(t, r.IsMutableAtRuntime(ContextAccessTierAppend))
	assert.True(t, r.IsMutableAtRuntime(ContextAccessTierModelTrigger))
	assert.True(t, r.IsMutableAtRuntime(ContextAccessTierLazyLoad))
}

func TestFEAT038_IsMutableAtRuntimeUnknownIDReturnsFalse(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	assert.False(t, r.IsMutableAtRuntime("ghost_tier"))
}

func TestFEAT038_TiersMoreMutableThanReadOnly(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.TiersMoreMutableThan(ContextAccessTierReadOnly)
	// read_only is rank 1 → all five remaining tiers should be returned.
	assert.Len(t, tiers, 5)
	for _, t2 := range tiers {
		assert.Greater(t, t2.MutabilityRank, 1)
	}
}

func TestFEAT038_TiersLessMutableThanLazyLoad(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	tiers := r.TiersLessMutableThan(ContextAccessTierLazyLoad)
	// lazy_load is rank 6 → all five lower tiers should be returned.
	assert.Len(t, tiers, 5)
	for _, t2 := range tiers {
		assert.Less(t, t2.MutabilityRank, 6)
	}
}

func TestFEAT038_TiersMoreMutableThanUnknownIDReturnsNil(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	result := r.TiersMoreMutableThan("phantom")
	assert.Nil(t, result)
}

func TestFEAT038_TiersLessMutableThanUnknownIDReturnsNil(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	result := r.TiersLessMutableThan("phantom")
	assert.Nil(t, result)
}

func TestFEAT038_IsValidAccessTierIDAllSix(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	for _, id := range SeedContextWindowAccessTierIDs {
		assert.True(t, r.IsValidAccessTierID(string(id)), "expected valid: %s", id)
	}
}

func TestFEAT038_IsValidAccessTierIDUnknownReturnsFalse(t *testing.T) {
	r := NewContextWindowAccessTierRegistry()
	assert.False(t, r.IsValidAccessTierID("write_through"))
}

// --- Structural invariant tests ---

func TestFEAT038_InvariantMutabilityRanksAreContiguous(t *testing.T) {
	assert.True(t, ContextWindowAccessTierMutabilityRanksAreContiguous(),
		"MutabilityRanks should be exactly 1..6 with no gaps or duplicates")
}

func TestFEAT038_InvariantOrderMatchesRank(t *testing.T) {
	assert.True(t, ContextWindowAccessTierOrderMatchesRank(),
		"contextWindowAccessTierProfiles should be sorted in ascending MutabilityRank order")
}

func TestFEAT038_InvariantExactlyOneLazyResolved(t *testing.T) {
	assert.True(t, ContextWindowAccessTierExactlyOneLazyResolved(),
		"exactly one tier should have IsLazyResolved = true (lazy_load)")
}

func TestFEAT038_InvariantLazyLoadIsHighestRank(t *testing.T) {
	assert.True(t, ContextWindowAccessTierLazyLoadIsHighestRank(),
		"lazy_load should be rank 6 (highest mutability)")
}

func TestFEAT038_InvariantReadOnlyIsLowestRank(t *testing.T) {
	assert.True(t, ContextWindowAccessTierReadOnlyIsLowestRank(),
		"read_only should be rank 1 (lowest mutability)")
}
