package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSFSDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedExpectedSFSDTemplateSlugs))
	assert.Equal(t, 3, SeedExpectedSFSDTemplateRowCount)
}

func TestCoreSFSDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSFSDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSFSDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedSFSDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreSFSDTemplate_StrategiesMatchPERSIST005aEnum(t *testing.T) {
	expected := map[string]bool{
		"full_copy": true, "branch_pointer": true, "snapshot_isolated": true,
	}
	for _, s := range SeedExpectedSFSDTemplateStrategies {
		assert.True(t, expected[s], "strategy %q outside PERSIST-005a enum", s)
	}
	assert.Equal(t, 3, len(SeedExpectedSFSDTemplateStrategies))
}

func TestCoreSFSDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"routine_branch_exploration": true, "storage_optimized_branch": true,
		"compliance_branch": true,
	}
	for _, u := range SeedExpectedSFSDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreSFSDTemplate_PosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"balanced": true, "permissive": true, "strict": true,
	}
	for _, p := range SeedExpectedSFSDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
}

func TestCoreSFSDTemplate_StorageOverheadsClosedSet(t *testing.T) {
	expected := map[string]bool{"low": true, "medium": true, "high": true}
	for _, o := range SeedExpectedSFSDTemplateStorageOverheads {
		assert.True(t, expected[o])
	}
}

func TestCoreSFSDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedSFSDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreSFSDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedSFSDTemplateSlugs,
		SeedRecommendedSFSDTemplateSlugs)
}

func TestCoreSFSDTemplate_AdminReviewSubsetIsComplianceOnly(t *testing.T) {
	assert.Equal(t,
		[]string{"snapshot-isolated-compliance"},
		SeedAdminReviewSFSDTemplateSlugs)
}
