package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func makeDeferredTools() []agentic.LLMTool {
	return []agentic.LLMTool{
		{Name: "agenthub_create_agent", Description: "Create a new agent", InputSchema: json.RawMessage(`{"type":"object"}`), SearchHint: "create new agent assistant bot"},
		{Name: "agenthub_update_agent", Description: "Update an existing agent", InputSchema: json.RawMessage(`{"type":"object"}`), SearchHint: "update modify agent"},
		{Name: "agenthub_delete_agent", Description: "Delete an agent", InputSchema: json.RawMessage(`{"type":"object"}`), SearchHint: "delete remove agent"},
		{Name: "agenthub_create_skill", Description: "Create a new skill", InputSchema: json.RawMessage(`{"type":"object"}`), SearchHint: "create new skill capability"},
		{Name: "agenthub_sync_knowledge_base", Description: "Sync a knowledge base", InputSchema: json.RawMessage(`{"type":"object"}`), SearchHint: "sync reindex knowledge base documents"},
	}
}

func TestIsToolSearchCall(t *testing.T) {
	assert.True(t, agentic.IsToolSearchCall("tool_search"))
	assert.False(t, agentic.IsToolSearchCall("document_search"))
	assert.False(t, agentic.IsToolSearchCall(""))
}

func TestExecuteToolSearch_ExactName(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{"query": "agenthub_create_agent"}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.Nil(t, result.Error)
	assert.Equal(t, "tool_search", result.ToolName)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &resp))
	tools := resp["tools"].([]any)
	assert.Len(t, tools, 1)
	assert.Equal(t, "agenthub_create_agent", tools[0].(map[string]any)["name"])
}

func TestExecuteToolSearch_SelectMultiple(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{"query": "select:agenthub_create_agent,agenthub_delete_agent"}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.Nil(t, result.Error)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &resp))
	tools := resp["tools"].([]any)
	assert.Len(t, tools, 2)
}

func TestExecuteToolSearch_KeywordMatch(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{"query": "create agent"}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.Nil(t, result.Error)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &resp))
	tools := resp["tools"].([]any)
	assert.GreaterOrEqual(t, len(tools), 1)
	// First result should be agenthub_create_agent (highest score for "create agent").
	assert.Equal(t, "agenthub_create_agent", tools[0].(map[string]any)["name"])
}

func TestExecuteToolSearch_NoMatch(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{"query": "nonexistent_xyz"}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.Nil(t, result.Error)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &resp))
	tools := resp["tools"].([]any)
	assert.Len(t, tools, 0)
	assert.Contains(t, resp["message"], "No deferred tools found")
}

func TestExecuteToolSearch_MaxResults(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{"query": "agent", "max_results": 2}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.Nil(t, result.Error)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &resp))
	tools := resp["tools"].([]any)
	assert.LessOrEqual(t, len(tools), 2)
}

func TestExecuteToolSearch_MissingQuery(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "query")
}

func TestExecuteToolSearch_InvalidJSON(t *testing.T) {
	deferred := makeDeferredTools()
	input := json.RawMessage(`{invalid}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "Invalid input")
}

func TestExecuteToolSearch_SearchHintBoost(t *testing.T) {
	deferred := makeDeferredTools()
	// "sync reindex" matches only the sync tool's search_hint.
	input := json.RawMessage(`{"query": "sync reindex"}`)

	result := agentic.ExecuteToolSearch(input, deferred)

	assert.Nil(t, result.Error)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &resp))
	tools := resp["tools"].([]any)
	assert.Len(t, tools, 1)
	assert.Equal(t, "agenthub_sync_knowledge_base", tools[0].(map[string]any)["name"])
}
