package agentic

import (
	"context"
	"sync"
)

// Capacity-aware poll loop wakeup with signal merging.
//
// Inspired by Claude Code's capacityWake.ts — combines an outer shutdown
// signal with an internal wake-on-capacity controller. Used by poll loops
// that need to sleep when idle but wake early when work appears or when
// the system shuts down.

// CapacityWake controls a poll loop's sleep/wake cycle.
type CapacityWake struct {
	mu         sync.Mutex
	outerCtx   context.Context
	wakeCh     chan struct{}
	wakeCount  int
}

// NewCapacityWake creates a capacity wake controller.
// outerCtx is the lifecycle context (e.g., shutdown signal).
func NewCapacityWake(outerCtx context.Context) *CapacityWake {
	return &CapacityWake{
		outerCtx: outerCtx,
		wakeCh:   make(chan struct{}, 1),
	}
}

// Wake signals the poll loop to wake up immediately.
// Safe to call multiple times; extra calls are coalesced.
func (w *CapacityWake) Wake() {
	w.mu.Lock()
	w.wakeCount++
	w.mu.Unlock()

	select {
	case w.wakeCh <- struct{}{}:
	default:
		// Channel already has a pending wake signal — coalesce
	}
}

// WaitCtx returns a context that is cancelled when either:
// - The outer lifecycle context is done (shutdown), or
// - Wake() is called (work available).
// The caller should select on ctx.Done() to sleep efficiently.
func (w *CapacityWake) WaitCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(w.outerCtx)

	go func() {
		select {
		case <-w.wakeCh:
			cancel()
		case <-w.outerCtx.Done():
			cancel()
		case <-ctx.Done():
			// Caller cancelled — drain wake channel to prevent leak
			select {
			case <-w.wakeCh:
			default:
			}
		}
	}()

	return ctx, cancel
}

// Sleep blocks until either Wake() is called or the outer context is done.
// Returns true if woken by Wake(), false if by shutdown.
func (w *CapacityWake) Sleep() bool {
	select {
	case <-w.wakeCh:
		return true
	case <-w.outerCtx.Done():
		return false
	}
}

// IsDone returns true if the outer context is done (shutdown).
func (w *CapacityWake) IsDone() bool {
	return w.outerCtx.Err() != nil
}

// WakeCount returns the total number of Wake() calls.
func (w *CapacityWake) WakeCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.wakeCount
}
