package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// sideQueryMockModel implements ai.ChatModel for testing.
type sideQueryMockModel struct {
	chatResp *ai.ChatResponse
	chatErr  error
	// capturedOpts captures the last ChatOptions passed.
	capturedOpts *ai.ChatOptions
}

func (m *sideQueryMockModel) Chat(_ context.Context, _ []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	m.capturedOpts = &opts
	return m.chatResp, m.chatErr
}

func (m *sideQueryMockModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (m *sideQueryMockModel) GetProviderName() string { return "mock" }

// --- SideQueryConfig ---

func TestDefaultSideQueryConfig(t *testing.T) {
	cfg := agentic.DefaultSideQueryConfig()
	assert.NotEmpty(t, cfg.DefaultModel)
	assert.Equal(t, 1024, cfg.DefaultMaxTokens)
	assert.Equal(t, float64(0), cfg.DefaultTemperature)
}

// --- NewSideQuery ---

func TestNewSideQuery_DefaultsMaxTokens(t *testing.T) {
	sq := agentic.NewSideQuery(nil, agentic.SideQueryConfig{DefaultMaxTokens: 0})
	assert.NotNil(t, sq) // should not panic
}

// --- Query ---

func TestSideQuery_Query_NilModel(t *testing.T) {
	sq := agentic.NewSideQuery(nil, agentic.DefaultSideQueryConfig())
	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model is nil")
}

func TestSideQuery_Query_Success(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{
			Content:      "The answer is 42.",
			FinishReason: "stop",
			Usage:        ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			Model:        "claude-haiku-4-5-20251001",
		},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	result, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "What is 6*7?"}},
		SystemPrompt: "You are a calculator.",
	})

	require.NoError(t, err)
	assert.Equal(t, "The answer is 42.", result.Content)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Equal(t, 15, result.Usage.TotalTokens)
	// Should have passed system prompt through.
	assert.Equal(t, "You are a calculator.", model.capturedOpts.SystemMsg)
}

func TestSideQuery_Query_ModelOverride(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{Content: "ok", FinishReason: "stop"},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "test"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-20250514", model.capturedOpts.Model)
}

func TestSideQuery_Query_UsesDefaultModel(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{Content: "ok", FinishReason: "stop"},
	}
	sq := agentic.NewSideQuery(model, agentic.SideQueryConfig{
		DefaultModel:    "my-default-model",
		DefaultMaxTokens: 512,
	})

	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "test"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "my-default-model", model.capturedOpts.Model)
	assert.Equal(t, 512, model.capturedOpts.MaxTokens)
}

func TestSideQuery_Query_DisableThinking(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{Content: "fast", FinishReason: "stop"},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages:        []ai.Message{{Role: ai.RoleUser, Content: "test"}},
		DisableThinking: true,
	})

	require.NoError(t, err)
	require.NotNil(t, model.capturedOpts.Thinking)
	assert.Equal(t, ai.ThinkingDisabled, model.capturedOpts.Thinking.Type)
}

func TestSideQuery_Query_StopSequences(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{Content: "<result>42</result>", FinishReason: "stop"},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages:      []ai.Message{{Role: ai.RoleUser, Content: "test"}},
		StopSequences: []string{"</result>"},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"</result>"}, model.capturedOpts.StopSequences)
}

func TestSideQuery_Query_CustomTemperature(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{Content: "creative", FinishReason: "stop"},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	temp := 0.9
	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: "test"}},
		Temperature: &temp,
	})

	require.NoError(t, err)
	assert.Equal(t, 0.9, model.capturedOpts.Temperature)
}

func TestSideQuery_Query_ModelError(t *testing.T) {
	model := &sideQueryMockModel{
		chatErr: errors.New("rate limited"),
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	_, err := sq.Query(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "test"}},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

// --- QueryForText ---

func TestSideQuery_QueryForText(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{Content: "summary text", FinishReason: "stop"},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	text, err := sq.QueryForText(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "summarize"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "summary text", text)
}

// --- QueryForToolCall ---

func TestSideQuery_QueryForToolCall_Success(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{
			FinishReason: "tool_calls",
			ToolCalls: []ai.ToolCall{
				{
					ID:   "tc_1",
					Type: "function",
					Function: ai.ToolFunction{
						Name:      "classify",
						Arguments: `{"category": "safe", "confidence": 0.95}`,
					},
				},
			},
		},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	result, err := sq.QueryForToolCall(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "test input"}},
		Tools: []ai.Tool{{
			Type: "function",
			Function: ai.ToolSchema{
				Name: "classify",
			},
		}},
	}, "classify")

	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "safe", parsed["category"])
	assert.InDelta(t, 0.95, parsed["confidence"], 0.001)

	// Should have set forced tool choice.
	require.NotNil(t, model.capturedOpts.ToolChoice)
	assert.Equal(t, ai.ToolChoiceTool, model.capturedOpts.ToolChoice.Type)
	assert.Equal(t, "classify", model.capturedOpts.ToolChoice.Name)
}

func TestSideQuery_QueryForToolCall_NoToolCalls(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{
			Content:      "I can't use tools",
			FinishReason: "stop",
		},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	_, err := sq.QueryForToolCall(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "test"}},
	}, "classify")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected tool call")
}

func TestSideQuery_QueryForToolCall_WrongToolName(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{
			FinishReason: "tool_calls",
			ToolCalls: []ai.ToolCall{
				{
					ID:   "tc_1",
					Type: "function",
					Function: ai.ToolFunction{
						Name:      "other_tool",
						Arguments: `{"value": 1}`,
					},
				},
			},
		},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	// Should fall back to first tool call.
	result, err := sq.QueryForToolCall(context.Background(), agentic.SideQueryOptions{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "test"}},
	}, "classify")

	require.NoError(t, err)
	assert.Equal(t, `{"value": 1}`, string(result))
}

// --- ClassifyWithXML ---

func TestSideQuery_ClassifyWithXML(t *testing.T) {
	model := &sideQueryMockModel{
		chatResp: &ai.ChatResponse{
			Content:      "<block>safe</block>",
			FinishReason: "stop",
		},
	}
	sq := agentic.NewSideQuery(model, agentic.DefaultSideQueryConfig())

	result, err := sq.ClassifyWithXML(context.Background(),
		"Classify the input as safe or unsafe.",
		"This is a test input.",
		"</block>",
	)

	require.NoError(t, err)
	assert.Equal(t, "<block>safe</block>", result)

	// Should have set stop sequences and disabled thinking.
	assert.Equal(t, []string{"</block>"}, model.capturedOpts.StopSequences)
	require.NotNil(t, model.capturedOpts.Thinking)
	assert.Equal(t, ai.ThinkingDisabled, model.capturedOpts.Thinking.Type)
	assert.Equal(t, 512, model.capturedOpts.MaxTokens)
}
