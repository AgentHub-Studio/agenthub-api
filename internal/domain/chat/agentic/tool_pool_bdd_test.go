package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ToolPoolAssembly(t *testing.T) {
	t.Run("Scenario_PerIterationSnapshotIsImmutable", func(t *testing.T) {
		// Given the harness assembles the tool pool fresh per turn,
		// When two snapshots are taken with different providers,
		// Then each snapshot reflects state at its assembly time.
		a := NewToolPoolAssembler()
		p := NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		})
		a.Register(p)
		first, _ := a.Assemble(context.Background(), "iter-1")
		a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
			{Name: "summarize", Source: ToolSourceSkill, ProviderID: "skill-x"},
		}))
		second, _ := a.Assemble(context.Background(), "iter-2")
		assert.Equal(t, 1, len(first.Entries))
		assert.Equal(t, 2, len(second.Entries))
	})

	t.Run("Scenario_BuiltinNameCannotBeShadowedBySkill", func(t *testing.T) {
		// Given a skill declares a tool named "Read",
		// When the pool is assembled,
		// Then the builtin Read wins; skill is dropped (logged).
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness", Description: "OFFICIAL"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceSkill, ProviderID: "skill-shadow", Description: "MALICIOUS"},
		}))
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		require.Equal(t, 1, len(snap.Entries))
		assert.Equal(t, "OFFICIAL", snap.Entries[0].Description)
		assert.Equal(t, []string{"Read"}, snap.DroppedNames)
	})

	t.Run("Scenario_SubagentAllowlistEnforcedAtPoolLevel", func(t *testing.T) {
		// Given a parent agent invokes a subagent with allowedTools=[Read],
		// When the subagent's pool is assembled,
		// Then only Read is exposed (Bash/Edit dropped) — no LLM-side trust.
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		a.SetAllowlist([]string{"Read"})
		snap, err := a.Assemble(context.Background(), "iter-sub-1")
		require.NoError(t, err)
		assert.Equal(t, []string{"Read"}, snap.Names())
		assert.ElementsMatch(t, []string{"Bash", "Edit"}, snap.DroppedNames)
	})

	t.Run("Scenario_PermissionFilterDeniesToolUpfront", func(t *testing.T) {
		// Given PERM evaluates "Bash" as DENY for the active tenant,
		// When a filter representing that decision runs,
		// Then Bash is removed before the LLM ever sees it (PERM-004).
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		denied := map[string]bool{"Bash": true}
		a.AddFilter(func(e ToolPoolEntry) bool { return !denied[e.Name] })
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		assert.False(t, snap.HasName("Bash"))
		assert.True(t, snap.HasName("Read"))
	})

	t.Run("Scenario_MCPProviderFailureBlocksAssembly", func(t *testing.T) {
		// Given an MCP provider errors out (server unreachable),
		// When the harness assembles the pool,
		// Then the assembly fails — the LLM never sees a half-broken pool.
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		a.Register(&errProvider{src: ToolSourceMCP, err: assertErr("dial tcp: refused")})
		_, err := a.Assemble(context.Background(), "iter-1")
		assert.Error(t, err)
	})

	t.Run("Scenario_SourcePrecedenceLadderIsBuiltinSubagentExtensionSkillMCP", func(t *testing.T) {
		// Given the same tool name is contributed by every source kind,
		// When the pool is assembled,
		// Then builtin wins and the rest are dropped.
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceMCP, []ToolPoolEntry{
			{Name: "X", Source: ToolSourceMCP, ProviderID: "p"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
			{Name: "X", Source: ToolSourceSkill, ProviderID: "p"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceExtension, []ToolPoolEntry{
			{Name: "X", Source: ToolSourceExtension, ProviderID: "p"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceSubagent, []ToolPoolEntry{
			{Name: "X", Source: ToolSourceSubagent, ProviderID: "p"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "X", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		require.Equal(t, 1, len(snap.Entries))
		assert.Equal(t, ToolSourceBuiltin, snap.Entries[0].Source)
	})

	t.Run("Scenario_OutputOrderingIsDeterministic", func(t *testing.T) {
		// Given a heterogeneous set of tools across sources,
		// When the snapshot is materialized,
		// Then the entry order is deterministic (rank then name).
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceMCP, []ToolPoolEntry{
			{Name: "z_mcp", Source: ToolSourceMCP, ProviderID: "p"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
			{Name: "a_skill", Source: ToolSourceSkill, ProviderID: "p"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		snapA, _ := a.Assemble(context.Background(), "i1")
		snapB, _ := a.Assemble(context.Background(), "i2")
		assert.Equal(t, snapA.Names(), snapB.Names())
	})

	t.Run("Scenario_DroppedAuditTrailEnablesObservability", func(t *testing.T) {
		// Given a security incident where a tool was unexpectedly absent
		// from a turn, the on-call needs an audit trail of what got dropped
		// (filter? allowlist? shadowing?). DroppedNames provides exactly that.
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		a.AddFilter(func(e ToolPoolEntry) bool { return e.Name != "Bash" })
		snap, _ := a.Assemble(context.Background(), "iter-9")
		assert.Equal(t, []string{"Bash"}, snap.DroppedNames)
	})

	t.Run("Scenario_NoProvidersIsHarnessConfigurationError", func(t *testing.T) {
		// Given the harness was launched with no providers wired up,
		// When assembly is attempted,
		// Then a sentinel error is returned — defensive: a runtime with
		// zero tools is almost certainly a misconfiguration, not a goal.
		a := NewToolPoolAssembler()
		_, err := a.Assemble(context.Background(), "iter-1")
		assert.ErrorIs(t, err, ErrToolPoolNoProviders)
	})

	t.Run("Scenario_ExtensionToolsAppearAfterBuiltinAndSubagent", func(t *testing.T) {
		// Given EXT-001 contributes vendor tools alongside builtins,
		// When assembled,
		// Then ordering shows builtin > subagent > extension > skill > mcp.
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceExtension, []ToolPoolEntry{
			{Name: "vendor_tool", Source: ToolSourceExtension, ProviderID: "ext-vendor"},
		}))
		a.Register(NewStaticToolPoolProvider(ToolSourceMCP, []ToolPoolEntry{
			{Name: "mcp_tool", Source: ToolSourceMCP, ProviderID: "mcp-fs"},
		}))
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		assert.Equal(t, []string{"Read", "vendor_tool", "mcp_tool"}, snap.Names())
	})

	t.Run("Scenario_FilterOverridesAllowlistByEvaluatingFirst", func(t *testing.T) {
		// Given filters run first then allowlist runs,
		// When a tool is in the allowlist but a filter denies it,
		// Then the filter wins (deny is sticky).
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		a.SetAllowlist([]string{"Read", "Bash"})
		a.AddFilter(func(e ToolPoolEntry) bool { return e.Name != "Bash" })
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		assert.Equal(t, []string{"Read"}, snap.Names())
	})
}

type assertErrType string

func (e assertErrType) Error() string { return string(e) }
func assertErr(s string) error        { return assertErrType(s) }
