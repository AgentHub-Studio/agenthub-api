package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability KB template seed constants (migration 000096).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityKBTemplateCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityKBTemplateCount, len(SeedCapabilityKBTemplateSlugs),
		"SeedCapabilityKBTemplateCount must match len(SeedCapabilityKBTemplateSlugs)")
}

func TestSeedCapabilityKBTemplateSlugs_CountIsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityKBTemplateCount,
		"migration 000096 seeds exactly 3 capability KB templates (one per capability agent)")
	assert.Equal(t, 3, len(SeedCapabilityKBTemplateSlugs),
		"slug list must have exactly 3 entries matching SeedCapabilityKBTemplateCount")
}

func TestSeedCapabilityKBTemplateSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityKBTemplateSlugs {
		assert.False(t, seen[slug], "duplicate capability KB template slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityKBTemplateSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedCapabilityKBTemplateSlugs {
		assert.NotEmpty(t, slug, "capability KB template slug at index %d must be non-empty", i)
	}
}

func TestSeedCapabilityKBTemplateSlugs_AllEndWithTemplateSuffix(t *testing.T) {
	for _, slug := range SeedCapabilityKBTemplateSlugs {
		assert.True(t, strings.HasSuffix(slug, "-template"),
			"capability KB template slug %q must end with -template (namespace contract)", slug)
	}
}

func TestSeedCapabilityKBTemplateKinds_CountIsThree(t *testing.T) {
	assert.Equal(t, 3, len(SeedCapabilityKBTemplateKinds),
		"3 new capability kinds: research_collection / analysis_workspace / project_notes")
}

func TestSeedCapabilityKBTemplateKinds_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedCapabilityKBTemplateKinds {
		assert.False(t, seen[k], "duplicate capability KB template kind %q", k)
		seen[k] = true
	}
}

func TestSeedCapabilityKBTemplateKinds_AllNonEmpty(t *testing.T) {
	for i, k := range SeedCapabilityKBTemplateKinds {
		assert.NotEmpty(t, k, "capability KB template kind at index %d must be non-empty", i)
	}
}

func TestSeedCapabilityKBTemplateKinds_NoOverlapWithPlatformKinds(t *testing.T) {
	// Capability kinds must not collide with the 7 platform kinds seeded in
	// migration 000017 — they represent distinct use cases.
	platformSet := map[string]bool{}
	for _, k := range SeedExpectedKBTemplateKinds {
		platformSet[k] = true
	}
	for _, kind := range SeedCapabilityKBTemplateKinds {
		assert.False(t, platformSet[kind],
			"capability KB template kind %q must NOT collide with a platform kind (migration 000017)",
			kind)
	}
}

func TestSeedCapabilityKBTemplateSlugs_NoOverlapWithPlatformSeven(t *testing.T) {
	// Capability slugs must not collide with the 7 platform KB template slugs
	// seeded in migration 000017.
	platformSet := map[string]bool{}
	for _, s := range SeedExpectedKBTemplateSlugs {
		platformSet[s] = true
	}
	for _, slug := range SeedCapabilityKBTemplateSlugs {
		assert.False(t, platformSet[slug],
			"capability KB template slug %q must NOT collide with platform KB template (migration 000017)",
			slug)
	}
}

func TestSeedCapabilityKBTemplateKinds_AllAreNewKinds(t *testing.T) {
	// Verify each expected new kind is present in the capability kinds list.
	kindSet := map[string]bool{}
	for _, k := range SeedCapabilityKBTemplateKinds {
		kindSet[k] = true
	}
	assert.True(t, kindSet["research_collection"], "research_collection must be a capability kind")
	assert.True(t, kindSet["analysis_workspace"], "analysis_workspace must be a capability kind")
	assert.True(t, kindSet["project_notes"], "project_notes must be a capability kind")
}

func TestSeedResearchKBTemplateSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityKBTemplateSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedResearchKBTemplateSlug],
		"SeedResearchKBTemplateSlug %q must be in SeedCapabilityKBTemplateSlugs",
		SeedResearchKBTemplateSlug)
}

func TestSeedAnalysisKBTemplateSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityKBTemplateSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedAnalysisKBTemplateSlug],
		"SeedAnalysisKBTemplateSlug %q must be in SeedCapabilityKBTemplateSlugs",
		SeedAnalysisKBTemplateSlug)
}

func TestSeedPlannerKBTemplateSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityKBTemplateSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedPlannerKBTemplateSlug],
		"SeedPlannerKBTemplateSlug %q must be in SeedCapabilityKBTemplateSlugs",
		SeedPlannerKBTemplateSlug)
}
