package agentic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- mocks ---

// mockChatModel implements ai.ChatModel for testing.
type mockChatModel struct {
	mu            sync.Mutex
	callCount     int
	chatCallCount int
	chatFn        func(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error)
	// streamFn returns the stream channel for each call. Index is the call number (0-based).
	streamFn func(callIndex int, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error)
}

func (m *mockChatModel) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	m.mu.Lock()
	m.chatCallCount++
	chatFn := m.chatFn
	m.mu.Unlock()
	if chatFn != nil {
		return chatFn(ctx, messages, opts)
	}
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

func (m *mockChatModel) ChatCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chatCallCount
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

type interruptAwarePersister struct {
	mockPersister
	toolResultContextErr error
	toolResultPersisted  bool
}

func (m *interruptAwarePersister) CreateMessage(ctx context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	if msg.MessageType == chat.MessageTypeToolResult {
		m.mu.Lock()
		m.toolResultContextErr = ctx.Err()
		m.toolResultPersisted = true
		m.mu.Unlock()
	}
	return m.mockPersister.CreateMessage(ctx, msg)
}

func (m *interruptAwarePersister) ToolResultContextErr() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.toolResultContextErr
}

func (m *interruptAwarePersister) ToolResultPersisted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.toolResultPersisted
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

func makeFragmentedToolCallStream(tcID, name string, argParts ...string) <-chan ai.StreamChunk {
	ch := make(chan ai.StreamChunk, len(argParts)+2)
	go func() {
		defer close(ch)
		ch <- ai.StreamChunk{
			ToolCallDelta: &ai.ToolCall{
				ID:   tcID,
				Type: "function",
				Function: ai.ToolFunction{
					Name: name,
				},
			},
		}
		for _, part := range argParts {
			ch <- ai.StreamChunk{
				ToolCallDelta: &ai.ToolCall{
					ID:   tcID,
					Type: "function",
					Function: ai.ToolFunction{
						Arguments: part,
					},
				},
			}
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

func filterEvents(events []agentic.RunEvent, typ agentic.RunEventType) []agentic.RunEvent {
	var out []agentic.RunEvent
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
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
		persister, history,
		nil, // no hook executor
		config,
	)
}

type testElicitationSubmitter struct {
	action     agentic.ElicitationAction
	mu         sync.Mutex
	requestIDs []string
	params     []agentic.ElicitationParams
}

func (s *testElicitationSubmitter) Submit(_ context.Context, _ string, requestID string, params agentic.ElicitationParams) agentic.ElicitationResult {
	s.mu.Lock()
	s.requestIDs = append(s.requestIDs, requestID)
	s.params = append(s.params, params)
	s.mu.Unlock()
	return agentic.ElicitationResult{Action: s.action}
}

func (s *testElicitationSubmitter) Requests() ([]string, []agentic.ElicitationParams) {
	s.mu.Lock()
	defer s.mu.Unlock()
	requestIDs := append([]string(nil), s.requestIDs...)
	params := append([]agentic.ElicitationParams(nil), s.params...)
	return requestIDs, params
}

func newRunnerWithBoundSQLTool(t *testing.T, model ai.ChatModel, config agentic.RunConfig) (*agentic.Runner, *atomic.Int32) {
	t.Helper()

	var executeRequests atomic.Int32
	skillRuntime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executeRequests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"executed":true},"latencyMs":1}`))
	}))
	t.Cleanup(skillRuntime.Close)

	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{{
		ID:          skillID,
		Name:        "Execute SQL",
		Slug:        "execute-sql",
		Description: "Execute SQL queries",
	}}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:          toolID,
			Name:        "SQL Tool",
			Type:        "SQL",
			Config:      json.RawMessage(`{"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}`),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		}},
	}

	prompt := agentic.NewPromptBuilder(skills, &mockKBLister{}, nil, agentic.DefaultPromptConfig())
	toolBuilder := agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{})
	skillClient := agentic.NewSkillRuntimeClient(skillRuntime.URL)

	runner := agentic.NewRunner(
		model, skillClient, prompt, toolBuilder,
		nil, nil,
		&mockPersister{}, &mockHistoryLoader{},
		nil,
		config,
	)
	return runner, &executeRequests
}

func containsMessage(messages []ai.Message, want string) bool {
	for _, message := range messages {
		if message.Content == want {
			return true
		}
	}
	return false
}

func containsPersistedMessage(messages []chat.ChatMessage, want string) bool {
	for _, message := range messages {
		if message.Content == want {
			return true
		}
	}
	return false
}

type postToolHTTPHookRepository struct {
	hooks []agentic.AgentHook
}

func (r *postToolHTTPHookRepository) FindByAgentAndEvent(_ context.Context, agentID uuid.UUID, event agentic.HookEvent) ([]agentic.AgentHook, error) {
	var matching []agentic.AgentHook
	for _, hook := range r.hooks {
		if hook.AgentID == agentID && hook.Event == event && hook.Enabled {
			matching = append(matching, hook)
		}
	}
	return matching, nil
}

func (r *postToolHTTPHookRepository) DisableHook(_ context.Context, hookID uuid.UUID) error {
	for index, hook := range r.hooks {
		if hook.ID == hookID {
			r.hooks[index].Enabled = false
		}
	}
	return nil
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
	require.NotNil(t, runComplete.Timing)
	assert.GreaterOrEqual(t, runComplete.Timing.TotalMS, runComplete.Timing.FirstOutputMS)
	assert.GreaterOrEqual(t, runComplete.Timing.TotalMS, runComplete.Timing.StreamCompleteMS)

	// Should have persisted user + assistant messages.
	msgs := persister.Messages()
	assert.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "Hello, world!", msgs[1].Content)
}

