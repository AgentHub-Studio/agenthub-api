package agentic

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Mailbox message source types.
//
// Inspired by Claude Code's Mailbox.ts — in-process typed message bus
// for coordinating between conversation turns, agents, and system events.

// MessageSource identifies the origin of a mailbox message.
type MessageSource string

const (
	// SourceUser is a message from the end user.
	SourceUser MessageSource = "user"
	// SourceTeammate is a message from a sub-agent / forked agent.
	SourceTeammate MessageSource = "teammate"
	// SourceSystem is a system-generated message (compaction, errors, etc).
	SourceSystem MessageSource = "system"
	// SourceTick is a periodic tick event (heartbeat, polling).
	SourceTick MessageSource = "tick"
	// SourceTask is a message from a background task.
	SourceTask MessageSource = "task"
)

// MailMessage is an envelope for messages flowing through the Mailbox.
type MailMessage struct {
	// ID is a unique identifier for this message.
	ID string `json:"id"`
	// Source indicates who sent the message.
	Source MessageSource `json:"source"`
	// Content is the textual payload.
	Content string `json:"content"`
	// From is an optional sender identifier (agent ID, task ID, etc).
	From string `json:"from,omitempty"`
	// Color is an optional display hint for UI rendering.
	Color string `json:"color,omitempty"`
	// Metadata holds arbitrary key-value data.
	Metadata map[string]any `json:"metadata,omitempty"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// NewMailMessage creates a MailMessage with a generated ID and timestamp.
func NewMailMessage(source MessageSource, content string) MailMessage {
	return MailMessage{
		ID:        uuid.New().String(),
		Source:    source,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// SubscriberFunc is called when a new message arrives.
type SubscriberFunc func(MailMessage)

// Mailbox is an in-process message queue with subscribe/poll semantics.
//
// It supports:
//   - Send: enqueue a message (wakes any waiting Receive)
//   - Poll: non-blocking check for pending messages
//   - Receive: blocking wait for the next message (with context-like done channel)
//   - Subscribe: register a callback for every incoming message
//
// Inspired by Claude Code's Mailbox with waiter-deferral pattern.
type Mailbox struct {
	mu          sync.Mutex
	messages    []MailMessage
	subscribers []SubscriberFunc
	waiters     []chan MailMessage
	closed      bool
}

// NewMailbox creates an empty Mailbox.
func NewMailbox() *Mailbox {
	return &Mailbox{}
}

// Send enqueues a message. If there are blocked Receive callers,
// the message is delivered directly to the oldest waiter instead of
// being queued (waiter-deferral pattern).
func (m *Mailbox) Send(msg MailMessage) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	m.mu.Lock()

	if m.closed {
		m.mu.Unlock()
		return
	}

	// Notify subscribers (non-blocking).
	subs := make([]SubscriberFunc, len(m.subscribers))
	copy(subs, m.subscribers)

	// Waiter-deferral: if someone is blocked on Receive, hand off directly.
	if len(m.waiters) > 0 {
		w := m.waiters[0]
		m.waiters = m.waiters[1:]
		m.mu.Unlock()

		// Deliver to waiter.
		w <- msg

		// Notify subscribers.
		for _, fn := range subs {
			if fn != nil {
				fn(msg)
			}
		}
		return
	}

	// No waiters — enqueue.
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	// Notify subscribers.
	for _, fn := range subs {
		if fn != nil {
			fn(msg)
		}
	}
}

// Poll returns the next pending message without blocking.
// Returns the message and true, or zero value and false if empty.
func (m *Mailbox) Poll() (MailMessage, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return MailMessage{}, false
	}

	msg := m.messages[0]
	m.messages = m.messages[1:]
	return msg, true
}

// PollAll returns all pending messages, draining the queue.
func (m *Mailbox) PollAll() []MailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return nil
	}

	msgs := m.messages
	m.messages = nil
	return msgs
}

// Receive blocks until a message is available or done is closed.
// Returns the message and true, or zero value and false if done was closed.
func (m *Mailbox) Receive(done <-chan struct{}) (MailMessage, bool) {
	m.mu.Lock()

	// Fast path: message already pending.
	if len(m.messages) > 0 {
		msg := m.messages[0]
		m.messages = m.messages[1:]
		m.mu.Unlock()
		return msg, true
	}

	if m.closed {
		m.mu.Unlock()
		return MailMessage{}, false
	}

	// Slow path: register as waiter.
	ch := make(chan MailMessage, 1)
	m.waiters = append(m.waiters, ch)
	m.mu.Unlock()

	select {
	case msg, ok := <-ch:
		return msg, ok
	case <-done:
		// Remove ourselves from waiters.
		m.mu.Lock()
		for i, w := range m.waiters {
			if w == ch {
				m.waiters = append(m.waiters[:i], m.waiters[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
		return MailMessage{}, false
	}
}

// Subscribe registers a callback invoked on every Send.
// Returns an unsubscribe function.
func (m *Mailbox) Subscribe(fn SubscriberFunc) func() {
	m.mu.Lock()
	m.subscribers = append(m.subscribers, fn)
	idx := len(m.subscribers) - 1
	m.mu.Unlock()

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		// Mark nil to preserve indices (avoids shifting).
		if idx < len(m.subscribers) {
			m.subscribers[idx] = nil
		}
	}
}

// Pending returns the count of queued messages.
func (m *Mailbox) Pending() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

// Close prevents new messages and unblocks all waiters.
func (m *Mailbox) Close() {
	m.mu.Lock()
	m.closed = true
	waiters := m.waiters
	m.waiters = nil
	m.mu.Unlock()

	// Unblock all waiters by closing their channels.
	for _, w := range waiters {
		close(w)
	}
}

// FilterBySource returns pending messages matching the given source.
func (m *Mailbox) FilterBySource(source MessageSource) []MailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []MailMessage
	for _, msg := range m.messages {
		if msg.Source == source {
			result = append(result, msg)
		}
	}
	return result
}
