package agentic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- mocks ---

// mockChatModel implements ai.ChatModel for testing.
type mockChatModel struct {
	mu        sync.Mutex
	callCount int
	// streamFn returns the stream channel for each call. Index is the call number (0-based).
	streamFn func(callIndex int, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error)
}

func (m *mockChatModel) Chat(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
	return nil, fmt.Errorf("Chat not implemented in mock")
}

func (m *mockChatModel) ChatStream(_ context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	m.mu.Lock()
	idx := m.callCount
	m.callCount++
	m.mu.Unlock()

	return m.streamFn(idx, messages, opts)
}

func (m *mockChatModel) GetProviderName() string { return "mock" }

func (m *mockChatModel) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockPersister records all persisted messages.
type mockPersister struct {
	mu       sync.Mutex
	messages []chat.ChatMessage
}

func (m *mockPersister) CreateMessage(_ context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.ID = uuid.New()
	msg.CreatedAt = time.Now()
	m.messages = append(m.messages, msg)
	return msg, nil
}

func (m *mockPersister) Messages() []chat.ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]chat.ChatMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

// mockHistoryLoader returns pre-defined messages.
type mockHistoryLoader struct {
	messages []chat.ChatMessage
}

func (m *mockHistoryLoader) FindAllMessages(_ context.Context, _ uuid.UUID) ([]chat.ChatMessage, error) {
	return m.messages, nil
}

// --- helper to make a stream that returns text then stops ---

func makeTextStream(text string) <-chan ai.StreamChunk {
	ch := make(chan ai.StreamChunk, 10)
	go func() {
		defer close(ch)
		ch <- ai.StreamChunk{Delta: text}
		ch <- ai.StreamChunk{FinishReason: "stop"}
	}()
	return ch
}

// makeToolCallStream returns a stream that emits a tool call.
func makeToolCallStream(tcID, name, args string) <-chan ai.StreamChunk {
	ch := make(chan ai.StreamChunk, 10)
	go func() {
		defer close(ch)
		ch <- ai.StreamChunk{
			ToolCallDelta: &ai.ToolCall{
				ID:   tcID,
				Type: "function",
				Function: ai.ToolFunction{
					Name:      name,
					Arguments: args,
				},
			},
		}
		ch <- ai.StreamChunk{FinishReason: "tool_calls"}
	}()
	return ch
}

// collectEvents drains the event channel into a slice.
func collectEvents(ch <-chan agentic.RunEvent) []agentic.RunEvent {
	var events []agentic.RunEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

// findEvent returns the first event of the given type, or fails the test.
func findEvent(t *testing.T, events []agentic.RunEvent, typ agentic.RunEventType) agentic.RunEvent {
	t.Helper()
	for _, ev := range events {
		if ev.Type == typ {
			return ev
		}
	}
	t.Fatalf("event %s not found in %d events", typ, len(events))
	return agentic.RunEvent{}
}

func hasEventType(events []agentic.RunEvent, typ agentic.RunEventType) bool {
	for _, ev := range events {
		if ev.Type == typ {
			return true
		}
	}
	return false
}

// newTestRunner creates a Runner with mocked dependencies for testing.
func newTestRunner(
	model ai.ChatModel,
	persister agentic.MessagePersister,
	history agentic.HistoryLoader,
	config agentic.RunConfig,
) *agentic.Runner {
	skills := &mockSkillLister{skills: nil}
	kbs := &mockKBLister{kbs: nil}

	prompt := agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig())
	tools := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:9999")

	return agentic.NewRunner(
		model, skillClient, prompt, tools,
		nil, nil, // no context manager or memory bridge
		persister, history, config,
	)
}

// --- tests ---

