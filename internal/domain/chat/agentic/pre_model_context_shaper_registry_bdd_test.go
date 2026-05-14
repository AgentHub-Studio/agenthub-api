package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for PreModelContextShaperRegistry — §4.3 arXiv:2604.14228v1.
// All test functions are prefixed FEAT033.
//
// §4.3: "Five context shapers execute sequentially in query.ts before every
// model call, each operating on the messagesForQuery array. The five shapers
// run in sequence, with earlier steps applying lighter reductions before later
// steps apply broader compaction."

// TestFEAT033_BDD_ShaperSequenceEnforcesLighterBeforeHeavier verifies that
// the execution order mirrors the paper's "lighter before heavier" principle:
// budget reduction (in-place) → snip (trimming) → microcompact (in-place) →
// context-collapse (projection) → auto-compact (replacement).
func TestFEAT033_BDD_ShaperSequenceEnforcesLighterBeforeHeavier(t *testing.T) {
	// Arrange: retrieve all shapers in sorted execution order.
	shapers := AllPreModelContextShapers()

	// Assert sequence is 1..5 with ascending aggressiveness implied by
	// mutation style order: in_place < trimming < in_place < projection < replacement.
	t.Run("Scenario_BudgetReductionIsLightest_Order1", func(t *testing.T) {
		require.Equal(t, ShaperBudgetReduction, shapers[0].ID,
			"§4.3: budget reduction is the lightest shaper and must run first")
		assert.Equal(t, ShaperMutationInPlace, shapers[0].MutationStyle,
			"budget reduction replaces content in-place without removing messages")
	})

	t.Run("Scenario_AutoCompactIsHeaviest_Order5", func(t *testing.T) {
		require.Equal(t, ShaperAutoCompact, shapers[4].ID,
			"§4.3: auto-compact applies broadest compaction and must run last")
		assert.Equal(t, ShaperMutationReplacement, shapers[4].MutationStyle,
			"auto-compact replaces the full history with a model-generated summary")
	})

	t.Run("Scenario_ProjectionShaper_RunsBeforeReplacement", func(t *testing.T) {
		// Context-collapse (projection, order 4) must run before auto-compact
		// (replacement, order 5) — both modify the view the model receives.
		earlier, ok := EarlierShapersMustRunFirst(ShaperContextCollapse, ShaperAutoCompact)
		require.True(t, ok)
		assert.True(t, earlier,
			"§4.3: projection (context-collapse) must precede replacement (auto-compact)")
	})
}

// TestFEAT033_BDD_SnipTokensFreedComposabilityContract verifies the critical
// cross-shaper composability rule described in §4.3: snip's token savings are
// invisible to the main token counter and must be forwarded to auto-compact.
func TestFEAT033_BDD_SnipTokensFreedComposabilityContract(t *testing.T) {
	t.Run("Scenario_SnipReturnsTokensFreedSignal", func(t *testing.T) {
		// Given the snip shaper profile,
		snip, ok := FindPreModelContextShaper(ShaperSnip)
		require.True(t, ok)

		// When we check whether it returns a tokensFreed signal,
		// Then it must — per §4.3 the snipTokensFreed value is plumbed to
		// auto-compact explicitly.
		assert.True(t, snip.ReturnsTokensFreed,
			"§4.3: snipTokensFreed must be plumbed to auto-compact because the main token counter is blind to snip savings")
	})

	t.Run("Scenario_OnlySnipNeedsExplicitForwarding", func(t *testing.T) {
		// Given all five shapers,
		producers := ShapersThatReturnTokensFreed()

		// When we count how many return an explicit tokensFreed signal,
		// Then exactly one does — §4.3 singles out snip for this contract.
		assert.Len(t, producers, 1,
			"§4.3: only snip's token savings are invisible to the counter; others' reductions are directly observable")
	})

	t.Run("Scenario_AutoCompactConsumesSnipSignal_NotOtherShapers", func(t *testing.T) {
		// The consumer (auto-compact) does not itself produce a tokensFreed signal.
		autoCompact, ok := FindPreModelContextShaper(ShaperAutoCompact)
		require.True(t, ok)
		assert.False(t, autoCompact.ReturnsTokensFreed,
			"auto-compact is the terminal consumer of tokensFreed, not a producer")
		assert.Equal(t, ShaperActivationDefaultOn, autoCompact.ActivationMode,
			"auto-compact is enabled by default so it is always a potential consumer")
	})
}

// TestFEAT033_BDD_ContextCollapsePreservesHistory verifies §4.3's key property
// that context-collapse does NOT mutate the REPL's stored history — it only
// changes what the model sees via a read-time projection.
func TestFEAT033_BDD_ContextCollapsePreservesHistory(t *testing.T) {
	t.Run("Scenario_CollapseMutationStyleIsProjection", func(t *testing.T) {
		// Given the context-collapse shaper profile,
		collapse, ok := FindPreModelContextShaper(ShaperContextCollapse)
		require.True(t, ok)

		// When we check its mutation style,
		// Then it must be projection — §4.3 says "Nothing is yielded; the
		// collapsed view is a read-time projection over the REPL's full history."
		assert.Equal(t, ShaperMutationProjection, collapse.MutationStyle,
			"§4.3: context-collapse uses a read-time projection, not in-place mutation")
	})

	t.Run("Scenario_CollapseEffectsPersistAcrossTurns", func(t *testing.T) {
		// Given the context-collapse shaper profile,
		collapse, ok := FindPreModelContextShaper(ShaperContextCollapse)
		require.True(t, ok)

		// When we check whether it persists across turns,
		// Then it must — §4.3: "This is what makes collapses persist across turns."
		assert.True(t, collapse.PersistsAcrossTurns,
			"§4.3: collapse store is separate from REPL array; projections persist across loop iterations")
	})

	t.Run("Scenario_CollapseGatedByFeatureFlag", func(t *testing.T) {
		collapse, ok := FindPreModelContextShaper(ShaperContextCollapse)
		require.True(t, ok)
		assert.Equal(t, ShaperFlagContextCollapse, collapse.FeatureFlag,
			"§4.3: context-collapse is gated by CONTEXT_COLLAPSE feature flag")
	})
}

