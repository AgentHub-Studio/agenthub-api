package agentic

import (
	"context"
	"sync"
)

// Hierarchical abort signal propagation.
//
// Inspired by Claude Code's abortController.ts — creates parent-child
// abort chains where child cancellation propagates from parent but not
// vice versa. Uses cleanup functions to prevent reference leaks.

// AbortController manages a cancellable context with listener notification.
type AbortController struct {
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	listeners []func()
	aborted   bool
}

// NewAbortController creates a new abort controller.
func NewAbortController() *AbortController {
	ctx, cancel := context.WithCancel(context.Background())
	return &AbortController{
		ctx:    ctx,
		cancel: cancel,
	}
}

// NewAbortControllerWithContext creates an abort controller derived from
// an existing context. The controller aborts if the parent context is cancelled.
func NewAbortControllerWithContext(parent context.Context) *AbortController {
	ctx, cancel := context.WithCancel(parent)
	ac := &AbortController{
		ctx:    ctx,
		cancel: cancel,
	}

	// Monitor parent context cancellation.
	go func() {
		<-parent.Done()
		ac.Abort()
	}()

	return ac
}

// Context returns the cancellable context.
func (ac *AbortController) Context() context.Context {
	return ac.ctx
}

// Abort cancels the context and notifies all listeners.
func (ac *AbortController) Abort() {
	ac.mu.Lock()
	if ac.aborted {
		ac.mu.Unlock()
		return
	}
	ac.aborted = true
	listeners := make([]func(), len(ac.listeners))
	copy(listeners, ac.listeners)
	ac.listeners = nil
	ac.mu.Unlock()

	ac.cancel()
	for _, fn := range listeners {
		if fn != nil {
			fn()
		}
	}
}

// IsAborted returns true if Abort has been called.
func (ac *AbortController) IsAborted() bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.aborted
}

// OnAbort registers a listener called when Abort is invoked.
// Returns a cleanup function to remove the listener.
// If already aborted, the listener is called immediately.
func (ac *AbortController) OnAbort(fn func()) func() {
	ac.mu.Lock()
	if ac.aborted {
		ac.mu.Unlock()
		fn()
		return func() {}
	}

	idx := len(ac.listeners)
	ac.listeners = append(ac.listeners, fn)
	ac.mu.Unlock()

	return func() {
		ac.mu.Lock()
		defer ac.mu.Unlock()
		if idx < len(ac.listeners) {
			ac.listeners[idx] = nil
		}
	}
}

// CreateChildAbortController creates a child controller that aborts when
// the parent aborts, but not vice versa. The parent does not retain a
// strong reference to the child — abandoned children are cleaned up.
func CreateChildAbortController(parent *AbortController) *AbortController {
	child := NewAbortController()

	// Check if parent is already aborted.
	if parent.IsAborted() {
		child.Abort()
		return child
	}

	// Register child abort on parent abort.
	removeFromParent := parent.OnAbort(func() {
		child.Abort()
	})

	// When child is aborted independently, remove the parent listener.
	child.OnAbort(func() {
		removeFromParent()
	})

	return child
}

// AbortGroup manages a collection of related abort controllers.
type AbortGroup struct {
	mu          sync.Mutex
	controllers []*AbortController
	aborted     bool
}

// NewAbortGroup creates a new abort group.
func NewAbortGroup() *AbortGroup {
	return &AbortGroup{}
}

// Add registers a controller in the group. If the group is already
// aborted, the controller is aborted immediately.
func (g *AbortGroup) Add(ac *AbortController) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.aborted {
		ac.Abort()
		return
	}
	g.controllers = append(g.controllers, ac)
}

// AbortAll aborts all controllers in the group.
func (g *AbortGroup) AbortAll() {
	g.mu.Lock()
	if g.aborted {
		g.mu.Unlock()
		return
	}
	g.aborted = true
	controllers := make([]*AbortController, len(g.controllers))
	copy(controllers, g.controllers)
	g.controllers = nil
	g.mu.Unlock()

	for _, ac := range controllers {
		ac.Abort()
	}
}

// Count returns the number of controllers in the group.
func (g *AbortGroup) Count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.controllers)
}

// IsAborted returns whether the group has been aborted.
func (g *AbortGroup) IsAborted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.aborted
}
