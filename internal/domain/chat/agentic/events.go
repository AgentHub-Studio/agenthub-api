package agentic

import "encoding/json"

// RunEventType identifies the kind of event emitted by the Runner.
type RunEventType string

const (
	EventTextDelta        RunEventType = "text_delta"
	EventToolCallStart    RunEventType = "tool_call_start"
	EventToolResult       RunEventType = "tool_result"
	EventTurnComplete     RunEventType = "turn_complete"
	EventRunComplete      RunEventType = "run_complete"
	EventError            RunEventType = "error"
	EventContextCompacted RunEventType = "context_compacted"
	EventToolProgress     RunEventType = "tool_progress"
	EventSubtaskStart     RunEventType = "subtask_start"
	EventSubtaskComplete  RunEventType = "subtask_complete"
	EventRunProgress      RunEventType = "run_progress"
	EventModelFallback    RunEventType = "model_fallback"
	EventToolDenied       RunEventType = "tool_denied"
	EventAgentMessage     RunEventType = "agent_message"
	EventThinkingDelta    RunEventType = "thinking_delta"
	EventSubtaskProgress  RunEventType = "subtask_progress"
	EventToolUseSummary   RunEventType = "tool_use_summary"
	EventStopHookSummary  RunEventType = "stop_hook_summary"
	// EventInputRequest is emitted when the agentic loop needs structured user input
	// via an elicitation request (form, URL confirmation, etc.).
	EventInputRequest RunEventType = "input_request"
	// EventHeartbeat is a keep-alive event emitted periodically while the runner
	// is blocked waiting for user input (elicitation). This prevents reverse
	// proxies and browsers from closing the idle SSE connection.
	EventHeartbeat RunEventType = "heartbeat"
	// EventWarning is a non-fatal advisory emitted when a recoverable issue is
	// detected during a run (e.g. a skill with no tools, an unavailable KB).
	EventWarning RunEventType = "warning"
	// EventCanvasUpdate is emitted when the agent produces rich visual output
	// (markdown, HTML table, JSON) intended for display in the canvas panel.
	// The frontend renders this in a sandboxed panel alongside the chat.
	EventCanvasUpdate RunEventType = "canvas_update"
	// EventFrontendActionCall is emitted when the LLM calls a tool that was
	// declared by the Flutter client via `POST /api/chat/sessions/{id}/client-state`.
	// The runner blocks for the result via [FrontendActionHandler.Submit]; the
	// client posts it back through the same `client-state` endpoint with
	// `actionResults: [{id, status, result?, error?}]`.
	EventFrontendActionCall RunEventType = "frontend_action_call"
)

// ToolUseSummaryData carries a human-readable summary of a completed tool batch.
// Inspired by Claude Code's ToolUseSummaryMessage.
type ToolUseSummaryData struct {
	TurnIndex int    `json:"turnIndex"`
	Summary   string `json:"summary"`
}

// RunEvent is the envelope sent through the Runner's output channel.
type RunEvent struct {
	Type RunEventType    `json:"type"`
	Data json.RawMessage `json:"data"`
}

// NewRunEvent serialises the typed payload into a RunEvent.
func NewRunEvent(typ RunEventType, data any) RunEvent {
	raw, _ := json.Marshal(data)
	return RunEvent{Type: typ, Data: raw}
}

// --- typed payloads ---

// TextDeltaData carries a chunk of streamed assistant text.
type TextDeltaData struct {
	Content string `json:"content"`
}

// ThinkingDeltaData carries a chunk of streamed thinking/reasoning content.
type ThinkingDeltaData struct {
	Content string `json:"content"`
}

// ToolCallStartData is emitted when the LLM requests a tool execution.
type ToolCallStartData struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultData is emitted after a tool execution completes.
type ToolResultData struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Output     json.RawMessage `json:"output,omitempty"`
	DurationMs int64           `json:"durationMs"`
	Error      *string         `json:"error,omitempty"`
}

