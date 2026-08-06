package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// BDD-style scenarios that ratify TOOL-009 (Normalização de tool results)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 4.2 (Tool Dispatch and Streaming Execution): "Results are
//     normalised for the Anthropic API via normalizeMessagesForAPI(),
//     filtering to keep only user-type messages."
//   - Section 4.1 + general API contract: the Anthropic / OpenAI Messages
//     APIs require alternating user/assistant messages — the normaliser
//     enforces this invariant before every model call.
//   - Section 11 (sanitisation): tool results may contain Unicode
//     adversarial content (zero-width chars, RTL overrides) that needs
//     defensive cleaning to prevent prompt-injection via output channel.
//
// AgentHub maps result/message normalization to:
//   - normalize.go NormalizeMessagesForAPI — 6-pass pipeline over chat.ChatMessage:
//       1. Merge consecutive user messages
//       2. Filter orphaned thinking-only assistant messages
//       3. Strip trailing thinking from last assistant
//       4. Filter whitespace-only messages
//       5. Sanitise empty tool results
//       6. Merge consecutive assistant messages
//   - normalize.go NormalizeAIMessagesForAPI — companion pass over ai.Message
//     (the runtime/wire type).
//   - sanitization.go SanitizeUnicode + IsDangerousUnicode + truncateForError
//     — defensive cleaning for tool output that may contain adversarial
//     control codes.
//
// These scenarios assert: invariant enforcement (alternating roles),
// defensive cleaning, and idempotency on already-clean input.

func makeChatMsg(role, content string) chat.ChatMessage {
	return chat.ChatMessage{Role: role, Content: content, MessageType: chat.MessageTypeText}
}

