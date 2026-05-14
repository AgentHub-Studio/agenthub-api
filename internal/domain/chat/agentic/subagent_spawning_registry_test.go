package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for SubagentSpawningRegistry — §8 subagent lifecycle states
// from arXiv:2604.14228v1.

func TestSubagentSpawningRegistry_NewReturnsNonNil(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	assert.NotNil(t, r, "NewSubagentSpawningRegistry must return a non-nil registry")
}

func TestSubagentSpawningRegistry_Count(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	assert.Equal(t, SeedSubagentSpawnStateCount, r.Count(),
		"registry must contain exactly SeedSubagentSpawnStateCount lifecycle states")
}

func TestSubagentSpawningRegistry_SeedConstantIs4(t *testing.T) {
	assert.Equal(t, 4, SeedSubagentSpawnStateCount,
		"§8 defines exactly four lifecycle states: spawned/running/summary_returned/context_discarded")
}

func TestSubagentSpawningRegistry_FindStateByID_KnownStates(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	known := []SubagentLifecycleState{
		SubagentStateSpawned,
		SubagentStateRunning,
		SubagentStateSummaryReturned,
		SubagentStateContextDiscarded,
	}
	for _, id := range known {
		p, ok := r.FindStateByID(id)
		assert.True(t, ok, "FindStateByID must find registered state %q", id)
		require.NotNil(t, p, "profile must be non-nil for %q", id)
		assert.Equal(t, id, p.StateID, "StateID must match query key for %q", id)
	}
}

func TestSubagentSpawningRegistry_FindStateByID_UnknownReturnsNil(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	p, ok := r.FindStateByID("nonexistent_state")
	assert.False(t, ok, "FindStateByID must return false for unknown states")
	assert.Nil(t, p, "profile must be nil for unknown states")
}

func TestSubagentSpawningRegistry_AllStates_Length(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	all := r.AllStates()
	assert.Len(t, all, SeedSubagentSpawnStateCount,
		"AllStates must return all four states")
}

func TestSubagentSpawningRegistry_AllStates_IsDefensiveCopy(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	a := r.AllStates()
	b := r.AllStates()
	assert.NotSame(t, &a, &b, "AllStates must return a new slice each call")
	a[0] = nil
	c := r.AllStates()
	assert.NotNil(t, c[0], "mutation of returned slice must not affect registry internals")
}

func TestSubagentSpawningRegistry_AllStates_OrderedByTransitionRank(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	all := r.AllStates()
	for i := 1; i < len(all); i++ {
		assert.Less(t, all[i-1].TransitionRank, all[i].TransitionRank,
			"AllStates must be ordered by ascending TransitionRank; index %d > %d violated", i-1, i)
	}
}

func TestSubagentSpawningRegistry_IsValidStateID_TrueForKnown(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	assert.True(t, r.IsValidStateID(SubagentStateSpawned))
	assert.True(t, r.IsValidStateID(SubagentStateRunning))
	assert.True(t, r.IsValidStateID(SubagentStateSummaryReturned))
	assert.True(t, r.IsValidStateID(SubagentStateContextDiscarded))
}

func TestSubagentSpawningRegistry_IsValidStateID_FalseForUnknown(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	assert.False(t, r.IsValidStateID("idle"), "unregistered state must be invalid")
	assert.False(t, r.IsValidStateID(""), "empty string must be invalid")
}

func TestSubagentSpawningRegistry_TerminalStates_ExactlyOne(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	ts := r.TerminalStates()
	assert.Len(t, ts, 1, "§8 has exactly one terminal state")
	assert.Equal(t, SubagentStateContextDiscarded, ts[0].StateID,
		"the sole terminal state must be context_discarded")
}

func TestSubagentSpawningRegistry_ActiveStates_Count(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	active := r.ActiveStates()
	assert.Len(t, active, 3, "§8 has three active (non-terminal) states")
}

func TestSubagentSpawningRegistry_ActiveStates_ContainsExpected(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	active := r.ActiveStates()
	ids := make([]SubagentLifecycleState, len(active))
	for i, p := range active {
		ids[i] = p.StateID
	}
	assert.Contains(t, ids, SubagentStateSpawned)
	assert.Contains(t, ids, SubagentStateRunning)
	assert.Contains(t, ids, SubagentStateSummaryReturned)
}

func TestSubagentSpawningRegistry_StatesWithSummaryAvailable_TwoStates(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	states := r.StatesWithSummaryAvailable()
	assert.Len(t, states, 2,
		"summary is available in summary_returned and context_discarded (2 states)")
}

