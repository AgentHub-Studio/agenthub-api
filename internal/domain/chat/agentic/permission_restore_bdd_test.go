package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PermissionRestore(t *testing.T) {
	t.Run("Scenario_ResumeDoesNotSilentlyRestoreBashGrant", func(t *testing.T) {
		// Given the original session granted Bash at 10:00,
		// And the user resumes the session 24h later,
		// When the resume pipeline applies discard_all,
		// Then Bash is dropped — the LLM must re-request.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow,
				Durability: GrantDurabilitySessionScoped,
				GrantedAt: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
		}
		ctx := RestoreContext{
			Operation: RestoreOperationResume, GapSeconds: 86400,
		}
		res, err := FilterRestorableGrants(PermissionRestoreDiscardAll, ctx, grants)
		require.NoError(t, err)
		assert.Empty(t, res.GrantsKept)
	})

	t.Run("Scenario_DenyGrantNeverOverruledByResume", func(t *testing.T) {
		// Given a user explicitly DENIED Bash in the original session,
		// When any policy applies on resume,
		// Then the deny grant survives — never silently relax past refusal.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionDeny,
				Durability: GrantDurabilitySessionScoped},
		}
		for _, policy := range allPermissionRestorePolicies {
			res, err := FilterRestorableGrants(policy,
				RestoreContext{Operation: RestoreOperationResume}, grants)
			require.NoError(t, err)
			assert.Equal(t, 1, len(res.GrantsKept),
				"policy %q dropped a deny grant", policy)
		}
	})

	t.Run("Scenario_AdminGrantSurvivesPreserveDurableOnly", func(t *testing.T) {
		// Given an admin granted Bash explicitly,
		// And the tenant uses preserve_durable_only,
		// When the session is resumed,
		// Then the admin grant survives.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow,
				Durability: GrantDurabilityExplicitAdmin, GrantedBy: "admin"},
		}
		res, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		assert.Equal(t, 1, len(res.GrantsKept))
	})

	t.Run("Scenario_PersistedSessionGrantsDroppedUnderExplicitOnly", func(t *testing.T) {
		// Given the tenant uses preserve_explicit_grants (compliance-strict),
		// And a non-admin user previously granted Edit (persisted),
		// When the session resumes,
		// Then the persisted-but-not-admin grant is dropped.
		grants := []PreviousGrant{
			{ToolName: "Edit", Decision: PermissionAllow,
				Durability: GrantDurabilityPersisted, GrantedBy: "alice"},
		}
		res, _ := FilterRestorableGrants(PermissionRestorePreserveExplicitGrants,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		assert.Empty(t, res.GrantsKept)
	})

	t.Run("Scenario_OneShotGrantsCannotSurviveResume", func(t *testing.T) {
		// Given a one_shot grant authorised exactly one Bash call,
		// When the session resumes (or forks),
		// Then the one_shot grant is dropped regardless of policy.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow,
				Durability: GrantDurabilityOneShot},
		}
		for _, policy := range allPermissionRestorePolicies {
			res, _ := FilterRestorableGrants(policy,
				RestoreContext{Operation: RestoreOperationResume}, grants)
			assert.Empty(t, res.GrantsKept,
				"policy %q kept a one_shot grant", policy)
		}
	})

	t.Run("Scenario_StrictReRequestFlagsRuntimeToConfirmFirstUse", func(t *testing.T) {
		// Given an audit-strict tenant resumes a session after a long
		// gap,
		// When strict_re_request applies,
		// Then ALL grants are dropped AND RequireFirstUseConfirmation
		// is true so the runtime asks on every first tool call.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow,
				Durability: GrantDurabilityExplicitAdmin},
		}
		res, _ := FilterRestorableGrants(PermissionRestoreStrictReRequest,
			RestoreContext{Operation: RestoreOperationResume, GapSeconds: 3600}, grants)
		assert.Empty(t, res.GrantsKept)
		assert.True(t, res.RequireFirstUseConfirmation)
	})

	t.Run("Scenario_ForkOperationFiltersSameWayAsResume", func(t *testing.T) {
		// Given fork and resume both cross the permission boundary,
		// When the same policy applies to both,
		// Then the kept-grants set is identical (operation type is
		// audit info, not a policy modifier).
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow,
				Durability: GrantDurabilityPersisted},
		}
		resResume, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		resFork, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
			RestoreContext{Operation: RestoreOperationFork}, grants)
		assert.Equal(t, len(resResume.GrantsKept), len(resFork.GrantsKept))
	})

	t.Run("Scenario_DecisionsAuditEveryConsideredGrant", func(t *testing.T) {
		// Given compliance review asks "why did the runtime not
		// re-grant tool X?",
		// When the resolution is queried,
		// Then there's exactly one Decision per grant with a non-empty
		// Reason.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilitySessionScoped},
			{ToolName: "Edit", Decision: PermissionAllow, Durability: GrantDurabilityPersisted},
			{ToolName: "Write", Decision: PermissionDeny, Durability: GrantDurabilityPersisted},
		}
		res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		require.Equal(t, 3, len(res.Decisions))
		for _, d := range res.Decisions {
			assert.NotEmpty(t, d.Reason, "grant %q has empty reason", d.Grant.ToolName)
		}
	})

	t.Run("Scenario_OutputIsDeterministicForReplayability", func(t *testing.T) {
		// Given two runs with identical inputs,
		// When the filter runs,
		// Then the kept-grants slice is byte-for-byte identical (sorted
		// by name then GrantedAt).
		grants := []PreviousGrant{
			{ToolName: "Z", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
			{ToolName: "A", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
		}
		r1, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		r2, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		require.Equal(t, len(r1.GrantsKept), len(r2.GrantsKept))
		for i := range r1.GrantsKept {
			assert.Equal(t, r1.GrantsKept[i].ToolName, r2.GrantsKept[i].ToolName)
		}
	})

	t.Run("Scenario_FilterFeedsPermissionHookChainAndPreFilter", func(t *testing.T) {
		// Given the runtime, after resume, rebuilds its permission
		// state from the kept grants,
		// When the kept grants are translated to PermissionRules,
		// Then they slot into PERM-005 PermissionHookChain and PERM-004
		// PermissionPreFilter unchanged — kept grants become Allow rules.
		grants := []PreviousGrant{
			{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
		}
		res, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		require.Equal(t, 1, len(res.GrantsKept))
		// Translate kept grants → rules (simplified: each kept Allow
		// becomes an Allow pattern).
		rules := &PermissionRules{}
		for _, g := range res.GrantsKept {
			if g.Decision == PermissionAllow {
				rules.Allow = append(rules.Allow, g.ToolName)
			}
		}
		chain := NewPermissionHookChain(rules)
		assert.NotNil(t, chain)
	})
}
