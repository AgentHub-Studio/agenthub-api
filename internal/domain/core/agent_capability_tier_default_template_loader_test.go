package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCapabilityTier_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedCapabilityTierSlugs))
	assert.Equal(t, 5, SeedExpectedCapabilityTierRowCount)
}

func TestCoreCapabilityTier_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCapabilityTierSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreCapabilityTier_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedCapabilityTierSlugs {
		assert.True(t, SeedCapabilityTierSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreCapabilityTier_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedCapabilityTierRowCount, len(SeedExpectedCapabilityTierSlugs))
}

func TestCoreCapabilityTier_DefaultSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedCapabilityTierSlugs {
		if s == SeedCapabilityTierDefaultSlug {
			found = true
		}
	}
	assert.True(t, found, "standard must be in slug list as the default")
}

func TestCoreCapabilityTier_GovernedSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedCapabilityTierSlugs {
		if s == SeedCapabilityTierGovernedSlug {
			found = true
		}
	}
	assert.True(t, found, "governed must be in slug list")
}

func TestCoreCapabilityTier_ReadOnlySlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedCapabilityTierSlugs {
		if s == SeedCapabilityTierReadOnlySlug {
			found = true
		}
	}
	assert.True(t, found, "read-only must be in slug list")
}

func TestCoreCapabilityTier_PermissionModesCount(t *testing.T) {
	assert.Equal(t, 2, len(SeedCapabilityTierPermissionModes))
}

func TestCoreCapabilityTier_GovernanceLevelsCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedCapabilityTierGovernanceLevels))
}

func TestCoreCapabilityTier_GovernanceLevelsCoverAllVariants(t *testing.T) {
	levelSet := map[string]bool{}
	for _, l := range SeedCapabilityTierGovernanceLevels {
		levelSet[l] = true
	}
	assert.True(t, levelSet["none"])
	assert.True(t, levelSet["standard"])
	assert.True(t, levelSet["strict"])
}

func TestCoreCapabilityTier_ToolAccessLevelsCoverAll(t *testing.T) {
	levelSet := map[string]bool{}
	for _, l := range SeedCapabilityTierToolAccessLevels {
		levelSet[l] = true
	}
	assert.True(t, levelSet["read-only"])
	assert.True(t, levelSet["standard"])
	assert.True(t, levelSet["full"])
}
