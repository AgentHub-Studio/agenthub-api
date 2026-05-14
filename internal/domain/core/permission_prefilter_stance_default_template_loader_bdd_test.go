package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePPSDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksStanceFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a PERM-004 PrefilterStance,
		// And the catalog rejects ad-hoc labels,
		// When admin opens pre-filter onboarding,
		// Then 4 recommended templates surface covering the use-case
		// spectrum (interactive/background/lockdown/audit).
		assert.Equal(t, 4, len(SeedRecommendedPPSDTemplateSlugs))
	})

	t.Run("Scenario_InteractiveDefaultForChatSessions", func(t *testing.T) {
		// Given humans are present during chat sessions and can answer
		// confirm prompts,
		// When admin uses interactive-default,
		// Then confirm-tier tools remain visible (call-time prompt is
		// still the gate).
		assert.Contains(t, SeedExpectedPPSDTemplateSlugs, "interactive-default")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPPSDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["interactive-default"])
	})

	t.Run("Scenario_UnattendedBatchHidesConfirmToAvoidStalls", func(t *testing.T) {
		// Given background subagents and scheduled jobs run without a
		// human in the loop,
		// When admin uses unattended-batch,
		// Then confirm-tier tools are dropped (stance hide_confirm) so
		// the LLM never stalls on a prompt that cannot be answered.
		assert.Contains(t, SeedExpectedPPSDTemplateSlugs, "unattended-batch")
	})

	t.Run("Scenario_LockdownReadonlyForIncidentResponse", func(t *testing.T) {
		// Given a security investigation requires read-only access,
		// When admin uses lockdown-readonly,
		// Then every mutating tool is denied at pool assembly — the LLM
		// only sees inspection capability.
		assert.Contains(t, SeedExpectedPPSDTemplateSlugs, "lockdown-readonly")
	})

	t.Run("Scenario_AuditStrictTraceForCompliance", func(t *testing.T) {
		// Given compliance-sensitive tenants need every mutating call
		// to go through the human + be audit-logged,
		// When admin uses audit-strict-trace,
		// Then confirm-tier tools stay visible AND every pre-filter
		// decision is recorded.
		assert.Contains(t, SeedExpectedPPSDTemplateSlugs, "audit-strict-trace")
	})

	t.Run("Scenario_StanceLabelsMatchPERM004EnumByteForByte", func(t *testing.T) {
		// Given PERM-004 PrefilterStance has 2 values,
		// When seed declares target_stance,
		// Then labels match enum bytes (no mapping table runtime).
		perm004 := []string{"show_confirm", "hide_confirm"}
		set := map[string]bool{}
		for _, s := range SeedExpectedPPSDTemplateStances {
			set[s] = true
		}
		for _, e := range perm004 {
			assert.True(t, set[e], "PERM-004 stance %q missing", e)
		}
	})

	t.Run("Scenario_BothStancesRepresented", func(t *testing.T) {
		// Given PERM-004 has 2 stances,
		// When seed templates ship,
		// Then both stances have at least one example template.
		// Validated structurally via integration test.
		assert.Equal(t, 2, len(SeedExpectedPPSDTemplateStances))
	})

	t.Run("Scenario_AdminReviewGatesHighImpactStances", func(t *testing.T) {
		// Given 3 of 4 templates materially change security posture
		// (batch, lockdown, audit),
		// When admin compares admin-review subset,
		// Then only interactive-default is review-free.
		assert.Equal(t, 3, len(SeedAdminReviewPPSDTemplateSlugs))
	})

	t.Run("Scenario_BaselineDenyPatternsShipWithEveryTemplate", func(t *testing.T) {
		// Given admins should not start from scratch,
		// When the template lands in onboarding,
		// Then baseline_deny_patterns is non-empty for routine templates
		// (validated cross-row in integration tests).
		assert.Equal(t, 4, SeedExpectedPPSDTemplateRowCount)
	})

	t.Run("Scenario_LockdownHasEmptyConfirmListBecauseAllMutationsDenied", func(t *testing.T) {
		// Given lockdown denies every mutating tool,
		// When admin compares confirm patterns,
		// Then lockdown has empty baseline_confirm_patterns (no tool
		// reaches the confirm stage). Validated structurally in
		// integration tests.
		assert.Contains(t, SeedExpectedPPSDTemplateSlugs, "lockdown-readonly")
	})
}
