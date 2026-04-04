package agentic_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestHistoryLog_Constants(t *testing.T) {
	assert.Equal(t, 1024, agentic.HistoryInlineThreshold)
	assert.Equal(t, 5, agentic.HistoryMaxFlushRetries)
}

// --- PastedContent ---

func TestPastedContent_IsReference(t *testing.T) {
	inline := agentic.PastedContent{Inline: "hello"}
	assert.False(t, inline.IsReference())

	ref := agentic.PastedContent{HashRef: "abc123"}
	assert.True(t, ref.IsReference())
}

// --- NewHistoryLog ---

func TestNewHistoryLog(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	assert.Equal(t, 0, h.Len())
	assert.Equal(t, 0, h.PendingCount())
}

// --- Append ---

func TestHistoryLog_Append(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	h.Append(agentic.HistoryEntry{Display: "hello", SessionID: "s1"})
	assert.Equal(t, 1, h.Len())
	assert.Equal(t, 1, h.PendingCount())
}

func TestHistoryLog_Append_AutoTimestamp(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	h.Append(agentic.HistoryEntry{Display: "auto"})
	entries := h.GetHistory("")
	require.Len(t, entries, 1)
	assert.False(t, entries[0].Timestamp.IsZero())
}

func TestHistoryLog_Append_LargePasteConvertedToRef(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	largePaste := strings.Repeat("x", 2000)
	h.Append(agentic.HistoryEntry{
		Display: "with paste",
		PastedContents: []agentic.PastedContent{
			{Inline: largePaste, Label: "big"},
		},
	})

	entries := h.GetHistory("")
	require.Len(t, entries, 1)
	require.Len(t, entries[0].PastedContents, 1)
	assert.True(t, entries[0].PastedContents[0].IsReference())
	assert.Equal(t, "big", entries[0].PastedContents[0].Label)
}

func TestHistoryLog_Append_SmallPasteStaysInline(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	h.Append(agentic.HistoryEntry{
		Display: "small paste",
		PastedContents: []agentic.PastedContent{
			{Inline: "small content", Label: "tiny"},
		},
	})

	entries := h.GetHistory("")
	require.Len(t, entries[0].PastedContents, 1)
	assert.False(t, entries[0].PastedContents[0].IsReference())
	assert.Equal(t, "small content", entries[0].PastedContents[0].Inline)
}

// --- GetHistory ---

func TestHistoryLog_GetHistory_NewestFirst(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	h.Append(agentic.HistoryEntry{Display: "first", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	h.Append(agentic.HistoryEntry{Display: "second", Timestamp: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})

	entries := h.GetHistory("")
	require.Len(t, entries, 2)
	assert.Equal(t, "second", entries[0].Display)
	assert.Equal(t, "first", entries[1].Display)
}

func TestHistoryLog_GetHistory_Deduped(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	h.Append(agentic.HistoryEntry{Display: "same"})
	h.Append(agentic.HistoryEntry{Display: "same"})
	h.Append(agentic.HistoryEntry{Display: "different"})

	entries := h.GetHistory("")
	assert.Len(t, entries, 2)
}

func TestHistoryLog_GetHistory_SessionFirst(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	h.Append(agentic.HistoryEntry{Display: "other-session", SessionID: "s2"})
	h.Append(agentic.HistoryEntry{Display: "this-session", SessionID: "s1"})

	entries := h.GetHistory("s1")
	require.Len(t, entries, 2)
	assert.Equal(t, "this-session", entries[0].Display, "session entries should come first")
}

// --- Remove ---

func TestHistoryLog_Remove(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h.Append(agentic.HistoryEntry{Display: "to remove", Timestamp: ts})
	h.Append(agentic.HistoryEntry{Display: "keep"})

	h.Remove(ts)

	entries := h.GetHistory("")
	require.Len(t, entries, 1)
	assert.Equal(t, "keep", entries[0].Display)
}

// --- ResolveEntry ---

func TestHistoryLog_ResolveEntry(t *testing.T) {
	resolver := func(hashRef string) (string, bool) {
		if hashRef == "hash123" {
			return "resolved content", true
		}
		return "", false
	}

	h := agentic.NewHistoryLog(nil, resolver)
	entry := agentic.HistoryEntry{
		Display: "test",
		PastedContents: []agentic.PastedContent{
			{HashRef: "hash123", Label: "doc"},
		},
	}

	resolved := h.ResolveEntry(entry)
	require.Len(t, resolved.PastedContents, 1)
	assert.Equal(t, "resolved content", resolved.PastedContents[0].Inline)
	assert.Equal(t, "doc", resolved.PastedContents[0].Label)
}

func TestHistoryLog_ResolveEntry_NilResolver(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	entry := agentic.HistoryEntry{
		Display:        "test",
		PastedContents: []agentic.PastedContent{{HashRef: "abc"}},
	}
	resolved := h.ResolveEntry(entry)
	assert.True(t, resolved.PastedContents[0].IsReference(), "should remain unresolved")
}

// --- Flush ---

func TestHistoryLog_Flush_NoPending(t *testing.T) {
	h := agentic.NewHistoryLog(nil, nil)
	n, err := h.Flush()
	assert.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestHistoryLog_Flush_Success(t *testing.T) {
	var flushed []agentic.HistoryEntry
	flusher := func(entries []agentic.HistoryEntry) error {
		flushed = append(flushed, entries...)
		return nil
	}

	h := agentic.NewHistoryLog(flusher, nil)
	h.Append(agentic.HistoryEntry{Display: "a"})
	h.Append(agentic.HistoryEntry{Display: "b"})

	n, err := h.Flush()
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Len(t, flushed, 2)
	assert.Equal(t, 0, h.PendingCount())
}

func TestHistoryLog_Flush_RetryOnError(t *testing.T) {
	callCount := 0
	flusher := func(entries []agentic.HistoryEntry) error {
		callCount++
		if callCount < 3 {
			return fmt.Errorf("transient error")
		}
		return nil
	}

	h := agentic.NewHistoryLog(flusher, nil)
	h.Append(agentic.HistoryEntry{Display: "retry"})

	n, err := h.Flush()
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 3, callCount)
}

func TestHistoryLog_Flush_RequeueOnAllRetryFail(t *testing.T) {
	flusher := func(entries []agentic.HistoryEntry) error {
		return fmt.Errorf("always fail")
	}

	h := agentic.NewHistoryLog(flusher, nil)
	h.Append(agentic.HistoryEntry{Display: "fail"})

	_, err := h.Flush()
	assert.Error(t, err)
	assert.Equal(t, 1, h.PendingCount(), "should re-enqueue on failure")
}

// --- MarshalJSONL ---

func TestMarshalJSONL(t *testing.T) {
	entries := []agentic.HistoryEntry{
		{Display: "line1"},
		{Display: "line2"},
	}
	jsonl, err := agentic.MarshalJSONL(entries)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(jsonl), "\n")
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "line1")
}