func TestRunner_StreamingEndpoint404RecoversAsSSEEvents(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return nil, fmt.Errorf("provider returned HTTP 404 for streaming endpoint")
		},
		chatFn: func(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
			return &ai.ChatResponse{
				Content:      "recovered as SSE",
				FinishReason: "stop",
				Usage:        ai.Usage{TotalTokens: 7},
			}, nil
		},
	}
	config := agentic.DefaultRunConfig()
	config.RetryMaxAttempts = 1
	config.MaxIterations = 1

	events := collectEvents(newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config).Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hello",
		SystemPrompt: "test",
		TenantID:     "test-tenant",
	}))

	assert.Equal(t, 1, model.CallCount())
	assert.Equal(t, 1, model.ChatCallCount())
	assert.True(t, hasEventType(events, agentic.EventTextDelta))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.False(t, hasEventType(events, agentic.EventError))

	textDelta := findEvent(t, events, agentic.EventTextDelta)
	var textData agentic.TextDeltaData
	require.NoError(t, json.Unmarshal(textDelta.Data, &textData))
	assert.Equal(t, "recovered as SSE", textData.Content)
}

func TestRunner_OutputProcessorsRedactTextDeltasAndPersistedAssistantMessage(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("Contact user@example.com"), nil
		},
	}

	persister := &mockPersister{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	runner := newTestRunner(model, persister, &mockHistoryLoader{}, config)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:        uuid.New(),
		AgentID:          uuid.New(),
		UserMessage:      "Hi",
		SystemPrompt:     "You are a test assistant.",
		TenantID:         "test-tenant",
		OutputProcessors: []string{"pii_redactor"},
	}))

	textEvents := filterEvents(events, agentic.EventTextDelta)
	require.Len(t, textEvents, 1)
	var textData agentic.TextDeltaData
	require.NoError(t, json.Unmarshal(textEvents[0].Data, &textData))
	assert.Equal(t, "Contact [REDACTED:EMAIL]", textData.Content)
	assert.NotContains(t, textData.Content, "user@example.com")

	msgs := persister.Messages()
	require.Len(t, msgs, 2)
	assert.Equal(t, "Contact [REDACTED:EMAIL]", msgs[1].Content)
	assert.NotContains(t, msgs[1].Content, "user@example.com")
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

func TestRunner_BlockInterruptCompletesAndPersistsToolResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runtimeCancelled := make(chan struct{})
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			_, _ = w.Write([]byte(`{"output":{"status":"completed"}}`))
		case <-r.Context().Done():
			close(runtimeCancelled)
		}
	}))
	t.Cleanup(runtime.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	block := "block"
	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{{
		ID: skillID, Name: "Destructive Write", Slug: "destructive_write",
	}}}
	toolsBySkill := newMockToolsBySkill()
	toolsBySkill.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID: toolID, Name: "Destructive Write", Slug: "destructive_write", Type: "HTTP",
			InputSchema:       json.RawMessage(`{"type":"object","properties":{}}`),
			InterruptBehavior: &block,
		}},
	}
	prompt := agentic.NewPromptBuilder(skills, &mockKBLister{}, nil, agentic.DefaultPromptConfig())
	model := &mockChatModel{streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
		if idx == 0 {
			return makeToolCallStream("block-call", "destructive_write", `{}`), nil
		}
		return makeTextStream("the runner must not make a second model call"), nil
	}}
	persister := &interruptAwarePersister{}
	config := agentic.DefaultRunConfig()
	config.ToolTimeout = time.Second
	config.TotalTimeout = 2 * time.Second
	runner := agentic.NewRunner(
		model,
		agentic.NewSkillRuntimeClient(runtime.URL),
		prompt,
		agentic.NewToolSchemaBuilder(skills, toolsBySkill, &mockKBLister{}),
		nil,
		nil,
		persister,
		&mockHistoryLoader{},
		nil,
		config,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := runner.Run(ctx, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "interrupt-test",
		UserMessage: "perform the write", SystemPrompt: "Use the provided tool.",
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tool request did not start")
	}
	cancel()
	select {
	case <-runtimeCancelled:
		t.Fatal("block interrupt cancelled the in-flight tool request")
	case <-time.After(75 * time.Millisecond):
	}
	close(release)
	collected := collectEvents(events)

	assert.True(t, hasEventType(collected, agentic.EventToolResult))
	assert.Equal(t, 1, model.CallCount())
	assert.True(t, persister.ToolResultPersisted())
	assert.NoError(t, persister.ToolResultContextErr())
}

