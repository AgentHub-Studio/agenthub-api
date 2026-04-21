package ai

import "context"

// Role constants for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

// Message represents a single chat message.
type Message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	ToolCalls  []ToolCall     `json:"toolCalls,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ToolCall represents an LLM tool call request.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction holds the name and arguments of a tool call.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Tool defines a tool available to the LLM.
type Tool struct {
	Type     string     `json:"type"` // "function"
	Function ToolSchema `json:"function"`
}

// ToolSchema describes a function tool.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// ThinkingType identifies the thinking mode.
type ThinkingType string

const (
	// ThinkingAdaptive lets the model decide when and how much to think.
	// Supported by Claude Opus 4.6+ and Sonnet 4.6+.
	ThinkingAdaptive ThinkingType = "adaptive"
	// ThinkingEnabled activates extended thinking with a fixed budget.
	ThinkingEnabled ThinkingType = "enabled"
	// ThinkingDisabled turns off extended thinking.
	ThinkingDisabled ThinkingType = "disabled"
)

// ThinkingConfig controls extended thinking / chain-of-thought behaviour.
// Inspired by Claude Code's ThinkingConfig in utils/thinking.ts.
type ThinkingConfig struct {
	// Type selects the thinking mode (adaptive, enabled, disabled).
	Type ThinkingType `json:"type"`
	// BudgetTokens is the token budget for thinking (only used when Type == ThinkingEnabled).
	// Must be less than MaxTokens.
	BudgetTokens int `json:"budgetTokens,omitempty"`
}

// EffortLevel controls how much reasoning effort the model applies.
// Maps to the output_config.effort API parameter (beta: output-config-2025-04-08).
// Inspired by Claude Code's effort.ts.
type EffortLevel string

const (
	// EffortLow is quick, straightforward processing with minimal overhead.
	EffortLow EffortLevel = "low"
	// EffortMedium is a balanced approach (recommended default for Opus 4.6).
	EffortMedium EffortLevel = "medium"
	// EffortHigh is comprehensive processing (API default when no effort is sent).
	EffortHigh EffortLevel = "high"
	// EffortMax is maximum reasoning capability (Opus 4.6 only).
	EffortMax EffortLevel = "max"
)

// ChatOptions holds configuration for a chat request.
type ChatOptions struct {
	Model       string  `json:"model"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"topP,omitempty"`
	Tools       []Tool  `json:"tools,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
	SystemMsg   string  `json:"systemMessage,omitempty"`
	// Thinking configures extended thinking / chain-of-thought. Nil means disabled.
	Thinking *ThinkingConfig `json:"thinking,omitempty"`
	// CacheControl enables prompt caching for the system message.
	// When true, the provider should add cache_control markers to maximise
	// cache hits across turns (e.g. Anthropic's ephemeral cache).
	// Inspired by Claude Code's prompt caching strategy.
	CacheControl bool `json:"cacheControl,omitempty"`
	// Effort controls reasoning effort level. Nil means no effort parameter is sent
	// (API defaults to "high"). Inspired by Claude Code's effort.ts.
	Effort *EffortLevel `json:"effort,omitempty"`
	// ToolChoice controls how the model selects tools. Nil or empty means "auto"
	// (model decides). Use ToolChoiceNone to prevent tool use, or ToolChoiceForced
	// to force calling a specific tool for structured output.
	// Inspired by Claude Code's sideQuery.ts forced tool use pattern.
	ToolChoice *ToolChoice `json:"toolChoice,omitempty"`
	// StopSequences causes the model to stop generating when any of these strings
	// is emitted. Useful for structured query flows that need early termination.
	// Inspired by Claude Code's sideQuery.ts stop_sequences parameter.
	StopSequences []string `json:"stopSequences,omitempty"`
	// PreviousResponseID enables response chaining for providers that support
	// server-side conversation history (e.g. OpenAI Responses API). When set,
	// the provider appends only new messages to the stored context rather than
	// resending the full conversation. The response ID returned by the previous
	// call is passed as PreviousResponseID on the next call.
	PreviousResponseID string `json:"previousResponseId,omitempty"`
	// ProviderOptions carries backend-specific parameters that don't fit the
	// generic ChatOptions schema. Providers that recognise a key pass it through
	// to the underlying API; others ignore it. Ollama uses num_ctx, num_predict,
	// seed, top_k, repeat_penalty, mirostat, etc. — see the Ollama options table.
	ProviderOptions map[string]any `json:"providerOptions,omitempty"`
}

