package agentic

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// StopHookEvent identifies what type of stop hook is firing.
// Inspired by Claude Code's HookEvent type in hooks/types.ts.
type StopHookEvent string

const (
	// StopHookStop fires after each assistant turn completes (no more tool calls).
	StopHookStop StopHookEvent = "stop"
	// StopHookTaskCompleted fires when a teammate finishes a task.
	StopHookTaskCompleted StopHookEvent = "task_completed"
	// StopHookTeammateIdle fires when a teammate has no more tasks.
	StopHookTeammateIdle StopHookEvent = "teammate_idle"
)

// StopHookResult is the outcome of running all stop hooks for a turn.
type StopHookResult struct {
	// BlockingErrors are errors that must be fed back to the LLM as user messages.
	BlockingErrors []string
	// PreventContinuation, when true, stops the agentic loop (hook vetoed continuation).
	PreventContinuation bool
	// StopReason explains why continuation was prevented.
	StopReason string
	// HookCount is the number of hooks that executed.
	HookCount int
	// HookErrors collects non-fatal errors from individual hooks.
	HookErrors []string
	// Duration is the total wall-clock time of all hook execution.
	Duration time.Duration
}

// StopHookInfo holds metadata about a single executed hook for summary messages.
type StopHookInfo struct {
	Name       string        `json:"name"`
	Event      StopHookEvent `json:"event"`
	DurationMs int64         `json:"durationMs,omitempty"`
	Error      *string       `json:"error,omitempty"`
}

// StopHookContext carries everything stop hooks need to execute.
type StopHookContext struct {
	// Messages is the full conversation including the latest assistant turn.
	Messages []ai.Message
	// SystemPrompt is the current system prompt.
	SystemPrompt string
	// QuerySource identifies the origin of this query (main_loop, subtask, etc.).
	QuerySource QuerySource
	// AgentID is the agent that produced this turn.
	AgentID uuid.UUID
	// SessionID is the chat session.
	SessionID uuid.UUID
	// TenantID for multi-tenant hook resolution.
	TenantID string
	// CacheSafeParams snapshot for background forks.
	CacheSafeParams *CacheSafeParams
	// CurrentDepth is 0 for root agent, >0 for sub-agents.
	CurrentDepth int
	// TurnIndex is the current turn number.
	TurnIndex int
	// CurrentTokens is the estimated total tokens in the conversation.
	CurrentTokens int
	// TurnHadToolCalls indicates whether this turn included tool executions.
	TurnHadToolCalls bool
}

// StopHooksOrchestrator coordinates post-turn lifecycle hooks.
// After each assistant turn completes, it:
//  1. Saves CacheSafeParams snapshot for background forks
//  2. Executes registered stop hooks (with blocking error collection)
//  3. Fires background tasks (session memory extraction, auto-dream)
//  4. Emits summary events
//
// Inspired by Claude Code's handleStopHooks() in query/stopHooks.ts.
type StopHooksOrchestrator struct {
	hookExecutor     *HookExecutor
	memoryExtractor  *SessionMemoryExtractor
	cacheSafeSnap    *CacheSafeParamsSnapshot
	eventCh          chan<- RunEvent
}

// NewStopHooksOrchestrator creates a StopHooksOrchestrator.
// All dependencies are optional — features are gracefully skipped when nil.
func NewStopHooksOrchestrator(
	hookExecutor *HookExecutor,
	memoryExtractor *SessionMemoryExtractor,
	cacheSafeSnap *CacheSafeParamsSnapshot,
	eventCh chan<- RunEvent,
) *StopHooksOrchestrator {
	return &StopHooksOrchestrator{
		hookExecutor:    hookExecutor,
		memoryExtractor: memoryExtractor,
		cacheSafeSnap:   cacheSafeSnap,
		eventCh:         eventCh,
	}
}

// HandleStopHooks executes all post-turn hooks and returns the result.
// This is the main entry point called by the runner after each assistant turn.
//
// Execution order:
//  1. Save CacheSafeParams (only for main_loop/subtask — sub-agents must not overwrite)
//  2. Execute registered stop hooks (blocking — may veto continuation)
//  3. Fire background tasks (non-blocking — memory extraction)
//  4. Build and return result with hook summary
func (o *StopHooksOrchestrator) HandleStopHooks(ctx context.Context, hookCtx StopHookContext) StopHookResult {
	start := time.Now()
	result := StopHookResult{}

	// 1. Save CacheSafeParams snapshot for background forks.
	// Only for main thread queries — sub-agents must not overwrite the parent's snapshot.
	if hookCtx.CurrentDepth == 0 && o.cacheSafeSnap != nil && hookCtx.CacheSafeParams != nil {
		o.cacheSafeSnap.Save(hookCtx.CacheSafeParams)
	}

	// 2. Execute registered stop hooks (blocking).
	if o.hookExecutor != nil && hookCtx.QuerySource == SourceMainLoop {
		hookResults := o.executeStopHooks(ctx, hookCtx)
		for _, hr := range hookResults {
			result.HookCount++
			if hr.Error != nil {
				result.HookErrors = append(result.HookErrors, *hr.Error)
			}
		}

		// Check for blocking errors that should be fed back to the LLM.
		for _, hr := range hookResults {
			if hr.Error != nil && isBlockingHookError(*hr.Error) {
				result.BlockingErrors = append(result.BlockingErrors, *hr.Error)
			}
		}
	}

	// 3. Fire background tasks (non-blocking, fire-and-forget).
	// Only for main thread — sub-agents and background forks don't spawn their own background work.
	if hookCtx.CurrentDepth == 0 && hookCtx.QuerySource == SourceMainLoop {
		o.fireBackgroundTasks(ctx, hookCtx)
	}

	result.Duration = time.Since(start)

	// 4. Emit hook summary event if hooks ran.
	if result.HookCount > 0 {
		o.emitHookSummary(hookCtx, result)
	}

	return result
}

