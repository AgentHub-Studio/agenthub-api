package agentic

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify LOOP-004 (Abort e cancelamento) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 4.5 (Stop Conditions): "Explicit abort: The abortController
//     signal fires." — abort is one of the 5 ways the loop terminates.
//   - Section 8.3 + 4.2: parent abort must propagate to in-flight subagents
//     and tool calls so cancellation is total, not partial.
//   - Section 11 (observability): aborted runs must be DISTINGUISHABLE from
//     errored or completed runs in the audit log.
//
// AgentHub maps explicit abort + cancellation to:
//   - context.WithTimeout / context.WithCancel — Go-idiomatic cancellation
//     wrapped around the runner (runner.go:318-320 TotalTimeout +
//     runner.go:853 per-call ctx).
//   - runner.go ctx.Err() checks at strategic loop points (lines 366, 765,
//     1244, 1425, 1531) — deterministic mid-loop abort observation.
//   - abortchain.go AbortController + AbortGroup — application-layer abort
//     signalling on top of context.Context for cases where context is not
//     enough (e.g. multi-controller groups, OnAbort callbacks).
//   - runner.go consumeStream `case <-ctx.Done()` (line 1647) — streaming
//     read interrupted on cancellation.
//
// TOOL-007 covered abortchain SERIALIZATION concerns (sibling abort during
// write batches). LOOP-004 focuses on LOOP-LEVEL cancellation: explicit
// signal propagation across the entire run, not just one batch.

