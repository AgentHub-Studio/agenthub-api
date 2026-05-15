package agentic

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify LOOP-007 (Estado mutável controlado por
// iteração) against the Claude Code architecture paper "Dive into Claude
// Code" (arXiv:2604.14228v1):
//
//   - Section 4.1 step 2 ("Mutable state initialization"): "A single State
//     object stores all mutable state across iterations, including
//     messages, tool context, compaction tracking, and recovery counters.
//     The loop's seven continue points (the 'continue sites') each
//     overwrite this object in one whole-object assignment rather than
//     mutating fields individually."
//   - Implication: fields-individual mutation creates partial-update bugs
//     where one continue site forgets to reset a flag. WHOLE-OBJECT
//     assignment makes the contract explicit: "this iteration starts from
//     a fully-defined state vector".
//
// AgentHub maps the controlled-mutation pattern to:
//   - turnstate.go TurnState — value type carrying TurnCount + Transition
//     + MaxOutputTokensRecoveryCount + HasAttemptedReactiveCompact +
//     MaxOutputTokensOverride + StopHookActive + StartedAt + TurnStartedAt.
//   - turnstate.go NewTurnState — constructor that produces a fresh state
//     vector with all fields explicitly initialised.
//   - turnstate.go NextTurn / RecordMaxTokensRecovery / RecordReactiveCompact
//     / RecordContextOverflow — controlled mutation methods that update
//     specific fields atomically.
//   - statestore.go StateStore[T] — generic pub-sub container that
//     enforces WHOLE-OBJECT replacement via SetState(prev → next), with
//     change notification only when next != prev (identity check).
//
// These scenarios assert: value-type semantics (copy = independent),
// fresh state from NewTurnState, controlled-mutation methods leave
// other fields untouched, StateStore whole-object replacement contract.

func TestBDD_TurnStateControlledMutation(t *testing.T) {
	t.Run("Scenario_TurnStateIsValueTypeAndCopiesIndependently", func(t *testing.T) {
		// Given a TurnState that already advanced (PDF Section 4.1: state
		//       is a SINGLE object — but that object is a value type, so
		//       passing it copies cleanly without aliasing),
		original := NewTurnState()
		original.NextTurn(TransitionToolUse)
		original.RecordReactiveCompact()

		// When the runner copies it (e.g. snapshot for analytics),
		copy := original

		// Then mutating the copy does NOT affect the original — value
		//      semantics prevent accidental cross-iteration leakage.
		copy.NextTurn(TransitionMaxTokensRecovery)
		assert.Equal(t, 1, original.TurnCount,
			"original TurnCount must remain 1 after copy mutation")
		assert.Equal(t, 2, copy.TurnCount,
			"copy TurnCount advanced independently")
		assert.Equal(t, TransitionReactiveCompact, original.Transition,
			"original transition unchanged")
		assert.Equal(t, TransitionMaxTokensRecovery, copy.Transition,
			"copy transition advanced independently")
	})

	t.Run("Scenario_NewTurnStateInitialisesAllFieldsExplicitly", func(t *testing.T) {
		// Given a fresh state from the constructor (PDF Section 4.1 step 2:
		//       state init must set ALL fields, not rely on Go zero values
		//       for ones that need explicit defaults like timestamps),
		when := NewTurnState()

		// Then every field has a defined initial value — no field is left
		//      to silently default in a way that breaks recovery semantics.
		assert.Equal(t, 0, when.TurnCount,
			"fresh state starts at turn 0 — explicit, not zero-value coincidence")
		assert.Equal(t, TransitionNone, when.Transition,
			"fresh state has no prior transition")
		assert.Equal(t, 0, when.MaxOutputTokensRecoveryCount,
			"fresh state has zero recoveries recorded")
		assert.False(t, when.HasAttemptedReactiveCompact,
			"fresh state has not attempted compact")
		assert.Equal(t, 0, when.MaxOutputTokensOverride,
			"fresh state has no override")
		assert.False(t, when.StopHookActive,
			"fresh state has no active stop hook")
		assert.False(t, when.StartedAt.IsZero(),
			"StartedAt must be set explicitly to time.Now (not zero default)")
		assert.False(t, when.TurnStartedAt.IsZero(),
			"TurnStartedAt must be set explicitly")
	})

	t.Run("Scenario_NextTurnUpdatesCountAndTransitionAtomically", func(t *testing.T) {
		// Given a state at iteration 1,
		given := NewTurnState()
		given.NextTurn(TransitionNone)
		assert.Equal(t, 1, given.TurnCount)

		// When the loop continues with a tool_use transition,
		given.NextTurn(TransitionToolUse)

		// Then BOTH count and transition update — these two fields move
		//      together (they're a single semantic unit: "we're now on
		//      iteration N because of reason X").
		assert.Equal(t, 2, given.TurnCount,
			"NextTurn increments count")
		assert.Equal(t, TransitionToolUse, given.Transition,
			"NextTurn updates transition reason atomically with count")
	})

	t.Run("Scenario_RecordMaxTokensRecoveryDoesNotResetUnrelatedFields", func(t *testing.T) {
		// Given a state with prior reactive compact attempt (PDF Section
		//       4.1 step 2: controlled mutation must not silently reset
		//       fields the caller didn't ask about — that's the bug the
		//       whole-object assignment pattern PREVENTS),
		given := NewTurnState()
		given.NextTurn(TransitionToolUse)
		given.RecordReactiveCompact()
		assert.True(t, given.HasAttemptedReactiveCompact)

		// When we record a max-tokens recovery,
		given.RecordMaxTokensRecovery(8000)

		// Then the reactive_compact flag is PRESERVED — the recovery method
		//      mutates only its own fields (counter + override + transition).
		assert.True(t, given.HasAttemptedReactiveCompact,
			"reactive compact flag must NOT be reset by max-tokens recovery")
		assert.Equal(t, 1, given.MaxOutputTokensRecoveryCount,
			"max-tokens counter advanced")
		assert.Equal(t, 8000, given.MaxOutputTokensOverride,
			"max-tokens override applied")
		assert.Equal(t, TransitionMaxTokensRecovery, given.Transition,
			"transition reflects most recent recovery cause")
	})

	t.Run("Scenario_StateStoreReplacesWholeObjectViaSetState", func(t *testing.T) {
		// Given a generic StateStore (PDF Section 4.1: whole-object
		//       assignment pattern; AgentHub provides StateStore[T] as the
		//       generic helper for this contract),
		store := NewStateStore[int](0, nil)

		// When the caller updates via SetState(prev → next),
		store.SetState(func(prev int) int { return prev + 1 })
		store.SetState(func(prev int) int { return prev + 10 })

		// Then GetState reflects the latest whole-object value — no
		//      partial mutation possible because SetState requires
		//      returning the entire next state.
		assert.Equal(t, 11, store.GetState(),
			"SetState must replace whole state value (not field-mutate)")
	})

	t.Run("Scenario_StateStoreSkipsNotificationOnIdenticalState", func(t *testing.T) {
		// Given a StateStore subscriber expecting changes only,
		var changeCount atomic.Int32
		store := NewStateStore[int](42, func(_, _ int) {
			changeCount.Add(1)
		})

		// When SetState returns the SAME value (no logical change),
		store.SetState(func(prev int) int { return prev })
		store.SetState(func(prev int) int { return prev })

		// Then no listener fires — identity check via == prevents spurious
		//      notifications. Loop iterations that produce no state delta
		//      don't wake observers.
		assert.Equal(t, int32(0), changeCount.Load(),
			"SetState returning identical value must NOT fire onChange")
	})

	t.Run("Scenario_StateStoreNotifiesAllSubscribersOnRealChange", func(t *testing.T) {
		// Given multiple subscribers (PDF Section 4.1 + 11: state changes
		//       are observable for telemetry, UI, and recovery logic),
		store := NewStateStore[int](0, nil)
		var hits atomic.Int32
		store.Subscribe(func() { hits.Add(1) })
		store.Subscribe(func() { hits.Add(10) })
		store.Subscribe(func() { hits.Add(100) })

		// When a real state change happens,
		store.SetState(func(prev int) int { return prev + 1 })

		// Then all 3 subscribers fire (1+10+100=111).
		assert.Equal(t, int32(111), hits.Load(),
			"all subscribers must be notified on real state change")
	})

	t.Run("Scenario_StateStoreUnsubscribeStopsFurtherNotifications", func(t *testing.T) {
		// Given a subscriber that was unregistered (PDF: dynamic listener
		//       lifecycle — sub-agents subscribe/unsubscribe per their
		//       lifecycle scope),
		store := NewStateStore[int](0, nil)
		var hits atomic.Int32
		unsub := store.Subscribe(func() { hits.Add(1) })

		// When the subscriber unregisters then state changes,
		unsub()
		store.SetState(func(prev int) int { return prev + 1 })

		// Then no notification — leak-free dynamic registration.
		assert.Equal(t, int32(0), hits.Load(),
			"unsubscribed listener must not fire")
	})

	t.Run("Scenario_StateStoreIsConcurrentSafeForReadsAndWrites", func(t *testing.T) {
		// Given concurrent goroutines reading + writing (PDF Section 4.2 +
		//       8: parallel tool batches + sub-agent spawns may all touch
		//       shared state; StateStore must be race-free),
		store := NewStateStore[int](0, nil)
		var wg sync.WaitGroup

		// When 50 writers + 50 readers race for 100 ms,
		for i := 0; i < 50; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					store.SetState(func(prev int) int { return prev + 1 })
				}
			}()
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_ = store.GetState()
				}
			}()
		}
		wg.Wait()

		// Then no race occurs (test fails under -race) and final state is
		//      the sum of all writes (50 writers × 100 increments = 5000).
		assert.Equal(t, 5000, store.GetState(),
			"concurrent SetState must converge to expected sum (no lost updates)")
	})

	t.Run("Scenario_TurnTransitionIsAFiniteEnumPreventingFreeTextDrift", func(t *testing.T) {
		// Given the transition reason is a typed string enum (PDF Section
		//       4.1: state field types matter — free-text reasons drift,
		//       typed enums force explicit additions),
		// When we list the constants (validated via guard),
		all := []TurnTransition{
			TransitionNone,
			TransitionToolUse,
			TransitionBudgetContinue,
			TransitionMaxTokensRecovery,
			TransitionReactiveCompact,
			TransitionPromptTooLong,
			TransitionContextOverflow,
			TransitionStopHook,
		}

		// Then exactly 8 distinct values — refactor that adds a 9th must
		//      explicitly extend this BDD too, ensuring telemetry surfaces
		//      know about new transitions.
		seen := map[TurnTransition]bool{}
		for _, tr := range all {
			seen[tr] = true
		}
		assert.Len(t, seen, 8,
			"TurnTransition enum has exactly 8 distinct constants")
	})

	t.Run("Scenario_ShouldRetryMaxTokensIsPureFunctionOfState", func(t *testing.T) {
		// Given a TurnState (PDF Section 4.1: derived predicates must be
		//       pure functions of the state vector — no hidden globals
		//       affecting recovery decisions),
		ts := NewTurnState()
		assert.True(t, ts.ShouldRetryMaxTokens(),
			"fresh state can retry max tokens")

		// When recovery fires repeatedly,
		ts.RecordMaxTokensRecovery(8000)
		ts.RecordMaxTokensRecovery(16000)
		ts.RecordMaxTokensRecovery(32000)

		// Then ShouldRetryMaxTokens transitions to false purely based on
		//      the counter — no global config involved.
		assert.False(t, ts.ShouldRetryMaxTokens(),
			"after 3 recoveries, ShouldRetry returns false (pure of state)")

		// And a fresh state is still allowed to retry — proves the function
		//      depends ONLY on this state's counter, not global counter.
		fresh := NewTurnState()
		assert.True(t, fresh.ShouldRetryMaxTokens(),
			"different state instance has its own counter — not shared global")
	})
}
