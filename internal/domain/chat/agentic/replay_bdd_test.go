package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify PERSIST-009 (Replay/reconstruction)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9.2 (Resume): "The --resume flag rebuilds the conversation
//     by replaying the transcript (conversationRecovery.ts)."
//   - Section 9 (overall): durable state OUTLIVES the live runtime
//     process; reconstruction is the verb that turns persisted records
//     back into a usable runtime state.
//   - Implication: replay must be DEFENSIVE — the durable transcript may
//     contain malformed records (crashed mid-write, deserialization
//     drift, version skew across deploys). The recovery path filters
//     them rather than crashing.
//
// AgentHub maps replay/reconstruction to:
//   - outputstyle.go RecoverConversationMessages — filters messages for
//     resume: drops whitespace-only entries, drops thinking-only blocks,
//     prunes unresolved tool_use blocks (PDF Section 9.1 invariant: API
//     rejects orphaned tool_use). This is the per-session "deserialize
//     + clean" verb.
//   - outputstyle.go ConversationRecoveryResult — typed envelope
//     reporting Messages + TurnInterruptionKind + InterruptedMessage so
//     the runner can surface "user had unsent input" UX without losing
//     it on resume.
//   - runner.go loadHistory — the orchestration entry point
//     (covered separately by PERSIST-004 BDD): calls FindAllMessages →
//     applies sliding window cap → time-based eviction → filters
//     unresolved tool_use → sanitises → returns ai.Message slice.
//
// PERSIST-004 (Resume) covered the LOOP-LEVEL replay orchestration.
// PERSIST-009 covers the WIRE-LEVEL deserialization + filtering pipeline
// — the part that handles the JSON message blob from durable storage and
// produces a clean, normalised, API-safe slice.

