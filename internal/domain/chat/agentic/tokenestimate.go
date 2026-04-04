package agentic

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Token estimation constants.
//
// Inspired by Claude Code's tokenEstimation.ts.
const (
	// DefaultBytesPerToken is the default ratio for general text.
	DefaultBytesPerToken = 4
	// JSONBytesPerToken is the ratio for JSON content (denser, more single-char tokens).
	JSONBytesPerToken = 2
	// ImageEstimatedTokens is a conservative estimate for image/document blocks.
	ImageEstimatedTokens = 2000
)

// RoughTokenEstimate estimates token count from content length using a bytes-per-token ratio.
//
// Inspired by Claude Code's roughTokenCountEstimation.
func RoughTokenEstimate(content string, bytesPerToken int) int {
	if bytesPerToken <= 0 {
		bytesPerToken = DefaultBytesPerToken
	}
	charCount := utf8.RuneCountInString(content)
	if charCount == 0 {
		return 0
	}
	return (charCount + bytesPerToken - 1) / bytesPerToken
}

// BytesPerTokenForFileType returns the bytes-per-token ratio for a given file extension.
// JSON-like files use a denser ratio; everything else uses the default.
//
// Inspired by Claude Code's bytesPerTokenForFileType.
func BytesPerTokenForFileType(filename string) int {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json", ".jsonl", ".jsonc":
		return JSONBytesPerToken
	default:
		return DefaultBytesPerToken
	}
}

// EstimateFileTokens estimates the token count for a file's content.
func EstimateFileTokens(filename, content string) int {
	ratio := BytesPerTokenForFileType(filename)
	return RoughTokenEstimate(content, ratio)
}

// EstimateMessageTokens estimates tokens for a slice of chat messages.
// Each message's content is estimated, plus overhead for role/metadata.
func EstimateMessageTokens(messages json.RawMessage) int {
	if len(messages) == 0 {
		return 0
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(messages, &parsed); err != nil {
		// Fallback: raw character estimate.
		return RoughTokenEstimate(string(messages), DefaultBytesPerToken)
	}

	total := 0
	for _, msg := range parsed {
		total += estimateSingleMessageTokens(msg)
	}
	return total
}

// estimateSingleMessageTokens estimates tokens for one message.
func estimateSingleMessageTokens(msg map[string]interface{}) int {
	tokens := 4 // per-message overhead (role, formatting)

	// Content.
	if content, ok := msg["content"].(string); ok {
		tokens += RoughTokenEstimate(content, DefaultBytesPerToken)
	}

	// Tool calls: stringify and estimate.
	if toolCalls, ok := msg["toolCalls"]; ok && toolCalls != nil {
		raw, _ := json.Marshal(toolCalls)
		tokens += RoughTokenEstimate(string(raw), DefaultBytesPerToken)
	}

	// Thinking content.
	if meta, ok := msg["metadata"].(map[string]interface{}); ok {
		if thinking, ok := meta["thinkingContent"].(string); ok {
			tokens += RoughTokenEstimate(thinking, DefaultBytesPerToken)
		}
	}

	return tokens
}

// EstimateToolSchemaTokens estimates tokens for a tool's JSON schema definition.
// Includes name, description, and parameter schema.
func EstimateToolSchemaTokens(name, description string, inputSchema json.RawMessage) int {
	tokens := RoughTokenEstimate(name, DefaultBytesPerToken)
	tokens += RoughTokenEstimate(description, DefaultBytesPerToken)
	if len(inputSchema) > 0 {
		tokens += RoughTokenEstimate(string(inputSchema), JSONBytesPerToken)
	}
	return tokens
}

// EstimateSystemPromptTokens estimates tokens for a system prompt string.
func EstimateSystemPromptTokens(prompt string) int {
	return RoughTokenEstimate(prompt, DefaultBytesPerToken)
}

// EstimateSkillFrontmatterTokens estimates the token cost of a skill's metadata
// (name, description, when-to-use) without parsing the full content.
// Used for lazy-loading decisions.
//
// Inspired by Claude Code's estimateSkillFrontmatterTokens.
func EstimateSkillFrontmatterTokens(name, description, whenToUse string) int {
	parts := make([]string, 0, 3)
	if name != "" {
		parts = append(parts, name)
	}
	if description != "" {
		parts = append(parts, description)
	}
	if whenToUse != "" {
		parts = append(parts, whenToUse)
	}
	combined := strings.Join(parts, " ")
	return RoughTokenEstimate(combined, DefaultBytesPerToken)
}

// TokenBudget calculates remaining tokens and usage percentage.
type TokenBudget struct {
	// Used is the estimated tokens consumed.
	Used int `json:"used"`
	// Limit is the effective context window (after reserves).
	Limit int `json:"limit"`
	// Remaining is Limit - Used (floored at 0).
	Remaining int `json:"remaining"`
	// UsagePercent is Used/Limit as a percentage (0-100).
	UsagePercent int `json:"usagePercent"`
}

// CalculateTokenBudget computes the budget from usage and limit.
func CalculateTokenBudget(used, limit int) TokenBudget {
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	pct := 0
	if limit > 0 {
		pct = int(float64(used) / float64(limit) * 100)
		if pct > 100 {
			pct = 100
		}
	}
	return TokenBudget{
		Used:         used,
		Limit:        limit,
		Remaining:    remaining,
		UsagePercent: pct,
	}
}

// IsOverBudget returns true if usage exceeds the limit.
func (b TokenBudget) IsOverBudget() bool {
	return b.Used > b.Limit
}

// HasRoom returns true if at least minTokens remain.
func (b TokenBudget) HasRoom(minTokens int) bool {
	return b.Remaining >= minTokens
}
