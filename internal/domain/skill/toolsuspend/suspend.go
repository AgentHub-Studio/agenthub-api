// Package toolsuspend defines the scaffold for tools that pause execution
// until a human (or external system) provides the data required to resume —
// e.g. an approval step inside an agent run.
//
// Full runtime integration (runner + persistence) lands in a follow-up PR;
// this package ships the type vocabulary and the resume buffer so downstream
// work has a stable import path.
package toolsuspend

import (
	"errors"
	"sync"
	"time"
)

// State is the lifecycle of a suspend-capable tool execution.
type State string

const (
	StatePending   State = "PENDING"
	StateRunning   State = "RUNNING"
	StateSuspended State = "RUNNING_SUSPENDED"
	StateResumed   State = "RESUMED"
	StateCompleted State = "COMPLETED"
	StateFailed    State = "FAILED"
)

// SuspendRequest is emitted when a tool pauses the agent loop awaiting external
// input. It is surfaced to the caller (e.g. as an SSE `input_request` event).
type SuspendRequest struct {
	ExecutionID string
	ToolName    string
	Prompt      string
	Schema      map[string]any
	CreatedAt   time.Time
}

// ResumeData is the payload supplied by the operator that unblocks execution.
type ResumeData struct {
	ExecutionID string
	Data        map[string]any
}

// ErrNotSuspended is returned by Resume when the execution is not waiting.
var ErrNotSuspended = errors.New("execution is not suspended")

// ResumeBuffer stores suspended executions keyed by ID. In-memory only —
// production impl will back this with a repository.
type ResumeBuffer struct {
	mu       sync.Mutex
	requests map[string]SuspendRequest
	waiters  map[string]chan ResumeData
}

func NewResumeBuffer() *ResumeBuffer {
	return &ResumeBuffer{
		requests: make(map[string]SuspendRequest),
		waiters:  make(map[string]chan ResumeData),
	}
}

// Suspend registers a pending request and returns the channel the tool body
// should receive on. The caller unblocks the tool via Resume(...) with the
// same ExecutionID.
func (b *ResumeBuffer) Suspend(req SuspendRequest) <-chan ResumeData {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan ResumeData, 1)
	b.requests[req.ExecutionID] = req
	b.waiters[req.ExecutionID] = ch
	return ch
}

// Resume delivers data to the waiting tool. Returns ErrNotSuspended if no
// matching request is registered.
func (b *ResumeBuffer) Resume(data ResumeData) error {
	b.mu.Lock()
	ch, ok := b.waiters[data.ExecutionID]
	if ok {
		delete(b.waiters, data.ExecutionID)
		delete(b.requests, data.ExecutionID)
	}
	b.mu.Unlock()
	if !ok {
		return ErrNotSuspended
	}
	ch <- data
	close(ch)
	return nil
}

// Pending lists the suspended requests (for UI / debug).
func (b *ResumeBuffer) Pending() []SuspendRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]SuspendRequest, 0, len(b.requests))
	for _, r := range b.requests {
		out = append(out, r)
	}
	return out
}
