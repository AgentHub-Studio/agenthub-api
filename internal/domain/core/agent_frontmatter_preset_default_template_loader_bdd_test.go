package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for AgentFrontmatterPresetDefaultTemplate seed.
// Frontmatter presets define YAML header blocks injected into agent system
// prompts, encoding persona, priority, and response style metadata.

func TestBDD_AhCoreAgentFrontmatterPresetSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesSixFrontmatterPresets", func(t *testing.T) {
		// Given a new tenant bootstraps for the first time
		// When the system loads frontmatter preset templates
		// Then exactly 6 presets are available covering common agent personas
		assert.Equal(t, 6, SeedExpectedAFPTRowCount)
		assert.Equal(t, 6, len(SeedExpectedAFPTSlugs))
	})

	t.Run("Scenario_AllExpectedPresetsAreInCatalog", func(t *testing.T) {
		// Given the canonical preset slugs are defined in the seed
		// When admin inspects the available presets
		// Then each named preset slug appears in the expected list
		slugSet := map[string]bool{}
		for _, s := range SeedExpectedAFPTSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["fast-responder"], "fast-responder preset required")
		assert.True(t, slugSet["careful-analyst"], "careful-analyst preset required")
		assert.True(t, slugSet["creative-writer"], "creative-writer preset required")
		assert.True(t, slugSet["security-auditor"], "security-auditor preset required")
		assert.True(t, slugSet["data-extractor"], "data-extractor preset required")
		assert.True(t, slugSet["research-assistant"], "research-assistant preset required")
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each preset slug is validated
		// Then every slug matches the ^[a-z0-9][a-z0-9-]*[a-z0-9]$ pattern
		for _, s := range SeedExpectedAFPTSlugs {
			assert.True(t, SeedAFPTSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})

	t.Run("Scenario_SecurityAuditorIsHighPriorityPreset", func(t *testing.T) {
		// Given security-auditor agents must run at elevated priority
		// When the system classifies high-priority presets
		// Then security-auditor appears in the high-priority subset
		assert.Contains(t, SeedHighPrioritySlugs, "security-auditor")
	})

	t.Run("Scenario_HighPrioritySubsetIsStrictlySmaller", func(t *testing.T) {
		// Given most presets operate at normal priority
		// When the high-priority list is compared to the full catalog
		// Then the high-priority count is less than the total preset count
		assert.Less(t, len(SeedHighPrioritySlugs), SeedExpectedAFPTRowCount)
	})

	t.Run("Scenario_HighPrioritySlugsMustBeInMainCatalog", func(t *testing.T) {
		// Given high-priority slugs reference real presets
		// When each high-priority slug is validated
		// Then every high-priority slug also exists in the full catalog
		slugSet := map[string]bool{}
		for _, s := range SeedExpectedAFPTSlugs {
			slugSet[s] = true
		}
		for _, hp := range SeedHighPrioritySlugs {
			assert.True(t, slugSet[hp], "high-priority slug %q not in main catalog", hp)
		}
	})
}