// executeStopHooks runs registered stop hooks via the HookExecutor.
func (o *StopHooksOrchestrator) executeStopHooks(ctx context.Context, hookCtx StopHookContext) []HookResult {
	payload := HookPayload{
		Event:     HookSessionEnd, // Reuse session_end as the stop hook event.
		AgentID:   hookCtx.AgentID.String(),
		SessionID: hookCtx.SessionID.String(),
	}
	return o.hookExecutor.Execute(ctx, payload)
}

// fireBackgroundTasks launches non-blocking background work after a turn.
func (o *StopHooksOrchestrator) fireBackgroundTasks(ctx context.Context, hookCtx StopHookContext) {
	// Session memory extraction.
	if o.memoryExtractor != nil {
		if o.memoryExtractor.ShouldExtract(hookCtx.CurrentTokens, hookCtx.TurnHadToolCalls) {
			slog.Debug("firing session memory extraction",
				"turn", hookCtx.TurnIndex,
				"tokens", hookCtx.CurrentTokens,
			)
			o.memoryExtractor.Extract(ctx, hookCtx.CacheSafeParams, hookCtx.CurrentTokens, nil)
		}
	}
}

// emitHookSummary emits a summary event for the executed hooks.
func (o *StopHooksOrchestrator) emitHookSummary(hookCtx StopHookContext, result StopHookResult) {
	if o.eventCh == nil {
		return
	}

	summary := buildHookSummaryText(result)
	o.eventCh <- NewRunEvent(EventStopHookSummary, StopHookSummaryData{
		TurnIndex:           hookCtx.TurnIndex,
		HookCount:           result.HookCount,
		ErrorCount:          len(result.HookErrors),
		PreventContinuation: result.PreventContinuation,
		Summary:             summary,
		DurationMs:          result.Duration.Milliseconds(),
	})
}

// StopHookSummaryData carries the summary of post-turn hook execution.
type StopHookSummaryData struct {
	TurnIndex           int    `json:"turnIndex"`
	HookCount           int    `json:"hookCount"`
	ErrorCount          int    `json:"errorCount"`
	PreventContinuation bool   `json:"preventContinuation,omitempty"`
	Summary             string `json:"summary"`
	DurationMs          int64  `json:"durationMs"`
}

// buildHookSummaryText creates a human-readable summary of hook execution.
func buildHookSummaryText(result StopHookResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d hook(s) executed", result.HookCount)
	if len(result.HookErrors) > 0 {
		fmt.Fprintf(&sb, ", %d error(s)", len(result.HookErrors))
	}
	if result.PreventContinuation {
		fmt.Fprintf(&sb, " — continuation prevented: %s", result.StopReason)
	}
	return sb.String()
}

// isBlockingHookError returns true if a hook error should be fed back to
// the LLM as a user message (blocking the loop until resolved).
func isBlockingHookError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "blocking") || strings.Contains(lower, "fatal")
}

// --- Background Task Registry ---

// BackgroundTask represents a fire-and-forget task spawned after a turn.
type BackgroundTask struct {
	ID        string
	Label     string
	StartedAt time.Time
}

// BackgroundTaskTracker manages active background tasks for observability.
type BackgroundTaskTracker struct {
	mu    sync.Mutex
	tasks map[string]*BackgroundTask
}

// NewBackgroundTaskTracker creates a BackgroundTaskTracker.
func NewBackgroundTaskTracker() *BackgroundTaskTracker {
	return &BackgroundTaskTracker{
		tasks: make(map[string]*BackgroundTask),
	}
}

// Start registers a new background task. Returns the task ID.
func (t *BackgroundTaskTracker) Start(label string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := uuid.New().String()
	t.tasks[id] = &BackgroundTask{
		ID:        id,
		Label:     label,
		StartedAt: time.Now(),
	}
	return id
}

// Complete removes a background task from tracking.
func (t *BackgroundTaskTracker) Complete(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.tasks, id)
}

// Active returns the number of currently running background tasks.
func (t *BackgroundTaskTracker) Active() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.tasks)
}

// List returns a snapshot of active background tasks.
func (t *BackgroundTaskTracker) List() []BackgroundTask {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]BackgroundTask, 0, len(t.tasks))
	for _, task := range t.tasks {
		result = append(result, *task)
	}
	return result
}
