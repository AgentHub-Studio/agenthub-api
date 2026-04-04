package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestTokenEstimateConstants(t *testing.T) {
	assert.Equal(t, 4, agentic.DefaultBytesPerToken)
	assert.Equal(t, 2, agentic.JSONBytesPerToken)
	assert.Equal(t, 2000, agentic.ImageEstimatedTokens)
}

// --- RoughTokenEstimate ---

func TestRoughTokenEstimate_Empty(t *testing.T) {
	assert.Equal(t, 0, agentic.RoughTokenEstimate("", 4))
}

func TestRoughTokenEstimate_Default(t *testing.T) {
	// 100 chars / 4 = 25 tokens
	content := make([]byte, 100)
	for i := range content {
		content[i] = 'a'
	}
	assert.Equal(t, 25, agentic.RoughTokenEstimate(string(content), 4))
}

func TestRoughTokenEstimate_JSON(t *testing.T) {
	// 100 chars / 2 = 50 tokens
	content := make([]byte, 100)
	for i := range content {
		content[i] = '{'
	}
	assert.Equal(t, 50, agentic.RoughTokenEstimate(string(content), 2))
}

func TestRoughTokenEstimate_ZeroRatio(t *testing.T) {
	result := agentic.RoughTokenEstimate("hello", 0)
	assert.Equal(t, 2, result, "should fall back to default ratio 4")
}

func TestRoughTokenEstimate_RoundsUp(t *testing.T) {
	// 5 chars / 4 = 1.25 → rounds up to 2
	assert.Equal(t, 2, agentic.RoughTokenEstimate("hello", 4))
}

// --- BytesPerTokenForFileType ---

func TestBytesPerTokenForFileType_JSON(t *testing.T) {
	assert.Equal(t, 2, agentic.BytesPerTokenForFileType("data.json"))
	assert.Equal(t, 2, agentic.BytesPerTokenForFileType("log.jsonl"))
	assert.Equal(t, 2, agentic.BytesPerTokenForFileType("config.jsonc"))
}

func TestBytesPerTokenForFileType_Default(t *testing.T) {
	assert.Equal(t, 4, agentic.BytesPerTokenForFileType("main.go"))
	assert.Equal(t, 4, agentic.BytesPerTokenForFileType("readme.md"))
	assert.Equal(t, 4, agentic.BytesPerTokenForFileType("style.css"))
	assert.Equal(t, 4, agentic.BytesPerTokenForFileType("noext"))
}

func TestBytesPerTokenForFileType_CaseInsensitive(t *testing.T) {
	assert.Equal(t, 2, agentic.BytesPerTokenForFileType("Data.JSON"))
}

// --- EstimateFileTokens ---

func TestEstimateFileTokens_Go(t *testing.T) {
	content := "package main\n\nfunc main() {}\n" // 30 chars
	tokens := agentic.EstimateFileTokens("main.go", content)
	assert.Equal(t, 8, tokens) // 30/4 = 7.5 → 8
}

func TestEstimateFileTokens_JSON(t *testing.T) {
	content := `{"key": "value"}` // 16 chars
	tokens := agentic.EstimateFileTokens("data.json", content)
	assert.Equal(t, 8, tokens) // 16/2 = 8
}

// --- EstimateMessageTokens ---

func TestEstimateMessageTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, agentic.EstimateMessageTokens(nil))
}

func TestEstimateMessageTokens_SingleMessage(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello world"}, // 11 chars / 4 = 3 + 4 overhead = 7
	}
	raw, _ := json.Marshal(msgs)
	tokens := agentic.EstimateMessageTokens(raw)
	assert.Greater(t, tokens, 0)
}

func TestEstimateMessageTokens_MultipleMessages(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi there, how can I help you today?"},
	}
	raw, _ := json.Marshal(msgs)
	tokens := agentic.EstimateMessageTokens(raw)
	assert.Greater(t, tokens, 5, "should account for both messages")
}

