package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePHDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksHookPatternsFromCatalog", func(t *testing.T) {
		// Given fresh tenants must implement PERM-005 PermissionHook
		// patterns and the chain rejects ad-hoc handles,
		// When admin opens hook onboarding,
		// Then 6 recommended templates surface across the phase × outcome
		// matrix.
		assert.Equal(t, 6, len(SeedRecommendedPHDTemplateSlugs))
	})

	t.Run("Scenario_OncallBypassForEmergencyResponse", func(t *testing.T) {
		// Given on-call engineers must never be blocked during incidents,
		// When admin uses oncall-bypass-allow,
		// Then a Before-phase override_allow hook implements the pattern.
		assert.Contains(t, SeedExpectedPHDTemplateSlugs, "oncall-bypass-allow")
	})

	t.Run("Scenario_BusinessHoursGateReducesBlastRadius", func(t *testing.T) {
		// Given no human is around outside business hours,
		// When admin uses business-hours-gate-deny,
		// Then a Before-phase override_deny shrinks blast radius.
		assert.Contains(t, SeedExpectedPHDTemplateSlugs, "business-hours-gate-deny")
	})

	t.Run("Scenario_PIIEscalationKeepsRoutineCallsFastButGatesSensitive", func(t *testing.T) {
		// Given the engine says Allow for execute-sql but the input has PII,
		// When admin uses pii-input-escalate-confirm,
		// Then an After-phase override_confirm inserts a human checkpoint.
		assert.Contains(t, SeedExpectedPHDTemplateSlugs, "pii-input-escalate-confirm")
	})

	t.Run("Scenario_AuditObserverIsPureSideEffectNoDecisionChange", func(t *testing.T) {
		// Given every tenant needs decision-level audit (GOV-001),
		// When admin uses audit-trace-observer-continue,
		// Then an After-phase continue hook records without overriding.
		assert.Contains(t, SeedExpectedPHDTemplateSlugs, "audit-trace-observer-continue")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPHDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["audit-trace-observer-continue"])
	})

	t.Run("Scenario_RateLimitDefendsAgainstRunawayLoops", func(t *testing.T) {
		// Given a runaway agent could DoS itself by hammering a tool,
		// When admin uses rate-limit-cooldown-deny,
		// Then a Before-phase override_deny denies past N calls/M min.
		assert.Contains(t, SeedExpectedPHDTemplateSlugs, "rate-limit-cooldown-deny")
	})

	t.Run("Scenario_SensitiveCustomerDenyIsStricterThanPII", func(t *testing.T) {
		// Given some customers are tagged "no automation may touch",
		// When admin uses sensitive-customer-deny,
		// Then an After-phase override_deny is stricter than the PII
		// confirm (deny vs confirm).
		assert.Contains(t, SeedExpectedPHDTemplateSlugs, "sensitive-customer-deny")
	})

	t.Run("Scenario_PhaseLabelsMatchPERM005EnumByteForByte", func(t *testing.T) {
		// Given PERM-005 PermissionHookPhase has 2 values,
		// When seed declares target_phase,
		// Then labels match enum bytes (no mapping table runtime).
		perm005 := []string{"before_evaluate", "after_evaluate"}
		set := map[string]bool{}
		for _, p := range SeedExpectedPHDTemplatePhases {
			set[p] = true
		}
		for _, e := range perm005 {
			assert.True(t, set[e], "PERM-005 phase %q missing", e)
		}
	})

	t.Run("Scenario_OutcomeLabelsMatchPERM005EnumByteForByte", func(t *testing.T) {
		// Given PERM-005 PermissionHookOutcome has 4 values,
		// When seed declares target_outcome,
		// Then labels match enum bytes.
		perm005 := []string{"continue", "override_allow", "override_deny", "override_confirm"}
		set := map[string]bool{}
		for _, o := range SeedExpectedPHDTemplateOutcomes {
			set[o] = true
		}
		for _, e := range perm005 {
			assert.True(t, set[e], "PERM-005 outcome %q missing", e)
		}
	})

	t.Run("Scenario_BothPhasesRepresented", func(t *testing.T) {
		// Given PERM-005 has 2 phases (before/after),
		// When seed templates ship,
		// Then both phases have at least one example template.
		// Validated structurally via integration test.
		assert.Equal(t, 2, len(SeedExpectedPHDTemplatePhases))
	})

	t.Run("Scenario_AdminReviewGatesEveryDecisionAlteringHook", func(t *testing.T) {
		// Given any hook that overrides the engine materially changes
		// policy,
		// When admin compares admin-review subset,
		// Then 5 of 6 templates require review (only pure observer is
		// review-free).
		assert.Equal(t, 5, len(SeedAdminReviewPHDTemplateSlugs))
	})
}
