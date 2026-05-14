package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// FEAT037 — LoopStopConditionRegistry (§4.5)
//
// Unit tests: 22 cases covering profile access, category/source queries,
// recoverability, enumeration order, structural invariants, and AgentHub mappings.
// BDD tests: 7 scenario-style cases covering runtime behaviour.
// ─────────────────────────────────────────────────────────────────────────────

// ── helpers ──────────────────────────────────────────────────────────────────

func newFEAT037Registry() *LoopStopConditionRegistry {
	return NewLoopStopConditionRegistry()
}

// ── FEAT037 Unit tests ────────────────────────────────────────────────────────

// TestFEAT037_RegistryCreation ensures the constructor returns a non-nil registry.
func TestFEAT037_RegistryCreation(t *testing.T) {
	r := newFEAT037Registry()
	require.NotNil(t, r)
}

// TestFEAT037_Count verifies exactly 5 conditions are registered (§4.5).
func TestFEAT037_Count(t *testing.T) {
	r := newFEAT037Registry()
	assert.Equal(t, 5, r.Count())
}

// TestFEAT037_SeedConstant verifies SeedLoopStopConditionCount equals 5.
func TestFEAT037_SeedConstant(t *testing.T) {
	assert.Equal(t, 5, SeedLoopStopConditionCount)
}

// TestFEAT037_AllConditionsLength verifies AllConditions returns exactly 5 items.
func TestFEAT037_AllConditionsLength(t *testing.T) {
	r := newFEAT037Registry()
	all := r.AllConditions()
	assert.Len(t, all, 5)
}

// TestFEAT037_AllConditionsOrder verifies the §4.5 enumeration order:
// no_tool_use(1) → max_turns(2) → context_overflow(3) → hook_intervention(4) → explicit_abort(5).
func TestFEAT037_AllConditionsOrder(t *testing.T) {
	r := newFEAT037Registry()
	all := r.AllConditions()
	expectedIDs := []LoopStopConditionID{
		LoopStopNoToolUse,
		LoopStopMaxTurns,
		LoopStopContextOverflow,
		LoopStopHookIntervention,
		LoopStopExplicitAbort,
	}
	require.Len(t, all, len(expectedIDs))
	for i, want := range expectedIDs {
		assert.Equal(t, want, all[i].ID, "position %d", i+1)
	}
}

// TestFEAT037_ProfileLookup_KnownID verifies Profile returns a valid profile for each known ID.
func TestFEAT037_ProfileLookup_KnownID(t *testing.T) {
	r := newFEAT037Registry()
	knownIDs := []LoopStopConditionID{
		LoopStopNoToolUse,
		LoopStopMaxTurns,
		LoopStopContextOverflow,
		LoopStopHookIntervention,
		LoopStopExplicitAbort,
	}
	for _, id := range knownIDs {
		p, ok := r.Profile(id)
		assert.True(t, ok, "Profile(%q) should return ok=true", id)
		assert.Equal(t, id, p.ID)
		assert.Equal(t, "4.5", p.PDFSection)
		assert.NotEmpty(t, p.Label)
		assert.NotEmpty(t, p.Description)
	}
}

// TestFEAT037_ProfileLookup_UnknownID verifies Profile returns ok=false for unknown slugs.
func TestFEAT037_ProfileLookup_UnknownID(t *testing.T) {
	r := newFEAT037Registry()
	_, ok := r.Profile("nonexistent_condition")
	assert.False(t, ok)
}

// TestFEAT037_PrimaryStopCondition verifies only no_tool_use is the primary stop.
func TestFEAT037_PrimaryStopCondition(t *testing.T) {
	r := newFEAT037Registry()
	primary := r.PrimaryStopCondition()
	assert.Equal(t, LoopStopNoToolUse, primary.ID)
	assert.True(t, primary.IsPrimaryStopCondition)

	// All other conditions must NOT be primary.
	for _, p := range r.AllConditions() {
		if p.ID == LoopStopNoToolUse {
			continue
		}
		assert.False(t, p.IsPrimaryStopCondition, "%s should not be primary", p.ID)
	}
}

