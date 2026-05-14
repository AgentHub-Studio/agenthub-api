package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreEffortLevel_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedEffortLevelSlugs))
	assert.Equal(t, 5, SeedExpectedEffortLevelRowCount)
}

func TestCoreEffortLevel_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedEffortLevelSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreEffortLevel_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedEffortLevelSlugs {
		assert.True(t, SeedEffortLevelSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreEffortLevel_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedEffortLevelRowCount, len(SeedExpectedEffortLevelSlugs))
}

func TestCoreEffortLevel_DefaultSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedEffortLevelSlugs {
		if s == SeedEffortLevelDefaultSlug {
			found = true
		}
	}
	assert.True(t, found, "medium must be in slug list as the default tier")
}

func TestCoreEffortLevel_DisabledSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedEffortLevelSlugs {
		if s == SeedEffortLevelDisabledSlug {
			found = true
		}
	}
	assert.True(t, found, "lowest must be in slug list as the no-thinking tier")
}

func TestCoreEffortLevel_MaxTokensIsPositive(t *testing.T) {
	assert.Greater(t, SeedEffortLevelMaxTokens, 0)
}

func TestCoreEffortLevel_SlugsOrderedByEffort(t *testing.T) {
	expected := []string{"lowest", "low", "medium", "high", "highest"}
	assert.Equal(t, expected, SeedExpectedEffortLevelSlugs,
		"slugs must be ordered lowest→highest for readability and sort_order alignment")
}
