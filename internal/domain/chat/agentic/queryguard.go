package agentic

import (
	"sync"
	"sync/atomic"
)

// QueryStatus represents the lifecycle state of a query in the QueryGuard FSM.
type QueryStatus string

const (
	// QueryIdle means no query is active or pending.
	QueryIdle QueryStatus = "idle"
	// QueryDispatching means a query has been reserved but not yet started.
	// This prevents new queries from being accepted while one is being prepared.
	QueryDispatching QueryStatus = "dispatching"
	// QueryRunning means a query is actively executing.
	QueryRunning QueryStatus = "running"
)

// QueryGuard is a state machine that prevents concurrent query execution.
// It tracks three states (idle → dispatching → running) and uses generation
// numbers to invalidate stale cleanup from cancelled queries.
//
// Thread-safe for concurrent access from multiple goroutines.
//
// State transitions:
//   idle → dispatching (Reserve)
//   dispatching → running (TryStart)
//   idle → running (TryStart, direct start)
//   running → idle (End with matching generation)
//   dispatching → idle (CancelReservation)
//   any non-idle → idle (ForceEnd, increments generation)
//
// Inspired by Claude Code's QueryGuard in utils/QueryGuard.ts.
type QueryGuard struct {
	mu         sync.Mutex
	status     QueryStatus
	generation int64
	onChange   func(status QueryStatus)
}

// NewQueryGuard creates a new guard in the idle state.
// The optional onChange callback is invoked on every state transition.
func NewQueryGuard(onChange func(QueryStatus)) *QueryGuard {
	return &QueryGuard{
		status:   QueryIdle,
		onChange: onChange,
	}
}

// Status returns the current state.
func (g *QueryGuard) Status() QueryStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.status
}

// IsActive returns true if the guard is in dispatching or running state.
func (g *QueryGuard) IsActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.status != QueryIdle
}

// Generation returns the current generation number.
func (g *QueryGuard) Generation() int64 {
	return atomic.LoadInt64(&g.generation)
}

// Reserve transitions from idle to dispatching. Returns true if successful,
// false if the guard is not idle (another query is already active).
// Use this to "claim" the guard before async preparation work.
func (g *QueryGuard) Reserve() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.status != QueryIdle {
		return false
	}
	g.status = QueryDispatching
	g.notify()
	return true
}

// CancelReservation transitions from dispatching back to idle.
// No-op if not in dispatching state.
func (g *QueryGuard) CancelReservation() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.status == QueryDispatching {
		g.status = QueryIdle
		g.notify()
	}
}

// TryStart transitions to running and returns the new generation number.
// Returns (generation, true) on success, (0, false) if already running
// (concurrent query guard).
//
// Can be called from idle (direct start) or dispatching (after Reserve).
func (g *QueryGuard) TryStart() (int64, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.status == QueryRunning {
		return 0, false
	}

	g.status = QueryRunning
	g.generation++
	gen := g.generation
	g.notify()
	return gen, true
}

// End transitions from running to idle if the generation matches.
// Returns true if this was the current generation and cleanup should proceed.
// Returns false if the generation is stale (query was force-ended or
// superseded by a newer query), meaning cleanup should be skipped.
func (g *QueryGuard) End(generation int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.status != QueryRunning || g.generation != generation {
		return false
	}

	g.status = QueryIdle
	g.notify()
	return true
}

// ForceEnd unconditionally transitions to idle and increments the generation,
// invalidating any stale cleanup from the previous query. Use this when
// cancelling or aborting a query externally.
func (g *QueryGuard) ForceEnd() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.status == QueryIdle {
		return
	}

	g.status = QueryIdle
	g.generation++
	g.notify()
}

func (g *QueryGuard) notify() {
	if g.onChange != nil {
		g.onChange(g.status)
	}
}