func TestRunner_PostToolHTTPHookInjectsResponseIntoNextTurn(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()

	type hookRequest struct {
		Method string
		Header string
		Body   agentic.HookPayload
	}
	hookRequests := make(chan hookRequest, 1)
	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload agentic.HookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		hookRequests <- hookRequest{
			Method: r.Method,
			Header: r.Header.Get("X-Hook-Key"),
			Body:   payload,
		}
		_, _ = w.Write([]byte("post-tool policy accepted"))
	}))
	t.Cleanup(hookServer.Close)

	skillRuntime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/skills/execute-sql/execute", r.URL.Path)
		_, _ = w.Write([]byte(`{"output":{"executed":true},"latencyMs":1}`))
	}))
	t.Cleanup(skillRuntime.Close)

	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{{
		ID:          skillID,
		Name:        "Execute SQL",
		Slug:        "execute-sql",
		Description: "Execute SQL queries",
	}}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:          toolID,
			Name:        "SQL Tool",
			Type:        "SQL",
			Config:      json.RawMessage(`{"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}`),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		}},
	}

	hooks := &postToolHTTPHookRepository{hooks: []agentic.AgentHook{{
		ID:       uuid.New(),
		AgentID:  agentID,
		Event:    agentic.HookPostToolUse,
		Matcher:  "execute-*",
		HookType: agentic.HookTypeHTTP,
		Config: json.RawMessage(`{
			"url": "` + hookServer.URL + `",
			"method": "POST",
			"headers": {"X-Hook-Key": "hook-secret"}
		}`),
		Enabled: true,
	}}}

	var secondTurnMessages []ai.Message
	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("hook-call", "execute-sql", `{"query":"SELECT 42"}`), nil
			case 1:
				secondTurnMessages = append([]ai.Message(nil), messages...)
				return makeTextStream("hook observed"), nil
			default:
				return nil, fmt.Errorf("unexpected model call %d", idx)
			}
		},
	}
	persister := &mockPersister{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	runner := agentic.NewRunner(
		model,
		agentic.NewSkillRuntimeClient(skillRuntime.URL),
		agentic.NewPromptBuilder(skills, &mockKBLister{}, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}),
		nil,
		nil,
		persister,
		&mockHistoryLoader{},
		agentic.NewHookExecutor(hooks),
		config,
	)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    sessionID,
		AgentID:      agentID,
		UserMessage:  "Run the SQL tool",
		SystemPrompt: "Use the available tool.",
		TenantID:     "hook-integration",
	}))

	select {
	case request := <-hookRequests:
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "hook-secret", request.Header)
		assert.Equal(t, agentic.HookPostToolUse, request.Body.Event)
		assert.Equal(t, agentID.String(), request.Body.AgentID)
		assert.Equal(t, sessionID.String(), request.Body.SessionID)
		assert.Equal(t, "execute-sql", request.Body.ToolName)
		assert.JSONEq(t, `{"query":"SELECT 42"}`, string(request.Body.ToolInput))
		assert.JSONEq(t, `{"executed":true}`, string(request.Body.ToolOutput))
	case <-time.After(time.Second):
		t.Fatal("post_tool_use HTTP hook was not called")
	}

	require.Equal(t, 2, model.CallCount())
	require.NotEmpty(t, secondTurnMessages)
	assert.True(t, containsMessage(secondTurnMessages, "[SYSTEM NOTE from hook]\npost-tool policy accepted"))
	assert.True(t, hasEventType(events, agentic.EventToolResult))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.True(t, containsPersistedMessage(persister.Messages(), "[SYSTEM NOTE from hook]\npost-tool policy accepted"))
}

func TestRunner_EmitsToolUseSummaryWhenGeneratorConfigured(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_1", "execute-sql", `{"query":"SELECT 1"}`), nil
			default:
				return makeTextStream("The result is 1."), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	config.ToolTimeout = 5 * time.Second

	runner := newTestRunner(model, persister, history, config).
		WithToolUseSummaryGenerator(agentic.NewToolUseSummaryGenerator(
			&capturingSummaryChatModel{response: "Ran SQL query"},
			"summary-model",
		))

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Run SELECT 1",
		SystemPrompt: "You are a SQL assistant.",
		TenantID:     "test-tenant",
	}))

	ev := findEvent(t, events, agentic.EventToolUseSummary)
	var data agentic.ToolUseSummaryData
	require.NoError(t, json.Unmarshal(ev.Data, &data))
	assert.Equal(t, "Ran SQL query", data.Summary)
}

func TestRunner_FragmentedToolCallDeltasAreMergedByID(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeFragmentedToolCallStream(
					"call_ask_user_1",
					"ask_user",
					`{"message":"`,
					`Qual skill voce quer criar?`,
					`","inputSchema":{"type":"object"}}`,
				), nil
			default:
				return makeTextStream("ok"), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5

	runner := newTestRunner(model, persister, history, config)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Criar uma skill",
		SystemPrompt: "You are a test assistant.",
		TenantID:     "test-tenant",
	}))

	assert.False(t, hasEventType(events, agentic.EventError))
	assert.True(t, hasEventType(events, agentic.EventToolCallStart))
	assert.Equal(t, 2, model.CallCount())

	msgs := persister.Messages()
	require.GreaterOrEqual(t, len(msgs), 4)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, chat.MessageTypeToolUse, msgs[1].MessageType)
	assert.Equal(t, "tool", msgs[2].Role)
	assert.Equal(t, "assistant", msgs[3].Role)

	var toolCalls []ai.ToolCall
	require.NoError(t, json.Unmarshal(msgs[1].ToolCalls, &toolCalls))
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_ask_user_1", toolCalls[0].ID)
	assert.Equal(t, "ask_user", toolCalls[0].Function.Name)
	assert.JSONEq(t, `{"message":"Qual skill voce quer criar?","inputSchema":{"type":"object"}}`, toolCalls[0].Function.Arguments)
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

