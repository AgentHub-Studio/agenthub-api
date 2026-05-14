package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for PreModelContextShaperRegistry — §4.3 of arXiv:2604.14228v1.
// All test functions are prefixed FEAT033.

// ---------------------------------------------------------------------------
// FEAT033 — Shaper count and seed constant
// ---------------------------------------------------------------------------

func TestFEAT033_SeedCount_IsFive(t *testing.T) {
	assert.Equal(t, 5, SeedPreModelContextShaperCount,
		"§4.3 specifies exactly five pre-model context shapers")
}

func TestFEAT033_AllProfiles_ReturnsFive(t *testing.T) {
	profiles := AllPreModelContextShapers()
	assert.Len(t, profiles, SeedPreModelContextShaperCount,
		"AllPreModelContextShapers must return exactly 5 profiles (§4.3)")
}

// ---------------------------------------------------------------------------
// FEAT033 — Execution order is sequential 1→5
// ---------------------------------------------------------------------------

func TestFEAT033_ExecutionOrder_IsStrictlySequential(t *testing.T) {
	profiles := AllPreModelContextShapers()
	for i, p := range profiles {
		wantOrder := i + 1
		assert.Equal(t, wantOrder, p.ExecutionOrder,
			"shaper at index %d must have ExecutionOrder=%d (§4.3 sequential run order)", i, wantOrder)
	}
}

func TestFEAT033_ExecutionOrder_BudgetReductionIsFirst(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperBudgetReduction)
	require.True(t, ok)
	assert.Equal(t, 1, p.ExecutionOrder,
		"§4.3: budget reduction applies lighter reductions first")
}

func TestFEAT033_ExecutionOrder_AutoCompactIsLast(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperAutoCompact)
	require.True(t, ok)
	assert.Equal(t, 5, p.ExecutionOrder,
		"§4.3: auto-compact fires only when context still exceeds pressure threshold after all four previous shapers")
}

func TestFEAT033_ExecutionOrder_SnipBeforeMicrocompact(t *testing.T) {
	earlier, ok := EarlierShapersMustRunFirst(ShaperSnip, ShaperMicrocompact)
	require.True(t, ok)
	assert.True(t, earlier,
		"§4.3 implies snip (2) must precede microcompact (3)")
}

func TestFEAT033_ExecutionOrder_MicrocompactBeforeContextCollapse(t *testing.T) {
	earlier, ok := EarlierShapersMustRunFirst(ShaperMicrocompact, ShaperContextCollapse)
	require.True(t, ok)
	assert.True(t, earlier,
		"§4.3 implies microcompact (3) must precede context collapse (4)")
}

// ---------------------------------------------------------------------------
// FEAT033 — Feature flags
// ---------------------------------------------------------------------------

func TestFEAT033_FeatureFlag_BudgetReductionHasNoFlag(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperBudgetReduction)
	require.True(t, ok)
	assert.Equal(t, ShaperFlagNone, p.FeatureFlag,
		"§4.3: budget reduction is always active — no feature flag")
	assert.Equal(t, ShaperActivationAlways, p.ActivationMode)
}

func TestFEAT033_FeatureFlag_SnipGatedByHistorySnip(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperSnip)
	require.True(t, ok)
	assert.Equal(t, ShaperFlagHistorySnip, p.FeatureFlag,
		"§4.3: snip is gated by HISTORY_SNIP feature flag")
	assert.Equal(t, ShaperActivationFeatureFlag, p.ActivationMode)
}

func TestFEAT033_FeatureFlag_MicrocompactGatedByCachedMicrocompact(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperMicrocompact)
	require.True(t, ok)
	assert.Equal(t, ShaperFlagCachedMicrocompact, p.FeatureFlag,
		"§4.3: microcompact is gated by CACHED_MICROCOMPACT feature flag")
}

func TestFEAT033_FeatureFlag_ContextCollapseGatedByContextCollapse(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperContextCollapse)
	require.True(t, ok)
	assert.Equal(t, ShaperFlagContextCollapse, p.FeatureFlag,
		"§4.3: context collapse is gated by CONTEXT_COLLAPSE feature flag")
}

