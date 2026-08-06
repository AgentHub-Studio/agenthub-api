package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// agentToolName is the builtin tool name for sub-agent spawning.
const agentToolName = "agent"

// SubtaskStatus represents the outcome of a sub-agent run.
type SubtaskStatus string

const (
	SubtaskCompleted SubtaskStatus = "completed"
	SubtaskFailed    SubtaskStatus = "failed"
	SubtaskKilled    SubtaskStatus = "killed"
)

// allSubtaskStatuses is the closed canonical set (mirrors the same
// pattern used by every other bounded enum in this package).
var allSubtaskStatuses = []SubtaskStatus{
	SubtaskCompleted, SubtaskFailed, SubtaskKilled,
}

// IsValidSubtaskStatus returns true for the bounded set. Useful when
// deserialising OBS-006 SubtaskCompleteData from external sources
// (e.g., persisted audit rows, cross-agent replay) and you need to
// reject unknown labels before they flow into the trace pipeline.
func IsValidSubtaskStatus(s SubtaskStatus) bool {
	for _, v := range allSubtaskStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllSubtaskStatuses returns a defensive copy of the closed set —
// useful for catalog UIs, validation tables, and audit emission.
func AllSubtaskStatuses() []SubtaskStatus {
	out := make([]SubtaskStatus, len(allSubtaskStatuses))
	copy(out, allSubtaskStatuses)
	return out
}

// SubtaskResult is the structured envelope for sub-agent results.
// It is serialised as the tool_result output so the LLM can parse it.
type SubtaskResult struct {
	SubtaskID   string        `json:"subtaskId"`
	Status      SubtaskStatus `json:"status"`
	Summary     string        `json:"summary"`
	Result      string        `json:"result"`
	TotalTurns  int           `json:"totalTurns"`
	TotalTokens int           `json:"totalTokens"`
	CostUSD     float64       `json:"costUsd"`
	DurationMs  int64         `json:"durationMs"`
}

// summarizeResult generates a concise summary from the result content or error.
func summarizeResult(content string, err *string) string {
	if err != nil {
		msg := sanitizeToolError(*err)
		if len(msg) > 200 {
			return msg[:197] + "..."
		}
		return "Error: " + msg
	}
	content = sanitizeSSEMessage(content)
	if content == "" {
		return "(no output)"
	}
	if len(content) > 200 {
		return content[:197] + "..."
	}
	return content
}

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
	agentMailbox *AgentMailbox
	// coordinator persists delegated work when task persistence is configured
	// for the parent chat session.
	coordinator *CoordinatorState
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
func (s *SubtaskExecutor) WithAgentMailbox(m *AgentMailbox) *SubtaskExecutor {
	s.agentMailbox = m
	return s
}

// WithCoordinatorState wires task lifecycle persistence for delegated work.
// A nil coordinator preserves the in-memory-only behavior used by isolated
// runners and tests that do not own a persisted chat session.
func (s *SubtaskExecutor) WithCoordinatorState(coordinator *CoordinatorState) *SubtaskExecutor {
	s.coordinator = coordinator
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
	if s.coordinator != nil {
		subtaskID = s.coordinator.CreateTask(input.Prompt, PhaseImplementation, nil)
		if err := s.coordinator.AssignTask(subtaskID, "subagent-"+subtaskID); err != nil {
			slog.Warn("agentic: failed to assign persisted subtask", "taskID", subtaskID, "error", err)
		}
	}

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
			s.recordTaskCompletion(subtaskID, summarizeResult("", &errMsg), &errMsg)
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
	if s.agentMailbox != nil {
		childRunner.WithAgentMailbox(s.agentMailbox)
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
		IsAdmin:            false,           // SEC-01: sub-agents never inherit admin scope
		EnableManagement:   false,           // SEC-01: sub-agents never inherit management tools
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

	latency := time.Since(start).Milliseconds()

	// Build structured result.
	status := SubtaskCompleted
	if childErr != nil {
		status = SubtaskFailed
	}

	if resultContent == "" && childErr == nil {
		resultContent = "(sub-agent completed with no text output)"
	}

	subtaskResult := SubtaskResult{
		SubtaskID:   subtaskID,
		Status:      status,
		Summary:     summarizeResult(resultContent, childErr),
		Result:      resultContent,
		TotalTurns:  totalTurns,
		TotalTokens: totalTokens,
		CostUSD:     childCost,
		DurationMs:  latency,
	}
	s.recordTaskCompletion(subtaskID, subtaskResult.Summary, childErr)

	// Emit subtask_complete with summary.
	parentCh <- NewRunEvent(EventSubtaskComplete, SubtaskCompleteData{
		ID:          subtaskID,
		TotalTurns:  totalTurns,
		TotalTokens: totalTokens,
		TotalCost:   childCost,
		Summary:     subtaskResult.Summary,
		Error:       childErr,
	})

	output, _ := json.Marshal(subtaskResult)
	if childErr != nil {
		return ToolExecResult{
			Output:    output,
			Error:     childErr,
			LatencyMs: latency,
			// P-C336-1 (ACT-F3-13): propagate sub-agent metrics to parent runner.
			SubtaskTokens:  totalTokens,
			SubtaskCostUSD: childCost,
		}
	}

	return ToolExecResult{
		Output:    output,
		LatencyMs: latency,
		// P-C336-1 (ACT-F3-13): propagate sub-agent metrics to parent runner.
		SubtaskTokens:  totalTokens,
		SubtaskCostUSD: childCost,
	}
}

func (s *SubtaskExecutor) recordTaskCompletion(taskID, summary string, runErr *string) {
	if s.coordinator == nil {
		return
	}

	status := TaskStatusCompleted
	if runErr != nil {
		status = TaskStatusFailed
		if err := s.coordinator.FailTask(taskID); err != nil {
			slog.Warn("agentic: failed to mark persisted subtask as failed", "taskID", taskID, "error", err)
		}
	} else if err := s.coordinator.CompleteTask(taskID); err != nil {
		slog.Warn("agentic: failed to mark persisted subtask as completed", "taskID", taskID, "error", err)
	}

	s.coordinator.AddNotification(TaskNotification{
		WorkerID:   "subagent-" + taskID,
		WorkerName: "sub-agent",
		TaskID:     taskID,
		Status:     status,
		Summary:    summary,
		Error:      runErr,
		Timestamp:  time.Now(),
	})
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
