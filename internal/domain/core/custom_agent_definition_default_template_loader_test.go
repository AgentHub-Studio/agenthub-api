package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCADTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedCADTemplateSlugs))
	assert.Equal(t, 4, SeedExpectedCADTemplateRowCount)
}

func TestCoreCADTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCADTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCADTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCADTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreCADTemplate_ShapeKindsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"bare": true, "researcher_derived": true,
		"derived": true, "dual_loop": true,
	}
	for _, k := range SeedExpectedCADTemplateShapeKinds {
		assert.True(t, expected[k], "shape_kind %q outside closed set", k)
	}
	assert.Equal(t, 4, len(SeedExpectedCADTemplateShapeKinds))
}

func TestCoreCADTemplate_ToolsetRefsAreSUB005Slugs(t *testing.T) {
	expected := map[string]bool{
		"documentation-readonly-allowlist": true,
		"code-write-scoped":                true,
	}
	for _, s := range SeedExpectedCADTemplateToolsetSlugs {
		assert.True(t, expected[s])
	}
}

func TestCoreCADTemplate_InheritanceRefsAreSUB006Slugs(t *testing.T) {
	expected := map[string]bool{
		"restrict-to-readonly": true,
		"extend-parent-rights": true,
	}
	for _, s := range SeedExpectedCADTemplateInheritanceSlugs {
		assert.True(t, expected[s])
	}
}

func TestCoreCADTemplate_SummaryRefsAreSUB010Slugs(t *testing.T) {
	expected := map[string]bool{
		"structured-findings":    true,
		"success-with-artifacts": true,
		"plan-only":              true,
	}
	for _, s := range SeedExpectedCADTemplateSummarySlugs {
		assert.True(t, expected[s])
	}
}

func TestCoreCADTemplate_LineageRefsAreSUB002Slugs(t *testing.T) {
	expected := map[string]bool{
		"researcher-baseline": true,
		"coder-baseline":      true,
		"planner-baseline":    true,
	}
	for _, s := range SeedExpectedCADTemplateBuiltinLineageSlugs {
		assert.True(t, expected[s], "lineage slug %q outside SUB-002 expected refs", s)
	}
}

func TestCoreCADTemplate_VisibilitiesClosedSet(t *testing.T) {
	expected := map[string]bool{"private": true}
	for _, v := range SeedExpectedCADTemplateVisibilities {
		assert.True(t, expected[v])
	}
}

func TestCoreCADTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t, SeedExpectedCADTemplateSlugs, SeedRecommendedCADTemplateSlugs)
}

func TestCoreCADTemplate_GreenfieldOnlyBare(t *testing.T) {
	assert.Equal(t, []string{"bare-greenfield"}, SeedGreenfieldCADTemplateSlugs)
}
