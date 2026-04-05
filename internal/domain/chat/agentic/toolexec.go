package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// TrackedTool tracks the state of a single tool execution.
type TrackedTool struct {
	mu        sync.Mutex
	ID        string
	Name      string
	Input     json.RawMessage
	State     ToolState
	Result    *ToolExecResult
	StartedAt time.Time
	DoneAt    time.Time
}

func newTrackedTool(id, name string, input json.RawMessage) *TrackedTool {
	return &TrackedTool{
		ID:    id,
		Name:  name,
		Input: input,
		State: ToolStateQueued,
	}
}

func (t *TrackedTool) transition(state ToolState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.State = state
	if state == ToolStateExecuting {
		t.StartedAt = time.Now()
	}
	if state == ToolStateCompleted || state == ToolStateAborted {
		t.DoneAt = time.Now()
	}
}

func (t *TrackedTool) complete(result ToolExecResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Result = &result
	t.State = ToolStateCompleted
	t.DoneAt = time.Now()
}

func (t *TrackedTool) abort() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.State == ToolStateQueued || t.State == ToolStateExecuting {
		t.State = ToolStateAborted
		t.DoneAt = time.Now()
	}
}

// StreamingToolExecutor runs tool calls with state tracking and progress events.
type StreamingToolExecutor struct {
	skillClient    *SkillRuntimeClient
	mcpBridge      *MCPToolBridge
	hookExecutor   *HookExecutor
	cache          *ToolResultCache
	stallDetector  *StallDetector
	config         RunConfig
}

// NewStreamingToolExecutor creates a StreamingToolExecutor.
func NewStreamingToolExecutor(skillClient *SkillRuntimeClient, hookExecutor *HookExecutor, config RunConfig) *StreamingToolExecutor {
	var cache *ToolResultCache
	if config.ToolCacheCapacity > 0 {
		cache = NewToolResultCache(config.ToolCacheCapacity)
	}
	var detector *StallDetector
	if config.StallThreshold > 0 {
		detector = NewStallDetector(config.StallCheckInterval, config.StallThreshold)
	}
	return &StreamingToolExecutor{
		skillClient:   skillClient,
		hookExecutor:  hookExecutor,
		cache:         cache,
		stallDetector: detector,
		config:        config,
	}
}

func (e *StreamingToolExecutor) emitToolResult(ch chan<- RunEvent, tt *TrackedTool, result ToolExecResult) {
	duration := int64(0)
	if !tt.StartedAt.IsZero() && !tt.DoneAt.IsZero() {
		duration = tt.DoneAt.Sub(tt.StartedAt).Milliseconds()
	}

	ch <- NewRunEvent(EventToolResult, ToolResultData{
		ID:         tt.ID,
		Name:       tt.Name,
		Output:     result.Output,
		DurationMs: duration,
		Error:      result.Error,
	})
}

// WithMCPBridge attaches an MCP tool bridge for routing mcp__ prefixed tool calls.
func (e *StreamingToolExecutor) WithMCPBridge(bridge *MCPToolBridge) *StreamingToolExecutor {
	e.mcpBridge = bridge
	return e
}

// Cache returns the tool result cache (may be nil if caching is disabled).
func (e *StreamingToolExecutor) Cache() *ToolResultCache {
	return e.cache
}

// ToolBatch represents a group of tool calls that share the same concurrency policy.
// Consecutive read-only tools form a single concurrent batch; each write tool forms
// its own serial batch. This preserves LLM-intended ordering while maximizing parallelism.
//
// Inspired by Claude Code's partitionToolCalls() in services/tools/toolOrchestration.ts.
type ToolBatch struct {
	// IsConcurrencySafe indicates whether tools in this batch can run in parallel.
	IsConcurrencySafe bool
	// Indices are the positions in the original toolCalls slice.
	Indices []int
}

// PartitionToolCalls splits tool calls into batches of consecutive read-only tools
// (run concurrently) and individual write tools (run serially). This preserves the
// LLM's intended execution order while maximizing parallelism for safe operations.
//
// The readOnlyIndex supplements the hardcoded readOnlySlugs map with the DB
// read_only flag computed by ToolSchemaBuilder. A tool is considered read-only if
// EITHER source marks it as such. Pass nil to use only the hardcoded map.
//
// Example: [read, read, write, read] → [{read,read}, {write}, {read}]
//
// Inspired by Claude Code's partitionToolCalls() in services/tools/toolOrchestration.ts.
func PartitionToolCalls(toolCalls []ai.ToolCall, readOnlyIndex map[string]bool) []ToolBatch {
	if len(toolCalls) == 0 {
		return nil
	}

	var batches []ToolBatch
	for i, tc := range toolCalls {
		isSafe := IsReadOnlyTool(tc.Function.Name) || readOnlyIndex[tc.Function.Name]

		// Extend the last batch if both are concurrency-safe.
		if isSafe && len(batches) > 0 && batches[len(batches)-1].IsConcurrencySafe {
			batches[len(batches)-1].Indices = append(batches[len(batches)-1].Indices, i)
		} else {
			batches = append(batches, ToolBatch{
				IsConcurrencySafe: isSafe,
				Indices:           []int{i},
			})
		}
	}
	return batches
}

