package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// agentToolName is the builtin tool name for sub-agent spawning.
const agentToolName = "agent"

// SubtaskInput is the expected JSON input for the agent builtin tool.
type SubtaskInput struct {
	// Prompt is the task description for the sub-agent.
	Prompt string `json:"prompt"`
	// Tools is an optional list of tool names the sub-agent should use.
	// If empty, the sub-agent inherits all parent tools.
	Tools []string `json:"tools,omitempty"`
}

// SubtaskExecutor intercepts "agent" tool calls and spawns a child Runner
// that shares the parent's tenant, agent, skills, and budget.
type SubtaskExecutor struct {
	// runnerFactory creates a new Runner for the sub-agent.
	// It receives the parent's RunConfig (with adjusted budget/depth).
	runnerFactory RunnerFactory
	// mailbox is the shared inter-agent mailbox for sub-agent communication.
	mailbox *Mailbox
}

// RunnerFactory builds a Runner with the given configuration.
// This allows the SubtaskExecutor to create child runners without
// knowing about all the Runner dependencies.
type RunnerFactory interface {
	NewRunner(config RunConfig) *Runner
}

// NewSubtaskExecutor creates a SubtaskExecutor.
func NewSubtaskExecutor(factory RunnerFactory) *SubtaskExecutor {
	return &SubtaskExecutor{runnerFactory: factory}
}

// WithMailbox attaches a shared mailbox for inter-agent messaging.
func (s *SubtaskExecutor) WithMailbox(m *Mailbox) *SubtaskExecutor {
	s.mailbox = m
	return s
}

// IsAgentToolCall returns true if the tool call is for the agent builtin.
func IsAgentToolCall(tc ai.ToolCall) bool {
	return tc.Function.Name == agentToolName
}

