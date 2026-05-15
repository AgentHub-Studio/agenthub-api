package agentic_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	commonsai "github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// stubFrontendActionsProvider implements [agentic.FrontendActionsProvider]
// for runner-level tests: the catalog is pre-populated and Submit resolves
// synchronously with [Result].
type stubFrontendActionsProvider struct {
	actions []agentic.FrontendAction
	Result  agentic.FrontendActionResult
	called  atomic.Int32
}

func (s *stubFrontendActionsProvider) GetActions(_ uuid.UUID) []agentic.FrontendAction {
	return s.actions
}

func (s *stubFrontendActionsProvider) Submit(_ context.Context, _ uuid.UUID, callID, _ string, _ json.RawMessage) agentic.FrontendActionResult {
	s.called.Add(1)
	res := s.Result
	res.ID = callID
	return res
}

func (s *stubFrontendActionsProvider) Calls() int32 { return s.called.Load() }

// TestRunner_FrontendAction_HappyPath verifies the full Phase 1 flow inside
// the runner: a declared frontend action is injected into the LLM tool catalog,
// the LLM calls it, the runner emits EventFrontendActionCall + EventToolResult,
// and the action's Result is forwarded to the next LLM turn.
func TestRunner_FrontendAction_HappyPath(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []commonsai.Message, opts commonsai.ChatOptions) (<-chan commonsai.StreamChunk, error) {
			switch idx {
			case 0:
				// Verify the frontend action made it into the tool catalog.
				found := false
				for _, t := range opts.Tools {
					if t.Function.Name == "navigate_to" {
						found = true
					}
				}
				if !found {
					return makeTextStream("no navigate_to in catalog"), nil
				}
				return makeToolCallStream("tc-1", "navigate_to", `{"route":"/x"}`), nil
			default:
				return makeTextStream("done"), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	config.ToolTimeout = 5 * time.Second

	runner := newTestRunner(model, persister, history, config)

	provider := &stubFrontendActionsProvider{
		actions: []agentic.FrontendAction{
			{
				Name:        "navigate_to",
				Description: "Navega o usuário",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"route":{"type":"string"}}}`),
			},
		},
		Result: agentic.FrontendActionResult{
			Status: "ok",
			Result: json.RawMessage(`{"navigated":true}`),
		},
	}

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:       uuid.New(),
		AgentID:         uuid.New(),
		UserMessage:     "abra /x",
		SystemPrompt:    "you can navigate the user",
		TenantID:        "test-tenant",
		FrontendActions: provider,
	})

	events := collectEvents(ch)

	// The runner must have called Submit on our provider exactly once.
	assert.Equal(t, int32(1), provider.Calls(), "Submit should fire for the frontend tool call")

	// And emitted the new event type to the SSE stream.
	if assert.True(t, hasEventType(events, agentic.EventFrontendActionCall), "EventFrontendActionCall not emitted") {
		ev := findEvent(t, events, agentic.EventFrontendActionCall)
		var data agentic.FrontendActionCallData
		require.NoError(t, json.Unmarshal(ev.Data, &data))
		assert.Equal(t, "tc-1", data.ID)
		assert.Equal(t, "navigate_to", data.Name)
	}

	// The Result must reach the chat history as a tool_result, with the
	// JSON payload we supplied via the stub.
	assert.True(t, hasEventType(events, agentic.EventToolResult))

	// And the run must complete normally (no errors).
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
}

// TestRunner_FrontendAction_ErrorResult ensures status="error" results land
// as a tool error and the run still proceeds to completion.
func TestRunner_FrontendAction_ErrorResult(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []commonsai.Message, _ commonsai.ChatOptions) (<-chan commonsai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc-1", "navigate_to", `{}`), nil
			default:
				return makeTextStream("recovered"), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	config.ToolTimeout = 5 * time.Second

	runner := newTestRunner(model, persister, history, config)

	provider := &stubFrontendActionsProvider{
		actions: []agentic.FrontendAction{{Name: "navigate_to", Description: "x"}},
		Result:  agentic.FrontendActionResult{Status: "error", Error: "user cancelled"},
	}

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:       uuid.New(),
		AgentID:         uuid.New(),
		UserMessage:     "navega",
		SystemPrompt:    "p",
		TenantID:        "test-tenant",
		FrontendActions: provider,
	}))

	// EventFrontendActionCall fires regardless of success/failure.
	assert.True(t, hasEventType(events, agentic.EventFrontendActionCall))

	// EventToolResult fires with the error string surfaced.
	tr := findEvent(t, events, agentic.EventToolResult)
	var trData agentic.ToolResultData
	require.NoError(t, json.Unmarshal(tr.Data, &trData))
	if assert.NotNil(t, trData.Error, "tool result should carry an error") {
		assert.Contains(t, *trData.Error, "user cancelled")
	}

	// Run still terminates cleanly.
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
}

// TestRunner_NoFrontendActions_NoSubmit ensures the provider's Submit is never
// invoked when the LLM doesn't call any declared frontend action.
func TestRunner_NoFrontendActions_NoSubmit(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []commonsai.Message, _ commonsai.ChatOptions) (<-chan commonsai.StreamChunk, error) {
			return makeTextStream("ok"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()

	runner := newTestRunner(model, persister, history, config)

	provider := &stubFrontendActionsProvider{actions: nil}

	collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:       uuid.New(),
		AgentID:         uuid.New(),
		UserMessage:     "hi",
		SystemPrompt:    "p",
		TenantID:        "test-tenant",
		FrontendActions: provider,
	}))

	assert.Equal(t, int32(0), provider.Calls(),
		"Submit must not fire when LLM doesn't call a frontend action")
}