// TestFEAT037_NoToolUse_IsNormalCompletion verifies no_tool_use category and trigger.
func TestFEAT037_NoToolUse_IsNormalCompletion(t *testing.T) {
	r := newFEAT037Registry()
	p, ok := r.Profile(LoopStopNoToolUse)
	require.True(t, ok)
	assert.Equal(t, LoopStopCategoryNormalCompletion, p.Category)
	assert.Equal(t, LoopStopTriggerModelOutput, p.TriggerSource)
	assert.False(t, p.IsRecoverable)
	assert.Empty(t, p.RuntimeSignal) // no explicit runtime flag for text-only detection
}

// TestFEAT037_MaxTurns_IsResourceLimit verifies max_turns category and trigger.
func TestFEAT037_MaxTurns_IsResourceLimit(t *testing.T) {
	r := newFEAT037Registry()
	p, ok := r.Profile(LoopStopMaxTurns)
	require.True(t, ok)
	assert.Equal(t, LoopStopCategoryResourceLimit, p.Category)
	assert.Equal(t, LoopStopTriggerHarness, p.TriggerSource)
	assert.False(t, p.IsRecoverable)
}

// TestFEAT037_ContextOverflow_IsRecoverable verifies context_overflow recoverability
// and that its RecoveryMechanismSlugs are non-empty.
func TestFEAT037_ContextOverflow_IsRecoverable(t *testing.T) {
	r := newFEAT037Registry()
	p, ok := r.Profile(LoopStopContextOverflow)
	require.True(t, ok)
	assert.Equal(t, LoopStopCategoryError, p.Category)
	assert.Equal(t, LoopStopTriggerAPIError, p.TriggerSource)
	assert.True(t, p.IsRecoverable, "context_overflow must be recoverable per §4.4/§4.5")
	assert.NotEmpty(t, p.RecoveryMechanismSlugs)
	assert.Equal(t, "prompt_too_long", p.RuntimeSignal)
}

// TestFEAT037_ContextOverflow_RecoveryMechanismsMatchSection44 verifies the recovery slugs
// are consistent with the §4.4 RecoveryMechanismID constants.
func TestFEAT037_ContextOverflow_RecoveryMechanismsMatchSection44(t *testing.T) {
	r := newFEAT037Registry()
	p, _ := r.Profile(LoopStopContextOverflow)
	// Both slugs must be valid §4.4 IDs (cross-registry consistency check).
	validSlugs := map[string]bool{
		string(RecoveryMaxOutputTokensEscalation): true,
		string(RecoveryReactiveCompaction):        true,
		string(RecoveryPromptTooLongHandling):     true,
		string(RecoveryStreamingFallback):         true,
		string(RecoveryFallbackModel):             true,
	}
	for _, slug := range p.RecoveryMechanismSlugs {
		assert.True(t, validSlugs[slug], "recovery slug %q is not a known §4.4 ID", slug)
	}
}

// TestFEAT037_HookIntervention_RuntimeSignal verifies hook_intervention carries the
// correct runtime signal name from §4.2/§4.5.
func TestFEAT037_HookIntervention_RuntimeSignal(t *testing.T) {
	r := newFEAT037Registry()
	p, ok := r.Profile(LoopStopHookIntervention)
	require.True(t, ok)
	assert.Equal(t, "hook_stopped_continuation", p.RuntimeSignal)
	assert.Equal(t, LoopStopTriggerHook, p.TriggerSource)
	assert.Equal(t, LoopStopCategoryUserOrHarnessAction, p.Category)
}

// TestFEAT037_ExplicitAbort_RuntimeSignal verifies explicit_abort carries AbortController.
func TestFEAT037_ExplicitAbort_RuntimeSignal(t *testing.T) {
	r := newFEAT037Registry()
	p, ok := r.Profile(LoopStopExplicitAbort)
	require.True(t, ok)
	assert.Equal(t, "AbortController", p.RuntimeSignal)
	assert.Equal(t, LoopStopTriggerOSSignal, p.TriggerSource)
}

// TestFEAT037_RecoverableConditions_OnlyContextOverflow verifies exactly one condition
// is marked recoverable (§4.5 only names context_overflow as having §4.4 recovery).
func TestFEAT037_RecoverableConditions_OnlyContextOverflow(t *testing.T) {
	r := newFEAT037Registry()
	recoverable := r.RecoverableConditions()
	require.Len(t, recoverable, 1, "exactly one condition should be recoverable per §4.4/§4.5")
	assert.Equal(t, LoopStopContextOverflow, recoverable[0].ID)
}

