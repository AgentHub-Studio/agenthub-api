package agentic

import (
	"sync"
	"time"
)

// Activity tracking for user and CLI operations.
//
// Inspired by Claude Code's activityManager.ts — tracks user interactions
// and CLI operations with deduplication and automatic cleanup.

// ActivityKind distinguishes user-driven from system-driven activity.
type ActivityKind string

const (
	// ActivityUser is a user interaction (keystroke, message, scroll).
	ActivityUser ActivityKind = "user"
	// ActivityCLI is a system/CLI operation (tool exec, LLM call, etc).
	ActivityCLI ActivityKind = "cli"
)

// ActivityState represents the current state of activity tracking.
type ActivityState struct {
	// UserActive is true if user activity was recorded within the timeout window.
	UserActive bool `json:"userActive"`
	// CLIActive is true if any CLI operation is in progress.
	CLIActive bool `json:"cliActive"`
	// ActiveOperations is the set of currently running operation IDs.
	ActiveOperations map[string]time.Time `json:"activeOperations,omitempty"`
	// LastUserActivity is the timestamp of the last user interaction.
	LastUserActivity time.Time `json:"lastUserActivity,omitempty"`
}

const (
	// defaultUserActivityTimeout is how long after the last interaction
	// the user is still considered "active".
	defaultUserActivityTimeout = 5 * time.Second
)

// ActivityManager tracks user and CLI operation activity.
type ActivityManager struct {
	mu sync.Mutex

	// userTimeout is the window after last interaction where user is "active".
	userTimeout time.Duration

	// lastUserActivity is when the user last interacted.
	lastUserActivity time.Time

	// activeOps tracks currently running CLI operations by ID.
	activeOps map[string]time.Time

	// subscribers are notified on state changes.
	subscribers []func(ActivityState)
}

// NewActivityManager creates an ActivityManager with default settings.
func NewActivityManager() *ActivityManager {
	return &ActivityManager{
		userTimeout: defaultUserActivityTimeout,
		activeOps:   make(map[string]time.Time),
	}
}

// NewActivityManagerWithTimeout creates an ActivityManager with a custom user timeout.
func NewActivityManagerWithTimeout(userTimeout time.Duration) *ActivityManager {
	return &ActivityManager{
		userTimeout: userTimeout,
		activeOps:   make(map[string]time.Time),
	}
}

// RecordUserActivity marks the current time as the last user interaction.
func (am *ActivityManager) RecordUserActivity() {
	am.mu.Lock()
	am.lastUserActivity = time.Now()
	am.mu.Unlock()
	am.notifySubscribers()
}

// StartCLIActivity marks a CLI operation as started.
// Returns a stop function that must be called when the operation completes.
func (am *ActivityManager) StartCLIActivity(operationID string) func() {
	am.mu.Lock()
	am.activeOps[operationID] = time.Now()
	am.mu.Unlock()
	am.notifySubscribers()

	return func() {
		am.EndCLIActivity(operationID)
	}
}

// EndCLIActivity marks a CLI operation as completed.
func (am *ActivityManager) EndCLIActivity(operationID string) {
	am.mu.Lock()
	delete(am.activeOps, operationID)
	am.mu.Unlock()
	am.notifySubscribers()
}

// TrackOperation wraps a function with automatic start/end tracking.
func (am *ActivityManager) TrackOperation(operationID string, fn func() error) error {
	stop := am.StartCLIActivity(operationID)
	defer stop()
	return fn()
}

// GetState returns the current activity state.
func (am *ActivityManager) GetState() ActivityState {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	userActive := !am.lastUserActivity.IsZero() && now.Sub(am.lastUserActivity) <= am.userTimeout

	ops := make(map[string]time.Time, len(am.activeOps))
	for k, v := range am.activeOps {
		ops[k] = v
	}

	return ActivityState{
		UserActive:       userActive,
		CLIActive:        len(am.activeOps) > 0,
		ActiveOperations: ops,
		LastUserActivity: am.lastUserActivity,
	}
}

// IsUserActive returns true if the user is currently active.
func (am *ActivityManager) IsUserActive() bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.lastUserActivity.IsZero() {
		return false
	}
	return time.Since(am.lastUserActivity) <= am.userTimeout
}

// IsCLIActive returns true if any CLI operation is in progress.
func (am *ActivityManager) IsCLIActive() bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	return len(am.activeOps) > 0
}

// ActiveOperationCount returns the number of running operations.
func (am *ActivityManager) ActiveOperationCount() int {
	am.mu.Lock()
	defer am.mu.Unlock()
	return len(am.activeOps)
}

// Subscribe registers a callback for state changes.
// Returns an unsubscribe function.
func (am *ActivityManager) Subscribe(fn func(ActivityState)) func() {
	am.mu.Lock()
	idx := len(am.subscribers)
	am.subscribers = append(am.subscribers, fn)
	am.mu.Unlock()

	return func() {
		am.mu.Lock()
		defer am.mu.Unlock()
		if idx < len(am.subscribers) {
			am.subscribers[idx] = nil
		}
	}
}

// notifySubscribers calls all registered subscribers with the current state.
func (am *ActivityManager) notifySubscribers() {
	state := am.GetState()

	am.mu.Lock()
	subs := make([]func(ActivityState), len(am.subscribers))
	copy(subs, am.subscribers)
	am.mu.Unlock()

	for _, fn := range subs {
		if fn != nil {
			fn(state)
		}
	}
}

// Reset clears all activity state.
func (am *ActivityManager) Reset() {
	am.mu.Lock()
	am.lastUserActivity = time.Time{}
	am.activeOps = make(map[string]time.Time)
	am.mu.Unlock()
}
