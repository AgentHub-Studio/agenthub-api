package agentic

import (
	"sync"
	"time"
)

// Min-display-time throttle for value updates.
//
// Inspired by Claude Code's useMinDisplayTime — guarantees each distinct
// value stays visible for at least a minimum duration before being replaced.
// Prevents fast-cycling progress text from flickering past before it's
// readable. Unlike debounce (waits for quiet) or throttle (limits rate),
// this ensures every value gets its screen time.

// MinDisplayThrottle throttles value updates so each value is visible
// for at least MinDuration before being replaced.
type MinDisplayThrottle[T comparable] struct {
	mu          sync.Mutex
	current     T
	pending     *T
	lastShownAt time.Time
	minDuration time.Duration
	timer       *time.Timer
	onUpdate    func(T)
	closed      bool
}

// NewMinDisplayThrottle creates a throttle that holds each value for at
// least minDuration. onUpdate is called (in a separate goroutine) when
// the displayed value changes.
func NewMinDisplayThrottle[T comparable](initial T, minDuration time.Duration, onUpdate func(T)) *MinDisplayThrottle[T] {
	return &MinDisplayThrottle[T]{
		current:     initial,
		minDuration: minDuration,
		onUpdate:    onUpdate,
	}
}

// Set proposes a new value. If enough time has passed since the last
// update, the value is applied immediately. Otherwise it is queued and
// applied after the remaining minimum time elapses.
func (m *MinDisplayThrottle[T]) Set(value T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	if value == m.current {
		// Same value — no change needed
		m.pending = nil
		if m.timer != nil {
			m.timer.Stop()
			m.timer = nil
		}
		return
	}

	elapsed := time.Since(m.lastShownAt)
	if elapsed >= m.minDuration {
		m.apply(value)
		return
	}

	// Queue the value for later
	v := value
	m.pending = &v

	if m.timer != nil {
		m.timer.Stop()
	}

	remaining := m.minDuration - elapsed
	m.timer = time.AfterFunc(remaining, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.closed || m.pending == nil {
			return
		}
		m.apply(*m.pending)
		m.pending = nil
		m.timer = nil
	})
}

// Current returns the currently displayed value.
func (m *MinDisplayThrottle[T]) Current() T {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// Pending returns the queued value, if any.
func (m *MinDisplayThrottle[T]) Pending() (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending != nil {
		return *m.pending, true
	}
	var zero T
	return zero, false
}

// Flush immediately applies any pending value.
func (m *MinDisplayThrottle[T]) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	if m.pending != nil {
		m.apply(*m.pending)
		m.pending = nil
	}
}

// Close stops the throttle and discards any pending update.
func (m *MinDisplayThrottle[T]) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	m.pending = nil
}

// apply sets the current value and notifies. Caller must hold mu.
func (m *MinDisplayThrottle[T]) apply(value T) {
	m.current = value
	m.lastShownAt = time.Now()
	if m.onUpdate != nil {
		go m.onUpdate(value)
	}
}