// TestFEAT037_ConditionsByCategory_NormalCompletion verifies the normal completion group.
func TestFEAT037_ConditionsByCategory_NormalCompletion(t *testing.T) {
	r := newFEAT037Registry()
	nc := r.ConditionsByCategory(LoopStopCategoryNormalCompletion)
	require.Len(t, nc, 1)
	assert.Equal(t, LoopStopNoToolUse, nc[0].ID)
}

// TestFEAT037_ConditionsByCategory_UserOrHarnessAction verifies external stop group.
func TestFEAT037_ConditionsByCategory_UserOrHarnessAction(t *testing.T) {
	r := newFEAT037Registry()
	ext := r.ConditionsByCategory(LoopStopCategoryUserOrHarnessAction)
	// §4.5 lists max_turns (harness) and hook_intervention and explicit_abort
	// both in user_or_harness_action. But max_turns is resource_limit.
	// So only hook_intervention and explicit_abort are user_or_harness_action.
	require.Len(t, ext, 2)
	ids := []LoopStopConditionID{ext[0].ID, ext[1].ID}
	assert.Contains(t, ids, LoopStopHookIntervention)
	assert.Contains(t, ids, LoopStopExplicitAbort)
}

// TestFEAT037_ConditionByEnumerationOrder_ValidRange verifies 1-based access.
func TestFEAT037_ConditionByEnumerationOrder_ValidRange(t *testing.T) {
	r := newFEAT037Registry()
	expectedOrder := []LoopStopConditionID{
		LoopStopNoToolUse,
		LoopStopMaxTurns,
		LoopStopContextOverflow,
		LoopStopHookIntervention,
		LoopStopExplicitAbort,
	}
	for i, want := range expectedOrder {
		p, ok := r.ConditionByEnumerationOrder(i + 1)
		assert.True(t, ok, "order %d should be valid", i+1)
		assert.Equal(t, want, p.ID)
	}
}

// TestFEAT037_ConditionByEnumerationOrder_OutOfRange verifies boundary rejection.
func TestFEAT037_ConditionByEnumerationOrder_OutOfRange(t *testing.T) {
	r := newFEAT037Registry()
	_, ok0 := r.ConditionByEnumerationOrder(0)
	assert.False(t, ok0)
	_, ok6 := r.ConditionByEnumerationOrder(6)
	assert.False(t, ok6)
}

// TestFEAT037_ConditionByRuntimeSignal_Known verifies signal-based lookup.
func TestFEAT037_ConditionByRuntimeSignal_Known(t *testing.T) {
	r := newFEAT037Registry()
	cases := []struct {
		signal string
		wantID LoopStopConditionID
	}{
		{"prompt_too_long", LoopStopContextOverflow},
		{"hook_stopped_continuation", LoopStopHookIntervention},
		{"AbortController", LoopStopExplicitAbort},
	}
	for _, tc := range cases {
		p, ok := r.ConditionByRuntimeSignal(tc.signal)
		assert.True(t, ok, "signal %q should find a condition", tc.signal)
		assert.Equal(t, tc.wantID, p.ID)
	}
}

// TestFEAT037_ConditionByRuntimeSignal_Unknown verifies unknown signal returns false.
func TestFEAT037_ConditionByRuntimeSignal_Unknown(t *testing.T) {
	r := newFEAT037Registry()
	_, ok := r.ConditionByRuntimeSignal("unknown_signal")
	assert.False(t, ok)
	_, okEmpty := r.ConditionByRuntimeSignal("")
	assert.False(t, okEmpty)
}

// TestFEAT037_ExternallyTriggeredConditions verifies the externally-triggered subset.
func TestFEAT037_ExternallyTriggeredConditions(t *testing.T) {
	r := newFEAT037Registry()
	ext := r.ExternallyTriggeredConditions()
	// harness=max_turns, hook=hook_intervention, os_signal=explicit_abort → 3 total
	require.Len(t, ext, 3)
	ids := make(map[LoopStopConditionID]bool)
	for _, p := range ext {
		ids[p.ID] = true
	}
	assert.True(t, ids[LoopStopMaxTurns])
	assert.True(t, ids[LoopStopHookIntervention])
	assert.True(t, ids[LoopStopExplicitAbort])
	// model_output and api_error must NOT be in the external set
	assert.False(t, ids[LoopStopNoToolUse])
	assert.False(t, ids[LoopStopContextOverflow])
}

