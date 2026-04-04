package openairesponses

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestDefaultsEmptyFunctionCallArgumentsToEmptyJSONObject(t *testing.T) {
	p := New("test-key", "")

	req := p.buildRequest([]ai.Message{
		{
			Role: ai.RoleAssistant,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: ai.ToolFunction{
						Name:      "search_documents",
						Arguments: "",
					},
				},
			},
		},
	}, ai.ChatOptions{Model: "gpt-5.4"}, false)

	items, ok := req.Input.([]inputItem)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "function_call", items[0].Type)
	assert.Empty(t, items[0].ID)
	assert.Equal(t, "call_123", items[0].CallID)
	assert.Equal(t, "{}", items[0].Args)
}

func TestBuildRequestPreservesNonEmptyFunctionCallArguments(t *testing.T) {
	p := New("test-key", "")

	req := p.buildRequest([]ai.Message{
		{
			Role: ai.RoleAssistant,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: ai.ToolFunction{
						Name:      "search_documents",
						Arguments: `{"query":"hello"}`,
					},
				},
			},
		},
	}, ai.ChatOptions{Model: "gpt-5.4"}, false)

	items, ok := req.Input.([]inputItem)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].ID)
	assert.Equal(t, "call_456", items[0].CallID)
	assert.Equal(t, `{"query":"hello"}`, items[0].Args)
}
