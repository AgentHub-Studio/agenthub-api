package agentic

import (
	"sync"
)

// Priority-based command queue.
//
// Inspired by Claude Code's messageQueueManager.ts — manages queued
// commands with three priority levels (now, next, later). Commands
// within the same priority level are processed in FIFO order.

// QueuePriority defines the urgency level of a queued command.
type QueuePriority int

const (
	// PriorityNow is the highest priority — processed first.
	PriorityNow QueuePriority = iota
	// PriorityNext is user-input priority — processed after "now".
	PriorityNext
	// PriorityLater is the lowest priority (e.g., notifications).
	PriorityLater
)

// String returns a human-readable priority label.
func (p QueuePriority) String() string {
	switch p {
	case PriorityNow:
		return "now"
	case PriorityNext:
		return "next"
	case PriorityLater:
		return "later"
	default:
		return "unknown"
	}
}

// QueuedCommand represents a command waiting in the queue.
type QueuedCommand struct {
	// ID uniquely identifies this command.
	ID string `json:"id"`
	// Priority determines processing order.
	Priority QueuePriority `json:"priority"`
	// Content is the command text.
	Content string `json:"content"`
	// Source indicates the origin (e.g., "user", "task", "system").
	Source string `json:"source,omitempty"`
	// Metadata holds arbitrary extra data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Editable indicates whether the command can be edited before execution.
	Editable bool `json:"editable,omitempty"`
	// Visible indicates whether the command shows up in queue previews.
	Visible bool `json:"visible,omitempty"`
}

// CommandQueue is a thread-safe priority queue for commands.
type CommandQueue struct {
	mu        sync.RWMutex
	commands  []QueuedCommand
	listeners []func()
}

// NewCommandQueue creates a new empty command queue.
func NewCommandQueue() *CommandQueue {
	return &CommandQueue{}
}

// Enqueue adds a command to the queue. If priority is unset, defaults to PriorityNext.
func (q *CommandQueue) Enqueue(cmd QueuedCommand) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.commands = append(q.commands, cmd)
	q.notifyLocked()
}

// Dequeue removes and returns the highest-priority command.
// Within the same priority, FIFO order is respected.
// Returns the command and true, or a zero-value and false if empty.
func (q *CommandQueue) Dequeue() (QueuedCommand, bool) {
	return q.DequeueFiltered(nil)
}

// DequeueFiltered removes and returns the highest-priority command
// that matches the filter predicate. If filter is nil, any command matches.
func (q *CommandQueue) DequeueFiltered(filter func(QueuedCommand) bool) (QueuedCommand, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	bestIdx := -1
	bestPriority := QueuePriority(999)

	for i, cmd := range q.commands {
		if filter != nil && !filter(cmd) {
			continue
		}
		if cmd.Priority < bestPriority {
			bestPriority = cmd.Priority
			bestIdx = i
		}
	}

	if bestIdx == -1 {
		return QueuedCommand{}, false
	}

	cmd := q.commands[bestIdx]
	q.commands = append(q.commands[:bestIdx], q.commands[bestIdx+1:]...)
	q.notifyLocked()
	return cmd, true
}

// DequeueAll removes and returns all commands sorted by priority then FIFO.
func (q *CommandQueue) DequeueAll() []QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.commands) == 0 {
		return nil
	}

	result := q.sortedCopy()
	q.commands = nil
	q.notifyLocked()
	return result
}

// DequeueAllMatching removes and returns all commands matching the predicate.
func (q *CommandQueue) DequeueAllMatching(predicate func(QueuedCommand) bool) []QueuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()

	var removed []QueuedCommand
	var kept []QueuedCommand

	for _, cmd := range q.commands {
		if predicate(cmd) {
			removed = append(removed, cmd)
		} else {
			kept = append(kept, cmd)
		}
	}

	if len(removed) == 0 {
		return nil
	}

	q.commands = kept
	q.notifyLocked()
	return removed
}

// Peek returns the highest-priority command without removing it.
func (q *CommandQueue) Peek() (QueuedCommand, bool) {
	return q.PeekFiltered(nil)
}

// PeekFiltered returns the highest-priority command matching the filter.
func (q *CommandQueue) PeekFiltered(filter func(QueuedCommand) bool) (QueuedCommand, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	bestIdx := -1
	bestPriority := QueuePriority(999)

	for i, cmd := range q.commands {
		if filter != nil && !filter(cmd) {
			continue
		}
		if cmd.Priority < bestPriority {
			bestPriority = cmd.Priority
			bestIdx = i
		}
	}

	if bestIdx == -1 {
		return QueuedCommand{}, false
	}
	return q.commands[bestIdx], true
}

// GetByMaxPriority returns all commands at or above the given priority level
// (i.e., priority value <= maxPriority).
func (q *CommandQueue) GetByMaxPriority(maxPriority QueuePriority) []QueuedCommand {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []QueuedCommand
	for _, cmd := range q.commands {
		if cmd.Priority <= maxPriority {
			result = append(result, cmd)
		}
	}
	return result
}

// Snapshot returns a copy of all queued commands in priority order.
func (q *CommandQueue) Snapshot() []QueuedCommand {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.sortedCopy()
}

// Len returns the number of commands in the queue.
func (q *CommandQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.commands)
}

// IsEmpty returns true if the queue has no commands.
func (q *CommandQueue) IsEmpty() bool {
	return q.Len() == 0
}

// Clear removes all commands from the queue.
func (q *CommandQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.commands) > 0 {
		q.commands = nil
		q.notifyLocked()
	}
}

// Subscribe registers a listener called when the queue changes.
// Returns an unsubscribe function.
func (q *CommandQueue) Subscribe(fn func()) func() {
	q.mu.Lock()
	defer q.mu.Unlock()

	idx := len(q.listeners)
	q.listeners = append(q.listeners, fn)

	return func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		if idx < len(q.listeners) {
			q.listeners[idx] = nil
		}
	}
}

// notifyLocked calls all registered listeners. Must be called with mu held.
func (q *CommandQueue) notifyLocked() {
	for _, fn := range q.listeners {
		if fn != nil {
			fn()
		}
	}
}

// sortedCopy returns a priority-sorted copy of commands. Must be called with mu held.
func (q *CommandQueue) sortedCopy() []QueuedCommand {
	if len(q.commands) == 0 {
		return nil
	}

	// Stable sort by priority buckets.
	var now, next, later []QueuedCommand
	for _, cmd := range q.commands {
		switch cmd.Priority {
		case PriorityNow:
			now = append(now, cmd)
		case PriorityNext:
			next = append(next, cmd)
		default:
			later = append(later, cmd)
		}
	}

	result := make([]QueuedCommand, 0, len(q.commands))
	result = append(result, now...)
	result = append(result, next...)
	result = append(result, later...)
	return result
}
