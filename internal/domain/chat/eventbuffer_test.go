package chat_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func makeEvent(typ string, content string) chat.RunEvent {
	data, _ := json.Marshal(map[string]string{"content": content})
	return chat.RunEvent{Type: typ, Data: data}
}

func TestNewEventBuffer_DefaultCapacity(t *testing.T) {
	buf := chat.NewEventBuffer(0)
	assert.Equal(t, 0, buf.Len())
	assert.Equal(t, uint64(0), buf.OldestSeq())
	assert.Equal(t, uint64(0), buf.NewestSeq())
}

func TestEventBuffer_AppendAndLen(t *testing.T) {
	buf := chat.NewEventBuffer(10)

	seq1 := buf.Append(makeEvent("text_delta", "hello"))
	seq2 := buf.Append(makeEvent("text_delta", "world"))

	assert.Equal(t, uint64(1), seq1)
	assert.Equal(t, uint64(2), seq2)
	assert.Equal(t, 2, buf.Len())
	assert.Equal(t, uint64(1), buf.OldestSeq())
	assert.Equal(t, uint64(2), buf.NewestSeq())
}

func TestEventBuffer_EventsSince_All(t *testing.T) {
	buf := chat.NewEventBuffer(10)
	buf.Append(makeEvent("text_delta", "a"))
	buf.Append(makeEvent("text_delta", "b"))
	buf.Append(makeEvent("tool_call_start", "c"))

	events, ok := buf.EventsSince(0)
	require.True(t, ok)
	assert.Len(t, events, 3)
	assert.Equal(t, uint64(1), events[0].ID)
	assert.Equal(t, uint64(2), events[1].ID)
	assert.Equal(t, uint64(3), events[2].ID)
}

func TestEventBuffer_EventsSince_Partial(t *testing.T) {
	buf := chat.NewEventBuffer(10)
	buf.Append(makeEvent("text_delta", "a"))
	buf.Append(makeEvent("text_delta", "b"))
	buf.Append(makeEvent("text_delta", "c"))

	events, ok := buf.EventsSince(1)
	require.True(t, ok)
	assert.Len(t, events, 2)
	assert.Equal(t, uint64(2), events[0].ID)
	assert.Equal(t, uint64(3), events[1].ID)
}

func TestEventBuffer_EventsSince_AfterNewest(t *testing.T) {
	buf := chat.NewEventBuffer(10)
	buf.Append(makeEvent("text_delta", "a"))

	events, ok := buf.EventsSince(1)
	require.True(t, ok)
	assert.Len(t, events, 0)
}

func TestEventBuffer_CircularOverwrite(t *testing.T) {
	buf := chat.NewEventBuffer(3)

	buf.Append(makeEvent("text_delta", "a"))  // seq 1
	buf.Append(makeEvent("text_delta", "b"))  // seq 2
	buf.Append(makeEvent("text_delta", "c"))  // seq 3
	buf.Append(makeEvent("text_delta", "d"))  // seq 4 — evicts seq 1
	buf.Append(makeEvent("text_delta", "e"))  // seq 5 — evicts seq 2

	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, uint64(3), buf.OldestSeq())
	assert.Equal(t, uint64(5), buf.NewestSeq())

	// Events since 2 — seq 2 is evicted, so overflow.
	events, ok := buf.EventsSince(2)
	assert.False(t, ok)
	assert.Nil(t, events)

	// Events since 3 — seq 4 and 5 should be returned.
	events, ok = buf.EventsSince(3)
	require.True(t, ok)
	assert.Len(t, events, 2)
	assert.Equal(t, uint64(4), events[0].ID)
	assert.Equal(t, uint64(5), events[1].ID)
}

func TestEventBuffer_Overflow_Empty(t *testing.T) {
	buf := chat.NewEventBuffer(5)
	events, ok := buf.EventsSince(0)
	require.True(t, ok)
	assert.Len(t, events, 0)
}

func TestEventBuffer_MarkDone(t *testing.T) {
	buf := chat.NewEventBuffer(10)
	assert.False(t, buf.IsDone())
	buf.MarkDone()
	assert.True(t, buf.IsDone())
}

func TestParseSSEID_Valid(t *testing.T) {
	runID, seq, ok := chat.ParseSSEID("abc-123:42")
	assert.True(t, ok)
	assert.Equal(t, "abc-123", runID)
	assert.Equal(t, uint64(42), seq)
}

func TestParseSSEID_UUIDRunID(t *testing.T) {
	runID, seq, ok := chat.ParseSSEID("550e8400-e29b-41d4-a716-446655440000:100")
	assert.True(t, ok)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", runID)
	assert.Equal(t, uint64(100), seq)
}

func TestParseSSEID_Invalid(t *testing.T) {
	tests := []string{
		"",
		"nocolon",
		"prefix:",
		"prefix:notanumber",
		":42",
	}
	for _, input := range tests {
		_, _, ok := chat.ParseSSEID(input)
		assert.False(t, ok, "expected false for input: %q", input)
	}
}

func TestFormatSSEID(t *testing.T) {
	be := chat.BufferedEvent{ID: 42, Event: makeEvent("text_delta", "hello")}
	assert.Equal(t, "run-123:42", be.FormatSSEID("run-123"))
}

func TestRunEventBufferRegistry_GetOrCreate(t *testing.T) {
	reg := chat.NewRunEventBufferRegistry()
	assert.Equal(t, 0, reg.Len())

	buf1 := reg.GetOrCreate("run-1", 100)
	assert.NotNil(t, buf1)
	assert.Equal(t, 1, reg.Len())

	// Same key returns same buffer.
	buf2 := reg.GetOrCreate("run-1", 200)
	assert.Equal(t, buf1, buf2)
	assert.Equal(t, 1, reg.Len())
}

func TestRunEventBufferRegistry_Get(t *testing.T) {
	reg := chat.NewRunEventBufferRegistry()
	assert.Nil(t, reg.Get("nonexistent"))

	reg.GetOrCreate("run-1", 100)
	buf := reg.Get("run-1")
	assert.NotNil(t, buf)
}

func TestRunEventBufferRegistry_Remove(t *testing.T) {
	reg := chat.NewRunEventBufferRegistry()
	reg.GetOrCreate("run-1", 100)
	assert.Equal(t, 1, reg.Len())

	reg.Remove("run-1")
	assert.Equal(t, 0, reg.Len())
	assert.Nil(t, reg.Get("run-1"))
}

func TestEventBuffer_ConcurrentAccess(t *testing.T) {
	buf := chat.NewEventBuffer(100)
	done := make(chan struct{})

	// Writer goroutine.
	go func() {
		for i := 0; i < 200; i++ {
			buf.Append(makeEvent("text_delta", "data"))
		}
		buf.MarkDone()
		close(done)
	}()

	// Reader goroutine.
	for i := 0; i < 100; i++ {
		buf.EventsSince(0)
		buf.Len()
		buf.IsDone()
	}

	<-done
	assert.True(t, buf.IsDone())
	assert.Equal(t, 100, buf.Len()) // capacity is 100, wrote 200
}
