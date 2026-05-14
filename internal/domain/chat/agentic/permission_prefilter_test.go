package agentic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefilter_IsValidStance(t *testing.T) {
	for _, s := range allPrefilterStances {
		assert.True(t, IsValidPrefilterStance(s))
	}
	assert.False(t, IsValidPrefilterStance(PrefilterStance("nope")))
	assert.False(t, IsValidPrefilterStance(PrefilterStance("")))
}

func TestPrefilter_ConfigValidateRulesRequired(t *testing.T) {
	cfg := PrefilterConfig{}
	assert.ErrorIs(t, cfg.Validate(), ErrPrefilterRulesRequired)
}

func TestPrefilter_ConfigValidateBadStance(t *testing.T) {
	cfg := PrefilterConfig{
		Rules:  &PermissionRules{Deny: []string{"Bash"}},
		Stance: PrefilterStance("nope"),
	}
	assert.ErrorIs(t, cfg.Validate(), ErrPrefilterBadStance)
}

func TestPrefilter_ConfigValidateEmptyStanceOK(t *testing.T) {
	cfg := PrefilterConfig{Rules: &PermissionRules{Deny: []string{"Bash"}}}
	assert.NoError(t, cfg.Validate())
}

func TestPrefilter_NewRejectsBadConfig(t *testing.T) {
	_, err := NewPermissionPreFilter(PrefilterConfig{}, nil)
	assert.ErrorIs(t, err, ErrPrefilterRulesRequired)
}

func TestPrefilter_NewDefaultsStanceToShowConfirm(t *testing.T) {
	p, err := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, PrefilterStanceShowConfirm, p.cfg.Stance)
}

func TestPrefilter_DecideDenyTool(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, nil)
	d := p.Decide(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"})
	assert.Equal(t, PermissionDeny, d)
}

func TestPrefilter_DecideAllowTool(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, nil)
	d := p.Decide(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"})
	assert.Equal(t, PermissionAllow, d)
}

func TestPrefilter_DecideConfirmTool(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Confirm: []string{"Write"}},
	}, nil)
	d := p.Decide(ToolPoolEntry{Name: "Write", Source: ToolSourceBuiltin, ProviderID: "harness"})
	assert.Equal(t, PermissionConfirm, d)
}

func TestPrefilter_FilterDropsDeny(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, nil)
	f := p.AsToolPoolFilter()
	assert.True(t, f(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"}))
	assert.False(t, f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"}))
}

func TestPrefilter_FilterShowConfirmKeepsConfirm(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules:  &PermissionRules{Confirm: []string{"Write"}},
		Stance: PrefilterStanceShowConfirm,
	}, nil)
	f := p.AsToolPoolFilter()
	assert.True(t, f(ToolPoolEntry{Name: "Write", Source: ToolSourceBuiltin, ProviderID: "p"}))
}

func TestPrefilter_FilterHideConfirmDropsConfirm(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules:  &PermissionRules{Confirm: []string{"Write"}},
		Stance: PrefilterStanceHideConfirm,
	}, nil)
	f := p.AsToolPoolFilter()
	assert.False(t, f(ToolPoolEntry{Name: "Write", Source: ToolSourceBuiltin, ProviderID: "p"}))
}

func TestPrefilter_AuditRecordsDecisions(t *testing.T) {
	audit := NewInMemoryPrefilterAudit()
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, audit)
	f := p.AsToolPoolFilter()
	f(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"})
	f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"})
	got := audit.Snapshot()
	require.Equal(t, 2, len(got))
	assert.Equal(t, PermissionAllow, got[0].Decision)
	assert.True(t, got[0].Kept)
	assert.Equal(t, PermissionDeny, got[1].Decision)
	assert.False(t, got[1].Kept)
}

func TestPrefilter_AuditDroppedSubset(t *testing.T) {
	audit := NewInMemoryPrefilterAudit()
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, audit)
	f := p.AsToolPoolFilter()
	f(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"})
	f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"})
	dropped := audit.DroppedEntries()
	require.Equal(t, 1, len(dropped))
	assert.Equal(t, "Bash", dropped[0].Entry.Name)
}

func TestPrefilter_AuditSortedByNameThenTime(t *testing.T) {
	audit := NewInMemoryPrefilterAudit()
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, audit)
	times := []time.Time{
		time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 11, 12, 0, 1, 0, time.UTC),
	}
	i := 0
	p.SetClock(func() time.Time {
		t := times[i]
		i++
		return t
	})
	f := p.AsToolPoolFilter()
	f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"})
	f(ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"})
	got := audit.SortedDecisions()
	assert.Equal(t, "Bash", got[0].Entry.Name)
	assert.Equal(t, "Read", got[1].Entry.Name)
}

func TestPrefilter_NilAuditNoCrash(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, nil)
	f := p.AsToolPoolFilter()
	assert.False(t, f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"}))
}

func TestPrefilter_ApplyAllBatchHelper(t *testing.T) {
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, nil)
	in := []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "p"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"},
		{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "p"},
	}
	out := p.ApplyAll(in)
	assert.Equal(t, 2, len(out))
	assert.Equal(t, "Read", out[0].Name)
	assert.Equal(t, "Edit", out[1].Name)
}

func TestPrefilter_SampleInputProbeAppliesPatternMatch(t *testing.T) {
	// Rule denies `execute-sql(DROP)`. Pattern is whole-word substring
	// (case-insensitive). Without the probe, the pattern's argument
	// check never fires; with a probe returning a string containing
	// "DROP" as a whole word, the entry is dropped.
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
	d := p.Decide(ToolPoolEntry{Name: "execute-sql", Source: ToolSourceSkill, ProviderID: "p"})
	assert.Equal(t, PermissionDeny, d)
}

func TestPrefilter_IntegratesWithToolPoolAssembler(t *testing.T) {
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
	assert.Equal(t, []string{"Read"}, snap.Names())
	assert.Equal(t, []string{"Bash"}, snap.DroppedNames)
	require.Equal(t, 2, len(audit.Snapshot()))
}

func TestPrefilter_ConcurrentFilterIsSafe(t *testing.T) {
	audit := NewInMemoryPrefilterAudit()
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, audit)
	f := p.AsToolPoolFilter()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"})
		}()
	}
	wg.Wait()
	assert.Equal(t, 50, len(audit.Snapshot()))
}

func TestPrefilter_AuditTimestampsFromInjectedClock(t *testing.T) {
	stamp := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	audit := NewInMemoryPrefilterAudit()
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"Bash"}},
	}, audit)
	p.SetClock(func() time.Time { return stamp })
	f := p.AsToolPoolFilter()
	f(ToolPoolEntry{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "p"})
	got := audit.Snapshot()
	require.Equal(t, 1, len(got))
	assert.Equal(t, stamp, got[0].At)
}

func TestPrefilter_EmptyInputIgnoresArgumentPattern(t *testing.T) {
	// Rule denies execute-sql(DROP). Without a probe, the input is "",
	// the argument pattern doesn't match, so the call is Allow.
	p, _ := NewPermissionPreFilter(PrefilterConfig{
		Rules: &PermissionRules{Deny: []string{"execute-sql(DROP)"}},
	}, nil)
	d := p.Decide(ToolPoolEntry{Name: "execute-sql", Source: ToolSourceSkill, ProviderID: "p"})
	assert.Equal(t, PermissionAllow, d)
}
