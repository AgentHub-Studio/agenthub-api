package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestNewRunEvent_TextDelta(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventTextDelta, agentic.TextDeltaData{Content: "Hello"})
	assert.Equal(t, agentic.EventTextDelta, evt.Type)

	var data agentic.TextDeltaData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, "Hello", data.Content)
}

func TestNewRunEvent_ToolCallStart(t *testing.T) {
	input := json.RawMessage(`{"query":"test"}`)
	evt := agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc_1", Name: "document_search", Input: input,
	})
	assert.Equal(t, agentic.EventToolCallStart, evt.Type)

	var data agentic.ToolCallStartData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, "tc_1", data.ID)
	assert.Equal(t, "document_search", data.Name)
	assert.JSONEq(t, `{"query":"test"}`, string(data.Input))
}

func TestNewRunEvent_ToolResult(t *testing.T) {
	output := json.RawMessage(`{"results":[]}`)
	evt := agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{
		ID: "tc_1", Name: "document_search", Output: output, DurationMs: 450,
	})

	var data agentic.ToolResultData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, int64(450), data.DurationMs)
	assert.Nil(t, data.Error)
}

func TestNewRunEvent_ToolResultWithError(t *testing.T) {
	errMsg := "timeout"
	evt := agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{
		ID: "tc_2", Name: "execute-sql", DurationMs: 30000, Error: &errMsg,
	})

	var data agentic.ToolResultData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	require.NotNil(t, data.Error)
	assert.Equal(t, "timeout", *data.Error)
}

func TestNewRunEvent_TurnComplete(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventTurnComplete, agentic.TurnCompleteData{
		TurnIndex: 2,
		TokenUsage: agentic.TokenUsage{
			PromptTokens: 1500, CompletionTokens: 300, TotalTokens: 1800,
		},
	})

	var data agentic.TurnCompleteData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, 2, data.TurnIndex)
	assert.Equal(t, 1800, data.TokenUsage.TotalTokens)
}

func TestNewRunEvent_RunComplete(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns: 3, TotalTokens: 5200,
	})

	var data agentic.RunCompleteData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, 3, data.TotalTurns)
	assert.Equal(t, 5200, data.TotalTokens)
}

func TestNewRunEvent_RunComplete_CumulativeTokenAccounting(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns:                    5,
		TotalTokens:                   12000,
		TotalCost:                     0.045,
		LatestInputTokens:             3500,
		CumulativeOutputTokens:        8500,
		CumulativeCacheReadTokens:     6000,
		CumulativeCacheCreationTokens: 1200,
	})

	var data agentic.RunCompleteData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, 5, data.TotalTurns)
	assert.Equal(t, 12000, data.TotalTokens)
	assert.Equal(t, 3500, data.LatestInputTokens, "should be latest (not cumulative) input tokens")
	assert.Equal(t, 8500, data.CumulativeOutputTokens)
	assert.Equal(t, 6000, data.CumulativeCacheReadTokens)
	assert.Equal(t, 1200, data.CumulativeCacheCreationTokens)
}

func TestRunCompleteData_OmitsZeroCumulativeFields(t *testing.T) {
	// When cumulative fields are zero, they should be omitted from JSON.
	data := agentic.RunCompleteData{TotalTurns: 1, TotalTokens: 100}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "latestInputTokens")
	assert.NotContains(t, string(raw), "cumulativeOutputTokens")
	assert.NotContains(t, string(raw), "cumulativeCacheReadTokens")
}

func TestNewRunEvent_Error(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventError, agentic.ErrorData{
		Message: "rate limited", Code: "rate_limit",
	})

	var data agentic.ErrorData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, "rate limited", data.Message)
	assert.Equal(t, "rate_limit", data.Code)
}

func TestNewRunEvent_ContextCompacted(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventContextCompacted, agentic.CompactData{
		OriginalMessages: 50, CompactedTo: 8,
	})

	var data agentic.CompactData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, 50, data.OriginalMessages)
	assert.Equal(t, 8, data.CompactedTo)
}

func TestRunEvent_JSONRoundtrip(t *testing.T) {
	original := agentic.NewRunEvent(agentic.EventTextDelta, agentic.TextDeltaData{Content: "test"})

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded agentic.RunEvent
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Type, decoded.Type)
	assert.JSONEq(t, string(original.Data), string(decoded.Data))
}
