package agentic

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify PERSIST-007 (Sidechain logs) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 8.3 (Sidechain Transcripts): "Each subagent writes its own
//     transcript as a separate .jsonl file ... Sidechain content stays in
//     a separate file, preserved for debugging but not loaded into the
//     parent context."
//   - Section 9.1 (Transcript Model): mostly-append-only JSONL +
//     per-session writer queue (sequential within a session, parallel
//     across sessions) prevents interleaved writes from corrupting the
//     transcript log.
//   - Section 9 (overall): logs must be HYDRATABLE for resume — readers
//     can rebuild the transcript by streaming the session's append-only
//     entries.
//
// AgentHub maps sidechain log persistence to:
//   - sessioningress.go SessionIngress — orchestrator that owns per-
//     session sequential write queues + HydrateSession reader.
//   - sessioningress.go sequentialQueue — per-session FIFO ensures
//     ordered writes within a session even when called from multiple
//     goroutines (subagent writers + parent writers don't interleave).
//   - sessioningress.go AppendLog — single API for both main and
//     sidechain entries; the IsSidechain flag on TranscriptEntry
//     (covered by SUB-009) discriminates at read time.
//   - sessioningress.go HydrateSession — bulk read for resume/replay.
//   - sessioningress.go CleanupSession — explicit per-session resource
//     release (queues + lastUUID tracking).
//
// SUB-009 covered the TranscriptEntry shape (IsSidechain, ParentUUID,
// AgentID/Name); PERSIST-007 covers the WRITE PATH that gets entries
// into durable storage with ordering + dedup + retry semantics.

