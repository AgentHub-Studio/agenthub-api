package agentic

import (
	"context"
	"sync"
)

// Centralized shutdown/cleanup management.
//
// Inspired by Claude Code's cleanupRegistry.ts — a dependency-free
// registry for cleanup handlers. Any module can register a cleanup
// function; all are executed in parallel during shutdown.

// CleanupFunc is a function to be called during shutdown.
type CleanupFunc func(ctx context.Context) error

// CleanupRegistry tracks cleanup handlers for graceful shutdown.
type CleanupRegistry struct {
	mu       sync.Mutex
	handlers map[int]CleanupFunc
	counter  int
	closed   bool
}

// NewCleanupRegistry creates a new cleanup registry.
func NewCleanupRegistry() *CleanupRegistry {
	return &CleanupRegistry{
		handlers: make(map[int]CleanupFunc),
	}
}

// Register adds a cleanup handler and returns an unregister function.
func (r *CleanupRegistry) Register(fn CleanupFunc) func() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return func() {}
	}

	r.counter++
	id := r.counter
	r.handlers[id] = fn

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.handlers, id)
	}
}

// RunAll executes all registered cleanup handlers in parallel.
// Returns a slice of errors from handlers that failed.
// After RunAll, the registry is closed and no new handlers can be registered.
func (r *CleanupRegistry) RunAll(ctx context.Context) []error {
	r.mu.Lock()
	r.closed = true
	handlers := make([]CleanupFunc, 0, len(r.handlers))
	for _, fn := range r.handlers {
		handlers = append(handlers, fn)
	}
	r.handlers = make(map[int]CleanupFunc)
	r.mu.Unlock()

	if len(handlers) == 0 {
		return nil
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		errors []error
	)

	for _, fn := range handlers {
		wg.Add(1)
		go func(f CleanupFunc) {
			defer wg.Done()
			if err := f(ctx); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(fn)
	}

	wg.Wait()
	return errors
}

// Count returns the number of registered handlers.
func (r *CleanupRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.handlers)
}

// IsClosed returns whether RunAll has been called.
func (r *CleanupRegistry) IsClosed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// Reset clears all handlers and reopens the registry.
func (r *CleanupRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = make(map[int]CleanupFunc)
	r.closed = false
}