func TestSubagentSpawningRegistry_StatesWithSummaryAvailable_IDs(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	states := r.StatesWithSummaryAvailable()
	ids := make([]SubagentLifecycleState, len(states))
	for i, p := range states {
		ids[i] = p.StateID
	}
	assert.Contains(t, ids, SubagentStateSummaryReturned)
	assert.Contains(t, ids, SubagentStateContextDiscarded)
}

func TestSubagentSpawningRegistry_StatesWhereContextRetained_ThreeStates(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	states := r.StatesWhereContextRetained()
	assert.Len(t, states, 3,
		"context is retained in spawned, running, summary_returned (3 states)")
}

func TestSubagentSpawningRegistry_StatesWhereContextRetained_DoesNotIncludeTerminal(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	states := r.StatesWhereContextRetained()
	for _, p := range states {
		assert.NotEqual(t, SubagentStateContextDiscarded, p.StateID,
			"context_discarded must not appear in context-retained states")
	}
}

func TestSubagentSpawningRegistry_StatesWhereParentNotified_TwoStates(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	states := r.StatesWhereParentNotified()
	assert.Len(t, states, 2,
		"parent is notified in summary_returned and context_discarded (2 states)")
}

func TestSubagentSpawningRegistry_StatesWhereParentNotified_NotInEarlyStates(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	states := r.StatesWhereParentNotified()
	ids := make([]SubagentLifecycleState, len(states))
	for i, p := range states {
		ids[i] = p.StateID
	}
	assert.NotContains(t, ids, SubagentStateSpawned,
		"spawned state must not notify parent")
	assert.NotContains(t, ids, SubagentStateRunning,
		"running state must not notify parent")
}

func TestSubagentSpawningRegistry_StateByTransitionRank_AllRanks(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	expected := map[int]SubagentLifecycleState{
		1: SubagentStateSpawned,
		2: SubagentStateRunning,
		3: SubagentStateSummaryReturned,
		4: SubagentStateContextDiscarded,
	}
	for rank, wantID := range expected {
		p, ok := r.StateByTransitionRank(rank)
		assert.True(t, ok, "rank %d must be found", rank)
		require.NotNil(t, p)
		assert.Equal(t, wantID, p.StateID, "rank %d must map to %q", rank, wantID)
	}
}

func TestSubagentSpawningRegistry_StateByTransitionRank_OutOfRange(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	_, ok0 := r.StateByTransitionRank(0)
	assert.False(t, ok0, "rank 0 is out of range")
	_, ok5 := r.StateByTransitionRank(5)
	assert.False(t, ok5, "rank 5 is out of range")
}

func TestSubagentSpawningRegistry_InitialState(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	initial := r.InitialState()
	require.NotNil(t, initial, "InitialState must not return nil")
	assert.Equal(t, SubagentStateSpawned, initial.StateID,
		"initial state must be spawned (TransitionRank=1)")
	assert.Equal(t, 1, initial.TransitionRank)
}

func TestSubagentSpawningRegistry_TerminalStateAfterSummary(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	terminal := r.TerminalStateAfterSummary()
	require.NotNil(t, terminal, "TerminalStateAfterSummary must not return nil")
	assert.Equal(t, SubagentStateContextDiscarded, terminal.StateID,
		"terminal state after summary must be context_discarded")
	assert.True(t, terminal.IsTerminal)
	assert.False(t, terminal.IsContextRetained,
		"context_discarded must not retain context")
}

func TestSubagentSpawnProfile_FieldsSpawned(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	p, _ := r.FindStateByID(SubagentStateSpawned)
	require.NotNil(t, p)
	assert.Equal(t, 1, p.TransitionRank)
	assert.NotEmpty(t, p.Label)
	assert.NotEmpty(t, p.Description)
	assert.NotEmpty(t, p.PDFSection)
	assert.False(t, p.IsTerminal)
	assert.True(t, p.IsContextRetained)
	assert.False(t, p.SummaryAvailable)
	assert.False(t, p.ParentNotified)
}

func TestSubagentSpawnProfile_FieldsRunning(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	p, _ := r.FindStateByID(SubagentStateRunning)
	require.NotNil(t, p)
	assert.Equal(t, 2, p.TransitionRank)
	assert.False(t, p.IsTerminal)
	assert.True(t, p.IsContextRetained)
	assert.False(t, p.SummaryAvailable)
	assert.False(t, p.ParentNotified)
}

func TestSubagentSpawnProfile_FieldsSummaryReturned(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	p, _ := r.FindStateByID(SubagentStateSummaryReturned)
	require.NotNil(t, p)
	assert.Equal(t, 3, p.TransitionRank)
	assert.False(t, p.IsTerminal)
	assert.True(t, p.IsContextRetained,
		"context is still live when summary is first returned")
	assert.True(t, p.SummaryAvailable)
	assert.True(t, p.ParentNotified)
}

