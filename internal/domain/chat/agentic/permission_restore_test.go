package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermRestore_IsValidPolicy(t *testing.T) {
	for _, p := range allPermissionRestorePolicies {
		assert.True(t, IsValidPermissionRestorePolicy(p))
	}
	assert.False(t, IsValidPermissionRestorePolicy(PermissionRestorePolicy("nope")))
}

func TestPermRestore_IsValidDurability(t *testing.T) {
	for _, d := range allGrantDurabilities {
		assert.True(t, IsValidGrantDurability(d))
	}
	assert.False(t, IsValidGrantDurability(GrantDurability("nope")))
}

func TestPermRestore_IsValidOperation(t *testing.T) {
	for _, o := range allRestoreOperations {
		assert.True(t, IsValidRestoreOperation(o))
	}
	assert.False(t, IsValidRestoreOperation(RestoreOperation("nope")))
}

func TestPermRestore_GrantValidateEmptyTool(t *testing.T) {
	g := PreviousGrant{Durability: GrantDurabilityPersisted}
	assert.ErrorIs(t, g.Validate(), ErrPermissionRestoreEmptyToolName)
}

func TestPermRestore_GrantValidateBadDurability(t *testing.T) {
	g := PreviousGrant{ToolName: "Bash", Durability: GrantDurability("nope")}
	assert.ErrorIs(t, g.Validate(), ErrPermissionRestoreBadDurability)
}

func TestPermRestore_ContextValidateBadOperation(t *testing.T) {
	c := RestoreContext{Operation: RestoreOperation("nope")}
	assert.ErrorIs(t, c.Validate(), ErrPermissionRestoreBadOperation)
}

func TestPermRestore_ContextValidateNegativeGap(t *testing.T) {
	c := RestoreContext{Operation: RestoreOperationResume, GapSeconds: -1}
	assert.ErrorIs(t, c.Validate(), ErrPermissionRestoreNegativeGap)
}

func TestPermRestore_FilterRejectsBadPolicy(t *testing.T) {
	_, err := FilterRestorableGrants(
		PermissionRestorePolicy("nope"),
		RestoreContext{Operation: RestoreOperationResume},
		nil)
	assert.ErrorIs(t, err, ErrPermissionRestoreBadPolicy)
}

func TestPermRestore_FilterRejectsBadContext(t *testing.T) {
	_, err := FilterRestorableGrants(
		PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperation("nope")},
		nil)
	assert.ErrorIs(t, err, ErrPermissionRestoreBadOperation)
}

func TestPermRestore_FilterRejectsBadGrant(t *testing.T) {
	_, err := FilterRestorableGrants(
		PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume},
		[]PreviousGrant{{Durability: GrantDurabilityPersisted}}) // empty tool name
	assert.ErrorIs(t, err, ErrPermissionRestoreEmptyToolName)
}

func TestPermRestore_DiscardAllDropsAllow(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityPersisted},
		{ToolName: "Write", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
	}
	res, err := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	require.NoError(t, err)
	assert.Empty(t, res.GrantsKept)
	assert.Equal(t, 2, len(res.GrantsDropped))
}

func TestPermRestore_DenyGrantsAlwaysSurvive(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionDeny, Durability: GrantDurabilitySessionScoped},
	}
	for _, policy := range allPermissionRestorePolicies {
		res, err := FilterRestorableGrants(policy,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res.GrantsKept), "policy %q dropped a deny grant", policy)
	}
}

func TestPermRestore_OneShotNeverSurvives(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityOneShot},
	}
	for _, policy := range allPermissionRestorePolicies {
		res, err := FilterRestorableGrants(policy,
			RestoreContext{Operation: RestoreOperationResume}, grants)
		require.NoError(t, err)
		assert.Empty(t, res.GrantsKept, "policy %q kept a one_shot grant", policy)
	}
}

func TestPermRestore_PreserveDurableOnlyKeepsPersisted(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityPersisted},
		{ToolName: "Edit", Decision: PermissionAllow, Durability: GrantDurabilitySessionScoped},
	}
	res, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	require.Equal(t, 1, len(res.GrantsKept))
	assert.Equal(t, "Bash", res.GrantsKept[0].ToolName)
}

func TestPermRestore_PreserveDurableOnlyKeepsAdmin(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
	}
	res, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	assert.Equal(t, 1, len(res.GrantsKept))
}

