package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000093 capability hook templates.
// These assert seed shape and event coverage without requiring a database.

func TestBDD_CapabilityHookTemplateSeed(t *testing.T) {
	t.Run("Scenario_FourCapabilityHooksInstrumentDistinctPhasesOfCapabilityAgentInvocation", func(t *testing.T) {
		// Given the capability agent layer (migrations 000090-000092) introduces
		//       web search, document search, and task management tools,
		// When migration 000093 seeds capability hooks,
		// Then exactly 4 hooks exist — each targeting a distinct invocation phase:
		//   PostToolUse×2 (cite-web-sources, index-doc-citations)
		//   PreToolUse×1  (validate-search-query)
		//   SessionStart×1 (load-task-context)
		assert.Equal(t, 4, SeedCapabilityHookCount,
			"migration 000093 must seed exactly 4 capability hooks")
		assert.Equal(t, 4, len(SeedCapabilityHookSlugs),
			"slug list must contain exactly 4 entries")

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityHookSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["capability-posttooluse-cite-web-sources"],
			"PostToolUse web citation hook must exist")
		assert.True(t, slugSet["capability-posttooluse-index-doc-citations"],
			"PostToolUse doc citation hook must exist")
		assert.True(t, slugSet["capability-pretooluse-validate-search-query"],
			"PreToolUse query validation hook must exist")
		assert.True(t, slugSet["capability-sessionstart-load-task-context"],
			"SessionStart task context hook must exist")
	})

	t.Run("Scenario_CapabilityHookSlugsDoNotOverlapWithPlatformBaselineHooks", func(t *testing.T) {
		// Given the platform baseline (migration 000010) seeds 11 hooks
		//       (4 safety + 3 lifecycle + 2 context + 2 coordination),
		// When capability hooks are introduced in migration 000093,
		// Then NO capability slug collides with a platform slug — the two
		//      layers are independently versioned and separately queryable.
		platformSet := map[string]bool{}
		for _, s := range SeedExpectedHookSlugs {
			platformSet[s] = true
		}

		for _, slug := range SeedCapabilityHookSlugs {
			assert.False(t, platformSet[slug],
				"capability hook slug %q must NOT collide with platform hook slug (migration 000010)",
				slug)
		}

		// Platform baseline count must remain unchanged.
		assert.Equal(t, 11, len(SeedExpectedHookSlugs),
			"platform hook count must remain 11 after migration 000093")
	})

	t.Run("Scenario_WebResearchHooksUseMatcherToScopeToWebTools", func(t *testing.T) {
		// Given the capability layer provides core-web-search and core-web-fetch
		//       as distinct tool slugs for web research operations,
		// When the web research hook group is inspected,
		// Then it contains exactly 2 hooks — one PostToolUse (citation enforcement)
		//      and one PreToolUse (query validation) — both scoped via matcher
		//      to web tools (not doc-search, not global).
		assert.Equal(t, 2, len(SeedCapabilityWebResearchHookSlugs),
			"web research group must have exactly 2 hooks (cite + validate)")

		webSet := map[string]bool{}
		for _, s := range SeedCapabilityWebResearchHookSlugs {
			webSet[s] = true
		}
		assert.True(t, webSet["capability-posttooluse-cite-web-sources"],
			"cite-web-sources must be in the web research hook group")
		assert.True(t, webSet["capability-pretooluse-validate-search-query"],
			"validate-search-query must be in the web research hook group")

		// Doc-search hook must NOT be in the web research group.
		assert.False(t, webSet["capability-posttooluse-index-doc-citations"],
			"index-doc-citations is a doc hook, NOT a web research hook")
	})

	t.Run("Scenario_CapabilityHookEventsAreDistinctFromPlatformEventBaseline", func(t *testing.T) {
		// Given the runner recognises lifecycle events from ExtendedHookEvent
		//       (PreToolUse, PostToolUse, SessionStart, PostToolUseFailure, etc.),
		// When the SeedCapabilityHookEvents constant is inspected,
		// Then all three events are present: PostToolUse, PreToolUse, SessionStart.
		//
		// NOTE: The platform baseline (migration 000010) uses PostToolUseFailure
		// but NOT PostToolUse. Capability hooks introduce PostToolUse as a new
		// event target — both are valid ExtendedHookEvent values; they are distinct.
		assert.Len(t, SeedCapabilityHookEvents, 3,
			"capability hooks use exactly 3 distinct events")

		eventSet := map[string]bool{}
		for _, e := range SeedCapabilityHookEvents {
			eventSet[e] = true
		}
		assert.True(t, eventSet["PostToolUse"], "PostToolUse must be in capability hook events")
		assert.True(t, eventSet["PreToolUse"], "PreToolUse must be in capability hook events")
		assert.True(t, eventSet["SessionStart"], "SessionStart must be in capability hook events")

		// PostToolUseFailure is in the platform set but NOT in capability events.
		// PostToolUse (success path) is in capability events but NOT in the platform set.
		// Both are valid — they cover complementary lifecycle moments.
		platformEventSet := map[string]bool{}
		for _, e := range SeedExpectedHookEvents {
			platformEventSet[e] = true
		}
		assert.True(t, platformEventSet["PostToolUseFailure"],
			"platform uses PostToolUseFailure (failure path) — distinct from PostToolUse")
		assert.False(t, platformEventSet["PostToolUse"],
			"PostToolUse (success path) is introduced by capability hooks in 000093, not in platform 000010")

		// PreToolUse and SessionStart ARE shared between platform and capability layers.
		assert.True(t, platformEventSet["PreToolUse"],
			"PreToolUse is used by both platform hooks and capability hooks")
		assert.True(t, platformEventSet["SessionStart"],
			"SessionStart is used by both platform hooks and capability hooks")
	})

	t.Run("Scenario_HookSubgroupsPartitionAllFourSlugsWithNoGapsOrOverlaps", func(t *testing.T) {
		// Given three functional sub-groups of capability hooks:
		//   web research (2), doc analysis (1), session lifecycle (1),
		// When the sub-group slices are unioned,
		// Then they cover exactly the 4 slugs in SeedCapabilityHookSlugs
		//      with no duplicates and no missing entries — a clean partition.
		allGroups := map[string]int{}
		for _, s := range SeedCapabilityWebResearchHookSlugs {
			allGroups[s]++
		}
		for _, s := range SeedCapabilityDocAnalysisHookSlugs {
			allGroups[s]++
		}
		for _, s := range SeedCapabilitySessionHookSlugs {
			allGroups[s]++
		}

		assert.Equal(t, SeedCapabilityHookCount, len(allGroups),
			"sub-groups combined must cover exactly %d distinct slugs", SeedCapabilityHookCount)

		for _, slug := range SeedCapabilityHookSlugs {
			count := allGroups[slug]
			assert.Equal(t, 1, count,
				"slug %q must appear in exactly 1 sub-group (got %d)", slug, count)
		}
	})
}
