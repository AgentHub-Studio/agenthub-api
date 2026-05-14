package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD tests for ContextWindowAccessTierRegistry — §7.1 / Figure 6.
// All test functions are prefixed FEAT038_BDD.

// Scenario: Figure 6 defines exactly six access tiers arranged by increasing mutability.
func TestFEAT038_BDD_SixTiersInMutabilityOrder(t *testing.T) {
	// Given a fresh registry
	r := NewContextWindowAccessTierRegistry()

	// When I request all tiers
	tiers := r.AllTiers()

	// Then there are exactly six tiers
	require.Len(t, tiers, SeedContextWindowAccessTierCount)

	// And their MutabilityRanks form the contiguous sequence 1..6
	for i, tier := range tiers {
		assert.Equal(t, i+1, tier.MutabilityRank,
			"tier at index %d should have MutabilityRank %d, got %d", i, i+1, tier.MutabilityRank)
	}

	// And read_only is at position 0, lazy_load at position 5
	assert.Equal(t, ContextAccessTierReadOnly, tiers[0].TierID)
	assert.Equal(t, ContextAccessTierLazyLoad, tiers[5].TierID)
}

// Scenario: The read_only tier represents session-static content that never changes.
func TestFEAT038_BDD_ReadOnlyTierCharacteristics(t *testing.T) {
	// Given a registry
	r := NewContextWindowAccessTierRegistry()

	// When I look up the read_only tier
	p, ok := r.FindContextWindowAccessTierBySlug("read_only")

	// Then it exists and has the expected properties
	require.True(t, ok)
	require.NotNil(t, p)
	assert.Equal(t, 1, p.MutabilityRank)
	assert.False(t, p.WrittenBySystem)
	assert.False(t, p.WrittenByModel)
	assert.False(t, p.IsLazyResolved)
	assert.False(t, p.IsAppendOnly)

	// And it is reported as NOT mutable at runtime
	assert.False(t, r.IsMutableAtRuntime(ContextAccessTierReadOnly))

	// And there are five tiers MORE mutable than it
	moreMutable := r.TiersMoreMutableThan(ContextAccessTierReadOnly)
	assert.Len(t, moreMutable, 5)
}

// Scenario: The lazy_load tier represents on-demand deferred content that the model pulls.
func TestFEAT038_BDD_LazyLoadTierCharacteristics(t *testing.T) {
	// Given a registry
	r := NewContextWindowAccessTierRegistry()

	// When I retrieve the lazy_load tier
	p, ok := r.FindContextWindowAccessTierBySlug("lazy_load")

	// Then it is the most mutable tier
	require.True(t, ok)
	assert.Equal(t, 6, p.MutabilityRank)

	// And it is marked IsLazyResolved because it is not present at assembly time
	assert.True(t, p.IsLazyResolved)

	// And it is triggered by the model
	assert.True(t, p.WrittenByModel)

	// And it is the only lazy-resolved tier
	lazyTiers := r.LazyResolvedTiers()
	require.Len(t, lazyTiers, 1)
	assert.Equal(t, ContextAccessTierLazyLoad, lazyTiers[0].TierID)

	// And there are NO tiers more mutable than it
	none := r.TiersMoreMutableThan(ContextAccessTierLazyLoad)
	assert.Len(t, none, 0)
}

// Scenario: The sys_write tier is the only tier written by system subsystems.
func TestFEAT038_BDD_SysWriteTierIsOnlySystemWritten(t *testing.T) {
	// Given a registry
	r := NewContextWindowAccessTierRegistry()

	// When I query tiers written by system subsystems
	sysTiers := r.TiersWrittenBySystem()

	// Then only one tier matches
	require.Len(t, sysTiers, 1)
	assert.Equal(t, ContextAccessTierSysWrite, sysTiers[0].TierID)

	// And the sys_write tier is mutable at runtime
	assert.True(t, r.IsMutableAtRuntime(ContextAccessTierSysWrite))

	// And its example sources include auto_memory and compact_summaries
	p := sysTiers[0]
	assert.Contains(t, p.ExampleSources, "auto_memory")
	assert.Contains(t, p.ExampleSources, "compact_summaries")
}

