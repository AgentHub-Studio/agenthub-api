package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedCapabilityAgentCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityAgentCount, len(SeedCapabilityAgentSlugs),
		"SeedCapabilityAgentCount must match len(SeedCapabilityAgentSlugs)")
}

func TestSeedCapabilityAgentSlugs_ContainsResearcher(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentSlugs, "core-researcher")
}

func TestSeedCapabilityAgentSlugs_ContainsAnalyst(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentSlugs, "core-analyst")
}

func TestSeedCapabilityAgentSlugs_ContainsPlanner(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentSlugs, "core-planner")
}

func TestSeedCapabilityAgentSlugs_AllUseCorePrefixNamespace(t *testing.T) {
	for _, slug := range SeedCapabilityAgentSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"),
			"capability agent slug %q must start with core-", slug)
	}
}

func TestSeedCapabilityAgentType_IsAssistant(t *testing.T) {
	assert.Equal(t, "ASSISTANT", SeedCapabilityAgentType,
		"all capability agents must be ASSISTANT type — they are interactive, user-facing")
}

func TestSeedCapabilityAgentSkillBindingCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityAgentSkillBindingCount,
		"1 skill per agent × 3 agents = 3 bindings")
}

func TestSeedCapabilityAgentSkillBindingCount_EqualsAgentCount(t *testing.T) {
	// Each capability agent binds exactly one skill.
	assert.Equal(t, SeedCapabilityAgentCount, SeedCapabilityAgentSkillBindingCount,
		"binding count must equal agent count (1:1 agent-to-skill mapping)")
}

func TestSeedResearcherSkillSlug_IsInCapabilitySkillSlugs(t *testing.T) {
	found := false
	for _, slug := range SeedCapabilitySkillSlugs {
		if slug == SeedResearcherSkillSlug {
			found = true
			break
		}
	}
	assert.True(t, found,
		"SeedResearcherSkillSlug %q must reference a skill seeded in migration 000090",
		SeedResearcherSkillSlug)
}

func TestSeedAnalystSkillSlug_IsInCapabilitySkillSlugs(t *testing.T) {
	found := false
	for _, slug := range SeedCapabilitySkillSlugs {
		if slug == SeedAnalystSkillSlug {
			found = true
			break
		}
	}
	assert.True(t, found,
		"SeedAnalystSkillSlug %q must reference a skill seeded in migration 000090",
		SeedAnalystSkillSlug)
}

func TestSeedPlannerSkillSlug_IsInCapabilitySkillSlugs(t *testing.T) {
	found := false
	for _, slug := range SeedCapabilitySkillSlugs {
		if slug == SeedPlannerSkillSlug {
			found = true
			break
		}
	}
	assert.True(t, found,
		"SeedPlannerSkillSlug %q must reference a skill seeded in migration 000090",
		SeedPlannerSkillSlug)
}

func TestSeedCapabilityAgentSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityAgentSlugs {
		assert.False(t, seen[slug], "duplicate capability agent slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityAgentSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedCapabilityAgentSlugs {
		assert.NotEmpty(t, slug, "capability agent slug at index %d must be non-empty", i)
	}
}

func TestSeedCapabilityAgentSlugs_AreFilesystemAndURLSafe(t *testing.T) {
	for _, slug := range SeedCapabilityAgentSlugs {
		for _, r := range slug {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok, "capability agent slug %q has invalid char %q", slug, r)
		}
	}
}

func TestSeedCapabilityAgentSlugs_DistinctFromSpecialistSlugs(t *testing.T) {
	specialistSet := map[string]bool{}
	for _, s := range SeedExpectedAgentSlugs {
		specialistSet[s] = true
	}
	for _, slug := range SeedCapabilityAgentSlugs {
		assert.False(t, specialistSet[slug],
			"capability agent slug %q must not collide with specialist agent slugs", slug)
	}
}