func TestFEAT033_FeatureFlag_ExactlyThreeShapersFlagGated(t *testing.T) {
	flagGated := ShapersByActivationMode(ShaperActivationFeatureFlag)
	assert.Len(t, flagGated, 3,
		"§4.3: exactly three shapers are gated by feature flags (snip, microcompact, context-collapse)")
}

func TestFEAT033_FeatureFlag_AutoCompactIsDefaultOn(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperAutoCompact)
	require.True(t, ok)
	assert.Equal(t, ShaperActivationDefaultOn, p.ActivationMode,
		"§4.3: auto-compact is enabled by default (user may disable via configuration)")
	assert.True(t, p.UserCanDisable, "§4.3: the user may disable auto-compact")
}

// ---------------------------------------------------------------------------
// FEAT033 — tokensFreed signal (composability between snip and auto-compact)
// ---------------------------------------------------------------------------

func TestFEAT033_TokensFreed_OnlySnipReturnsSignal(t *testing.T) {
	shapers := ShapersThatReturnTokensFreed()
	require.Len(t, shapers, 1,
		"§4.3: only snip returns a tokensFreed signal")
	assert.Equal(t, ShaperSnip, shapers[0].ID)
}

func TestFEAT033_TokensFreed_BudgetReductionDoesNot(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperBudgetReduction)
	require.True(t, ok)
	assert.False(t, p.ReturnsTokensFreed,
		"budget reduction does not return an explicit tokensFreed signal")
}

func TestFEAT033_TokensFreed_AutoCompactDoesNot(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperAutoCompact)
	require.True(t, ok)
	assert.False(t, p.ReturnsTokensFreed,
		"auto-compact is the consumer of tokensFreed, not a producer")
}

// ---------------------------------------------------------------------------
// FEAT033 — Mutation styles
// ---------------------------------------------------------------------------

func TestFEAT033_MutationStyle_ContextCollapseIsProjection(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperContextCollapse)
	require.True(t, ok)
	assert.Equal(t, ShaperMutationProjection, p.MutationStyle,
		"§4.3: context collapse is a read-time projection — it does not mutate the REPL's stored history")
}

func TestFEAT033_MutationStyle_AutoCompactIsReplacement(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperAutoCompact)
	require.True(t, ok)
	assert.Equal(t, ShaperMutationReplacement, p.MutationStyle,
		"§4.3: auto-compact replaces the full history with a model-generated summary")
}

func TestFEAT033_MutationStyle_SnipIsTrimming(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperSnip)
	require.True(t, ok)
	assert.Equal(t, ShaperMutationTrimming, p.MutationStyle,
		"§4.3: snip removes older history segments")
}

// ---------------------------------------------------------------------------
// FEAT033 — Persistence across turns
// ---------------------------------------------------------------------------

func TestFEAT033_Persistence_ContextCollapsePersists(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperContextCollapse)
	require.True(t, ok)
	assert.True(t, p.PersistsAcrossTurns,
		"§4.3: 'This is what makes collapses persist across turns' — collapse store survives loop iterations")
}

func TestFEAT033_Persistence_SnipDoesNotPersist(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperSnip)
	require.True(t, ok)
	assert.False(t, p.PersistsAcrossTurns,
		"snip is re-applied each turn as needed; it has no durable store")
}

func TestFEAT033_Persistence_ExactlyThreeShapersPersist(t *testing.T) {
	// budget_reduction (persists content replacements), context_collapse, auto_compact
	persistent := ShapersThatPersistAcrossTurns()
	assert.Len(t, persistent, 3,
		"§4.3 implies three shapers have durable effects: budget_reduction, context_collapse, auto_compact")
}

// ---------------------------------------------------------------------------
// FEAT033 — Pressure-conditional (auto-compact only)
// ---------------------------------------------------------------------------

func TestFEAT033_PressureConditional_OnlyAutoCompact(t *testing.T) {
	shapers := ShaperPressureConditional()
	require.Len(t, shapers, 1,
		"§4.3: exactly one shaper is pressure-conditional (auto-compact)")
	assert.Equal(t, ShaperAutoCompact, shapers[0].ID)
}

