package agentic

import (
	"sync"
	"time"
)

// File change detector with debounced batch reload.
//
// Inspired by Claude Code's skillChangeDetector — watches for change
// events and debounces rapid-fire notifications into a single batch
// reload. Prevents cascading reloads when many files change at once
// (e.g., git operations, deployments). Emits a signal with all changed
// paths once the debounce window expires.

// ChangeDetectorConfig configures the change detector.
type ChangeDetectorConfig struct {
	// DebounceDuration is how long to wait after the last change before
	// firing the reload. Default 300ms.
	DebounceDuration time.Duration
}

// ChangeHandler is called with the batch of changed paths after debounce.
type ChangeHandler func(changedPaths []string)

// ChangeDetector batches rapid change events into a single callback.
type ChangeDetector struct {
	mu         sync.Mutex
	config     ChangeDetectorConfig
	pending    map[string]struct{}
	timer      *time.Timer
	handler    ChangeHandler
	disposed   bool
	changeCount int
}

// NewChangeDetector creates a detector with the given config and handler.
func NewChangeDetector(config ChangeDetectorConfig, handler ChangeHandler) *ChangeDetector {
	if config.DebounceDuration <= 0 {
		config.DebounceDuration = 300 * time.Millisecond
	}
	return &ChangeDetector{
		config:  config,
		pending: make(map[string]struct{}),
		handler: handler,
	}
}

// Notify reports a change at the given path. Multiple rapid calls are
// debounced into a single handler invocation.
func (d *ChangeDetector) Notify(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.disposed {
		return
	}

	d.pending[path] = struct{}{}
	d.changeCount++

	// Reset debounce timer
	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.config.DebounceDuration, func() {
		d.flush()
	})
}

// NotifyBatch reports multiple changes at once.
func (d *ChangeDetector) NotifyBatch(paths []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.disposed {
		return
	}

	for _, p := range paths {
		d.pending[p] = struct{}{}
		d.changeCount++
	}

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.config.DebounceDuration, func() {
		d.flush()
	})
}

// Flush immediately fires the handler with any pending paths.
func (d *ChangeDetector) Flush() {
	d.flush()
}

// PendingCount returns the number of unique paths waiting to be flushed.
func (d *ChangeDetector) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

// TotalChanges returns the total number of change notifications received.
func (d *ChangeDetector) TotalChanges() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.changeCount
}

// Dispose stops the detector and discards any pending changes.
func (d *ChangeDetector) Dispose() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.disposed = true
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.pending = make(map[string]struct{})
}

func (d *ChangeDetector) flush() {
	d.mu.Lock()
	if len(d.pending) == 0 || d.disposed {
		d.mu.Unlock()
		return
	}
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	paths := make([]string, 0, len(d.pending))
	for p := range d.pending {
		paths = append(paths, p)
	}
	d.pending = make(map[string]struct{})
	handler := d.handler
	d.mu.Unlock()

	if handler != nil {
		handler(paths)
	}
}