// BuildReadOnlyIndex creates a name→bool map from LLMTool definitions.
// A tool is considered concurrency-safe if either ReadOnly or ConcurrencySafe is true.
// Used to pass the DB flags from ToolSchemaBuilder to PartitionToolCalls.
func BuildReadOnlyIndex(tools []LLMTool) map[string]bool {
	idx := make(map[string]bool, len(tools))
	for _, t := range tools {
		if t.ReadOnly || t.ConcurrencySafe {
			idx[t.Name] = true
		}
	}
	return idx
}

// BuildDestructiveIndex creates a name→bool map from LLMTool definitions.
// Used to auto-require confirmation for destructive tools even in permissive modes.
// Inspired by Claude Code's isDestructive per-tool flag.
func BuildDestructiveIndex(tools []LLMTool) map[string]bool {
	idx := make(map[string]bool, len(tools))
	for _, t := range tools {
		if t.IsDestructive {
			idx[t.Name] = true
		}
	}
	return idx
}

// BuildMaxResultIndex creates a name→maxChars map from LLMTool definitions.
// Tools with non-zero MaxResultChars override the global limit.
func BuildMaxResultIndex(tools []LLMTool) map[string]int {
	idx := make(map[string]int)
	for _, t := range tools {
		if t.MaxResultChars > 0 {
			idx[t.Name] = t.MaxResultChars
		}
	}
	return idx
}

// BuildAllowedToolsIndex creates a name→[]string map from LLMTool definitions.
// Each entry maps a skill/tool name to the set of tools it can use when invoked
// as a worker. Empty slice means all tools are allowed.
// Propagated from skill.AllowedTools (see migration 000018).
// Inspired by Claude Code's BundledSkillDefinition.allowedTools.
func BuildAllowedToolsIndex(tools []LLMTool) map[string][]string {
	idx := make(map[string][]string)
	for _, t := range tools {
		if len(t.AllowedTools) > 0 {
			idx[t.Name] = t.AllowedTools
		}
	}
	return idx
}

// BuildContextModeIndex creates a name→mode map from LLMTool definitions.
// Tools with ContextMode="fork" run in a sub-agent with isolated context.
// Inspired by Claude Code's BundledSkillDefinition.context ('inline' | 'fork').
func BuildContextModeIndex(tools []LLMTool) map[string]string {
	idx := make(map[string]string)
	for _, t := range tools {
		if t.ContextMode != "" && t.ContextMode != "inline" {
			idx[t.Name] = t.ContextMode
		}
	}
	return idx
}

// BuildInterruptBehaviorIndex creates a name→behavior map from LLMTool definitions.
// Only tools with InterruptBehavior="block" are included; absent/empty means "cancel".
// Used by the SSE handler to decide whether to wait for tool completion before stopping.
// Inspired by Claude Code's Tool.ts interruptBehavior(): 'cancel' | 'block'.
func BuildInterruptBehaviorIndex(tools []LLMTool) map[string]string {
	idx := make(map[string]string)
	for _, t := range tools {
		if t.InterruptBehavior == "block" {
			idx[t.Name] = "block"
		}
	}
	return idx
}

// BuildSearchOrReadIndex creates a name→bool map from LLMTool definitions.
// Tools with IsSearchOrRead=true should have their results auto-collapsed in the UI.
// Inspired by Claude Code's Tool.ts isSearchOrReadCommand().
func BuildSearchOrReadIndex(tools []LLMTool) map[string]bool {
	idx := make(map[string]bool)
	for _, t := range tools {
		if t.IsSearchOrRead {
			idx[t.Name] = true
		}
	}
	return idx
}

