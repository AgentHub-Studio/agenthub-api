package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
)

func TestRunner_ExecutesMultipleDistinctToolsInSequence(t *testing.T) {
	var mu sync.Mutex
	var requestSlugs []string
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := skillSlugFromExecutePath(r.URL.Path)
		if slug == "" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		requestSlugs = append(requestSlugs, slug)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"output": map[string]any{
				"tool": slug,
				"ok":   true,
			},
			"latencyMs": 1,
		}))
	}))
	defer runtime.Close()

	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_sql", "execute-sql", `{"query":"SELECT 1"}`), nil
			case 1:
				toolMessages := messagesByRole(messages, ai.RoleTool)
				require.Len(t, toolMessages, 1)
				assert.Contains(t, toolMessages[0].Content, `"tool":"execute-sql"`)
				return makeToolCallStream("tc_http", "http-call", `{"url":"https://example.test"}`), nil
			default:
				toolMessages := messagesByRole(messages, ai.RoleTool)
				require.GreaterOrEqual(t, len(toolMessages), 2)
				assert.Contains(t, toolMessages[len(toolMessages)-2].Content, `"tool":"execute-sql"`)
				assert.Contains(t, toolMessages[len(toolMessages)-1].Content, `"tool":"http-call"`)
				return makeTextStream("completed both tools"), nil
			}
		},
	}

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 4
	config.ToolTimeout = 5 * time.Second
	persister := &mockPersister{}
	runner := newRunnerWithRuntime(model, runtime.URL, twoSkillCatalog(), config, persister)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "run both tools",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	assert.False(t, hasEventType(events, agentic.EventError))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.Equal(t, 3, model.CallCount())

	toolEvents := filterEvents(events, agentic.EventToolResult)
	require.Len(t, toolEvents, 2)
	var first, second agentic.ToolResultData
	require.NoError(t, json.Unmarshal(toolEvents[0].Data, &first))
	require.NoError(t, json.Unmarshal(toolEvents[1].Data, &second))
	assert.Equal(t, "execute-sql", first.Name)
	assert.Equal(t, "http-call", second.Name)
	assert.Nil(t, first.Error)
	assert.Nil(t, second.Error)

	mu.Lock()
	seen := append([]string(nil), requestSlugs...)
	mu.Unlock()
	assert.Equal(t, []string{"execute-sql", "http-call"}, seen)

	var persistedToolResults int
	for _, msg := range persister.Messages() {
		if msg.MessageType == chat.MessageTypeToolResult {
			persistedToolResults++
		}
	}
	assert.Equal(t, 2, persistedToolResults)
}

func TestRunner_SkillRuntimeHTTPFailureReturnsToolErrorAndContinues(t *testing.T) {
	var mu sync.Mutex
	runtimeCalls := 0
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "execute-sql", skillSlugFromExecutePath(r.URL.Path))
		assert.Equal(t, http.MethodPost, r.Method)

		mu.Lock()
		runtimeCalls++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"error": "runtime unavailable"}))
	}))
	defer runtime.Close()

	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_runtime_failure", "execute-sql", `{"query":"SELECT 1"}`), nil
			default:
				toolMessages := messagesByRole(messages, ai.RoleTool)
				require.Len(t, toolMessages, 1)
				assert.Contains(t, toolMessages[0].Content, "runtime unavailable")
				return makeTextStream("I could not execute the tool, so I continued safely."), nil
			}
		},
	}

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	config.ToolTimeout = 5 * time.Second
	runner := newRunnerWithRuntime(model, runtime.URL, twoSkillCatalog(), config, &mockPersister{})
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "run the tool",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	assert.False(t, hasEventType(events, agentic.EventError))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.Equal(t, 2, model.CallCount())

	toolEvents := filterEvents(events, agentic.EventToolResult)
	require.Len(t, toolEvents, 1)
	var result agentic.ToolResultData
	require.NoError(t, json.Unmarshal(toolEvents[0].Data, &result))
	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "runtime unavailable")

	mu.Lock()
	gotRuntimeCalls := runtimeCalls
	mu.Unlock()
	assert.Equal(t, 1, gotRuntimeCalls)
}

func TestRunner_InvalidToolInputReturnsStructuredToolError(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_bad", "agent", `{"task":"missing prompt"}`), nil
			default:
				toolMessages := messagesByRole(messages, ai.RoleTool)
				require.NotEmpty(t, toolMessages)
				assert.Contains(t, toolMessages[len(toolMessages)-1].Content, "prompt")
				return makeTextStream("input error handled"), nil
			}
		},
	}

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	config.ToolTimeout = 5 * time.Second
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "delegate",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	assert.False(t, hasEventType(events, agentic.EventError))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.Equal(t, 2, model.CallCount())

	toolEvents := filterEvents(events, agentic.EventToolResult)
	require.Len(t, toolEvents, 1)
	var result agentic.ToolResultData
	require.NoError(t, json.Unmarshal(toolEvents[0].Data, &result))
	require.NotNil(t, result.Error)
	assert.Equal(t, "agent", result.Name)
	assert.Contains(t, *result.Error, "prompt")
}

func TestRunner_ProviderRateLimitEmitsLLMErrorAndRunComplete(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return nil, errors.New("429 rate limit exceeded")
		},
	}

	persister := &mockPersister{}
	config := agentic.DefaultRunConfig()
	config.RetryMaxAttempts = 1
	runner := newTestRunner(model, persister, &mockHistoryLoader{}, config)
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hello",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	errEvent := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEvent.Data, &errData))
	assert.Equal(t, "llm_call", errData.Code)
	assert.Contains(t, errData.Message, "429")
	assert.Contains(t, errData.Message, "rate limit")
	assert.True(t, hasEventType(events, agentic.EventRunComplete))

	msgs := persister.Messages()
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[len(msgs)-1].Content, "rate limit")
}

