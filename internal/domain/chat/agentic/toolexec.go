package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
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
	skillClient   *SkillRuntimeClient
	mcpBridge     *MCPToolBridge
	hookExecutor  *HookExecutor
	cache         *ToolResultCache
	stallDetector *StallDetector
	config        RunConfig
	// P-C179-1: document_search is executed locally (not delegated to skill-runtime).
	docSearch   knowledge.DocumentSearchClient
	activeKBIDs []uuid.UUID
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

	// Redact credential-bearing fields before emitting the SSE tool_result event.
	output := result.Output
	if len(output) > 0 {
		output = RedactSensitiveFields(output)
	}

	ch <- NewRunEvent(EventToolResult, ToolResultData{
		ID:         tt.ID,
		Name:       tt.Name,
		Output:     output,
		DurationMs: duration,
		Error:      result.Error,
	})
}

// WithMCPBridge attaches an MCP tool bridge for routing mcp__ prefixed tool calls.
func (e *StreamingToolExecutor) WithMCPBridge(bridge *MCPToolBridge) *StreamingToolExecutor {
	e.mcpBridge = bridge
	return e
}

// WithDocumentSearch wires the local document search client and the set of active
// knowledge base IDs for this run. When set, document_search calls are executed
// locally via pgvector (P-C179-1) instead of being delegated to the skill-runtime.
func (e *StreamingToolExecutor) WithDocumentSearch(client knowledge.DocumentSearchClient, kbIDs []uuid.UUID) *StreamingToolExecutor {
	e.docSearch = client
	e.activeKBIDs = kbIDs
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
				validationResult := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name, EmittedToStream: true}
				tt.complete(validationResult)
				results[i] = validationResult
				ch <- NewRunEvent(EventToolProgress, ToolProgressData{
					ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
				})
				e.emitToolResult(ch, tt, validationResult)
				return
			}

			// Pre-tool hooks may block execution or rewrite the tool input.
			if e.hookExecutor != nil {
				hookResults := e.hookExecutor.Execute(ctx, HookPayload{
					Event:     HookPreToolUse,
					AgentID:   in.AgentID.String(),
					SessionID: in.SessionID.String(),
					ToolName:  tc.Function.Name,
					ToolInput: toolInput,
				})
				var blocked *string
				toolInput, blocked = applyPreToolHookResults(hookResults, toolInput)
				if blocked != nil {
					blockedResult := ToolExecResult{Error: blocked, ToolName: tc.Function.Name, EmittedToStream: true}
					tt.complete(blockedResult)
					results[i] = blockedResult
					ch <- NewRunEvent(EventToolProgress, ToolProgressData{
						ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
					})
					e.emitToolResult(ch, tt, blockedResult)
					return
				}
				if vErr := ValidateToolInput(tc.Function.Name, toolInput); vErr != "" {
					errMsg := vErr
					validationResult := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name, EmittedToStream: true}
					tt.complete(validationResult)
					results[i] = validationResult
					ch <- NewRunEvent(EventToolProgress, ToolProgressData{
						ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
					})
					e.emitToolResult(ch, tt, validationResult)
					return
				}
			}

			// Check cache for cacheable tools.
			if e.cache != nil && IsCacheable(tc.Function.Name) {
				if cached := e.cache.Get(tc.Function.Name, toolInput); cached != nil {
					tt.complete(*cached)
					results[i] = truncateToolResult(*cached, e.config.MaxToolResultChars)
					results[i].EmittedToStream = true
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

			// Execute the tool.
			toolCtx := abortCtx
			if e.config.ToolTimeout > 0 {
				var cancel context.CancelFunc
				toolCtx, cancel = context.WithTimeout(abortCtx, e.config.ToolTimeout)
				defer cancel()
			}

			var execResult *ToolExecResult
			var execErr error

			// Route document_search locally (P-C179-1), then MCP, then skill-runtime.
			if tc.Function.Name == "document_search" && e.docSearch != nil {
				execResult, execErr = executeDocumentSearchInternal(toolCtx, e.docSearch, e.activeKBIDs, toolInput)
			} else if tc.Function.Name == "document_search" && e.docSearch == nil {
				// P-E1-1: docSearch client not wired — surface clear error instead of delegating to
				// skill-runtime (which returns the confusing "skill not found" message).
				msg := "Document search is not available for this agent. The knowledge base search client is not connected. Please check that the agent has an active knowledge base linked."
				execResult = &ToolExecResult{Error: &msg}
			} else if IsMCPToolCall(tc.Function.Name) && e.mcpBridge != nil {
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
			results[i].EmittedToStream = true
			e.emitToolResult(ch, tt, results[i])

			// Post-tool hooks. Prompt hooks may inject extra text into the
			// tool result so the LLM sees it in the next turn.
			if e.hookExecutor != nil {
				hookResults := e.hookExecutor.Execute(ctx, HookPayload{
					Event:      HookPostToolUse,
					AgentID:    in.AgentID.String(),
					SessionID:  in.SessionID.String(),
					ToolName:   tc.Function.Name,
					ToolInput:  toolInput,
					ToolOutput: results[i].Output,
					ToolError:  results[i].Error,
				})
				// BUG-HOOK-PROMPT-INJECT fix: store InjectText separately. Transform
				// hooks may also rewrite the output seen by the next LLM turn.
				applyPostToolHookResults(hookResults, &results[i])
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
	toolInput := json.RawMessage(tc.Function.Arguments)

	// Phase 1: Validate input before permission checks or execution.
	// Structural validation catches missing params and invalid types early,
	// returning LLM-readable errors without showing permission dialogs.
	// Inspired by Claude Code's Tool.ts two-phase validateInput/checkPermissions.
	if vErr := ValidateToolInput(tc.Function.Name, toolInput); vErr != "" {
		errMsg := vErr
		validationResult := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name, EmittedToStream: true}
		tt.complete(validationResult)
		*result = validationResult
		ch <- NewRunEvent(EventToolProgress, ToolProgressData{
			ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
		})
		e.emitToolResult(ch, tt, validationResult)
		return true
	}

	// Pre-tool hooks may block execution or rewrite the tool input.
	if e.hookExecutor != nil {
		hookResults := e.hookExecutor.Execute(ctx, HookPayload{
			Event:     HookPreToolUse,
			AgentID:   in.AgentID.String(),
			SessionID: in.SessionID.String(),
			ToolName:  tc.Function.Name,
			ToolInput: toolInput,
		})
		var blocked *string
		toolInput, blocked = applyPreToolHookResults(hookResults, toolInput)
		if blocked != nil {
			blockedResult := ToolExecResult{Error: blocked, ToolName: tc.Function.Name, EmittedToStream: true}
			tt.complete(blockedResult)
			*result = blockedResult
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
			})
			e.emitToolResult(ch, tt, blockedResult)
			return true
		}
		if vErr := ValidateToolInput(tc.Function.Name, toolInput); vErr != "" {
			errMsg := vErr
			validationResult := ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name, EmittedToStream: true}
			tt.complete(validationResult)
			*result = validationResult
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
			})
			e.emitToolResult(ch, tt, validationResult)
			return true
		}
	}

	tt.transition(ToolStateExecuting)
	ch <- NewRunEvent(EventToolProgress, ToolProgressData{
		ID: tt.ID, Name: tt.Name, State: ToolStateExecuting,
	})

	// Execute the tool.
	toolCtx := ctx
	if e.config.ToolTimeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(ctx, e.config.ToolTimeout)
		defer cancel()
	}

	// Route document_search locally (P-C179-1), then MCP, then skill-runtime.
	var execResult *ToolExecResult
	var err error
	if tc.Function.Name == "document_search" && e.docSearch != nil {
		execResult, err = executeDocumentSearchInternal(toolCtx, e.docSearch, e.activeKBIDs, toolInput)
	} else if tc.Function.Name == "document_search" && e.docSearch == nil {
		// P-E1-1: docSearch client not wired — surface clear error instead of delegating to
		// skill-runtime (which returns the confusing "skill not found" message).
		msg := "Document search is not available for this agent. The knowledge base search client is not connected. Please check that the agent has an active knowledge base linked."
		execResult = &ToolExecResult{Error: &msg}
	} else if IsMCPToolCall(tc.Function.Name) && e.mcpBridge != nil {
		execResult, err = e.mcpBridge.Execute(toolCtx, tc.Function.Name, toolInput)
	} else {
		execResult, err = e.skillClient.Execute(
			toolCtx,
			tc.Function.Name,
			toolInput,
			in.TenantID, in.AgentID.String(), in.SessionID.String(),
		)
	}

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
	// Bug 198: marca EmittedToStream para o runner pular re-emissão
	// no main loop (runner.go:1311). Sem essa flag, tool_result era
	// emitido 2x na SSE com durationMs ligeiramente diferente.
	result.EmittedToStream = true

	// Post-tool hooks.
	if e.hookExecutor != nil {
		hookResults := e.hookExecutor.Execute(ctx, HookPayload{
			Event:      HookPostToolUse,
			AgentID:    in.AgentID.String(),
			SessionID:  in.SessionID.String(),
			ToolName:   tc.Function.Name,
			ToolInput:  toolInput,
			ToolOutput: result.Output,
			ToolError:  result.Error,
		})
		// BUG-HOOK-PROMPT-INJECT fix (serial path): collect InjectText separately.
		// Transform hooks may also rewrite output before the next LLM turn.
		applyPostToolHookResults(hookResults, result)

		// Post-tool-failure hooks — fired only when tool execution failed.
		// Inspired by Claude Code's PostToolFailure hook event.
		if hasErr {
			for _, event := range []HookEvent{HookPostToolFailure, HookOnError} {
				e.hookExecutor.Execute(ctx, HookPayload{
					Event:     event,
					AgentID:   in.AgentID.String(),
					SessionID: in.SessionID.String(),
					ToolName:  tc.Function.Name,
					ToolInput: toolInput,
					ToolError: result.Error,
				})
			}
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

// executeDocumentSearchInternal routes a document_search tool call to the local
// DocumentSearchClient (pgvector). It decodes the tool arguments, calls Search,
// and returns the results serialised as JSON.
func executeDocumentSearchInternal(ctx context.Context, client knowledge.DocumentSearchClient, kbIDs []uuid.UUID, rawArgs json.RawMessage) (*ToolExecResult, error) {
	var args struct {
		Query           string `json:"query"`
		TopK            int    `json:"top_k"`
		Limit           int    `json:"limit"`             // BUG-DOCSEARCH-PARAMS: alias accepted from LLM schema
		KnowledgeBaseID string `json:"knowledge_base_id"` // BUG-DOCSEARCH-PARAMS: optional KB filter
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("document_search: invalid arguments: %w", err)
	}
	// Prefer limit over top_k (limit is the schema-visible field name).
	if args.Limit > 0 && args.TopK <= 0 {
		args.TopK = args.Limit
	}
	if args.TopK <= 0 {
		args.TopK = 5
	}

	// BUG-DOCSEARCH-PARAMS: when the LLM provides knowledge_base_id, restrict the search
	// to that KB only — but only if it is already in the agent's allowed kbIDs list.
	// This prevents the LLM from searching arbitrary KBs beyond what the agent can access.
	effectiveKBIDs := kbIDs
	if args.KnowledgeBaseID != "" {
		if kbID, err := uuid.Parse(args.KnowledgeBaseID); err == nil {
			allowed := false
			for _, id := range kbIDs {
				if id == kbID {
					allowed = true
					break
				}
			}
			if allowed {
				effectiveKBIDs = []uuid.UUID{kbID}
			}
			// When kbIDs is nil (search all active KBs), any valid UUID is allowed.
			if kbIDs == nil {
				effectiveKBIDs = []uuid.UUID{kbID}
			}
		}
	}

	results, err := client.Search(ctx, args.Query, effectiveKBIDs, args.TopK)
	if err != nil {
		return nil, fmt.Errorf("document_search: search failed: %w", err)
	}

	// BUG-KB-PAUSE-TRANSPARENT: when no results are returned, include a diagnostic
	// note so the LLM understands the possible cause instead of silently receiving [].
	// The search client filters out PAUSED knowledge bases — an empty result may mean
	// "no relevant content" OR "all knowledge bases are currently paused". Without this
	// note the LLM retries blindly or fabricates an answer.
	if len(results) == 0 {
		type emptyResult struct {
			Results []knowledge.SearchResult `json:"results"`
			Note    string                   `json:"note"`
		}
		out, err := json.Marshal(emptyResult{
			Results: []knowledge.SearchResult{},
			Note:    "No matching documents found. The knowledge base may not contain content relevant to this query, or all associated knowledge bases may currently be paused. Do not retry — inform the user that the information is not available.",
		})
		if err != nil {
			return nil, fmt.Errorf("document_search: failed to marshal empty result: %w", err)
		}
		return &ToolExecResult{Output: out, ToolName: "document_search"}, nil
	}

	out, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("document_search: failed to marshal results: %w", err)
	}
	return &ToolExecResult{Output: out, ToolName: "document_search"}, nil
}

// ExecuteDocumentSearch is the exported entry point for unit tests and wiring.
// It delegates to executeDocumentSearchInternal.
func ExecuteDocumentSearch(ctx context.Context, client knowledge.DocumentSearchClient, kbIDs []uuid.UUID, rawArgs json.RawMessage) (*ToolExecResult, error) {
	return executeDocumentSearchInternal(ctx, client, kbIDs, rawArgs)
}

// stripStatusCodeFromToolOutput removes the `status_code` / `statusCode` key from
// a JSON tool result before it is sent to the LLM. HTTP status codes are transport-
// level metadata that the LLM should not use for reasoning — it should focus on the
// content of the response body. P-C176-1.
//
// If raw is not a JSON object, or neither key is present, the input is returned unchanged
// (preserving exact byte-for-byte formatting of the original JSON).
func stripStatusCodeFromToolOutput(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw // not a JSON object — leave unchanged
	}
	_, hasSnake := m["status_code"]
	_, hasCamel := m["statusCode"]
	if !hasSnake && !hasCamel {
		return raw // neither key present — return original unchanged (preserves formatting)
	}
	delete(m, "status_code")
	delete(m, "statusCode")
	if len(m) == 0 {
		return raw // all keys removed — keep original rather than returning empty object
	}
	sanitised, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return sanitised
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
