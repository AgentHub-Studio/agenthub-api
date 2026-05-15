package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreLocaleTranslation_LocalesNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range SeedExpectedLocales {
		assert.False(t, seen[l])
		seen[l] = true
	}
}

func TestCoreLocaleTranslation_LocalesCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedLocales))
}

func TestCoreLocaleTranslation_LocalesUseBCP47Format(t *testing.T) {
	// BCP 47: language-region (e.g. en-US, pt-BR).
	for _, l := range SeedExpectedLocales {
		parts := strings.Split(l, "-")
		assert.Len(t, parts, 2, "locale %q must be BCP 47 (lang-REGION)", l)
		assert.Len(t, parts[0], 2, "language code must be 2 chars")
		assert.Len(t, parts[1], 2, "region code must be 2 chars (uppercase)")
		assert.Equal(t, strings.ToUpper(parts[1]), parts[1], "region must be uppercase")
	}
}

func TestCoreLocaleTranslation_CategoriesCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedTranslationCategories))
}

func TestCoreLocaleTranslation_KeysCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedTranslationKeys))
}

func TestCoreLocaleTranslation_KeysFollowDottedPathConvention(t *testing.T) {
	for _, k := range SeedExpectedTranslationKeys {
		assert.Contains(t, k, ".",
			"key %q must use category.name dotted path", k)
	}
}

func TestCoreLocaleTranslation_KeyPrefixMatchesCategory(t *testing.T) {
	allowedCats := map[string]bool{}
	for _, c := range SeedExpectedTranslationCategories {
		allowedCats[c] = true
	}
	for _, k := range SeedExpectedTranslationKeys {
		idx := strings.Index(k, ".")
		require := assert.New(t)
		require.Greater(idx, 0)
		prefix := k[:idx]
		assert.True(t, allowedCats[prefix],
			"key %q has prefix %q outside allowed categories", k, prefix)
	}
}

func TestCoreLocaleTranslation_DefaultLocaleIsEnglishUS(t *testing.T) {
	assert.Equal(t, "en-US", SeedDefaultLocale)

	set := map[string]bool{}
	for _, l := range SeedExpectedLocales {
		set[l] = true
	}
	assert.True(t, set[SeedDefaultLocale],
		"default locale must be in seed")
}

func TestCoreLocaleTranslation_RowCountIsFourLocalesTimesSixKeys(t *testing.T) {
	assert.Equal(t, 4*6, SeedExpectedTranslationRowCount)
}
