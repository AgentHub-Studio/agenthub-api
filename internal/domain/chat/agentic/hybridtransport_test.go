package agentic_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func makeMsg(typ string) agentic.TransportMessage {
	return agentic.TransportMessage{
		Type:      typ,
		Data:      json.RawMessage(`{}`),
		Timestamp: time.Now(),
	}
}

// --- DefaultHybridTransportConfig ---

func TestDefaultHybridTransportConfig(t *testing.T) {
	cfg := agentic.DefaultHybridTransportConfig()
	assert.Equal(t, 100*time.Millisecond, cfg.StreamBufferWindow)
	assert.Equal(t, 100, cfg.MaxBatchSize)
	assert.True(t, cfg.StreamEventTypes["stream_event"])
	assert.True(t, cfg.StreamEventTypes["text_delta"])
}

// --- NewHybridTransport ---

func TestNewHybridTransport(t *testing.T) {
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		return nil
	})
	assert.NotNil(t, ht)
	assert.False(t, ht.IsClosed())
}

// --- Write non-stream message ---

func TestHybridTransport_Write_NonStream(t *testing.T) {
	var batches [][]agentic.TransportMessage
	var mu sync.Mutex
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		return nil
	})

	err := ht.Write(makeMsg("tool_result"))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, batches, 1)
	assert.Equal(t, "tool_result", batches[0][0].Type)
}

// --- Read channel ---

func TestHybridTransport_ReadDispatch(t *testing.T) {
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		return nil
	})

	var received agentic.TransportMessage
	ht.OnRead(func(msg agentic.TransportMessage) {
		received = msg
	})

	msg := makeMsg("test")
	ht.DispatchRead(msg)

	assert.Equal(t, "test", received.Type)
}

func TestHybridTransport_ReadMultipleHandlers(t *testing.T) {
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		return nil
	})

	count := 0
	ht.OnRead(func(msg agentic.TransportMessage) { count++ })
	ht.OnRead(func(msg agentic.TransportMessage) { count++ })

	ht.DispatchRead(makeMsg("x"))
	assert.Equal(t, 2, count)
}

// --- Stream buffering ---

func TestHybridTransport_StreamBuffering(t *testing.T) {
	var batches [][]agentic.TransportMessage
	var mu sync.Mutex
	cfg := agentic.DefaultHybridTransportConfig()
	cfg.StreamBufferWindow = 50 * time.Millisecond

	ht := agentic.NewHybridTransport(cfg, func(batch []agentic.TransportMessage) error {
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		return nil
	})

	// Write stream events
	require.NoError(t, ht.Write(makeMsg("stream_event")))
	require.NoError(t, ht.Write(makeMsg("text_delta")))

	// Wait for timer flush
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(batches), 1)
	// Both stream events should be in the same batch
	totalMsgs := 0
	for _, b := range batches {
		totalMsgs += len(b)
	}
	assert.Equal(t, 2, totalMsgs)
}

// --- Non-stream flushes buffer ---

func TestHybridTransport_NonStreamFlushesBuffer(t *testing.T) {
	var batches [][]agentic.TransportMessage
	var mu sync.Mutex
	cfg := agentic.DefaultHybridTransportConfig()
	cfg.StreamBufferWindow = 5 * time.Second // long window, won't fire

	ht := agentic.NewHybridTransport(cfg, func(batch []agentic.TransportMessage) error {
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		return nil
	})

	// Buffer a stream event
	require.NoError(t, ht.Write(makeMsg("stream_event")))
	// Write non-stream — should flush buffer first
	require.NoError(t, ht.Write(makeMsg("tool_result")))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, batches, 2)
	assert.Equal(t, "stream_event", batches[0][0].Type)
	assert.Equal(t, "tool_result", batches[1][0].Type)
}

// --- Flush ---

func TestHybridTransport_Flush(t *testing.T) {
	var batches [][]agentic.TransportMessage
	var mu sync.Mutex
	cfg := agentic.DefaultHybridTransportConfig()
	cfg.StreamBufferWindow = 5 * time.Second

	ht := agentic.NewHybridTransport(cfg, func(batch []agentic.TransportMessage) error {
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		return nil
	})

	require.NoError(t, ht.Write(makeMsg("stream_event")))
	require.NoError(t, ht.Flush())

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, batches, 1)
}

func TestHybridTransport_Flush_Empty(t *testing.T) {
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		return nil
	})

	err := ht.Flush()
	assert.NoError(t, err)
}

// --- Close ---

func TestHybridTransport_Close(t *testing.T) {
	var batches [][]agentic.TransportMessage
	var mu sync.Mutex

	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		return nil
	})

	require.NoError(t, ht.Write(makeMsg("stream_event")))
	require.NoError(t, ht.Close())

	assert.True(t, ht.IsClosed())

	mu.Lock()
	defer mu.Unlock()
	// Buffered events should have been flushed on close
	assert.GreaterOrEqual(t, len(batches), 1)
}

// --- Stats ---

func TestHybridTransport_Stats(t *testing.T) {
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		return nil
	})

	require.NoError(t, ht.Write(makeMsg("tool_result")))
	require.NoError(t, ht.Write(makeMsg("tool_result")))

	assert.Equal(t, 2, ht.TotalWrites())
	assert.Equal(t, 0, ht.DroppedBatches())
}

// --- WriteBatch ---

func TestHybridTransport_WriteBatch(t *testing.T) {
	var totalMsgs int
	var mu sync.Mutex

	cfg := agentic.DefaultHybridTransportConfig()
	cfg.MaxBatchSize = 3

	ht := agentic.NewHybridTransport(cfg, func(batch []agentic.TransportMessage) error {
		mu.Lock()
		totalMsgs += len(batch)
		mu.Unlock()
		return nil
	})

	msgs := make([]agentic.TransportMessage, 7)
	for i := range msgs {
		msgs[i] = makeMsg("data")
	}

	err := ht.WriteBatch(msgs)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 7, totalMsgs)
}

// --- Write after close ---

func TestHybridTransport_WriteAfterClose(t *testing.T) {
	ht := agentic.NewHybridTransport(agentic.DefaultHybridTransportConfig(), func(batch []agentic.TransportMessage) error {
		return nil
	})

	require.NoError(t, ht.Close())
	err := ht.Write(makeMsg("x"))
	assert.NoError(t, err) // silently ignored
}