// ExecuteAll runs all tool calls with state tracking, hooks, and abort cascade.
// Tool calls are partitioned into batches: consecutive read-only tools run concurrently,
// write tools run serially. This preserves LLM-intended ordering while maximizing parallelism.
//
// readOnlyIndex supplements the hardcoded map with DB-sourced read_only flags.
// Pass nil to use only the hardcoded readOnlySlugs map.
//
// Inspired by Claude Code's runTools() in services/tools/toolOrchestration.ts:
// "Partition tool calls into batches where each batch is either a single non-read-only
// tool, or multiple consecutive read-only tools."
func (e *StreamingToolExecutor) ExecuteAll(
	ctx context.Context,
	ch chan<- RunEvent,
	toolCalls []ai.ToolCall,
	in RunInput,
	readOnlyIndex map[string]bool,
) []ToolExecResult {
	// Create tracked tools.
	tracked := make([]*TrackedTool, len(toolCalls))
	for i, tc := range toolCalls {
		tracked[i] = newTrackedTool(tc.ID, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
	}

	// Emit queued state for all.
	for _, t := range tracked {
		ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
			ID:    t.ID,
			Name:  t.Name,
			Input: t.Input,
		})
		ch <- NewRunEvent(EventToolProgress, ToolProgressData{
			ID:    t.ID,
			Name:  t.Name,
			State: ToolStateQueued,
		})
	}

	results := make([]ToolExecResult, len(toolCalls))

	// Partition into batches preserving order.
	batches := PartitionToolCalls(toolCalls, readOnlyIndex)

	for _, batch := range batches {
		if batch.IsConcurrencySafe && len(batch.Indices) > 1 {
			// Concurrent batch: run read-only tools in parallel.
			e.executeParallel(ctx, ch, toolCalls, tracked, results, batch.Indices, in)
		} else {
			// Serial batch: run each tool sequentially.
			for _, idx := range batch.Indices {
				if ctx.Err() != nil {
					tracked[idx].abort()
					ch <- NewRunEvent(EventToolProgress, ToolProgressData{
						ID: tracked[idx].ID, Name: tracked[idx].Name, State: ToolStateAborted,
					})
					errMsg := "aborted: context cancelled"
					results[idx] = ToolExecResult{Error: &errMsg}
					continue
				}
				e.executeSingle(ctx, ch, toolCalls[idx], tracked[idx], &results[idx], in)
			}
		}
	}

	return results
}

