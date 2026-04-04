package agentic

import (
	"fmt"
	"sync"
)

// Concurrent tool executor with concurrency control and FIFO result ordering.
//
// Inspired by Claude Code's StreamingToolExecutor.ts — manages concurrent
// tool execution with safety constraints: non-concurrent tools get exclusive
// access, concurrent-safe tools run in parallel. Results stream in FIFO order
// regardless of completion order.

// ConcToolStatus tracks a tool's execution lifecycle.
type ConcToolStatus string

const (
	ConcToolQueued    ConcToolStatus = "queued"
	ConcToolExecuting ConcToolStatus = "executing"
	ConcToolCompleted ConcToolStatus = "completed"
	ConcToolYielded   ConcToolStatus = "yielded"
	ConcToolFailed    ConcToolStatus = "failed"
)

// ConcTrackedTool tracks a single tool execution with concurrency metadata.
type ConcTrackedTool struct {
	ID               string         `json:"id"`
	ToolName         string         `json:"toolName"`
	Status           ConcToolStatus `json:"status"`
	ConcurrencySafe  bool           `json:"concurrencySafe"`
	Result           interface{}    `json:"result,omitempty"`
	Error            error          `json:"-"`
	ProgressMessages []interface{}  `json:"-"`
}

// ConcurrencySafeFunc determines if a tool can run concurrently.
type ConcurrencySafeFunc func(toolName string) bool

// ConcToolExecutorConfig configures the concurrent executor.
type ConcToolExecutorConfig struct {
	// IsConcurrencySafe determines if a tool can run in parallel.
	IsConcurrencySafe ConcurrencySafeFunc
	// MaxConcurrent limits parallel tool executions (0 = unlimited).
	MaxConcurrent int
}

// ConcToolExecutor manages tool execution with concurrency control.
type ConcToolExecutor struct {
	mu            sync.Mutex
	config        ConcToolExecutorConfig
	tools         []*ConcTrackedTool
	executing     int
	exclusiveLock bool
	discarded     bool
}

// NewConcToolExecutor creates a concurrent tool executor.
func NewConcToolExecutor(config ConcToolExecutorConfig) *ConcToolExecutor {
	if config.IsConcurrencySafe == nil {
		config.IsConcurrencySafe = func(string) bool { return false }
	}
	return &ConcToolExecutor{
		config: config,
	}
}

// AddTool registers a tool for execution. Returns the tracked tool.
func (e *ConcToolExecutor) AddTool(id, toolName string) *ConcTrackedTool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.discarded {
		return nil
	}

	tt := &ConcTrackedTool{
		ID:              id,
		ToolName:        toolName,
		Status:          ConcToolQueued,
		ConcurrencySafe: e.config.IsConcurrencySafe(toolName),
	}

	e.tools = append(e.tools, tt)
	return tt
}

// GetReady returns the next tools eligible for execution based on concurrency rules.
func (e *ConcToolExecutor) GetReady() []*ConcTrackedTool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.discarded || e.exclusiveLock {
		return nil
	}

	var ready []*ConcTrackedTool

	for _, tt := range e.tools {
		if tt.Status != ConcToolQueued {
			continue
		}

		if !tt.ConcurrencySafe {
			if e.executing > 0 {
				break
			}
			ready = append(ready, tt)
			break
		}

		if e.config.MaxConcurrent > 0 && (e.executing+len(ready)) >= e.config.MaxConcurrent {
			break
		}

		ready = append(ready, tt)
	}

	return ready
}

// MarkExecuting transitions a tool to executing state.
func (e *ConcToolExecutor) MarkExecuting(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	tt := e.findTool(id)
	if tt == nil || tt.Status != ConcToolQueued {
		return false
	}

	tt.Status = ConcToolExecuting
	e.executing++

	if !tt.ConcurrencySafe {
		e.exclusiveLock = true
	}

	return true
}