// TestFEAT037_ConditionsNotifyingParentAgent verifies parent-notification subset.
func TestFEAT037_ConditionsNotifyingParentAgent(t *testing.T) {
	r := newFEAT037Registry()
	notifying := r.ConditionsNotifyingParentAgent()
	// explicit_abort does NOT notify parent (it tears down the whole call tree).
	// All others notify parent (4 conditions).
	require.Len(t, notifying, 4)
	ids := make(map[LoopStopConditionID]bool)
	for _, p := range notifying {
		ids[p.ID] = true
	}
	assert.False(t, ids[LoopStopExplicitAbort], "explicit_abort should NOT notify parent")
}

// TestFEAT037_StructuralInvariant_EnumerationOrderContiguous validates §4.5 order.
func TestFEAT037_StructuralInvariant_EnumerationOrderContiguous(t *testing.T) {
	assert.True(t, LoopStopEnumerationOrdersAreContiguous(),
		"§4.5 EnumerationOrder values must be contiguous 1..5")
}

// TestFEAT037_StructuralInvariant_AllPDFSectionsCanonical validates PDF references.
func TestFEAT037_StructuralInvariant_AllPDFSectionsCanonical(t *testing.T) {
	assert.True(t, LoopStopAllPDFSectionsAreCanonical(),
		"all §4.5 profiles must carry PDFSection=4.5")
}

// TestFEAT037_AllConditionsHaveAgenthubMapping verifies each condition has a mapping.
func TestFEAT037_AllConditionsHaveAgenthubMapping(t *testing.T) {
	r := newFEAT037Registry()
	for _, p := range r.AllConditions() {
		assert.NotEmpty(t, p.AgenthubMapping, "condition %q must have an AgenthubMapping", p.ID)
	}
}

// ── FEAT037 BDD tests ─────────────────────────────────────────────────────────

// TestFEAT037_BDD_NormalTurnCompletesOnNoToolUse models a normal text-only model turn.
// Scenario: the model returns no tool_call blocks → loop must stop (primary condition).
func TestFEAT037_BDD_NormalTurnCompletesOnNoToolUse(t *testing.T) {
	// Given: a loop iteration where the model produced text content only
	r := newFEAT037Registry()
	primary := r.PrimaryStopCondition()

	// Then: the primary stop condition is no_tool_use (§4.5 item 1)
	assert.Equal(t, LoopStopNoToolUse, primary.ID)
	assert.Equal(t, LoopStopCategoryNormalCompletion, primary.Category)
	assert.Equal(t, LoopStopTriggerModelOutput, primary.TriggerSource)
	assert.True(t, primary.NotifiesParentAgent,
		"text completion must propagate result to parent context")
}

// TestFEAT037_BDD_MaxTurnsLimitEnforcedByHarness models the harness guard-rail.
// Scenario: turns counter reaches the configured maxTurns → harness stops the loop.
func TestFEAT037_BDD_MaxTurnsLimitEnforcedByHarness(t *testing.T) {
	r := newFEAT037Registry()
	p, ok := r.Profile(LoopStopMaxTurns)
	require.True(t, ok)

	// The harness (not the model, not the API) detects this condition.
	assert.Equal(t, LoopStopTriggerHarness, p.TriggerSource)
	// It is a resource limit, not an error.
	assert.Equal(t, LoopStopCategoryResourceLimit, p.Category)
	// No §4.4 recovery is attempted for a turns limit.
	assert.False(t, p.IsRecoverable)
}

// TestFEAT037_BDD_ContextOverflowTriggersRecoveryFirst models the §4.4/§4.5 interaction.
// Scenario: API returns prompt_too_long → §4.4 reactive compaction + collapse run first,
// then if still failing the §4.5 context_overflow stop is applied.
func TestFEAT037_BDD_ContextOverflowTriggersRecoveryFirst(t *testing.T) {
	r := newFEAT037Registry()

	// Given: the API has returned prompt_too_long
	p, ok := r.ConditionByRuntimeSignal("prompt_too_long")
	require.True(t, ok)
	require.Equal(t, LoopStopContextOverflow, p.ID)

	// Then: recovery is attempted before final stop
	assert.True(t, p.IsRecoverable)
	assert.NotEmpty(t, p.RecoveryMechanismSlugs,
		"context_overflow must reference §4.4 recovery mechanisms")

	// And: the §4.4 mechanisms referenced are valid
	validRecovery := map[string]bool{
		"prompt_too_long_handling": true,
		"reactive_compaction":      true,
	}
	for _, slug := range p.RecoveryMechanismSlugs {
		assert.True(t, validRecovery[slug],
			"slug %q must be a §4.4 mechanism relevant to context overflow", slug)
	}
}

