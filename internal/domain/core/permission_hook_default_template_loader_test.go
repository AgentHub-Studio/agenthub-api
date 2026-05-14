package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePHDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedPHDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedPHDTemplateRowCount)
}

func TestCorePHDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPHDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePHDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPHDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePHDTemplate_PhasesMatchPERM005Enum(t *testing.T) {
	expected := map[string]bool{"before_evaluate": true, "after_evaluate": true}
	for _, p := range SeedExpectedPHDTemplatePhases {
		assert.True(t, expected[p], "phase %q outside PERM-005 enum", p)
	}
	assert.Equal(t, 2, len(SeedExpectedPHDTemplatePhases))
}

func TestCorePHDTemplate_OutcomesMatchPERM005Enum(t *testing.T) {
	expected := map[string]bool{
		"continue": true, "override_allow": true,
		"override_deny": true, "override_confirm": true,
	}
	for _, o := range SeedExpectedPHDTemplateOutcomes {
		assert.True(t, expected[o], "outcome %q outside PERM-005 enum", o)
	}
	assert.Equal(t, 4, len(SeedExpectedPHDTemplateOutcomes))
}

func TestCorePHDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"oncall_escalation": true, "time_window_gate": true,
		"data_sensitivity": true, "audit_observability": true,
		"rate_limit": true, "customer_protection": true,
	}
	for _, u := range SeedExpectedPHDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 6, len(SeedExpectedPHDTemplateUseCases))
}

func TestCorePHDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	for _, k := range SeedExpectedPHDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCorePHDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPHDTemplateSlugs,
		SeedRecommendedPHDTemplateSlugs)
}

func TestCorePHDTemplate_AdminReviewExcludesPureObserver(t *testing.T) {
	// The audit-trace observer doesn't change decisions — no admin
	// review needed. Every other template materially alters policy.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewPHDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["audit-trace-observer-continue"])
	assert.True(t, set["oncall-bypass-allow"])
	assert.True(t, set["business-hours-gate-deny"])
	assert.True(t, set["pii-input-escalate-confirm"])
	assert.True(t, set["rate-limit-cooldown-deny"])
	assert.True(t, set["sensitive-customer-deny"])
}
