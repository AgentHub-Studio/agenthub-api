package agentic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestRunner_ToolResultRedactsSensitiveFieldsInHistoryAndSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"ok": true,
				"auth_token": "short-token",
				"nested": {"apiKey": "api-key", "password": "pw", "safe": "kept"},
				"items": [{"secret": "secret-value", "name": "kept-name"}]
			},
			"latencyMs": 1
		}`))
	}))
	defer srv.Close()

	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_secret", "execute-sql", `{"query":"SELECT 1"}`), nil
			default:
				require.NotEmpty(t, messages)
				last := messages[len(messages)-1]
				assert.Equal(t, ai.RoleTool, last.Role)
				assert.Contains(t, last.Content, "kept")
				assert.NotContains(t, last.Content, "auth_token")
				assert.NotContains(t, last.Content, "short-token")
				assert.NotContains(t, last.Content, "apiKey")
				assert.NotContains(t, last.Content, "api-key")
				assert.NotContains(t, last.Content, "password")
				assert.NotContains(t, last.Content, "secret-value")
				return makeTextStream("done"), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	config.ToolTimeout = 5 * time.Second

	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{{
		ID:   skillID,
		Name: "Execute SQL",
		Slug: "execute-sql",
	}}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:     toolID,
			Name:   "SQL Tool",
			Type:   "SQL",
			Config: json.RawMessage(`{"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}`),
		}},
	}
	kbs := &mockKBLister{}
	runner := agentic.NewRunner(
		model,
		agentic.NewSkillRuntimeClient(srv.URL),
		agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, toolsMock, kbs),
		nil,
		nil,
		persister,
		history,
		nil,
		config,
	)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "run the tool",
		SystemPrompt: "test",
		TenantID:     "tenant",
	}))

	toolEvent := findEvent(t, events, agentic.EventToolResult)
	var eventData agentic.ToolResultData
	require.NoError(t, json.Unmarshal(toolEvent.Data, &eventData))
	assert.Contains(t, string(eventData.Output), "kept")
	assert.NotContains(t, string(eventData.Output), "auth_token")
	assert.NotContains(t, string(eventData.Output), "short-token")
	assert.NotContains(t, string(eventData.Output), "apiKey")
	assert.NotContains(t, string(eventData.Output), "api-key")
	assert.NotContains(t, string(eventData.Output), "password")
	assert.NotContains(t, string(eventData.Output), "secret-value")

	var persistedToolResult string
	for _, msg := range persister.Messages() {
		if msg.MessageType == chat.MessageTypeToolResult {
			persistedToolResult = msg.Content
			break
		}
	}
	require.NotEmpty(t, persistedToolResult)
	assert.Contains(t, persistedToolResult, "kept")
	assert.NotContains(t, persistedToolResult, "auth_token")
	assert.NotContains(t, persistedToolResult, "short-token")
	assert.NotContains(t, persistedToolResult, "apiKey")
	assert.NotContains(t, persistedToolResult, "api-key")
	assert.NotContains(t, persistedToolResult, "password")
	assert.NotContains(t, persistedToolResult, "secret-value")
}

func TestRunner_ToolResultRedactsSensitiveErrorInHistoryAndSSE(t *testing.T) {
	const secret = "runner-tool-error-secret"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	errMsg := "Authorization: Bearer " + secret + "\nupstream=" + internalURL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":` + strconv.Quote(errMsg) + `,"latencyMs":1}`))
	}))
	defer srv.Close()

	model := &mockChatModel{
		streamFn: func(idx int, messages []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_error", "execute-sql", `{"query":"SELECT 1"}`), nil
			default:
				require.NotEmpty(t, messages)
				last := messages[len(messages)-1]
				assert.Equal(t, ai.RoleTool, last.Role)
				assert.NotContains(t, last.Content, secret)
				assert.NotContains(t, last.Content, internalURL)
				assert.Contains(t, last.Content, "[REDACTED]")
				assert.Contains(t, last.Content, "<upstream>")
				return makeTextStream("done"), nil
			}
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	config := agentic.DefaultRunConfig()
	config.MaxIterations = 3
	config.ToolTimeout = 5 * time.Second

	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{{
		ID: skillID, Name: "Execute SQL", Slug: "execute-sql",
	}}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID: toolID, Name: "SQL Tool", Type: "SQL",
			Config: json.RawMessage(`{"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}`),
		}},
	}
	runner := agentic.NewRunner(
		model,
		agentic.NewSkillRuntimeClient(srv.URL),
		agentic.NewPromptBuilder(skills, &mockKBLister{}, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}),
		nil, nil, persister, history, nil, config,
	)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), UserMessage: "run the tool",
		SystemPrompt: "test", TenantID: "tenant",
	}))

	toolEvent := findEvent(t, events, agentic.EventToolResult)
	var eventData agentic.ToolResultData
	require.NoError(t, json.Unmarshal(toolEvent.Data, &eventData))
	require.NotNil(t, eventData.Error)
	assert.NotContains(t, *eventData.Error, secret)
	assert.NotContains(t, *eventData.Error, internalURL)
	assert.Contains(t, *eventData.Error, "[REDACTED]")
	assert.Contains(t, *eventData.Error, "<upstream>")

	var persistedToolResult string
	for _, msg := range persister.Messages() {
		if msg.MessageType == chat.MessageTypeToolResult {
			persistedToolResult = msg.Content
			break
		}
	}
	require.NotEmpty(t, persistedToolResult)
	assert.NotContains(t, persistedToolResult, secret)
	assert.NotContains(t, persistedToolResult, internalURL)
	assert.Contains(t, persistedToolResult, "[REDACTED]")
	assert.Contains(t, persistedToolResult, "<upstream>")
}