// Execute runs a sub-agent for the given tool call and returns the result.
// It forwards subtask_start/subtask_complete events to the parent channel.
// The sub-agent runs synchronously — this method blocks until the sub-agent finishes.
func (s *SubtaskExecutor) Execute(
	ctx context.Context,
	parentCh chan<- RunEvent,
	tc ai.ToolCall,
	parentInput RunInput,
	config RunConfig,
	totalCostSoFar float64,
) ToolExecResult {
	start := time.Now()

	// Parse input.
	var input SubtaskInput
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
		errMsg := fmt.Sprintf("invalid agent tool input: %s", err.Error())
		return ToolExecResult{Error: &errMsg, LatencyMs: 0}
	}
	if input.Prompt == "" {
		errMsg := "agent tool requires a 'prompt' parameter"
		return ToolExecResult{Error: &errMsg, LatencyMs: 0}
	}

	// Check depth limit.
	nextDepth := parentInput.CurrentDepth + 1
	if nextDepth > config.MaxDepth {
		errMsg := fmt.Sprintf("maximum sub-agent depth (%d) exceeded", config.MaxDepth)
		return ToolExecResult{Error: &errMsg, LatencyMs: 0}
	}

	subtaskID := uuid.New().String()

	// Emit subtask_start.
	parentCh <- NewRunEvent(EventSubtaskStart, SubtaskStartData{
		ID:          subtaskID,
		Description: truncateString(input.Prompt, 200),
		Depth:       nextDepth,
	})

	// Calculate remaining budget for sub-agent.
	var remainingBudget float64
	effectiveBudget := config.MaxBudgetUSD
	if parentInput.RemainingBudgetUSD > 0 {
		effectiveBudget = parentInput.RemainingBudgetUSD
	}
	if effectiveBudget > 0 {
		remainingBudget = effectiveBudget - totalCostSoFar
		if remainingBudget <= 0 {
			errMsg := "no budget remaining for sub-agent"
			parentCh <- NewRunEvent(EventSubtaskComplete, SubtaskCompleteData{
				ID:    subtaskID,
				Error: &errMsg,
			})
			return ToolExecResult{Error: &errMsg, LatencyMs: time.Since(start).Milliseconds()}
		}
	}

	// Create child config with reduced iterations for sub-agents.
	childConfig := config
	if childConfig.MaxIterations > 10 {
		childConfig.MaxIterations = 10 // Sub-agents have tighter iteration limits.
	}

	// Create child runner.
	childRunner := s.runnerFactory.NewRunner(childConfig)
	if s.mailbox != nil {
		childRunner.WithMailbox(s.mailbox)
	}

	// Create a sub-session ID for the child (ephemeral — not persisted as a chat session).
	subSessionID := uuid.New()

	// Determine the parent session ID for the shared mailbox.
	parentSessionID := parentInput.ParentSessionID
	if parentSessionID == uuid.Nil {
		parentSessionID = parentInput.SessionID
	}

	childInput := RunInput{
		SessionID:          subSessionID,
		AgentID:            parentInput.AgentID,
		UserMessage:        input.Prompt,
		SystemPrompt:       parentInput.SystemPrompt,
		TenantID:           parentInput.TenantID,
		PermissionRules:    parentInput.PermissionRules,
		CurrentDepth:       nextDepth,
		RemainingBudgetUSD: remainingBudget,
		ParentEventCh:      parentCh,
		SubtaskID:          subtaskID,
		ParentSessionID:    parentSessionID,
	}

	// Run the sub-agent and collect results.
	childCh := childRunner.Run(ctx, childInput)

	var resultContent string
	var totalTurns, totalTokens int
	var childCost float64
	var childErr *string

	for ev := range childCh {
		// Forward relevant events to parent with subtask context.
		switch ev.Type {
		case EventTextDelta:
			// Accumulate text for the tool result.
			var td TextDeltaData
			if err := json.Unmarshal(ev.Data, &td); err == nil {
				resultContent += td.Content
			}
			// Forward text deltas so the parent SSE stream shows sub-agent output.
			parentCh <- ev

		case EventToolCallStart, EventToolResult, EventToolProgress:
			// Forward tool events from sub-agent.
			parentCh <- ev

		case EventSubtaskStart, EventSubtaskComplete, EventAgentMessage:
			// Forward nested subtask and agent message events.
			parentCh <- ev

		case EventRunComplete:
			var rc RunCompleteData
			if err := json.Unmarshal(ev.Data, &rc); err == nil {
				totalTurns = rc.TotalTurns
				totalTokens = rc.TotalTokens
				childCost = rc.TotalCost
			}

		case EventError:
			var ed ErrorData
			if err := json.Unmarshal(ev.Data, &ed); err == nil {
				childErr = &ed.Message
			}
		}
	}

	// Emit subtask_complete.
	parentCh <- NewRunEvent(EventSubtaskComplete, SubtaskCompleteData{
		ID:          subtaskID,
		TotalTurns:  totalTurns,
		TotalTokens: totalTokens,
		TotalCost:   childCost,
		Error:       childErr,
	})

	latency := time.Since(start).Milliseconds()

	if childErr != nil {
		return ToolExecResult{
			Error:     childErr,
			LatencyMs: latency,
		}
	}

	if resultContent == "" {
		resultContent = "(sub-agent completed with no text output)"
	}

	output, _ := json.Marshal(map[string]any{
		"result":     resultContent,
		"turns":      totalTurns,
		"tokens":     totalTokens,
		"costUsd":    childCost,
		"subtaskId":  subtaskID,
	})

	return ToolExecResult{
		Output:    output,
		LatencyMs: latency,
	}
}

// ExecuteParallel runs multiple sub-agent tool calls concurrently with a shared
// cost tracker. Returns results in the same order as input.
func (s *SubtaskExecutor) ExecuteParallel(
	ctx context.Context,
	parentCh chan<- RunEvent,
	toolCalls []ai.ToolCall,
	parentInput RunInput,
	config RunConfig,
	initialCost float64,
) []ToolExecResult {
	results := make([]ToolExecResult, len(toolCalls))
	var costAccum atomic.Int64 // atomic cost accumulation in micro-USD

	// Store initial cost as micro-USD for atomic operations.
	costAccum.Store(int64(initialCost * 1_000_000))

	var wg sync.WaitGroup
	// Limit concurrent sub-agents to 3.
	sem := make(chan struct{}, 3)

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc ai.ToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			currentCost := float64(costAccum.Load()) / 1_000_000
			result := s.Execute(ctx, parentCh, tc, parentInput, config, currentCost)
			results[idx] = result

			// Track cost from sub-agent result.
			if result.Output != nil {
				var out map[string]any
				if err := json.Unmarshal(result.Output, &out); err == nil {
					if cost, ok := out["costUsd"].(float64); ok {
						costAccum.Add(int64(cost * 1_000_000))
					}
				}
			}
		}(i, tc)
	}

	wg.Wait()
	return results
}

// truncateString truncates a string to maxLen characters with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
