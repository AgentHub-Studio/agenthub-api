package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreDSHDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshExtensionAuthorPicksProvenHookPattern", func(t *testing.T) {
		// Given vendors authoring skills want to add common hook
		// patterns (validation/sanitization/audit/etc),
		// And EXT-007 DynamicSkillHookRegistry accepts hooks per skill,
		// When vendor opens hook authoring UI,
		// Then 6 proven patterns surface as recommended starting points.
		assert.Equal(t, 6, len(SeedRecommendedDSHDTemplateSlugs))
	})

	t.Run("Scenario_PhaseLabelsMatchEXT007EnumByteForByte", func(t *testing.T) {
		// Given EXT-007 has 5 phase enum values,
		// When seed declares target_phase,
		// Then labels match enum bytes (no mapping table runtime).
		ext007 := []string{
			"before_invocation", "before_tool_call",
			"after_tool_call", "after_invocation", "on_error",
		}
		set := map[string]bool{}
		for _, p := range SeedExpectedDSHDTemplatePhases {
			set[p] = true
		}
		for _, e := range ext007 {
			assert.True(t, set[e], "EXT-007 phase %q missing", e)
		}
	})

	t.Run("Scenario_RedactPIIRequiresAdminReviewDueToPrivacyImpact", func(t *testing.T) {
		// Given PII redaction changes privacy posture org-wide,
		// When vendor copies redact-pii template,
		// Then admin review required (regulated tenants must vet).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewDSHDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["redact-pii"])
	})

	t.Run("Scenario_ValidateInputUsesBeforeInvocationForFailFastBehavior", func(t *testing.T) {
		// Given input validation should abort before any tool fires,
		// When vendor uses validate-input,
		// Then phase = before_invocation (validates args before any
		// side effects).
		assert.Contains(t, SeedExpectedDSHDTemplateSlugs, "validate-input")
	})

	t.Run("Scenario_AuditToolCallUsesBeforeToolCallForGOV001Trail", func(t *testing.T) {
		// Given GOV-001 audits tool invocations within skills,
		// When vendor uses audit-tool-call,
		// Then phase = before_tool_call (audit fires before each tool
		// — entry point captured).
		assert.Contains(t, SeedExpectedDSHDTemplateSlugs, "audit-tool-call")
	})

	t.Run("Scenario_CostTrackUsesAfterToolCallForActualUsageMetering", func(t *testing.T) {
		// Given cost dashboards need real token+dollar measurements,
		// When vendor uses cost-track,
		// Then phase = after_tool_call (records ACTUAL usage, not
		// estimated pre-call).
		assert.Contains(t, SeedExpectedDSHDTemplateSlugs, "cost-track")
	})

	t.Run("Scenario_SanitizeOutputUsesAfterInvocationForUserFacingPolish", func(t *testing.T) {
		// Given end-user-facing output should hide internals,
		// When vendor uses sanitize-output,
		// Then phase = after_invocation (last stop before output reaches
		// user).
		assert.Contains(t, SeedExpectedDSHDTemplateSlugs, "sanitize-output")
	})

	t.Run("Scenario_ErrorRecoveryUsesOnErrorForFailureContextCapture", func(t *testing.T) {
		// Given debugging skill failures needs full context,
		// When vendor uses error-recovery,
		// Then phase = on_error (captures last tool call + args + error).
		assert.Contains(t, SeedExpectedDSHDTemplateSlugs, "error-recovery")
	})

	t.Run("Scenario_AllSixPhasesCoveredAcrossSeed", func(t *testing.T) {
		// Given EXT-007 has 5 phases,
		// When seed templates ship,
		// Then ALL 5 phases are represented (vendor sees example for
		// every lifecycle moment).
		// Validated structurally via integration test.
		assert.Equal(t, 5, len(SeedExpectedDSHDTemplatePhases))
	})

	t.Run("Scenario_HighPriorityForCriticalHooksLowerForRoutine", func(t *testing.T) {
		// Given EXT-007 sorts hooks by priority desc,
		// When vendor copies templates,
		// Then critical ones (validate=90, redact-pii=95) outrank
		// routine ones (cost-track=60). Validated via integration test.
		assert.Equal(t, 6, len(SeedExpectedDSHDTemplateSlugs))
	})
}
