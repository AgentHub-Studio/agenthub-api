package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- AdaptedMessageType values ---

func TestAdaptedMessageType_Values(t *testing.T) {
	assert.Equal(t, agentic.AdaptedMessageType("message"), agentic.AdaptedMessage)
	assert.Equal(t, agentic.AdaptedMessageType("stream_event"), agentic.AdaptedStreamEvent)
	assert.Equal(t, agentic.AdaptedMessageType("ignored"), agentic.AdaptedIgnored)
}

// --- NewMessageAdapter ---

func TestNewMessageAdapter(t *testing.T) {
	a := agentic.NewMessageAdapter()
	assert.NotNil(t, a)
	assert.Equal(t, 0, a.IgnoredCount())
}

// --- RegisterConverter ---

func TestMessageAdapter_RegisterConverter(t *testing.T) {
	a := agentic.NewMessageAdapter()
	a.RegisterConverter("assistant", func(raw json.RawMessage, opts agentic.AdaptOptions) agentic.AdaptedResult {
		return agentic.AdaptedResult{Type: agentic.AdaptedMessage}
	})

	types := a.RegisteredTypes()
	assert.Contains(t, types, "assistant")
}

// --- Convert ---

func TestMessageAdapter_Convert_Known(t *testing.T) {
	a := agentic.NewMessageAdapter()
	a.RegisterConverter("assistant", func(raw json.RawMessage, opts agentic.AdaptOptions) agentic.AdaptedResult {
		return agentic.AdaptedResult{
			Type: agentic.AdaptedMessage,
			Message: &agentic.NormalizedMessage{
				ID:   "msg-1",
				Role: "assistant",
				Content: "Hello",
			},
		}
	})

	raw := json.RawMessage(`{"type":"assistant","text":"Hello"}`)
	result := a.Convert(raw, agentic.DefaultAdaptOptions())

	assert.Equal(t, agentic.AdaptedMessage, result.Type)
	require.NotNil(t, result.Message)
	assert.Equal(t, "assistant", result.Message.Role)
	assert.Equal(t, "Hello", result.Message.Content)
}

func TestMessageAdapter_Convert_Unknown(t *testing.T) {
	a := agentic.NewMessageAdapter()

	raw := json.RawMessage(`{"type":"unknown_type"}`)
	result := a.Convert(raw, agentic.DefaultAdaptOptions())

	assert.Equal(t, agentic.AdaptedIgnored, result.Type)
	assert.Contains(t, result.Reason, "unknown message type")
	assert.Equal(t, 1, a.IgnoredCount())
}

func TestMessageAdapter_Convert_InvalidJSON(t *testing.T) {
	a := agentic.NewMessageAdapter()

	raw := json.RawMessage(`{invalid`)
	result := a.Convert(raw, agentic.DefaultAdaptOptions())

	assert.Equal(t, agentic.AdaptedIgnored, result.Type)
	assert.Contains(t, result.Reason, "unmarshal error")
}

func TestMessageAdapter_Convert_StreamEvent(t *testing.T) {
	a := agentic.NewMessageAdapter()
	a.RegisterConverter("stream_event", func(raw json.RawMessage, opts agentic.AdaptOptions) agentic.AdaptedResult {
		return agentic.AdaptedResult{
			Type: agentic.AdaptedStreamEvent,
			Event: &agentic.StreamEventData{
				EventType: "text_delta",
				Data:      json.RawMessage(`{"text":"chunk"}`),
			},
		}
	})

	raw := json.RawMessage(`{"type":"stream_event"}`)
	result := a.Convert(raw, agentic.DefaultAdaptOptions())

	assert.Equal(t, agentic.AdaptedStreamEvent, result.Type)
	require.NotNil(t, result.Event)
	assert.Equal(t, "text_delta", result.Event.EventType)
}

// --- ConvertBatch ---

func TestMessageAdapter_ConvertBatch(t *testing.T) {
	a := agentic.NewMessageAdapter()
	a.RegisterConverter("msg", func(raw json.RawMessage, opts agentic.AdaptOptions) agentic.AdaptedResult {
		return agentic.AdaptedResult{Type: agentic.AdaptedMessage, Message: &agentic.NormalizedMessage{Role: "user"}}
	})

	msgs := []json.RawMessage{
		json.RawMessage(`{"type":"msg"}`),
		json.RawMessage(`{"type":"unknown"}`), // ignored
		json.RawMessage(`{"type":"msg"}`),
	}

	results := a.ConvertBatch(msgs, agentic.DefaultAdaptOptions())
	assert.Len(t, results, 2, "ignored messages should be filtered")
}

func TestMessageAdapter_ConvertBatch_Empty(t *testing.T) {
	a := agentic.NewMessageAdapter()
	results := a.ConvertBatch(nil, agentic.DefaultAdaptOptions())
	assert.Empty(t, results)
}

// --- Options control ---

func TestMessageAdapter_Options(t *testing.T) {
	a := agentic.NewMessageAdapter()
	a.RegisterConverter("tool_result", func(raw json.RawMessage, opts agentic.AdaptOptions) agentic.AdaptedResult {
		if !opts.ConvertToolResults {
			return agentic.AdaptedResult{Type: agentic.AdaptedIgnored, Reason: "tool results disabled"}
		}
		return agentic.AdaptedResult{Type: agentic.AdaptedMessage, Message: &agentic.NormalizedMessage{Role: "tool"}}
	})

	raw := json.RawMessage(`{"type":"tool_result"}`)

	// With tool results enabled
	result := a.Convert(raw, agentic.DefaultAdaptOptions())
	assert.Equal(t, agentic.AdaptedMessage, result.Type)

	// With tool results disabled
	opts := agentic.AdaptOptions{ConvertToolResults: false}
	result = a.Convert(raw, opts)
	assert.Equal(t, agentic.AdaptedIgnored, result.Type)
}

// --- TokenUsageInfo ---

func TestTokenUsageInfo(t *testing.T) {
	usage := &agentic.TokenUsageInfo{
		InputTokens:       100,
		OutputTokens:      50,
		CacheReadTokens:   80,
		CacheCreateTokens: 20,
	}
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
}

// --- DefaultAdaptOptions ---

func TestDefaultAdaptOptions(t *testing.T) {
	opts := agentic.DefaultAdaptOptions()
	assert.True(t, opts.ConvertToolResults)
	assert.True(t, opts.ConvertUserMessages)
	assert.True(t, opts.ConvertSystemMessages)
}

// --- IgnoredCount accumulates ---

func TestMessageAdapter_IgnoredCount(t *testing.T) {
	a := agentic.NewMessageAdapter()

	a.Convert(json.RawMessage(`{"type":"x"}`), agentic.DefaultAdaptOptions())
	a.Convert(json.RawMessage(`{"type":"y"}`), agentic.DefaultAdaptOptions())
	a.Convert(json.RawMessage(`{bad`), agentic.DefaultAdaptOptions())

	assert.Equal(t, 3, a.IgnoredCount())
}
