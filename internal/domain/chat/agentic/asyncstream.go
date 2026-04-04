package agentic

import (
	"fmt"
	"sync"
)

// Async stream with backpressure and error propagation.
//
// Inspired by Claude Code's stream.ts — provides a channel-based
// stream abstraction with explicit done/error signaling and single-use
// iteration guarantee.

// StreamItem wraps either a value or an error.
type StreamItem[T any] struct {
	Value T
	Err   error
	Done  bool
}

// Stream is a single-use async iterator with backpressure.
type Stream[T any] struct {
	mu       sync.Mutex
	ch       chan StreamItem[T]
	closed   bool
	iterated bool
	bufSize  int
}

// NewStream creates a stream with the given buffer size.
func NewStream[T any](bufSize int) *Stream[T] {
	if bufSize <= 0 {
		bufSize = 16
	}
	return &Stream[T]{
		ch:      make(chan StreamItem[T], bufSize),
		bufSize: bufSize,
	}
}

// Enqueue adds a value to the stream. Blocks if the buffer is full.
// Returns false if the stream is closed.
func (s *Stream[T]) Enqueue(value T) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	s.ch <- StreamItem[T]{Value: value}
	return true
}

// Error sends an error and closes the stream.
func (s *Stream[T]) Error(err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	s.ch <- StreamItem[T]{Err: err}
	close(s.ch)
}

// Done signals the stream is complete.
func (s *Stream[T]) Done() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	s.ch <- StreamItem[T]{Done: true}
	close(s.ch)
}

// Chan returns the read channel for iteration.
// Panics if called more than once (single-use guarantee).
func (s *Stream[T]) Chan() <-chan StreamItem[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.iterated {
		panic("stream: Chan() called more than once (single-use)")
	}
	s.iterated = true
	return s.ch
}

// IsClosed returns whether the stream has been closed.
func (s *Stream[T]) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Collect reads all items from the stream into a slice.
// Returns collected values and any error encountered.
func (s *Stream[T]) Collect() ([]T, error) {
	var result []T
	for item := range s.Chan() {
		if item.Err != nil {
			return result, item.Err
		}
		if item.Done {
			break
		}
		result = append(result, item.Value)
	}
	return result, nil
}

// Transform creates a new stream by applying a function to each item.
func Transform[T, U any](input *Stream[T], fn func(T) (U, error)) *Stream[U] {
	output := NewStream[U](input.bufSize)

	go func() {
		for item := range input.Chan() {
			if item.Err != nil {
				output.Error(item.Err)
				return
			}
			if item.Done {
				output.Done()
				return
			}
			result, err := fn(item.Value)
			if err != nil {
				output.Error(fmt.Errorf("transform: %w", err))
				return
			}
			if !output.Enqueue(result) {
				return
			}
		}
		output.Done()
	}()

	return output
}
