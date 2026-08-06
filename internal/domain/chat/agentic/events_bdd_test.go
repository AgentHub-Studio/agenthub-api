package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify OBS-001 (Structured event logs) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 3.2 (Figure 1) frames the runtime as an event-emitting loop.
//   - Section 11 ("Tensions, trade-offs, observability, governance") states
//     that observability of tool calls, permission decisions, hook
//     interception, compaction, and subagent dispatch is a first-class
//     concern — not an after-thought.
//   - Section 4.1 explicitly enumerates StreamEvent, RequestStartEvent,
//     Message, TombstoneMessage, and ToolUseSummaryMessage as the events the
//     queryLoop AsyncGenerator yields to the consumer.
//
// AgentHub maps the event stream to a typed `<-chan RunEvent` (the
// Go-idiomatic AsyncGenerator from LOOP-001). Every event is a `{type,
// data}` envelope where `data` is JSON-encoded — ready for SSE transport,
// audit log persistence, and metrics extraction without further marshalling.
//
// These scenarios assert the wire shape, the event-type catalogue, and the
// constructor's correctness — guarding against silent regressions that
// would shrink observability surface.

func TestBDD_StructuredEventLogs(t *testing.T) {
	t.Run("Scenario_RunEventEnvelopeIsTypeAndJSONData", func(t *testing.T) {
		// Given a typed payload for a streamed text chunk,
		payload := TextDeltaData{Content: "Hello, "}

		// When the runner constructs a RunEvent,
		when := NewRunEvent(EventTextDelta, payload)

		// Then the envelope carries the discriminator type and JSON-encoded
		//      data (PDF Section 11: every observable event must be
		//      structured for downstream tooling).
		assert.Equal(t, EventTextDelta, when.Type,
			"event type must round-trip from constructor to envelope")
		assert.NotEmpty(t, when.Data,
			"event data must be JSON-encoded, not nil")
		var decoded TextDeltaData
		assert.NoError(t, json.Unmarshal(when.Data, &decoded),
			"event data must parse back as the original typed payload")
		assert.Equal(t, "Hello, ", decoded.Content,
			"payload content must round-trip without mutation")
	})

	t.Run("Scenario_AllLoopLifecycleEventsAreEnumerated", func(t *testing.T) {
		// Given the PDF requires observability of the full agentic lifecycle
		//       (Section 4.1 + Section 11),
		// When we list AgentHub's event-type catalogue,
		// Then every PDF-described observable lifecycle phase has a typed
		//      event constant — refactor that drops one fails this guard.
		lifecycle := map[RunEventType]string{
			EventTextDelta:        "streamed assistant text",
			EventThinkingDelta:    "streamed reasoning content",
			EventToolCallStart:    "tool dispatch start (Section 4.2)",
			EventToolResult:       "tool execution result (Section 4.2)",
			EventToolProgress:     "intra-tool progress (Section 4.2)",
			EventToolDenied:       "permission deny feedback (Section 5)",
			EventToolUseSummary:   "summary of completed tool batch",
			EventTurnComplete:     "turn boundary (Section 4.1)",
			EventRunComplete:      "loop terminal (Section 4.5)",
			EventRunProgress:      "incremental run progress",
			EventError:            "error envelope (Section 4.4)",
			EventWarning:          "non-fatal advisory",
			EventContextCompacted: "compaction event (Section 4.3 + 7.3)",
			EventSubtaskStart:     "subagent spawn (Section 8)",
			EventSubtaskComplete:  "subagent terminal (Section 8)",
			EventSubtaskProgress:  "subagent progress (Section 8)",
			EventModelFallback:    "fallback model swap (Section 4.4)",
			EventStopHookSummary:  "stop hook summary (Section 4.5)",
			EventTranscription:    "speech-to-text result",
			EventAudioDelta:       "text-to-speech audio chunk",
			EventInputRequest:     "elicitation / structured user input",
			EventHeartbeat:        "keep-alive while waiting for input",
			EventCanvasUpdate:     "rich UI canvas update",
			EventAgentMessage:     "inter-agent message (Section 8.3 mailbox)",
		}
		assert.GreaterOrEqual(t, len(lifecycle), 22,
			"AgentHub must enumerate at least 22 distinct lifecycle events for full observability")
		// Each constant must be a non-empty string token.
		for ty := range lifecycle {
			assert.NotEmpty(t, string(ty),
				"event type token must be non-empty for SSE wire compat")
		}
	})

	t.Run("Scenario_PermissionDecisionsHaveTheirOwnEvent", func(t *testing.T) {
		// Given the PDF Section 11 lists permission-decision tracing as a
		//       first-class observability concern,
		// When the runner denies a tool call,
		// Then a typed event_tool_denied surfaces — not embedded in a generic
		//      error — so audit systems can filter for safety telemetry
		//      cleanly.
		assert.Equal(t, RunEventType("tool_denied"), EventToolDenied,
			"tool denial must be a first-class event for permission audit")
	})

	t.Run("Scenario_CompactionEventCarriesObservableMarker", func(t *testing.T) {
		// Given context compaction fires (PDF Section 4.3 / 7.3),
		// When the runner emits the compaction event,
		// Then EventContextCompacted is the typed marker — observability
		//      systems can correlate compaction with token-usage spikes.
		assert.Equal(t, RunEventType("context_compacted"), EventContextCompacted,
			"compaction must surface as a discrete event for tracing")
	})

	t.Run("Scenario_ToolCallStartCarriesIDForCorrelation", func(t *testing.T) {
		// Given a tool dispatch (PDF Section 4.2: streamed tool execution),
		payload := ToolCallStartData{
			ID:    "call_abc123",
			Name:  "document-search",
			Input: json.RawMessage(`{"query":"x"}`),
		}

		// When the event envelope is constructed,
		when := NewRunEvent(EventToolCallStart, payload)
		var decoded ToolCallStartData
		assert.NoError(t, json.Unmarshal(when.Data, &decoded))

		// Then the event carries an ID so the eventual ToolResultData can
		//      correlate against it — required for distributed tracing.
		assert.Equal(t, "call_abc123", decoded.ID,
			"tool call event must carry the correlation ID")
		assert.Equal(t, "document-search", decoded.Name,
			"tool name must accompany the ID for span attribution")
	})

	t.Run("Scenario_ToolResultIncludesDurationAndOptionalError", func(t *testing.T) {
		// Given a tool execution that succeeded,
		ok := ToolResultData{ID: "call_ok", Name: "document-search",
			Output: json.RawMessage(`["a","b"]`), DurationMs: 42}

		// When emitted as event,
		evtOK := NewRunEvent(EventToolResult, ok)
		var decodedOK ToolResultData
		assert.NoError(t, json.Unmarshal(evtOK.Data, &decodedOK))

		// Then duration is observable (latency telemetry) and error is
		//      omitted via omitempty (clean wire).
		assert.Equal(t, int64(42), decodedOK.DurationMs,
			"duration must surface for per-tool latency analysis")
		assert.Nil(t, decodedOK.Error,
			"successful result must omit the error field")
		assert.False(t, decodedOK.IsError,
			"successful result must expose isError=false for SSE consumers")

		// And given a tool execution that failed,
		errMsg := "deny: external network blocked"
		fail := ToolResultData{ID: "call_fail", Name: "shell",
			DurationMs: 7, Error: &errMsg}

		evtFail := NewRunEvent(EventToolResult, fail)
		var decodedFail ToolResultData
		assert.NoError(t, json.Unmarshal(evtFail.Data, &decodedFail))

		// Then the error is preserved verbatim — observable as an explicit
		//      field for audit, not merged into output.
		assert.NotNil(t, decodedFail.Error,
			"failed result must surface the error field")
		assert.Equal(t, errMsg, *decodedFail.Error,
			"error message must round-trip exactly")
		assert.True(t, decodedFail.IsError,
			"failed result must expose isError=true for SSE consumers")
	})

	t.Run("Scenario_TokenUsageIsObservablePerEvent", func(t *testing.T) {
		// Given the PDF requires cost / token-usage telemetry (Section 11),
		usage := TokenUsage{
			PromptTokens:        100,
			CompletionTokens:    50,
			TotalTokens:         150,
			CacheReadTokens:     90,
			CacheCreationTokens: 10,
			CostUSD:             0.0042,
			Model:               "claude-sonnet-4-20250514",
		}

		// When the runner attaches usage to a turn event,
		raw, err := json.Marshal(usage)
		assert.NoError(t, err)

		// Then JSON includes both raw and cache token counts plus cost in
		//      USD — necessary for post-hoc cost attribution.
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Contains(t, decoded, "promptTokens")
		assert.Contains(t, decoded, "completionTokens")
		assert.Contains(t, decoded, "totalTokens")
		assert.Contains(t, decoded, "cacheReadTokens")
		assert.Contains(t, decoded, "costUsd",
			"costUsd must be observable for billing/budget telemetry")
	})

	t.Run("Scenario_EventEnvelopeRoundTripsThroughJSON", func(t *testing.T) {
		// Given the runner needs to serialise events for SSE wire transport,
		original := NewRunEvent(EventTurnComplete, ToolUseSummaryData{
			TurnIndex: 3, Summary: "did the thing",
		})

		// When the SSE layer marshals/unmarshals,
		raw, err := json.Marshal(original)
		assert.NoError(t, err)
		var roundtripped RunEvent
		assert.NoError(t, json.Unmarshal(raw, &roundtripped))

		// Then both type and data are preserved — clients can dispatch on
		//      type and decode data independently.
		assert.Equal(t, original.Type, roundtripped.Type,
			"event type must round-trip through SSE")
		assert.JSONEq(t, string(original.Data), string(roundtripped.Data),
			"event data must round-trip through SSE")
	})

	t.Run("Scenario_HeartbeatAndWarningAreFirstClassNonFatalEvents", func(t *testing.T) {
		// Given long-running operations may need to keep SSE connections
		//       alive (PDF Section 11 implication: observability must not
		//       starve when waiting for input) and recoverable issues need
		//       a non-fatal channel,
		// When the runner emits these,
		// Then they have dedicated typed events — operators can distinguish
		//      "alive but waiting" from "error" without parsing payloads.
		assert.Equal(t, RunEventType("heartbeat"), EventHeartbeat,
			"heartbeat must be observable as its own type")
		assert.Equal(t, RunEventType("warning"), EventWarning,
			"warning must be observable as its own type")
		assert.NotEqual(t, EventWarning, EventError,
			"warning and error must be distinct event types")
	})
}
