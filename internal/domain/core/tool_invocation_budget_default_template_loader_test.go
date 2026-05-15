package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTIBTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedTIBTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedTIBTemplateRowCount)
}

func TestCoreTIBTemplate_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedTIBTemplateSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreTIBTemplate_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedTIBTemplateSlugs {
		assert.True(t, SeedTIBTemplateSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreTIBTemplate_PoliciesCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedTIBTemplatePolicies))
}

func TestCoreTIBTemplate_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedTIBTemplateRowCount, len(SeedExpectedTIBTemplateSlugs))
}

func TestCoreTIBTemplate_ReadOnlySlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedTIBTemplateSlugs {
		if s == SeedTIBReadOnlySlug {
			found = true
		}
	}
	assert.True(t, found, "compliance-audit must be in slug list")
}

func TestCoreTIBTemplate_UnlimitedSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedTIBTemplateSlugs {
		if s == SeedTIBUnlimitedSlug {
			found = true
		}
	}
	assert.True(t, found, "unlimited must be in slug list")
}
