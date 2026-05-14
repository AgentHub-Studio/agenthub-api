package agentic

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify PERSIST-003 (Prompt history) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9.1 ("Transcript Model"): "Global prompt history: User
//     prompts only, stored in history.jsonl at the Claude configuration
//     home directory (history.ts). The makeHistoryReader() generator yields
//     entries in reverse order via readLinesReverse(), supporting Up-arrow
//     and ctrl+r navigation."
//   - Section 9 (overall): three persistence channels operate independently
//     (transcripts, GLOBAL PROMPT HISTORY, subagent sidechains).
//
// AgentHub maps the prompt history pattern to:
//   - historylog.go HistoryLog — in-memory append-only log with background
//     flush + retry (5 attempts).
//   - HistoryEntry — Display + Timestamp + SessionID + Project + PastedContents.
//   - GetHistory(sessionID) — newest-first iteration (the PDF reverse-reader
//     equivalent), session-scoped entries float to the top, dedup by Display.
//   - HistoryInlineThreshold (1024 bytes) — large pastes auto-converted to
//     hash references, mirroring PDF's external-storage offload.
//   - HistoryFlusher / PasteResolver — pluggable persistence + resolution.
//
// These scenarios assert: append-only, reverse iteration, dedup, paste
// reference handling, soft-delete, flush retry, session-scoping.

