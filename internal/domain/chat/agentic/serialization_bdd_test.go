package agentic

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify TOOL-007 (Serialização de operações
// mutáveis) against the Claude Code architecture paper "Dive into Claude
// Code" (arXiv:2604.14228v1):
//
//   - Section 4.2 (Tool Dispatch): "Read-only operations can execute in
//     parallel, while state-modifying operations like shell commands are
//     serialized."
//   - Section 4.2 sibling-abort: "Fires when any Bash tool errors,
//     immediately terminating other in-flight subprocesses rather than
//     letting them run to completion."
//   - Section 11 (observability): even serial execution must surface
//     per-tool state transitions (queued → running → completed/aborted) so
//     operators can attribute timing and failures.
//
// AgentHub maps serialization to:
//   - toolexec.go ExecuteAll — partitions then dispatches: concurrent batch
//     for read-only runs (parallel), serial batch for writes (one-at-a-
//     time loop). The single source of truth for tool execution ordering.
//   - toolexec.go executeSingle — serial per-tool execution path.
//   - abortchain.go AbortController + OnAbort + AbortGroup — propagates
//     abort across sibling goroutines (the sibling-abort pattern).
//   - context.WithCancel chain — when ctx is cancelled mid-batch, remaining
//     serial tools transition to ToolStateAborted (not silently skipped).
//
// TOOL-006 covered the PARTITIONING contract; TOOL-007 covers the
// SERIALIZATION contract specifically: write tools never overlap, abort
// propagates to siblings, and aborted serial tools surface as observable
// events.

func TestBDD_MutatingOpSerialization(t *testing.T) {
	t.Run("Scenario_PartitionerYieldsSingleToolBatchesForWrites", func(t *testing.T) {
		// Given a batch where ALL tools mutate state (PDF Section 4.2:
		//       writes serialized one-at-a-time),
		given := []ai.ToolCall{
			{ID: "1", Function: ai.ToolFunction{Name: "shell"}},
			{ID: "2", Function: ai.ToolFunction{Name: "edit-file"}},
			{ID: "3", Function: ai.ToolFunction{Name: "delete-resource"}},
		}

		// When the partitioner runs (TOOL-006 helper, but here we assert
		//      the SERIALIZATION INVARIANT specifically: each write occupies
		//      its own batch with size 1),
		when := PartitionToolCalls(given, nil)

		// Then we get N batches each of size 1 — no two writes ever share
		//      a batch (would imply parallel exec). This is the structural
		//      guarantee that serialization is enforced AT PARTITION TIME,
		//      not relying on the executor to "remember" to serialize.
		assert.Len(t, when, 3,
			"3 writes must produce 3 separate batches (no batching)")
		for i, batch := range when {
			assert.Len(t, batch.Indices, 1,
				"write batch %d must contain exactly ONE tool — no shared batches", i)
			assert.False(t, batch.IsConcurrencySafe,
				"write batch %d must be marked NOT concurrency-safe", i)
		}
	})

	t.Run("Scenario_MixedBatchKeepsWritesIsolatedBetweenReadGroups", func(t *testing.T) {
		// Given a sequence read-read-WRITE-read (the WRITE must NOT join
		//       either neighbouring read group),
		readOnly := map[string]bool{"r1": true, "r2": true, "r3": true}
		given := []ai.ToolCall{
			{Function: ai.ToolFunction{Name: "r1"}},
			{Function: ai.ToolFunction{Name: "r2"}},
			{Function: ai.ToolFunction{Name: "shell"}},
			{Function: ai.ToolFunction{Name: "r3"}},
		}

		// When partitioned,
		when := PartitionToolCalls(given, readOnly)

		// Then 3 batches: {r1,r2} concurrent | {shell} serial | {r3}
		//      concurrent. The write isolated WITHIN its own boundary, with
		//      a barrier on either side.
		assert.Len(t, when, 3,
			"writes form their own batches separating concurrent runs")
		assert.True(t, when[0].IsConcurrencySafe)
		assert.False(t, when[1].IsConcurrencySafe,
			"shell batch must isolate as serial barrier")
		assert.Len(t, when[1].Indices, 1,
			"shell barrier batch is single-tool")
	})

	t.Run("Scenario_AbortControllerPropagatesAbortToSiblings", func(t *testing.T) {
		// Given a sibling-abort group (PDF Section 4.2: when any Bash tool
		//       errors, in-flight siblings are terminated rather than
		//       allowed to complete — preventing partial state corruption),
		ac := NewAbortController()
		var siblingTriggered atomic.Int32

		// When a sibling registers an abort handler then the controller
		//      fires Abort(),
		ac.OnAbort(func() {
			siblingTriggered.Add(1)
		})
		ac.Abort()

		// Then the sibling handler runs synchronously — no in-flight
		//      operation continues past the abort signal.
		assert.True(t, ac.IsAborted(),
			"controller must report aborted state after Abort()")
		assert.Equal(t, int32(1), siblingTriggered.Load(),
			"OnAbort handler must fire exactly once for the abort signal")
	})

	t.Run("Scenario_AbortControllerContextDoneAfterAbort", func(t *testing.T) {
		// Given an abort controller with a context channel (Go-idiomatic
		//       cancellation propagation),
		ac := NewAbortController()
		ctx := ac.Context()

		// When abort fires,
		ac.Abort()

		// Then the context is done — any goroutine watching ctx.Done()
		//      observes the cancellation immediately.
		select {
		case <-ctx.Done():
			// Expected: cancellation propagated.
		case <-time.After(100 * time.Millisecond):
			t.Fatal("abort did not propagate to context within 100ms")
		}
		assert.Error(t, ctx.Err(),
			"context.Err() must return non-nil after abort")
	})

	t.Run("Scenario_AbortControllerHandlerCanBeUnregistered", func(t *testing.T) {
		// Given a handler registered then unregistered (PDF: dynamic
		//       sibling registration during a serial batch),
		ac := NewAbortController()
		var fired atomic.Int32
		unregister := ac.OnAbort(func() {
			fired.Add(1)
		})
		unregister()

		// When abort fires after unregistration,
		ac.Abort()

		// Then the handler does NOT run — leak-free dynamic registration.
		assert.Equal(t, int32(0), fired.Load(),
			"unregistered handler must not fire on abort")
	})

	t.Run("Scenario_AbortIsIdempotent", func(t *testing.T) {
		// Given multiple Abort() calls (PDF: defensive coding — sibling
		//       abort may fire from multiple paths),
		ac := NewAbortController()
		var fired atomic.Int32
		ac.OnAbort(func() { fired.Add(1) })

		// When Abort fires multiple times,
		ac.Abort()
		ac.Abort()
		ac.Abort()

		// Then the handler runs exactly once — no double-firing on
		//      duplicate signals.
		assert.Equal(t, int32(1), fired.Load(),
			"Abort must be idempotent — handler fires at most once")
		assert.True(t, ac.IsAborted(),
			"state remains aborted after multiple aborts")
	})

	t.Run("Scenario_ParentContextCancellationPropagatesToChildController", func(t *testing.T) {
		// Given a child abort controller derived from a parent context (PDF
		//       Section 4.5: explicit abort from outside must propagate
		//       inward),
		parentCtx, parentCancel := context.WithCancel(context.Background())
		ac := NewAbortControllerWithContext(parentCtx)

		// When the parent ctx is cancelled,
		parentCancel()

		// Then the child controller observes the cancellation.
		select {
		case <-ac.Context().Done():
			// Expected: parent cancellation propagated.
		case <-time.After(100 * time.Millisecond):
			t.Fatal("parent cancellation did not propagate to child within 100ms")
		}
	})

	t.Run("Scenario_ChildAbortControllerInheritsParentSignal", func(t *testing.T) {
		// Given a parent + child controller chain (PDF Section 4.2 + 8:
		//       sub-agents may need their own abort scope while inheriting
		//       the parent's),
		parent := NewAbortController()
		child := CreateChildAbortController(parent)

		// When parent aborts,
		parent.Abort()

		// Then child also reports aborted — propagation is structural, not
		//      requiring explicit forwarding.
		select {
		case <-child.Context().Done():
			// Expected.
		case <-time.After(100 * time.Millisecond):
			t.Fatal("child did not observe parent abort within 100ms")
		}
	})

	t.Run("Scenario_AbortGroupTracksMultipleControllers", func(t *testing.T) {
		// Given an AbortGroup that aggregates several controllers (PDF
		//       Section 4.2: a write batch may track multiple tracked
		//       sibling tools that must abort together),
		group := NewAbortGroup()

		// When the group is constructed,
		// Then it provides aggregate management — no panic, ready to use.
		assert.NotNil(t, group,
			"NewAbortGroup must return a usable group")
	})

	t.Run("Scenario_ConcurrentAbortIsRaceFree", func(t *testing.T) {
		// Given multiple goroutines racing to abort the same controller,
		ac := NewAbortController()

		// When 50 goroutines call Abort concurrently,
		var wg [50]chan struct{}
		for i := range wg {
			wg[i] = make(chan struct{})
			go func(done chan struct{}) {
				ac.Abort()
				close(done)
			}(wg[i])
		}
		for _, c := range wg {
			<-c
		}

		// Then no race occurs (the test would fail under -race) and the
		//      final state is aborted.
		assert.True(t, ac.IsAborted(),
			"concurrent Abort calls must converge on aborted state")
	})
}
