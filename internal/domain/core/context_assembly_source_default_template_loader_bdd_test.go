package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for context_assembly_source_template seed.
// Maps §7.1 nine-source assembly order to ah_core default templates.

func TestBDD_AhCoreContextAssemblySourceSeed(t *testing.T) {
	t.Run("Scenario_NineSourcesRepresentFullContextWindowFromSpec", func(t *testing.T) {
		// Given §7.1 specifies exactly nine ordered sources for context assembly
		// When the platform loads source presets
		// Then exactly 9 sources are available from system_prompt to compact_summaries
		assert.Equal(t, 9, SeedExpectedContextAssemblySourceRowCount)
		assert.Equal(t, 9, len(SeedExpectedContextAssemblySourceSlugs))
	})

	t.Run("Scenario_SixDomainsCoverAllFunctionalRoles", func(t *testing.T) {
		// Given sources are grouped by functional role for clarity
		// When domains are enumerated
		// Then 6 domains cover all source categories
		assert.Equal(t, 6, len(SeedContextAssemblySourceDomains))
	})

	t.Run("Scenario_SixSourcesAreAlwaysIncludedThreeAreConditional", func(t *testing.T) {
		// Given path_scoped_rules, auto_memory, compact_summaries are conditional
		// When always-included slugs are inspected
		// Then exactly 6 are always included (9 total - 3 conditional)
		assert.Equal(t, 6, len(SeedContextAssemblyAlwaysIncludedSlugs))
		conditionals := []string{"path_scoped_rules", "auto_memory", "compact_summaries"}
		for _, s := range conditionals {
			found := false
			for _, a := range SeedContextAssemblyAlwaysIncludedSlugs {
				if a == s {
					found = true
				}
			}
			assert.False(t, found, "conditional source %q must not be in always-included list", s)
		}
	})

	t.Run("Scenario_OnlyAutoMemoryAndToolMetadataAreAsynchronous", func(t *testing.T) {
		// Given auto_memory is prefetched async and tool_metadata deferred via ToolSearch
		// When async slugs are inspected
		// Then exactly 2 sources are asynchronous
		assert.Equal(t, 2, len(SeedContextAssemblyAsyncSlugs))
		assert.Contains(t, SeedContextAssemblyAsyncSlugs, "auto_memory")
		assert.Contains(t, SeedContextAssemblyAsyncSlugs, "tool_metadata")
	})

	t.Run("Scenario_AllAsyncSlugsAreSubsetOfCanonicalList", func(t *testing.T) {
		// Given async sources must exist in the canonical 9-source list
		// When each async slug is checked
		// Then no orphan slug exists
		all := map[string]bool{}
		for _, s := range SeedExpectedContextAssemblySourceSlugs {
			all[s] = true
		}
		for _, s := range SeedContextAssemblyAsyncSlugs {
			assert.True(t, all[s], "async slug %q not in canonical list", s)
		}
	})
}
