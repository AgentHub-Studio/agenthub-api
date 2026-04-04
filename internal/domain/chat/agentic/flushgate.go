package agentic

import (
	"sync"
)

// Message ordering gate for async initialization.
//
// Inspired by Claude Code's flushGate.ts — prevents message interleaving
// during initial data flush. While the gate is active, new items are
// queued. When the gate ends, queued items are returned for draining.

// FlushGateState represents the gate's current state.
type FlushGateState int

const (
	// GateInactive means the gate is not active (pass-through).
	GateInactive FlushGateState = iota
	// GateActive means the gate is collecting items.
	GateActive
	// GateDeactivated means the gate was cleared without draining.
	GateDeactivated
)

// FlushGate queues items while active, releases them on end.
type FlushGate[T any] struct {
	mu    sync.Mutex
	state FlushGateState
	queue []T
}

// NewFlushGate creates a new inactive flush gate.
func NewFlushGate[T any]() *FlushGate[T] {
	return &FlushGate[T]{state: GateInactive}
}

// Start activates the gate. Items sent while active are queued.
func (g *FlushGate[T]) Start() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state = GateActive
	g.queue = nil
}

// Send attempts to queue an item if the gate is active.
// Returns true if the item was queued (gate active), false if passed through.
func (g *FlushGate[T]) Send(item T) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state != GateActive {
		return false
	}

	g.queue = append(g.queue, item)
	return true
}

// End deactivates the gate and returns all queued items for draining.
func (g *FlushGate[T]) End() []T {
	g.mu.Lock()
	defer g.mu.Unlock()

	queued := g.queue
	g.queue = nil
	g.state = GateInactive
	return queued
}

// Deactivate clears the gate without returning items.
// Used when the transport is replaced (items are discarded, not drained).
func (g *FlushGate[T]) Deactivate() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.queue = nil
	g.state = GateDeactivated
}

// IsActive returns true if the gate is currently collecting items.
func (g *FlushGate[T]) IsActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state == GateActive
}

// State returns the current gate state.
func (g *FlushGate[T]) State() FlushGateState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state
}

// QueueLen returns the number of items currently queued.
func (g *FlushGate[T]) QueueLen() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.queue)
}
