package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestClassifyTool_BuiltinReadOnly(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectReadOnly, agentic.ClassifyTool("document_search", "", nil))
	assert.Equal(t, agentic.ToolEffectReadOnly, agentic.ClassifyTool("memory_recall", "", nil))
}

func TestClassifyTool_DefaultSideEffect(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("send-email", "", nil))
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("unknown-tool", "", nil))
}

func TestClassifyTool_ExplicitOverride(t *testing.T) {
	overrides := map[string]agentic.ToolEffect{
		"custom-tool": agentic.ToolEffectReadOnly,
	}
	assert.Equal(t, agentic.ToolEffectReadOnly, agentic.ClassifyTool("custom-tool", "", overrides))
}

func TestClassifyTool_OverrideTakesPriority(t *testing.T) {
	// Override a builtin read-only tool to side-effect.
	overrides := map[string]agentic.ToolEffect{
		"document_search": agentic.ToolEffectSideEffect,
	}
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("document_search", "", overrides))
}

func TestClassifyTool_HTTPGetReadOnly(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectReadOnly, agentic.ClassifyTool("http-get-users", "", nil))
	assert.Equal(t, agentic.ToolEffectReadOnly, agentic.ClassifyTool("http_get_status", "", nil))
	assert.Equal(t, agentic.ToolEffectReadOnly, agentic.ClassifyTool("HTTP-GET-data", "", nil))
}

func TestClassifyTool_HTTPPostSideEffect(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("http-post-users", "", nil))
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("http-put-data", "", nil))
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("http-delete-item", "", nil))
}

func TestClassifyTool_SQLSelect(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectReadOnly,
		agentic.ClassifyTool("execute-sql", `{"query":"SELECT * FROM users"}`, nil))
}

func TestClassifyTool_SQLInsert(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect,
		agentic.ClassifyTool("execute-sql", `{"query":"INSERT INTO users VALUES (1)"}`, nil))
}

func TestClassifyTool_SQLDelete(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect,
		agentic.ClassifyTool("execute-sql", `{"query":"DELETE FROM users WHERE id=1"}`, nil))
}

func TestClassifyTool_SQLDropTable(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect,
		agentic.ClassifyTool("execute-sql", `{"query":"DROP TABLE users"}`, nil))
}

func TestClassifyTool_SQLSelectWithSubInsert(t *testing.T) {
	// SELECT that contains INSERT keyword — treat as side-effect for safety.
	assert.Equal(t, agentic.ToolEffectSideEffect,
		agentic.ClassifyTool("execute-sql", `{"query":"SELECT * FROM insert_log"}`, nil))
}

func TestClassifyTool_SQLEmptyInput(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect,
		agentic.ClassifyTool("execute-sql", "", nil))
}

func TestClassifyTool_MCPToolsDefault(t *testing.T) {
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("mcp__github__create_issue", "", nil))
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("mcp__slack__send_message", "", nil))
}

func TestClassifyTool_NilOverrides(t *testing.T) {
	// Should not panic with nil overrides.
	assert.Equal(t, agentic.ToolEffectSideEffect, agentic.ClassifyTool("some-tool", "", nil))
}

func TestPartitionToolCalls_Mixed(t *testing.T) {
	calls := []ai.ToolCall{
		{ID: "1", Function: ai.ToolFunction{Name: "document_search", Arguments: `{"q":"test"}`}},
		{ID: "2", Function: ai.ToolFunction{Name: "send-email", Arguments: `{"to":"a@b.com"}`}},
		{ID: "3", Function: ai.ToolFunction{Name: "memory_recall", Arguments: `{"q":"history"}`}},
		{ID: "4", Function: ai.ToolFunction{Name: "execute-sql", Arguments: `{"query":"SELECT 1"}`}},
	}

	readOnly, sideEffect := agentic.SplitToolCallsByEffect(calls, nil)
	assert.Len(t, readOnly, 3)
	assert.Len(t, sideEffect, 1)
	assert.Equal(t, "send-email", sideEffect[0].Function.Name)
}

func TestPartitionToolCalls_AllReadOnly(t *testing.T) {
	calls := []ai.ToolCall{
		{ID: "1", Function: ai.ToolFunction{Name: "document_search"}},
		{ID: "2", Function: ai.ToolFunction{Name: "memory_recall"}},
	}

	readOnly, sideEffect := agentic.SplitToolCallsByEffect(calls, nil)
	assert.Len(t, readOnly, 2)
	assert.Empty(t, sideEffect)
}

func TestPartitionToolCalls_Empty(t *testing.T) {
	readOnly, sideEffect := agentic.SplitToolCallsByEffect(nil, nil)
	assert.Empty(t, readOnly)
	assert.Empty(t, sideEffect)
}

func TestPartitionToolCalls_WithOverrides(t *testing.T) {
	calls := []ai.ToolCall{
		{ID: "1", Function: ai.ToolFunction{Name: "custom-api"}},
	}
	overrides := map[string]agentic.ToolEffect{
		"custom-api": agentic.ToolEffectReadOnly,
	}

	readOnly, sideEffect := agentic.SplitToolCallsByEffect(calls, overrides)
	assert.Len(t, readOnly, 1)
	assert.Empty(t, sideEffect)
}