// TestFEAT037_BDD_HookInterventionPreventsNextTurn models hook-driven stop.
// Scenario: a PostToolUse hook writes shouldPreventContinuation → loop stops after
// tool result collection, before the next model call.
func TestFEAT037_BDD_HookInterventionPreventsNextTurn(t *testing.T) {
	r := newFEAT037Registry()

	// Given: the tool result collection phase detected hook_stopped_continuation
	p, ok := r.ConditionByRuntimeSignal("hook_stopped_continuation")
	require.True(t, ok)

	// Then: stop condition is hook_intervention, category is external action
	assert.Equal(t, LoopStopHookIntervention, p.ID)
	assert.Equal(t, LoopStopCategoryUserOrHarnessAction, p.Category)
	// And: the parent agent is notified so it can surface the hook's reason
	assert.True(t, p.NotifiesParentAgent)
	// And: no §4.4 recovery is attempted (the hook explicitly said stop)
	assert.False(t, p.IsRecoverable)
}

// TestFEAT037_BDD_ExplicitAbortTearsDownWithoutParentNotification models OS abort.
// Scenario: user hits Ctrl-C → abortController fires → entire execution tree exits.
func TestFEAT037_BDD_ExplicitAbortTearsDownWithoutParentNotification(t *testing.T) {
	r := newFEAT037Registry()

	// Given: abortController signal fired
	p, ok := r.ConditionByRuntimeSignal("AbortController")
	require.True(t, ok)

	// Then: explicit_abort, triggered by OS signal
	assert.Equal(t, LoopStopExplicitAbort, p.ID)
	assert.Equal(t, LoopStopTriggerOSSignal, p.TriggerSource)
	// And: unlike other stops, explicit abort does NOT propagate a parent notification
	// (the parent is also being torn down by the same signal)
	assert.False(t, p.NotifiesParentAgent,
		"explicit abort tears down the full agent tree; parent is not separately notified")
}

// TestFEAT037_BDD_ConditionSetCoversFiveDistinctCategories models full taxonomy.
// Scenario: a monitoring system must handle all possible stop reasons and classify them.
func TestFEAT037_BDD_ConditionSetCoversFiveDistinctCategories(t *testing.T) {
	r := newFEAT037Registry()
	all := r.AllConditions()

	// Collect all distinct categories present
	categories := make(map[LoopStopCategory]int)
	for _, p := range all {
		categories[p.Category]++
	}

	// Exactly four distinct stop categories across five conditions:
	// normal_completion(1), resource_limit(1), error(1), user_or_harness_action(2)
	assert.Len(t, categories, 4, "§4.5 five conditions span exactly four categories")
	assert.Equal(t, 1, categories[LoopStopCategoryNormalCompletion])
	assert.Equal(t, 1, categories[LoopStopCategoryResourceLimit])
	assert.Equal(t, 1, categories[LoopStopCategoryError])
	assert.Equal(t, 2, categories[LoopStopCategoryUserOrHarnessAction])
}

// TestFEAT037_BDD_TriggerSourceDistributionMatchesPaper validates trigger taxonomy.
// Scenario: a telemetry pipeline tags each stop event with its trigger source.
func TestFEAT037_BDD_TriggerSourceDistributionMatchesPaper(t *testing.T) {
	r := newFEAT037Registry()

	// §4.5 trigger distribution:
	// model_output=1 (no_tool_use), harness=1 (max_turns),
	// api_error=1 (context_overflow), hook=1 (hook_intervention), os_signal=1 (explicit_abort)
	bySource := make(map[LoopStopTriggerSource]int)
	for _, p := range r.AllConditions() {
		bySource[p.TriggerSource]++
	}
	assert.Equal(t, 1, bySource[LoopStopTriggerModelOutput], "one model_output stop")
	assert.Equal(t, 1, bySource[LoopStopTriggerHarness], "one harness stop")
	assert.Equal(t, 1, bySource[LoopStopTriggerAPIError], "one api_error stop")
	assert.Equal(t, 1, bySource[LoopStopTriggerHook], "one hook stop")
	assert.Equal(t, 1, bySource[LoopStopTriggerOSSignal], "one os_signal stop")
}