func TestRunner_LLMErrorRedactsSecretsBeforeSSE(t *testing.T) {
	const bearerSecret = "sk-ant-abcdefghijklmnopqrstuvwxyz123456"
	const cookieSecret = "session=very-sensitive-session-value"
	const basicSecret = "dXNlcjphLWZha2Utc2VjcmV0"
	const internalURL = "http://agenthub-provider:8080/v1/chat"

	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return nil, fmt.Errorf("provider rejected request\nAuthorization: Basic %s\nCookie: %s\nupstream: %s\nkey: %s", basicSecret, cookieSecret, internalURL, bearerSecret)
		},
	}

	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, agentic.DefaultRunConfig())
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	}))

	errEvent := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEvent.Data, &errData))
	assert.NotContains(t, errData.Message, bearerSecret)
	assert.NotContains(t, errData.Message, cookieSecret)
	assert.NotContains(t, errData.Message, basicSecret)
	assert.NotContains(t, errData.Message, internalURL)
	assert.Contains(t, errData.Message, "[REDACTED]")
	assert.Contains(t, errData.Message, "<upstream>")
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

func TestRunner_BudgetExceeded(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 5)
			go func() {
				defer close(ch)
				ch <- ai.StreamChunk{Delta: "text"}
				// Emit usage — 500K input + 500K output for claude-sonnet-4 = $1.5 + $7.5 = $9.0
				ch <- ai.StreamChunk{Usage: &ai.Usage{PromptTokens: 500_000}}
				ch <- ai.StreamChunk{Usage: &ai.Usage{CompletionTokens: 500_000}}
				ch <- ai.StreamChunk{FinishReason: "stop"}
			}()
			return ch, nil
		},
	}

	config := agentic.DefaultRunConfig()
	config.Model = "claude-sonnet-4"
	config.MaxBudgetUSD = 0.001 // Very low budget — $0.001
	config.RetryMaxAttempts = 1

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
	assert.Equal(t, "budget_exceeded", errData.Code)
}

func TestRunner_CostInRunComplete(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("Hello"), nil
		},
	}

	config := agentic.DefaultRunConfig()
	config.Model = "claude-sonnet-4"
	config.RetryMaxAttempts = 1

	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Hi",
		SystemPrompt: "",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)
	rc := findEvent(t, events, agentic.EventRunComplete)
	var runComplete agentic.RunCompleteData
	require.NoError(t, json.Unmarshal(rc.Data, &runComplete))
	// Cost should be >= 0 (may be 0 if usage is not propagated through mock stream).
	assert.GreaterOrEqual(t, runComplete.TotalCost, 0.0)
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

func TestRunner_PermissionDeny(t *testing.T) {
	// LLM calls a bound but denied tool. The runner should emit a permission
	// denial and must not execute the skill-runtime side effect.
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			if idx == 0 {
				return makeToolCallStream("tc_1", "execute-sql", `{"query":"DROP TABLE users"}`), nil
			}
			// After denied result, LLM should produce a text response.
			return makeTextStream("I cannot execute that query."), nil
		},
	}

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 2 * time.Second
	runner, executeRequests := newRunnerWithBoundSQLTool(t, model, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:   uuid.New(),
		AgentID:     uuid.New(),
		UserMessage: "drop users table",
		TenantID:    "test-tenant",
		PermissionRules: &agentic.PermissionRules{
			Deny: []string{"execute-sql"},
		},
	})

	events := collectEvents(ch)

	assert.Equal(t, int32(0), executeRequests.Load(), "denied tool must not call skill-runtime")
	assert.False(t, hasEventType(events, agentic.EventToolCallStart), "permission deny must stop before tool execution starts")

	deniedEvents := filterEvents(events, agentic.EventToolDenied)
	require.Len(t, deniedEvents, 1, "should emit one tool_denied event")
	var denied agentic.ToolDeniedData
	require.NoError(t, json.Unmarshal(deniedEvents[0].Data, &denied))
	assert.Equal(t, "tc_1", denied.ID)
	assert.Equal(t, "execute-sql", denied.Name)
	assert.Contains(t, denied.Reason, "not permitted")
	assert.Equal(t, 1, denied.DenialCount)

	resultEvents := filterEvents(events, agentic.EventToolResult)
	require.Len(t, resultEvents, 1, "denied tool still needs one tool_result for the LLM turn")
	var result agentic.ToolResultData
	require.NoError(t, json.Unmarshal(resultEvents[0].Data, &result))
	require.NotNil(t, result.Error)
	assert.Equal(t, "tc_1", result.ID)
	assert.Equal(t, "execute-sql", result.Name)
	assert.Contains(t, *result.Error, "not permitted")
	assert.NotContains(t, *result.Error, "not available")

	// Should still reach run_complete (LLM generates text after denial).
	types := make([]agentic.RunEventType, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}
	assert.Contains(t, types, agentic.EventRunComplete)
}

func TestRunner_PermissionDeny_HidesToolFromLLMAdvertisement(t *testing.T) {
	var advertisedToolNames []string
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			advertisedToolNames = make([]string, 0, len(opts.Tools))
			for _, tool := range opts.Tools {
				advertisedToolNames = append(advertisedToolNames, tool.Function.Name)
			}
			return makeTextStream("I will not attempt a denied tool."), nil
		},
	}

	config := agentic.DefaultRunConfig()
	runner, _ := newRunnerWithBoundSQLTool(t, model, config)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:   uuid.New(),
		AgentID:     uuid.New(),
		UserMessage: "hello",
		TenantID:    "test-tenant",
		PermissionRules: &agentic.PermissionRules{
			Deny: []string{"execute-sql"},
		},
	}))

	assert.NotContains(t, advertisedToolNames, "execute-sql",
		"a blanket-denied tool must not be advertised to the LLM")
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.False(t, hasEventType(events, agentic.EventError))
}