// ToolChoiceType identifies how the model should use tools.
type ToolChoiceType string

const (
	// ToolChoiceAuto lets the model decide whether and which tools to use (default).
	ToolChoiceAuto ToolChoiceType = "auto"
	// ToolChoiceNone prevents the model from using any tools.
	ToolChoiceNone ToolChoiceType = "none"
	// ToolChoiceAny forces the model to use at least one tool (model picks which).
	ToolChoiceAny ToolChoiceType = "any"
	// ToolChoiceTool forces the model to call a specific named tool.
	ToolChoiceTool ToolChoiceType = "tool"
)

// ToolChoice specifies tool selection behaviour.
type ToolChoice struct {
	// Type is the selection mode: auto, none, any, or tool.
	Type ToolChoiceType `json:"type"`
	// Name is the tool to force (only used when Type == ToolChoiceTool).
	Name string `json:"name,omitempty"`
}

// ForcedToolChoice creates a ToolChoice that forces calling a specific tool.
// Useful for getting structured JSON output from classifiers/validators.
func ForcedToolChoice(toolName string) *ToolChoice {
	return &ToolChoice{Type: ToolChoiceTool, Name: toolName}
}

// ChatResponse holds the response from a chat request.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
	FinishReason string     `json:"finishReason"` // stop, tool_calls, length
	Usage        Usage      `json:"usage"`
	Model        string     `json:"model"`
	// ThinkingContent holds the model's chain-of-thought reasoning (if thinking was enabled).
	ThinkingContent string `json:"thinkingContent,omitempty"`
	// ResponseID is the server-assigned identifier for this response. Providers
	// that support response chaining (e.g. OpenAI Responses API) return this ID;
	// callers pass it as PreviousResponseID on the next request.
	ResponseID string `json:"responseId,omitempty"`
}

// Usage holds token usage statistics.
type Usage struct {
	PromptTokens        int `json:"promptTokens"`
	CompletionTokens    int `json:"completionTokens"`
	TotalTokens         int `json:"totalTokens"`
	CacheReadTokens     int `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int `json:"cacheCreationTokens,omitempty"`
}

// Merge returns a copy of o with non-zero fields from other overwriting the originals.
// Useful for combining per-request overrides with provider defaults.
func (o ChatOptions) Merge(other ChatOptions) ChatOptions {
	if other.Model != "" {
		o.Model = other.Model
	}
	if other.MaxTokens != 0 {
		o.MaxTokens = other.MaxTokens
	}
	if other.Temperature != 0 {
		o.Temperature = other.Temperature
	}
	if other.TopP != 0 {
		o.TopP = other.TopP
	}
	if len(other.Tools) > 0 {
		o.Tools = other.Tools
	}
	if other.SystemMsg != "" {
		o.SystemMsg = other.SystemMsg
	}
	if other.Stream {
		o.Stream = other.Stream
	}
	if other.Thinking != nil {
		o.Thinking = other.Thinking
	}
	if other.Effort != nil {
		o.Effort = other.Effort
	}
	if other.ToolChoice != nil {
		o.ToolChoice = other.ToolChoice
	}
	if len(other.StopSequences) > 0 {
		o.StopSequences = other.StopSequences
	}
	return o
}

// ChatModel is the core interface for LLM providers.
type ChatModel interface {
	// Chat sends a chat request and returns a complete response.
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error)

	// ChatStream sends a chat request and streams tokens via the channel.
	ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan StreamChunk, error)

	// GetProviderName returns the provider identifier (e.g., "openai", "anthropic").
	GetProviderName() string
}

// StreamChunk represents a single streamed token chunk.
type StreamChunk struct {
	Delta         string    `json:"delta"`
	ToolCallDelta *ToolCall `json:"toolCallDelta,omitempty"`
	FinishReason  string    `json:"finishReason,omitempty"`
	Usage         *Usage    `json:"usage,omitempty"`
	Error         error     `json:"-"`
	// ThinkingDelta holds incremental thinking content (when extended thinking is enabled).
	ThinkingDelta string `json:"thinkingDelta,omitempty"`
	// ResponseID, when set, is the server-assigned ID for the response being streamed.
	// Used for response chaining (PreviousResponseID on the next call).
	ResponseID string `json:"responseId,omitempty"`
}
