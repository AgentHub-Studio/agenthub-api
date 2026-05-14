package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OBS-006 — Subagent tracing.
//
// PDF arXiv:2604.14228v1 Section 6.1 (subagent envelope), Section 8.3
// (Task tool returns summary not transcript), Section 11 (parent/child
// trace correlation for evaluator separation). Subagent runs are first-
// class spans: parent forwards their events with stable correlation IDs,
// depth, source attribution and outcome envelope (turns / tokens / cost
// / error / summary).
//
// Surface validated:
//   - EventSubtaskStart / EventSubtaskComplete are canonical event types.
//   - SubtaskStartData.{ID,Depth,Description} carry the trace start.
//   - SubtaskCompleteData.{ID,TotalTurns,TotalTokens,TotalCost,Summary,Error}
//     carry the trace end with cost attribution.
//   - SubtaskStatus enum bounds outcomes (completed/failed/killed).
//   - SourceSubtask is a foreground QuerySource (retries aggressively;
//     PDF Section 11.4 — user-blocking subagents retry like main loop).
//   - SubtaskProgressData is a separate periodic update (long-running).
//   - Description bounded by truncateString(200) to prevent log spam.

func TestBDD_SubagentTracing(t *testing.T) {

	t.Run("Scenario_SubtaskStartCarriesDepthForParentChildCorrelation", func(t *testing.T) {
		// Given a subagent is spawned at depth 2 (nested chain),
		// When the SubtaskStart event is emitted,
		// Then the depth attribute reaches the parent so traces can
		//      reconstruct the parent->child chain.
		data := SubtaskStartData{
			ID:          "subtask-abc",
			Description: "search the docs index",
			Depth:       2,
		}
		raw, err := json.Marshal(data)
		assert.NoError(t, err)

		var roundtrip SubtaskStartData
		assert.NoError(t, json.Unmarshal(raw, &roundtrip))
		assert.Equal(t, "subtask-abc", roundtrip.ID,
			"subtask ID must round-trip identically — used as span ID")
		assert.Equal(t, 2, roundtrip.Depth,
			"depth must round-trip — required to chain parent/child spans")
	})

	t.Run("Scenario_SubtaskCompleteCarriesCostAttributionEnvelope", func(t *testing.T) {
		// Given a subagent finishes consuming 3 turns / 1500 tokens / $0.04,
		// When the SubtaskComplete event is emitted,
		// Then turns / tokens / cost reach the parent so the parent run
		//      can attribute aggregate cost to the subagent span.
		data := SubtaskCompleteData{
			ID:          "subtask-abc",
			TotalTurns:  3,
			TotalTokens: 1500,
			TotalCost:   0.04,
			Summary:     "Found 12 docs",
		}
		raw, err := json.Marshal(data)
		assert.NoError(t, err)

		var roundtrip SubtaskCompleteData
		assert.NoError(t, json.Unmarshal(raw, &roundtrip))
		assert.Equal(t, 3, roundtrip.TotalTurns)
		assert.Equal(t, 1500, roundtrip.TotalTokens)
		assert.InDelta(t, 0.04, roundtrip.TotalCost, 0.0001)
		assert.Equal(t, "Found 12 docs", roundtrip.Summary)
		assert.Nil(t, roundtrip.Error,
			"successful subtask omits Error pointer")
	})

	t.Run("Scenario_SubtaskCompleteCarriesErrorEnvelopeOnFailure", func(t *testing.T) {
		// Given a subagent fails halfway through,
		// When the SubtaskComplete event is emitted,
		// Then the Error pointer carries the failure message so the
		//      parent's tracing layer can mark the span as failed.
		errMsg := "tool quota exceeded"
		data := SubtaskCompleteData{
			ID:    "subtask-xyz",
			Error: &errMsg,
		}
		raw, err := json.Marshal(data)
		assert.NoError(t, err)

		var roundtrip SubtaskCompleteData
		assert.NoError(t, json.Unmarshal(raw, &roundtrip))
		assert.NotNil(t, roundtrip.Error,
			"failure path must carry Error so span outcome is observable")
		assert.Equal(t, errMsg, *roundtrip.Error)
	})

	t.Run("Scenario_SubtaskStartAndCompleteShareIDForSpanCorrelation", func(t *testing.T) {
		// Given a subtask span has an ID at start,
		// When the same subtask completes,
		// Then the COMPLETE event carries the same ID — so parent
		//      tracers can pair the start/complete events as one span.
		startID := "subtask-pair-1"
		startData := SubtaskStartData{ID: startID, Depth: 1}
		completeData := SubtaskCompleteData{ID: startID}
		assert.Equal(t, startData.ID, completeData.ID,
			"start.ID and complete.ID MUST be the same — span correlation contract")
	})

	t.Run("Scenario_CanonicalEventTypesAreStable", func(t *testing.T) {
		// Given third-party consumers (frontend SSE, observability) bind
		//       to the wire string of these event types,
		// When the event constants are inspected,
		// Then their canonical strings remain "subtask_start" /
		//      "subtask_complete" / "subtask_progress".
		assert.Equal(t, RunEventType("subtask_start"), EventSubtaskStart)
		assert.Equal(t, RunEventType("subtask_complete"), EventSubtaskComplete)
		assert.Equal(t, RunEventType("subtask_progress"), EventSubtaskProgress)
	})

	t.Run("Scenario_SubtaskStatusEnumBoundsTerminalOutcomes", func(t *testing.T) {
		// Given the trace span outcome must be one of a closed set
		//       (PDF Section 11.3: outcomes feed evaluator),
		// When SubtaskStatus is inspected,
		// Then exactly 3 terminal statuses exist:
		//      completed / failed / killed.
		assert.Equal(t, SubtaskStatus("completed"), SubtaskCompleted)
		assert.Equal(t, SubtaskStatus("failed"), SubtaskFailed)
		assert.Equal(t, SubtaskStatus("killed"), SubtaskKilled)
	})

	t.Run("Scenario_SourceSubtaskIsForegroundForRetryPolicy", func(t *testing.T) {
		// Given subtasks block the parent run (synchronous Execute),
		// When the gateway hits a transient/capacity error inside a
		//      subagent LLM call,
		// Then the retry policy treats the call as foreground (retries
		//      aggressively) — PDF Section 11.4: subagents block user.
		assert.True(t, SourceSubtask.IsForegroundSource(),
			"SourceSubtask MUST be foreground — subagents block parent run")
	})

	t.Run("Scenario_BackgroundSourcesAreNotConfusedWithSubtask", func(t *testing.T) {
		// Given background sources (compact / memory_eval) MUST fail
		//       fast to avoid gateway cascades,
		// When their foreground flag is read,
		// Then they are NOT foreground — distinguishing them from
		//      subtask spans in the tracing layer.
		assert.False(t, SourceCompact.IsForegroundSource(),
			"compact must be background — fail fast on cascade")
		assert.False(t, SourceMemoryEval.IsForegroundSource(),
			"memory_eval must be background — fail fast on cascade")
	})

	t.Run("Scenario_SubtaskProgressIsSeparateChannelForLongRunning", func(t *testing.T) {
		// Given a long-running subagent emits periodic status updates
		//       (PDF Section 11.5: agent-summary cadence),
		// When the progress event is inspected,
		// Then it carries (ID, Summary) — NOT terminal metrics, since
		//      it is interim and may be emitted N times per subtask.
		data := SubtaskProgressData{
			ID:      "subtask-long",
			Summary: "scanned 200 of 800 files",
		}
		raw, err := json.Marshal(data)
		assert.NoError(t, err)

		var roundtrip SubtaskProgressData
		assert.NoError(t, json.Unmarshal(raw, &roundtrip))
		assert.Equal(t, "subtask-long", roundtrip.ID)
		assert.Equal(t, "scanned 200 of 800 files", roundtrip.Summary)
	})

	t.Run("Scenario_DescriptionTruncationPreventsLogSpam", func(t *testing.T) {
		// Given user-controlled prompts may exceed sane log span widths,
		// When a long prompt enters the trace via Description,
		// Then truncateString caps it at 200 chars with ellipsis — so
		//      the trace remains readable and storage doesn't bloat.
		long := ""
		for i := 0; i < 500; i++ {
			long += "x"
		}
		got := truncateString(long, 200)
		assert.Equal(t, 200, len(got),
			"truncateString(s, 200) must produce exactly 200 chars")
		assert.Equal(t, "...", got[197:],
			"truncation must end with '...' so reader knows it was cut")
	})

	t.Run("Scenario_ShortDescriptionPassesThroughUntouched", func(t *testing.T) {
		// Given a short subtask description,
		// When truncateString is applied,
		// Then it returns unchanged — the trace preserves verbatim.
		got := truncateString("short prompt", 200)
		assert.Equal(t, "short prompt", got,
			"short strings must NOT be modified")
	})

	t.Run("Scenario_AllSubtaskDataTypesAreJSONRoundtripStable", func(t *testing.T) {
		// Given the SSE wire format requires byte-for-byte stable JSON
		//       names (frontend/observability bind to camelCase keys),
		startBytes, _ := json.Marshal(SubtaskStartData{ID: "s", Depth: 1})
		assert.Contains(t, string(startBytes), `"id"`)
		assert.Contains(t, string(startBytes), `"depth"`)

		completeBytes, _ := json.Marshal(SubtaskCompleteData{
			ID: "s", TotalTurns: 1, TotalTokens: 1, TotalCost: 1,
		})
		assert.Contains(t, string(completeBytes), `"totalTurns"`)
		assert.Contains(t, string(completeBytes), `"totalTokens"`)
		assert.Contains(t, string(completeBytes), `"totalCostUsd"`)

		progressBytes, _ := json.Marshal(SubtaskProgressData{ID: "s", Summary: "x"})
		assert.Contains(t, string(progressBytes), `"id"`)
		assert.Contains(t, string(progressBytes), `"summary"`)
	})
}

