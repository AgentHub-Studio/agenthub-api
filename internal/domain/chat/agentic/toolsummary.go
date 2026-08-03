package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// ToolUseSummaryGenerator generates human-readable summaries of completed tool
// batches. Uses a small/fast model (e.g. Haiku) to produce a short label describing
// what tools accomplished — similar to a git commit subject line.
//
// Inspired by Claude Code's toolUseSummary/toolUseSummaryGenerator.ts.
type ToolUseSummaryGenerator struct {
	chatModel ai.ChatModel
	model     string
	tpl       PromptTemplateResolver
}

// ToolSummaryInfo describes a single tool execution for summary generation.
type ToolSummaryInfo struct {
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  *string         `json:"error,omitempty"`
}

const toolUseSummarySystemPrompt = `Write a short summary label describing what these tool calls accomplished. It appears as a single-line row in a mobile app and truncates around 30 characters, so think git-commit-subject, not sentence.

Keep the verb in past tense and the most distinctive noun. Drop articles, connectors, and long location context first.

Examples:
- Searched in auth/
- Fixed NPE in UserService
- Created signup endpoint
- Read config.json
- Ran failing tests`

// NewToolUseSummaryGenerator creates a ToolUseSummaryGenerator.
// If chatModel is nil, Generate() returns empty string (non-fatal).
func NewToolUseSummaryGenerator(chatModel ai.ChatModel, model string) *ToolUseSummaryGenerator {
	return &ToolUseSummaryGenerator{
		chatModel: chatModel,
		model:     model,
	}
}

// WithPromptTemplateResolver attaches an optional resolver for the summary prompt.
func (g *ToolUseSummaryGenerator) WithPromptTemplateResolver(resolver PromptTemplateResolver) *ToolUseSummaryGenerator {
	g.tpl = resolver
	return g
}

// Generate produces a brief summary of the given tool executions.
// Returns empty string on error (non-fatal — summaries are cosmetic).
func (g *ToolUseSummaryGenerator) Generate(ctx context.Context, agentID uuid.UUID, tools []ToolSummaryInfo, lastAssistantText string) string {
	if g == nil || g.chatModel == nil || len(tools) == 0 {
		return ""
	}

	// Build concise representation of what tools did.
	var sb strings.Builder
	if lastAssistantText != "" {
		fmt.Fprintf(&sb, "User's intent (from assistant's last message): %s\n\n",
			truncateToolSummaryString(lastAssistantText, 200))
	}

	sb.WriteString("Tools completed:\n\n")
	for _, t := range tools {
		fmt.Fprintf(&sb, "Tool: %s\n", t.Name)
		fmt.Fprintf(&sb, "Input: %s\n", truncateJSON(t.Input, 300))
		if t.Error != nil {
			fmt.Fprintf(&sb, "Error: %s\n", truncateToolSummaryString(*t.Error, 200))
		} else {
			fmt.Fprintf(&sb, "Output: %s\n", truncateJSON(t.Output, 300))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Label:")

	resp, err := g.chatModel.Chat(ctx, []ai.Message{
		{Role: ai.RoleUser, Content: sb.String()},
	}, ai.ChatOptions{
		Model:     g.model,
		MaxTokens: 50,
		SystemMsg: resolvePromptTemplateOrFallback(
			ctx,
			g.tpl,
			agentID,
			promptTemplateSlugToolUseSummarySystemPrompt,
			toolUseSummarySystemPrompt,
		),
	})
	if err != nil {
		return ""
	}

	summary := strings.TrimSpace(resp.Content)
	// Strip leading "- " if present (the model sometimes mirrors the example format).
	summary = strings.TrimPrefix(summary, "- ")
	return summary
}

// truncateJSON truncates a JSON raw message to maxLen characters.
func truncateJSON(raw json.RawMessage, maxLen int) string {
	if len(raw) == 0 {
		return "{}"
	}
	s := string(raw)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func truncateToolSummaryString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
