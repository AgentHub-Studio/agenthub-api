package agentic

import (
	"sync"
	"time"
)

// Time-windowed micro-batching for high-frequency events.
//
// Inspired by Claude Code's stream event buffering in HybridTransport.ts —
// accumulates events within a configurable time window and flushes them
// as a batch. Reduces per-event overhead (network calls, writes) during
// high-volume streaming while bounding latency.

// StreamBufferConfig configures the stream buffer.
type StreamBufferConfig struct {
	// Window is the maximum time to accumulate events before flushing.
	Window time.Duration
	// MaxSize is the maximum number of events before forced flush.
	MaxSize int
}

// DefaultStreamBufferConfig returns sensible defaults.
func DefaultStreamBufferConfig() StreamBufferConfig {
	return StreamBufferConfig{
		Window:  100 * time.Millisecond,
		MaxSize: 50,
	}
}

// StreamBufferFlushFunc is called when the buffer flushes.
type StreamBufferFlushFunc[T any] func(batch []T)

// StreamBuffer accumulates events and flushes them periodically or when full.
type StreamBuffer[T any] struct {
	mu       sync.Mutex
	config   StreamBufferConfig
	items    []T
	flushFn  StreamBufferFlushFunc[T]
	timer    *time.Timer
	closed   bool
	flushes  int
}

// NewStreamBuffer creates a stream buffer.
func NewStreamBuffer[T any](config StreamBufferConfig, flushFn StreamBufferFlushFunc[T]) *StreamBuffer[T] {
	if config.Window <= 0 {
		config.Window = 100 * time.Millisecond
	}
	if config.MaxSize <= 0 {
		config.MaxSize = 50
	}
	return &StreamBuffer[T]{
		config:  config,
		flushFn: flushFn,
	}
}

// Add appends an event to the buffer. May trigger a flush if the
// buffer reaches MaxSize.
func (b *StreamBuffer[T]) Add(item T) {
	b.mu.Lock()

	if b.closed {
		b.mu.Unlock()
		return
	}

	b.items = append(b.items, item)

	// Start timer on first item
	if len(b.items) == 1 && b.timer == nil {
		b.timer = time.AfterFunc(b.config.Window, func() {
			b.flushLocked()
		})
	}

	// Flush if full
	if len(b.items) >= b.config.MaxSize {
		items := b.drain()
		b.mu.Unlock()
		b.flushFn(items)
		b.mu.Lock()
		b.flushes++
		b.mu.Unlock()
		return
	}

	b.mu.Unlock()
}

// Flush forces an immediate flush of all buffered items.
func (b *StreamBuffer[T]) Flush() {
	b.flushLocked()
}

// Close flushes remaining items and prevents further additions.
func (b *StreamBuffer[T]) Close() {
	b.mu.Lock()
	b.closed = true
	items := b.drain()
	b.mu.Unlock()

	if len(items) > 0 {
		b.flushFn(items)
		b.mu.Lock()
		b.flushes++
		b.mu.Unlock()
	}
}

// IsClosed returns whether the buffer is closed.
func (b *StreamBuffer[T]) IsClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// Len returns the current number of buffered items.
func (b *StreamBuffer[T]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.items)
}

// FlushCount returns the total number of flushes performed.
func (b *StreamBuffer[T]) FlushCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushes
}

// flushLocked performs the flush, acquiring the lock itself.
func (b *StreamBuffer[T]) flushLocked() {
	b.mu.Lock()
	items := b.drain()
	b.mu.Unlock()

	if len(items) > 0 {
		b.flushFn(items)
		b.mu.Lock()
		b.flushes++
		b.mu.Unlock()
	}
}

// drain returns and clears the buffer. Caller must hold b.mu.
func (b *StreamBuffer[T]) drain() []T {
	if len(b.items) == 0 {
		return nil
	}

	items := b.items
	b.items = nil

	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}

	return items
}