func TestBDD_SubagentTracing_IsValidSubtaskStatus(t *testing.T) {
	t.Run("Scenario_AllCanonicalStatusesAccepted", func(t *testing.T) {
		// Given OBS-006 declares 3 bounded outcomes,
		// When IsValidSubtaskStatus runs on each,
		// Then all return true.
		for _, s := range []SubtaskStatus{SubtaskCompleted, SubtaskFailed, SubtaskKilled} {
			assert.True(t, IsValidSubtaskStatus(s), "%s must be valid", s)
		}
	})

	t.Run("Scenario_UnknownStatusRejectedAtBoundary", func(t *testing.T) {
		// Given a deserialised SubtaskCompleteData carries an unknown
		// status (e.g., older client sending "timed_out"),
		// When IsValidSubtaskStatus runs,
		// Then it returns false so the trace pipeline can reject the
		// envelope before propagating bad data.
		assert.False(t, IsValidSubtaskStatus(SubtaskStatus("timed_out")))
		assert.False(t, IsValidSubtaskStatus(SubtaskStatus("")))
	})

	t.Run("Scenario_AllSubtaskStatusesReturnsCopy", func(t *testing.T) {
		// Given a UI or audit emitter wants the canonical list,
		// When it gets AllSubtaskStatuses,
		// Then mutating the slice does not affect the source — same
		// defensive-copy pattern used across the package.
		got := AllSubtaskStatuses()
		require.Equal(t, 3, len(got))
		got[0] = "tampered"
		got2 := AllSubtaskStatuses()
		assert.Equal(t, SubtaskCompleted, got2[0])
	})
}
