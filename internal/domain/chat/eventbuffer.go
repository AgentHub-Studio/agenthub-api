package chat

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const (
	// DefaultEventBufferSize is the default capacity of the circular event buffer.
	DefaultEventBufferSize = 1000
)

// BufferedEvent is a RunEvent annotated with a sequential ID.
type BufferedEvent struct {
	// ID is the monotonic sequence number within the run.
	ID    uint64
	Event RunEvent
}

// FormatSSEID returns the SSE id field value: "{runID}:{sequence}".
func (be BufferedEvent) FormatSSEID(runID string) string {
	return fmt.Sprintf("%s:%d", runID, be.ID)
}

// ParseSSEID extracts runID and sequence from an SSE id string.
// Returns ("", 0, false) if the format is invalid.
func ParseSSEID(sseID string) (runID string, seq uint64, ok bool) {
	idx := strings.LastIndex(sseID, ":")
	if idx <= 0 || idx == len(sseID)-1 {
		return "", 0, false
	}
	runID = sseID[:idx]
	n, err := strconv.ParseUint(sseID[idx+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return runID, n, true
}

// EventBuffer is a thread-safe circular buffer that stores recent RunEvents
// with monotonic sequence IDs. It supports replay from a given sequence number
// for SSE reconnection via Last-Event-ID.
type EventBuffer struct {
	mu       sync.RWMutex
	buf      []BufferedEvent
	capacity int
	nextSeq  uint64
	// start is the index in buf where the oldest event lives.
	start int
	// count is the number of events currently in the buffer.
	count int
	// done indicates the run has completed and no more events will arrive.
	done bool
}

// NewEventBuffer creates a buffer with the given capacity.
// If capacity <= 0, DefaultEventBufferSize is used.
func NewEventBuffer(capacity int) *EventBuffer {
	if capacity <= 0 {
		capacity = DefaultEventBufferSize
	}
	return &EventBuffer{
		buf:      make([]BufferedEvent, capacity),
		capacity: capacity,
		nextSeq:  1, // IDs start at 1.
	}
}

// Append adds an event to the buffer and returns its sequence ID.
func (b *EventBuffer) Append(ev RunEvent) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	seq := b.nextSeq
	b.nextSeq++

	idx := (b.start + b.count) % b.capacity
	b.buf[idx] = BufferedEvent{ID: seq, Event: ev}

	if b.count < b.capacity {
		b.count++
	} else {
		// Buffer full — advance start, dropping the oldest event.
		b.start = (b.start + 1) % b.capacity
	}

	return seq
}

// MarkDone signals that no more events will be appended.
func (b *EventBuffer) MarkDone() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done = true
}

// IsDone returns true if the run has completed.
func (b *EventBuffer) IsDone() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.done
}

// EventsSince returns all buffered events with ID > afterSeq.
// If afterSeq is older than the oldest buffered event, returns (nil, false)
// indicating overflow (the client missed events that were evicted).
// If afterSeq == 0, all buffered events are returned.
func (b *EventBuffer) EventsSince(afterSeq uint64) (events []BufferedEvent, ok bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil, true
	}

	oldestSeq := b.buf[b.start].ID

	// afterSeq == 0 means "give me everything".
	if afterSeq == 0 {
		result := make([]BufferedEvent, b.count)
		for i := 0; i < b.count; i++ {
			result[i] = b.buf[(b.start+i)%b.capacity]
		}
		return result, true
	}

	// If the requested seq is older than what we have, it's an overflow.
	if afterSeq < oldestSeq {
		return nil, false
	}

	// Find events with ID > afterSeq.
	var result []BufferedEvent
	for i := 0; i < b.count; i++ {
		ev := b.buf[(b.start+i)%b.capacity]
		if ev.ID > afterSeq {
			result = append(result, ev)
		}
	}

	return result, true
}

// Len returns the number of events currently buffered.
func (b *EventBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// OldestSeq returns the sequence ID of the oldest buffered event,
// or 0 if the buffer is empty.
func (b *EventBuffer) OldestSeq() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.count == 0 {
		return 0
	}
	return b.buf[b.start].ID
}

// NewestSeq returns the sequence ID of the newest buffered event,
// or 0 if the buffer is empty.
func (b *EventBuffer) NewestSeq() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.count == 0 {
		return 0
	}
	return b.nextSeq - 1
}

// RunEventBufferRegistry manages EventBuffers for active runs, keyed by runID.
// A runID is generated when a run starts and used as the SSE id prefix.
type RunEventBufferRegistry struct {
	mu      sync.RWMutex
	buffers map[string]*EventBuffer
}

// NewRunEventBufferRegistry creates a new registry.
func NewRunEventBufferRegistry() *RunEventBufferRegistry {
	return &RunEventBufferRegistry{
		buffers: make(map[string]*EventBuffer),
	}
}

// GetOrCreate returns the buffer for the given runID, creating one if absent.
func (r *RunEventBufferRegistry) GetOrCreate(runID string, capacity int) *EventBuffer {
	r.mu.Lock()
	defer r.mu.Unlock()

	if buf, ok := r.buffers[runID]; ok {
		return buf
	}
	buf := NewEventBuffer(capacity)
	r.buffers[runID] = buf
	return buf
}

// Get returns the buffer for the given runID, or nil if not found.
func (r *RunEventBufferRegistry) Get(runID string) *EventBuffer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.buffers[runID]
}

// Remove deletes the buffer for the given runID.
func (r *RunEventBufferRegistry) Remove(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buffers, runID)
}

// Len returns the number of active buffers.
func (r *RunEventBufferRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.buffers)
}
