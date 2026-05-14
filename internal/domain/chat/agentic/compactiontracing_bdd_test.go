package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify OBS-005 (Compaction tracing) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 11 (observability): compaction is one of the four
//     first-class tracing concerns alongside tool calls, permission
//     decisions, and subagent dispatch. Operators must distinguish a
//     "compact happened" event from spurious context spikes.
//   - Section 4.4 (Recovery Mechanisms): reactive compaction fires
//     at most once per turn; analytics must correlate post-compact
//     turns with the compaction that triggered them.
//   - Section 7.3 (Compaction Pipeline): chain analytics matter —
//     "is this the second compact in a row?" affects cost attribution
//     and informs the circuit-breaker behaviour.
//
// AgentHub maps compaction tracing to:
//   - events.go EventContextCompacted — typed event the runner emits
//     at every successful compaction (covered already by OBS-001 enum
//     guard; here the focus is the per-event analytics payload).
//   - context.go AutoCompactTracking — per-session lifecycle state:
//     Compacted (chain started?), TurnCounter (turns since last
//     compaction), TurnID (UUID per compaction event for correlation),
//     ConsecutiveFailures (circuit breaker).
//   - context.go RecompactionInfo — analytics envelope exposed to the
//     runtime: IsRecompactionInChain + TurnsSincePreviousCompact +
//     PreviousCompactTurnID. Mirrors Claude Code's RecompactionInfo
//     in services/compact/compact.ts.
//   - context.go RecompactionInfoFromPtr — nil-safe helper returning
//     TurnsSincePreviousCompact=-1 when no tracking state exists.

