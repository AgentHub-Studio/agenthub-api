package agentic_test

import (
	"bytes"
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

func TestNewRunEvent_ToolCallStartRedactsSensitiveInput(t *testing.T) {
	const bearerSecret = "Bearer abcdefghijklmnopqrstuvwx.yyyyyyyyyyyyyyyyyy.zzzzzzzzzzzzzzzzzz"
	const cookieSecret = "session=very-sensitive-session-value"
	const apiKeySecret = "sk-ant-abcdefghijklmnopqrstuvwxyz123456"
	input := json.RawMessage(`{
		"query":"safe query",
		"headers":{
			"Authorization":"` + bearerSecret + `",
			"Cookie":"` + cookieSecret + `"
		},
		"api_key":"` + apiKeySecret + `"
	}`)

	event := agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{
		ID: "tc_1", Name: "http_request", Input: input,
	})

	assert.NotContains(t, string(event.Data), bearerSecret)
	assert.NotContains(t, string(event.Data), cookieSecret)
	assert.NotContains(t, string(event.Data), apiKeySecret)

	var data agentic.ToolCallStartData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	assert.Contains(t, string(data.Input), "safe query")
	assert.NotContains(t, string(data.Input), "Authorization")
	assert.NotContains(t, string(data.Input), "Cookie")
	assert.NotContains(t, string(data.Input), "api_key")
}

func TestNewRunEvent_ToolResultRedactsSensitiveError(t *testing.T) {
	const secret = "tool-error-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	errMsg := "Authorization: Bearer " + secret + "\nupstream=" + internalURL

	event := agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{
		ID: "tc_1", Name: "http_request", Error: &errMsg,
	})

	var data agentic.ToolResultData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	require.NotNil(t, data.Error)
	assert.NotContains(t, *data.Error, secret)
	assert.NotContains(t, *data.Error, internalURL)
	assert.Contains(t, *data.Error, "[REDACTED]")
	assert.Contains(t, *data.Error, "<upstream>")
}

func TestNewRunEvent_SubtaskSummariesRedactSecrets(t *testing.T) {
	const secret = "subtask-summary-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	summary := "Authorization: Bearer " + secret + "\nupstream=" + internalURL

	tests := []struct {
		name  string
		event agentic.RunEvent
		field string
	}{
		{
			name:  "progress",
			event: agentic.NewRunEvent(agentic.EventSubtaskProgress, agentic.SubtaskProgressData{Summary: summary}),
			field: "summary",
		},
		{
			name: "completion",
			event: agentic.NewRunEvent(agentic.EventSubtaskComplete, agentic.SubtaskCompleteData{
				Summary: summary,
			}),
			field: "summary",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(tc.event.Data, &data))
			var got string
			require.NoError(t, json.Unmarshal(data[tc.field], &got))
			assert.NotContains(t, got, secret)
			assert.NotContains(t, got, internalURL)
			assert.Contains(t, got, "[REDACTED]")
			assert.Contains(t, got, "<upstream>")
		})
	}
}

func TestNewRunEvent_AgentMessageRedactsSecrets(t *testing.T) {
	const secret = "agent-message-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	content := "Authorization: Bearer " + secret + "\nupstream=" + internalURL

	event := agentic.NewRunEvent(agentic.EventAgentMessage, agentic.AgentMessageData{
		ID: "message-1", From: "subtask-a", To: "subtask-b", Content: content,
	})

	var data agentic.AgentMessageData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	assert.NotContains(t, data.Content, secret)
	assert.NotContains(t, data.Content, internalURL)
	assert.Contains(t, data.Content, "[REDACTED]")
	assert.Contains(t, data.Content, "<upstream>")
}

func TestNewRunEvent_SubtaskStartRedactsSecrets(t *testing.T) {
	const secret = "subtask-start-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	description := "Authorization: Bearer " + secret + "\nupstream=" + internalURL
	payload := agentic.SubtaskStartData{ID: "subtask-1", Description: description, Depth: 1}

	event := agentic.NewRunEvent(agentic.EventSubtaskStart, payload)

	var data agentic.SubtaskStartData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	assert.NotContains(t, data.Description, secret)
	assert.NotContains(t, data.Description, internalURL)
	assert.Contains(t, data.Description, "[REDACTED]")
	assert.Contains(t, data.Description, "<upstream>")
	assert.Equal(t, description, payload.Description)
}

