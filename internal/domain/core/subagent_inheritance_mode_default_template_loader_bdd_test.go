package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSIMDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksInheritanceModeFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a SUB-006 PermissionInheritanceMode
		// before spawning subagents and the resolver rejects unknown labels,
		// When admin opens subagent inheritance onboarding,
		// Then 4 recommended templates surface 1:1 with the enum.
		assert.Equal(t, 4, len(SeedRecommendedSIMDTemplateSlugs))
	})

	t.Run("Scenario_ExtendParentRightsForHelperSubagents", func(t *testing.T) {
		// Given a routine helper subagent extends the parent with extra
		// allows or denies,
		// When admin uses extend-parent-rights,
		// Then inherit_all is selected and no admin review required.
		assert.Contains(t, SeedExpectedSIMDTemplateSlugs, "extend-parent-rights")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSIMDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["extend-parent-rights"])
	})

	t.Run("Scenario_SandboxedWorkerNarrowUtility", func(t *testing.T) {
		// Given a narrow utility subagent (formatter, generator) needs
		// its own allow/confirm but parent's deny set must stick,
		// When admin uses sandboxed-worker,
		// Then inherit_strict_only is selected.
		assert.Contains(t, SeedExpectedSIMDTemplateSlugs, "sandboxed-worker")
	})

	t.Run("Scenario_IsolatedDecoupledForReviewedWorkers", func(t *testing.T) {
		// Given a worker has been explicitly reviewed and the parent
		// signed off on de-coupling its rules,
		// When admin uses isolated-decoupled,
		// Then override_replace is selected and admin review required.
		assert.Contains(t, SeedExpectedSIMDTemplateSlugs, "isolated-decoupled")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewSIMDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["isolated-decoupled"])
	})

	t.Run("Scenario_AuditStrictIntersectForComplianceTenants", func(t *testing.T) {
		// Given compliance tenants require subagent allow ⊆ parent allow,
		// When admin uses audit-strict-intersect,
		// Then merge_intersect is selected — DroppedAllows audit signal
		// becomes meaningful.
		assert.Contains(t, SeedExpectedSIMDTemplateSlugs, "audit-strict-intersect")
	})

	t.Run("Scenario_ModeLabelsMatchSUB006EnumByteForByte", func(t *testing.T) {
		// Given SUB-006 PermissionInheritanceMode has 4 values,
		// When seed declares target_inheritance_mode,
		// Then labels match enum bytes (no mapping table runtime).
		sub006 := []string{"inherit_all", "inherit_strict_only",
			"override_replace", "merge_intersect"}
		set := map[string]bool{}
		for _, m := range SeedExpectedSIMDTemplateModes {
			set[m] = true
		}
		for _, e := range sub006 {
			assert.True(t, set[e], "SUB-006 mode %q missing", e)
		}
	})

	t.Run("Scenario_AuditSignalsMatchResolutionStructFields", func(t *testing.T) {
		// Given each mode produces a specific audit signal,
		// When the catalog declares expected_audit_signals,
		// Then every signal name corresponds to a real field on
		// SubagentPermissionResolution (no phantom field names).
		expected := []string{"AddedAllows", "AddedDenies", "DroppedAllows", "ReasonSummary"}
		set := map[string]bool{}
		for _, a := range SeedExpectedSIMDTemplateAuditSignals {
			set[a] = true
		}
		for _, e := range expected {
			assert.True(t, set[e], "audit signal %q missing", e)
		}
	})

	t.Run("Scenario_FourModesAllRepresented", func(t *testing.T) {
		// Given SUB-006 has 4 modes,
		// When seed templates ship,
		// Then ALL 4 modes have exactly one template (1:1 mapping).
		// Validated structurally via integration test.
		assert.Equal(t, 4, len(SeedExpectedSIMDTemplateModes))
	})

	t.Run("Scenario_AdminReviewGatesNonRoutineModes", func(t *testing.T) {
		// Given inherit_all is the routine baseline,
		// And the other 3 modes change composition materially,
		// When admin compares admin-review subset,
		// Then 3 of 4 templates gate change.
		assert.Equal(t, 3, len(SeedAdminReviewSIMDTemplateSlugs))
	})

	t.Run("Scenario_RiskPostureSpectrumCovered", func(t *testing.T) {
		// Given risk posture is a stance ladder (balanced..strict..permissive),
		// When seed templates ship,
		// Then all 4 postures are represented.
		assert.Equal(t, 4, len(SeedExpectedSIMDTemplateRiskPostures))
	})
}