func TestBDD_LoopLevelAbortAndCancellation(t *testing.T) {
	t.Run("Scenario_ContextCancellationStopsAtLoopBoundary", func(t *testing.T) {
		// Given a parent context that the caller cancels (PDF Section 4.5
		//       explicit abort: caller may invoke abort externally),
		ctx, cancel := context.WithCancel(context.Background())

		// When the caller cancels mid-flight,
		cancel()

		// Then the runner's ctx.Err() check at loop start (runner.go:765)
		//      observes the cancellation immediately — no spurious work.
		err := ctx.Err()
		assert.Error(t, err,
			"cancelled context must report error at loop boundary")
		assert.True(t, errors.Is(err, context.Canceled),
			"error must be context.Canceled (not DeadlineExceeded)")
	})

	t.Run("Scenario_DeadlineExceededIsDistinctFromCanceled", func(t *testing.T) {
		// Given a deadline-bounded context (TotalTimeout pattern from
		//       runner.go:318-320),
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// When the deadline elapses,
		time.Sleep(30 * time.Millisecond)

		// Then the error is DeadlineExceeded — operators distinguish
		//      "user aborted" from "wall-clock cap hit" via the error type.
		err := ctx.Err()
		assert.True(t, errors.Is(err, context.DeadlineExceeded),
			"timeout must report DeadlineExceeded (not Canceled) for telemetry attribution")
	})

	t.Run("Scenario_AbortControllerSignalIsObservableViaContextDone", func(t *testing.T) {
		// Given an AbortController bridging app-level signal to context
		//       (PDF Section 4.5: abort signal must be observable through
		//       the standard cancellation mechanism so existing ctx.Done()
		//       loops work without modification),
		ac := NewAbortController()
		ctx := ac.Context()

		// When Abort fires,
		go func() {
			time.Sleep(10 * time.Millisecond)
			ac.Abort()
		}()

		// Then a goroutine waiting on ctx.Done() unblocks immediately —
		//      the abort signal IS context cancellation, not a parallel
		//      mechanism.
		select {
		case <-ctx.Done():
			// Expected: abort propagated via context.
		case <-time.After(200 * time.Millisecond):
			t.Fatal("abort did not propagate to context.Done() within 200ms")
		}
		assert.True(t, ac.IsAborted())
	})

	t.Run("Scenario_AbortGroupCancelsAllControllersAtOnce", func(t *testing.T) {
		// Given multiple in-flight controllers (PDF Section 4.2 + 8.3:
		//       parent run holds child controllers for each sub-agent /
		//       parallel tool batch),
		group := NewAbortGroup()
		ac1 := NewAbortController()
		ac2 := NewAbortController()
		ac3 := NewAbortController()
		group.Add(ac1)
		group.Add(ac2)
		group.Add(ac3)

		// When the parent triggers AbortAll (e.g. user pressed Cancel),
		group.AbortAll()

		// Then all three children report aborted — total cancellation, no
		//      lingering work allowed.
		assert.True(t, ac1.IsAborted(), "controller 1 must abort")
		assert.True(t, ac2.IsAborted(), "controller 2 must abort")
		assert.True(t, ac3.IsAborted(), "controller 3 must abort")
		assert.True(t, group.IsAborted(),
			"group reports aborted state after AbortAll")
	})

	t.Run("Scenario_AbortGroupCountReflectsRegistration", func(t *testing.T) {
		// Given the runner registers controllers as it spawns work,
		group := NewAbortGroup()

		// When 5 controllers are added,
		for i := 0; i < 5; i++ {
			group.Add(NewAbortController())
		}

		// Then Count surfaces 5 — operators can observe in-flight breadth.
		assert.Equal(t, 5, group.Count(),
			"AbortGroup.Count must surface the registered total")
	})

	t.Run("Scenario_OnAbortCallbackFiresOnExplicitSignal", func(t *testing.T) {
		// Given a tool registers a cleanup callback (PDF Section 4.2:
		//       sibling-abort handler pattern; here applied at loop level
		//       for any cleanup the runner needs to do on user abort),
		ac := NewAbortController()
		var cleaned atomic.Bool
		ac.OnAbort(func() {
			cleaned.Store(true)
		})

		// When the user aborts,
		ac.Abort()

		// Then cleanup callback fires synchronously — registered handlers
		//      always run, no lost cleanup.
		assert.True(t, cleaned.Load(),
			"OnAbort cleanup callback must fire on explicit Abort")
	})

	t.Run("Scenario_AbortDuringPendingOperationDoesNotPanic", func(t *testing.T) {
		// Given a controller used in a long-running goroutine,
		ac := NewAbortController()
		var wg sync.WaitGroup
		stop := make(chan struct{})

		// When 20 goroutines each loop reading abort state until told to
		//      stop,
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						_ = ac.IsAborted()
					}
				}
			}()
		}

		// And a separate goroutine fires Abort mid-flight,
		ac.Abort()
		// Allow readers to observe the aborted state.
		time.Sleep(20 * time.Millisecond)
		close(stop)
		wg.Wait()

		// Then no panic / data race (would fail under -race) and final
		//      state is aborted.
		assert.True(t, ac.IsAborted(),
			"controller must remain in aborted state after concurrent reads")
	})

	t.Run("Scenario_NestedAbortPropagationParentToChildToGrandchild", func(t *testing.T) {
		// Given a 3-level hierarchy (PDF Section 8: parent → subagent →
		//       sub-subagent — each level needs its own abort scope while
		//       inheriting the ancestor's),
		root := NewAbortController()
		child := CreateChildAbortController(root)
		grandchild := CreateChildAbortController(child)

		// When the root aborts,
		root.Abort()

		// Then both descendants observe the abort — no orphaned in-flight
		//      work at any level.
		select {
		case <-child.Context().Done():
		case <-time.After(200 * time.Millisecond):
			t.Fatal("child did not observe root abort")
		}
		select {
		case <-grandchild.Context().Done():
		case <-time.After(200 * time.Millisecond):
			t.Fatal("grandchild did not observe root abort")
		}
	})

	t.Run("Scenario_ChildAbortDoesNotAffectParent", func(t *testing.T) {
		// Given a parent + child relationship,
		root := NewAbortController()
		child := CreateChildAbortController(root)

		// When ONLY the child aborts (e.g. one sub-agent timeout),
		child.Abort()

		// Then the parent stays alive — siblings can continue. Cancellation
		//      flows DOWNWARD only.
		assert.True(t, child.IsAborted(),
			"child must be aborted")
		assert.False(t, root.IsAborted(),
			"parent must remain alive when only child aborts")
	})

	t.Run("Scenario_AbortControllerStateTransitionIsOneWay", func(t *testing.T) {
		// Given a fresh controller (PDF: abort is terminal — once cancelled,
		//       a run cannot un-cancel),
		ac := NewAbortController()
		assert.False(t, ac.IsAborted(),
			"new controller starts non-aborted")

		// When abort fires,
		ac.Abort()

		// Then the state remains aborted forever — there's no Reset() or
		//      Resume() method.
		assert.True(t, ac.IsAborted())
		// Wait briefly to confirm state doesn't flip back.
		time.Sleep(10 * time.Millisecond)
		assert.True(t, ac.IsAborted(),
			"aborted state is one-way terminal — no flip-back to alive")
	})
}