func TestNewRunEvent_RunProgressRedactsActivitySummaries(t *testing.T) {
	const secret = "run-progress-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	summary := "Started: Authorization: Bearer " + secret + "\nupstream=" + internalURL
	payload := agentic.RunProgressData{
		RecentActivity: []agentic.ActivityItem{{Type: agentic.ActivitySubtask, Summary: summary}},
	}

	event := agentic.NewRunEvent(agentic.EventRunProgress, payload)

	var data agentic.RunProgressData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	require.Len(t, data.RecentActivity, 1)
	assert.NotContains(t, data.RecentActivity[0].Summary, secret)
	assert.NotContains(t, data.RecentActivity[0].Summary, internalURL)
	assert.Contains(t, data.RecentActivity[0].Summary, "[REDACTED]")
	assert.Contains(t, data.RecentActivity[0].Summary, "<upstream>")
	assert.Equal(t, summary, payload.RecentActivity[0].Summary)
}

func TestNewRunEvent_ToolUseSummaryRedactsSecrets(t *testing.T) {
	const secret = "sk-ant-abcdefghijklmnopqrstuvwxyz123456"
	event := agentic.NewRunEvent(agentic.EventToolUseSummary, agentic.ToolUseSummaryData{
		TurnIndex: 1,
		Summary:   "Fetched api_key=" + secret + " from http://agenthub-provider:8080/v1/chat",
	})

	var data agentic.ToolUseSummaryData
	require.NoError(t, json.Unmarshal(event.Data, &data))
	assert.NotContains(t, data.Summary, secret)
	assert.NotContains(t, data.Summary, "http://agenthub-provider:8080/v1/chat")
	assert.Contains(t, data.Summary, "[REDACTED]")
	assert.Contains(t, data.Summary, "<upstream>")
}

func TestNewRunEvent_DiagnosticPayloadsRedactSecrets(t *testing.T) {
	const secret = "diagnostic-secret-value"
	const internalURL = "http://agenthub-provider:8080/v1/chat"
	message := "Authorization: Bearer " + secret + "\nprovider=" + internalURL
	tests := []struct {
		name  string
		event agentic.RunEvent
		field string
	}{
		{
			name:  "error",
			event: agentic.NewRunEvent(agentic.EventError, agentic.ErrorData{Message: message}),
			field: "message",
		},
		{
			name:  "warning",
			event: agentic.NewRunEvent(agentic.EventWarning, agentic.WarningData{Message: message}),
			field: "message",
		},
		{
			name:  "model fallback",
			event: agentic.NewRunEvent(agentic.EventModelFallback, agentic.ModelFallbackData{Reason: message}),
			field: "reason",
		},
		{
			name:  "tool denied",
			event: agentic.NewRunEvent(agentic.EventToolDenied, agentic.ToolDeniedData{Reason: message}),
			field: "reason",
		},
		{
			name:  "stop hook summary",
			event: agentic.NewRunEvent(agentic.EventStopHookSummary, agentic.StopHookSummaryData{Summary: message}),
			field: "summary",
		},
		{
			name:  "subtask completion",
			event: agentic.NewRunEvent(agentic.EventSubtaskComplete, agentic.SubtaskCompleteData{Error: &message}),
			field: "error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(tc.event.Data, &data))
			var got string
			require.NoError(t, json.Unmarshal(data[tc.field], &got))
			assert.NotContains(t, got, secret)
			assert.NotContains(t, got, internalURL)
			assert.Contains(t, got, "[REDACTED]")
			assert.Contains(t, got, "<upstream>")
		})
	}
}

