package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolPool_IsValidToolSource(t *testing.T) {
	for _, s := range allToolSources {
		assert.True(t, IsValidToolSource(s))
	}
	assert.False(t, IsValidToolSource(ToolSource("nope")))
	assert.False(t, IsValidToolSource(ToolSource("")))
}

func TestToolPool_EntryValidate(t *testing.T) {
	good := ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"}
	assert.NoError(t, good.Validate())

	missingName := ToolPoolEntry{Source: ToolSourceBuiltin, ProviderID: "harness"}
	assert.ErrorIs(t, missingName.Validate(), ErrToolPoolEntryNameRequired)

	missingProvider := ToolPoolEntry{Name: "Read", Source: ToolSourceBuiltin}
	assert.ErrorIs(t, missingProvider.Validate(), ErrToolPoolEntryProviderRequired)

	badSource := ToolPoolEntry{Name: "Read", Source: ToolSource("nope"), ProviderID: "x"}
	assert.ErrorIs(t, badSource.Validate(), ErrToolPoolInvalidSource)
}

func TestToolPool_AssembleNoProvidersFails(t *testing.T) {
	a := NewToolPoolAssembler()
	_, err := a.Assemble(context.Background(), "iter-1")
	assert.ErrorIs(t, err, ErrToolPoolNoProviders)
}

func TestToolPool_AssembleEmptyIterationFails(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	_, err := a.Assemble(context.Background(), "  ")
	assert.ErrorIs(t, err, ErrToolPoolIterationIDRequired)
}

func TestToolPool_AssembleHappyPath(t *testing.T) {
	a := NewToolPoolAssembler()
	a.SetClock(func() time.Time { return time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC) })
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
		{Name: "summarize", Source: ToolSourceSkill, ProviderID: "skill-summarizer"},
	}))
	snap, err := a.Assemble(context.Background(), "iter-7")
	require.NoError(t, err)
	assert.Equal(t, "iter-7", snap.IterationID)
	assert.Equal(t, 3, len(snap.Entries))
	assert.Equal(t, []string{"Bash", "Read", "summarize"}, snap.Names())
	assert.Empty(t, snap.DroppedNames)
}

func TestToolPool_OrderingByRankThenName(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceMCP, []ToolPoolEntry{
		{Name: "mcp_tool", Source: ToolSourceMCP, ProviderID: "mcp-fs"},
	}))
	a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
		{Name: "skill_tool", Source: ToolSourceSkill, ProviderID: "skill-x"},
	}))
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Z_builtin", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "A_builtin", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	// Builtin (rank 0) first, alphabetical within rank, then skill, then MCP.
	assert.Equal(t,
		[]string{"A_builtin", "Z_builtin", "skill_tool", "mcp_tool"},
		snap.Names())
}

func TestToolPool_NameCollisionBuiltinWins(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness", Description: "builtin"},
	}))
	a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceSkill, ProviderID: "skill-shadow", Description: "shadow"},
	}))
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	require.Equal(t, 1, len(snap.Entries))
	assert.Equal(t, ToolSourceBuiltin, snap.Entries[0].Source)
	assert.Equal(t, "builtin", snap.Entries[0].Description)
	assert.Equal(t, []string{"Read"}, snap.DroppedNames)
}

func TestToolPool_FilterDropsEntry(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	a.AddFilter(func(e ToolPoolEntry) bool { return e.Name != "Bash" })
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Read"}, snap.Names())
	assert.Equal(t, []string{"Bash"}, snap.DroppedNames)
}

func TestToolPool_AllowlistRestrictsToNamed(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "Bash", Source: ToolSourceBuiltin, ProviderID: "harness"},
		{Name: "Edit", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	a.SetAllowlist([]string{"Read", "Edit"})
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Edit", "Read"}, snap.Names())
	assert.Equal(t, []string{"Bash"}, snap.DroppedNames)
}

func TestToolPool_AllowlistEmptyDisablesRestriction(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	a.SetAllowlist([]string{"Read"})
	a.SetAllowlist(nil) // reset
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	assert.Equal(t, 1, len(snap.Entries))
}

func TestToolPool_ProviderErrorPropagates(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(&errProvider{src: ToolSourceMCP, err: errors.New("connection refused")})
	_, err := a.Assemble(context.Background(), "iter-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestToolPool_InvalidEntryFromProviderFails(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	_, err := a.Assemble(context.Background(), "iter-1")
	assert.ErrorIs(t, err, ErrToolPoolEntryNameRequired)
}

func TestToolPool_SnapshotTimestampsFromInjectedClock(t *testing.T) {
	stamp := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	a := NewToolPoolAssembler()
	a.SetClock(func() time.Time { return stamp })
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	assert.Equal(t, stamp, snap.AssembledAt)
}

func TestToolPool_HasNameAndNames(t *testing.T) {
	snap := ToolPoolSnapshot{Entries: []ToolPoolEntry{
		{Name: "Read"}, {Name: "Bash"},
	}}
	assert.True(t, snap.HasName("Read"))
	assert.False(t, snap.HasName("Edit"))
	assert.Equal(t, []string{"Read", "Bash"}, snap.Names())
}

func TestToolPool_DroppedNamesAreUniqueAndSorted(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceSkill, []ToolPoolEntry{
		{Name: "tool_a", Source: ToolSourceSkill, ProviderID: "s1"},
		{Name: "tool_b", Source: ToolSourceSkill, ProviderID: "s2"},
	}))
	a.AddFilter(func(e ToolPoolEntry) bool { return false })
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"tool_a", "tool_b"}, snap.DroppedNames)
}

func TestToolPool_EntrySourceFilledFromProviderWhenEmpty(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceMCP, []ToolPoolEntry{
		{Name: "list_files", ProviderID: "mcp-fs"},
	}))
	snap, err := a.Assemble(context.Background(), "iter-1")
	require.NoError(t, err)
	require.Equal(t, 1, len(snap.Entries))
	assert.Equal(t, ToolSourceMCP, snap.Entries[0].Source)
}

func TestToolPool_ConcurrentAssembleSafe(t *testing.T) {
	a := NewToolPoolAssembler()
	a.Register(NewStaticToolPoolProvider(ToolSourceBuiltin, []ToolPoolEntry{
		{Name: "Read", Source: ToolSourceBuiltin, ProviderID: "harness"},
	}))
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			_, err := a.Assemble(context.Background(), "iter")
			assert.NoError(t, err)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// errProvider helps verify provider-side error propagation.
type errProvider struct {
	src ToolSource
	err error
}

func (e *errProvider) Source() ToolSource { return e.src }
func (e *errProvider) Provide(_ context.Context) ([]ToolPoolEntry, error) {
	return nil, e.err
}
