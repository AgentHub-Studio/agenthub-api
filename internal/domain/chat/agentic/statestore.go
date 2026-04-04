package agentic

import "sync"

// Generic pub-sub state store.
//
// Inspired by Claude Code's state/store.ts — a minimal in-memory
// state container with getState/setState and a subscribe/unsubscribe
// listener pattern. Useful for observable configuration, session
// state, or any typed state that needs change notification.

// StateStore is a thread-safe generic state container with change
// notification. Listeners are called synchronously after each state
// mutation.
type StateStore[T comparable] struct {
	mu        sync.RWMutex
	state     T
	listeners []func()
	onChange  func(oldState, newState T)
}

// NewStateStore creates a new state store with the given initial state.
// The optional onChange callback is called on every successful state
// mutation with the old and new state values.
func NewStateStore[T comparable](initial T, onChange func(oldState, newState T)) *StateStore[T] {
	return &StateStore[T]{
		state:    initial,
		onChange: onChange,
	}
}

// GetState returns the current state.
func (s *StateStore[T]) GetState() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState updates the state using an updater function. If the new
// state is identical to the old state (==), no listeners are notified.
func (s *StateStore[T]) SetState(updater func(prev T) T) {
	s.mu.Lock()
	prev := s.state
	next := updater(prev)
	if prev == next {
		s.mu.Unlock()
		return
	}
	s.state = next
	listeners := make([]func(), len(s.listeners))
	copy(listeners, s.listeners)
	onChange := s.onChange
	s.mu.Unlock()

	if onChange != nil {
		onChange(prev, next)
	}
	for _, fn := range listeners {
		if fn != nil {
			fn()
		}
	}
}

// Subscribe registers a listener that is called on every state change.
// Returns an unsubscribe function.
func (s *StateStore[T]) Subscribe(listener func()) func() {
	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	idx := len(s.listeners) - 1
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		// Mark as nil rather than splice to avoid index shifting
		if idx < len(s.listeners) {
			s.listeners[idx] = nil
		}
	}
}