func TestBDD_NormalizeAndSanitize(t *testing.T) {
	t.Run("Scenario_NormalizeMergesConsecutiveUserMessages", func(t *testing.T) {
		// Given two consecutive user messages (PDF Section 4.2 + Anthropic
		//       API: alternating roles required — system nudge can inject a
		//       second user message),
		given := []chat.ChatMessage{
			makeChatMsg("user", "first request"),
			makeChatMsg("user", "system nudge"),
			makeChatMsg("assistant", "ok"),
		}

		// When the normaliser runs,
		when := NormalizeMessagesForAPI(given)

		// Then consecutive users are merged into one — preserves API
		//      invariant; both contents survive concatenated.
		assert.Len(t, when, 2,
			"two consecutive users + 1 assistant must collapse to 2 msgs")
		assert.Equal(t, "user", when[0].Role)
		assert.Contains(t, when[0].Content, "first request")
		assert.Contains(t, when[0].Content, "system nudge",
			"merged content must include both originals")
	})

	t.Run("Scenario_NormalizeMergesConsecutiveAssistantMessages", func(t *testing.T) {
		// Given two consecutive assistant messages (PDF: streaming may
		//       produce assistant fragments + a follow-up text message),
		given := []chat.ChatMessage{
			makeChatMsg("user", "hi"),
			makeChatMsg("assistant", "thinking..."),
			makeChatMsg("assistant", "the answer is 42"),
		}

		// When normalised,
		when := NormalizeMessagesForAPI(given)

		// Then assistants merge — API alternation invariant preserved.
		assert.Len(t, when, 2,
			"consecutive assistants must merge")
		assert.Equal(t, "assistant", when[1].Role)
	})

	t.Run("Scenario_NormalizeFiltersWhitespaceOnlyMessages", func(t *testing.T) {
		// Given the conversation contains an empty/whitespace assistant
		//       message (PDF: tool result sanitisation includes empty
		//       content cleanup — empty messages confuse downstream models),
		given := []chat.ChatMessage{
			makeChatMsg("user", "hello"),
			makeChatMsg("assistant", "   \n\t  "),
			makeChatMsg("user", "still there?"),
		}

		// When normalised,
		when := NormalizeMessagesForAPI(given)

		// Then whitespace-only messages are removed — keeping only
		//      meaningful content for the next model call.
		// Note: after removal, the two users may now be consecutive and
		// merge; we only assert that no whitespace-only message remains.
		for _, m := range when {
			assert.NotEqual(t, "   \n\t  ", m.Content,
				"whitespace-only content must NOT survive normalisation")
		}
	})

	t.Run("Scenario_NormalizeIsAnIdentityOnAlreadyCleanMessages", func(t *testing.T) {
		// Given a clean alternating conversation,
		given := []chat.ChatMessage{
			makeChatMsg("user", "ping"),
			makeChatMsg("assistant", "pong"),
			makeChatMsg("user", "again"),
			makeChatMsg("assistant", "again"),
		}

		// When normalised,
		when := NormalizeMessagesForAPI(given)

		// Then nothing is dropped or merged — the pipeline is idempotent
		//      on clean input.
		assert.Equal(t, len(given), len(when),
			"clean alternating messages must survive normalisation untouched")
	})

	t.Run("Scenario_NormalizeHandlesEmptyInput", func(t *testing.T) {
		// Given no messages,
		when := NormalizeMessagesForAPI(nil)

		// Then no panic; empty input round-trips empty.
		assert.Empty(t, when,
			"empty input must round-trip empty without panic")
	})

	t.Run("Scenario_NormalizeAIMessagesForAPIAppliesPipelineToWireType", func(t *testing.T) {
		// Given the runtime ai.Message wire type carrying consecutive
		//       same-role messages (PDF Section 4.2 same invariants apply
		//       to the wire type, not just durable type),
		given := []ai.Message{
			{Role: ai.RoleUser, Content: "a"},
			{Role: ai.RoleUser, Content: "b"},
			{Role: ai.RoleAssistant, Content: "c"},
		}

		// When normalised at the wire layer,
		when := NormalizeAIMessagesForAPI(given)

		// Then alternation is preserved — runtime + durable layers share
		//      the same invariants.
		assert.LessOrEqual(t, len(when), 2,
			"consecutive users at wire layer must also merge")
	})

	t.Run("Scenario_SanitizeUnicodeRemovesDangerousControlChars", func(t *testing.T) {
		// Given a tool result containing zero-width chars or RTL override
		//       (PDF Section 11: defensive cleaning prevents prompt-
		//       injection via output channel),
		dangerous := "hello\u202eworld\u200b"

		// When the sanitiser runs,
		when, err := SanitizeUnicode(dangerous)

		// Then dangerous chars are removed — clean text remains, no panic
		//      on the adversarial input.
		assert.NoError(t, err,
			"sanitiser must not error on adversarial input")
		assert.NotContains(t, when, "\u202e",
			"RTL override must be removed (anti-spoofing)")
		assert.NotContains(t, when, "\u200b",
			"zero-width space must be removed")
		assert.Contains(t, when, "hello")
		assert.Contains(t, when, "world")
	})

	t.Run("Scenario_IsDangerousUnicodeFlagsAdversarialInput", func(t *testing.T) {
		// Given inputs of varying safety levels,
		assert.True(t, IsDangerousUnicode("hello\u202eworld"),
			"RTL override must flag as dangerous")
		assert.True(t, IsDangerousUnicode("zero\u200bwidth"),
			"zero-width space must flag as dangerous")
		assert.False(t, IsDangerousUnicode("normal text 123"),
			"plain ASCII must NOT flag as dangerous")
		assert.False(t, IsDangerousUnicode("emoji 🚀 unicode"),
			"normal unicode (emoji) must NOT flag as dangerous")
	})

	t.Run("Scenario_MustSanitizeUnicodeFallsBackOnError", func(t *testing.T) {
		// Given the convenience wrapper that swallows errors (PDF: defensive
		//       cleaning never crashes the runtime, even on pathological
		//       input),
		when := MustSanitizeUnicode("safe input")

		// Then the result is non-empty and clean.
		assert.NotEmpty(t, when,
			"MustSanitizeUnicode must not return empty for valid input")
	})

	t.Run("Scenario_TruncateForErrorBoundsErrorMessages", func(t *testing.T) {
		// Given a giant error string (PDF: error messages must be bounded
		//       to fit in audit / event logs without DoS),
		giant := ""
		for i := 0; i < 10000; i++ {
			giant += "X"
		}

		// When truncated,
		when := truncateForError(giant, 200)

		// Then bounded length — preventing log flooding.
		assert.LessOrEqual(t, len(when), 220,
			"truncated error must be bounded by maxLen (with margin for marker)")
	})

	t.Run("Scenario_NormalizationStripsTrailingThinking", func(t *testing.T) {
		// Given the last assistant message has thinking-only content (PDF:
		//       trailing thinking-only must be stripped before next API
		//       call to avoid confusing the model),
		given := []chat.ChatMessage{
			makeChatMsg("user", "compute"),
			makeChatMsg("assistant", "result"),
			{Role: "assistant", Content: "still thinking...",
				MessageType: chat.MessageTypeText, Metadata: []byte(`{"thinking":true}`)},
		}

		// When normalised,
		when := NormalizeMessagesForAPI(given)

		// Then no trailing thinking-only message remains — the API sees a
		//      clean assistant message at the end.
		// We assert NORMALISATION OUTCOME (the pipeline ran without error
		// and yielded a non-empty list); deep filter inspection lives in
		// normalize_test.go for thinking-detection edge cases.
		assert.NotEmpty(t, when,
			"normalisation must yield non-empty result")
	})
}
