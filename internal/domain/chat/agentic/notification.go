package agentic

import (
	"sync"
	"time"
)

// NotificationPriority controls the display order and behavior of notifications.
//
// Inspired by Claude Code's Priority type in context/notifications.tsx.
type NotificationPriority int

const (
	// NotifyImmediate pre-empts the current notification and displays instantly.
	NotifyImmediate NotificationPriority = 0
	// NotifyHigh is processed before medium and low but waits for current to finish.
	NotifyHigh NotificationPriority = 1
	// NotifyMedium is the default priority.
	NotifyMedium NotificationPriority = 2
	// NotifyLow is processed last.
	NotifyLow NotificationPriority = 3
)

// DefaultNotificationTimeout is the default display duration.
const DefaultNotificationTimeout = 8 * time.Second

// Notification is an item in the notification queue.
//
// Inspired by Claude Code's Notification union in context/notifications.tsx.
type Notification struct {
	// Key uniquely identifies this notification for dedup and invalidation.
	Key string `json:"key"`
	// Text is the notification message.
	Text string `json:"text"`
	// Priority controls display ordering.
	Priority NotificationPriority `json:"priority"`
	// TimeoutMs overrides the default display duration. 0 means use default.
	TimeoutMs int `json:"timeoutMs,omitempty"`
	// Invalidates lists keys of notifications this one supersedes.
	// Invalidated notifications are removed from the queue and cleared if displayed.
	Invalidates []string `json:"invalidates,omitempty"`
	// Metadata carries arbitrary data for notification consumers.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Timeout returns the effective timeout duration for this notification.
func (n *Notification) Timeout() time.Duration {
	if n.TimeoutMs > 0 {
		return time.Duration(n.TimeoutMs) * time.Millisecond
	}
	return DefaultNotificationTimeout
}

// FoldFn combines two notifications with the same key, like Array.reduce().
// Called as fold(accumulator, incoming) when a matching key exists.
type FoldFn func(accumulator, incoming Notification) Notification

// NotificationQueue manages a priority-ordered queue of notifications with
// fold-based deduplication and invalidation rules.
//
// Inspired by Claude Code's useNotifications hook.
type NotificationQueue struct {
	mu      sync.Mutex
	queue   []Notification
	current *Notification
	foldFns map[string]FoldFn
	onShow  func(Notification) // callback when a notification becomes current
	onClear func(string)       // callback when a notification is dismissed (key)
	timer   *time.Timer
}

// NewNotificationQueue creates a new notification queue.
// onShow is called when a notification becomes the current display item.
// onClear is called when a notification is dismissed.
func NewNotificationQueue(onShow func(Notification), onClear func(string)) *NotificationQueue {
	return &NotificationQueue{
		foldFns: make(map[string]FoldFn),
		onShow:  onShow,
		onClear: onClear,
	}
}

// RegisterFold registers a fold function for a notification key.
// When a notification with this key already exists, fold merges them.
func (q *NotificationQueue) RegisterFold(key string, fn FoldFn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.foldFns[key] = fn
}

// Add enqueues a notification. Handles priority, fold, and invalidation.
func (q *NotificationQueue) Add(notif Notification) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Apply invalidation: remove invalidated items from queue and current.
	if len(notif.Invalidates) > 0 {
		invalidSet := make(map[string]bool, len(notif.Invalidates))
		for _, k := range notif.Invalidates {
			invalidSet[k] = true
		}
		q.queue = filterQueue(q.queue, func(n Notification) bool {
			return !invalidSet[n.Key]
		})
		if q.current != nil && invalidSet[q.current.Key] {
			q.clearCurrentLocked()
		}
	}

	// Immediate priority: pre-empt current.
	if notif.Priority == NotifyImmediate {
		if q.current != nil {
			// Re-queue current if it's not immediate.
			if q.current.Priority != NotifyImmediate {
				q.queue = append(q.queue, *q.current)
			}
			q.clearCurrentLocked()
		}
		q.setCurrentLocked(notif)
		return
	}

	// Fold: merge with existing notification sharing the same key.
	if foldFn, ok := q.foldFns[notif.Key]; ok {
		// Check current.
		if q.current != nil && q.current.Key == notif.Key {
			merged := foldFn(*q.current, notif)
			q.current = &merged
			return
		}
		// Check queue.
		for i, existing := range q.queue {
			if existing.Key == notif.Key {
				q.queue[i] = foldFn(existing, notif)
				return
			}
		}
	}

	// Prevent duplicate keys.
	for _, existing := range q.queue {
		if existing.Key == notif.Key {
			return
		}
	}

	q.queue = append(q.queue, notif)
	q.processQueueLocked()
}

// Remove removes a notification by key from both queue and current.
func (q *NotificationQueue) Remove(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.current != nil && q.current.Key == key {
		q.clearCurrentLocked()
		q.processQueueLocked()
		return
	}
	q.queue = filterQueue(q.queue, func(n Notification) bool {
		return n.Key != key
	})
}

// Current returns the currently displayed notification, if any.
func (q *NotificationQueue) Current() *Notification {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.current == nil {
		return nil
	}
	cp := *q.current
	return &cp
}

// QueueLen returns the number of queued (not current) notifications.
func (q *NotificationQueue) QueueLen() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// Clear removes all notifications and stops any timer.
func (q *NotificationQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = nil
	if q.current != nil {
		q.clearCurrentLocked()
	}
}

// processQueueLocked advances the queue. Caller must hold mu.
func (q *NotificationQueue) processQueueLocked() {
	if q.current != nil || len(q.queue) == 0 {
		return
	}
	next := q.popNextLocked()
	if next == nil {
		return
	}
	q.setCurrentLocked(*next)
}

// setCurrentLocked sets the current notification and starts the timeout timer.
func (q *NotificationQueue) setCurrentLocked(notif Notification) {
	q.current = &notif
	if q.onShow != nil {
		q.onShow(notif)
	}
	timeout := notif.Timeout()
	q.timer = time.AfterFunc(timeout, func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		if q.current != nil && q.current.Key == notif.Key {
			q.clearCurrentLocked()
			q.processQueueLocked()
		}
	})
}

// clearCurrentLocked clears the current notification. Caller must hold mu.
func (q *NotificationQueue) clearCurrentLocked() {
	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
	if q.current != nil {
		key := q.current.Key
		q.current = nil
		if q.onClear != nil {
			q.onClear(key)
		}
	}
}

// popNextLocked returns the highest-priority item and removes it from the queue.
func (q *NotificationQueue) popNextLocked() *Notification {
	if len(q.queue) == 0 {
		return nil
	}
	bestIdx := 0
	for i := 1; i < len(q.queue); i++ {
		if q.queue[i].Priority < q.queue[bestIdx].Priority {
			bestIdx = i
		}
	}
	item := q.queue[bestIdx]
	q.queue = append(q.queue[:bestIdx], q.queue[bestIdx+1:]...)
	return &item
}

// filterQueue returns items that match the predicate.
func filterQueue(queue []Notification, keep func(Notification) bool) []Notification {
	var result []Notification
	for _, n := range queue {
		if keep(n) {
			result = append(result, n)
		}
	}
	return result
}
