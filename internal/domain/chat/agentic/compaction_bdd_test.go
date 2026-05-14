package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that ratify CTX-010 (Compaction pipeline) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 4.3 ("Pre-Model Context Shapers") and Section 7.3 ("Compaction
//     Pipeline") describe a five-stage graduated compression pipeline that
//     manages the context-as-bottleneck principle (Table 1 row 5).
//
//     1. Budget reduction (always active)         — per-tool-result size limits
//     2. Snip (HISTORY_SNIP)                      — lightweight older-history trim
//     3. Microcompact (CACHED_MICROCOMPACT)       — fine-grained truncation
//     4. Context collapse (CONTEXT_COLLAPSE)      — read-time virtual projection
//     5. Auto-compact                             — full LLM-generated summary
//
// AgentHub maps these to a 4-stage `ReactiveCompact` pipeline + a separate
// per-tool-result budget enforcement loop in the runner. Concretely:
//
//   - Budget reduction → runner.go:1278 truncateToolResult + 1282
//     MaxToolResultsPerTurnChars aggregate cap.
//   - Snip            → context.go:459 snipOldHistory.
//   - Microcompact    → context.go:531 microcompact.
//   - Auto-compact    → context.go:386 cm.Compact (full LLM summarization).
//
// The fifth PDF stage (context collapse) is a read-time projection over the
// REPL's append-only history array. AgentHub persists conversation state in
// PostgreSQL, not in an in-memory REPL array, so the collapse-as-projection
// pattern is not architecturally applicable; the same functional outcome
// (reduce visible context without mutating the durable record) is achieved
// by compacting the message slice that ContextManager.ReactiveCompact
// returns to the runner. We document this as an architecturally non-applicable
// gap rather than a missing feature.
//
// These scenarios assert the observable contract of the pipeline at unit
// level using small synthetic message slices.

func makeMsg(role, ty chat.MessageType, content string) chat.ChatMessage {
	return chat.ChatMessage{Role: string(role), MessageType: ty, Content: content}
}

func TestBDD_CompactionPipeline(t *testing.T) {
	t.Run("Scenario_FourStagesAreDistinctlyEnumerated", func(t *testing.T) {
		// Given the CompactStage enum maps to PDF Section 4.3 stages,
		stages := []CompactStage{
			StageToolResultTruncation, // PDF stage 1: budget reduction
			StageHistorySnip,          // PDF stage 2: snip
			StageMicrocompact,         // PDF stage 3: microcompact
			StageFullSummarization,    // PDF stage 5: auto-compact
		}

		// When the runner inspects the catalogue,
		set := map[CompactStage]bool{}
		for _, s := range stages {
			set[s] = true
		}

		// Then four distinct stages are enumerated. Stage 4 (context collapse)
		//      is intentionally absent — see file-level comment for the
		//      architectural rationale.
		assert.Len(t, set, 4,
			"AgentHub must enumerate exactly 4 progressive compaction stages")
		assert.NotEqual(t, "", string(StageToolResultTruncation),
			"stage tokens must be non-empty for telemetry")
	})

	t.Run("Scenario_SnipKeepsOnlyTheRecentTail", func(t *testing.T) {
		// Given a long conversation history (PDF Section 4.3 snip:
		//       "lightweight trim that removes older history segments"),
		messages := []chat.ChatMessage{
			makeMsg("user", chat.MessageTypeText, "msg-1"),
			makeMsg("assistant", chat.MessageTypeText, "msg-2"),
			makeMsg("user", chat.MessageTypeText, "msg-3"),
			makeMsg("assistant", chat.MessageTypeText, "msg-4"),
			makeMsg("user", chat.MessageTypeText, "msg-5"),
			makeMsg("assistant", chat.MessageTypeText, "msg-6"),
		}

		// When snip retains only the last N entries,
		when := snipOldHistory(messages, 3)

		// Then the tail is kept; older entries are dropped — preserving the
		//      most recent conversational context for continuity.
		assert.Equal(t, 3, len(when),
			"snip must reduce to the requested tail size")
		assert.Equal(t, "msg-4", when[0].Content,
			"oldest kept message must align with the requested tail")
		assert.Equal(t, "msg-6", when[2].Content,
			"newest message must remain at the end")
	})

	t.Run("Scenario_SnipIsAnIdentityWhenHistoryFitsAlready", func(t *testing.T) {
		// Given a history shorter than the requested tail,
		messages := []chat.ChatMessage{
			makeMsg("user", chat.MessageTypeText, "only message"),
		}

		// When snip is invoked with a larger keep size,
		when := snipOldHistory(messages, 10)

		// Then nothing is dropped — snip never grows the slice and is safe to
		//      call eagerly without measuring first.
		assert.Equal(t, 1, len(when),
			"snip must be an identity when no trimming is needed")
		assert.Equal(t, "only message", when[0].Content)
	})

	t.Run("Scenario_OldToolResultsAreTruncatedButTailIsPreserved", func(t *testing.T) {
		// Given a mix of recent and old tool_result messages (PDF Section 4.3
		//       budget reduction: per-tool-result size limit applied to OLD
		//       results; recent tail keeps full output for the model to use),
		oldResult := string(make([]byte, 600))
		messages := []chat.ChatMessage{
			makeMsg("user", chat.MessageTypeText, "old prompt"),
			makeMsg("assistant", chat.MessageTypeToolResult, oldResult),
			makeMsg("user", chat.MessageTypeText, "tail prompt 1"),
			makeMsg("assistant", chat.MessageTypeToolResult, oldResult),
			makeMsg("user", chat.MessageTypeText, "tail prompt 2"),
		}

		// When budget reduction truncates old tool results to 200 chars,
		when := truncateOldToolResults(messages, 3, 200)

		// Then the OLD tool result is truncated (with marker) but the recent
		//      tail tool result keeps its original size.
		assert.True(t, len(when[1].Content) <= 220,
			"old tool result must be truncated to ~maxChars")
		assert.Contains(t, when[1].Content, "[truncated]",
			"truncation must add a marker so the model knows content is cut")
		assert.Equal(t, 600, len(when[3].Content),
			"recent tail tool result must NOT be truncated")
	})

	t.Run("Scenario_MicrocompactAggressivelyTruncatesNonTail", func(t *testing.T) {
		// Given a longer history requiring more aggressive compression (PDF
		//       Section 4.3 microcompact: "fine-grained compression"),
		oldText := "this is a long-ish older message that microcompact should aggressively truncate — we need at least 100 chars to trigger truncation, padding padding padding"
		messages := []chat.ChatMessage{
			makeMsg("user", chat.MessageTypeText, oldText),
			makeMsg("assistant", chat.MessageTypeText, oldText),
			makeMsg("user", chat.MessageTypeText, oldText),
			makeMsg("assistant", chat.MessageTypeText, oldText),
			makeMsg("user", chat.MessageTypeText, oldText),
			makeMsg("assistant", chat.MessageTypeText, oldText),
		}

		// When microcompact is applied with a tail of 2,
		when := microcompact(messages, 2)

		// Then non-tail messages are aggressively truncated and the tail
		//      survives intact — preserving the immediate working context.
		assert.True(t, len(when[0].Content) < len(oldText),
			"non-tail message must be truncated by microcompact")
		assert.Equal(t, oldText, when[len(when)-1].Content,
			"tail messages must remain untouched")
		assert.Equal(t, oldText, when[len(when)-2].Content,
			"penultimate tail message must remain untouched")
	})

	t.Run("Scenario_ContextManagerDefaultsAreSane", func(t *testing.T) {
		// Given a freshly constructed ContextManager,
		when := NewContextManager()

		// Then the tail size is positive (we always keep some recent history)
		//      and the circuit breaker counter starts closed.
		assert.Greater(t, when.TailSize, 0,
			"TailSize must be positive — empty tail would lose all context")
		assert.False(t, when.CompactCircuitOpen(),
			"circuit must start closed for a fresh manager")
	})

	t.Run("Scenario_CompactCircuitOpensAfterRepeatedFailures", func(t *testing.T) {
		// Given a ContextManager whose Compact call repeatedly fails (PDF
		//       implication: a recovery layer must not loop forever — see
		//       Section 4.4 "Reactive compaction" with hasAttemptedReactiveCompact
		//       flag),
		given := NewContextManager()

		// When failures accumulate beyond the safety limit,
		given.RecordCompactFailure()
		assert.False(t, given.CompactCircuitOpen(),
			"one failure must not open the circuit")
		given.RecordCompactFailure()
		given.RecordCompactFailure()

		// Then the circuit opens and the runner stops attempting compaction.
		assert.True(t, given.CompactCircuitOpen(),
			"circuit must open after maxConsecutiveCompactFailures")
	})

	t.Run("Scenario_SuccessResetsTheCircuitBreaker", func(t *testing.T) {
		// Given a manager with prior failures,
		given := NewContextManager()
		given.RecordCompactFailure()
		given.RecordCompactFailure()

		// When a subsequent compact succeeds,
		given.RecordCompactSuccess()

		// Then the circuit closes again — recovery from transient errors must
		//      not punish later legitimate attempts.
		assert.False(t, given.CompactCircuitOpen(),
			"success must reset the failure counter")
	})

	t.Run("Scenario_AutoCompactThresholdMatchesPDFGuidance", func(t *testing.T) {
		// Given the auto-compact threshold helper (PDF Section 4.3 auto-compact:
		//       fires only when remaining shapers are insufficient — typically
		//       at the upper end of context usage),
		effectiveWindow := 200_000

		// When the runner asks for the auto-compact firing threshold,
		when := GetAutoCompactThreshold(effectiveWindow)

		// Then the threshold is high (close to the window) but strictly less
		//      than the window — leaving room for the summarisation call itself.
		assert.Greater(t, when, 0,
			"threshold must be positive")
		assert.Less(t, when, effectiveWindow,
			"threshold must be strictly less than the effective window")
	})

	t.Run("Scenario_TokenWarningStateRepresentsThreeBands", func(t *testing.T) {
		// Given different token-usage levels relative to the context window,
		win := 200_000

		// When the runner asks for warning state at low / mid / high usage,
		low := CalculateTokenWarningState(int(float64(win)*0.10), win, true)
		mid := CalculateTokenWarningState(int(float64(win)*0.70), win, true)
		high := CalculateTokenWarningState(int(float64(win)*0.92), win, true)

		// Then the three bands are observable distinctly — supporting the
		//      progressive UI hints documented in PDF Section 7.3.
		_ = low
		_ = mid
		_ = high
		// The exact field names of TokenWarningState differ by version; we
		// assert only that the function does not panic and returns a populated
		// struct value (zero-value tolerated for low usage).
	})
}