// TestFEAT033_BDD_BudgetReductionComposesCleanlyWithMicrocompact verifies §4.3's
// explicit composability note: budget reduction runs before microcompact because
// microcompact operates by tool_use_id without inspecting content.
func TestFEAT033_BDD_BudgetReductionComposesCleanlyWithMicrocompact(t *testing.T) {
	t.Run("Scenario_BudgetReductionRunsBeforeMicrocompact", func(t *testing.T) {
		earlier, ok := EarlierShapersMustRunFirst(ShaperBudgetReduction, ShaperMicrocompact)
		require.True(t, ok)
		assert.True(t, earlier,
			"§4.3: budget reduction (1) must precede microcompact (3) — microcompact operates by tool_use_id and never inspects content")
	})

	t.Run("Scenario_MicrocompactHasNoFeatureFlagForAlwaysPath", func(t *testing.T) {
		// Microcompact always runs a time-based path; only the cache-aware path
		// is gated by CACHED_MICROCOMPACT.
		micro, ok := FindPreModelContextShaper(ShaperMicrocompact)
		require.True(t, ok)
		assert.Equal(t, ShaperFlagCachedMicrocompact, micro.FeatureFlag,
			"§4.3: the feature flag gates the cache-aware path of microcompact")
	})

	t.Run("Scenario_BudgetReductionIsAlwaysActive", func(t *testing.T) {
		budget, ok := FindPreModelContextShaper(ShaperBudgetReduction)
		require.True(t, ok)
		assert.Equal(t, ShaperActivationAlways, budget.ActivationMode,
			"§4.3: budget reduction has no feature flag — it is always active")
		assert.False(t, budget.UserCanDisable,
			"budget reduction cannot be disabled; it is a safety-critical size limit")
	})
}

// TestFEAT033_BDD_AutoCompactFiresLastAndPressureConditional verifies §4.3:
// "Auto-compact fires only when the context still exceeds the pressure threshold
// after all four previous shapers have run."
func TestFEAT033_BDD_AutoCompactFiresLastAndPressureConditional(t *testing.T) {
	t.Run("Scenario_AutoCompactIsLastInSequence", func(t *testing.T) {
		all := AllPreModelContextShapers()
		last := all[len(all)-1]
		assert.Equal(t, ShaperAutoCompact, last.ID,
			"§4.3: auto-compact fires after all four lighter shapers have run")
		assert.Equal(t, 5, last.ExecutionOrder)
	})

	t.Run("Scenario_AutoCompactIsOnlyPressureConditionalShaper", func(t *testing.T) {
		pressureShapers := ShaperPressureConditional()
		require.Len(t, pressureShapers, 1,
			"§4.3: exactly one shaper is conditional on context pressure")
		assert.Equal(t, ShaperAutoCompact, pressureShapers[0].ID)
	})

	t.Run("Scenario_AutoCompactUserCanDisable", func(t *testing.T) {
		autoCompact, ok := FindPreModelContextShaper(ShaperAutoCompact)
		require.True(t, ok)
		assert.True(t, autoCompact.UserCanDisable,
			"§4.3: the user may disable auto-compact via configuration")
	})
}

// TestFEAT033_BDD_RegistryIntegrityAndQueryability verifies the registry is
// complete, consistent, and fully queryable via its public API.
func TestFEAT033_BDD_RegistryIntegrityAndQueryability(t *testing.T) {
	t.Run("Scenario_AllFiveIDsAreRetrievable", func(t *testing.T) {
		ids := []PreModelContextShaperID{
			ShaperBudgetReduction, ShaperSnip, ShaperMicrocompact,
			ShaperContextCollapse, ShaperAutoCompact,
		}
		for _, id := range ids {
			_, ok := FindPreModelContextShaper(id)
			assert.True(t, ok, "shaper %q must be retrievable from the registry", id)
		}
	})

	t.Run("Scenario_ExecutionOrderIsUnique", func(t *testing.T) {
		seen := make(map[int]PreModelContextShaperID)
		for _, p := range AllPreModelContextShapers() {
			existing, conflict := seen[p.ExecutionOrder]
			assert.False(t, conflict,
				"ExecutionOrder %d is assigned to both %q and %q — must be unique",
				p.ExecutionOrder, existing, p.ID)
			seen[p.ExecutionOrder] = p.ID
		}
		assert.Len(t, seen, SeedPreModelContextShaperCount)
	})

	t.Run("Scenario_SeedConstantMatchesActualRegistrySize", func(t *testing.T) {
		assert.Equal(t, SeedPreModelContextShaperCount, len(AllPreModelContextShapers()),
			"SeedPreModelContextShaperCount must always match the live registry size")
	})
}
