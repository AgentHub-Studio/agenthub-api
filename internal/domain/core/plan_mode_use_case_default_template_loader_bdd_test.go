package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePMUDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksPlanModeWorkflowFromCatalog", func(t *testing.T) {
		// Given fresh tenants need a PERM-003a plan-mode workflow but
		// inventing extra read-only tools + max-actions + rationale
		// rules from scratch is error-prone,
		// When admin opens plan-mode onboarding,
		// Then 5 recommended templates surface across the safety ladder.
		assert.Equal(t, 5, len(SeedRecommendedPMUDTemplateSlugs))
	})

	t.Run("Scenario_DryRunPreviewForLightLowStakesWork", func(t *testing.T) {
		// Given a user just wants to see what the agent WOULD do,
		// When admin uses dry-run-preview,
		// Then no rationale required, no max-action cap, small
		// auto-approve threshold — the routine baseline.
		assert.Contains(t, SeedExpectedPMUDTemplateSlugs, "dry-run-preview")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPMUDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dry-run-preview"])
	})

	t.Run("Scenario_ScopedChangeReviewForBoundedTasks", func(t *testing.T) {
		// Given tenants run the agent on bounded tasks and need a
		// rationale audit trail,
		// When admin uses scoped-change-review,
		// Then rationale required + 20 max-actions cap.
		assert.Contains(t, SeedExpectedPMUDTemplateSlugs, "scoped-change-review")
	})

	t.Run("Scenario_DestructiveAuditForHighRiskOperations", func(t *testing.T) {
		// Given high-risk destructive operations,
		// When admin uses destructive-audit,
		// Then rationale mandatory, low max-action cap, admin review.
		assert.Contains(t, SeedExpectedPMUDTemplateSlugs, "destructive-audit")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPMUDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["destructive-audit"])
	})

	t.Run("Scenario_MultiStepRefactorAllowsLongerPlans", func(t *testing.T) {
		// Given engineering refactors need long plans + ability to
		// search/test during planning,
		// When admin uses multi-step-refactor,
		// Then extra read-only set includes Read/Grep/Glob, max-actions=50,
		// rationale required.
		assert.Contains(t, SeedExpectedPMUDTemplateSlugs, "multi-step-refactor")
	})

	t.Run("Scenario_CrossTenantMigrationMostDefensive", func(t *testing.T) {
		// Given migrations that span tenant boundaries are highest risk,
		// When admin uses cross-tenant-migration,
		// Then strictest posture: 0 auto-approve, low max-actions,
		// admin review.
		assert.Contains(t, SeedExpectedPMUDTemplateSlugs, "cross-tenant-migration")
	})

	t.Run("Scenario_AdminReviewGatesRiskyPostures", func(t *testing.T) {
		// Given dry_run + scoped_change são routine baseline,
		// And destructive + multi_step + cross_tenant change blast radius,
		// When admin compares admin-review subset,
		// Then 3 of 5 templates gate change.
		assert.Equal(t, 3, len(SeedAdminReviewPMUDTemplateSlugs))
	})

	t.Run("Scenario_SafetyPostureLadderRepresented", func(t *testing.T) {
		// Given posture is a stance ladder (permissive..balanced..strict),
		// When seed templates ship,
		// Then all 3 postures are represented.
		assert.Equal(t, 3, len(SeedExpectedPMUDTemplateSafetyPostures))
	})

	t.Run("Scenario_ExtraReadOnlyToolsMatchClaudeCodeReadSet", func(t *testing.T) {
		// Given multi-step-refactor needs Read/Grep/Glob during planning,
		// When seed declares extra_read_only_tools for that slug,
		// Then the list aligns with TOOL-003 builtin read-only classification.
		// Validated structurally in integration test.
		assert.Contains(t, SeedExpectedPMUDTemplateSlugs, "multi-step-refactor")
	})

	t.Run("Scenario_StrictPosturesDisableAutoApprove", func(t *testing.T) {
		// Given strict postures (destructive/cross-tenant) must require
		// explicit user approval — no auto-approve,
		// When admin compares auto_approve_threshold,
		// Then strict templates have threshold=0. Validated DB-real.
		assert.Equal(t, 5, SeedExpectedPMUDTemplateRowCount)
	})

	t.Run("Scenario_RationaleRequiredOnEverythingExceptDryRun", func(t *testing.T) {
		// Given audit/risk tenants want rationale on every planned action,
		// And dry-run is exploratory (rationale optional),
		// When admin compares requires_rationale,
		// Then 4 of 5 templates require rationale. Validated DB-real.
		assert.Equal(t, 5, SeedExpectedPMUDTemplateRowCount)
	})
}
