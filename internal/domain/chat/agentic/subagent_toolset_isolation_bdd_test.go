package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_SubagentToolsetIsolation(t *testing.T) {
	t.Run("Scenario_DocumentationSubagentSeesOnlyReadOnly", func(t *testing.T) {
		// Given a documentation-generator subagent never needs Bash/Edit,
		// And the parent has Read, Grep, Glob, Bash, Edit,
		// When the parent spawns it with explicit_allowlist=[Read,Grep,Glob],
		// Then the subagent sees ONLY the three read-only tools.
		parent := []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Grep", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Glob", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetExplicitAllowlist,
			AllowedToolNames: []string{"Read", "Grep", "Glob"},
		}
		kept, _, err := ResolveSubagentToolset(parent, policy, 1)
		require.NoError(t, err)
		assert.Equal(t, 3, len(kept))
	})

	t.Run("Scenario_RecursionSafetyAgentToolHiddenDeep", func(t *testing.T) {
		// Given an `Agent` tool can recursively spawn subagents,
		// And the policy excludes it at depth > 1,
		// When a depth-2 subagent's pool is assembled,
		// Then the Agent tool is gone — no infinite recursion possible.
		parent := []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Agent", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetDepthFiltered,
			ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1},
		}
		kept, _, err := ResolveSubagentToolset(parent, policy, 2)
		require.NoError(t, err)
		for _, e := range kept {
			assert.NotEqual(t, "Agent", e.Name)
		}
	})

	t.Run("Scenario_AdminToolsBlockedCategoryWideAtAnyDepth", func(t *testing.T) {
		// Given the platform exposes admin_* tools that only the parent
		// agent should ever use,
		// And the policy excludes anything starting with "admin_",
		// When any subagent at any depth assembles,
		// Then no admin_* tool is visible.
		parent := []ToolPoolEntry{
			{Name: "admin_create_tenant", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "admin_delete_user", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetCategoricalExclusion,
			CategoryPrefixesExcluded: []string{"admin_"},
		}
		for depth := 1; depth <= 5; depth++ {
			kept, _, _ := ResolveSubagentToolset(parent, policy, depth)
			for _, e := range kept {
				assert.NotContains(t, e.Name, "admin_")
			}
		}
	})

	t.Run("Scenario_ParentMinusBlocklistKeepsBroadAccessButRemovesSharpEdges", func(t *testing.T) {
		// Given a research subagent should inherit the parent's pool
		// EXCEPT Bash and Write (sharp edges),
		// When parent_minus_blocklist is used,
		// Then everything else flows through.
		parent := []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Write", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "document_search", Source: ToolSourceSkill, ProviderID: "kb"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetParentMinusBlocklist,
			BlockedToolNames: []string{"Bash", "Write"},
		}
		kept, res, _ := ResolveSubagentToolset(parent, policy, 1)
		assert.Equal(t, 2, len(kept))
		assert.ElementsMatch(t, []string{"Bash", "Write"}, res.DroppedToolNames)
	})

	t.Run("Scenario_FilterAdapterWiresIntoToolPoolAssembler", func(t *testing.T) {
		// Given TOOL-003 ToolPoolAssembler accepts ToolPoolFilter,
		// When the SUB-005 policy emits AsToolPoolFilter,
		// Then the subagent's pool snapshot is filtered at assembly
		// time — no separate plumbing needed.
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Agent", Source: ToolSourceBuiltin, ProviderID: "h"},
		}))
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetDepthFiltered,
			ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1},
		}
		filter, err := policy.AsToolPoolFilter(2)
		require.NoError(t, err)
		a.AddFilter(filter)
		snap, err := a.Assemble(context.Background(), "iter-deep")
		require.NoError(t, err)
		assert.False(t, snap.HasName("Agent"))
		assert.True(t, snap.HasName("Read"))
	})

	t.Run("Scenario_DroppedNamesEnableAuditOfWhyToolMissing", func(t *testing.T) {
		// Given a security review asks "why didn't subagent N see tool X?",
		// When the SubagentToolsetResolution is captured,
		// Then DroppedToolNames lists exactly the names removed by this
		// policy (separate from other filters).
		parent := []ToolPoolEntry{
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetExplicitAllowlist,
			AllowedToolNames: []string{"Read"},
		}
		_, res, _ := ResolveSubagentToolset(parent, policy, 1)
		assert.Equal(t, []string{"Bash"}, res.DroppedToolNames)
	})

	t.Run("Scenario_DepthZeroParentSeesEverythingByDefault", func(t *testing.T) {
		// Given the parent agent is at depth 0,
		// And depth-filtered policy excludes Agent at depth > 1,
		// When the policy applies at depth 0,
		// Then Agent is still visible (parent retains all rights).
		parent := []ToolPoolEntry{
			{Name: "Agent", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetDepthFiltered,
			ExcludedAtDepthGreaterThan: map[string]int{"Agent": 1},
		}
		kept, _, _ := ResolveSubagentToolset(parent, policy, 0)
		assert.Equal(t, 1, len(kept))
	})

	t.Run("Scenario_PolicyFeedsBothResolveAndAsFilterPaths", func(t *testing.T) {
		// Given the same policy is used twice (one for an audit log,
		// one for runtime filtering),
		// When both paths run on the same inputs,
		// Then they agree on which tools survive.
		parent := []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetParentMinusBlocklist,
			BlockedToolNames: []string{"Bash"},
		}
		// Path 1: Resolve for audit.
		kept, _, _ := ResolveSubagentToolset(parent, policy, 1)
		// Path 2: AsToolPoolFilter for runtime.
		filter, _ := policy.AsToolPoolFilter(1)
		runtimeKept := []ToolPoolEntry{}
		for _, e := range parent {
			if filter(e) {
				runtimeKept = append(runtimeKept, e)
			}
		}
		assert.Equal(t, len(kept), len(runtimeKept))
		for i := range kept {
			assert.Equal(t, kept[i].Name, runtimeKept[i].Name)
		}
	})

	t.Run("Scenario_InputsImmutableSafeForConcurrentSpawn", func(t *testing.T) {
		// Given the parent spawns 50 subagents concurrently from the
		// same pool snapshot,
		// When each spawn calls ResolveSubagentToolset,
		// Then the source pool is untouched.
		parent := []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetExplicitAllowlist,
			AllowedToolNames: []string{"Read"},
		}
		originalParent := append([]ToolPoolEntry(nil), parent...)
		done := make(chan struct{})
		for i := 0; i < 50; i++ {
			go func() {
				_, _, _ = ResolveSubagentToolset(parent, policy, 1)
				done <- struct{}{}
			}()
		}
		for i := 0; i < 50; i++ {
			<-done
		}
		assert.Equal(t, originalParent, parent)
	})

	t.Run("Scenario_CombinedPrefixAndSuffixCatchesBoth", func(t *testing.T) {
		// Given two naming conventions for sensitive tools (prefix
		// "admin_" and suffix "_dangerous"),
		// When categorical_exclusion lists both patterns,
		// Then both are blocked.
		parent := []ToolPoolEntry{
			{Name: "admin_x", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "y_dangerous", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
		}
		policy := SubagentToolsetPolicy{
			Mode: SubagentToolsetCategoricalExclusion,
			CategoryPrefixesExcluded: []string{"admin_"},
			CategorySuffixesExcluded: []string{"_dangerous"},
		}
		kept, _, _ := ResolveSubagentToolset(parent, policy, 1)
		assert.Equal(t, 1, len(kept))
		assert.Equal(t, "Read", kept[0].Name)
	})
}