func TestRunner_PermissionConfirmBlocksWithoutApproval(t *testing.T) {
	for _, tt := range []struct {
		name             string
		elicitation      *testElicitationSubmitter
		expectedReason   string
		expectedDecision agentic.PermissionAuditDecision
	}{
		{
			name:             "automated mode escalates confirm to deny",
			expectedReason:   "requires confirmation but running in automated mode",
			expectedDecision: agentic.AuditDecisionConfirmEscalated,
		},
		{
			name: "user declines confirm prompt",
			elicitation: &testElicitationSubmitter{
				action: agentic.ElicitationDecline,
			},
			expectedReason:   "was not approved by the user",
			expectedDecision: agentic.AuditDecisionConfirmDenied,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			model := &mockChatModel{
				streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
					if idx == 0 {
						return makeToolCallStream("tc_1", "execute-sql", `{"query":"DROP TABLE users"}`), nil
					}
					return makeTextStream("The query was not approved."), nil
				},
			}

			config := agentic.DefaultRunConfig()
			config.ToolTimeout = 2 * time.Second
			runner, executeRequests := newRunnerWithBoundSQLTool(t, model, config)
			audit := agentic.NewInMemoryPermissionAuditStore()
			runID := uuid.New()
			var elicitation agentic.ElicitationSubmitter
			if tt.elicitation != nil {
				elicitation = tt.elicitation
			}

			events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
				RunID:       runID,
				SessionID:   uuid.New(),
				AgentID:     uuid.New(),
				UserMessage: "drop users table",
				TenantID:    "test-tenant",
				PermissionRules: &agentic.PermissionRules{
					Confirm: []string{"execute-sql"},
				},
				Elicitation:     elicitation,
				PermissionAudit: audit,
			}))

			assert.Equal(t, int32(0), executeRequests.Load(), "unapproved confirm tool must not call skill-runtime")
			assert.False(t, hasEventType(events, agentic.EventToolCallStart), "unapproved confirm must stop before tool execution starts")

			deniedEvents := filterEvents(events, agentic.EventToolDenied)
			require.Len(t, deniedEvents, 1, "unapproved confirm should emit one tool_denied event")
			var denied agentic.ToolDeniedData
			require.NoError(t, json.Unmarshal(deniedEvents[0].Data, &denied))
			assert.Equal(t, "tc_1", denied.ID)
			assert.Equal(t, "execute-sql", denied.Name)
			assert.Contains(t, denied.Reason, tt.expectedReason)

			resultEvents := filterEvents(events, agentic.EventToolResult)
			require.Len(t, resultEvents, 1, "unapproved confirm still needs one tool_result for the LLM turn")
			var result agentic.ToolResultData
			require.NoError(t, json.Unmarshal(resultEvents[0].Data, &result))
			require.NotNil(t, result.Error)
			assert.Equal(t, "execute-sql", result.Name)
			assert.Contains(t, *result.Error, tt.expectedReason)

			entries := audit.Snapshot()
			require.Len(t, entries, 1)
			assert.Equal(t, "execute-sql", entries[0].ToolName)
			assert.Equal(t, tt.expectedDecision, entries[0].Decision)
			assert.Equal(t, runID, *entries[0].RunID)
			assert.Contains(t, entries[0].InputSnippet, "DROP TABLE users")

			if tt.elicitation != nil {
				requestIDs, params := tt.elicitation.Requests()
				require.Len(t, requestIDs, 1)
				require.Len(t, params, 1)
				assert.Contains(t, requestIDs[0], "consent-execute-sql-")
				assert.Equal(t, agentic.ElicitationModeForm, params[0].Mode)
				assert.Contains(t, params[0].Message, "requires your approval")
				require.Len(t, params[0].Questions, 1)
				assert.Equal(t, "consent", params[0].Questions[0].ID)
				assert.Equal(t, "confirm", params[0].Questions[0].Type)
				assert.Contains(t, params[0].Questions[0].Question, "execute-sql")
				assert.Contains(t, params[0].Questions[0].Question, "DROP TABLE users")
			}
		})
	}
}

func TestRunner_PermissionConfirmAcceptExecutesBoundTool(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			if idx == 0 {
				return makeToolCallStream("tc_1", "execute-sql", `{"query":"DROP TABLE users"}`), nil
			}
			return makeTextStream("The approved query completed."), nil
		},
	}

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 2 * time.Second
	runner, executeRequests := newRunnerWithBoundSQLTool(t, model, config)
	elicitation := &testElicitationSubmitter{action: agentic.ElicitationAccept}
	audit := agentic.NewInMemoryPermissionAuditStore()
	runID := uuid.New()

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		RunID:       runID,
		SessionID:   uuid.New(),
		AgentID:     uuid.New(),
		UserMessage: "drop users table",
		TenantID:    "test-tenant",
		PermissionRules: &agentic.PermissionRules{
			Confirm: []string{"execute-sql"},
		},
		Elicitation:     elicitation,
		PermissionAudit: audit,
	}))

	assert.Equal(t, int32(1), executeRequests.Load(), "approved confirm tool should call skill-runtime exactly once")
	assert.True(t, hasEventType(events, agentic.EventToolCallStart), "approved confirm should reach tool execution")
	assert.False(t, hasEventType(events, agentic.EventToolDenied), "approved confirm must not emit tool_denied")

	resultEvents := filterEvents(events, agentic.EventToolResult)
	require.Len(t, resultEvents, 1)
	var result agentic.ToolResultData
	require.NoError(t, json.Unmarshal(resultEvents[0].Data, &result))
	require.Nil(t, result.Error)
	assert.Equal(t, "tc_1", result.ID)
	assert.Equal(t, "execute-sql", result.Name)
	assert.JSONEq(t, `{"executed":true}`, string(result.Output))

	entries := audit.Snapshot()
	require.Len(t, entries, 1)
	assert.Equal(t, "execute-sql", entries[0].ToolName)
	assert.Equal(t, agentic.AuditDecisionConfirmApproved, entries[0].Decision)
	assert.Equal(t, runID, *entries[0].RunID)
	assert.Contains(t, entries[0].InputSnippet, "DROP TABLE users")

	requestIDs, params := elicitation.Requests()
	require.Len(t, requestIDs, 1)
	require.Len(t, params, 1)
	assert.Contains(t, requestIDs[0], "consent-execute-sql-")
	assert.Equal(t, agentic.ElicitationModeForm, params[0].Mode)
	require.Len(t, params[0].Questions, 1)
	assert.Equal(t, "confirm", params[0].Questions[0].Type)

	types := make([]agentic.RunEventType, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}
	assert.Contains(t, types, agentic.EventRunComplete)
}