func TestRunner_SimpleTextResponse(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("Hello, world!"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5

	runner := newTestRunner(model, persister, history, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Hi",
		SystemPrompt: "You are a test assistant.",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	// Should have: text_delta, turn_complete, run_complete
	assert.True(t, hasEventType(events, agentic.EventTextDelta))
	assert.True(t, hasEventType(events, agentic.EventTurnComplete))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.False(t, hasEventType(events, agentic.EventError))

	// Check text delta content.
	td := findEvent(t, events, agentic.EventTextDelta)
	var textData agentic.TextDeltaData
	require.NoError(t, json.Unmarshal(td.Data, &textData))
	assert.Equal(t, "Hello, world!", textData.Content)

	// Check run complete.
	rc := findEvent(t, events, agentic.EventRunComplete)
	var runComplete agentic.RunCompleteData
	require.NoError(t, json.Unmarshal(rc.Data, &runComplete))
	assert.Equal(t, 1, runComplete.TotalTurns)

	// Should have persisted user + assistant messages.
	msgs := persister.Messages()
	assert.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "Hello, world!", msgs[1].Content)
}

func TestRunner_ToolCallThenStop(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				// First call: LLM wants to call a tool.
				return makeToolCallStream("tc_1", "execute-sql", `{"query":"SELECT 1"}`), nil
			default:
				// Second call: LLM responds with text.
				return makeTextStream("The result is 1."), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	config.ToolTimeout = 5 * time.Second

	runner := newTestRunner(model, persister, history, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Run SELECT 1",
		SystemPrompt: "You are a SQL assistant.",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	// Should have: tool_call_start, tool_result, turn_complete (tool turn),
	// text_delta, turn_complete (final), run_complete
	assert.True(t, hasEventType(events, agentic.EventToolCallStart))
	assert.True(t, hasEventType(events, agentic.EventToolResult))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))

	// The LLM should have been called twice.
	assert.Equal(t, 2, model.CallCount())

	// Check tool_call_start event.
	tcs := findEvent(t, events, agentic.EventToolCallStart)
	var toolStart agentic.ToolCallStartData
	require.NoError(t, json.Unmarshal(tcs.Data, &toolStart))
	assert.Equal(t, "tc_1", toolStart.ID)
	assert.Equal(t, "execute-sql", toolStart.Name)

	// Persisted messages: user, assistant(tool_use), tool(result), assistant(text).
	msgs := persister.Messages()
	assert.GreaterOrEqual(t, len(msgs), 4)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, chat.MessageTypeToolUse, msgs[1].MessageType)
	assert.Equal(t, "tool", msgs[2].Role)
	assert.Equal(t, chat.MessageTypeToolResult, msgs[2].MessageType)
}

func TestRunner_MaxIterationsSafetyBrake(t *testing.T) {
	// LLM always returns tool calls, never stops.
	callIdx := 0
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			callIdx++
			return makeToolCallStream(
				fmt.Sprintf("tc_%d", callIdx),
				"execute-sql",
				`{"query":"SELECT 1"}`,
			), nil
		},
	}

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	config.ToolTimeout = 2 * time.Second

	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "loop forever",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	// Should hit max iterations error.
	assert.True(t, hasEventType(events, agentic.EventError))
	errEv := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEv.Data, &errData))
	assert.Equal(t, "max_iterations", errData.Code)
	assert.Contains(t, errData.Message, "maximum iterations")
}

func TestRunner_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			// Cancel context before returning stream.
			cancel()
			ch := make(chan ai.StreamChunk, 1)
			close(ch)
			return ch, nil
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	evCh := runner.Run(ctx, agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "test",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(evCh)

	// Channel should be closed (events may or may not contain an error depending on timing).
	// The important thing is no panic and the channel closes.
	_ = events
}

func TestRunner_LLMError(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return nil, fmt.Errorf("API key expired")
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "test",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	assert.True(t, hasEventType(events, agentic.EventError))
	errEv := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEv.Data, &errData))
	assert.Equal(t, "llm_call", errData.Code)
	assert.Contains(t, errData.Message, "API key expired")
}

func TestRunner_StreamError(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 2)
			go func() {
				defer close(ch)
				ch <- ai.StreamChunk{Delta: "partial"}
				ch <- ai.StreamChunk{Error: fmt.Errorf("stream disconnected")}
			}()
			return ch, nil
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "test",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	assert.True(t, hasEventType(events, agentic.EventError))
	errEv := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEv.Data, &errData))
	assert.Equal(t, "stream_consume", errData.Code)
}

func TestRunner_MultipleTextDeltas(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 10)
			go func() {
				defer close(ch)
				ch <- ai.StreamChunk{Delta: "Hello"}
				ch <- ai.StreamChunk{Delta: " "}
				ch <- ai.StreamChunk{Delta: "world"}
				ch <- ai.StreamChunk{FinishReason: "stop"}
			}()
			return ch, nil
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Hi",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	// Count text_delta events.
	deltaCount := 0
	fullText := ""
	for _, ev := range events {
		if ev.Type == agentic.EventTextDelta {
			deltaCount++
			var td agentic.TextDeltaData
			_ = json.Unmarshal(ev.Data, &td)
			fullText += td.Content
		}
	}
	assert.Equal(t, 3, deltaCount)
	assert.Equal(t, "Hello world", fullText)
}

