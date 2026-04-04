package agentic

import (
	"regexp"
	"strings"
)

// Code indexing tool detection via CLI commands and MCP server names.
//
// Inspired by Claude Code's codeIndexing.ts — detects usage of
// code search engines (Sourcegraph, Hound), AI coding assistants
// (Cody, Copilot, Cursor), and MCP code indexing servers from
// command strings and MCP tool names. Useful for analytics,
// capability detection, and context-aware suggestions.

// cliCommandMapping maps CLI command names to code indexing tools.
var cliCommandMapping = map[string]string{
	"src":    "sourcegraph",
	"cody":   "cody",
	"aider":  "aider",
	"tabby":  "tabby",
	"tabnine": "tabnine",
	"augment": "augment",
	"pieces": "pieces",
	"qodo":   "qodo",
	"aide":   "aide",
	"hound":  "hound",
	"seagoat": "seagoat",
	"bloop":  "bloop",
	"gitloop": "gitloop",
	"q":      "amazon-q",
	"gemini": "gemini",
}

type mcpServerPattern struct {
	pattern *regexp.Regexp
	tool    string
}

var mcpServerPatterns = []mcpServerPattern{
	{regexp.MustCompile(`(?i)^sourcegraph$`), "sourcegraph"},
	{regexp.MustCompile(`(?i)^cody$`), "cody"},
	{regexp.MustCompile(`(?i)^openctx$`), "openctx"},
	{regexp.MustCompile(`(?i)^aider$`), "aider"},
	{regexp.MustCompile(`(?i)^continue$`), "continue"},
	{regexp.MustCompile(`(?i)^github[-_]?copilot$`), "github-copilot"},
	{regexp.MustCompile(`(?i)^copilot$`), "github-copilot"},
	{regexp.MustCompile(`(?i)^cursor$`), "cursor"},
	{regexp.MustCompile(`(?i)^tabby$`), "tabby"},
	{regexp.MustCompile(`(?i)^codeium$`), "codeium"},
	{regexp.MustCompile(`(?i)^tabnine$`), "tabnine"},
	{regexp.MustCompile(`(?i)^augment[-_]?code$`), "augment"},
	{regexp.MustCompile(`(?i)^augment$`), "augment"},
	{regexp.MustCompile(`(?i)^windsurf$`), "windsurf"},
	{regexp.MustCompile(`(?i)^aide$`), "aide"},
	{regexp.MustCompile(`(?i)^codestory$`), "aide"},
	{regexp.MustCompile(`(?i)^pieces$`), "pieces"},
	{regexp.MustCompile(`(?i)^qodo$`), "qodo"},
	{regexp.MustCompile(`(?i)^amazon[-_]?q$`), "amazon-q"},
	{regexp.MustCompile(`(?i)^gemini[-_]?code[-_]?assist$`), "gemini"},
	{regexp.MustCompile(`(?i)^gemini$`), "gemini"},
	{regexp.MustCompile(`(?i)^hound$`), "hound"},
	{regexp.MustCompile(`(?i)^seagoat$`), "seagoat"},
	{regexp.MustCompile(`(?i)^bloop$`), "bloop"},
	{regexp.MustCompile(`(?i)^gitloop$`), "gitloop"},
	{regexp.MustCompile(`(?i)^claude[-_]?context$`), "claude-context"},
	{regexp.MustCompile(`(?i)^code[-_]?index[-_]?mcp$`), "code-index-mcp"},
	{regexp.MustCompile(`(?i)^code[-_]?index$`), "code-index-mcp"},
	{regexp.MustCompile(`(?i)^local[-_]?code[-_]?search$`), "local-code-search"},
	{regexp.MustCompile(`(?i)^codebase$`), "autodev-codebase"},
	{regexp.MustCompile(`(?i)^autodev[-_]?codebase$`), "autodev-codebase"},
	{regexp.MustCompile(`(?i)^code[-_]?context$`), "claude-context"},
}

// DetectCodeIndexingFromCommand detects if a bash command invokes a
// known code indexing CLI tool. Returns the tool name or empty string.
func DetectCodeIndexingFromCommand(command string) string {
	trimmed := strings.TrimSpace(command)
	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return ""
	}

	first := strings.ToLower(words[0])

	// Check npx/bunx prefixed commands.
	if (first == "npx" || first == "bunx") && len(words) > 1 {
		second := strings.ToLower(words[1])
		if tool, ok := cliCommandMapping[second]; ok {
			return tool
		}
	}

	if tool, ok := cliCommandMapping[first]; ok {
		return tool
	}
	return ""
}

// DetectCodeIndexingFromMCPTool detects if an MCP tool name
// (mcp__serverName__toolName) is from a code indexing server.
func DetectCodeIndexingFromMCPTool(toolName string) string {
	if !strings.HasPrefix(toolName, "mcp__") {
		return ""
	}
	parts := strings.SplitN(toolName, "__", 3)
	if len(parts) < 3 || parts[1] == "" {
		return ""
	}
	return DetectCodeIndexingFromMCPServerName(parts[1])
}

// DetectCodeIndexingFromMCPServerName detects if an MCP server name
// corresponds to a known code indexing tool.
func DetectCodeIndexingFromMCPServerName(serverName string) string {
	for _, p := range mcpServerPatterns {
		if p.pattern.MatchString(serverName) {
			return p.tool
		}
	}
	return ""
}
