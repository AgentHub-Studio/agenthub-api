package chat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterReplayableEvents_RemovesResolvedInputRequest(t *testing.T) {
	events := []BufferedEvent{
		{
			ID: 1,
			Event: RunEvent{
				Type: "input_request",
				Data: json.RawMessage(`{"requestId":"call_1","payload":{"type":"form"}}`),
			},
		},
		{
			ID: 2,
			Event: RunEvent{
				Type: "tool_progress",
				Data: json.RawMessage(`{"id":"call_1","name":"ask_user","state":"completed"}`),
			},
		},
		{
			ID: 3,
			Event: RunEvent{
				Type: "tool_result",
				Data: json.RawMessage(`{"id":"call_1","name":"ask_user","output":{"name":"Minha Skill"}}`),
			},
		},
		{
			ID: 4,
			Event: RunEvent{
				Type: "run_complete",
				Data: json.RawMessage(`{"totalTurns":1}`),
			},
		},
	}

	filtered := filterReplayableEvents(events)
	require.Len(t, filtered, 3)
	assert.Equal(t, "tool_progress", filtered[0].Event.Type)
	assert.Equal(t, "tool_result", filtered[1].Event.Type)
	assert.Equal(t, "run_complete", filtered[2].Event.Type)
}

func TestFilterReplayableEvents_KeepsPendingInputRequest(t *testing.T) {
	events := []BufferedEvent{
		{
			ID: 1,
			Event: RunEvent{
				Type: "input_request",
				Data: json.RawMessage(`{"requestId":"call_2","payload":{"type":"form"}}`),
			},
		},
		{
			ID: 2,
			Event: RunEvent{
				Type: "heartbeat",
				Data: json.RawMessage(`{"ts":123}`),
			},
		},
	}

	filtered := filterReplayableEvents(events)
	require.Len(t, filtered, 2)
	assert.Equal(t, "input_request", filtered[0].Event.Type)
	assert.Equal(t, "heartbeat", filtered[1].Event.Type)
}