// TokenUsage tracks prompt and completion token counts for a single LLM call.
type TokenUsage struct {
	PromptTokens        int     `json:"promptTokens"`
	CompletionTokens    int     `json:"completionTokens"`
	TotalTokens         int     `json:"totalTokens"`
	CacheReadTokens     int     `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int     `json:"cacheCreationTokens,omitempty"`
	CostUSD             float64 `json:"costUsd,omitempty"`
	Model               string  `json:"model,omitempty"`
}

// ModelFallbackData is emitted when a fallback model is used.
type ModelFallbackData struct {
	FromModel string `json:"fromModel"`
	ToModel   string `json:"toModel"`
	Reason    string `json:"reason"`
}

// TurnCompleteData is emitted at the end of each agentic turn
// (one LLM call that may be followed by tool executions).
type TurnCompleteData struct {
	TurnIndex  int        `json:"turnIndex"`
	TokenUsage TokenUsage `json:"tokenUsage"`
	BudgetUsed  int        `json:"budgetUsed,omitempty"`
	BudgetLimit int        `json:"budgetLimit,omitempty"`
	// Model is the model that actually served this turn (may differ from
	// config if a fallback was used). Empty means the primary model was used.
	Model string `json:"model,omitempty"`
	// Source identifies the origin of this LLM call (main_loop, compact, subtask, etc.)
	// for differentiated analytics and retry policies.
	Source QuerySource `json:"source,omitempty"`
}

// RunCompleteData is the final event emitted when the agentic loop finishes.
// Token accounting follows Claude Code's cumulative vs incremental pattern:
// - LatestInputTokens: prompt tokens from the LAST LLM call (replaces, not accumulates)
// - CumulativeOutputTokens: sum of all output tokens across all turns
// - CumulativeCacheReadTokens/CacheCreationTokens: sum across all turns
type RunCompleteData struct {
	TotalTurns               int     `json:"totalTurns"`
	TotalTokens              int     `json:"totalTokens"`
	TotalCost                float64 `json:"totalCostUsd,omitempty"`
	LatestInputTokens        int     `json:"latestInputTokens,omitempty"`
	CumulativeOutputTokens   int     `json:"cumulativeOutputTokens,omitempty"`
	CumulativeCacheReadTokens    int `json:"cumulativeCacheReadTokens,omitempty"`
	CumulativeCacheCreationTokens int `json:"cumulativeCacheCreationTokens,omitempty"`
}

// ErrorData carries error information.
type ErrorData struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// CompactData is emitted when the context window is compressed.
type CompactData struct {
	OriginalMessages int `json:"originalMessages"`
	CompactedTo      int `json:"compactedTo"`
}

// ToolState represents the lifecycle of a tool execution.
type ToolState string

const (
	ToolStateQueued    ToolState = "queued"
	ToolStateExecuting ToolState = "executing"
	ToolStateCompleted ToolState = "completed"
	ToolStateStalled   ToolState = "stalled"
	ToolStateAborted   ToolState = "aborted"
)

// ToolProgressData is emitted when a tool's state changes.
type ToolProgressData struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	State ToolState `json:"state"`
}

// SubtaskStartData is emitted when a sub-agent is spawned.
type SubtaskStartData struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Depth       int    `json:"depth"`
}

// SubtaskCompleteData is emitted when a sub-agent finishes.
type SubtaskCompleteData struct {
	ID          string  `json:"id"`
	TotalTurns  int     `json:"totalTurns"`
	TotalTokens int     `json:"totalTokens"`
	TotalCost   float64 `json:"totalCostUsd,omitempty"`
	Summary     string  `json:"summary,omitempty"`
	Error       *string `json:"error,omitempty"`
}

// QuerySource identifies the origin of an LLM call for analytics and retry policies.
// Foreground sources (user-blocking) retry aggressively on capacity errors;
// background sources bail early to avoid amplifying cascades.
type QuerySource string

const (
	// SourceMainLoop is the primary agentic loop (user-blocking).
	SourceMainLoop QuerySource = "main_loop"
	// SourceCompact is context compaction via LLM summarization.
	SourceCompact QuerySource = "compact"
	// SourceSubtask is a sub-agent execution.
	SourceSubtask QuerySource = "subtask"
	// SourceMemoryEval is memory evaluation/storage.
	SourceMemoryEval QuerySource = "memory_eval"
	// SourceBudgetNudge is a budget continuation nudge turn.
	SourceBudgetNudge QuerySource = "budget_nudge"
)

// IsForegroundSource returns true if the source is user-blocking and should
// retry aggressively on transient/capacity errors. Background sources (compact,
// memory_eval) should fail fast to avoid gateway amplification during cascades.
func (s QuerySource) IsForegroundSource() bool {
	switch s {
	case SourceMainLoop, SourceSubtask, SourceBudgetNudge:
		return true
	default:
		return false
	}
}

// SubtaskProgressData is emitted periodically during long-running sub-agents
// to provide a brief status update. Inspired by Claude Code's agentSummary.ts.
type SubtaskProgressData struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// ToolDeniedData is emitted when a tool call is denied by permission rules.
type ToolDeniedData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Reason      string `json:"reason"`
	DenialCount int    `json:"denialCount"`
	Escalated   bool   `json:"escalated"`
}

// AgentMessageData is emitted when a sub-agent sends a message via the mailbox.
type AgentMessageData struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
}

// InputRequestData is emitted when the agentic loop needs structured user input.
// The Payload field carries a UiFormPayload-compatible JSON object that the
// Flutter client uses to render a dynamic form via UiSelectionPanel.
type InputRequestData struct {
	RequestID  string          `json:"requestId"`
	ServerName string          `json:"serverName,omitempty"`
	Payload    json.RawMessage `json:"payload"` // UiFormPayload JSON
}

// UiOptionDto mirrors the Flutter UiOptionDto — a single option in a radio/select field.
type UiOptionDto struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// UiElementDto mirrors the Flutter UiElementDto — a single form field descriptor.
type UiElementDto struct {
	Type     string        `json:"type"`               // "text" | "checkbox" | "radio" | "select"
	ID       string        `json:"id"`                 // field name / key
	Label    string        `json:"label"`              // display label
	Required bool          `json:"required,omitempty"` // whether the field is mandatory
	Options  []UiOptionDto `json:"options,omitempty"`  // for radio/select types
}

// UiFormPayloadDto mirrors the Flutter UiFormPayloadDto.
type UiFormPayloadDto struct {
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Elements    []UiElementDto `json:"elements"`
	SubmitLabel string         `json:"submitLabel,omitempty"`
}


// WarningData is emitted for non-fatal advisories during a run.
type WarningData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CanvasFormat identifies the format of content sent to the canvas panel.
type CanvasFormat string

const (
	// CanvasFormatMarkdown renders the content as rich markdown with syntax
	// highlighting for code blocks.
	CanvasFormatMarkdown CanvasFormat = "markdown"
	// CanvasFormatHTML renders sanitized HTML inside a sandboxed iframe.
	CanvasFormatHTML CanvasFormat = "html"
	// CanvasFormatTable renders structured tabular data as an interactive table.
	// The content field must be valid JSON: {"columns":[...],"rows":[[...]]}.
	CanvasFormatTable CanvasFormat = "table"
)

// FrontendActionCallData is the payload for EventFrontendActionCall.
// Carries everything the client needs to dispatch the call to its local
// CopilotAction registry.
type FrontendActionCallData struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// CanvasUpdateData is the payload for EventCanvasUpdate.
// Emitted when the agent renders rich visual content to the canvas panel.
type CanvasUpdateData struct {
	// ID uniquely identifies this canvas artifact. When the same ID is re-emitted,
	// the frontend replaces the previous content (live update).
	ID      string       `json:"id"`
	Title   string       `json:"title,omitempty"`
	Format  CanvasFormat `json:"format"`
	Content string       `json:"content"`
	// Exportable indicates that the content can be downloaded (CSV, HTML, etc.).
	Exportable bool `json:"exportable,omitempty"`
}
