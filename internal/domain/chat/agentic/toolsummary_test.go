package agentic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestToolUseSummaryGenerator_Generate(t *testing.T) {
	model := &simpleSummaryChatModel{response: "Searched auth module"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	tools := []agentic.ToolSummaryInfo{
		{
			Name:   "document_search",
			Input:  json.RawMessage(`{"query":"auth middleware"}`),
			Output: json.RawMessage(`{"results":[{"title":"Auth.go"}]}`),
		},
	}

	summary := gen.Generate(context.Background(), tools, "Let me search for auth info")
	assert.Equal(t, "Searched auth module", summary)
	assert.Equal(t, 1, model.calls)
}

func TestToolUseSummaryGenerator_StripsLeadingDash(t *testing.T) {
	model := &simpleSummaryChatModel{response: "- Read config files"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	tools := []agentic.ToolSummaryInfo{
		{Name: "read_file", Input: json.RawMessage(`{"path":"config.json"}`)},
	}

	summary := gen.Generate(context.Background(), tools, "")
	assert.Equal(t, "Read config files", summary)
}

func TestToolUseSummaryGenerator_NilModel(t *testing.T) {
	gen := agentic.NewToolUseSummaryGenerator(nil, "test-model")
	summary := gen.Generate(context.Background(), []agentic.ToolSummaryInfo{
		{Name: "test"},
	}, "")
	assert.Empty(t, summary)
}

func TestToolUseSummaryGenerator_NilGenerator(t *testing.T) {
	var gen *agentic.ToolUseSummaryGenerator
	summary := gen.Generate(context.Background(), []agentic.ToolSummaryInfo{
		{Name: "test"},
	}, "")
	assert.Empty(t, summary)
}

func TestToolUseSummaryGenerator_EmptyTools(t *testing.T) {
	model := &simpleSummaryChatModel{response: "something"}
	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	summary := gen.Generate(context.Background(), nil, "")
	assert.Empty(t, summary)
	assert.Equal(t, 0, model.calls, "should not call model for empty tools")
}

func TestToolUseSummaryGenerator_WithError(t *testing.T) {
	model := &simpleSummaryChatModel{response: "Failed SQL query"}

	gen := agentic.NewToolUseSummaryGenerator(model, "test-model")

	errMsg := "connection refused"
	tools := []agentic.ToolSummaryInfo{
		{
			Name:  "execute_sql",
			Input: json.RawMessage(`{"query":"SELECT 1"}`),
			Error: &errMsg,
		},
	}

	summary := gen.Generate(context.Background(), tools, "")
	assert.Equal(t, "Failed SQL query", summary)
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
