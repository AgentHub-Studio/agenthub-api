package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePCMDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 11, len(SeedExpectedPCMDTemplateSlugs))
	assert.Equal(t, 11, SeedExpectedPCMDTemplateRowCount)
}

func TestCorePCMDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPCMDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePCMDTemplate_SlugsAreSnakeCase(t *testing.T) {
	for _, s := range SeedExpectedPCMDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, " ")
	}
}

func TestCorePCMDTemplate_KindsMatchCORE_SEED_001Enum(t *testing.T) {
	// Byte-for-byte alignment with CORE-SEED-001 PlatformCatalogKind enum
	// in internal/domain/core/platform_catalog.go.
	expected := map[string]bool{
		"subagent_roster": true, "agent_definition": true,
		"toolset_policy": true, "inheritance_mode": true, "summary_shape": true,
		"fork_strategy": true, "background_lane": true,
		"context_policy": true, "permission_policy": true,
		"extension_descriptor": true, "operational_template": true,
	}
	for _, k := range SeedExpectedPCMDTemplateKinds {
		assert.True(t, expected[k], "kind %q outside CORE-SEED-001 enum", k)
	}
	assert.Equal(t, 11, len(SeedExpectedPCMDTemplateKinds))
}

func TestCorePCMDTemplate_KindsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedPCMDTemplateKinds {
		assert.False(t, seen[k])
		seen[k] = true
	}
}

func TestCorePCMDTemplate_LoaderPackagesClosedSet(t *testing.T) {
	expected := map[string]bool{"core": true}
	for _, p := range SeedExpectedPCMDTemplateLoaderPackages {
		assert.True(t, expected[p])
	}
}

func TestCorePCMDTemplate_AllRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPCMDTemplateSlugs,
		SeedRecommendedPCMDTemplateSlugs)
}

func TestCorePCMDTemplate_SlugCountEqualsKindCount(t *testing.T) {
	// 1:1 mapping invariant: every PlatformCatalogKind has exactly one
	// seeded manifest in this version.
	assert.Equal(t, len(SeedExpectedPCMDTemplateKinds), len(SeedExpectedPCMDTemplateSlugs))
}
