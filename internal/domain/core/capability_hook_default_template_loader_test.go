package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability hook seed constants (migration 000093).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityHookCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityHookCount, len(SeedCapabilityHookSlugs),
		"SeedCapabilityHookCount must match len(SeedCapabilityHookSlugs)")
}

func TestSeedCapabilityHookSlugs_CountIsFour(t *testing.T) {
	assert.Equal(t, 4, SeedCapabilityHookCount,
		"migration 000093 seeds exactly 4 capability hooks")
	assert.Equal(t, 4, len(SeedCapabilityHookSlugs),
		"slug list must have exactly 4 entries matching SeedCapabilityHookCount")
}

func TestSeedCapabilityHookSlugs_ContainsCiteWebSources(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookSlugs, "capability-posttooluse-cite-web-sources",
		"capability hooks must include the PostToolUse web-citation hook")
}

func TestSeedCapabilityHookSlugs_ContainsIndexDocCitations(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookSlugs, "capability-posttooluse-index-doc-citations",
		"capability hooks must include the PostToolUse doc-citation hook")
}

func TestSeedCapabilityHookSlugs_ContainsValidateSearchQuery(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookSlugs, "capability-pretooluse-validate-search-query",
		"capability hooks must include the PreToolUse query-validation hook")
}

func TestSeedCapabilityHookSlugs_ContainsLoadTaskContext(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookSlugs, "capability-sessionstart-load-task-context",
		"capability hooks must include the SessionStart task-context hook")
}

func TestSeedCapabilityHookSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityHookSlugs {
		assert.False(t, seen[slug], "duplicate capability hook slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityHookSlugs_AllUseKebabCase(t *testing.T) {
	// Slugs must use only lowercase letters, digits, and hyphens.
	for _, slug := range SeedCapabilityHookSlugs {
		assert.NotEmpty(t, slug, "slug must not be empty")
		for _, r := range slug {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok,
				"capability hook slug %q has invalid char %q (must be lowercase + digits + hyphen only)",
				slug, r)
		}
	}
}

func TestSeedCapabilityHookSlugs_NoOverlapWithPlatformHooks(t *testing.T) {
	// Capability hook slugs must not clash with the 11 platform baseline hooks.
	platformSet := map[string]bool{}
	for _, s := range SeedExpectedHookSlugs {
		platformSet[s] = true
	}
	for _, slug := range SeedCapabilityHookSlugs {
		assert.False(t, platformSet[slug],
			"capability hook slug %q must NOT collide with platform hook slug (migration 000010)",
			slug)
	}
}

func TestSeedCapabilityHookEvents_ContainsPostToolUse(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookEvents, "PostToolUse",
		"capability hook events must include PostToolUse (cite-web-sources, index-doc-citations)")
}

func TestSeedCapabilityHookEvents_ContainsPreToolUse(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookEvents, "PreToolUse",
		"capability hook events must include PreToolUse (validate-search-query)")
}

func TestSeedCapabilityHookEvents_ContainsSessionStart(t *testing.T) {
	assert.Contains(t, SeedCapabilityHookEvents, "SessionStart",
		"capability hook events must include SessionStart (load-task-context)")
}

func TestSeedCapabilityWebResearchHookSlugs_CountIsTwo(t *testing.T) {
	assert.Equal(t, 2, len(SeedCapabilityWebResearchHookSlugs),
		"web research hook group must have exactly 2 slugs (cite-web + validate-query)")
}

func TestSeedCapabilityWebResearchHookSlugs_ContainsBothWebHooks(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityWebResearchHookSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet["capability-posttooluse-cite-web-sources"],
		"web research hooks must include cite-web-sources (PostToolUse)")
	assert.True(t, slugSet["capability-pretooluse-validate-search-query"],
		"web research hooks must include validate-search-query (PreToolUse)")
}

func TestSeedCapabilityDocAnalysisHookSlugs_CountIsOne(t *testing.T) {
	assert.Equal(t, 1, len(SeedCapabilityDocAnalysisHookSlugs),
		"doc analysis hook group must have exactly 1 slug (index-doc-citations)")
}

func TestSeedCapabilitySessionHookSlugs_CountIsOne(t *testing.T) {
	assert.Equal(t, 1, len(SeedCapabilitySessionHookSlugs),
		"session hook group must have exactly 1 slug (load-task-context)")
}

func TestSeedCapabilityHookSubgroups_CoverAllFourSlugs(t *testing.T) {
	// The three sub-group slices must partition the 4 capability hook slugs.
	// No slug may be absent and no slug may appear in more than one group.
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

	for _, slug := range SeedCapabilityHookSlugs {
		count, present := allGroups[slug]
		assert.True(t, present,
			"slug %q is missing from all sub-group slices — it must appear in exactly one", slug)
		assert.Equal(t, 1, count,
			"slug %q appears in %d sub-groups — must appear in exactly 1", slug, count)
	}
	// Total coverage must equal the full set.
	assert.Equal(t, SeedCapabilityHookCount, len(allGroups),
		"sub-groups combined must cover exactly %d distinct slugs", SeedCapabilityHookCount)
}
