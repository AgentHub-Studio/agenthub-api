package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubagentPerm_IsValidMode(t *testing.T) {
	for _, m := range allPermissionInheritanceModes {
		assert.True(t, IsValidPermissionInheritanceMode(m))
	}
	assert.False(t, IsValidPermissionInheritanceMode(PermissionInheritanceMode("nope")))
}

func TestSubagentPerm_ResolveRejectsBadMode(t *testing.T) {
	_, err := ResolveSubagentPermissions(nil, nil, PermissionInheritanceMode("nope"))
	assert.ErrorIs(t, err, ErrSubagentPermBadMode)
}

func TestSubagentPerm_InheritAllUnionsAllows(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Read"}}
	child := &PermissionRules{Allow: []string{"Edit"}}
	res, err := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Edit", "Read"}, res.EffectiveRules.Allow)
}

func TestSubagentPerm_InheritAllUnionsDenies(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	child := &PermissionRules{Deny: []string{"Write"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.ElementsMatch(t, []string{"Bash", "Write"}, res.EffectiveRules.Deny)
}

func TestSubagentPerm_InheritAllDeduplicates(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Read"}}
	child := &PermissionRules{Allow: []string{"Read"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.Equal(t, []string{"Read"}, res.EffectiveRules.Allow)
}

func TestSubagentPerm_InheritAllChildModeWins(t *testing.T) {
	parent := &PermissionRules{Mode: PermissionModeDefault}
	child := &PermissionRules{Mode: PermissionModeBypass}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.Equal(t, PermissionModeBypass, res.EffectiveRules.Mode)
}

func TestSubagentPerm_InheritAllParentModeUsedWhenChildEmpty(t *testing.T) {
	parent := &PermissionRules{Mode: PermissionModeBypass}
	child := &PermissionRules{}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.Equal(t, PermissionModeBypass, res.EffectiveRules.Mode)
}

func TestSubagentPerm_InheritStrictOnlyKeepsParentDeny(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}, Allow: []string{"Read"}}
	child := &PermissionRules{Allow: []string{"Edit"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritStrictOnly)
	// Allow comes only from child.
	assert.Equal(t, []string{"Edit"}, res.EffectiveRules.Allow)
	// Parent deny is inherited.
	assert.Contains(t, res.EffectiveRules.Deny, "Bash")
}

func TestSubagentPerm_InheritStrictOnlyDenyUnion(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	child := &PermissionRules{Deny: []string{"Write"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritStrictOnly)
	assert.ElementsMatch(t, []string{"Bash", "Write"}, res.EffectiveRules.Deny)
}

func TestSubagentPerm_OverrideReplaceIgnoresParent(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}, Allow: []string{"Read"}}
	child := &PermissionRules{Allow: []string{"Edit"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionOverrideReplace)
	// Parent Bash deny is GONE — caller took explicit responsibility.
	assert.NotContains(t, res.EffectiveRules.Deny, "Bash")
	assert.Equal(t, []string{"Edit"}, res.EffectiveRules.Allow)
}

func TestSubagentPerm_OverrideReplaceNilChildProducesEmpty(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	res, err := ResolveSubagentPermissions(parent, nil, PermissionOverrideReplace)
	require.NoError(t, err)
	require.NotNil(t, res.EffectiveRules)
	assert.Empty(t, res.EffectiveRules.Deny)
	assert.Empty(t, res.EffectiveRules.Allow)
}

func TestSubagentPerm_MergeIntersectShrinksAllows(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Read", "Edit"}}
	child := &PermissionRules{Allow: []string{"Read"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionMergeIntersect)
	assert.Equal(t, []string{"Read"}, res.EffectiveRules.Allow)
	assert.Equal(t, []string{"Edit"}, res.DroppedAllows)
}

func TestSubagentPerm_MergeIntersectDenyStillUnioned(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	child := &PermissionRules{Deny: []string{"Write"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionMergeIntersect)
	assert.ElementsMatch(t, []string{"Bash", "Write"}, res.EffectiveRules.Deny)
}

func TestSubagentPerm_MergeIntersectEmptyAllowOverlapNil(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Read"}}
	child := &PermissionRules{Allow: []string{"Edit"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionMergeIntersect)
	assert.Empty(t, res.EffectiveRules.Allow)
	assert.Equal(t, []string{"Read"}, res.DroppedAllows)
}

func TestSubagentPerm_BothNilReturnsNil(t *testing.T) {
	res, err := ResolveSubagentPermissions(nil, nil, PermissionInheritAll)
	require.NoError(t, err)
	assert.Nil(t, res.EffectiveRules)
}

func TestSubagentPerm_ParentOnlyNilChildInheritAll(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	res, _ := ResolveSubagentPermissions(parent, nil, PermissionInheritAll)
	assert.Equal(t, []string{"Bash"}, res.EffectiveRules.Deny)
}

func TestSubagentPerm_ChildOnlyNilParent(t *testing.T) {
	child := &PermissionRules{Allow: []string{"Read"}}
	res, _ := ResolveSubagentPermissions(nil, child, PermissionInheritAll)
	assert.Equal(t, []string{"Read"}, res.EffectiveRules.Allow)
}

func TestSubagentPerm_InputsNotMutated(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Read"}, Deny: []string{"Bash"}}
	child := &PermissionRules{Allow: []string{"Edit"}, Deny: []string{"Write"}}
	originalParentAllow := append([]string(nil), parent.Allow...)
	originalChildAllow := append([]string(nil), child.Allow...)
	_, _ = ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.Equal(t, originalParentAllow, parent.Allow)
	assert.Equal(t, originalChildAllow, child.Allow)
}

func TestSubagentPerm_AddedDeniesAuditField(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	child := &PermissionRules{Deny: []string{"Bash", "Write"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.Equal(t, []string{"Write"}, res.AddedDenies)
}

func TestSubagentPerm_AddedAllowsAuditField(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Read"}}
	child := &PermissionRules{Allow: []string{"Edit"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	assert.Equal(t, []string{"Edit"}, res.AddedAllows)
}

func TestSubagentPerm_ReasonSummaryPopulated(t *testing.T) {
	for _, mode := range allPermissionInheritanceModes {
		res, err := ResolveSubagentPermissions(nil, &PermissionRules{Allow: []string{"x"}}, mode)
		require.NoError(t, err)
		assert.NotEmpty(t, res.ReasonSummary, "mode %q missing reason", mode)
	}
}

func TestSubagentPerm_OutputDeterministicSorted(t *testing.T) {
	parent := &PermissionRules{Allow: []string{"Zeta", "Alpha", "Mu"}}
	res, _ := ResolveSubagentPermissions(parent, nil, PermissionInheritAll)
	assert.Equal(t, []string{"Alpha", "Mu", "Zeta"}, res.EffectiveRules.Allow)
}

func TestSubagentPerm_ResolutionSnapshotPreservesInputs(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	child := &PermissionRules{Allow: []string{"Read"}}
	res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
	require.NotNil(t, res.ParentRules)
	require.NotNil(t, res.ChildRules)
	assert.Equal(t, []string{"Bash"}, res.ParentRules.Deny)
	assert.Equal(t, []string{"Read"}, res.ChildRules.Allow)
}

func TestSubagentPerm_InheritStrictOnlyEmptyChildKeepsParentDeny(t *testing.T) {
	parent := &PermissionRules{Deny: []string{"Bash"}}
	res, _ := ResolveSubagentPermissions(parent, nil, PermissionInheritStrictOnly)
	require.NotNil(t, res.EffectiveRules)
	assert.Equal(t, []string{"Bash"}, res.EffectiveRules.Deny)
}
