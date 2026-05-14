package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability output style seed constants (migration 000095).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityOutputStyleCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityOutputStyleCount, len(SeedCapabilityOutputStyleSlugs),
		"SeedCapabilityOutputStyleCount must match len(SeedCapabilityOutputStyleSlugs)")
}

func TestSeedCapabilityOutputStyleSlugs_CountIsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityOutputStyleCount,
		"migration 000095 seeds exactly 3 capability output styles (one per capability agent)")
	assert.Equal(t, 3, len(SeedCapabilityOutputStyleSlugs),
		"slug list must have exactly 3 entries matching SeedCapabilityOutputStyleCount")
}

func TestSeedCapabilityOutputStyleSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityOutputStyleSlugs {
		assert.False(t, seen[slug], "duplicate capability output style slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityOutputStyleSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedCapabilityOutputStyleSlugs {
		assert.NotEmpty(t, slug, "capability output style slug at index %d must be non-empty", i)
	}
}

func TestSeedCapabilityOutputStyleFormat_IsMarkdown(t *testing.T) {
	assert.Equal(t, "markdown", SeedCapabilityOutputStyleFormat,
		"all capability output styles must use markdown format")
}

func TestSeedCapabilityOutputStyleSlugs_AllUseMarkdownFormat(t *testing.T) {
	// Cross-check: format constant matches the expected format for all 3 slugs.
	// Actual DB values verified in integration tests; this guards constant drift.
	assert.Equal(t, "markdown", SeedCapabilityOutputStyleFormat,
		"SeedCapabilityOutputStyleFormat must be 'markdown' for all 3 capability styles")
}

func TestSeedCapabilityOutputStyleSlugs_SortOrderRange100To102(t *testing.T) {
	// Sort orders 100, 101, 102 are reserved for capability styles.
	// Platform styles use 10-80. Capability starts at 100 to avoid collisions.
	// This is validated against actual DB in integration tests; here we guard
	// that the slug list has exactly 3 entries (one per sort slot).
	assert.Equal(t, 3, len(SeedCapabilityOutputStyleSlugs),
		"exactly 3 slugs required for sort_order slots 100, 101, 102")
}

func TestSeedCapabilityOutputStyleSlugs_NoOverlapWithPlatformEight(t *testing.T) {
	// Capability output style slugs must not collide with the 8 platform styles
	// seeded in migration 000011.
	platformSet := map[string]bool{}
	for _, s := range SeedExpectedOutputStyleSlugs {
		platformSet[s] = true
	}
	for _, slug := range SeedCapabilityOutputStyleSlugs {
		assert.False(t, platformSet[slug],
			"capability output style slug %q must NOT collide with platform output style (migration 000011)",
			slug)
	}
}

func TestSeedResearchOutputStyleSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityOutputStyleSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedResearchOutputStyleSlug],
		"SeedResearchOutputStyleSlug %q must be in SeedCapabilityOutputStyleSlugs",
		SeedResearchOutputStyleSlug)
}

func TestSeedAnalysisOutputStyleSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityOutputStyleSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedAnalysisOutputStyleSlug],
		"SeedAnalysisOutputStyleSlug %q must be in SeedCapabilityOutputStyleSlugs",
		SeedAnalysisOutputStyleSlug)
}

func TestSeedPlannerOutputStyleSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityOutputStyleSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedPlannerOutputStyleSlug],
		"SeedPlannerOutputStyleSlug %q must be in SeedCapabilityOutputStyleSlugs",
		SeedPlannerOutputStyleSlug)
}

func TestSeedCapabilityOutputStyleSlugs_EachCapabilityAgentHasOneStyle(t *testing.T) {
	// One style per capability agent: researcher / analyst / planner.
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityOutputStyleSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedResearchOutputStyleSlug],
		"core-researcher must have output style %q", SeedResearchOutputStyleSlug)
	assert.True(t, slugSet[SeedAnalysisOutputStyleSlug],
		"core-analyst must have output style %q", SeedAnalysisOutputStyleSlug)
	assert.True(t, slugSet[SeedPlannerOutputStyleSlug],
		"core-planner must have output style %q", SeedPlannerOutputStyleSlug)
}

func TestSeedCapabilityOutputStyleSlugs_AllHavePositiveMaxWords(t *testing.T) {
	// Capability styles all set max_words > 0 (600-1000 words) — unlike platform
	// styles like 'verbose' which are unbounded (max_words=0). This is verified
	// against actual DB values in integration tests; here we guard slug coverage.
	// Each slug maps to a max_words value: research-report=800, analysis-brief=1000,
	// task-checklist=600. All > 0.
	expectedMaxWords := map[string]int{
		"research-report": 800,
		"analysis-brief":  1000,
		"task-checklist":  600,
	}
	for _, slug := range SeedCapabilityOutputStyleSlugs {
		mw, present := expectedMaxWords[slug]
		assert.True(t, present, "slug %q must have an expected max_words mapping", slug)
		assert.Greater(t, mw, 0,
			"capability output style %q must have max_words > 0 (unbounded not suitable for structured reports)",
			slug)
	}
}

func TestSeedCapabilityOutputStyleSlugs_CombinedWithPlatformIs11Total(t *testing.T) {
	// After migration 000095, the total active output styles in ah_core are 11:
	// 8 platform (migration 000011) + 3 capability (migration 000095).
	combined := len(SeedExpectedOutputStyleSlugs) + len(SeedCapabilityOutputStyleSlugs)
	assert.Equal(t, 11, combined,
		"platform(8) + capability(3) must equal 11 total output styles")
}