func TestBDD_PromptHistory(t *testing.T) {
	t.Run("Scenario_AppendAddsEntryWithAutomaticTimestamp", func(t *testing.T) {
		// Given a fresh history log (PDF Section 9.1: prompts persisted
		//       individually, ordered by time),
		log := NewHistoryLog(nil, nil)

		// When the runner appends a prompt without explicit timestamp,
		log.Append(HistoryEntry{Display: "summarise the docs"})

		// Then the entry exists with an auto-assigned timestamp.
		assert.Equal(t, 1, log.Len(),
			"Append must add the entry to the log")
		entries := log.GetHistory("")
		assert.Len(t, entries, 1)
		assert.Equal(t, "summarise the docs", entries[0].Display)
		assert.False(t, entries[0].Timestamp.IsZero(),
			"missing timestamp must be auto-assigned at Append")
	})

	t.Run("Scenario_GetHistoryReturnsNewestFirst", func(t *testing.T) {
		// Given multiple prompts appended over time (PDF Section 9.1:
		//       readLinesReverse — reverse order for Up-arrow navigation),
		log := NewHistoryLog(nil, nil)
		log.Append(HistoryEntry{Display: "first", Timestamp: time.Unix(100, 0)})
		log.Append(HistoryEntry{Display: "second", Timestamp: time.Unix(200, 0)})
		log.Append(HistoryEntry{Display: "third", Timestamp: time.Unix(300, 0)})

		// When the UI reads history,
		when := log.GetHistory("")

		// Then entries appear newest-first — direct equivalent of PDF's
		//      reverse iteration.
		assert.Equal(t, "third", when[0].Display,
			"newest entry must come first (reverse-iter equivalent)")
		assert.Equal(t, "second", when[1].Display)
		assert.Equal(t, "first", when[2].Display)
	})

	t.Run("Scenario_GetHistoryDedupsByDisplay", func(t *testing.T) {
		// Given the same prompt entered multiple times (PDF Section 9.1:
		//       readers dedup so Up-arrow doesn't cycle through identical
		//       repeats),
		log := NewHistoryLog(nil, nil)
		log.Append(HistoryEntry{Display: "ls", Timestamp: time.Unix(100, 0)})
		log.Append(HistoryEntry{Display: "pwd", Timestamp: time.Unix(200, 0)})
		log.Append(HistoryEntry{Display: "ls", Timestamp: time.Unix(300, 0)}) // dup
		log.Append(HistoryEntry{Display: "ls", Timestamp: time.Unix(400, 0)}) // dup

		// When the UI reads history,
		when := log.GetHistory("")

		// Then duplicates are collapsed to the most recent occurrence.
		assert.Len(t, when, 2,
			"dedup must collapse 'ls' duplicates to one entry")
		assert.Equal(t, "ls", when[0].Display,
			"latest 'ls' wins position 0 (newest-first + dedup)")
		assert.Equal(t, "pwd", when[1].Display)
	})

	t.Run("Scenario_GetHistorySessionScopedFloatsToTop", func(t *testing.T) {
		// Given prompts from multiple sessions,
		log := NewHistoryLog(nil, nil)
		log.Append(HistoryEntry{Display: "global-1", Timestamp: time.Unix(100, 0)})
		log.Append(HistoryEntry{Display: "session-A", SessionID: "A", Timestamp: time.Unix(200, 0)})
		log.Append(HistoryEntry{Display: "global-2", Timestamp: time.Unix(300, 0)})
		log.Append(HistoryEntry{Display: "session-A2", SessionID: "A", Timestamp: time.Unix(400, 0)})

		// When the UI requests history for session A,
		when := log.GetHistory("A")

		// Then session-A entries appear FIRST, then global entries — the UI
		//      surfaces the user's recent contextual prompts before global
		//      history.
		assert.Equal(t, "session-A2", when[0].Display)
		assert.Equal(t, "session-A", when[1].Display)
		assert.Equal(t, "global-2", when[2].Display)
		assert.Equal(t, "global-1", when[3].Display)
	})

	t.Run("Scenario_LargePasteIsConvertedToHashReference", func(t *testing.T) {
		// Given a prompt with a large paste exceeding the inline threshold
		//       (PDF Section 9.1 + economy: history file would explode if
		//       multi-MB pastes were inlined),
		large := strings.Repeat("A", HistoryInlineThreshold+1)
		log := NewHistoryLog(nil, nil)

		// When the runner appends it,
		log.Append(HistoryEntry{
			Display: "review this code",
			PastedContents: []PastedContent{
				{Inline: large, Label: "code.go"},
			},
		})

		// Then the inline content is replaced by a hash reference — keeping
		//      the history log compact, with the large content in external
		//      storage.
		entries := log.GetHistory("")
		assert.Len(t, entries[0].PastedContents, 1)
		paste := entries[0].PastedContents[0]
		assert.True(t, paste.IsReference(),
			"oversize paste must be auto-converted to hash reference")
		assert.Empty(t, paste.Inline,
			"inline must be cleared after reference conversion")
		assert.NotEmpty(t, paste.HashRef,
			"HashRef must be populated by the conversion")
	})

	t.Run("Scenario_SmallPasteRemainsInlineForSpeed", func(t *testing.T) {
		// Given a small paste under the threshold,
		small := strings.Repeat("x", HistoryInlineThreshold-1)
		log := NewHistoryLog(nil, nil)

		// When appended,
		log.Append(HistoryEntry{
			Display: "fix this",
			PastedContents: []PastedContent{{Inline: small}},
		})

		// Then the inline content is preserved — fast retrieval, no extra
		//      I/O for tiny pastes.
		entries := log.GetHistory("")
		paste := entries[0].PastedContents[0]
		assert.False(t, paste.IsReference(),
			"small paste must remain inline")
		assert.Equal(t, small, paste.Inline,
			"inline content must round-trip verbatim")
	})

	t.Run("Scenario_ResolveEntryExpandsHashReferences", func(t *testing.T) {
		// Given a history with a hash reference + a resolver that knows the
		//       content,
		log := NewHistoryLog(nil,
			func(ref string) (string, bool) {
				if ref == "ref_001" {
					return "expanded full content", true
				}
				return "", false
			},
		)
		entry := HistoryEntry{
			Display: "review it",
			PastedContents: []PastedContent{
				{HashRef: "ref_001", Label: "big.go"},
			},
		}

		// When the UI resolves the entry for display,
		when := log.ResolveEntry(entry)

		// Then the inline content is restored from the resolver — the user
		//      sees the actual content, not a cryptic hash.
		assert.Len(t, when.PastedContents, 1)
		assert.False(t, when.PastedContents[0].IsReference(),
			"after Resolve, paste must be inline")
		assert.Equal(t, "expanded full content", when.PastedContents[0].Inline)
		assert.Equal(t, "big.go", when.PastedContents[0].Label,
			"label survives resolve")
	})

	t.Run("Scenario_RemoveSoftDeletesByTimestamp", func(t *testing.T) {
		// Given a sensitive prompt the user wants to forget (PDF Section 11
		//       implication: history must support targeted removal for
		//       privacy/GDPR),
		ts := time.Unix(1234567890, 0)
		log := NewHistoryLog(nil, nil)
		log.Append(HistoryEntry{Display: "delete me", Timestamp: ts})
		log.Append(HistoryEntry{Display: "keep me", Timestamp: time.Unix(1234567900, 0)})

		// When the user removes the offending entry,
		log.Remove(ts)

		// Then it disappears from history but the log retains structure
		//      (soft-delete via removed map).
		when := log.GetHistory("")
		for _, e := range when {
			assert.NotEqual(t, "delete me", e.Display,
				"removed entry must not appear in subsequent reads")
		}
	})

	t.Run("Scenario_FlushRetriesOnFailureAndReQueuesPersistedItems", func(t *testing.T) {
		// Given a flusher that fails the first 2 attempts then succeeds (PDF
		//       robustness: history flush must not lose entries on transient
		//       errors),
		var attempts atomic.Int32
		log := NewHistoryLog(
			func(_ []HistoryEntry) error {
				attempts.Add(1)
				if attempts.Load() < 3 {
					return errors.New("transient")
				}
				return nil
			},
			nil,
		)
		log.Append(HistoryEntry{Display: "to flush"})
		assert.Equal(t, 1, log.PendingCount(),
			"new entry must be in pending queue")

		// When the runner flushes,
		flushed, err := log.Flush()

		// Then it succeeds after retries — entries are not lost.
		assert.NoError(t, err,
			"flush must succeed after retries within HistoryMaxFlushRetries")
		assert.Equal(t, 1, flushed,
			"one entry must report flushed")
		assert.GreaterOrEqual(t, attempts.Load(), int32(3),
			"flusher must be invoked until success or retry-cap")
	})

	t.Run("Scenario_FlushFailureReQueuesEntriesForRetry", func(t *testing.T) {
		// Given a flusher that always fails,
		log := NewHistoryLog(
			func(_ []HistoryEntry) error { return errors.New("permanent") },
			nil,
		)
		log.Append(HistoryEntry{Display: "doomed"})

		// When the runner attempts flush,
		flushed, err := log.Flush()

		// Then the error surfaces and entries are re-queued — preventing
		//      data loss; the next flush attempt will retry them.
		assert.Error(t, err,
			"persistent flush failure must surface as error")
		assert.Equal(t, 0, flushed,
			"no entries reported flushed when failure persisted")
		assert.GreaterOrEqual(t, log.PendingCount(), 1,
			"failed entries must remain in pending queue for retry")
	})

	t.Run("Scenario_HistoryEntrySerializesAsJSONLLine", func(t *testing.T) {
		// Given the on-disk format is JSONL (PDF Section 9.1: history.jsonl
		//       at config home),
		entries := []HistoryEntry{
			{Display: "a", Timestamp: time.Unix(100, 0)},
			{Display: "b", Timestamp: time.Unix(200, 0)},
		}

		// When marshalled,
		raw, err := MarshalJSONL(entries)

		// Then it produces line-delimited JSON suitable for append-only file
		//      semantics — direct equivalent of PDF history.jsonl.
		assert.NoError(t, err)
		assert.Contains(t, raw, "\n",
			"JSONL output must use newline delimiter between entries")
	})
}
