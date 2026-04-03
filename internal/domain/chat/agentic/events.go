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
	EventAgentMessage     RunEventType = "agent_message"
)

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
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	CostUSD          float64 `json:"costUsd,omitempty"`
}

// TurnCompleteData is emitted at the end of each agentic turn
// (one LLM call that may be followed by tool executions).
type TurnCompleteData struct {
	TurnIndex  int        `json:"turnIndex"`
	TokenUsage TokenUsage `json:"tokenUsage"`
}

// RunCompleteData is the final event emitted when the agentic loop finishes.
type RunCompleteData struct {
	TotalTurns  int     `json:"totalTurns"`
	TotalTokens int     `json:"totalTokens"`
	TotalCost   float64 `json:"totalCostUsd,omitempty"`
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
	Error       *string `json:"error,omitempty"`
}

// AgentMessageData is emitted when a sub-agent sends a message via the mailbox.
type AgentMessageData struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
}
