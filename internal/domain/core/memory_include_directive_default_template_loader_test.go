package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreMIDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedMIDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedMIDTemplateRowCount)
}

func TestCoreMIDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedMIDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreMIDTemplate_IncludeKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedMIDTemplateIncludeKeys {
		assert.False(t, seen[k])
		seen[k] = true
	}
}

func TestCoreMIDTemplate_IncludeKeysMatchCTX007Regex(t *testing.T) {
	// Cross-feature invariant: every include_key must satisfy the
	// CTX-007 includePattern key regex (otherwise the resolver will
	// never match the inline @include{key}).
	for _, k := range SeedExpectedMIDTemplateIncludeKeys {
		assert.True(t, MemoryIncludeKeyRE.MatchString(k),
			"include_key %q must match CTX-007 [a-zA-Z0-9._-]+", k)
	}
}

func TestCoreMIDTemplate_CategoriesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"organization": true, "citation": true, "code_style": true,
		"safety": true, "locale": true, "handoff": true,
	}
	for _, c := range SeedExpectedMIDTemplateCategories {
		assert.True(t, expected[c], "category %q outside closed set", c)
	}
	assert.Equal(t, 6, len(SeedExpectedMIDTemplateCategories))
}

func TestCoreMIDTemplate_CategoriesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range SeedExpectedMIDTemplateCategories {
		assert.False(t, seen[c])
		seen[c] = true
	}
}

func TestCoreMIDTemplate_PlaceholderSubsetIsOrgOnly(t *testing.T) {
	assert.Equal(t,
		[]string{"core-org-identity"},
		SeedPlaceholderMIDTemplateSlugs,
		"only core-org-identity should declare placeholders by default")
}

func TestCoreMIDTemplate_AllRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedMIDTemplateSlugs,
		SeedRecommendedMIDTemplateSlugs)
}

func TestCoreMIDTemplate_SlugCountEqualsCategoryCount(t *testing.T) {
	// 1:1 invariant: each category has exactly one starter directive.
	assert.Equal(t,
		len(SeedExpectedMIDTemplateCategories),
		SeedExpectedMIDTemplateRowCount)
}

func TestCoreMIDTemplate_IncludeKeyRegexAllowsExpectedChars(t *testing.T) {
	// Sanity: regex accepts dot/underscore/dash as needed by seed keys.
	cases := map[string]bool{
		"core.org.identity":     true,
		"core_safety":           true,
		"core-code":             true,
		"core_123":              true,
		"with space":            false,
		"with/slash":            false,
		"with@at":               false,
		"":                      false,
	}
	for input, expected := range cases {
		got := MemoryIncludeKeyRE.MatchString(input)
		assert.Equal(t, expected, got, "MemoryIncludeKeyRE(%q)", input)
	}
}
