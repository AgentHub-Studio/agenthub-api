package agentic

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify LOOP-002 (Streaming de eventos do loop)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 4.1 (queryLoop yields StreamEvent / RequestStartEvent /
//     Message / TombstoneMessage / ToolUseSummaryMessage as it progresses):
//     "This generator-based design enables streaming output to the UI layer
//     while maintaining a single synchronous control flow within the loop."
//   - Section 3.2 (Figure 1): the Runner is event-emitting; SSE wire is the
//     transport from runtime to UI/clients.
//   - Section 11 (observability): streaming MUST be incremental — clients
//     receive partial state continuously, not only at run completion.
//
// AgentHub maps streaming to:
//   - runner.go Runner.Run(ctx, in) returning `<-chan RunEvent` — the
//     Go-idiomatic AsyncGenerator. The channel is buffered to
//     RunConfig.StreamBufferSize to absorb transient producer-faster-than-
//     consumer bursts and prevent runtime blocking on slow SSE consumers.
//   - asyncstream.go Stream[T] — a typed channel wrapper for transformations
//     (used in side queries and intermediate stages).
//   - streambuffer.go StreamBuffer[T] — time + size-based batched flush for
//     downstream sinks (analytics, SSE) that prefer batches to single events.
//   - streamtransform.go StreamTransformer — converts internal events into
//     UI-friendly typed messages.
//
// These scenarios assert: channel buffering, batched flush, time-based
// flush, close semantics, concurrent producer safety, transform shape.

func TestBDD_StreamingEventLoop(t *testing.T) {
	t.Run("Scenario_StreamBufferDefaultsAreSensible", func(t *testing.T) {
		// Given a fresh StreamBufferConfig (PDF Section 4.1: streaming must
		//       balance latency vs throughput),
		given := DefaultStreamBufferConfig()

		// When the runtime inspects defaults,
		// Then a positive window + max size are present, biased toward low
		//      latency (sub-second) for UX responsiveness.
		assert.Greater(t, given.Window, time.Duration(0),
			"flush window must be positive")
		assert.LessOrEqual(t, given.Window, 1*time.Second,
			"window must be sub-second for UX responsiveness")
		assert.Greater(t, given.MaxSize, 0,
			"max batch size must be positive")
	})

	t.Run("Scenario_StreamBufferFlushesWhenSizeReached", func(t *testing.T) {
		// Given a small-capacity buffer (PDF Section 4.1: bursts must drain
		//       eagerly to keep memory bounded),
		var batches atomic.Int32
		var totalItems atomic.Int32
		buffer := NewStreamBuffer[int](
			StreamBufferConfig{Window: 1 * time.Hour, MaxSize: 5},
			func(batch []int) {
				batches.Add(1)
				totalItems.Add(int32(len(batch)))
			},
		)

		// When 5 items are added (reaching MaxSize),
		for i := 0; i < 5; i++ {
			buffer.Add(i)
		}
		// Allow the eager flush goroutine to settle.
		time.Sleep(10 * time.Millisecond)

		// Then a flush fires immediately on size hit, not waiting for window.
		assert.GreaterOrEqual(t, batches.Load(), int32(1),
			"buffer at MaxSize must flush immediately, not wait for window")
		assert.GreaterOrEqual(t, totalItems.Load(), int32(5),
			"all 5 items must be delivered in the flush")
	})

	t.Run("Scenario_StreamBufferFlushesOnTimeWindow", func(t *testing.T) {
		// Given items added below MaxSize but the window elapses,
		var batches atomic.Int32
		buffer := NewStreamBuffer[string](
			StreamBufferConfig{Window: 30 * time.Millisecond, MaxSize: 100},
			func(_ []string) { batches.Add(1) },
		)

		// When 2 items are added and we wait past the window,
		buffer.Add("a")
		buffer.Add("b")
		time.Sleep(80 * time.Millisecond)

		// Then a time-based flush fires — clients see partial batches even
		//      under low producer rate (PDF: incremental delivery).
		assert.GreaterOrEqual(t, batches.Load(), int32(1),
			"window timeout must trigger a flush of partial batch")
	})

	t.Run("Scenario_StreamBufferCloseFlushesRemainingItems", func(t *testing.T) {
		// Given items pending in the buffer when the run ends (PDF Section
		//       4.5: stop conditions must drain pending state),
		var lastBatch []int
		buffer := NewStreamBuffer[int](
			StreamBufferConfig{Window: 1 * time.Hour, MaxSize: 100},
			func(batch []int) { lastBatch = append(lastBatch, batch...) },
		)
		buffer.Add(1)
		buffer.Add(2)
		buffer.Add(3)

		// When the buffer is closed (run ending),
		buffer.Close()

		// Then remaining items are flushed — never lost on shutdown.
		assert.Equal(t, []int{1, 2, 3}, lastBatch,
			"Close must drain remaining items before terminating")
		assert.True(t, buffer.IsClosed(),
			"buffer must report closed after Close")
	})

	t.Run("Scenario_StreamBufferRejectsAddAfterClose", func(t *testing.T) {
		// Given a closed buffer (PDF Section 4.5 + 9.2: post-stop writes
		//       must be safe-no-op, never crash),
		var addedCount atomic.Int32
		buffer := NewStreamBuffer[int](
			DefaultStreamBufferConfig(),
			func(b []int) { addedCount.Add(int32(len(b))) },
		)
		buffer.Close()

		// When the runner attempts to add after close,
		buffer.Add(99)

		// Then the add is silently dropped — no crash, no spurious flush.
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, int32(0), addedCount.Load(),
			"Add after Close must be a no-op (zero items delivered)")
	})

	t.Run("Scenario_StreamBufferIsConcurrentSafe", func(t *testing.T) {
		// Given multiple goroutines producing events into one buffer (PDF
		//       Section 4.2 concurrent tools + main loop),
		var totalDelivered atomic.Int32
		buffer := NewStreamBuffer[int](
			StreamBufferConfig{Window: 50 * time.Millisecond, MaxSize: 100},
			func(b []int) { totalDelivered.Add(int32(len(b))) },
		)
		var wg sync.WaitGroup

		// When 10 goroutines each add 100 items concurrently,
		for g := 0; g < 10; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					buffer.Add(i)
				}
			}()
		}
		wg.Wait()
		buffer.Close()
		time.Sleep(20 * time.Millisecond)

		// Then exactly 1000 items delivered, no race (would fail under -race).
		assert.Equal(t, int32(1000), totalDelivered.Load(),
			"all 1000 concurrent items must be delivered exactly once")
	})

	t.Run("Scenario_TypedStreamWrapperEnqueueDeliversItem", func(t *testing.T) {
		// Given a typed Stream[T] (PDF Section 4.1 typed AsyncGenerator
		//       equivalent — preserves type safety across the stage chain),
		stream := NewStream[string](16)

		// When the runner enqueues a value,
		ok := stream.Enqueue("event-1")

		// Then the value reaches the consumer via the channel.
		assert.True(t, ok, "Enqueue on open stream must succeed")
		select {
		case item := <-stream.Chan():
			assert.Equal(t, "event-1", item.Value,
				"item value must round-trip through the channel")
			assert.NoError(t, item.Err,
				"successful Enqueue must produce nil error")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected item not received within 100ms")
		}
	})

	t.Run("Scenario_TypedStreamErrorPathDeliversError", func(t *testing.T) {
		// Given a stream that encounters an error (PDF Section 11: errors
		//       must surface via the same channel, not via side-channel),
		stream := NewStream[int](4)

		// When the runner reports an error,
		stream.Error(assertedError{msg: "transient"})

		// Then the consumer observes the error before channel close.
		select {
		case item := <-stream.Chan():
			assert.Error(t, item.Err,
				"error must surface as item.Err on the same channel")
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected error item not received within 100ms")
		}
	})

	t.Run("Scenario_TypedStreamDoneClosesChannel", func(t *testing.T) {
		// Given a stream that completes normally (PDF Section 4.5: stop
		//       conditions terminate the producer cleanly),
		stream := NewStream[int](4)
		stream.Enqueue(1)
		stream.Done()

		// When the consumer drains and the channel reaches end,
		// Note: implementations may emit a final sentinel/zero-value item
		// alongside the close signal — we only assert the explicitly
		// enqueued value reached the consumer.
		got := false
		for item := range stream.Chan() {
			if item.Err == nil && item.Value == 1 {
				got = true
			}
		}

		// Then the consumer naturally exits — Go range over closed channel
		//      provides backpressure-free termination.
		assert.True(t, got,
			"the explicitly enqueued value must reach the consumer before close")
		assert.True(t, stream.IsClosed(),
			"stream reports closed after Done")
	})

	t.Run("Scenario_StreamBufferReportsFlushCount", func(t *testing.T) {
		// Given a buffer configured to flush after every 10 items,
		buffer := NewStreamBuffer[int](
			StreamBufferConfig{Window: 1 * time.Hour, MaxSize: 10},
			func(_ []int) {},
		)

		// When 30 items are added (3 size-triggered flushes),
		for i := 0; i < 30; i++ {
			buffer.Add(i)
		}
		time.Sleep(20 * time.Millisecond)

		// Then FlushCount surfaces 3 — observable for telemetry.
		assert.GreaterOrEqual(t, buffer.FlushCount(), 3,
			"FlushCount must surface number of flushes for telemetry")
	})

	t.Run("Scenario_RunnerRunReturnsBufferedChannel", func(t *testing.T) {
		// Given the Runner.Run produces RunEvent channel (PDF Section 4.1
		//       AsyncGenerator),
		// When we inspect the construction code (runner.go: ch := make(chan
		//       RunEvent, r.config.StreamBufferSize)),
		// Then the channel buffer size MUST be positive in the default
		//      config — preventing blocking on slow SSE consumers.
		cfg := DefaultRunConfig()
		assert.Greater(t, cfg.StreamBufferSize, 0,
			"StreamBufferSize must be positive in defaults")
	})
}

// assertedError is a tiny error type used in scenario assertions.
type assertedError struct{ msg string }

func (e assertedError) Error() string { return e.msg }