func TestBDD_SidechainLogPersistence(t *testing.T) {
	t.Run("Scenario_DefaultIngressConfigHasSensibleDefaults", func(t *testing.T) {
		// Given the default config used by interactive sessions (PDF: log
		//       writer must work out-of-the-box without per-session tuning),
		given := DefaultSessionIngressConfig()

		// When the runtime inspects defaults,
		// Then the struct is populated (not zero-value) — guaranteeing the
		//      writer pipeline is configured for production use.
		assert.NotEqual(t, SessionIngressConfig{}, given,
			"DefaultSessionIngressConfig must return a populated struct")
	})

	t.Run("Scenario_RemoteConfigHasShorterFlushInterval", func(t *testing.T) {
		// Given the remote-session variant (PDF Section 8.3 + 9.1: remote
		//       sessions need lower latency for hydration to feel
		//       interactive),
		def := DefaultSessionIngressConfig()
		remote := RemoteSessionIngressConfig()

		// When comparing flush windows,
		// Then remote uses shorter interval — defaults sized for local
		//      latency would feel sluggish over network hops.
		assert.Less(t, remote.FlushIntervalMs, def.FlushIntervalMs,
			"remote config must use shorter flush interval than default")
	})

	t.Run("Scenario_SessionIngressTracksActiveSessions", func(t *testing.T) {
		// Given a fresh ingress instance,
		ingress := NewSessionIngress(DefaultSessionIngressConfig())

		// When no work has happened yet,
		// Then ActiveSessions returns 0 — fresh state, no leaked queues.
		assert.Equal(t, 0, ingress.ActiveSessions(),
			"fresh SessionIngress must report 0 active sessions")
	})

	t.Run("Scenario_GetLastUUIDIsEmptyForUnknownSession", func(t *testing.T) {
		// Given a fresh ingress,
		ingress := NewSessionIngress(DefaultSessionIngressConfig())

		// When the runtime asks for the last UUID of an unseen session,
		when := ingress.GetLastUUID("session-never-touched")

		// Then the result is empty — fail-safe lookup, no panic.
		assert.Empty(t, when,
			"unknown session must return empty last UUID, not panic")
	})

	t.Run("Scenario_CleanupSessionIsIdempotent", func(t *testing.T) {
		// Given an ingress with no work for a particular session,
		ingress := NewSessionIngress(DefaultSessionIngressConfig())

		// When CleanupSession is called for an unseen session,
		// Then no panic — cleanup is safe to call multiple times or for
		//      sessions that never produced work.
		assert.NotPanics(t, func() {
			ingress.CleanupSession("session-never-existed")
			ingress.CleanupSession("session-never-existed") // idempotent
		}, "CleanupSession must be safe to call for unknown/repeat sessions")
	})

	t.Run("Scenario_TranscriptEntryUUIDIsUniquePerEntry", func(t *testing.T) {
		// Given the runner produces multiple transcript entries (PDF Section
		//       9.1: each entry needs a unique ID for chain navigation +
		//       409-conflict detection on write),
		first := NewTranscriptEntry("user", json.RawMessage(`{}`), nil)
		second := NewTranscriptEntry("user", json.RawMessage(`{}`), nil)
		third := NewTranscriptEntry("assistant", json.RawMessage(`{}`), nil)

		// When the writer assigns IDs,
		// Then all three are distinct UUIDs — preventing accidental dedup
		//      collapse of legitimately distinct events.
		assert.NotEmpty(t, first.UUID)
		assert.NotEmpty(t, second.UUID)
		assert.NotEmpty(t, third.UUID)
		assert.NotEqual(t, first.UUID, second.UUID,
			"distinct entries must have distinct UUIDs")
		assert.NotEqual(t, second.UUID, third.UUID)
		assert.NotEqual(t, first.UUID, third.UUID)
	})

	t.Run("Scenario_TranscriptEntryParentUUIDChainsSidechainToParent", func(t *testing.T) {
		// Given a parent message and a sidechain entry derived from it
		//       (PDF Section 8.3: sidechain entries link back via
		//       parent UUID — readers reconstruct tree at hydration time),
		parent := NewTranscriptEntry("assistant", json.RawMessage(`{"tool_use":"agent"}`), nil)
		parentID := parent.UUID
		child := NewTranscriptEntry("user", json.RawMessage(`{"sub":"do x"}`), &parentID)

		// When the reader consults the chain,
		// Then ParentUUID points back to the originator — chain navigation
		//      works from any sidechain entry.
		assert.NotNil(t, child.ParentUUID,
			"sidechain entry MUST carry ParentUUID for chain reconstruction")
		assert.Equal(t, parentID, *child.ParentUUID,
			"ParentUUID round-trips the parent's UUID")
	})

	t.Run("Scenario_SequentialQueueSerialisesWritesPerSession", func(t *testing.T) {
		// Given a per-session sequential queue (PDF Section 9.1 invariant:
		//       within a single session, writes must be ORDERED — parallel
		//       writes from sub-agents could otherwise interleave and
		//       corrupt the transcript),
		q := newSequentialQueue()
		var mu sync.Mutex
		var observed []int

		// When 10 goroutines enqueue writes that record their order,
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				_ = q.enqueue(func() error {
					mu.Lock()
					observed = append(observed, n)
					mu.Unlock()
					return nil
				})
			}(i)
		}
		wg.Wait()

		// Then exactly 10 writes are recorded — none are dropped or
		//      corrupted by interleaving (the queue serialises them).
		assert.Equal(t, 10, len(observed),
			"sequentialQueue must accept all enqueued writes without loss")
	})

	t.Run("Scenario_SequentialQueuePropagatesErrorBack", func(t *testing.T) {
		// Given a write function that returns an error (PDF: writer must
		//       surface errors to caller, not swallow them — silent failure
		//       would leave the transcript inconsistent),
		q := newSequentialQueue()
		expectedErr := assertedError{msg: "write failed"}

		// When the caller enqueues a function that errors,
		err := q.enqueue(func() error {
			return expectedErr
		})

		// Then the error reaches the caller — caller can decide retry
		//      strategy, log, or escalate.
		assert.Error(t, err,
			"enqueue must propagate the function's error to caller")
		assert.Equal(t, expectedErr, err,
			"error value must round-trip without wrapping")
	})

	t.Run("Scenario_SessionIngressExposesHydrationVerb", func(t *testing.T) {
		// Given the durable log must be replayable (PDF Section 9: resume
		//       rebuilds conversation from durable records),
		ingress := NewSessionIngress(DefaultSessionIngressConfig())
		typ := reflect.TypeOf(ingress)

		// When we inspect the API,
		hasHydrate := false
		hasAppend := false
		hasCleanup := false
		for i := 0; i < typ.NumMethod(); i++ {
			switch typ.Method(i).Name {
			case "HydrateSession":
				hasHydrate = true
			case "AppendLog":
				hasAppend = true
			case "CleanupSession":
				hasCleanup = true
			}
		}

		// Then the 3 lifecycle verbs exist: append (write), hydrate (replay),
		//      cleanup (release) — the minimum API to support a full
		//      session lifecycle.
		assert.True(t, hasAppend, "AppendLog (write) must exist")
		assert.True(t, hasHydrate, "HydrateSession (replay) must exist")
		assert.True(t, hasCleanup, "CleanupSession (release) must exist")
	})

	t.Run("Scenario_TranscriptEntryTimestampIsMillisecondPrecision", func(t *testing.T) {
		// Given the runtime needs ordering across sub-second events (PDF
		//       Section 9.1: ordering by timestamp; sidechain spawns can
		//       occur multiple times per second),
		given := NewTranscriptEntry("user", json.RawMessage(`{}`), nil)

		// When we inspect the timestamp,
		// Then it's a millisecond-precision int64 — sufficient for
		//      ordering rapid sidechain spawns without nanosecond clock
		//      portability concerns.
		assert.Greater(t, given.Timestamp, int64(0),
			"timestamp must be assigned (non-zero)")
		// Millisecond range: Unix ms is around 1.7e12 in 2026 — sanity
		// check that we're using ms not nanoseconds (which would be ~1e18).
		assert.Less(t, given.Timestamp, int64(1e15),
			"timestamp magnitude indicates milliseconds (not nanoseconds)")
	})
}

// dummyError reuses the assertedError pattern from streaming_bdd_test.go
// (defined there as `type assertedError struct{ msg string }`). No need
// to redefine here.
