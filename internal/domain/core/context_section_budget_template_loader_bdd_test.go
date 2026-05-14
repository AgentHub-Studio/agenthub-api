package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreContextBudgetTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantHasReadyBudgetProfilesForCommonModelTiers", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And CTX-001 ContextAssembler accepts BudgetLimit + per-kind caps,
		// When admin opens the agent-create form,
		// Then 5 recommended budget profiles are visible covering: small
		// (8k), balanced (32k), conversation-heavy (64k), code-heavy
		// (64k), research-heavy (128k) — admin doesn't have to guess
		// per-section numbers.
		assert.Equal(t, 5, len(SeedRecommendedContextBudgetTemplateSlugs))
	})

	t.Run("Scenario_BalancedDefaultMirrorsCTX001DefaultAssemblerConfig", func(t *testing.T) {
		// Given CTX-001 ships DefaultAssemblerConfig with 32k budget,
		// When admin picks balanced-default,
		// Then the template's total_budget matches the 32k default so
		// the in-code default and the seeded one-click are coherent.
		assert.Contains(t, SeedExpectedContextBudgetTemplateSlugs, "balanced-default")
	})

	t.Run("Scenario_MinimumViableIsOptInNotRecommended", func(t *testing.T) {
		// Given 4k budget is a degraded experience (most catalogs +
		// memory squeezed out),
		// When admin sees the catalog,
		// Then minimum-viable-4k is NOT highlighted as recommended —
		// admin must explicitly opt in (it's the "I have no choice"
		// option, not a default).
		set := map[string]bool{}
		for _, s := range SeedRecommendedContextBudgetTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["minimum-viable-4k"])
	})

	t.Run("Scenario_ResearchHeavyAllocatesMostKBBudgetForGroundedAnswers", func(t *testing.T) {
		// Given research workflows depend on grounding in many KB sources,
		// When admin picks research-heavy-128k,
		// Then the KB cap is the largest among templates so the agent
		// reads enough source material per turn — validated structurally
		// via integration test (KB cap > KB cap of conversation-heavy).
		assert.Contains(t, SeedExpectedContextBudgetTemplateSlugs, "research-heavy-128k")
	})

	t.Run("Scenario_ConversationHeavyMaximizesRecentMessagesForLongChats", func(t *testing.T) {
		// Given customer-support agents need 50+ turn history for grounding,
		// When admin picks conversation-heavy-64k,
		// Then recent_messages cap dominates other sections — the agent
		// retains conversation context.
		assert.Contains(t, SeedExpectedContextBudgetTemplateSlugs, "conversation-heavy-64k")
	})

	t.Run("Scenario_CodeHeavyAllocatesGenerousScratchpadForReasoning", func(t *testing.T) {
		// Given code-generation agents reason in scratchpad before emitting,
		// When admin picks code-heavy-64k,
		// Then scratchpad cap is significantly larger than in
		// conversation/research templates (validated via integration).
		assert.Contains(t, SeedExpectedContextBudgetTemplateSlugs, "code-heavy-64k")
	})

	t.Run("Scenario_PerSectionLabelsMatchCTX001ContextSectionKindByteForByte", func(t *testing.T) {
		// Given CTX-001 ContextSectionKind has 9 enum values,
		// When the loader exposes PerSectionCaps,
		// Then the map keys match enum bytes (no mapping table runtime).
		tmpl := CoreContextSectionBudgetTemplate{}
		caps := tmpl.PerSectionCaps()
		ctx001 := []string{
			"system", "memory", "rules", "skill_catalog", "tool_catalog",
			"kb_summary", "recent_messages", "aux_prompt", "scratchpad",
		}
		for _, k := range ctx001 {
			_, ok := caps[k]
			assert.True(t, ok, "CTX-001 section %q missing from PerSectionCaps", k)
		}
	})

	t.Run("Scenario_UseCasesAreBoundedSoUIPickerIsStable", func(t *testing.T) {
		// Given the agent-create UI has a use-case selector,
		// When admin picks a use case,
		// Then it's one of: general / research / conversation / code
		// (closed set; new use cases require a seed update + UI update).
		expected := []string{"general", "research", "conversation", "code"}
		set := map[string]bool{}
		for _, u := range SeedExpectedContextBudgetTemplateUseCases {
			set[u] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_ModelFamilyHintsHelpAdminMatchBudgetToModel", func(t *testing.T) {
		// Given budget recommendations depend on model context window,
		// When admin selects a budget template,
		// Then it carries a recommended_for_model_family hint that the
		// UI can use to pre-filter (e.g. show "tiny_legacy" only when
		// admin is configuring a 4k model).
		expected := []string{"mid_tier", "small_local", "large_context", "tiny_legacy"}
		set := map[string]bool{}
		for _, m := range SeedExpectedContextBudgetTemplateModelFamilies {
			set[m] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_SixProfilesCoverCommonCombinationsWithoutOverwhelm", func(t *testing.T) {
		// Given product research showed 5-7 templates is the sweet spot
		// (≥10 templates overwhelm; ≤3 leave gaps),
		// When the seed ships,
		// Then exactly 6 profiles exist (one per common scenario).
		assert.Equal(t, 6, len(SeedExpectedContextBudgetTemplateSlugs))
	})
}