func TestRunner_WithHistory(t *testing.T) {
	var capturedMessages []ai.Message

	model := &mockChatModel{
		streamFn: func(_ int, msgs []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			capturedMessages = msgs
			return makeTextStream("I remember!"), nil
		},
	}

	history := &mockHistoryLoader{
		messages: []chat.ChatMessage{
			{Role: "user", Content: "Hello", MessageType: chat.MessageTypeText},
			{Role: "assistant", Content: "Hi there!", MessageType: chat.MessageTypeText},
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, history, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Do you remember?",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	_ = collectEvents(ch)

	// Messages sent to LLM should include history + new user message.
	require.Len(t, capturedMessages, 3)
	assert.Equal(t, "user", capturedMessages[0].Role)
	assert.Equal(t, "Hello", capturedMessages[0].Content)
	assert.Equal(t, "assistant", capturedMessages[1].Role)
	assert.Equal(t, "Hi there!", capturedMessages[1].Content)
	assert.Equal(t, "user", capturedMessages[2].Role)
	assert.Equal(t, "Do you remember?", capturedMessages[2].Content)
}

func TestRunner_MaxTokensFinishReason(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 5)
			go func() {
				defer close(ch)
				ch <- ai.StreamChunk{Delta: "truncated..."}
				ch <- ai.StreamChunk{FinishReason: "length"}
			}()
			return ch, nil
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	evCh := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "test",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(evCh)

	assert.True(t, hasEventType(events, agentic.EventError))
	errEv := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEv.Data, &errData))
	assert.Equal(t, "max_tokens", errData.Code)
}

func TestRunner_PersistedAssistantHasToolCalls(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			if idx == 0 {
				return makeToolCallStream("tc_1", "web-scraper", `{"url":"https://example.com"}`), nil
			}
			return makeTextStream("Done."), nil
		},
	}

	persister := &mockPersister{}
	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 2 * time.Second
	runner := newTestRunner(model, persister, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "scrape example.com",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	_ = collectEvents(ch)

	msgs := persister.Messages()
	// Find the tool_use message.
	var toolUseMsg *chat.ChatMessage
	for i := range msgs {
		if msgs[i].MessageType == chat.MessageTypeToolUse {
			toolUseMsg = &msgs[i]
			break
		}
	}
	require.NotNil(t, toolUseMsg, "should have a tool_use message")
	assert.NotEmpty(t, toolUseMsg.ToolCalls)

	// Verify tool_calls JSON is valid.
	var tcs []ai.ToolCall
	require.NoError(t, json.Unmarshal(toolUseMsg.ToolCalls, &tcs))
	assert.Len(t, tcs, 1)
	assert.Equal(t, "tc_1", tcs[0].ID)
	assert.Equal(t, "web-scraper", tcs[0].Function.Name)
}

func TestRunner_EventSequence(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			if idx == 0 {
				return makeToolCallStream("tc_1", "execute-sql", `{"query":"SELECT 1"}`), nil
			}
			return makeTextStream("Result: 1"), nil
		},
	}

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 2 * time.Second
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "query",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)
	types := make([]agentic.RunEventType, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}

	// Expected sequence:
	// tool_call_start → tool_result → turn_complete → text_delta → turn_complete → run_complete
	tcStartIdx := -1
	tcResultIdx := -1
	firstTurnComplete := -1
	textDeltaIdx := -1
	runCompleteIdx := -1

	for i, typ := range types {
		switch typ {
		case agentic.EventToolCallStart:
			if tcStartIdx == -1 {
				tcStartIdx = i
			}
		case agentic.EventToolResult:
			if tcResultIdx == -1 {
				tcResultIdx = i
			}
		case agentic.EventTurnComplete:
			if firstTurnComplete == -1 {
				firstTurnComplete = i
			}
		case agentic.EventTextDelta:
			if textDeltaIdx == -1 {
				textDeltaIdx = i
			}
		case agentic.EventRunComplete:
			runCompleteIdx = i
		}
	}

	assert.Greater(t, tcStartIdx, -1, "should have tool_call_start")
	assert.Greater(t, tcResultIdx, tcStartIdx, "tool_result should follow tool_call_start")
	assert.Greater(t, firstTurnComplete, tcResultIdx, "turn_complete should follow tool_result")
	assert.Greater(t, textDeltaIdx, firstTurnComplete, "text_delta should follow first turn_complete")
	assert.Greater(t, runCompleteIdx, textDeltaIdx, "run_complete should be last")
}
