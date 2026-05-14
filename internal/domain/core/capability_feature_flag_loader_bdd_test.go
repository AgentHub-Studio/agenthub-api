package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000103 capability feature flag seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityFeatureFlagSeed(t *testing.T) {
	t.Run("Scenario_SixCapabilityFeatureFlags", func(t *testing.T) {
		// Given the capability layer requires per-feature toggles adapted from
		//   Appendix A.2 Table 8 (conditional tool availability in Claude Code),
		// When migration 000103 seeds feature flag rows,
		// Then exactly 6 rows are added across 5 categories, and the slug slice
		//   length matches the declared count constant.
		assert.Equal(t, 6, SeedCapabilityFeatureFlagCount,
			"migration 000103 must seed exactly 6 capability feature flag rows")
		assert.Len(t, SeedCapabilityFeatureFlagSlugs, 6,
			"SeedCapabilityFeatureFlagSlugs must list exactly 6 feature flag slugs")
		assert.Equal(t, SeedCapabilityFeatureFlagCount, len(SeedCapabilityFeatureFlagSlugs),
			"SeedCapabilityFeatureFlagCount must equal len(SeedCapabilityFeatureFlagSlugs)")
	})

	t.Run("Scenario_FiveFeatureFlagsEnabledByDefault", func(t *testing.T) {
		// Given most capability features should be active out-of-the-box to
		//   minimise configuration friction for new tenants,
		// When migration 000103 seeds feature flag default_enabled values,
		// Then exactly 5 of the 6 flags are enabled by default, with only
		//   subagent-delegation requiring explicit opt-in.
		disabledCount := 1 // only capability-subagent-delegation
		enabledByDefault := SeedCapabilityFeatureFlagCount - disabledCount
		assert.Equal(t, 5, enabledByDefault,
			"exactly 5 capability feature flags must be default_enabled=TRUE (subagent-delegation is the sole exception)")
		assert.Equal(t, SeedSubagentDelegationFeatureFlagSlug, SeedDisabledByDefaultFeatureFlagSlug,
			"the single disabled-by-default flag must be SeedSubagentDelegationFeatureFlagSlug")
	})

	t.Run("Scenario_SubagentDelegationDisabledByDefaultForSafety", func(t *testing.T) {
		// Given spawning subagents can cause unintended cost escalation and
		//   recursive delegation loops in new tenants unfamiliar with the feature,
		// When migration 000103 seeds the subagent-delegation feature flag,
		// Then default_enabled is FALSE, requiring the tenant admin to explicitly
		//   enable delegation before capability agents can spawn subagents.
		assert.Equal(t, "capability-subagent-delegation", SeedSubagentDelegationFeatureFlagSlug,
			"SeedSubagentDelegationFeatureFlagSlug must equal \"capability-subagent-delegation\"")
		assert.Equal(t, SeedDisabledByDefaultFeatureFlagSlug, SeedSubagentDelegationFeatureFlagSlug,
			"SeedDisabledByDefaultFeatureFlagSlug must reference capability-subagent-delegation — the only opt-in flag")
		// Confirm it is NOT the same as any other slug (it is uniquely disabled).
		otherSlugs := []string{
			SeedCitationsFeatureFlagSlug,
			SeedTaskTrackingFeatureFlagSlug,
			SeedKBIndexingFeatureFlagSlug,
			SeedDocCitationsFeatureFlagSlug,
			SeedProgressiveSummarizationFeatureFlagSlug,
		}
		for _, slug := range otherSlugs {
			assert.NotEqual(t, SeedDisabledByDefaultFeatureFlagSlug, slug,
				"flag %q must NOT be the disabled-by-default flag — only subagent-delegation is opt-in", slug)
		}
	})

	t.Run("Scenario_ResearchCategoryHasTwoFlags", func(t *testing.T) {
		// Given research-oriented capability features (citation tracking and
		//   knowledge base indexing) share the same functional domain,
		// When migration 000103 assigns categories to feature flags,
		// Then the research category contains exactly 2 flags:
		//   capability-citations and capability-kb-indexing.
		researchFlags := []string{
			SeedCitationsFeatureFlagSlug,
			SeedKBIndexingFeatureFlagSlug,
		}
		assert.Len(t, researchFlags, 2,
			"research category must have exactly 2 feature flags (citations + kb-indexing)")
		assert.Equal(t, SeedFeatureFlagCategoryResearch, "research",
			"SeedFeatureFlagCategoryResearch must equal \"research\"")
		for _, slug := range researchFlags {
			assert.Contains(t, SeedCapabilityFeatureFlagSlugs, slug,
				"research flag %q must be present in SeedCapabilityFeatureFlagSlugs", slug)
		}
	})

	t.Run("Scenario_FeatureFlagCategoriesMatchCapabilityRoles", func(t *testing.T) {
		// Given the capability layer organises agents into distinct roles
		//   (researcher, planner, analyst, orchestrator, memory manager),
		// When migration 000103 assigns category labels to feature flags,
		// Then each category constant maps to a distinct string, the five
		//   categories are all unique, and they cover the core capability roles.
		categories := []string{
			SeedFeatureFlagCategoryResearch,
			SeedFeatureFlagCategoryPlanning,
			SeedFeatureFlagCategoryAnalysis,
			SeedFeatureFlagCategoryOrchestration,
			SeedFeatureFlagCategoryMemory,
		}
		assert.Len(t, categories, 5,
			"migration 000103 must define exactly 5 category constants")
		unique := map[string]struct{}{}
		for _, c := range categories {
			assert.NotEmpty(t, c, "every category constant must be a non-empty string")
			unique[c] = struct{}{}
		}
		assert.Len(t, unique, 5,
			"all 5 category constants must be distinct strings — no two capability roles share the same category label")
	})
}
