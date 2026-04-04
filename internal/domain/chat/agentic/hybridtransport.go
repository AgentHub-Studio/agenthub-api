package agentic

import (
	"encoding/json"
	"sync"
	"time"
)

// Asymmetric dual-channel transport with stream buffering.
//
// Inspired by Claude Code's HybridTransport.ts — reads via a subscription
// channel (low-latency) while writes go through a batched uploader (reliable,
// ordered). Stream events are micro-batched within a configurable time window
// to reduce write overhead during high-volume streaming.

// TransportMessage represents a message flowing through the transport.
type TransportMessage struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// TransportWriteFunc sends a batch of messages to the remote endpoint.
type TransportWriteFunc func(batch []TransportMessage) error

// TransportReadFunc is called when a message is received.
type TransportReadFunc func(msg TransportMessage)

// HybridTransportConfig configures the hybrid transport.
type HybridTransportConfig struct {
	// StreamBufferWindow is how long to accumulate stream events before flushing.
	StreamBufferWindow time.Duration
	// MaxBatchSize limits the number of messages per write batch.
	MaxBatchSize int
	// StreamEventTypes are message types eligible for micro-batching.
	StreamEventTypes map[string]bool
}

// DefaultHybridTransportConfig returns sensible defaults.
func DefaultHybridTransportConfig() HybridTransportConfig {
	return HybridTransportConfig{
		StreamBufferWindow: 100 * time.Millisecond,
		MaxBatchSize:       100,
		StreamEventTypes: map[string]bool{
			"stream_event": true,
			"text_delta":   true,
		},
	}
}

// HybridTransport provides asymmetric read/write with stream buffering.
type HybridTransport struct {
	mu            sync.Mutex
	config        HybridTransportConfig
	writeFn       TransportWriteFunc
	readHandlers  []TransportReadFunc
	streamBuffer  []TransportMessage
	flushTimer    *time.Timer
	closed        bool
	droppedBatches int
	totalWrites   int
}

// NewHybridTransport creates a hybrid transport.
func NewHybridTransport(config HybridTransportConfig, writeFn TransportWriteFunc) *HybridTransport {
	if config.StreamBufferWindow <= 0 {
		config.StreamBufferWindow = 100 * time.Millisecond
	}
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 100
	}
	if config.StreamEventTypes == nil {
		config.StreamEventTypes = map[string]bool{"stream_event": true}
	}

	return &HybridTransport{
		config:  config,
		writeFn: writeFn,
	}
}

// OnRead registers a handler for incoming messages (read channel).
func (t *HybridTransport) OnRead(fn TransportReadFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.readHandlers = append(t.readHandlers, fn)
}

// DispatchRead simulates receiving a message on the read channel.
// Called by the underlying connection (e.g., WebSocket).
func (t *HybridTransport) DispatchRead(msg TransportMessage) {
	t.mu.Lock()
	handlers := make([]TransportReadFunc, len(t.readHandlers))
	copy(handlers, t.readHandlers)
	t.mu.Unlock()

	for _, fn := range handlers {
		fn(msg)
	}
}

// Write sends a message through the write channel.
// Stream events are micro-batched; other messages flush immediately.
func (t *HybridTransport) Write(msg TransportMessage) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	if t.config.StreamEventTypes[msg.Type] {
		return t.bufferStreamEvent(msg)
	}

	// Non-stream message: flush buffer first to preserve order, then send
	buffered := t.drainBuffer()
	t.mu.Unlock()

	if len(buffered) > 0 {
		if err := t.sendBatch(buffered); err != nil {
			return err
		}
	}

	return t.sendBatch([]TransportMessage{msg})
}

// WriteBatch sends multiple messages, preserving order.
func (t *HybridTransport) WriteBatch(msgs []TransportMessage) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}

	// Flush any buffered stream events first
	buffered := t.drainBuffer()
	t.mu.Unlock()

	if len(buffered) > 0 {
		if err := t.sendBatch(buffered); err != nil {
			return err
		}
	}

	// Send in MaxBatchSize chunks
	for i := 0; i < len(msgs); i += t.config.MaxBatchSize {
		end := i + t.config.MaxBatchSize
		if end > len(msgs) {
			end = len(msgs)
		}
		if err := t.sendBatch(msgs[i:end]); err != nil {
			return err
		}
	}

	return nil
}

// Flush forces all buffered stream events to be sent immediately.
func (t *HybridTransport) Flush() error {
	t.mu.Lock()
	buffered := t.drainBuffer()
	t.mu.Unlock()

	if len(buffered) == 0 {
		return nil
	}

	return t.sendBatch(buffered)
}

// Close stops the transport and flushes remaining messages.
func (t *HybridTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	buffered := t.drainBuffer()
	t.mu.Unlock()

	if len(buffered) > 0 {
		return t.sendBatch(buffered)
	}
	return nil
}

// IsClosed returns whether the transport is closed.
func (t *HybridTransport) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// DroppedBatches returns the count of failed writes.
func (t *HybridTransport) DroppedBatches() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.droppedBatches
}

// TotalWrites returns the total number of successful batch writes.
func (t *HybridTransport) TotalWrites() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalWrites
}

// BufferedCount returns the current stream buffer size.
func (t *HybridTransport) BufferedCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.streamBuffer)
}

// bufferStreamEvent adds a stream event to the buffer and starts the flush timer.
// Caller must hold t.mu.
func (t *HybridTransport) bufferStreamEvent(msg TransportMessage) error {
	t.streamBuffer = append(t.streamBuffer, msg)

	// Start timer if not already running
	if t.flushTimer == nil {
		t.flushTimer = time.AfterFunc(t.config.StreamBufferWindow, func() {
			t.mu.Lock()
			buffered := t.drainBuffer()
			t.mu.Unlock()

			if len(buffered) > 0 {
				_ = t.sendBatch(buffered) // best effort
			}
		})
	}

	// Flush if buffer is full
	if len(t.streamBuffer) >= t.config.MaxBatchSize {
		buffered := t.drainBuffer()
		t.mu.Unlock()
		err := t.sendBatch(buffered)
		t.mu.Lock()
		return err
	}

	t.mu.Unlock()
	return nil
}

// drainBuffer returns and clears the stream buffer. Caller must hold t.mu.
func (t *HybridTransport) drainBuffer() []TransportMessage {
	if len(t.streamBuffer) == 0 {
		return nil
	}

	buffered := t.streamBuffer
	t.streamBuffer = nil

	if t.flushTimer != nil {
		t.flushTimer.Stop()
		t.flushTimer = nil
	}

	return buffered
}

// sendBatch calls the write function and tracks stats.
func (t *HybridTransport) sendBatch(batch []TransportMessage) error {
	err := t.writeFn(batch)

	t.mu.Lock()
	defer t.mu.Unlock()

	if err != nil {
		t.droppedBatches++
		return err
	}

	t.totalWrites++
	return nil
}
