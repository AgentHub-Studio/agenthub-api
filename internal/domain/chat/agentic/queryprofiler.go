package agentic

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Query performance profiling framework.
//
// Inspired by Claude Code's queryProfiler.ts — timing instrumentation
// for the agentic query pipeline. Tracks checkpoints from user input
// to first token, through context loading, tool schema building,
// message normalization, and LLM network latency.

// ProfileCheckpoint records a named timing point in the query pipeline.
type ProfileCheckpoint struct {
	// Name is the checkpoint identifier (e.g., "context_loaded", "tools_built").
	Name string `json:"name"`
	// Timestamp is when this checkpoint was reached.
	Timestamp time.Time `json:"timestamp"`
	// DurationFromStart is the elapsed time since profile start.
	DurationFromStart time.Duration `json:"durationFromStart"`
	// DurationFromPrev is the elapsed time since the previous checkpoint.
	DurationFromPrev time.Duration `json:"durationFromPrev"`
}

// QueryProfile holds timing data for a complete query pipeline execution.
type QueryProfile struct {
	mu          sync.Mutex
	id          string
	startTime   time.Time
	endTime     time.Time
	checkpoints []ProfileCheckpoint
	metadata    map[string]any
}

// NewQueryProfile starts a new profiling session.
func NewQueryProfile(id string) *QueryProfile {
	return &QueryProfile{
		id:        id,
		startTime: time.Now(),
		metadata:  make(map[string]any),
	}
}

// Checkpoint records a named timing point.
func (qp *QueryProfile) Checkpoint(name string) {
	qp.mu.Lock()
	defer qp.mu.Unlock()

	now := time.Now()
	fromStart := now.Sub(qp.startTime)

	var fromPrev time.Duration
	if len(qp.checkpoints) > 0 {
		fromPrev = now.Sub(qp.checkpoints[len(qp.checkpoints)-1].Timestamp)
	} else {
		fromPrev = fromStart
	}

	qp.checkpoints = append(qp.checkpoints, ProfileCheckpoint{
		Name:              name,
		Timestamp:         now,
		DurationFromStart: fromStart,
		DurationFromPrev:  fromPrev,
	})
}

// SetMeta adds metadata to the profile (e.g., model name, token counts).
func (qp *QueryProfile) SetMeta(key string, value any) {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	qp.metadata[key] = value
}

// End marks the profile as complete.
func (qp *QueryProfile) End() {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	qp.endTime = time.Now()
}

// TotalDuration returns the total time from start to end (or now if not ended).
func (qp *QueryProfile) TotalDuration() time.Duration {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	if !qp.endTime.IsZero() {
		return qp.endTime.Sub(qp.startTime)
	}
	return time.Since(qp.startTime)
}

// TimeToCheckpoint returns the duration from start to the named checkpoint.
// Returns 0 if the checkpoint doesn't exist.
func (qp *QueryProfile) TimeToCheckpoint(name string) time.Duration {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	for _, cp := range qp.checkpoints {
		if cp.Name == name {
			return cp.DurationFromStart
		}
	}
	return 0
}

// Checkpoints returns a copy of all recorded checkpoints.
func (qp *QueryProfile) Checkpoints() []ProfileCheckpoint {
	qp.mu.Lock()
	defer qp.mu.Unlock()
	result := make([]ProfileCheckpoint, len(qp.checkpoints))
	copy(result, qp.checkpoints)
	return result
}

// Report generates a formatted performance report.
func (qp *QueryProfile) Report() string {
	qp.mu.Lock()
	defer qp.mu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "=== Query Profile: %s ===\n", qp.id)

	total := qp.endTime.Sub(qp.startTime)
	if qp.endTime.IsZero() {
		total = time.Since(qp.startTime)
	}
	fmt.Fprintf(&b, "Total: %s\n\n", total.Round(time.Microsecond))

	if len(qp.checkpoints) > 0 {
		b.WriteString("Phase breakdown:\n")
		maxNameLen := 0
		for _, cp := range qp.checkpoints {
			if len(cp.Name) > maxNameLen {
				maxNameLen = len(cp.Name)
			}
		}

		for _, cp := range qp.checkpoints {
			pct := 0.0
			if total > 0 {
				pct = float64(cp.DurationFromPrev) / float64(total) * 100
			}
			bar := strings.Repeat("█", int(pct/5))
			fmt.Fprintf(&b, "  %-*s  %8s  (%5.1f%%) %s\n",
				maxNameLen, cp.Name,
				cp.DurationFromPrev.Round(time.Microsecond),
				pct, bar)
		}
	}

	if len(qp.metadata) > 0 {
		b.WriteString("\nMetadata:\n")
		for k, v := range qp.metadata {
			fmt.Fprintf(&b, "  %s: %v\n", k, v)
		}
	}

	return b.String()
}

// --- Well-known checkpoint names ---

const (
	// CPContextLoaded is when conversation context has been loaded.
	CPContextLoaded = "context_loaded"
	// CPToolsBuilt is when tool schemas have been compiled.
	CPToolsBuilt = "tools_built"
	// CPPromptBuilt is when the system prompt has been assembled.
	CPPromptBuilt = "prompt_built"
	// CPMessagesNormalized is when messages have been normalized for the LLM.
	CPMessagesNormalized = "messages_normalized"
	// CPCompacted is when context compaction has been applied.
	CPCompacted = "compacted"
	// CPRequestSent is when the LLM API request has been dispatched.
	CPRequestSent = "request_sent"
	// CPFirstToken is when the first token has been received from the LLM.
	CPFirstToken = "first_token"
	// CPToolCallsReceived is when tool calls have been received from the LLM.
	CPToolCallsReceived = "tool_calls_received"
	// CPToolsExecuted is when all tools have been executed.
	CPToolsExecuted = "tools_executed"
	// CPResponseComplete is when the LLM response is fully received.
	CPResponseComplete = "response_complete"
)
