package agentic

import (
	"context"
	"encoding/json"
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
	config         RunConfig
}

// NewStreamingToolExecutor creates a StreamingToolExecutor.
func NewStreamingToolExecutor(skillClient *SkillRuntimeClient, hookExecutor *HookExecutor, config RunConfig) *StreamingToolExecutor {
	return &StreamingToolExecutor{
		skillClient:  skillClient,
		hookExecutor: hookExecutor,
		config:       config,
	}
}

// WithMCPBridge attaches an MCP tool bridge for routing mcp__ prefixed tool calls.
func (e *StreamingToolExecutor) WithMCPBridge(bridge *MCPToolBridge) *StreamingToolExecutor {
	e.mcpBridge = bridge
	return e
}

// ExecuteAll runs all tool calls with state tracking, hooks, and abort cascade.
// Returns results in the same order as the input toolCalls.
func (e *StreamingToolExecutor) ExecuteAll(
	ctx context.Context,
	ch chan<- RunEvent,
	toolCalls []ai.ToolCall,
	in RunInput,
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

	// Execute with concurrency limit and abort cascade.
	results := make([]ToolExecResult, len(toolCalls))
	sem := make(chan struct{}, e.config.ConcurrentReadTools)
	abortCtx, abortCancel := context.WithCancel(ctx)
	defer abortCancel()

	var wg sync.WaitGroup
	var firstError bool
	var errorMu sync.Mutex

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc ai.ToolCall, tt *TrackedTool) {
			defer wg.Done()

			// Check abort before acquiring semaphore.
			if abortCtx.Err() != nil {
				tt.abort()
				ch <- NewRunEvent(EventToolProgress, ToolProgressData{
					ID: tt.ID, Name: tt.Name, State: ToolStateAborted,
				})
				errMsg := "aborted: sibling tool failed"
				results[idx] = ToolExecResult{Error: &errMsg}
				return
			}

			sem <- struct{}{}
			defer func() { <-sem }()

			// Check abort again after acquiring semaphore.
			if abortCtx.Err() != nil {
				tt.abort()
				ch <- NewRunEvent(EventToolProgress, ToolProgressData{
					ID: tt.ID, Name: tt.Name, State: ToolStateAborted,
				})
				errMsg := "aborted: sibling tool failed"
				results[idx] = ToolExecResult{Error: &errMsg}
				return
			}

			// Transition to executing.
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
			toolCtx := abortCtx
			if e.config.ToolTimeout > 0 {
				var cancel context.CancelFunc
				toolCtx, cancel = context.WithTimeout(abortCtx, e.config.ToolTimeout)
				defer cancel()
			}

			toolInput := json.RawMessage(tc.Function.Arguments)
			var result *ToolExecResult
			var err error

			// Route MCP tool calls to the MCPToolBridge.
			if IsMCPToolCall(tc.Function.Name) && e.mcpBridge != nil {
				result, err = e.mcpBridge.Execute(toolCtx, tc.Function.Name, toolInput)
			} else {
				result, err = e.skillClient.Execute(
					toolCtx,
					tc.Function.Name,
					toolInput,
					in.TenantID, in.AgentID.String(), in.SessionID.String(),
				)
			}

			if err != nil {
				errMsg := err.Error()
				execResult := ToolExecResult{Error: &errMsg}
				tt.complete(execResult)
				results[idx] = truncateToolResult(execResult, e.config.MaxToolResultChars)

				// Abort cascade: cancel siblings on error.
				errorMu.Lock()
				if !firstError {
					firstError = true
					abortCancel()
				}
				errorMu.Unlock()
			} else {
				tt.complete(*result)
				results[idx] = truncateToolResult(*result, e.config.MaxToolResultChars)
			}

			// Completed state.
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tt.ID, Name: tt.Name, State: ToolStateCompleted,
			})

			// Post-tool hooks.
			if e.hookExecutor != nil {
				e.hookExecutor.Execute(ctx, HookPayload{
					Event:      HookPostToolUse,
					AgentID:    in.AgentID.String(),
					SessionID:  in.SessionID.String(),
					ToolName:   tc.Function.Name,
					ToolInput:  json.RawMessage(tc.Function.Arguments),
					ToolOutput: results[idx].Output,
					ToolError:  results[idx].Error,
				})
			}
		}(i, tc, tracked[i])
	}

	wg.Wait()
	return results
}