func TestEstimateMessageTokens_WithToolCalls(t *testing.T) {
	msgs := []map[string]interface{}{
		{
			"role":      "assistant",
			"content":   "Let me search",
			"toolCalls": []interface{}{map[string]interface{}{"name": "search", "input": map[string]interface{}{"q": "test"}}},
		},
	}
	raw, _ := json.Marshal(msgs)
	tokens := agentic.EstimateMessageTokens(raw)
	assert.Greater(t, tokens, 10, "should include tool call tokens")
}

func TestEstimateMessageTokens_InvalidJSON(t *testing.T) {
	tokens := agentic.EstimateMessageTokens(json.RawMessage(`not json at all`))
	assert.Greater(t, tokens, 0, "should fall back to raw estimation")
}

// --- EstimateToolSchemaTokens ---

func TestEstimateToolSchemaTokens(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)
	tokens := agentic.EstimateToolSchemaTokens("document_search", "Search documents by query", schema)
	assert.Greater(t, tokens, 10)
}

func TestEstimateToolSchemaTokens_NoSchema(t *testing.T) {
	tokens := agentic.EstimateToolSchemaTokens("simple_tool", "A simple tool", nil)
	assert.Greater(t, tokens, 0)
}

// --- EstimateSystemPromptTokens ---

func TestEstimateSystemPromptTokens(t *testing.T) {
	prompt := "You are a helpful assistant that helps with code."
	tokens := agentic.EstimateSystemPromptTokens(prompt)
	assert.Greater(t, tokens, 5)
}

// --- EstimateSkillFrontmatterTokens ---

func TestEstimateSkillFrontmatterTokens(t *testing.T) {
	tokens := agentic.EstimateSkillFrontmatterTokens(
		"document_search",
		"Search documents in knowledge base",
		"Use when user asks about documents",
	)
	assert.Greater(t, tokens, 10)
}

func TestEstimateSkillFrontmatterTokens_Partial(t *testing.T) {
	tokens := agentic.EstimateSkillFrontmatterTokens("search", "", "")
	assert.Greater(t, tokens, 0)
}

func TestEstimateSkillFrontmatterTokens_Empty(t *testing.T) {
	tokens := agentic.EstimateSkillFrontmatterTokens("", "", "")
	assert.Equal(t, 0, tokens)
}

// --- TokenBudget ---

func TestCalculateTokenBudget_Normal(t *testing.T) {
	budget := agentic.CalculateTokenBudget(50_000, 200_000)
	assert.Equal(t, 50_000, budget.Used)
	assert.Equal(t, 200_000, budget.Limit)
	assert.Equal(t, 150_000, budget.Remaining)
	assert.Equal(t, 25, budget.UsagePercent)
	assert.False(t, budget.IsOverBudget())
	assert.True(t, budget.HasRoom(10_000))
}

func TestCalculateTokenBudget_Full(t *testing.T) {
	budget := agentic.CalculateTokenBudget(200_000, 200_000)
	assert.Equal(t, 0, budget.Remaining)
	assert.Equal(t, 100, budget.UsagePercent)
	assert.False(t, budget.IsOverBudget())
	assert.False(t, budget.HasRoom(1))
}

func TestCalculateTokenBudget_Over(t *testing.T) {
	budget := agentic.CalculateTokenBudget(250_000, 200_000)
	assert.Equal(t, 0, budget.Remaining)
	assert.Equal(t, 100, budget.UsagePercent) // capped at 100
	assert.True(t, budget.IsOverBudget())
}

func TestCalculateTokenBudget_Zero(t *testing.T) {
	budget := agentic.CalculateTokenBudget(0, 0)
	assert.Equal(t, 0, budget.UsagePercent)
	assert.False(t, budget.IsOverBudget())
}

func TestTokenBudget_HasRoom(t *testing.T) {
	budget := agentic.CalculateTokenBudget(180_000, 200_000)
	assert.True(t, budget.HasRoom(20_000))
	assert.False(t, budget.HasRoom(20_001))
}
