package agentic

import (
	"context"
	"sync"
)

// Sequential execution queue for async operations.
//
// Inspired by Claude Code's sequential.ts — ensures that concurrent calls
// to a wrapped function execute one-at-a-time in the order received.
// Preserves return values and error propagation per call.
// Critical for preventing race conditions in file mutations and DB writes.

// SequentialFunc wraps a function so that concurrent invocations
// are serialized. Each call waits for the previous one to complete
// before starting.
//
// Usage:
//
//	writeFile := SequentialFunc(func(ctx context.Context, path string) error {
//	    return os.WriteFile(path, data, 0o644)
//	})
//
//	// These run one-at-a-time even if called concurrently:
//	go writeFile(ctx, "a.txt")
//	go writeFile(ctx, "b.txt")
func SequentialFunc[T any](fn func(context.Context, T) error) func(context.Context, T) error {
	var mu sync.Mutex
	return func(ctx context.Context, arg T) error {
		mu.Lock()
		defer mu.Unlock()
		return fn(ctx, arg)
	}
}

// SequentialFuncResult wraps a function that returns a value and error.
func SequentialFuncResult[T any, R any](fn func(context.Context, T) (R, error)) func(context.Context, T) (R, error) {
	var mu sync.Mutex
	return func(ctx context.Context, arg T) (R, error) {
		mu.Lock()
		defer mu.Unlock()
		return fn(ctx, arg)
	}
}

// SequentialQueue provides an explicit queue for sequential execution
// with cancellation support. Unlike SequentialFunc which blocks callers,
// this queues work items and processes them in order.
type SequentialQueue struct {
	mu      sync.Mutex
	queue   []queueItem
	running bool
	done    chan struct{}
}

type queueItem struct {
	fn   func(context.Context) error
	ctx  context.Context
	errC chan error
}

// NewSequentialQueue creates a new sequential execution queue.
func NewSequentialQueue() *SequentialQueue {
	return &SequentialQueue{
		done: make(chan struct{}),
	}
}

// Enqueue adds a function to the queue and returns a channel that
// will receive the error result when execution completes.
func (sq *SequentialQueue) Enqueue(ctx context.Context, fn func(context.Context) error) <-chan error {
	errC := make(chan error, 1)

	sq.mu.Lock()
	sq.queue = append(sq.queue, queueItem{fn: fn, ctx: ctx, errC: errC})
	if !sq.running {
		sq.running = true
		go sq.processLoop()
	}
	sq.mu.Unlock()

	return errC
}

// EnqueueWait adds a function and blocks until it completes.
func (sq *SequentialQueue) EnqueueWait(ctx context.Context, fn func(context.Context) error) error {
	errC := sq.Enqueue(ctx, fn)
	select {
	case err := <-errC:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Pending returns the number of items waiting in the queue.
func (sq *SequentialQueue) Pending() int {
	sq.mu.Lock()
	defer sq.mu.Unlock()
	return len(sq.queue)
}

// processLoop runs queued items one at a time.
func (sq *SequentialQueue) processLoop() {
	for {
		sq.mu.Lock()
		if len(sq.queue) == 0 {
			sq.running = false
			sq.mu.Unlock()
			return
		}
		item := sq.queue[0]
		sq.queue = sq.queue[1:]
		sq.mu.Unlock()

		// Skip if context already cancelled.
		var err error
		if item.ctx.Err() != nil {
			err = item.ctx.Err()
		} else {
			err = item.fn(item.ctx)
		}

		item.errC <- err
		close(item.errC)
	}
}