// executeParallel runs tools at the given indices concurrently with a semaphore.
func (e *StreamingToolExecutor) executeParallel(
	ctx context.Context,
	ch chan<- RunEvent,
	toolCalls []ai.ToolCall,
	tracked []*TrackedTool,
	results []ToolExecResult,
	indices []int,
	in RunInput,
) {
	sem := make(chan struct{}, e.config.ConcurrentReadTools)
	abortCtx, abortCancel := context.WithCancel(ctx)
	defer abortCancel()

	var wg sync.WaitGroup
	var firstError bool
	var errorMu sync.Mutex

	for _, idx := range indices {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tc := toolCalls[i]
			tt := tracked[i]

			if abortCtx.Err() != nil {
				tt.abort()
				ch <- NewRunEvent(EventToolProgress, ToolProgressData{
					ID: tt.ID, Name: tt.Name, State: ToolStateAborted,
				})
				errMsg := "aborted: sibling tool failed"
				results[i] = ToolExecResult{Error: &errMsg}
				return
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			if abortCtx.Err() != nil {
				tt.abort()
				ch <- NewRunEvent(EventToolProgress, ToolProgressData{
					ID: tt.ID, Name: tt.Name, State: ToolStateAborted,
				})
				errMsg := "aborted: sibling tool failed"
				results[i] = ToolExecResult{Error: &errMsg}
				return
			}

		// Validate tool input before execution.
			toolInput := json.RawMessage(tc.Function.Arguments)
			if vErr := ValidateToolInput(tc.Function.Name, toolInput); vErr != "" {
				errMsg := vErr
				validationResult := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name}
				tt.complete(validationResult)
				results[i] = validationResult
				ch <- NewRunEvent(EventToolProgress, ToolProgressData{
					ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
				})
				e.emitToolResult(ch, tt, validationResult)
				return
			}

			// Check cache for cacheable tools.
			if e.cache != nil && IsCacheable(tc.Function.Name) {
				if cached := e.cache.Get(tc.Function.Name, toolInput); cached != nil {
					tt.complete(*cached)
					results[i] = truncateToolResult(*cached, e.config.MaxToolResultChars)
					ch <- NewRunEvent(EventToolProgress, ToolProgressData{
						ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
					})
					e.emitToolResult(ch, tt, results[i])
					return
				}
			}

			// Transition to executing.
			tt.transition(ToolStateExecuting)
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tt.ID, Name: tt.Name, State: ToolStateExecuting,
			})

			// Start stall detection for this tool.
			var stallMon *StallMonitor
			if e.stallDetector != nil {
				stallMon = e.stallDetector.Monitor(
					tt.ID, tt.Name,
					func(toolID, toolName string) {
						ch <- NewRunEvent(EventToolProgress, ToolProgressData{
							ID: toolID, Name: toolName, State: ToolStateStalled,
						})
					},
					func(toolID, toolName string) {
						ch <- NewRunEvent(EventToolProgress, ToolProgressData{
							ID: toolID, Name: toolName, State: ToolStateExecuting,
						})
					},
				)
			}

			// Pre-tool hooks.
			if e.hookExecutor != nil {
				e.hookExecutor.Execute(ctx, HookPayload{
					Event:     HookPreToolUse,
					AgentID:   in.AgentID.String(),
					SessionID: in.SessionID.String(),
					ToolName:  tc.Function.Name,
					ToolInput: toolInput,
				})
			}

			// Execute the tool.
			toolCtx := abortCtx
			if e.config.ToolTimeout > 0 {
				var cancel context.CancelFunc
				toolCtx, cancel = context.WithTimeout(abortCtx, e.config.ToolTimeout)
				defer cancel()
			}

			var execResult *ToolExecResult
			var execErr error

			// Route MCP tool calls to the MCPToolBridge.
			if IsMCPToolCall(tc.Function.Name) && e.mcpBridge != nil {
				execResult, execErr = e.mcpBridge.Execute(toolCtx, tc.Function.Name, toolInput)
			} else {
				execResult, execErr = e.skillClient.Execute(
					toolCtx,
					tc.Function.Name,
					toolInput,
					in.TenantID, in.AgentID.String(), in.SessionID.String(),
				)
			}

			// Stop stall monitoring — tool execution completed.
			if stallMon != nil {
				stallMon.Stop()
			}

			if execErr != nil {
				errMsg := execErr.Error()
				failResult := ToolExecResult{Error: &errMsg}
				tt.complete(failResult)
				results[i] = truncateToolResult(failResult, e.config.MaxToolResultChars)

				// Abort cascade: cancel siblings on error.
				errorMu.Lock()
				if !firstError {
					firstError = true
					abortCancel()
				}
				errorMu.Unlock()
			} else {
				tt.complete(*execResult)
				results[i] = truncateToolResult(*execResult, e.config.MaxToolResultChars)

				// Cache successful results for cacheable tools.
				if e.cache != nil && IsCacheable(tc.Function.Name) && execResult.Error == nil {
					e.cache.Put(tc.Function.Name, toolInput, *execResult)
				}
			}

			// Completed state.
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
			})
			e.emitToolResult(ch, tt, results[i])

			// Post-tool hooks.
			if e.hookExecutor != nil {
				e.hookExecutor.Execute(ctx, HookPayload{
					Event:      HookPostToolUse,
					AgentID:    in.AgentID.String(),
					SessionID:  in.SessionID.String(),
					ToolName:   tc.Function.Name,
					ToolInput:  toolInput,
					ToolOutput: results[i].Output,
					ToolError:  results[i].Error,
				})
			}
		}(idx)
	}

	wg.Wait()
}

// executeSingle runs a single tool call synchronously.
func (e *StreamingToolExecutor) executeSingle(
	ctx context.Context,
	ch chan<- RunEvent,
	tc ai.ToolCall,
	tt *TrackedTool,
	result *ToolExecResult,
	in RunInput,
) {
	e.executeToolCall(ctx, ch, tc, tt, result, in)
}