// Scenario: The append and model_trigger tiers model append-only growth patterns.
func TestFEAT038_BDD_AppendOnlyTiersModelMonotonicGrowth(t *testing.T) {
	// Given a registry
	r := NewContextWindowAccessTierRegistry()

	// When I query append-only tiers
	appendTiers := r.AppendOnlyTiers()

	// Then exactly two tiers are append-only: append and model_trigger
	require.Len(t, appendTiers, 2)
	ids := make(map[ContextWindowAccessTierID]bool)
	for _, t2 := range appendTiers {
		ids[t2.TierID] = true
	}
	assert.True(t, ids[ContextAccessTierAppend])
	assert.True(t, ids[ContextAccessTierModelTrigger])

	// And neither is lazy-resolved (they are eagerly appended during the turn)
	for _, tier := range appendTiers {
		assert.False(t, tier.IsLazyResolved, "append-only tier %s should not be lazy-resolved", tier.TierID)
	}
}

// Scenario: MutabilityRank boundary correctly splits tiers into immutable and mutable groups.
func TestFEAT038_BDD_MutabilityBoundaryAtRankThree(t *testing.T) {
	// Given a registry
	r := NewContextWindowAccessTierRegistry()

	// When I classify all six tiers by IsMutableAtRuntime
	immutableTiers := 0
	mutableTiers := 0
	for _, tier := range r.AllTiers() {
		if r.IsMutableAtRuntime(tier.TierID) {
			mutableTiers++
		} else {
			immutableTiers++
		}
	}

	// Then ranks 1 and 2 (read_only, hot_reload) are NOT mutable at runtime
	assert.Equal(t, 2, immutableTiers)

	// And ranks 3–6 (sys_write, append, model_trigger, lazy_load) ARE mutable
	assert.Equal(t, 4, mutableTiers)
}

// Scenario: Structural invariants encoded in the registry seed data all hold.
func TestFEAT038_BDD_AllStructuralInvariantsHold(t *testing.T) {
	// All five invariant functions defined on the registry must pass.
	assert.True(t, ContextWindowAccessTierMutabilityRanksAreContiguous(),
		"invariant: ranks 1..6 contiguous")
	assert.True(t, ContextWindowAccessTierOrderMatchesRank(),
		"invariant: slice is sorted by ascending rank")
	assert.True(t, ContextWindowAccessTierExactlyOneLazyResolved(),
		"invariant: exactly one tier is lazy-resolved")
	assert.True(t, ContextWindowAccessTierLazyLoadIsHighestRank(),
		"invariant: lazy_load has rank 6")
	assert.True(t, ContextWindowAccessTierReadOnlyIsLowestRank(),
		"invariant: read_only has rank 1")
}

// Scenario: TiersMoreMutableThan and TiersLessMutableThan form complementary slices
// that together with the tier itself cover all six tiers.
func TestFEAT038_BDD_TiersMoreAndLessArePerfectComplement(t *testing.T) {
	// Given a registry and an arbitrary middle tier (append, rank 4)
	r := NewContextWindowAccessTierRegistry()
	id := ContextAccessTierAppend

	// When I fetch tiers on both sides
	more := r.TiersMoreMutableThan(id)
	less := r.TiersLessMutableThan(id)

	// Then their combined count plus one (the tier itself) equals six
	assert.Equal(t, SeedContextWindowAccessTierCount, len(more)+len(less)+1)

	// And no tier appears in both slices
	moreSet := make(map[ContextWindowAccessTierID]bool)
	for _, t2 := range more {
		moreSet[t2.TierID] = true
	}
	for _, t2 := range less {
		assert.False(t, moreSet[t2.TierID],
			"tier %s should not appear in both more and less", t2.TierID)
	}
}
