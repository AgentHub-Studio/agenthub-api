package agentic

// CircularBuffer is a fixed-capacity ring buffer that automatically evicts the
// oldest items when full. Thread-safe operations are NOT provided — callers
// must synchronize externally if needed (the buffer is typically owned by a
// single goroutine or protected by an enclosing struct's mutex).
//
// Inspired by Claude Code's CircularBuffer in utils/CircularBuffer.ts.
type CircularBuffer[T any] struct {
	buffer   []T
	head     int // next write position
	size     int // current number of items
	capacity int
}

// NewCircularBuffer creates a buffer with the given fixed capacity.
// Panics if capacity <= 0.
func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	if capacity <= 0 {
		panic("CircularBuffer: capacity must be > 0")
	}
	return &CircularBuffer[T]{
		buffer:   make([]T, capacity),
		capacity: capacity,
	}
}

// Add inserts an item into the buffer. If the buffer is full, the oldest item
// is overwritten.
func (b *CircularBuffer[T]) Add(item T) {
	b.buffer[b.head] = item
	b.head = (b.head + 1) % b.capacity
	if b.size < b.capacity {
		b.size++
	}
}

// AddAll inserts multiple items in order.
func (b *CircularBuffer[T]) AddAll(items []T) {
	for _, item := range items {
		b.Add(item)
	}
}

// GetRecent returns the last n items from oldest to newest.
// If n > Len(), returns all items.
func (b *CircularBuffer[T]) GetRecent(n int) []T {
	if n <= 0 || b.size == 0 {
		return nil
	}
	if n > b.size {
		n = b.size
	}

	result := make([]T, n)
	// Start position: n items back from the current write position.
	start := (b.head - n + b.capacity) % b.capacity
	for i := 0; i < n; i++ {
		result[i] = b.buffer[(start+i)%b.capacity]
	}
	return result
}

// ToArray returns all items from oldest to newest.
func (b *CircularBuffer[T]) ToArray() []T {
	if b.size == 0 {
		return nil
	}
	return b.GetRecent(b.size)
}

// Len returns the current number of items in the buffer.
func (b *CircularBuffer[T]) Len() int {
	return b.size
}

// Cap returns the fixed capacity of the buffer.
func (b *CircularBuffer[T]) Cap() int {
	return b.capacity
}

// Clear resets the buffer to empty state.
func (b *CircularBuffer[T]) Clear() {
	var zero T
	for i := range b.buffer {
		b.buffer[i] = zero
	}
	b.head = 0
	b.size = 0
}

// IsFull returns true when the buffer is at capacity.
func (b *CircularBuffer[T]) IsFull() bool {
	return b.size == b.capacity
}

// Peek returns the oldest item without removing it.
// Returns the zero value and false if the buffer is empty.
func (b *CircularBuffer[T]) Peek() (T, bool) {
	var zero T
	if b.size == 0 {
		return zero, false
	}
	start := (b.head - b.size + b.capacity) % b.capacity
	return b.buffer[start], true
}

// PeekNewest returns the most recently added item without removing it.
// Returns the zero value and false if the buffer is empty.
func (b *CircularBuffer[T]) PeekNewest() (T, bool) {
	var zero T
	if b.size == 0 {
		return zero, false
	}
	newest := (b.head - 1 + b.capacity) % b.capacity
	return b.buffer[newest], true
}
