package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_PermissionPreFilter(t *testing.T) {
	t.Run("Scenario_DeniedToolNeverReachesLLMContext", func(t *testing.T) {
		// Given the tenant policy denies "Bash",
		// When the harness assembles a pool with builtins,
		// Then the LLM never sees Bash — no wasted tokens on hallucinated
		// arguments, no information leak about the deny set.
		audit := NewInMemoryPrefilterAudit()
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Deny: []string{"Bash"}},
		}, audit)
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
		}))
		a.AddFilter(p.AsToolPoolFilter())
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		assert.False(t, snap.HasName("Bash"))
	})

	t.Run("Scenario_ConfirmToolRemainsVisibleForInteractiveSession", func(t *testing.T) {
		// Given the tenant policy puts "Write" in the confirm tier and
		// the session is interactive (default stance show_confirm),
		// When the harness assembles a pool,
		// Then Write is visible — the call-time confirm prompt is still
		// the gate, not the pre-filter.
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules:  &PermissionRules{Confirm: []string{"Write"}},
			Stance: PrefilterStanceShowConfirm,
		}, nil)
		f := p.AsToolPoolFilter()
		assert.True(t, f(ToolPoolEntry{Name: "Write", Source: ToolSourceBuiltin, ProviderID: "p"}))
	})

	t.Run("Scenario_ConfirmToolHiddenForUnattendedBatchRun", func(t *testing.T) {
		// Given a background subagent runs unattended (no human to
		// confirm) and the stance is hide_confirm,
		// When the pool is assembled,
		// Then Confirm-tier tools are dropped — no LLM stalling on a
		// prompt that will never be answered.
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules:  &PermissionRules{Confirm: []string{"Write"}},
			Stance: PrefilterStanceHideConfirm,
		}, nil)
		f := p.AsToolPoolFilter()
		assert.False(t, f(ToolPoolEntry{Name: "Write", Source: ToolSourceBuiltin, ProviderID: "p"}))
	})

	t.Run("Scenario_PreFilterDecisionsRecordedForObservability", func(t *testing.T) {
		// Given an audit asks "why didn't the LLM use tool X in turn N?",
		// When pre-filter decisions are recorded,
		// Then the operator can answer with the exact decision per entry.
		audit := NewInMemoryPrefilterAudit()
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Deny: []string{"Bash"}},
		}, audit)
		f := p.AsToolPoolFilter()
		f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"})
		f(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"})
		dropped := audit.DroppedEntries()
		require.Equal(t, 1, len(dropped))
		assert.Equal(t, "Bash", dropped[0].Entry.Name)
		assert.Equal(t, PermissionDeny, dropped[0].Decision)
	})

	t.Run("Scenario_PatternRuleAppliesWithSampleInputProbe", func(t *testing.T) {
		// Given a deny pattern includes a dangerous argument
		// (`execute-sql(DROP)`) and a skill provides a probe with a
		// representative input,
		// When the pre-filter evaluates,
		// Then the tool is dropped before the LLM is allowed to learn
		// the trick. Without the probe, only call-time evaluation can
		// catch it (regression to existing behavior).
		probe := func(e ToolPoolEntry) string {
			if e.Name == "execute-sql" {
				return "DROP TABLE users"
			}
			return ""
		}
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules:          &PermissionRules{Deny: []string{"execute-sql(DROP)"}},
			SampleInputFor: probe,
		}, nil)
		assert.Equal(t, PermissionDeny, p.Decide(ToolPoolEntry{Name: "execute-sql", Source: ToolSourceSkill, ProviderID: "p"}))

		pNoProbe, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Deny: []string{"execute-sql(DROP)"}},
		}, nil)
		assert.Equal(t, PermissionAllow, pNoProbe.Decide(ToolPoolEntry{Name: "execute-sql", Source: ToolSourceSkill, ProviderID: "p"}))
	})

	t.Run("Scenario_ConcurrentSubagentsShareSameRulesNoRace", func(t *testing.T) {
		// Given the parent harness spawns N parallel subagents that all
		// build pools through the same pre-filter,
		// When each subagent assembles concurrently,
		// Then audit records every decision; no race.
		audit := NewInMemoryPrefilterAudit()
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Deny: []string{"Bash"}},
		}, audit)
		done := make(chan struct{})
		for i := 0; i < 20; i++ {
			go func() {
				a := NewToolPoolAssembler()
				a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
					{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
					{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
				}))
				a.AddFilter(p.AsToolPoolFilter())
				_, _ = a.Assemble(context.Background(), "iter-x")
				done <- struct{}{}
			}()
		}
		for i := 0; i < 20; i++ {
			<-done
		}
		assert.Equal(t, 40, len(audit.Snapshot()))
	})

	t.Run("Scenario_NilAuditDoesNotPreventFiltering", func(t *testing.T) {
		// Given the caller does not care about the audit trail,
		// When the pre-filter runs with nil audit,
		// Then filtering still works (audit is optional, decisions are not).
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Deny: []string{"Bash"}},
		}, nil)
		f := p.AsToolPoolFilter()
		assert.False(t, f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"}))
	})

	t.Run("Scenario_AllowListEnforcedWhenAllowRulesPresent", func(t *testing.T) {
		// Given a tenant defines Allow rules (becomes an allowlist),
		// When the pre-filter evaluates a non-listed tool,
		// Then the tool is dropped — matches existing EvaluatePermission
		// semantics where Allow-with-no-match returns Deny.
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Allow: []string{"Read"}},
		}, nil)
		f := p.AsToolPoolFilter()
		assert.True(t, f(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"}))
		assert.False(t, f(ToolPoolEntry{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "p"}))
	})

	t.Run("Scenario_WildcardDenyDropsEntirePool", func(t *testing.T) {
		// Given a tenant in lockdown denies "*",
		// When the pool is assembled,
		// Then every tool is dropped — defensive default for incident
		// response.
		audit := NewInMemoryPrefilterAudit()
		p, _ := NewPermissionPreFilter(PrefilterConfig{
			Rules: &PermissionRules{Deny: []string{"*"}},
		}, audit)
		a := NewToolPoolAssembler()
		a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
			{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "h"},
			{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "h"},
		}))
		a.AddFilter(p.AsToolPoolFilter())
		snap, err := a.Assemble(context.Background(), "iter-1")
		require.NoError(t, err)
		assert.Empty(t, snap.Entries)
		assert.ElementsMatch(t, []string{"Bash", "Edit", "Read"}, snap.DroppedNames)
	})
}
