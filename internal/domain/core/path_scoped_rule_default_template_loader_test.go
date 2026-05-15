package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePSRDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedPSRDTemplateSlugs))
	assert.Equal(t, 8, SeedExpectedPSRDTemplateRowCount)
}

func TestCorePSRDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPSRDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePSRDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPSRDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePSRDTemplate_ScopesMatchCTX004Enum(t *testing.T) {
	expected := map[string]bool{
		"global": true, "tool": true, "file": true,
		"directory": true, "agent": true,
	}
	for _, s := range SeedExpectedPSRDTemplateScopes {
		assert.True(t, expected[s], "scope %q outside CTX-004 enum", s)
	}
	assert.Equal(t, 5, len(SeedExpectedPSRDTemplateScopes))
}

func TestCorePSRDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "regulated": true}
	for _, k := range SeedExpectedPSRDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCorePSRDTemplate_RecommendedAndCanonicalAreEqual(t *testing.T) {
	// Each implements a documented pattern; tenants opt in per template.
	assert.ElementsMatch(t,
		SeedExpectedPSRDTemplateSlugs,
		SeedRecommendedPSRDTemplateSlugs)
}

func TestCorePSRDTemplate_AdminReviewSubsetIsHighImpact(t *testing.T) {
	expected := []string{
		"no-pii-in-yaml-config",
		"shell-commands-no-rm-rf",
		"migrations-no-data-loss",
	}
	assert.ElementsMatch(t, expected, SeedAdminReviewPSRDTemplateSlugs)
}

func TestCorePSRDTemplate_AdminReviewExcludesLowImpact(t *testing.T) {
	// Conventions like docs-frontmatter and citation rules are routine
	// and don't require admin sign-off.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewPSRDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["docs-must-have-frontmatter"])
	assert.False(t, set["researcher-must-cite-sources"])
}