// executeToolCall handles the execution of a single tool call including hooks.
// Returns true if the execution resulted in an error.
func (e *StreamingToolExecutor) executeToolCall(
	ctx context.Context,
	ch chan<- RunEvent,
	tc ai.ToolCall,
	tt *TrackedTool,
	result *ToolExecResult,
	in RunInput,
) bool {
	// Phase 1: Validate input before permission checks or execution.
	// Structural validation catches missing params and invalid types early,
	// returning LLM-readable errors without showing permission dialogs.
	// Inspired by Claude Code's Tool.ts two-phase validateInput/checkPermissions.
	if vErr := ValidateToolInput(tc.Function.Name, json.RawMessage(tc.Function.Arguments)); vErr != "" {
		errMsg := vErr
		validationResult := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name}
		tt.complete(validationResult)
		*result = validationResult
		ch <- NewRunEvent(EventToolProgress, ToolProgressData{
			ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
		})
		e.emitToolResult(ch, tt, validationResult)
		return true
	}

	tt.transition(ToolStateExecuting)
	ch <- NewRunEvent(EventToolProgress, ToolProgressData{
		ID: tt.ID, Name: tt.Name, State: ToolStateExecuting,
	})

	// Pre-tool hooks.
	if e.hookExecutor != nil {
		e.hookExecutor.Execute(ctx, HookPayload{
			Event:     HookPreToolUse,
			AgentID:   in.AgentID.String(),
			SessionID: in.SessionID.String(),
			ToolName:  tc.Function.Name,
			ToolInput: json.RawMessage(tc.Function.Arguments),
		})
	}

	// Execute the tool.
	toolCtx := ctx
	if e.config.ToolTimeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(ctx, e.config.ToolTimeout)
		defer cancel()
	}

	execResult, err := e.skillClient.Execute(
		toolCtx,
		tc.Function.Name,
		json.RawMessage(tc.Function.Arguments),
		in.TenantID, in.AgentID.String(), in.SessionID.String(),
	)

	hasErr := false
	if err != nil {
		errMsg := err.Error()
		execRes := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name}
		tt.complete(execRes)
		*result = truncateToolResult(execRes, e.config.MaxToolResultChars)
		hasErr = true
	} else {
		execResult.ToolName = tc.Function.Name
		tt.complete(*execResult)
		*result = truncateToolResult(*execResult, e.config.MaxToolResultChars)
	}

	ch <- NewRunEvent(EventToolProgress, ToolProgressData{
		ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
	})
	e.emitToolResult(ch, tt, *result)

	// Post-tool hooks.
	if e.hookExecutor != nil {
		e.hookExecutor.Execute(ctx, HookPayload{
			Event:      HookPostToolUse,
			AgentID:    in.AgentID.String(),
			SessionID:  in.SessionID.String(),
			ToolName:   tc.Function.Name,
			ToolInput:  json.RawMessage(tc.Function.Arguments),
			ToolOutput: result.Output,
			ToolError:  result.Error,
		})

		// Post-tool-failure hooks — fired only when tool execution failed.
		// Inspired by Claude Code's PostToolFailure hook event.
		if hasErr {
			e.hookExecutor.Execute(ctx, HookPayload{
				Event:     HookPostToolFailure,
				AgentID:   in.AgentID.String(),
				SessionID: in.SessionID.String(),
				ToolName:  tc.Function.Name,
				ToolInput: json.RawMessage(tc.Function.Arguments),
				ToolError: result.Error,
			})
		}
	}

	return hasErr
}

// --- Input validation ---

// ValidateToolInput performs structural validation on tool input before permission
// checks or execution. Returns an empty string if valid, or an LLM-readable error
// message explaining what's wrong.
//
// This runs before checkPermissions to avoid showing permission dialogs for
// structurally invalid inputs (e.g. missing required params, invalid JSON).
// Inspired by Claude Code's Tool.ts validateInput phase.
func ValidateToolInput(toolName string, input json.RawMessage) string {
	// Basic JSON validity check.
	if len(input) == 0 {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal(input, &parsed); err != nil {
		return fmt.Sprintf("Invalid JSON input for tool '%s': %s", toolName, err.Error())
	}

	// Tool-specific validation for builtins.
	switch toolName {
	case agentToolName:
		prompt, _ := parsed["prompt"].(string)
		if prompt == "" {
			return "The required parameter 'prompt' is missing for the agent tool."
		}
	case "document_search":
		query, _ := parsed["query"].(string)
		if query == "" {
			return "The required parameter 'query' is missing for document_search."
		}
	case "memory_store":
		content, _ := parsed["content"].(string)
		if content == "" {
			return "The required parameter 'content' is missing for memory_store."
		}
	}

	return ""
}

// FormatToolError produces an LLM-readable error string with head+tail preservation
// for long errors. Errors exceeding maxChars keep the first half and last half with
// a truncation notice in the middle. This preserves both the error type (usually at
// the start) and the stack trace / details (usually at the end).
// Inspired by Claude Code's formatError in utils/toolErrors.ts.
func FormatToolError(errMsg string, maxChars int) string {
	if maxChars <= 0 || len(errMsg) <= maxChars {
		return errMsg
	}
	half := maxChars / 2
	notice := fmt.Sprintf("\n... [%d chars truncated] ...\n", len(errMsg)-maxChars)
	headSize := half
	tailSize := half
	// Ensure we don't exceed maxChars with the notice.
	if headSize+tailSize+len(notice) > maxChars {
		headSize = (maxChars - len(notice)) / 2
		tailSize = maxChars - len(notice) - headSize
	}
	return errMsg[:headSize] + notice + errMsg[len(errMsg)-tailSize:]
}
