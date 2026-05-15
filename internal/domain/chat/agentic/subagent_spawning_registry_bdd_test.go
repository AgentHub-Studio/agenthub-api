package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD-style scenarios for SubagentSpawningRegistry — §8 subagent lifecycle
// (arXiv:2604.14228v1).
//
// These scenarios validate the behavioral contracts of the four-phase
// spawning lifecycle: spawned → running → summary_returned → context_discarded.

func TestBDD_SubagentSpawningRegistry_LinearLifecycle(t *testing.T) {
	t.Run("Scenario_SpawnedIsFirstAndRunningIsSecond", func(t *testing.T) {
		// Given the §8 spawning lifecycle registry,
		reg := NewSubagentSpawningRegistry()

		// When inspecting TransitionRank ordering,
		spawned, okS := reg.FindStateByID(SubagentStateSpawned)
		running, okR := reg.FindStateByID(SubagentStateRunning)

		// Then spawned (rank 1) precedes running (rank 2) —
		// a subagent must be spawned before it can execute.
		require.True(t, okS, "spawned state must exist")
		require.True(t, okR, "running state must exist")
		assert.Less(t, spawned.TransitionRank, running.TransitionRank,
			"spawned must have a lower TransitionRank than running")
		assert.Equal(t, 1, spawned.TransitionRank, "spawned must be rank 1")
		assert.Equal(t, 2, running.TransitionRank, "running must be rank 2")
	})

	t.Run("Scenario_SummaryReturnedPrecedesContextDiscard", func(t *testing.T) {
		// Given the §8 spawning lifecycle registry,
		reg := NewSubagentSpawningRegistry()

		// When comparing the pivot state and the terminal state,
		sr, okSR := reg.FindStateByID(SubagentStateSummaryReturned)
		cd, okCD := reg.FindStateByID(SubagentStateContextDiscarded)

		// Then summary_returned (rank 3) precedes context_discarded (rank 4) —
		// §8 guarantees the parent has the summary BEFORE memory is freed.
		require.True(t, okSR, "summary_returned state must exist")
		require.True(t, okCD, "context_discarded state must exist")
		assert.Less(t, sr.TransitionRank, cd.TransitionRank,
			"summary must be returned before context is discarded")
		assert.True(t, SummaryReturnedPrecedesContextDiscarded(),
			"structural invariant SummaryReturnedPrecedesContextDiscarded must hold")
	})
}

func TestBDD_SubagentSpawningRegistry_ContextWindowBudget(t *testing.T) {
	t.Run("Scenario_ContextRetainedUntilDiscardState", func(t *testing.T) {
		// Given the §8 lifecycle — a core design goal is keeping the parent's
		// context budget bounded by discarding the child's context promptly,
		reg := NewSubagentSpawningRegistry()

		// When enumerating states where the child's context is retained,
		retained := reg.StatesWhereContextRetained()

		// Then the context is retained in spawned, running, and summary_returned —
		// three states — but NOT in context_discarded. This means the child's
		// context lives exactly as long as needed and no longer.
		assert.Len(t, retained, 3,
			"context must be retained in exactly 3 states: spawned, running, summary_returned")
		for _, p := range retained {
			assert.NotEqual(t, SubagentStateContextDiscarded, p.StateID,
				"context_discarded must never appear in retained-context states")
		}
	})

	t.Run("Scenario_TerminalStateReleasesContext", func(t *testing.T) {
		// Given the §8 lifecycle terminal state,
		reg := NewSubagentSpawningRegistry()
		terminal := reg.TerminalStateAfterSummary()

		// When the child reaches context_discarded,
		require.NotNil(t, terminal)

		// Then its IsContextRetained must be false and IsTerminal must be true —
		// releasing context is what makes this state terminal from the parent's
		// perspective.
		assert.False(t, terminal.IsContextRetained,
			"terminal state context_discarded must have released its context window")
		assert.True(t, terminal.IsTerminal,
			"context_discarded must be marked terminal — no further transitions")
		assert.Equal(t, SubagentStateContextDiscarded, terminal.StateID)
	})
}

