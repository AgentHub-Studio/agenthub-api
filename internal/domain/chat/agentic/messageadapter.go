package agentic

import (
	"encoding/json"
	"fmt"
	"sync"
)

// SDK message format translation.
//
// Inspired by Claude Code's sdkMessageAdapter.ts — converts external SDK
// message formats into internal normalized types using a discriminated union
// pattern. Graceful degradation: unknown types are logged and ignored.

// AdaptedMessageType classifies the result of a message adaptation.
type AdaptedMessageType string

const (
	// AdaptedMessage indicates a fully converted message.
	AdaptedMessage AdaptedMessageType = "message"
	// AdaptedStreamEvent indicates a streaming event.
	AdaptedStreamEvent AdaptedMessageType = "stream_event"
	// AdaptedIgnored indicates the message was unrecognized and skipped.
	AdaptedIgnored AdaptedMessageType = "ignored"
)

// AdaptedResult holds the outcome of adapting an external message.
type AdaptedResult struct {
	Type    AdaptedMessageType `json:"type"`
	Message *NormalizedMessage `json:"message,omitempty"`
	Event   *StreamEventData   `json:"event,omitempty"`
	Reason  string             `json:"reason,omitempty"` // why ignored
}

// NormalizedMessage is the internal canonical message format.
type NormalizedMessage struct {
	ID          string          `json:"id"`
	Role        string          `json:"role"` // user, assistant, system, tool
	Content     string          `json:"content,omitempty"`
	ToolCalls   json.RawMessage `json:"toolCalls,omitempty"`
	ToolCallID  string          `json:"toolCallId,omitempty"`
	Model       string          `json:"model,omitempty"`
	StopReason  string          `json:"stopReason,omitempty"`
	TokenUsage  *TokenUsageInfo `json:"tokenUsage,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

// TokenUsageInfo holds token usage from an LLM response.
type TokenUsageInfo struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	CacheReadTokens   int `json:"cacheReadTokens,omitempty"`
	CacheCreateTokens int `json:"cacheCreateTokens,omitempty"`
}

// StreamEventData holds data for a streaming event.
type StreamEventData struct {
	EventType string          `json:"eventType"`
	Data      json.RawMessage `json:"data"`
	Index     int             `json:"index,omitempty"`
}

// AdaptOptions controls which message types are converted.
type AdaptOptions struct {
	ConvertToolResults    bool
	ConvertUserMessages   bool
	ConvertSystemMessages bool
}

// DefaultAdaptOptions returns options that convert all message types.
func DefaultAdaptOptions() AdaptOptions {
	return AdaptOptions{
		ConvertToolResults:    true,
		ConvertUserMessages:   true,
		ConvertSystemMessages: true,
	}
}

// MessageAdapterFunc converts an external message into an AdaptedResult.
type MessageAdapterFunc func(raw json.RawMessage, opts AdaptOptions) AdaptedResult

// MessageAdapter routes external messages to type-specific converters.
type MessageAdapter struct {
	mu        sync.RWMutex
	converters map[string]MessageAdapterFunc // keyed by message type
	ignored   int
}

// NewMessageAdapter creates a message adapter.
func NewMessageAdapter() *MessageAdapter {
	return &MessageAdapter{
		converters: make(map[string]MessageAdapterFunc),
	}
}

// RegisterConverter adds a converter for a specific external message type.
func (a *MessageAdapter) RegisterConverter(msgType string, fn MessageAdapterFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.converters[msgType] = fn
}

// Convert adapts an external message. The raw message must have a "type" field.
func (a *MessageAdapter) Convert(raw json.RawMessage, opts AdaptOptions) AdaptedResult {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		a.mu.Lock()
		a.ignored++
		a.mu.Unlock()
		return AdaptedResult{
			Type:   AdaptedIgnored,
			Reason: fmt.Sprintf("unmarshal error: %v", err),
		}
	}

	a.mu.RLock()
	fn, ok := a.converters[envelope.Type]
	a.mu.RUnlock()

	if !ok {
		a.mu.Lock()
		a.ignored++
		a.mu.Unlock()
		return AdaptedResult{
			Type:   AdaptedIgnored,
			Reason: fmt.Sprintf("unknown message type: %s", envelope.Type),
		}
	}

	return fn(raw, opts)
}

// ConvertBatch adapts multiple messages, filtering out ignored ones.
func (a *MessageAdapter) ConvertBatch(messages []json.RawMessage, opts AdaptOptions) []AdaptedResult {
	results := make([]AdaptedResult, 0, len(messages))
	for _, msg := range messages {
		result := a.Convert(msg, opts)
		if result.Type != AdaptedIgnored {
			results = append(results, result)
		}
	}
	return results
}

// IgnoredCount returns the total number of ignored messages.
func (a *MessageAdapter) IgnoredCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ignored
}

// RegisteredTypes returns all registered converter type names.
func (a *MessageAdapter) RegisteredTypes() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	types := make([]string, 0, len(a.converters))
	for t := range a.converters {
		types = append(types, t)
	}
	return types
}
