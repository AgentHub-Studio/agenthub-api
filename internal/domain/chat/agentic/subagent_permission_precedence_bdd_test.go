package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_SubagentPermissionPrecedence(t *testing.T) {
	t.Run("Scenario_SubagentExtendsParentWithExtraAllows", func(t *testing.T) {
		// Given a parent allows Read and the subagent needs Edit too,
		// When inherit_all is used,
		// Then the subagent sees BOTH allows.
		parent := &PermissionRules{Allow: []string{"Read"}}
		child := &PermissionRules{Allow: []string{"Edit"}}
		res, err := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Edit", "Read"}, res.EffectiveRules.Allow)
	})

	t.Run("Scenario_ParentDenyAlwaysCarriedToSubagent", func(t *testing.T) {
		// Given the parent denies Bash and the child only declares allows,
		// When inherit_all merges,
		// Then Bash remains denied — child cannot relax parent's deny set.
		parent := &PermissionRules{Deny: []string{"Bash"}}
		child := &PermissionRules{Allow: []string{"Read"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		assert.Contains(t, res.EffectiveRules.Deny, "Bash")
	})

	t.Run("Scenario_InheritStrictOnlyForSandboxedWorker", func(t *testing.T) {
		// Given a sandboxed worker should keep parent denies but define
		// its own allows and confirms,
		// When inherit_strict_only is used,
		// Then allow=child only, deny=parent ∪ child.
		parent := &PermissionRules{
			Allow: []string{"Read"}, Deny: []string{"Bash"},
		}
		child := &PermissionRules{Allow: []string{"Edit"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritStrictOnly)
		assert.Equal(t, []string{"Edit"}, res.EffectiveRules.Allow)
		assert.Contains(t, res.EffectiveRules.Deny, "Bash")
	})

	t.Run("Scenario_OverrideReplaceForIsolatedWorkerExplicitlyDeCoupled", func(t *testing.T) {
		// Given the parent reviewed and signed-off an isolated worker,
		// When override_replace is used,
		// Then parent rules are IGNORED — only child rules apply.
		parent := &PermissionRules{Deny: []string{"Bash"}}
		child := &PermissionRules{Allow: []string{"Read"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionOverrideReplace)
		assert.NotContains(t, res.EffectiveRules.Deny, "Bash")
		assert.Equal(t, []string{"Read"}, res.EffectiveRules.Allow)
	})

	t.Run("Scenario_MergeIntersectShrinksAllowsToCommonGround", func(t *testing.T) {
		// Given an audit-strict tenant: subagent must be at LEAST as
		// restrictive as parent for allows,
		// When merge_intersect is used,
		// Then only tools allowed by BOTH survive; the rest go into
		// DroppedAllows for audit.
		parent := &PermissionRules{Allow: []string{"Read", "Edit"}}
		child := &PermissionRules{Allow: []string{"Read"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionMergeIntersect)
		assert.Equal(t, []string{"Read"}, res.EffectiveRules.Allow)
		assert.Equal(t, []string{"Edit"}, res.DroppedAllows)
	})

	t.Run("Scenario_AddedDeniesRecordedForAudit", func(t *testing.T) {
		// Given compliance asks "what new denies did the subagent add?",
		// When the resolver runs,
		// Then AddedDenies lists exactly the new patterns.
		parent := &PermissionRules{Deny: []string{"Bash"}}
		child := &PermissionRules{Deny: []string{"Bash", "Write", "Edit"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		assert.ElementsMatch(t, []string{"Edit", "Write"}, res.AddedDenies)
	})

	t.Run("Scenario_DroppedAllowsRecordedForIntersect", func(t *testing.T) {
		// Given the auditor wants to know what allows the subagent gave
		// up vs the parent,
		// When merge_intersect is used,
		// Then DroppedAllows lists them.
		parent := &PermissionRules{Allow: []string{"Read", "Edit", "Bash"}}
		child := &PermissionRules{Allow: []string{"Read"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionMergeIntersect)
		assert.ElementsMatch(t, []string{"Bash", "Edit"}, res.DroppedAllows)
	})

	t.Run("Scenario_ChildModeOverridesParentMode", func(t *testing.T) {
		// Given the parent is in default mode but the subagent should
		// run in bypass (e.g., a sandbox auto-allow worker),
		// When inherit_all is used,
		// Then mode comes from the child.
		parent := &PermissionRules{Mode: PermissionModeDefault}
		child := &PermissionRules{Mode: PermissionModeBypass}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		assert.Equal(t, PermissionModeBypass, res.EffectiveRules.Mode)
	})

	t.Run("Scenario_InputsNeverMutated", func(t *testing.T) {
		// Given the parent's rules are shared across many subagent
		// spawns,
		// When the resolver runs N times,
		// Then the input rule slices are never modified — defensive
		// against accidental cross-contamination.
		parent := &PermissionRules{Allow: []string{"Read"}, Deny: []string{"Bash"}}
		originalAllow := append([]string(nil), parent.Allow...)
		originalDeny := append([]string(nil), parent.Deny...)
		for i := 0; i < 5; i++ {
			_, _ = ResolveSubagentPermissions(parent,
				&PermissionRules{Allow: []string{"X"}},
				PermissionInheritAll)
		}
		assert.Equal(t, originalAllow, parent.Allow)
		assert.Equal(t, originalDeny, parent.Deny)
	})

	t.Run("Scenario_AuditOutputIsDeterministic", func(t *testing.T) {
		// Given two spawns with the same inputs,
		// When the resolver runs,
		// Then the effective rule slices are byte-for-byte identical
		// (sorted output → reproducible audit hashes).
		parent := &PermissionRules{Allow: []string{"Zeta", "Alpha"}}
		child := &PermissionRules{Allow: []string{"Mu"}}
		r1, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		r2, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		assert.Equal(t, r1.EffectiveRules.Allow, r2.EffectiveRules.Allow)
	})

	t.Run("Scenario_ResolutionFeedsHookChainAndPreFilter", func(t *testing.T) {
		// Given the resolver output should drop straight into PERM-005
		// PermissionHookChain and PERM-004 PermissionPreFilter,
		// When we wire them,
		// Then EffectiveRules is a regular *PermissionRules — no adapter
		// needed. Demonstrated by passing it into NewPermissionHookChain.
		parent := &PermissionRules{Deny: []string{"Bash"}}
		child := &PermissionRules{Allow: []string{"Read"}}
		res, _ := ResolveSubagentPermissions(parent, child, PermissionInheritAll)
		chain := NewPermissionHookChain(res.EffectiveRules)
		assert.NotNil(t, chain)
		// The chain is ready to use; we don't invoke Evaluate here
		// because the integration is type-level, not behaviour-level.
	})
}
