package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreAFPT_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedAFPTSlugs))
	assert.Equal(t, 6, SeedExpectedAFPTRowCount)
}

func TestCoreAFPT_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedAFPTSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreAFPT_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedAFPTSlugs {
		assert.True(t, SeedAFPTSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreAFPT_HighPrioritySlugsSubset(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedAFPTSlugs {
		all[s] = true
	}
	for _, s := range SeedHighPrioritySlugs {
		assert.True(t, all[s], "high-priority slug %q not in all slugs", s)
	}
}

func TestCoreAFPT_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedAFPTRowCount, len(SeedExpectedAFPTSlugs))
}

func TestCoreAFPT_SecurityAuditorInHighPriority(t *testing.T) {
	found := false
	for _, s := range SeedHighPrioritySlugs {
		if s == "security-auditor" {
			found = true
		}
	}
	assert.True(t, found, "security-auditor must be in high-priority slugs")
}