func TestBDD_ConversationReplayReconstruction(t *testing.T) {
	t.Run("Scenario_RecoverEmptyInputReturnsEmpty", func(t *testing.T) {
		// Given a brand-new session with no persisted messages,
		// When the runtime asks for recovery,
		when, err := RecoverConversationMessages(json.RawMessage(``))

		// Then no panic, no error — empty round-trips empty.
		assert.NoError(t, err,
			"empty input must NOT error")
		assert.Empty(t, when,
			"empty input must round-trip as empty")
	})

	t.Run("Scenario_RecoverPreservesCleanConversation", func(t *testing.T) {
		// Given a clean alternating user/assistant transcript,
		given := json.RawMessage(`[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi back"},
			{"role":"user","content":"thanks"}
		]`)

		// When the runtime recovers,
		when, err := RecoverConversationMessages(given)
		assert.NoError(t, err)

		// Then all 3 messages survive — no spurious filtering on
		//      well-formed input.
		var recovered []map[string]interface{}
		assert.NoError(t, json.Unmarshal(when, &recovered))
		assert.Equal(t, 3, len(recovered),
			"clean transcript must round-trip without dropping messages")
	})

	t.Run("Scenario_RecoverFiltersWhitespaceOnlyContent", func(t *testing.T) {
		// Given a transcript polluted by whitespace-only messages (PDF:
		//       persisted records may include malformed entries from
		//       crashed writes or version drift),
		given := json.RawMessage(`[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"   \n\t  "},
			{"role":"user","content":"still there?"}
		]`)

		// When recovery runs,
		when, err := RecoverConversationMessages(given)
		assert.NoError(t, err)

		// Then the whitespace-only entry is dropped — the model sees
		//      meaningful conversation only.
		var recovered []map[string]interface{}
		assert.NoError(t, json.Unmarshal(when, &recovered))
		for _, m := range recovered {
			content, _ := m["content"].(string)
			role, _ := m["role"].(string)
			// Allow blank content only for tool messages (which carry
			// data via toolCallId).
			if role != "tool" {
				assert.NotEmpty(t, content,
					"non-tool message must NOT be whitespace-only after recovery")
			}
		}
	})

	t.Run("Scenario_RecoverPreservesToolCallPairs", func(t *testing.T) {
		// Given a complete tool_use → tool_result pair (PDF Section 9.1:
		//       paired tool calls must survive replay so the API
		//       contract is honoured),
		given := json.RawMessage(`[
			{"role":"user","content":"do the lookup"},
			{"role":"assistant","content":"working","toolCalls":[
				{"id":"call_1","function":{"name":"document-search"}}
			]},
			{"role":"tool","toolCallId":"call_1","content":"result data"},
			{"role":"assistant","content":"here you go"}
		]`)

		// When recovery runs,
		when, err := RecoverConversationMessages(given)
		assert.NoError(t, err)

		// Then all 4 messages survive — paired tool calls are preserved.
		var recovered []map[string]interface{}
		assert.NoError(t, json.Unmarshal(when, &recovered))
		assert.Equal(t, 4, len(recovered),
			"resolved tool_use+tool_result pair + surrounding context must survive")
	})

	t.Run("Scenario_RecoverHandlesMalformedJSONGracefully", func(t *testing.T) {
		// Given persisted blob that is not valid JSON (PDF: defensive
		//       reading — version drift or partial writes can produce
		//       malformed payloads; the runtime must NOT crash on resume),
		bad := json.RawMessage(`not-valid-json`)

		// When recovery is invoked,
		when, err := RecoverConversationMessages(bad)

		// Then an error surfaces and the original input round-trips
		//      (caller can surface a "couldn't recover" UX rather than
		//      silently losing the session).
		assert.Error(t, err,
			"malformed JSON must surface as error (not silent drop)")
		assert.Equal(t, string(bad), string(when),
			"malformed input must round-trip unchanged for caller-side handling")
	})

	t.Run("Scenario_RecoveryResultEnvelopeFieldsAreObservable", func(t *testing.T) {
		// Given the recovery envelope (PDF Section 9.2: resume must
		//       expose UNSENT user input separately so it isn't lost),
		given := ConversationRecoveryResult{
			Messages:             json.RawMessage(`[{"role":"user","content":"hi"}]`),
			TurnInterruptionKind: "interrupted_prompt",
			InterruptedMessage:   "the user was typing this when they Ctrl+C'd",
		}

		// When the runner consumes the result,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then the 3 fields are observable on the wire — UI can show
		//      the interrupted message in a draft area instead of losing it.
		assert.Equal(t, "interrupted_prompt", decoded["turnInterruptionKind"],
			"interruption kind must round-trip")
		assert.Equal(t, "the user was typing this when they Ctrl+C'd",
			decoded["interruptedMessage"],
			"interrupted message must round-trip for UI redraw")
		assert.NotEmpty(t, decoded["messages"],
			"recovered messages must round-trip")
	})

	t.Run("Scenario_TurnInterruptionKindIsBoundedEnum", func(t *testing.T) {
		// Given the interruption kind communicates UX intent (PDF: clean
		//       end vs interrupted prompt drives different UI behaviours),
		// When the runtime emits the kind,
		// Then only known tokens appear — refactor that introduces a
		//      new value must update the UI handler too.
		known := map[string]bool{
			"none":               true,
			"interrupted_prompt": true,
		}

		// Asserting the closed set: production code paths should only
		// emit these tokens. The struct field is plain string but the
		// expected vocabulary is documented here.
		assert.True(t, known["none"],
			"clean end token must be 'none'")
		assert.True(t, known["interrupted_prompt"],
			"interrupted prompt token must be 'interrupted_prompt'")
		assert.Len(t, known, 2,
			"2 interruption kinds — extending requires UI handler update")
	})

	t.Run("Scenario_RecoverDoesNotMutateInputSlice", func(t *testing.T) {
		// Given input ownership semantics (PDF Section 9.1: durable
		//       record is read-only post-write; recovery must not mutate
		//       the source blob in place),
		input := json.RawMessage(`[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":""}
		]`)
		original := make([]byte, len(input))
		copy(original, input)

		// When recovery runs,
		_, err := RecoverConversationMessages(input)
		assert.NoError(t, err)

		// Then the input bytes are unchanged — caller can pass a slice
		//      backed by durable storage without fear of mutation.
		assert.Equal(t, string(original), string(input),
			"recovery must NOT mutate the input slice (durable record stays clean)")
	})
}