func TestRunner_ManagementToolCallOutsideAdvertisedScopeIsRejectedBeforeExecution(t *testing.T) {
	cases := []struct {
		name             string
		isAdmin          bool
		enableManagement bool
		currentDepth     int
	}{
		{name: "non-admin caller", isAdmin: false, enableManagement: true},
		{name: "agent opted out", isAdmin: true, enableManagement: false},
		{name: "sub-agent", isAdmin: true, enableManagement: true, currentDepth: 1},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			agentSvc := &stubAgentService{}
			management := agentic.NewManagementExecutor(agentSvc, nil, nil, nil, nil, nil)
			model := &mockChatModel{
				streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
					if idx == 0 {
						return makeToolCallStream("manage-blocked", "agenthub_manage", `{"operation":"create","resource":"agent","payload":{"name":"blocked"}}`), nil
					}
					return makeTextStream("management call rejected"), nil
				},
			}

			runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, agentic.DefaultRunConfig()).
				WithManagementExecutor(management)
			events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
				SessionID:        uuid.New(),
				AgentID:          uuid.New(),
				TenantID:         "test-tenant",
				IsAdmin:          tt.isAdmin,
				EnableManagement: tt.enableManagement,
				CurrentDepth:     tt.currentDepth,
			}))

			assert.False(t, agentSvc.createCalled, "management executor must not run outside advertised scope")
			toolResult := findEvent(t, events, agentic.EventToolResult)
			var result agentic.ToolResultData
			require.NoError(t, json.Unmarshal(toolResult.Data, &result))
			require.NotNil(t, result.Error)
			assert.Contains(t, *result.Error, "is not available for this agent")
			assert.Equal(t, 2, model.CallCount())
		})
	}
}

func TestRunner_PermissionAllowList(t *testing.T) {
	// Only document-search is allowed. execute-sql should be denied.
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			if idx == 0 {
				return makeToolCallStream("tc_1", "execute-sql", `{}`), nil
			}
			return makeTextStream("OK"), nil
		},
	}

	config := agentic.DefaultRunConfig()
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:   uuid.New(),
		AgentID:     uuid.New(),
		UserMessage: "test",
		TenantID:    "test-tenant",
		PermissionRules: &agentic.PermissionRules{
			Allow: []string{"document-search"},
		},
	})

	events := collectEvents(ch)

	// Tool result should have permission error.
	// "not available" fires when tool is not in the agent's bound set (allowedToolsIndex);
	// "not permitted" fires when blocked by PermissionRules allow list.
	var foundDenied bool
	for _, ev := range events {
		if ev.Type == agentic.EventToolResult {
			var data agentic.ToolResultData
			require.NoError(t, json.Unmarshal(ev.Data, &data))
			if data.Error != nil {
				errMsg := *data.Error
				isDenied := strings.Contains(errMsg, "not permitted") || strings.Contains(errMsg, "not available")
				assert.True(t, isDenied, "expected denial message, got: %s", errMsg)
				foundDenied = true
			}
		}
	}
	assert.True(t, foundDenied, "execute-sql should be denied by allow list")
}

func TestRunner_MaxTokensRecovery(t *testing.T) {
	// First call returns "length" (truncated), second call succeeds with "stop".
	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			if idx == 0 {
				// First call: truncated response.
				ch := make(chan ai.StreamChunk, 10)
				go func() {
					defer close(ch)
					ch <- ai.StreamChunk{Delta: "partial..."}
					ch <- ai.StreamChunk{FinishReason: "length"}
				}()
				return ch, nil
			}
			// Second call: verify max_tokens was increased and return success.
			assert.Greater(t, opts.MaxTokens, 4096, "max_tokens should have been increased")
			return makeTextStream("complete response"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 10
	config.MaxTokensPerCall = 4096

	runner := newTestRunner(model, persister, history, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Write something long",
		SystemPrompt: "You are helpful.",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	// Should recover: no error, has run_complete.
	assert.False(t, hasEventType(events, agentic.EventError))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))

	// Model should have been called twice (recovery retry).
	assert.Equal(t, 2, model.CallCount())
}

func TestRunner_MaxTokensRecoveryExhausted(t *testing.T) {
	// All calls return "length" — should error after 3 recovery attempts.
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 10)
			go func() {
				defer close(ch)
				ch <- ai.StreamChunk{Delta: "truncated"}
				ch <- ai.StreamChunk{FinishReason: "length"}
			}()
			return ch, nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 10
	config.MaxTokensPerCall = 4096

	runner := newTestRunner(model, persister, history, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Write something very long",
		SystemPrompt: "You are helpful.",
		TenantID:     "test-tenant",
	})

	events := collectEvents(ch)

	// Should error after exhausting recovery attempts.
	assert.True(t, hasEventType(events, agentic.EventError))

	// 1 initial + 3 recovery = 4 total calls.
	assert.Equal(t, 4, model.CallCount())
}