func TestBDD_SubagentSpawningRegistry_SummaryHandoff(t *testing.T) {
	t.Run("Scenario_SummaryBecomesAvailableAtPivotState", func(t *testing.T) {
		// Given the §8 summary-only return contract (PDF §8 + §8.3):
		// the parent receives a bounded SubagentReturnSummary, never the raw
		// transcript — protecting the parent's context budget,
		reg := NewSubagentSpawningRegistry()

		// When querying states where the summary is available,
		withSummary := reg.StatesWithSummaryAvailable()

		// Then only summary_returned and context_discarded expose a summary —
		// spawned and running have not yet completed the child's loop.
		assert.Len(t, withSummary, 2,
			"summary must be available in exactly 2 states: summary_returned and context_discarded")
		summaryIDs := make([]SubagentLifecycleState, len(withSummary))
		for i, p := range withSummary {
			summaryIDs[i] = p.StateID
		}
		assert.Contains(t, summaryIDs, SubagentStateSummaryReturned)
		assert.Contains(t, summaryIDs, SubagentStateContextDiscarded)
		assert.NotContains(t, summaryIDs, SubagentStateSpawned)
		assert.NotContains(t, summaryIDs, SubagentStateRunning)
	})

	t.Run("Scenario_ParentNotifiedSimultaneouslyWithSummary", func(t *testing.T) {
		// Given the §8 hand-off moment — the parent queryLoop() is unblocked
		// exactly when the summary is produced (atomically),
		reg := NewSubagentSpawningRegistry()

		// When comparing which states have ParentNotified vs SummaryAvailable,
		parentNotified := reg.StatesWhereParentNotified()
		summaryAvailable := reg.StatesWithSummaryAvailable()

		// Then both sets have the same size and the same members — the parent
		// notification and summary availability are inseparable events.
		require.Equal(t, len(parentNotified), len(summaryAvailable),
			"ParentNotified and SummaryAvailable must cover the same number of states")
		notifiedIDs := make(map[SubagentLifecycleState]bool, len(parentNotified))
		for _, p := range parentNotified {
			notifiedIDs[p.StateID] = true
		}
		for _, p := range summaryAvailable {
			assert.True(t, notifiedIDs[p.StateID],
				"state %q has SummaryAvailable but is not in ParentNotified — must be aligned", p.StateID)
		}
	})
}

func TestBDD_SubagentSpawningRegistry_RegistryInvariants(t *testing.T) {
	t.Run("Scenario_StructuralInvariantsAllPass", func(t *testing.T) {
		// Given the §8 spawning lifecycle registry,
		reg := NewSubagentSpawningRegistry()

		// When verifying the three structural invariants defined in the paper,
		// Then all three must hold simultaneously — any violation would indicate
		// a data corruption or misconfiguration of the lifecycle definition.
		assert.True(t, ExactlyOneInitialState(),
			"ExactlyOneInitialState: exactly one state must have TransitionRank==1")
		assert.True(t, TerminalStatesHaveNoContext(),
			"TerminalStatesHaveNoContext: no terminal state may retain context")
		assert.True(t, SummaryReturnedPrecedesContextDiscarded(),
			"SummaryReturnedPrecedesContextDiscarded: parent gets summary before context is freed")

		// Additionally the registry must be self-consistent in count.
		assert.Equal(t, SeedSubagentSpawnStateCount, reg.Count(),
			"registry count must equal seed constant")
	})

	t.Run("Scenario_AllStatesHaveMetadata", func(t *testing.T) {
		// Given the §8 spawning lifecycle registry,
		reg := NewSubagentSpawningRegistry()

		// When iterating all states,
		all := reg.AllStates()

		// Then each state must have a non-empty Label, Description, and PDFSection
		// — the registry is a documentation artefact as well as runtime logic.
		for _, p := range all {
			assert.NotEmpty(t, p.Label,
				"state %q must have a human-readable Label", p.StateID)
			assert.NotEmpty(t, p.Description,
				"state %q must have a Description explaining its semantics", p.StateID)
			assert.NotEmpty(t, p.PDFSection,
				"state %q must cite the paper section via PDFSection", p.StateID)
		}
	})
}

func TestBDD_SubagentSpawningRegistry_InitialAndTerminalEndpoints(t *testing.T) {
	t.Run("Scenario_InitialStateIsSpawned", func(t *testing.T) {
		// Given the §8 lifecycle — the parent creates the child agent via
		// AgentTool (the "agent" builtin) before any execution occurs,
		reg := NewSubagentSpawningRegistry()

		// When asking for the initial state,
		initial := reg.InitialState()

		// Then it must be spawned — rank 1, with context retained, no summary,
		// no parent notification. This matches the paper's description of the
		// child being created but not yet running.
		require.NotNil(t, initial, "InitialState must not return nil")
		assert.Equal(t, SubagentStateSpawned, initial.StateID)
		assert.Equal(t, 1, initial.TransitionRank)
		assert.True(t, initial.IsContextRetained,
			"context is allocated at spawn time — must be retained in initial state")
		assert.False(t, initial.SummaryAvailable,
			"no summary yet — child has not run")
		assert.False(t, initial.ParentNotified,
			"parent is blocked — not notified until summary_returned")
		assert.False(t, initial.IsTerminal,
			"spawned is not terminal — transitions remain")
	})

	t.Run("Scenario_TerminalEndpointIsContextDiscarded", func(t *testing.T) {
		// Given the §8 lifecycle — the lifecycle ends when the child's context
		// is freed, leaving only the durable sidechain transcript,
		reg := NewSubagentSpawningRegistry()

		// When asking for the terminal state after summary,
		terminal := reg.TerminalStateAfterSummary()

		// Then it must be context_discarded — rank 4, with no context retained,
		// summary available, parent already notified. This is the state where
		// the parent's context budget is restored.
		require.NotNil(t, terminal, "TerminalStateAfterSummary must not return nil")
		assert.Equal(t, SubagentStateContextDiscarded, terminal.StateID)
		assert.Equal(t, SeedSubagentSpawnStateCount, terminal.TransitionRank,
			"terminal state must have the highest TransitionRank")
		assert.True(t, terminal.IsTerminal)
		assert.False(t, terminal.IsContextRetained)
		assert.True(t, terminal.SummaryAvailable)
		assert.True(t, terminal.ParentNotified)
	})
}
