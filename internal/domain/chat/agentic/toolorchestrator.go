package agentic

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Concurrent tool execution orchestrator.
//
// Inspired by Claude Code's StreamingToolExecutor — manages parallel
// execution of tools with safety classification. Tools marked as
// "concurrent-safe" run in parallel; exclusive tools run alone.
// Results are collected in deterministic order regardless of completion time.

// ToolSafety classifies a tool's concurrency behavior.
type ToolSafety string

const (
	// ToolSafeConcurrent means the tool can run in parallel with others.
	ToolSafeConcurrent ToolSafety = "concurrent"
	// ToolSafeExclusive means the tool must run alone (side effects).
	ToolSafeExclusive ToolSafety = "exclusive"
)

// OrchestratedTool represents a tool execution being orchestrated.
type OrchestratedTool struct {
	// ID is the tool_call ID from the LLM response.
	ID string `json:"id"`
	// Name is the tool/skill name.
	Name string `json:"name"`
	// Input is the tool call arguments.
	Input map[string]any `json:"input,omitempty"`
	// Safety classifies concurrency behavior.
	Safety ToolSafety `json:"safety"`

	// Result is populated after execution.
	Result *OrchestratedToolResult `json:"result,omitempty"`
	// Error is populated if execution failed.
	Error error `json:"-"`
	// StartedAt is when execution started.
	StartedAt time.Time `json:"startedAt"`
	// CompletedAt is when execution finished.
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

// Duration returns the execution duration.
func (ot *OrchestratedTool) Duration() time.Duration {
	if ot.CompletedAt.IsZero() {
		return time.Since(ot.StartedAt)
	}
	return ot.CompletedAt.Sub(ot.StartedAt)
}

// OrchestratedToolResult holds the output of an orchestrated tool execution.
type OrchestratedToolResult struct {
	// Output is the result payload.
	Output map[string]any `json:"output,omitempty"`
	// IsError indicates the tool returned an error result.
	IsError bool `json:"isError,omitempty"`
	// ErrorMessage is the error description when IsError is true.
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// OrchestratorExecutorFunc is the function signature for executing a tool.
type OrchestratorExecutorFunc func(ctx context.Context, name string, input map[string]any) (*OrchestratedToolResult, error)

// ToolOrchestrator manages concurrent execution of tool calls from a single LLM turn.
type ToolOrchestrator struct {
	mu sync.Mutex

	// executor is the function that actually runs tools.
	executor OrchestratorExecutorFunc

	// safetyClassifier determines if a tool is safe for concurrent execution.
	safetyClassifier func(name string) ToolSafety

	// tools tracks all tools in submission order.
	tools []*OrchestratedTool

	// onStart is called when a tool starts executing.
	onStart func(OrchestratedTool)
	// onComplete is called when a tool finishes (success or error).
	onComplete func(OrchestratedTool)
}

// NewToolOrchestrator creates an orchestrator with the given executor.
func NewToolOrchestrator(executor OrchestratorExecutorFunc) *ToolOrchestrator {
	return &ToolOrchestrator{
		executor:         executor,
		safetyClassifier: defaultSafetyClassifier,
	}
}

// SetSafetyClassifier overrides the default safety classification function.
func (to *ToolOrchestrator) SetSafetyClassifier(fn func(name string) ToolSafety) {
	to.safetyClassifier = fn
}

// OnStart registers a callback for tool execution start.
func (to *ToolOrchestrator) OnStart(fn func(OrchestratedTool)) {
	to.onStart = fn
}

// OnComplete registers a callback for tool execution completion.
func (to *ToolOrchestrator) OnComplete(fn func(OrchestratedTool)) {
	to.onComplete = fn
}

// ExecuteAll runs all provided tool calls, respecting safety classifications.
// Concurrent-safe tools run in parallel; exclusive tools run sequentially.
// Returns results in the same order as the input tools, regardless of completion time.
func (to *ToolOrchestrator) ExecuteAll(ctx context.Context, tools []OrchestratedTool) []*OrchestratedTool {
	if len(tools) == 0 {
		return nil
	}

	// Classify and track all tools.
	to.mu.Lock()
	to.tools = make([]*OrchestratedTool, len(tools))
	for i := range tools {
		tools[i].Safety = to.safetyClassifier(tools[i].Name)
		to.tools[i] = &tools[i]
	}
	to.mu.Unlock()

	// Partition into concurrent and exclusive groups.
	var concurrent []*OrchestratedTool
	var exclusive []*OrchestratedTool
	for _, tt := range to.tools {
		if tt.Safety == ToolSafeConcurrent {
			concurrent = append(concurrent, tt)
		} else {
			exclusive = append(exclusive, tt)
		}
	}

	// Execute concurrent tools in parallel.
	if len(concurrent) > 0 {
		var wg sync.WaitGroup
		for _, tt := range concurrent {
			wg.Add(1)
			go func(tool *OrchestratedTool) {
				defer wg.Done()
				to.executeSingle(ctx, tool)
			}(tt)
		}
		wg.Wait()
	}

	// Execute exclusive tools sequentially.
	for _, tt := range exclusive {
		select {
		case <-ctx.Done():
			tt.Error = ctx.Err()
			tt.CompletedAt = time.Now()
			continue
		default:
		}
		to.executeSingle(ctx, tt)
	}

	return to.tools
}

// executeSingle runs a single tool and updates its tracking state.
func (to *ToolOrchestrator) executeSingle(ctx context.Context, tt *OrchestratedTool) {
	tt.StartedAt = time.Now()

	if to.onStart != nil {
		to.onStart(*tt)
	}

	result, err := to.executor(ctx, tt.Name, tt.Input)
	tt.CompletedAt = time.Now()
	tt.Result = result
	tt.Error = err

	if err != nil && result == nil {
		tt.Result = &OrchestratedToolResult{
			IsError:      true,
			ErrorMessage: err.Error(),
		}
	}

	if to.onComplete != nil {
		to.onComplete(*tt)
	}
}

// Results returns all tracked tools (in submission order).
func (to *ToolOrchestrator) Results() []*OrchestratedTool {
	to.mu.Lock()
	defer to.mu.Unlock()
	return to.tools
}

// HasErrors returns true if any tool execution failed.
func (to *ToolOrchestrator) HasErrors() bool {
	to.mu.Lock()
	defer to.mu.Unlock()
	for _, tt := range to.tools {
		if tt.Error != nil || (tt.Result != nil && tt.Result.IsError) {
			return true
		}
	}
	return false
}

// ToolCount returns the number of tracked tools.
func (to *ToolOrchestrator) ToolCount() int {
	to.mu.Lock()
	defer to.mu.Unlock()
	return len(to.tools)
}

// Summary returns a brief summary of execution results.
func (to *ToolOrchestrator) Summary() string {
	to.mu.Lock()
	defer to.mu.Unlock()

	var succeeded, failed int
	for _, tt := range to.tools {
		if tt.Error != nil || (tt.Result != nil && tt.Result.IsError) {
			failed++
		} else {
			succeeded++
		}
	}
	return fmt.Sprintf("%d succeeded, %d failed out of %d tools", succeeded, failed, len(to.tools))
}

// defaultSafetyClassifier marks all tools as concurrent by default.
func defaultSafetyClassifier(_ string) ToolSafety {
	return ToolSafeConcurrent
}