// --- SanitizeMessages tests ---

func TestSanitizeMessages_RemovesWhitespaceOnlyAssistant(t *testing.T) {
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "   \n  "},
		{Role: ai.RoleAssistant, Content: "real response"},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 2)
	assert.Equal(t, "hello", result[0].Content)
	assert.Equal(t, "real response", result[1].Content)
}

func TestSanitizeMessages_KeepsAssistantWithToolCalls(t *testing.T) {
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "search for docs"},
		{Role: ai.RoleAssistant, Content: "", ToolCalls: []ai.ToolCall{{ID: "tc1"}}},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 2) // whitespace content but has tool_calls → keep
}

func TestSanitizeMessages_RemovesDuplicateConsecutiveUser(t *testing.T) {
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "find me the report"},
		{Role: ai.RoleUser, Content: "find me the report"},
		{Role: ai.RoleAssistant, Content: "Here it is"},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 2)
	assert.Equal(t, ai.RoleUser, result[0].Role)
	assert.Equal(t, ai.RoleAssistant, result[1].Role)
}

func TestSanitizeMessages_CollapsesConsecutiveUserDifferentContent(t *testing.T) {
	// P-H1: orphaned user messages from failed LLM calls — keep only the last.
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "first question"},
		{Role: ai.RoleUser, Content: "second question"},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 1)
	assert.Equal(t, "second question", result[0].Content)
}

func TestSanitizeMessages_CollapsesManyOrphanedUserMessages(t *testing.T) {
	// Three consecutive user messages (two failed runs + current) → keep last.
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "Quem você é e qual seu papel?"},
		{Role: ai.RoleUser, Content: "Quem você é?"},
		{Role: ai.RoleUser, Content: "Me explique sua função"},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 1)
	assert.Equal(t, "Me explique sua função", result[0].Content)
}

func TestSanitizeMessages_OrphanedUserThenNewUser(t *testing.T) {
	// History has: user(orphan) → assistant → user(orphan) + new user appended after.
	// When new user is appended after loadHistory, the result is user+user at the end.
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "old question"},
		{Role: ai.RoleAssistant, Content: "answer"},
		{Role: ai.RoleUser, Content: "orphaned question"},
		{Role: ai.RoleUser, Content: "new question"},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 3)
	assert.Equal(t, "old question", result[0].Content)
	assert.Equal(t, "answer", result[1].Content)
	assert.Equal(t, "new question", result[2].Content)
}

func TestSanitizeMessages_EmptyInput(t *testing.T) {
	result := agentic.SanitizeMessages(nil)
	assert.Nil(t, result)
}

func TestSanitizeMessages_NoChangesNeeded(t *testing.T) {
	msgs := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "hi there"},
	}
	result := agentic.SanitizeMessages(msgs)
	require.Len(t, result, 2)
}

// --- Effort level tests ---

func TestModelSupportsEffort_Opus46(t *testing.T) {
	assert.True(t, agentic.ModelSupportsEffort("claude-opus-4-6-20250414"))
	assert.True(t, agentic.ModelSupportsEffort("claude-opus-4-6"))
}

func TestModelSupportsEffort_Sonnet46(t *testing.T) {
	assert.True(t, agentic.ModelSupportsEffort("claude-sonnet-4-6-20250514"))
}

func TestModelSupportsEffort_OlderModels(t *testing.T) {
	assert.False(t, agentic.ModelSupportsEffort("claude-sonnet-4-20250514"))
	assert.False(t, agentic.ModelSupportsEffort("claude-haiku-4-5-20251001"))
	assert.False(t, agentic.ModelSupportsEffort("gpt-4o"))
}

func TestModelSupportsMaxEffort_OnlyOpus46(t *testing.T) {
	assert.True(t, agentic.ModelSupportsMaxEffort("claude-opus-4-6-20250414"))
	assert.False(t, agentic.ModelSupportsMaxEffort("claude-sonnet-4-6-20250514"))
	assert.False(t, agentic.ModelSupportsMaxEffort("gpt-4o"))
}

func TestResolveEffortLevel_NilWhenNotSet(t *testing.T) {
	cfg := agentic.RunConfig{Model: "claude-opus-4-6-20250414"}
	assert.Nil(t, agentic.ResolveEffortLevel(cfg))
}

func TestResolveEffortLevel_NilForUnsupportedModel(t *testing.T) {
	medium := ai.EffortMedium
	cfg := agentic.RunConfig{Model: "gpt-4o", Effort: &medium}
	assert.Nil(t, agentic.ResolveEffortLevel(cfg))
}

func TestResolveEffortLevel_PassesThroughForSupportedModel(t *testing.T) {
	medium := ai.EffortMedium
	cfg := agentic.RunConfig{Model: "claude-opus-4-6-20250414", Effort: &medium}
	result := agentic.ResolveEffortLevel(cfg)
	require.NotNil(t, result)
	assert.Equal(t, ai.EffortMedium, *result)
}

func TestResolveEffortLevel_MaxDowngradedOnNonOpus(t *testing.T) {
	max := ai.EffortMax
	cfg := agentic.RunConfig{Model: "claude-sonnet-4-6-20250514", Effort: &max}
	result := agentic.ResolveEffortLevel(cfg)
	require.NotNil(t, result)
	assert.Equal(t, ai.EffortHigh, *result)
}

