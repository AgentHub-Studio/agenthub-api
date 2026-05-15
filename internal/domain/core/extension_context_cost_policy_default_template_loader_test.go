package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreECPTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedECPTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedECPTemplateRowCount)
}

func TestCoreECPTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedECPTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreECPTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedECPTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreECPTemplate_CategoriesMatchEXT010EnumByteForByte(t *testing.T) {
	// Cross-feature invariant: every category must match EXT-010
	// ExtensionContextCostCategory bounded enum byte-for-byte.
	expected := map[string]bool{
		"micro": true, "small": true, "medium": true,
		"large": true, "heavy": true,
	}
	for _, c := range SeedExpectedECPTemplateCategories {
		assert.True(t, expected[c], "category %q outside EXT-010 enum", c)
	}
	assert.Equal(t, 5, len(SeedExpectedECPTemplateCategories))
}

func TestCoreECPTemplate_CategoriesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range SeedExpectedECPTemplateCategories {
		assert.False(t, seen[c])
		seen[c] = true
	}
}

func TestCoreECPTemplate_ExtensionKindsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"readonly_lookup": true, "curated_search": true,
		"rag_bundle": true, "coding_suite": true,
		"multimodal_vision": true,
	}
	for _, k := range SeedExpectedECPTemplateExtensionKinds {
		assert.True(t, expected[k], "extension_kind %q outside closed set", k)
	}
	assert.Equal(t, 5, len(SeedExpectedECPTemplateExtensionKinds))
}

func TestCoreECPTemplate_ExtensionKindsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedECPTemplateExtensionKinds {
		assert.False(t, seen[k])
		seen[k] = true
	}
}

func TestCoreECPTemplate_AllRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedECPTemplateSlugs,
		SeedRecommendedECPTemplateSlugs)
}

func TestCoreECPTemplate_OneToOneCategoryToTemplate(t *testing.T) {
	assert.Equal(t, len(SeedExpectedECPTemplateCategories), SeedExpectedECPTemplateRowCount)
}

func TestCoreECPTemplate_OneToOneExtensionKindToCategory(t *testing.T) {
	// Each category exemplifies a distinct extension_kind (no two
	// categories share the same exemplar archetype).
	assert.Equal(t,
		len(SeedExpectedECPTemplateExtensionKinds),
		len(SeedExpectedECPTemplateCategories))
}

func TestCoreECPTemplate_BandBoundariesMatchEXT010Constants(t *testing.T) {
	// These constants are mirrored from EXT-010 ExtensionContextCost*MaxPerTurn
	// for cross-validation in integration tests.
	assert.Equal(t, 100, SeedEXT010MicroMaxPerTurn)
	assert.Equal(t, 500, SeedEXT010SmallMaxPerTurn)
	assert.Equal(t, 2000, SeedEXT010MediumMaxPerTurn)
	assert.Equal(t, 8000, SeedEXT010LargeMaxPerTurn)
}
