package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSkill_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSkillSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreSkill_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedSkillSlugs {
		assert.NotEmpty(t, s, "seed slug at %d must be non-empty", i)
	}
}

func TestCoreSkill_SeedExpectedSlugs_AllUseCorePrefix(t *testing.T) {
	for _, s := range SeedExpectedSkillSlugs {
		assert.True(t, strings.HasPrefix(s, SeedExpectedSkillSlugPrefix),
			"slug %q must start with %q", s, SeedExpectedSkillSlugPrefix)
	}
}

func TestCoreSkill_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 7 skills in canonical seed.
	assert.Equal(t, 7, len(SeedExpectedSkillSlugs),
		"7 skills in canonical seed (refactor must update migration too)")
}

func TestCoreSkill_SeedExpectedCategory_IsPlatform(t *testing.T) {
	// Seeded skills are platform-mgmt skills. Tenants can register
	// other categories themselves.
	assert.Equal(t, "platform", SeedExpectedSkillCategory,
		"seed skills are platform category by contract")
}

func TestCoreSkill_SeedExpectedContextMode_IsInline(t *testing.T) {
	// inline = instructions injected into system prompt; the default
	// for platform-mgmt skills.
	assert.Equal(t, "inline", SeedExpectedSkillContextMode)
}

func TestCoreSkill_SlugsAreFilesystemAndURLSafe(t *testing.T) {
	for _, s := range SeedExpectedSkillSlugs {
		for _, r := range s {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok, "slug %q has invalid char %q", s, r)
		}
	}
}

func TestCoreSkill_AllSkillsCoverManagementSurfaces(t *testing.T) {
	// Every seed skill should be a *-management skill (or the special
	// platform-settings one). This is an audit-time guarantee that the
	// seed is purpose-scoped.
	for _, s := range SeedExpectedSkillSlugs {
		isManagement := strings.Contains(s, "-management")
		isSettings := s == "core-platform-settings"
		assert.True(t, isManagement || isSettings,
			"slug %q must be a -management or -settings skill", s)
	}
}