func TestPermRestore_PreserveExplicitOnlyKeepsAdmin(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityPersisted},
		{ToolName: "Edit", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
	}
	res, _ := FilterRestorableGrants(PermissionRestorePreserveExplicitGrants,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	require.Equal(t, 1, len(res.GrantsKept))
	assert.Equal(t, "Edit", res.GrantsKept[0].ToolName)
}

func TestPermRestore_StrictReRequestDropsAllAndFlags(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
	}
	res, _ := FilterRestorableGrants(PermissionRestoreStrictReRequest,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	assert.Empty(t, res.GrantsKept)
	assert.True(t, res.RequireFirstUseConfirmation)
}

func TestPermRestore_DiscardAllDoesNotFlagFirstUse(t *testing.T) {
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, nil)
	assert.False(t, res.RequireFirstUseConfirmation)
}

func TestPermRestore_AuditRecordsAllDecisions(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilitySessionScoped},
		{ToolName: "Write", Decision: PermissionDeny, Durability: GrantDurabilityPersisted},
		{ToolName: "Edit", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
	}
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	require.Equal(t, 3, len(res.Decisions))
	// Verify each grant has a decision record.
	tools := map[string]bool{}
	for _, d := range res.Decisions {
		tools[d.Grant.ToolName] = true
		assert.NotEmpty(t, d.Reason)
	}
	assert.True(t, tools["Bash"])
	assert.True(t, tools["Write"])
	assert.True(t, tools["Edit"])
}

func TestPermRestore_GrantsConsideredCount(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilitySessionScoped},
		{ToolName: "Edit", Decision: PermissionAllow, Durability: GrantDurabilityPersisted},
	}
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	assert.Equal(t, 2, res.GrantsConsidered)
}

func TestPermRestore_GrantsKeptSortedDeterministic(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Zeta", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
		{ToolName: "Alpha", Decision: PermissionAllow, Durability: GrantDurabilityExplicitAdmin},
	}
	res, _ := FilterRestorableGrants(PermissionRestorePreserveDurableOnly,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	require.Equal(t, 2, len(res.GrantsKept))
	assert.Equal(t, "Alpha", res.GrantsKept[0].ToolName)
	assert.Equal(t, "Zeta", res.GrantsKept[1].ToolName)
}

func TestPermRestore_ContextPropagatedToResolution(t *testing.T) {
	ctx := RestoreContext{
		Operation:       RestoreOperationFork,
		OriginalSession: "sess-orig",
		NewSession:      "sess-fork",
		GapSeconds:      3600,
	}
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll, ctx, nil)
	assert.Equal(t, ctx, res.Context)
}

func TestPermRestore_EmptyGrantsReturnsEmpty(t *testing.T) {
	res, err := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, res.GrantsConsidered)
	assert.Empty(t, res.GrantsKept)
	assert.Empty(t, res.GrantsDropped)
}

func TestPermRestore_OneShotDeniedRemainsKeptByDenyRule(t *testing.T) {
	// Edge case: a one_shot grant with Decision=Deny — deny rule
	// outranks the one_shot drop rule (deny always preserved).
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionDeny, Durability: GrantDurabilityOneShot},
	}
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	assert.Equal(t, 1, len(res.GrantsKept))
}

func TestPermRestore_ConfirmGrantsTreatedAsAllow(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Write", Decision: PermissionConfirm, Durability: GrantDurabilityPersisted},
	}
	// Discard-all drops confirm grants the same as allow.
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	assert.Empty(t, res.GrantsKept)
}

func TestPermRestore_InputsNotMutated(t *testing.T) {
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityPersisted},
	}
	originalName := grants[0].ToolName
	_, _ = FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	assert.Equal(t, originalName, grants[0].ToolName)
}

func TestPermRestore_DecisionsCarryGrantPayload(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	grants := []PreviousGrant{
		{ToolName: "Bash", Decision: PermissionAllow, Durability: GrantDurabilityPersisted,
			GrantedAt: now, GrantedBy: "alice"},
	}
	res, _ := FilterRestorableGrants(PermissionRestoreDiscardAll,
		RestoreContext{Operation: RestoreOperationResume}, grants)
	require.Equal(t, 1, len(res.Decisions))
	assert.Equal(t, "Bash", res.Decisions[0].Grant.ToolName)
	assert.Equal(t, now, res.Decisions[0].Grant.GrantedAt)
	assert.Equal(t, "alice", res.Decisions[0].Grant.GrantedBy)
}