func TestBDD_CompactionTracing(t *testing.T) {
	t.Run("Scenario_FreshTrackingHasNoCompactionInChain", func(t *testing.T) {
		// Given a brand-new session (PDF Section 11: tracing must
		//       distinguish the FIRST compact from a re-compact),
		given := AutoCompactTracking{}

		// When the runtime asks for chain context,
		when := given.RecompactionInfo()

		// Then no chain — IsRecompactionInChain=false, no PreviousID.
		assert.False(t, when.IsRecompactionInChain,
			"fresh tracking must NOT report a chain in progress")
		assert.Empty(t, when.PreviousCompactTurnID,
			"no previous compaction means no previous turn ID")
		assert.Equal(t, 0, when.TurnsSincePreviousCompact,
			"fresh tracking has no elapsed turns")
	})

	t.Run("Scenario_AfterCompactionTrackingReportsChainInProgress", func(t *testing.T) {
		// Given a session that has already compacted once,
		given := AutoCompactTracking{
			Compacted:   true,
			TurnCounter: 5,
			TurnID:      "compact-uuid-1",
		}

		// When the runtime asks for chain context (PDF Section 7.3:
		//       chain analytics — "is this the Nth compact?"),
		when := given.RecompactionInfo()

		// Then IsRecompactionInChain is true; the prior turn ID is
		//      surfaced for correlation; turn delta is observable.
		assert.True(t, when.IsRecompactionInChain,
			"post-compaction tracking must report chain in progress")
		assert.Equal(t, "compact-uuid-1", when.PreviousCompactTurnID,
			"prior turn ID must surface for analytics correlation")
		assert.Equal(t, 5, when.TurnsSincePreviousCompact,
			"turn delta since last compact must be observable")
	})

	t.Run("Scenario_NilTrackingPointerReturnsSentinelMinusOne", func(t *testing.T) {
		// Given a runtime path where no tracking state was constructed
		//       (e.g. early bootstrap, in-memory mock without ContextManager),
		var noTracking *AutoCompactTracking

		// When the helper is invoked,
		when := RecompactionInfoFromPtr(noTracking)

		// Then a sentinel -1 is returned for TurnsSincePreviousCompact
		//      — operators distinguish "no tracking" from "0 turns ago".
		assert.Equal(t, -1, when.TurnsSincePreviousCompact,
			"nil tracking must return -1 sentinel (NOT zero — zero would mean 'just compacted')")
		assert.False(t, when.IsRecompactionInChain,
			"nil tracking has no chain")
	})

	t.Run("Scenario_TurnIDDistinguishesIndividualCompactions", func(t *testing.T) {
		// Given two separate compaction events (PDF Section 11: every
		//       compaction is uniquely identifiable for audit correlation),
		first := AutoCompactTracking{Compacted: true, TurnID: "compact-uuid-A"}
		second := AutoCompactTracking{Compacted: true, TurnID: "compact-uuid-B"}

		// When operators correlate events,
		// Then distinct TurnIDs allow each compaction to be queried
		//      independently in the audit log.
		assert.NotEqual(t, first.TurnID, second.TurnID,
			"distinct compaction events must have distinct TurnIDs")
		assert.NotEqual(t, first.RecompactionInfo().PreviousCompactTurnID,
			second.RecompactionInfo().PreviousCompactTurnID,
			"RecompactionInfo must surface the unique TurnID")
	})

	t.Run("Scenario_TurnCounterMeasuresPostCompactWork", func(t *testing.T) {
		// Given the same compaction state observed at two points
		//       (PDF: TurnCounter answers "how many turns has the
		//       agent run since the last compact?"),
		early := AutoCompactTracking{Compacted: true, TurnCounter: 1, TurnID: "x"}
		late := AutoCompactTracking{Compacted: true, TurnCounter: 12, TurnID: "x"}

		// When analytics ingests both,
		// Then the delta tells operators how productive the
		//      post-compact window was.
		assert.Less(t, early.TurnCounter, late.TurnCounter,
			"TurnCounter must increase with elapsed turns")
		assert.Equal(t, 1, early.RecompactionInfo().TurnsSincePreviousCompact)
		assert.Equal(t, 12, late.RecompactionInfo().TurnsSincePreviousCompact)
	})

	t.Run("Scenario_ConsecutiveFailuresCircuitBreakerHasPositiveBound", func(t *testing.T) {
		// Given the circuit-breaker constant (PDF Section 4.4: stop
		//       retrying when context is irrecoverably over limit),
		// When the runtime checks the bound,
		// Then a positive integer caps the wasted API calls.
		assert.Greater(t, maxConsecutiveCompactFailures, 0,
			"circuit-breaker bound must be positive")
		assert.LessOrEqual(t, maxConsecutiveCompactFailures, 10,
			"circuit-breaker bound must be sane (<=10)")
	})

	t.Run("Scenario_ContextManagerExposesTrackingForObservability", func(t *testing.T) {
		// Given a fresh ContextManager (PDF Section 11: observability
		//       requires the runner to surface internal tracking state
		//       to the analytics layer),
		given := NewContextManager()

		// When the analytics layer reads tracking,
		when := given.Tracking()

		// Then it returns a value (zero-value AutoCompactTracking) —
		//      analytics never has to nil-check.
		assert.False(t, when.Compacted,
			"fresh ContextManager has no prior compaction")
		assert.Empty(t, when.TurnID,
			"fresh ContextManager has no compaction TurnID yet")
	})

	t.Run("Scenario_RecompactionInfoZeroValueIsSafeForAnalyticsSink", func(t *testing.T) {
		// Given an analytics sink ingesting RecompactionInfo blobs
		//       (PDF Section 11: tracing payloads must serialize
		//       safely even on the zero-value path),
		given := RecompactionInfo{}

		// When the sink processes,
		// Then no panic, all fields readable.
		assert.False(t, given.IsRecompactionInChain)
		assert.Equal(t, 0, given.TurnsSincePreviousCompact)
		assert.Empty(t, given.PreviousCompactTurnID)
	})

	t.Run("Scenario_EventContextCompactedTokenIsCanonical", func(t *testing.T) {
		// Given the runner emits typed events (PDF Section 11 +
		//       OBS-001 envelope catalogue),
		// When the SSE consumer dispatches by type,
		// Then the canonical token is "context_compacted" — clients
		//      filter by string match.
		assert.Equal(t, RunEventType("context_compacted"), EventContextCompacted,
			"compaction event token must be 'context_compacted'")
	})
}
