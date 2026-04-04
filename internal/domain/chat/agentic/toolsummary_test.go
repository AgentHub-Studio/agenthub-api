package agentic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

type capturingSummaryChatModel struct {
	response string
	calls    int
	lastOpts ai.ChatOptions
}

func (m *capturingSummaryChatModel) Chat(_ context.Context, _ []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	m.calls++
	m.lastOpts = opts
	return &ai.ChatResponse{Content: m.response, FinishReason: "stop"}, nil
}

func (m *capturingSummaryChatModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (m *capturingSummaryChatModel) GetProviderName() string { return "mock" }

type promptTemplateResolverStub struct {
	templates map[string]string
}

func (s *promptTemplateResolverStub) ResolvePromptTemplate(_ context.Context, _ uuid.UUID, slug string) (string, bool, error) {
	content, ok := s.templates[slug]
	return content, ok, nil
}

func TestToolUseSummaryGenerator_Generate(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Searched auth module"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	tools := []agentic.ToolSummaryInfo{
		{
			Name:   "document_search",
			Input:  json.RawMessage(`{"query":"auth middleware"}`),
			Output: json.RawMessage(`{"results":[{"title":"Auth.go"}]}`),
		},
	}

	summary := gen.Generate(context.Background(), uuid.New(), tools, "Let me search for auth info")
	assert.Equal(t, "Searched auth module", summary)
	assert.Equal(t, 1, model.calls)
}

func TestToolUseSummaryGenerator_StripsLeadingDash(t *testing.T) {
	model := &capturingSummaryChatModel{response: "- Read config files"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	tools := []agentic.ToolSummaryInfo{
		{Name: "read_file", Input: json.RawMessage(`{"path":"config.json"}`)},
	}

	summary := gen.Generate(context.Background(), uuid.New(), tools, "")
	assert.Equal(t, "Read config files", summary)
}

func TestToolUseSummaryGenerator_NilModel(t *testing.T) {
	gen := agentic.NewToolUseSummaryGenerator(nil, "test-model")
	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "test"},
	}, "")
	assert.Empty(t, summary)
}

func TestToolUseSummaryGenerator_NilGenerator(t *testing.T) {
	var gen *agentic.ToolUseSummaryGenerator
	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "test"},
	}, "")
	assert.Empty(t, summary)
}

func TestToolUseSummaryGenerator_EmptyTools(t *testing.T) {
	model := &capturingSummaryChatModel{response: "something"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	summary := gen.Generate(context.Background(), uuid.New(), nil, "")
	assert.Empty(t, summary)
	assert.Equal(t, 0, model.calls, "should not call model for empty tools")
}

func TestToolUseSummaryGenerator_WithError(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Failed SQL query"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	errMsg := "connection refused"
	tools := []agentic.ToolSummaryInfo{
		{
			Name:  "execute_sql",
			Input: json.RawMessage(`{"query":"SELECT 1"}`),
			Error: &errMsg,
		},
	}

	summary := gen.Generate(context.Background(), uuid.New(), tools, "")
	assert.Equal(t, "Failed SQL query", summary)
}

func TestToolUseSummaryGenerator_UsesPromptTemplateOverride(t *testing.T) {
	model := &capturingSummaryChatModel{response: "Summarized tool batch"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model").
		WithPromptTemplateResolver(&promptTemplateResolverStub{
			templates: map[string]string{
				"agentic-tool-use-summary-system-prompt": "Custom summary prompt",
			},
		})

	summary := gen.Generate(context.Background(), uuid.New(), []agentic.ToolSummaryInfo{
		{Name: "document_search", Input: json.RawMessage(`{"query":"billing"}`)},
	}, "")

	assert.Equal(t, "Summarized tool batch", summary)
	assert.Equal(t, "Custom summary prompt", model.lastOpts.SystemMsg)
}

func TestToolUseSummaryData_JSON(t *testing.T) {
	evt := agentic.NewRunEvent(agentic.EventToolUseSummary, agentic.ToolUseSummaryData{
		TurnIndex: 3,
		Summary:   "Searched docs",
	})
	assert.Equal(t, agentic.EventToolUseSummary, evt.Type)

	var data agentic.ToolUseSummaryData
	assert.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, 3, data.TurnIndex)
	assert.Equal(t, "Searched docs", data.Summary)
}
