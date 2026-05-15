package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreAgentRolePreset_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedAgentRolePresetSlugs))
	assert.Equal(t, 5, SeedExpectedAgentRolePresetRowCount)
}

func TestCoreAgentRolePreset_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedAgentRolePresetSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreAgentRolePreset_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedAgentRolePresetSlugs {
		assert.True(t, SeedAgentRolePresetSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreAgentRolePreset_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedAgentRolePresetRowCount, len(SeedExpectedAgentRolePresetSlugs))
}

func TestCoreAgentRolePreset_DefaultSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedAgentRolePresetSlugs {
		if s == SeedAgentRoleDefaultSlug {
			found = true
		}
	}
	assert.True(t, found, "general-assistant must be in slug list as the default")
}

func TestCoreAgentRolePreset_PlannerSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedAgentRolePresetSlugs {
		if s == SeedAgentRolePlannerSlug {
			found = true
		}
	}
	assert.True(t, found, "planner must be in slug list")
}

func TestCoreAgentRolePreset_ReadOnlySlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedAgentRolePresetSlugs {
		if s == SeedAgentRoleReadOnlySlug {
			found = true
		}
	}
	assert.True(t, found, "read-researcher must be in slug list")
}

func TestCoreAgentRolePreset_SourceTypesCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedAgentRoleSourceTypes),
		"5 of the 6 Claude Code built-in types are mapped (statusline-setup→NOT_APPLICABLE_WEB)")
}

func TestCoreAgentRolePreset_SourceTypesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedAgentRoleSourceTypes {
		assert.False(t, seen[s], "duplicate source type %q", s)
		seen[s] = true
	}
}
