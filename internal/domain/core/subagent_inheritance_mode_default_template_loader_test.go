package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSIMDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedSIMDTemplateSlugs))
	assert.Equal(t, 4, SeedExpectedSIMDTemplateRowCount)
}

func TestCoreSIMDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSIMDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSIMDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedSIMDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreSIMDTemplate_ModesMatchSUB006Enum(t *testing.T) {
	expected := map[string]bool{
		"inherit_all": true, "inherit_strict_only": true,
		"override_replace": true, "merge_intersect": true,
	}
	for _, m := range SeedExpectedSIMDTemplateModes {
		assert.True(t, expected[m], "mode %q outside SUB-006 enum", m)
	}
	assert.Equal(t, 4, len(SeedExpectedSIMDTemplateModes))
}

func TestCoreSIMDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"helper_extension": true, "narrow_utility": true,
		"explicit_isolation": true, "compliance_audit": true,
	}
	for _, u := range SeedExpectedSIMDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 4, len(SeedExpectedSIMDTemplateUseCases))
}

func TestCoreSIMDTemplate_RiskPosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"balanced": true, "conservative": true,
		"strict": true, "permissive": true,
	}
	for _, p := range SeedExpectedSIMDTemplateRiskPostures {
		assert.True(t, expected[p])
	}
	assert.Equal(t, 4, len(SeedExpectedSIMDTemplateRiskPostures))
}

func TestCoreSIMDTemplate_AuditSignalsMatchSUB006Struct(t *testing.T) {
	// These must align with SubagentPermissionResolution audit fields
	// (AddedAllows / AddedDenies / DroppedAllows / ReasonSummary).
	expected := map[string]bool{
		"AddedAllows": true, "AddedDenies": true,
		"DroppedAllows": true, "ReasonSummary": true,
	}
	for _, a := range SeedExpectedSIMDTemplateAuditSignals {
		assert.True(t, expected[a], "signal %q not in SUB-006 audit struct", a)
	}
	assert.Equal(t, 4, len(SeedExpectedSIMDTemplateAuditSignals))
}

func TestCoreSIMDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedSIMDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreSIMDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedSIMDTemplateSlugs,
		SeedRecommendedSIMDTemplateSlugs)
}

func TestCoreSIMDTemplate_AdminReviewExcludesExtendRoutine(t *testing.T) {
	// extend-parent-rights is the routine default; others change
	// composition materially and require admin review.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewSIMDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["extend-parent-rights"])
	assert.True(t, set["sandboxed-worker"])
	assert.True(t, set["isolated-decoupled"])
	assert.True(t, set["audit-strict-intersect"])
}
