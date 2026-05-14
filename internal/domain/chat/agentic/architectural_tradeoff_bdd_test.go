package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FEAT023 BDD tests for ArchitecturalTradeoffRegistry (§11.3 + §11.7).

func TestFEAT023_BDD_ArchitecturalTradeoffRegistry(t *testing.T) {

	t.Run("Scenario_RegistryBootstrapsWithSixProfiles", func(t *testing.T) {
		// Given an empty application context
		// When a new ArchitecturalTradeoffRegistry is constructed
		r := NewArchitecturalTradeoffRegistry()

		// Then it must contain exactly six profiles (3 §11.3 trade-offs + 3 §11.7 choices)
		require.NotNil(t, r)
		assert.Equal(t, SeedArchitecturalTradeoffCount, len(r.AllProfiles()))
	})

	t.Run("Scenario_ConcreteAndRecurringChoicesAreDisjoint", func(t *testing.T) {
		// Given a constructed registry
		r := NewArchitecturalTradeoffRegistry()

		// When we query concrete trade-offs and recurring choices separately
		concretes := r.ConcreteTradeoffs()
		choices := r.RecurringChoices()

		// Then the two groups are disjoint and together cover all six profiles
		concreteIDs := make(map[ArchitecturalTradeoffID]bool)
		for _, p := range concretes {
			concreteIDs[p.ID] = true
		}
		for _, p := range choices {
			assert.False(t, concreteIDs[p.ID],
				"profile %q appears in both ConcreteTradeoffs and RecurringChoices", p.ID)
		}
		assert.Equal(t, SeedArchitecturalTradeoffCount, len(concretes)+len(choices))
	})

	t.Run("Scenario_AllSlugsResolveToCorrectPDFSection", func(t *testing.T) {
		// Given a registry with pre-loaded profiles
		r := NewArchitecturalTradeoffRegistry()

		// When each slug is looked up
		// Then §11.3 slugs map to PDFSection "11.3" and §11.7 slugs map to "11.7"
		section113 := []ArchitecturalTradeoffID{
			TradeoffSafetyVsAutonomy,
			TradeoffContextEfficiencyVsTransparency,
			TradeoffSimplicityVsExtensibility,
		}
		section117 := []ArchitecturalTradeoffID{
			ChoiceGraduatedLayering,
			ChoiceAppendOnlyAuditability,
			ChoiceModelJudgmentInHarness,
		}
		for _, slug := range section113 {
			p, ok := r.FindArchitecturalTradeoffBySlug(slug)
			require.True(t, ok, "slug %q should exist", slug)
			assert.Equal(t, "11.3", p.PDFSection)
		}
		for _, slug := range section117 {
			p, ok := r.FindArchitecturalTradeoffBySlug(slug)
			require.True(t, ok, "slug %q should exist", slug)
			assert.Equal(t, "11.7", p.PDFSection)
		}
	})

	t.Run("Scenario_SafetyValueCutsAcrossMultipleTradeoffs", func(t *testing.T) {
		// Given the paper's observation that safety tensions recur throughout (§11.2–§11.3)
		// When we query profiles involving DesignValueSafety
		r := NewArchitecturalTradeoffRegistry()
		results := r.InvolvingValue(DesignValueSafety)

		// Then at least three profiles reference safety
		// (safety_vs_autonomy, simplicity_vs_extensibility, graduated_layering)
		assert.GreaterOrEqual(t, len(results), 3,
			"safety tensions should appear in at least 3 of the 6 profiles")
	})

	t.Run("Scenario_PermissionsSubsystemIsHighlyEntangled", func(t *testing.T) {
		// Given that §11.3 and §11.7 both repeatedly reference the permissions subsystem
		// When we query profiles affecting "permissions"
		r := NewArchitecturalTradeoffRegistry()
		results := r.AffectingSubsystem("permissions")

		// Then at least three profiles are returned (safety_vs_autonomy, simplicity_vs_extensibility,
		// graduated_layering, append_only_auditability, model_judgment_in_harness all touch it)
		assert.GreaterOrEqual(t, len(results), 3,
			"permissions should be the most entangled subsystem across §11.3 and §11.7")
	})

	t.Run("Scenario_ModelJudgmentInHarnessCaptures1Point6Ratio", func(t *testing.T) {
		// Given §11.7's quantitative claim: 1.6% decision logic, 98.4% operational harness
		// When the model_judgment_in_harness profile is fetched
		r := NewArchitecturalTradeoffRegistry()
		p, ok := r.FindArchitecturalTradeoffBySlug(ChoiceModelJudgmentInHarness)

		// Then the profile exists, is a recurring choice, and its description encodes the ratio
		require.True(t, ok)
		assert.Equal(t, KindRecurringChoice, p.Kind)
		assert.Contains(t, p.ConsequenceDescription, "1.6%",
			"consequence description should cite the 1.6% decision-logic ratio from §11.7")
		assert.Contains(t, p.ConsequenceDescription, "98.4%",
			"consequence description should cite the 98.4% harness ratio from §11.7")
	})

	t.Run("Scenario_AppendOnlyChoicePreservesAuditabilityAtQueryCost", func(t *testing.T) {
		// Given §11.7's observation: append-only favors auditability over query power
		// When the append_only_auditability profile is fetched
		r := NewArchitecturalTradeoffRegistry()
		p, ok := r.FindArchitecturalTradeoffBySlug(ChoiceAppendOnlyAuditability)

		// Then the profile captures sessions and context_management as affected subsystems
		// and mentions the trade-off cost (post-hoc reconstruction)
		require.True(t, ok)
		assert.Equal(t, KindRecurringChoice, p.Kind)
		assert.Contains(t, p.SubsystemsAffected, "sessions")
		assert.Contains(t, p.SubsystemsAffected, "context_management")
		assert.Contains(t, p.ConsequenceDescription, "reconstruction",
			"should mention post-hoc reconstruction as the cost of append-only design")
	})

	t.Run("Scenario_UnknownSlugReturnsNoProfile", func(t *testing.T) {
		// Given a valid registry
		r := NewArchitecturalTradeoffRegistry()

		// When a slug that does not exist in the paper is looked up
		p, ok := r.FindArchitecturalTradeoffBySlug("made_up_tradeoff")

		// Then the lookup returns false and a nil pointer
		assert.False(t, ok)
		assert.Nil(t, p)
	})
}
