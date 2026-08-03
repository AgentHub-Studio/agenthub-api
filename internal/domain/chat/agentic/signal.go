package agentic

import (
	"sync"
	"time"
)

// --- Signal: typed pub/sub ---

// SignalListener is a callback function invoked when the signal fires.
type SignalListener[T any] func(T)

// Signal is a lightweight typed event emitter with subscribe/emit/clear.
// No stored state — purely "something happened" notifications.
// Thread-safe for concurrent subscribe/emit/clear from multiple goroutines.
//
// Inspired by Claude Code's createSignal in utils/signal.ts.
type Signal[T any] struct {
	mu        sync.RWMutex
	listeners map[uint64]SignalListener[T]
	nextID    uint64
}

// NewSignal creates a new signal instance.
func NewSignal[T any]() *Signal[T] {
	return &Signal[T]{
		listeners: make(map[uint64]SignalListener[T]),
	}
}

// Subscribe registers a listener and returns an unsubscribe function.
// The unsubscribe function is safe to call multiple times.
func (s *Signal[T]) Subscribe(fn SignalListener[T]) func() {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.listeners[id] = fn
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		delete(s.listeners, id)
		s.mu.Unlock()
	}
}

// Emit invokes all listeners with the given value.
// Listeners execute synchronously in arbitrary order.
func (s *Signal[T]) Emit(value T) {
	s.mu.RLock()
	snapshot := make([]SignalListener[T], 0, len(s.listeners))
	for _, fn := range s.listeners {
		snapshot = append(snapshot, fn)
	}
	s.mu.RUnlock()

	for _, fn := range snapshot {
		fn(value)
	}
}

// Len returns the current number of subscribers.
func (s *Signal[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.listeners)
}

// Clear removes all listeners.
func (s *Signal[T]) Clear() {
	s.mu.Lock()
	s.listeners = make(map[uint64]SignalListener[T])
	s.mu.Unlock()
}

// --- BufferedWriter: batched writes with flush interval ---

// WriteFn is a function that writes content (e.g., to a file, stream, or channel).
type WriteFn func(content string)

// BufferedWriterConfig configures the buffered writer.
type BufferedWriterConfig struct {
	// WriteFn is the function called to write batched content.
	WriteFn WriteFn
	// FlushInterval is the maximum time between automatic flushes.
	// Zero means no automatic flushing (manual only). Default: 1 second.
	FlushInterval time.Duration
	// MaxBufferSize is the maximum number of writes to buffer before flushing.
	// Zero means no count limit. Default: 100.
	MaxBufferSize int
	// MaxBufferBytes is the maximum byte count before flushing.
	// Zero means no byte limit.
	MaxBufferBytes int
	// ImmediateMode bypasses buffering entirely — each write goes directly to WriteFn.
	ImmediateMode bool
}

// BufferedWriter batches writes with configurable flush interval and size limits.
// When the buffer exceeds limits, it flushes immediately.
// Thread-safe for concurrent writes.
//
// Inspired by Claude Code's createBufferedWriter in utils/bufferedWriter.ts.
type BufferedWriter struct {
	config      BufferedWriterConfig
	mu          sync.Mutex
	buffer      []string
	bufferBytes int
	timer       *time.Timer
	disposed    bool
}

// NewBufferedWriter creates a new buffered writer.
func NewBufferedWriter(config BufferedWriterConfig) *BufferedWriter {
	if config.FlushInterval == 0 {
		config.FlushInterval = time.Second
	}
	if config.MaxBufferSize == 0 {
		config.MaxBufferSize = 100
	}
	return &BufferedWriter{
		config: config,
	}
}

// Write adds content to the buffer. If the buffer exceeds size/byte limits,
// it flushes immediately. In immediate mode, writes directly to WriteFn.
func (w *BufferedWriter) Write(content string) {
	if w.config.ImmediateMode {
		w.config.WriteFn(content)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.disposed {
		return
	}

	w.buffer = append(w.buffer, content)
	w.bufferBytes += len(content)

	// Check if buffer exceeds limits.
	shouldFlush := w.config.MaxBufferSize > 0 && len(w.buffer) >= w.config.MaxBufferSize

	if w.config.MaxBufferBytes > 0 && w.bufferBytes >= w.config.MaxBufferBytes {
		shouldFlush = true
	}

	if shouldFlush {
		w.flushLocked()
	} else {
		w.scheduleFlushLocked()
	}
}

// Flush writes all buffered content immediately.
func (w *BufferedWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushLocked()
}

// Dispose flushes remaining content and prevents future writes.
func (w *BufferedWriter) Dispose() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushLocked()
	w.disposed = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}

// Pending returns the number of buffered writes not yet flushed.
func (w *BufferedWriter) Pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buffer)
}

func (w *BufferedWriter) flushLocked() {
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}

	if len(w.buffer) == 0 {
		return
	}

	// Detach buffer.
	batch := w.buffer
	w.buffer = nil
	w.bufferBytes = 0

	// Write concatenated content.
	var total string
	for _, s := range batch {
		total += s
	}
	w.config.WriteFn(total)
}

func (w *BufferedWriter) scheduleFlushLocked() {
	if w.timer != nil {
		return
	}
	w.timer = time.AfterFunc(w.config.FlushInterval, func() {
		w.Flush()
	})
}
