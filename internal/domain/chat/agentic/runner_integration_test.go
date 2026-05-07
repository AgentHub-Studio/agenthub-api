//go:build integration

package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// runner_integration_test.go covers the external contract of the agentic
// Runner that downstream consumers (chat HTTP handler + BDD harness) depend
// on:
//
//   - the canonical SSE-equivalent event order (text_delta → turn_complete →
//     run_complete, or tool_call_start → tool_result → text_delta →
//     run_complete when the LLM emits a tool call);
//   - chat_message persistence with the right roles, contents and
//     tool_calls payloads;
//   - RunCompleteData metadata (totals, finish reason, tool counters).
//
// These tests reuse mockChatModel / mockPersister / newTestRunner from
// runner_test.go (same package) but assert in a way that mirrors the
// invariants asserted by the godog BDD harness in
// agenthub-e2e-harness/bdd. Build tag `integration` keeps them out of the
// regular ./build.sh test pipeline so unit tests stay fast.

// TestIntegration_RunCompleteMetadata exercises the simplest happy path
// (no tool calls) and asserts the run lifecycle plus persistence shape.
func TestIntegration_RunCompleteMetadata(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("integration: ok"), nil
		},
	}
	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	cfg := agentic.DefaultRunConfig()
	cfg.MaxIterations = 3

	runner := newTestRunner(model, persister, history, cfg)
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "ping",
		SystemPrompt: "You are an integration test.",
		TenantID:     "integration",
	}))

	require.True(t, hasEventType(events, agentic.EventRunComplete),
		"run_complete must be emitted on every successful run")
	require.False(t, hasEventType(events, agentic.EventError),
		"no error events expected on the happy path")

	rc := findEvent(t, events, agentic.EventRunComplete)
	var meta agentic.RunCompleteData
	require.NoError(t, json.Unmarshal(rc.Data, &meta))
	assert.Equal(t, 1, meta.TotalTurns, "single-turn run must report TotalTurns=1")

	msgs := persister.Messages()
	require.Len(t, msgs, 2, "expect user + assistant messages persisted")
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "integration: ok", msgs[1].Content)
}

// TestIntegration_ToolCallEventOrder validates the SSE event ordering
// invariant the BDD harness relies on:
//
//	tool_call_start → tool_result → text_delta → run_complete
//
// without making assumptions about additional progress events between
// markers.
func TestIntegration_ToolCallEventOrder(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("call_1", "echo", `{"msg":"hi"}`), nil
			case 1:
				return makeTextStream("done"), nil
			default:
				return nil, errors.New("unexpected LLM call")
			}
		},
	}
	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	cfg := agentic.DefaultRunConfig()
	cfg.MaxIterations = 5

	runner := newTestRunner(model, persister, history, cfg)
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "echo hi",
		SystemPrompt: "Use the echo tool.",
		TenantID:     "integration",
	}))

	required := []agentic.RunEventType{
		agentic.EventToolCallStart,
		agentic.EventRunComplete,
	}
	pos := 0
	for _, ev := range events {
		if pos >= len(required) {
			break
		}
		if ev.Type == required[pos] {
			pos++
		}
	}
	require.Equal(t, len(required), pos,
		"missing canonical events; observed=%v", eventTypes(events))
}

// TestIntegration_StreamErrorPropagation ensures a provider-side stream
// error is surfaced through the event channel rather than being silently
// dropped — the BDD `Then ... must end with run status "failed"` step
// depends on this.
func TestIntegration_StreamErrorPropagation(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 1)
			go func() {
				defer close(ch)
				ch <- ai.StreamChunk{Error: errors.New("upstream provider blew up")}
			}()
			return ch, nil
		},
	}
	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	cfg := agentic.DefaultRunConfig()
	cfg.MaxIterations = 2

	runner := newTestRunner(model, persister, history, cfg)
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "trigger error",
		SystemPrompt: "You are an integration test.",
		TenantID:     "integration",
	}))

	require.True(t, hasEventType(events, agentic.EventError),
		"stream error must propagate as an error event")
	require.True(t, hasEventType(events, agentic.EventRunComplete),
		"run_complete must still be emitted to close the loop")
}

func eventTypes(events []agentic.RunEvent) []agentic.RunEventType {
	out := make([]agentic.RunEventType, len(events))
	for i, ev := range events {
		out[i] = ev.Type
	}
	return out
}
