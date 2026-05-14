package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedExtensionMechanism_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 4, SeedExpectedExtensionMechanismRowCount)
}

func TestSeedExtensionMechanism_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedExtensionMechanismRowCount, len(SeedExpectedExtensionMechanismSlugs))
}

func TestSeedExtensionMechanism_SlugsContainHooks(t *testing.T) {
	assert.Contains(t, SeedExpectedExtensionMechanismSlugs, "hooks")
}

func TestSeedExtensionMechanism_SlugsContainSkills(t *testing.T) {
	assert.Contains(t, SeedExpectedExtensionMechanismSlugs, "skills")
}

func TestSeedExtensionMechanism_SlugsContainPlugins(t *testing.T) {
	assert.Contains(t, SeedExpectedExtensionMechanismSlugs, "plugins")
}

func TestSeedExtensionMechanism_SlugsContainMCPServers(t *testing.T) {
	assert.Contains(t, SeedExpectedExtensionMechanismSlugs, "mcp_servers")
}

func TestSeedExtensionMechanism_OnlyHooksIsZeroCost(t *testing.T) {
	assert.Equal(t, 1, len(SeedExtensionMechanismZeroCostSlugs))
	assert.Contains(t, SeedExtensionMechanismZeroCostSlugs, "hooks")
}

func TestSeedExtensionMechanism_OnlyPluginsCoversAllInsertPoints(t *testing.T) {
	assert.Equal(t, 1, len(SeedExtensionMechanismAllInsertPointSlugs))
	assert.Contains(t, SeedExtensionMechanismAllInsertPointSlugs, "plugins")
}

func TestSeedExtensionMechanism_FourInsertionPoints(t *testing.T) {
	assert.Equal(t, 4, len(SeedExtensionMechanismInsertionPoints))
	assert.Contains(t, SeedExtensionMechanismInsertionPoints, "assemble")
	assert.Contains(t, SeedExtensionMechanismInsertionPoints, "model")
	assert.Contains(t, SeedExtensionMechanismInsertionPoints, "execute")
	assert.Contains(t, SeedExtensionMechanismInsertionPoints, "all")
}

func TestSeedExtensionMechanism_SubsetSlugsAreInCanonicalList(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedExtensionMechanismSlugs {
		all[s] = true
	}
	for _, s := range SeedExtensionMechanismZeroCostSlugs {
		assert.True(t, all[s], "zero-cost slug %q not in canonical list", s)
	}
	for _, s := range SeedExtensionMechanismAllInsertPointSlugs {
		assert.True(t, all[s], "all-insert-point slug %q not in canonical list", s)
	}
}