func TestResolveEffortLevel_MaxKeptOnOpus46(t *testing.T) {
	max := ai.EffortMax
	cfg := agentic.RunConfig{Model: "claude-opus-4-6-20250414", Effort: &max}
	result := agentic.ResolveEffortLevel(cfg)
	require.NotNil(t, result)
	assert.Equal(t, ai.EffortMax, *result)
}

// --- Empty tool result injection tests ---

func TestFormatToolResult_NormalOutput(t *testing.T) {
	r := agentic.ToolExecResult{Output: json.RawMessage(`{"key": "value"}`)}
	assert.Equal(t, `{"key": "value"}`, agentic.FormatToolResult(r))
}

func TestFormatToolResult_Error(t *testing.T) {
	errMsg := "connection refused"
	r := agentic.ToolExecResult{Error: &errMsg}
	result := agentic.FormatToolResult(r)
	// P-F3-1 (BUG-F3): error results include an anti-ask_user [SYSTEM] instruction.
	assert.Contains(t, result, "Error: connection refused")
	assert.Contains(t, result, "[SYSTEM]")
	assert.Contains(t, result, "do NOT call ask_user")
}

func TestFormatToolResult_EmptyOutputWithToolName(t *testing.T) {
	r := agentic.ToolExecResult{Output: json.RawMessage(`{}`), ToolName: "document_search"}
	assert.Equal(t, "(document_search completed with no output)", agentic.FormatToolResult(r))
}

func TestFormatToolResult_NullOutputWithToolName(t *testing.T) {
	r := agentic.ToolExecResult{Output: json.RawMessage(`null`), ToolName: "execute_sql"}
	assert.Equal(t, "(execute_sql completed with no output)", agentic.FormatToolResult(r))
}

func TestFormatToolResult_EmptyOutputWithoutToolName(t *testing.T) {
	r := agentic.ToolExecResult{Output: nil}
	assert.Equal(t, "(tool completed with no output)", agentic.FormatToolResult(r))
}

func TestFormatToolResult_WhitespaceOnlyOutput(t *testing.T) {
	r := agentic.ToolExecResult{Output: json.RawMessage(`   `), ToolName: "my_tool"}
	assert.Equal(t, "(my_tool completed with no output)", agentic.FormatToolResult(r))
}

// --- TR-01-TASK-13: LLM call timeout (P-C102-1) ---

// TestRunner_LLMCallTimeout_ReturnsError verifies that when the LLM hangs beyond
// the configured per-call timeout, the run terminates with an error event.
func TestRunner_LLMCallTimeout_ReturnsError(t *testing.T) {
	// LLM that blocks until its context is cancelled.
	blocked := make(chan struct{})
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 1)
			go func() {
				defer close(ch)
				select {
				case <-blocked: // test cleanup
				case <-time.After(10 * time.Second): // slower than the 50ms test timeout
				}
				ch <- ai.StreamChunk{Error: context.DeadlineExceeded}
			}()
			return ch, nil
		},
	}
	defer close(blocked)

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 50 * time.Millisecond // very short for tests
	config.RetryMaxAttempts = 1                   // no retries

	runner := newTestRunner(model, persister, history, config)

	start := time.Now()
	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hi",
		SystemPrompt: "test",
		TenantID:     "t",
	})
	events := collectEvents(ch)
	elapsed := time.Since(start)

	// Must complete well before the 10-second "slow" provider timeout.
	assert.Less(t, elapsed, 5*time.Second, "run should terminate before slow provider")

	// Should emit an error event (not a clean run_complete).
	errEvents := filterEvents(events, agentic.EventError)
	assert.NotEmpty(t, errEvents, "expected at least one error event")
}

// TestRunner_LLMCallFast_NoTimeout verifies that a fast LLM is not affected by the
// per-call timeout — the run completes normally.
func TestRunner_LLMCallFast_NoTimeout(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("hello"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 5 * time.Second // generous for a fast mock
	config.MaxIterations = 1

	runner := newTestRunner(model, persister, history, config)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hello",
		SystemPrompt: "test",
		TenantID:     "t",
	})
	events := collectEvents(ch)

	// Should have a clean text_delta + run_complete, no errors.
	assert.True(t, hasEventType(events, agentic.EventTextDelta))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.False(t, hasEventType(events, agentic.EventError), "fast LLM should not trigger timeout")
}

// TestRunner_LLMCallTimeout_ZeroDisablesTimeout verifies that LLMCallTimeout=0
// disables the per-call timeout (run continues until the parent context is cancelled).
func TestRunner_LLMCallTimeout_ZeroDisablesTimeout(t *testing.T) {
	callStarted := make(chan struct{})
	unblock := make(chan struct{})

	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			ch := make(chan ai.StreamChunk, 2)
			go func() {
				defer close(ch)
				close(callStarted)
				<-unblock
				ch <- ai.StreamChunk{Delta: "done", FinishReason: "stop"}
			}()
			return ch, nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 0 // disabled
	config.MaxIterations = 1

	runner := newTestRunner(model, persister, history, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := runner.Run(ctx, agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hi",
		SystemPrompt: "test",
		TenantID:     "t",
	})

	// Wait until the LLM call is in progress, then unblock it.
	select {
	case <-callStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("LLM call never started")
	}
	close(unblock)

	events := collectEvents(ch)
	assert.False(t, hasEventType(events, agentic.EventError), "zero timeout should not interrupt a completing LLM call")
}