// ---------------------------------------------------------------------------
// FEAT033 — Source functions match §4.3 text
// ---------------------------------------------------------------------------

func TestFEAT033_SourceFunction_AllNonEmpty(t *testing.T) {
	for _, p := range AllPreModelContextShapers() {
		assert.NotEmpty(t, p.SourceFunction,
			"shaper %q must have a non-empty SourceFunction citing its query.ts entry point", p.ID)
	}
}

func TestFEAT033_SourceFunction_BudgetReduction(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperBudgetReduction)
	require.True(t, ok)
	assert.Equal(t, "applyToolResultBudget()", p.SourceFunction,
		"§4.3 names applyToolResultBudget() as the budget reduction entry point")
}

func TestFEAT033_SourceFunction_AutoCompact(t *testing.T) {
	p, ok := FindPreModelContextShaper(ShaperAutoCompact)
	require.True(t, ok)
	assert.Equal(t, "compactConversation()", p.SourceFunction,
		"§4.3 names compactConversation() in compact.ts as the auto-compact entry point")
}

// ---------------------------------------------------------------------------
// FEAT033 — FindPreModelContextShaper returns false for unknown ID
// ---------------------------------------------------------------------------

func TestFEAT033_Find_UnknownIDReturnsFalse(t *testing.T) {
	_, ok := FindPreModelContextShaper("nonexistent_shaper")
	assert.False(t, ok,
		"FindPreModelContextShaper must return false for an unknown ID")
}

// ---------------------------------------------------------------------------
// FEAT033 — EarlierShapersMustRunFirst edge cases
// ---------------------------------------------------------------------------

func TestFEAT033_EarlierShapersMustRunFirst_SameID_ReturnsFalse(t *testing.T) {
	earlier, ok := EarlierShapersMustRunFirst(ShaperSnip, ShaperSnip)
	require.True(t, ok, "both IDs are valid so ok must be true")
	assert.False(t, earlier,
		"a shaper cannot be earlier than itself (order equal)")
}

func TestFEAT033_EarlierShapersMustRunFirst_InvalidIDReturnsFalseOk(t *testing.T) {
	_, ok := EarlierShapersMustRunFirst("unknown", ShaperSnip)
	assert.False(t, ok,
		"unknown shaper ID must cause ok=false")
}

// ---------------------------------------------------------------------------
// FEAT033 — ShapersByFeatureFlag
// ---------------------------------------------------------------------------

func TestFEAT033_ShapersByFeatureFlag_HistorySnipReturnsOne(t *testing.T) {
	results := ShapersByFeatureFlag(ShaperFlagHistorySnip)
	require.Len(t, results, 1)
	assert.Equal(t, ShaperSnip, results[0].ID)
}

func TestFEAT033_ShapersByFeatureFlag_NoneReturnsTwoAlwaysActive(t *testing.T) {
	// ShaperFlagNone applies to both ShaperBudgetReduction (always) and
	// ShaperAutoCompact (default_on, no flag).
	results := ShapersByFeatureFlag(ShaperFlagNone)
	assert.Len(t, results, 2,
		"budget_reduction and auto_compact have no feature flag gate")
}

// ---------------------------------------------------------------------------
// FEAT033 — PDFSection populated for all entries
// ---------------------------------------------------------------------------

func TestFEAT033_PDFSection_AllCite43(t *testing.T) {
	for _, p := range AllPreModelContextShapers() {
		assert.Equal(t, "§4.3", p.PDFSection,
			"all shapers must cite §4.3 as their authoritative PDF section")
	}
}

// ---------------------------------------------------------------------------
// FEAT033 — Composability notes are non-empty
// ---------------------------------------------------------------------------

func TestFEAT033_ComposabilityNote_AllNonEmpty(t *testing.T) {
	for _, p := range AllPreModelContextShapers() {
		assert.NotEmpty(t, p.ComposabilityNote,
			"shaper %q must have a composability note explaining its interaction with neighbors (§4.3 emphasis)", p.ID)
	}
}