// MarkCompleted transitions a tool to completed state with its result.
func (e *ConcToolExecutor) MarkCompleted(id string, result interface{}) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	tt := e.findTool(id)
	if tt == nil || tt.Status != ConcToolExecuting {
		return false
	}

	tt.Status = ConcToolCompleted
	tt.Result = result
	e.executing--

	if !tt.ConcurrencySafe {
		e.exclusiveLock = false
	}

	return true
}

// MarkFailed transitions a tool to failed state with an error.
func (e *ConcToolExecutor) MarkFailed(id string, err error) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	tt := e.findTool(id)
	if tt == nil || tt.Status != ConcToolExecuting {
		return false
	}

	tt.Status = ConcToolFailed
	tt.Error = err
	e.executing--

	if !tt.ConcurrencySafe {
		e.exclusiveLock = false
	}

	return true
}

// AddProgress adds a progress message to a tracked tool.
func (e *ConcToolExecutor) AddProgress(id string, progress interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tt := e.findTool(id)
	if tt == nil {
		return
	}

	tt.ProgressMessages = append(tt.ProgressMessages, progress)
}

// YieldProgress returns and clears all pending progress messages.
func (e *ConcToolExecutor) YieldProgress() []interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()

	var all []interface{}
	for _, tt := range e.tools {
		if len(tt.ProgressMessages) > 0 {
			all = append(all, tt.ProgressMessages...)
			tt.ProgressMessages = nil
		}
	}
	return all
}

// GetResults returns completed tools in FIFO order (order added).
// Only returns results up to the first non-completed tool (maintains ordering).
func (e *ConcToolExecutor) GetResults() []*ConcTrackedTool {
	e.mu.Lock()
	defer e.mu.Unlock()

	var results []*ConcTrackedTool
	for _, tt := range e.tools {
		switch tt.Status {
		case ConcToolCompleted, ConcToolFailed:
			results = append(results, tt)
			tt.Status = ConcToolYielded
		case ConcToolYielded:
			continue // already yielded, skip
		default:
			// Queued or executing — stop to preserve FIFO order
			return results
		}
	}
	return results
}

// Discard abandons all in-flight tools.
func (e *ConcToolExecutor) Discard() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.discarded = true

	for _, tt := range e.tools {
		if tt.Status == ConcToolQueued || tt.Status == ConcToolExecuting {
			tt.Status = ConcToolFailed
			tt.Error = fmt.Errorf("discarded")
		}
	}

	e.executing = 0
	e.exclusiveLock = false
}

// IsDiscarded returns whether the executor has been discarded.
func (e *ConcToolExecutor) IsDiscarded() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.discarded
}

// AllDone returns true if all tools have completed or failed.
func (e *ConcToolExecutor) AllDone() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, tt := range e.tools {
		switch tt.Status {
		case ConcToolQueued, ConcToolExecuting:
			return false
		}
	}
	return true
}

// Count returns the total number of tracked tools.
func (e *ConcToolExecutor) Count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.tools)
}

// ExecutingCount returns the number of currently executing tools.
func (e *ConcToolExecutor) ExecutingCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executing
}

// ConcToolExecSummary returns a status summary.
func (e *ConcToolExecutor) ConcToolExecSummary() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	queued, executing, completed, failed := 0, 0, 0, 0
	for _, tt := range e.tools {
		switch tt.Status {
		case ConcToolQueued:
			queued++
		case ConcToolExecuting:
			executing++
		case ConcToolCompleted, ConcToolYielded:
			completed++
		case ConcToolFailed:
			failed++
		}
	}

	return fmt.Sprintf("queued=%d executing=%d completed=%d failed=%d",
		queued, executing, completed, failed)
}

func (e *ConcToolExecutor) findTool(id string) *ConcTrackedTool {
	for _, tt := range e.tools {
		if tt.ID == id {
			return tt
		}
	}
	return nil
}