func TestNewRunEvent_PreservesMalformedRawMessageAsJSONString(t *testing.T) {
	malformed := json.RawMessage(`{"query":`)
	cases := []struct {
		name  string
		event agentic.RunEvent
		field string
	}{
		{
			name:  "tool call input",
			event: agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{Input: malformed}),
			field: "input",
		},
		{
			name:  "tool result output",
			event: agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{Output: malformed}),
			field: "output",
		},
		{
			name:  "input request payload",
			event: agentic.NewRunEvent(agentic.EventInputRequest, agentic.InputRequestData{Payload: malformed}),
			field: "payload",
		},
		{
			name:  "frontend action arguments",
			event: agentic.NewRunEvent(agentic.EventFrontendActionCall, agentic.FrontendActionCallData{Arguments: malformed}),
			field: "arguments",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, json.Valid(tc.event.Data), "SSE envelope must remain valid JSON")

			var data map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(tc.event.Data, &data))

			var recovered string
			require.NoError(t, json.Unmarshal(data[tc.field], &recovered))
			assert.Equal(t, string(malformed), recovered)
		})
	}
}

func FuzzNewRunEventPreservesRawMessageEnvelope(f *testing.F) {
	f.Add("field-input", `{"query":`)
	f.Add("field-output", `{"results":[]}`)
	f.Add("field-payload", "")
	f.Add("field-arguments", "not-json")

	f.Fuzz(func(t *testing.T, fieldSeed, raw string) {
		field := rawMessageEventField(fieldSeed)
		value := json.RawMessage(raw)
		var event agentic.RunEvent

		switch field {
		case "input":
			event = agentic.NewRunEvent(agentic.EventToolCallStart, agentic.ToolCallStartData{Input: value})
		case "output":
			event = agentic.NewRunEvent(agentic.EventToolResult, agentic.ToolResultData{Output: value})
		case "payload":
			event = agentic.NewRunEvent(agentic.EventInputRequest, agentic.InputRequestData{Payload: value})
		case "arguments":
			event = agentic.NewRunEvent(agentic.EventFrontendActionCall, agentic.FrontendActionCallData{Arguments: value})
		}

		if !json.Valid(event.Data) {
			t.Fatalf("invalid SSE envelope for %q and %q: %q", field, raw, event.Data)
		}

		if raw == "" && field == "output" {
			return
		}

		var data map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode envelope for %q and %q: %v", field, raw, err)
		}
		fieldData, ok := data[field]
		if !ok {
			t.Fatalf("missing %q for %q in %q", field, raw, event.Data)
		}
		if json.Valid(value) {
			if !json.Valid(fieldData) {
				t.Fatalf("valid raw value was not preserved for %q: %q", field, fieldData)
			}
			return
		}

		var recovered string
		if err := json.Unmarshal(fieldData, &recovered); err != nil {
			t.Fatalf("malformed raw value was not converted to a JSON string for %q: %v", field, err)
		}
		expected := string(bytes.ToValidUTF8(value, []byte("�")))
		if recovered != expected {
			t.Fatalf("recovered raw value = %q, want %q", recovered, expected)
		}
	})
}

func rawMessageEventField(seed string) string {
	switch seed {
	case "field-input":
		return "input"
	case "field-output":
		return "output"
	case "field-payload":
		return "payload"
	case "field-arguments":
		return "arguments"
	}
	sum := 0
	for i := 0; i < len(seed); i++ {
		sum += int(seed[i])
	}
	return []string{"input", "output", "payload", "arguments"}[sum%4]
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

	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(evt.Data, &wire))
	var isError bool
	require.NoError(t, json.Unmarshal(wire["isError"], &isError))
	assert.False(t, isError)
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

	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(evt.Data, &wire))
	var isError bool
	require.NoError(t, json.Unmarshal(wire["isError"], &isError))
	assert.True(t, isError)
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

func TestNewRunEvent_RunComplete_Timing(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventRunComplete, agentic.RunCompleteData{
		TotalTurns:  1,
		TotalTokens: 42,
		Timing: &agentic.RunTimingData{
			FirstOutputMS:       12,
			StreamCompleteMS:    30,
			AssistantPersistMS:  7,
			PostTurnLifecycleMS: 4,
			TotalMS:             35,
		},
	})

	var data agentic.RunCompleteData
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	require.NotNil(t, data.Timing)
	assert.Equal(t, int64(12), data.Timing.FirstOutputMS)
	assert.Equal(t, int64(30), data.Timing.StreamCompleteMS)
	assert.Equal(t, int64(7), data.Timing.AssistantPersistMS)
	assert.Equal(t, int64(4), data.Timing.PostTurnLifecycleMS)
	assert.Equal(t, int64(35), data.Timing.TotalMS)
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