func TestRunner_OpenAICompatibleProviderUnavailableEmitsFriendlyErrorAndRunComplete(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		var request struct {
			Stream bool `json:"stream"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.True(t, request.Stream)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "upstream maintenance window"},
		}))
	}))
	defer provider.Close()

	persister := &mockPersister{}
	config := agentic.DefaultRunConfig()
	config.RetryMaxAttempts = 1
	runner := newTestRunner(openai.New("test-key", provider.URL), persister, &mockHistoryLoader{}, config)
	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hello",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	errEvent := findEvent(t, events, agentic.EventError)
	var errData agentic.ErrorData
	require.NoError(t, json.Unmarshal(errEvent.Data, &errData))
	assert.Equal(t, "llm_call", errData.Code)
	assert.Contains(t, errData.Message, "503")
	assert.True(t, hasEventType(events, agentic.EventRunComplete))

	msgs := persister.Messages()
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[len(msgs)-1].Content, "indisponível")
	assert.NotContains(t, msgs[len(msgs)-1].Content, "upstream maintenance window")
}

func TestRunner_MCPFailureReturnsToolErrorAndContinues(t *testing.T) {
	toolName := agentic.FormatMCPToolName("github", "list_issues")
	mcpClient := &failingMCPClient{}
	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_mcp", toolName, `{"repo":"agenthub"}`), nil
			default:
				toolMessages := messagesByRole(messages, ai.RoleTool)
				require.NotEmpty(t, toolMessages)
				assert.Contains(t, toolMessages[len(toolMessages)-1].Content, "mcp github unavailable")
				return makeTextStream("mcp failure handled"), nil
			}
		},
	}

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	config.ToolTimeout = 5 * time.Second
	runner := newTestRunner(model, &mockPersister{}, &mockHistoryLoader{}, config).
		WithMCPClient(mcpClient)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "list issues",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	assert.False(t, hasEventType(events, agentic.EventError))
	assert.True(t, hasEventType(events, agentic.EventRunComplete))
	assert.Equal(t, 1, mcpClient.callCount())

	toolEvents := filterEvents(events, agentic.EventToolResult)
	require.Len(t, toolEvents, 1)
	var result agentic.ToolResultData
	require.NoError(t, json.Unmarshal(toolEvents[0].Data, &result))
	require.NotNil(t, result.Error)
	assert.Equal(t, toolName, result.Name)
	assert.Contains(t, *result.Error, "mcp github unavailable")
}

func newRunnerWithRuntime(model ai.ChatModel, runtimeURL string, catalog *testSkillCatalog, config agentic.RunConfig, persister agentic.MessagePersister) *agentic.Runner {
	kbs := &mockKBLister{}
	if persister == nil {
		persister = &mockPersister{}
	}
	return agentic.NewRunner(
		model,
		agentic.NewSkillRuntimeClient(runtimeURL),
		agentic.NewPromptBuilder(catalog.skills, kbs, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(catalog.skills, catalog.tools, kbs),
		nil,
		nil,
		persister,
		&mockHistoryLoader{},
		nil,
		config,
	)
}

type testSkillCatalog struct {
	skills *mockSkillLister
	tools  *mockToolsBySkill
}

func twoSkillCatalog() *testSkillCatalog {
	sqlSkillID := uuid.New()
	httpSkillID := uuid.New()
	sqlToolID := uuid.New()
	httpToolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{
		{ID: sqlSkillID, Name: "Execute SQL", Slug: "execute-sql"},
		{ID: httpSkillID, Name: "HTTP Call", Slug: "http-call"},
	}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[sqlSkillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: sqlSkillID, ToolID: sqlToolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     sqlToolID,
			Name:   "SQL Tool",
			Type:   "SQL",
			Config: json.RawMessage(`{"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}`),
		}},
	}
	toolsMock.bySkill[httpSkillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: httpSkillID, ToolID: httpToolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     httpToolID,
			Name:   "HTTP Tool",
			Type:   "HTTP",
			Config: json.RawMessage(`{"inputSchema":{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}}`),
		}},
	}
	return &testSkillCatalog{skills: skills, tools: toolsMock}
}

func skillSlugFromExecutePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "skills" || parts[3] != "execute" {
		return ""
	}
	return parts[2]
}

func messagesByRole(messages []ai.Message, role string) []ai.Message {
	var out []ai.Message
	for _, msg := range messages {
		if string(msg.Role) == role {
			out = append(out, msg)
		}
	}
	return out
}

type failingMCPClient struct {
	mu        sync.Mutex
	callTotal int
}

func (f *failingMCPClient) ListTools(context.Context, string) ([]agentic.MCPToolInfo, error) {
	return []agentic.MCPToolInfo{{
		ServerName:  "github",
		Name:        "list_issues",
		Description: "List GitHub issues",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"repo":{"type":"string"}},"required":["repo"]}`),
	}}, nil
}

func (f *failingMCPClient) CallTool(context.Context, string, string, string, json.RawMessage) (json.RawMessage, error) {
	f.mu.Lock()
	f.callTotal++
	f.mu.Unlock()
	return nil, errors.New("mcp github unavailable")
}

func (f *failingMCPClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callTotal
}
