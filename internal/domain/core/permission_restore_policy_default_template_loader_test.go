package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePRPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedPRPDTemplateSlugs))
	assert.Equal(t, 4, SeedExpectedPRPDTemplateRowCount)
}

func TestCorePRPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPRPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePRPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPRPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePRPDTemplate_PoliciesMatchPERM009Enum(t *testing.T) {
	expected := map[string]bool{
		"discard_all": true, "preserve_durable_only": true,
		"preserve_explicit_grants": true, "strict_re_request": true,
	}
	for _, p := range SeedExpectedPRPDTemplatePolicies {
		assert.True(t, expected[p], "policy %q outside PERM-009 enum", p)
	}
	assert.Equal(t, 4, len(SeedExpectedPRPDTemplatePolicies))
}

func TestCorePRPDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"fresh_resume": true, "routine_resume": true,
		"compliance_audit": true, "audit_strict_resume": true,
	}
	for _, u := range SeedExpectedPRPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 4, len(SeedExpectedPRPDTemplateUseCases))
}

func TestCorePRPDTemplate_SafetyPosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"strict": true, "balanced": true, "conservative": true,
	}
	for _, p := range SeedExpectedPRPDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
}

func TestCorePRPDTemplate_DurabilitiesMatchPERM009Enum(t *testing.T) {
	expected := map[string]bool{
		"one_shot": true, "session_scoped": true,
		"persisted": true, "explicit_admin": true,
	}
	for _, d := range SeedExpectedPRPDTemplateDurabilities {
		assert.True(t, expected[d], "durability %q outside PERM-009 enum", d)
	}
	assert.Equal(t, 4, len(SeedExpectedPRPDTemplateDurabilities))
}

func TestCorePRPDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedPRPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCorePRPDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPRPDTemplateSlugs,
		SeedRecommendedPRPDTemplateSlugs)
}

func TestCorePRPDTemplate_AdminReviewExcludesRoutine(t *testing.T) {
	// preserve-durable-routine is the balanced default; others change
	// posture materially and require admin review.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewPRPDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["preserve-durable-routine"])
	assert.True(t, set["discard-all-fresh-context"])
	assert.True(t, set["preserve-explicit-compliance"])
	assert.True(t, set["strict-re-request-audit"])
}
