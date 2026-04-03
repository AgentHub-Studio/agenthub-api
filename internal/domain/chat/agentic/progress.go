package agentic

import (
	"sync"
	"time"
)

// ActivityType identifies the kind of activity recorded in the progress tracker.
type ActivityType string

const (
	ActivityLLMCall    ActivityType = "llm_call"
	ActivityToolCall   ActivityType = "tool_call"
	ActivitySubtask    ActivityType = "subtask"
	ActivityText       ActivityType = "text"
)

// ActivityItem records a single activity in the run timeline.
type ActivityItem struct {
	Timestamp time.Time    `json:"timestamp"`
	Type      ActivityType `json:"type"`
	Summary   string       `json:"summary"`
}

// RunProgressData is the payload for the run_progress SSE event.
type RunProgressData struct {
	TurnIndex      int            `json:"turnIndex"`
	TotalTokens    int            `json:"totalTokens"`
	TotalToolCalls int            `json:"totalToolCalls"`
	TotalCostUSD   float64        `json:"totalCostUsd"`
	ActiveSubtasks int            `json:"activeSubtasks"`
	RecentActivity []ActivityItem `json:"recentActivity"`
}

// RunProgressTracker accumulates metrics and activity throughout an agentic run.
// It is safe for concurrent use.
type RunProgressTracker struct {
	mu             sync.RWMutex
	turnIndex      int
	totalTokens    int
	totalToolCalls int
	totalCostUSD   float64
	activeSubtasks int
	activities     []ActivityItem
	maxActivities  int
}

// NewRunProgressTracker creates a tracker that keeps the last maxActivities items.
func NewRunProgressTracker(maxActivities int) *RunProgressTracker {
	if maxActivities <= 0 {
		maxActivities = 10
	}
	return &RunProgressTracker{
		maxActivities: maxActivities,
	}
}

// RecordLLMCall records an LLM call with token usage and cost.
func (t *RunProgressTracker) RecordLLMCall(tokens int, cost float64, model string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalTokens += tokens
	t.totalCostUSD += cost
	t.addActivity(ActivityLLMCall, "LLM call to "+model)
}

// RecordToolCall records a tool execution.
func (t *RunProgressTracker) RecordToolCall(toolName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalToolCalls++
	t.addActivity(ActivityToolCall, "Executed "+toolName)
}

// RecordSubtaskStart increments active subtask counter.
func (t *RunProgressTracker) RecordSubtaskStart(description string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.activeSubtasks++
	t.addActivity(ActivitySubtask, "Started: "+description)
}

// RecordSubtaskComplete decrements active subtask counter.
func (t *RunProgressTracker) RecordSubtaskComplete(description string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.activeSubtasks > 0 {
		t.activeSubtasks--
	}
	t.addActivity(ActivitySubtask, "Completed: "+description)
}

// RecordText records a text generation activity.
func (t *RunProgressTracker) RecordText(summary string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.addActivity(ActivityText, summary)
}

// SetTurnIndex updates the current turn index.
func (t *RunProgressTracker) SetTurnIndex(idx int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.turnIndex = idx
}

// Snapshot returns a copy of the current progress state.
func (t *RunProgressTracker) Snapshot() RunProgressData {
	t.mu.RLock()
	defer t.mu.RUnlock()
	activities := make([]ActivityItem, len(t.activities))
	copy(activities, t.activities)
	return RunProgressData{
		TurnIndex:      t.turnIndex,
		TotalTokens:    t.totalTokens,
		TotalToolCalls: t.totalToolCalls,
		TotalCostUSD:   t.totalCostUSD,
		ActiveSubtasks: t.activeSubtasks,
		RecentActivity: activities,
	}
}

// addActivity appends an activity, evicting the oldest if at capacity.
// Must be called with t.mu held.
func (t *RunProgressTracker) addActivity(typ ActivityType, summary string) {
	item := ActivityItem{
		Timestamp: time.Now(),
		Type:      typ,
		Summary:   summary,
	}
	if len(t.activities) >= t.maxActivities {
		// Shift left to evict oldest.
		copy(t.activities, t.activities[1:])
		t.activities[len(t.activities)-1] = item
	} else {
		t.activities = append(t.activities, item)
	}
}