func TestSubagentSpawnProfile_FieldsContextDiscarded(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	p, _ := r.FindStateByID(SubagentStateContextDiscarded)
	require.NotNil(t, p)
	assert.Equal(t, 4, p.TransitionRank)
	assert.True(t, p.IsTerminal)
	assert.False(t, p.IsContextRetained,
		"context_discarded must have released context")
	assert.True(t, p.SummaryAvailable)
	assert.True(t, p.ParentNotified)
}

// --- Structural invariant tests ---

func TestSubagentSpawningInvariant_ExactlyOneInitialState(t *testing.T) {
	assert.True(t, ExactlyOneInitialState(),
		"ExactlyOneInitialState invariant must hold — exactly one state with TransitionRank==1")
}

func TestSubagentSpawningInvariant_TerminalStatesHaveNoContext(t *testing.T) {
	assert.True(t, TerminalStatesHaveNoContext(),
		"TerminalStatesHaveNoContext invariant must hold — terminal states must not retain context")
}

func TestSubagentSpawningInvariant_SummaryReturnedPrecedesContextDiscarded(t *testing.T) {
	assert.True(t, SummaryReturnedPrecedesContextDiscarded(),
		"SummaryReturnedPrecedesContextDiscarded invariant must hold — parent receives summary before context is freed")
}

func TestSubagentSpawningInvariant_AllProfilesHaveNonEmptyPDFSection(t *testing.T) {
	for _, p := range subagentSpawnProfiles {
		assert.NotEmpty(t, p.PDFSection,
			"all profiles must cite a PDF section (state %q has empty PDFSection)", p.StateID)
	}
}

func TestSubagentSpawningInvariant_TransitionRanksContiguous(t *testing.T) {
	seen := make(map[int]bool, SeedSubagentSpawnStateCount)
	for _, p := range subagentSpawnProfiles {
		assert.False(t, seen[p.TransitionRank], "TransitionRank %d is duplicated", p.TransitionRank)
		seen[p.TransitionRank] = true
	}
	for rank := 1; rank <= SeedSubagentSpawnStateCount; rank++ {
		assert.True(t, seen[rank], "TransitionRank %d is missing — ranks must be contiguous [1,%d]", rank, SeedSubagentSpawnStateCount)
	}
}

func TestSubagentLifecycleState_ConstantValues(t *testing.T) {
	assert.Equal(t, SubagentLifecycleState("spawned"), SubagentStateSpawned)
	assert.Equal(t, SubagentLifecycleState("running"), SubagentStateRunning)
	assert.Equal(t, SubagentLifecycleState("summary_returned"), SubagentStateSummaryReturned)
	assert.Equal(t, SubagentLifecycleState("context_discarded"), SubagentStateContextDiscarded)
}

func TestSubagentSpawningRegistry_SummaryAndParentNotificationAligned(t *testing.T) {
	// §8 invariant: whenever the parent is notified (ParentNotified=true),
	// the summary must also be available (SummaryAvailable=true). They are
	// generated atomically — the parent receives the summary as the notification.
	r := NewSubagentSpawningRegistry()
	for _, p := range r.AllStates() {
		if p.ParentNotified {
			assert.True(t, p.SummaryAvailable,
				"state %q has ParentNotified=true but SummaryAvailable=false — inconsistent", p.StateID)
		}
	}
}

func TestSubagentSpawningRegistry_InitialStateHasContextRetained(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	initial := r.InitialState()
	require.NotNil(t, initial)
	assert.True(t, initial.IsContextRetained,
		"spawned state must have IsContextRetained=true — context is allocated upon spawn")
}

func TestSubagentSpawningRegistry_SummaryUnavailableInEarlyStates(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	earlyIDs := []SubagentLifecycleState{SubagentStateSpawned, SubagentStateRunning}
	for _, id := range earlyIDs {
		p, ok := r.FindStateByID(id)
		require.True(t, ok)
		assert.False(t, p.SummaryAvailable,
			"state %q must not have SummaryAvailable=true before the loop completes", id)
	}
}

func TestSubagentSpawningRegistry_AllStates_EachHasNonEmptyLabel(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	for _, p := range r.AllStates() {
		assert.NotEmpty(t, p.Label, "state %q must have a non-empty Label", p.StateID)
	}
}

func TestSubagentSpawningRegistry_AllStates_EachHasNonEmptyDescription(t *testing.T) {
	r := NewSubagentSpawningRegistry()
	for _, p := range r.AllStates() {
		assert.NotEmpty(t, p.Description, "state %q must have a non-empty Description", p.StateID)
	}
}
