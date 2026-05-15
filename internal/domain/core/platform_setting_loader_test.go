package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePlatformSetting_SeedExpectedKeys_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedPlatformSettingKeys {
		assert.False(t, seen[k], "duplicate key %q", k)
		seen[k] = true
	}
}

func TestCorePlatformSetting_SeedExpectedKeys_AllNonEmpty(t *testing.T) {
	for i, k := range SeedExpectedPlatformSettingKeys {
		assert.NotEmpty(t, k, "key at %d must be non-empty", i)
	}
}

func TestCorePlatformSetting_SeedExpectedKeys_CanonicalCount(t *testing.T) {
	// 15 settings = 4 runner + 3 evaluator + 1 policy + 2 checkpoint
	// + 2 ui + 2 security + 1 compaction.
	assert.Equal(t, 15, len(SeedExpectedPlatformSettingKeys),
		"15 platform settings in canonical seed")
}

func TestCorePlatformSetting_KeysUseDottedPathConvention(t *testing.T) {
	for _, k := range SeedExpectedPlatformSettingKeys {
		assert.Contains(t, k, ".",
			"key %q must use dotted-path convention (category.name)", k)
	}
}

func TestCorePlatformSetting_KeysCategoryPrefixIsValid(t *testing.T) {
	allowed := map[string]bool{}
	for _, c := range SeedExpectedPlatformSettingCategories {
		allowed[c] = true
	}
	for _, k := range SeedExpectedPlatformSettingKeys {
		idx := strings.Index(k, ".")
		require := assert.New(t)
		require.Greater(idx, 0, "key %q must have category prefix", k)
		prefix := k[:idx]
		assert.True(t, allowed[prefix],
			"key %q has prefix %q outside allowed categories", k, prefix)
	}
}

func TestCorePlatformSetting_CategoriesAreDistinctAndCounted(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range SeedExpectedPlatformSettingCategories {
		assert.False(t, seen[c], "duplicate category %q", c)
		seen[c] = true
	}
	assert.Equal(t, 7, len(SeedExpectedPlatformSettingCategories),
		"7 categories: runner/evaluator/policy/checkpoint/ui/security/compaction")
}

func TestCorePlatformSetting_ValueTypesAreClosed(t *testing.T) {
	allowed := map[string]bool{}
	for _, vt := range SeedExpectedPlatformSettingValueTypes {
		allowed[vt] = true
	}
	for _, want := range []string{"string", "number", "boolean", "json"} {
		assert.True(t, allowed[want], "value type %q must be in allowlist", want)
	}
	assert.Equal(t, 4, len(SeedExpectedPlatformSettingValueTypes))
}

func TestCorePlatformSetting_NonOverridableAreSecurityOnly(t *testing.T) {
	// Non-overridable contract: only security category baselines.
	for _, k := range SeedNonOverridablePlatformSettingKeys {
		assert.True(t, strings.HasPrefix(k, "security."),
			"non-overridable key %q must be security-prefixed", k)
	}
	assert.Equal(t, 2, len(SeedNonOverridablePlatformSettingKeys),
		"2 non-overridable security baselines expected")
}

func TestCorePlatformSetting_NonOverridableAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, k := range SeedExpectedPlatformSettingKeys {
		seedSet[k] = true
	}
	for _, k := range SeedNonOverridablePlatformSettingKeys {
		assert.True(t, seedSet[k],
			"non-overridable key %q must appear in seed", k)
	}
}

func TestCorePlatformSetting_AsBool_RejectsNonBoolean(t *testing.T) {
	s := CorePlatformSetting{Key: "x", Value: "true", ValueType: "string"}
	_, err := s.AsBool()
	assert.Error(t, err, "non-boolean value_type must error")
}

func TestCorePlatformSetting_AsBool_ParsesTrue(t *testing.T) {
	s := CorePlatformSetting{Key: "x", Value: "true", ValueType: "boolean"}
	v, err := s.AsBool()
	assert.NoError(t, err)
	assert.True(t, v)
}

func TestCorePlatformSetting_AsFloat_ParsesNumber(t *testing.T) {
	s := CorePlatformSetting{Key: "x", Value: "0.7", ValueType: "number"}
	v, err := s.AsFloat()
	assert.NoError(t, err)
	assert.InDelta(t, 0.7, v, 0.0001)
}

func TestCorePlatformSetting_AsInt_ParsesNumber(t *testing.T) {
	s := CorePlatformSetting{Key: "x", Value: "25", ValueType: "number"}
	v, err := s.AsInt()
	assert.NoError(t, err)
	assert.Equal(t, 25, v)
}

func TestCorePlatformSetting_AsInt_RejectsNonNumber(t *testing.T) {
	s := CorePlatformSetting{Key: "x", Value: "true", ValueType: "boolean"}
	_, err := s.AsInt()
	assert.Error(t, err)
}
