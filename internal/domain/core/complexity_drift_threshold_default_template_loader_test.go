package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCDTTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedCDTTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedCDTTemplateRowCount)
}

func TestCoreCDTTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCDTTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCDTTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCDTTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreCDTTemplate_SignalsMatchHUMAN006EnumByteForByte(t *testing.T) {
	// Cross-feature invariant: every signal must match HUMAN-006
	// ComplexityDriftSignal bounded enum byte-for-byte.
	expected := map[string]bool{
		"scope_creep": true, "dependency_explosion": true, "test_decay": true,
		"churn_spike": true, "goal_drift": true, "cognitive_load": true,
	}
	for _, s := range SeedExpectedCDTTemplateSignals {
		assert.True(t, expected[s], "signal %q outside HUMAN-006 enum", s)
	}
	assert.Equal(t, 6, len(SeedExpectedCDTTemplateSignals))
}

func TestCoreCDTTemplate_SignalsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCDTTemplateSignals {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCDTTemplate_UnitsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"sub_tasks": true, "modules_touched": true,
		"coverage_drop_ratio": true, "reedits_per_file": true,
		"cosine_distance": true, "context_fill_ratio": true,
	}
	for _, u := range SeedExpectedCDTTemplateUnits {
		assert.True(t, expected[u], "unit %q outside closed set", u)
	}
	assert.Equal(t, 6, len(SeedExpectedCDTTemplateUnits))
}

func TestCoreCDTTemplate_UnitsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, u := range SeedExpectedCDTTemplateUnits {
		assert.False(t, seen[u])
		seen[u] = true
	}
}

func TestCoreCDTTemplate_SubjectKindsClosedSet(t *testing.T) {
	expected := map[string]bool{"run": true}
	for _, k := range SeedExpectedCDTTemplateSubjectKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreCDTTemplate_AllRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedCDTTemplateSlugs,
		SeedRecommendedCDTTemplateSlugs)
}

func TestCoreCDTTemplate_OneToOneSignalToTemplate(t *testing.T) {
	assert.Equal(t, len(SeedExpectedCDTTemplateSignals), SeedExpectedCDTTemplateRowCount)
}

func TestCoreCDTTemplate_OneToOneUnitToSignal(t *testing.T) {
	// Each signal has a distinct unit (no two signals share the same
	// physical measurement).
	assert.Equal(t, len(SeedExpectedCDTTemplateUnits), len(SeedExpectedCDTTemplateSignals))
}
