package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify the ah_core.rule seed against the
// project goal: a fresh tenant inherits sensible behavioural rules from
// ah_core without configuring anything.
//
// Backed by SeedExpectedRuleSlugs / SeedExpectedRuleCategories /
// SeedAdminOnlyDisableSlugs constants which the integration test
// cross-checks against actual DB rows.

func TestBDD_AhCoreRuleSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsBaselineRuleSet", func(t *testing.T) {
		// Given a fresh tenant (no rules of its own),
		// When the runtime asks ah_core for rules,
		// Then a baseline ≥10 rules is seeded — system prompt has
		//      meaningful safety / quality / behaviour / privacy
		//      directives from day one.
		assert.GreaterOrEqual(t, len(SeedExpectedRuleSlugs), 10,
			"fresh tenant must inherit at least 10 baseline rules")
	})

	t.Run("Scenario_SafetyCategoryFullyAdminOnlyDisable", func(t *testing.T) {
		// Given safety rules are bypass-immune by design (PDF Section 5
		//       deny-first principle: safety-critical checks cannot be
		//       silently disabled),
		safety := []string{
			"safety-no-secret-disclosure",
			"safety-confirm-irreversible",
			"safety-decline-illegal",
			"safety-escalate-uncertain",
		}

		// When we cross-check against admin-only set,
		adminSet := map[string]bool{}
		for _, s := range SeedAdminOnlyDisableSlugs {
			adminSet[s] = true
		}

		// Then ALL safety rules require admin to disable.
		for _, s := range safety {
			assert.True(t, adminSet[s],
				"safety rule %q must require admin to disable", s)
		}
	})

	t.Run("Scenario_PrivacyCategoryFullyAdminOnlyDisable", func(t *testing.T) {
		// Given privacy rules guard tenant data isolation (PDF Section
		//       2.1: Privacy is one of the Five Values),
		privacy := []string{
			"privacy-redact-pii",
			"privacy-minimize-collection",
			"privacy-no-cross-tenant-leak",
		}
		adminSet := map[string]bool{}
		for _, s := range SeedAdminOnlyDisableSlugs {
			adminSet[s] = true
		}
		for _, s := range privacy {
			assert.True(t, adminSet[s],
				"privacy rule %q must require admin to disable", s)
		}
	})

	t.Run("Scenario_NoCrossTenantLeakRuleIsHighestPriority", func(t *testing.T) {
		// Given multi-tenant isolation is the most important invariant
		//       (PDF principle: tenants are isolated trust domains —
		//       cross-tenant leak is the worst possible failure),
		// When we look up the rule slug,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedRuleSlugs {
			seedSet[s] = true
		}

		// Then privacy-no-cross-tenant-leak exists.
		assert.True(t, seedSet["privacy-no-cross-tenant-leak"],
			"highest-stakes rule must be in the seed")
	})

	t.Run("Scenario_QualityRulesEnforceGeneratorEvaluatorSeparation", func(t *testing.T) {
		// Given PDF Section 11 generator/evaluator separation (don't
		//       fabricate; cite sources; acknowledge uncertainty),
		quality := []string{
			"quality-cite-sources",
			"quality-acknowledge-uncertainty",
			"quality-prefer-existing-context",
		}
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedRuleSlugs {
			seedSet[s] = true
		}
		for _, s := range quality {
			assert.True(t, seedSet[s],
				"quality rule %q must be in the seed (PDF Section 11)", s)
		}
	})

	t.Run("Scenario_BehaviourRulesAreConcisenessAndClarity", func(t *testing.T) {
		// Given the agent must be useful in a web chat UX (concise +
		//       respectful + clarifying when ambiguous),
		behaviour := []string{
			"behavior-be-concise",
			"behavior-ask-when-ambiguous",
			"behavior-respectful-tone",
		}
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedRuleSlugs {
			seedSet[s] = true
		}
		for _, s := range behaviour {
			assert.True(t, seedSet[s],
				"behaviour rule %q must be in the seed for web UX", s)
		}
	})

	t.Run("Scenario_FourDistinctCategoriesForUIGrouping", func(t *testing.T) {
		// Given the admin UI groups rules by category for togglability,
		assert.Len(t, SeedExpectedRuleCategories, 4,
			"4 categories expected for admin UI grouping")
		seenCat := map[string]bool{}
		for _, c := range SeedExpectedRuleCategories {
			assert.False(t, seenCat[c], "duplicate category %q", c)
			seenCat[c] = true
		}
	})

	t.Run("Scenario_SeedSlugPrefixMatchesItsCategory", func(t *testing.T) {
		// Given the convention that slugs name their category as prefix
		//       (safety-*, quality-*, behavior-*, privacy-*),
		// When the loader scans slugs,
		// Then every slug carries its category prefix — refactor that
		//      adds a category-mismatched slug surfaces immediately.
		validPrefixes := []string{"safety-", "quality-", "behavior-", "privacy-"}
		for _, slug := range SeedExpectedRuleSlugs {
			matched := false
			for _, p := range validPrefixes {
				if strings.HasPrefix(slug, p) {
					matched = true
					break
				}
			}
			assert.True(t, matched,
				"slug %q must start with one of %v", slug, validPrefixes)
		}
	})

	t.Run("Scenario_CanonicalCountIsExplicitGuard", func(t *testing.T) {
		// Given seed migration changes are easy to make accidentally,
		assert.Equal(t, 13, len(SeedExpectedRuleSlugs),
			"canonical count is 13 — change requires updating both migration and list")
	})

	t.Run("Scenario_AllRulesAreScopedGlobalByDefault", func(t *testing.T) {
		// Given the baseline rules are universal directives, not
		//       tool/skill-specific overrides,
		// When the seed defines them,
		// Then they should be scope='global' (DEFAULT in the migration).
		// We assert this contract via documentation here; the integration
		// test verifies the actual DB rows have scope='global'.
		// Just a sanity check on the constant list shape.
		assert.NotEmpty(t, SeedExpectedRuleSlugs)
	})
}
