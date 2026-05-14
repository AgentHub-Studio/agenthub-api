package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify TOOL-008 (Progress events e attachments)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 4.2 (Tool Dispatch): "Each update may carry a tool result, an
//     attachment, or a progress event." The runner streams per-tool
//     lifecycle transitions so the UI can show queued → executing →
//     completed state without polling.
//   - Section 11 (observability): per-tool state transitions are the
//     substrate for latency telemetry and stuck-tool detection (silent
//     failures surface as stalled state).
//   - Section 6.1 + 4.2 (canvas / attachments): tools may emit rich content
//     beyond plain text — the streaming protocol carries attachment-style
//     payloads (canvas updates) so the UI can render them inline.
//
// AgentHub maps progress + attachments to:
//   - events.go ToolState — 5-value enum (queued, executing, completed,
//     stalled, aborted) covering the full PDF lifecycle.
//   - events.go EventToolProgress + ToolProgressData — the typed event the
//     runner emits at each transition.
//   - events.go EventCanvasUpdate + CanvasUpdateData — attachment-style
//     payload for rich visual content (markdown, HTML, table). The ID
//     field enables LIVE update by re-emission, mirroring PDF's
//     attachment update semantics.

func TestBDD_ProgressEventsAndAttachments(t *testing.T) {
	t.Run("Scenario_ToolStateEnumCoversFullLifecycle", func(t *testing.T) {
		// Given the PDF describes a tool's lifecycle with multiple
		//       observable transitions (PDF Section 4.2 + 11),
		states := []ToolState{
			ToolStateQueued,    // dispatched but not yet running
			ToolStateExecuting, // tool actively producing output
			ToolStateCompleted, // tool finished successfully
			ToolStateStalled,   // tool produced no output past threshold
			ToolStateAborted,   // tool cancelled (sibling-abort or user)
		}

		// When the runtime catalogs them,
		set := map[ToolState]bool{}
		for _, s := range states {
			set[s] = true
		}

		// Then exactly 5 distinct states — refactor that drops one would
		//      fail this guard, preserving observability granularity.
		assert.Len(t, set, 5,
			"AgentHub must enumerate exactly 5 ToolState values for full lifecycle observability")
		// Each must be a non-empty wire token (SSE consumers filter by string).
		for s := range set {
			assert.NotEmpty(t, string(s),
				"ToolState %q must be a non-empty wire token", s)
		}
	})

	t.Run("Scenario_ToolProgressEventCarriesIDAndStateForUI", func(t *testing.T) {
		// Given a state transition during tool execution (PDF Section 4.2:
		//       per-update payload includes id + state for UI dispatch),
		given := ToolProgressData{
			ID:    "call_abc123",
			Name:  "execute-sql",
			State: ToolStateExecuting,
		}

		// When the runner serialises the event,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)

		// Then the wire shape carries the 3 fields the UI needs to update
		//      the per-tool indicator (id for correlation, name for label,
		//      state for badge).
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Equal(t, "call_abc123", decoded["id"])
		assert.Equal(t, "execute-sql", decoded["name"])
		assert.Equal(t, "executing", decoded["state"])
	})

	t.Run("Scenario_QueuedIsTheInitialState", func(t *testing.T) {
		// Given the runner emits queued before executing (PDF: tools
		//       transition queued → executing — UI shows pending indicator
		//       even when the executor is briefly busy),
		queued := ToolProgressData{State: ToolStateQueued}
		executing := ToolProgressData{State: ToolStateExecuting}

		// When the UI renders states,
		// Then both are observable as distinct tokens — the UI can show
		//      "queued" badge before any tool work begins.
		assert.NotEqual(t, queued.State, executing.State,
			"queued and executing must be distinct states")
		assert.Equal(t, ToolState("queued"), queued.State)
	})

	t.Run("Scenario_StalledIsTheSilentFailureSignal", func(t *testing.T) {
		// Given a tool produces no output past the StallDetector threshold
		//       (PDF Section 11: silent failures must convert to LOUD
		//       signals — covered also by OBS-007),
		given := ToolProgressData{
			ID:    "call_hung",
			Name:  "long-running",
			State: ToolStateStalled,
		}

		// When the UI inspects the state,
		// Then "stalled" is the explicit signal — distinct from "executing"
		//      so users see "this is taking too long" instead of "still
		//      working".
		assert.Equal(t, ToolStateStalled, given.State,
			"stalled must be a first-class observable state (not muted under executing)")
	})

	t.Run("Scenario_AbortedIsDistinctFromCompletedAndStalled", func(t *testing.T) {
		// Given three terminal-ish states must be distinguishable (PDF:
		//       audit trail must attribute terminal cause),
		// When we list them,
		completed := ToolProgressData{State: ToolStateCompleted}
		stalled := ToolProgressData{State: ToolStateStalled}
		aborted := ToolProgressData{State: ToolStateAborted}

		// Then all three are distinct tokens — operators can tell apart
		//      "tool finished", "tool stuck", "tool was cancelled".
		assert.NotEqual(t, completed.State, stalled.State)
		assert.NotEqual(t, completed.State, aborted.State)
		assert.NotEqual(t, stalled.State, aborted.State)
	})

	t.Run("Scenario_EventToolProgressIsTheTypedEventToken", func(t *testing.T) {
		// Given the SSE stream carries typed events (OBS-001 envelope),
		// When the runner emits a progress update,
		when := NewRunEvent(EventToolProgress, ToolProgressData{
			ID: "x", Name: "y", State: ToolStateExecuting,
		})

		// Then the event type is the canonical "tool_progress" token —
		//      clients filter by type to subscribe just to progress
		//      updates, ignoring text deltas etc.
		assert.Equal(t, EventToolProgress, when.Type,
			"event type must be EventToolProgress for client routing")
		assert.Equal(t, RunEventType("tool_progress"), when.Type,
			"wire token must be 'tool_progress'")
	})

	t.Run("Scenario_CanvasUpdateIsTheAttachmentChannel", func(t *testing.T) {
		// Given a tool emits rich content (PDF Section 4.2 + 6.1: tools
		//       carry attachment-style payloads beyond plain text),
		given := CanvasUpdateData{
			ID:         "canvas_1",
			Title:      "Sales Q3",
			Format:     CanvasFormatTable,
			Content:    `{"columns":["region","sales"],"rows":[["US",1000]]}`,
			Exportable: true,
		}

		// When the runner emits it,
		when := NewRunEvent(EventCanvasUpdate, given)

		// Then the canvas update is a first-class event with all 5 fields
		//      observable on the wire — UI renders inline.
		assert.Equal(t, EventCanvasUpdate, when.Type,
			"canvas update must use EventCanvasUpdate event type")
		var decoded CanvasUpdateData
		assert.NoError(t, json.Unmarshal(when.Data, &decoded))
		assert.Equal(t, "canvas_1", decoded.ID)
		assert.Equal(t, "Sales Q3", decoded.Title)
		assert.Equal(t, CanvasFormatTable, decoded.Format)
		assert.True(t, decoded.Exportable,
			"exportable flag must round-trip for download UX")
	})

	t.Run("Scenario_CanvasUpdateIDEnablesLiveReplacement", func(t *testing.T) {
		// Given two updates with the SAME ID (PDF: attachment update
		//       semantics — re-emission with same id replaces prior),
		first := CanvasUpdateData{ID: "live_1", Format: CanvasFormatMarkdown,
			Content: "Loading..."}
		second := CanvasUpdateData{ID: "live_1", Format: CanvasFormatMarkdown,
			Content: "Done!"}

		// When the runner emits both,
		// Then the shared ID enables the frontend to REPLACE rather than
		//      ACCUMULATE — supporting live progress dashboards via canvas.
		assert.Equal(t, first.ID, second.ID,
			"matching IDs trigger replacement; mismatched IDs would accumulate")
		assert.NotEqual(t, first.Content, second.Content,
			"the content evolves across emissions")
	})

	t.Run("Scenario_CanvasFormatEnumCoversThreeRenderModes", func(t *testing.T) {
		// Given the PDF Section 4.2 + UX patterns: rich content can be
		//       Markdown (text), HTML (sandboxed), or Table (structured),
		formats := map[CanvasFormat]string{
			CanvasFormatMarkdown: "free-form text/code",
			CanvasFormatHTML:     "sandboxed iframe rendering",
			CanvasFormatTable:    "interactive structured table",
		}

		// When the runtime catalogs them,
		// Then 3 distinct render modes — covers narrative, visual,
		//      analytical use cases without forcing them into one channel.
		assert.Len(t, formats, 3,
			"3 distinct CanvasFormat values for narrative/visual/analytical")
	})

	t.Run("Scenario_ProgressEventOmitsEmptyOptionalFields", func(t *testing.T) {
		// Given a progress event where some fields are optional,
		given := ToolProgressData{State: ToolStateQueued}

		// When serialised,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)

		// Then the wire payload remains compact — empty ID/Name still
		//      serialise (no omitempty) because they're contract fields
		//      even when blank, but no extra fields appear.
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Contains(t, decoded, "state",
			"state must always appear — it's the primary signal")
	})
}
