package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability prompt template seed constants (migration 000094).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityPromptTemplateCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityPromptTemplateCount, len(SeedCapabilityPromptTemplateSlugs),
		"SeedCapabilityPromptTemplateCount must match len(SeedCapabilityPromptTemplateSlugs)")
}

func TestSeedCapabilityPromptTemplateSlugs_CountIsFive(t *testing.T) {
	assert.Equal(t, 5, SeedCapabilityPromptTemplateCount,
		"migration 000094 seeds exactly 5 capability prompt templates")
	assert.Equal(t, 5, len(SeedCapabilityPromptTemplateSlugs),
		"slug list must have exactly 5 entries matching SeedCapabilityPromptTemplateCount")
}

func TestSeedCapabilityPromptTemplateSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedCapabilityPromptTemplateSlugs {
		assert.False(t, seen[s], "duplicate capability prompt template slug %q", s)
		seen[s] = true
	}
}

func TestSeedCapabilityPromptTemplateKind_IsCapability(t *testing.T) {
	assert.Equal(t, "capability", SeedCapabilityPromptTemplateKind,
		"kind constant must be 'capability' to distinguish from platform template kinds")
}

func TestSeedCapabilityPromptTemplateSlugs_AllUseKebabCase(t *testing.T) {
	for _, slug := range SeedCapabilityPromptTemplateSlugs {
		assert.NotEmpty(t, slug, "slug must not be empty")
		assert.False(t, strings.Contains(slug, "_"),
			"capability prompt template slug %q must use kebab-case (no underscores)", slug)
		for _, r := range slug {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok,
				"capability prompt template slug %q has invalid char %q (must be lowercase + digits + hyphen only)",
				slug, r)
		}
	}
}

func TestSeedWebResearchPromptSlugs_CountIsTwo(t *testing.T) {
	assert.Equal(t, 2, len(SeedWebResearchPromptSlugs),
		"web research prompt group must have exactly 2 slugs (web-research-brief + competitive-research)")
}

func TestSeedWebResearchPromptSlugs_ContainsBothWebTemplates(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedWebResearchPromptSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet["capability-web-research-brief"],
		"web research prompts must include capability-web-research-brief")
	assert.True(t, slugSet["capability-competitive-research"],
		"web research prompts must include capability-competitive-research")
}

func TestSeedDocAnalysisPromptSlugs_CountIsTwo(t *testing.T) {
	assert.Equal(t, 2, len(SeedDocAnalysisPromptSlugs),
		"doc analysis prompt group must have exactly 2 slugs (doc-analysis-summary + knowledge-synthesis)")
}

func TestSeedDocAnalysisPromptSlugs_ContainsBothDocTemplates(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedDocAnalysisPromptSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet["capability-doc-analysis-summary"],
		"doc analysis prompts must include capability-doc-analysis-summary")
	assert.True(t, slugSet["capability-knowledge-synthesis"],
		"doc analysis prompts must include capability-knowledge-synthesis")
}

func TestSeedTaskWorkflowPromptSlugs_CountIsOne(t *testing.T) {
	assert.Equal(t, 1, len(SeedTaskWorkflowPromptSlugs),
		"task workflow prompt group must have exactly 1 slug (task-breakdown)")
}

func TestSeedTaskWorkflowPromptSlugs_ContainsTaskBreakdown(t *testing.T) {
	assert.Contains(t, SeedTaskWorkflowPromptSlugs, "capability-task-breakdown",
		"task workflow prompts must include capability-task-breakdown")
}

func TestSeedCapabilityPromptTemplateSubgroups_CoverAllFiveSlugs(t *testing.T) {
	// The three sub-group slices must partition the 5 capability prompt template slugs.
	// No slug may be absent and no slug may appear in more than one group.
	allGroups := map[string]int{}
	for _, s := range SeedWebResearchPromptSlugs {
		allGroups[s]++
	}
	for _, s := range SeedDocAnalysisPromptSlugs {
		allGroups[s]++
	}
	for _, s := range SeedTaskWorkflowPromptSlugs {
		allGroups[s]++
	}

	assert.Equal(t, SeedCapabilityPromptTemplateCount, len(allGroups),
		"sub-groups combined must cover exactly %d distinct slugs", SeedCapabilityPromptTemplateCount)

	for _, slug := range SeedCapabilityPromptTemplateSlugs {
		count, present := allGroups[slug]
		assert.True(t, present,
			"slug %q is missing from all sub-group slices — it must appear in exactly one", slug)
		assert.Equal(t, 1, count,
			"slug %q appears in %d sub-groups — must appear in exactly 1", slug, count)
	}
}

func TestSeedCapabilityPromptTemplateSlugs_NoOverlapWithPlatformTemplateSlugs(t *testing.T) {
	// Capability prompt slugs must not clash with the 8 platform baseline templates.
	platformSet := map[string]bool{}
	for _, s := range SeedExpectedPromptTemplateSlugs {
		platformSet[s] = true
	}
	for _, slug := range SeedCapabilityPromptTemplateSlugs {
		assert.False(t, platformSet[slug],
			"capability prompt slug %q must NOT collide with platform prompt slug (migration 000019)",
			slug)
	}
}

func TestSeedCapabilityPromptTemplateKind_NotInPlatformKinds(t *testing.T) {
	// The 'capability' kind must NOT be in the platform template kind set — it is
	// a new kind introduced by migration 000094 for the capability layer.
	platformKindSet := map[string]bool{}
	for _, k := range SeedExpectedPromptTemplateKinds {
		platformKindSet[k] = true
	}
	assert.False(t, platformKindSet[SeedCapabilityPromptTemplateKind],
		"kind 'capability' must NOT overlap with the platform template kinds (migration 000019)")
}
