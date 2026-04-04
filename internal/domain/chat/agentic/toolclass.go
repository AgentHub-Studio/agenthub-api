package agentic

import (
	"strings"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// ToolEffect classifies a tool's side-effect behavior.
type ToolEffect string

const (
	// ToolEffectReadOnly indicates the tool has no side effects and can be
	// executed concurrently with other read-only tools.
	ToolEffectReadOnly ToolEffect = "read_only"

	// ToolEffectSideEffect indicates the tool may modify state and should
	// be executed serially to avoid conflicts.
	ToolEffectSideEffect ToolEffect = "side_effect"
)

// builtinReadOnlyTools are tools that are known to be read-only.
var builtinReadOnlyTools = map[string]bool{
	"document_search": true,
	"memory_recall":   true,
}

// httpReadOnlyPrefixes are HTTP tool name patterns that indicate GET requests.
var httpReadOnlyPrefixes = []string{
	"http-get",
	"http_get",
}

// ClassifyTool determines the effect classification of a tool call.
// Priority: explicit override > builtin rules > heuristics > default (side_effect).
func ClassifyTool(toolName string, toolInput string, overrides map[string]ToolEffect) ToolEffect {
	// 1. Explicit override (from skill metadata or config).
	if overrides != nil {
		if eff, ok := overrides[toolName]; ok {
			return eff
		}
	}

	// 2. Builtin read-only tools.
	if builtinReadOnlyTools[toolName] {
		return ToolEffectReadOnly
	}

	// 3. HTTP tools with GET method are read-only.
	lowerName := strings.ToLower(toolName)
	for _, prefix := range httpReadOnlyPrefixes {
		if strings.HasPrefix(lowerName, prefix) {
			return ToolEffectReadOnly
		}
	}

	// 4. SQL tools with SELECT-only queries are read-only.
	if isSQLTool(lowerName) && isSelectOnly(toolInput) {
		return ToolEffectReadOnly
	}

	// 5. MCP tools default to side-effect (unknown external behavior).
	// 6. Default: assume side-effect for safety.
	return ToolEffectSideEffect
}

// PartitionToolCalls splits tool calls into read-only and side-effect groups.
func PartitionToolCalls(toolCalls []ai.ToolCall, overrides map[string]ToolEffect) (readOnly, sideEffect []ai.ToolCall) {
	for _, tc := range toolCalls {
		if ClassifyTool(tc.Function.Name, tc.Function.Arguments, overrides) == ToolEffectReadOnly {
			readOnly = append(readOnly, tc)
		} else {
			sideEffect = append(sideEffect, tc)
		}
	}
	return readOnly, sideEffect
}

func isSQLTool(name string) bool {
	return strings.Contains(name, "sql") || strings.Contains(name, "query") || name == "execute-sql"
}

func isSelectOnly(input string) bool {
	if input == "" {
		return false
	}
	lower := strings.ToLower(input)
	// Must contain SELECT and must NOT contain any write keywords.
	if !strings.Contains(lower, "select") {
		return false
	}
	writeKeywords := []string{"insert", "update", "delete", "drop", "alter", "create", "truncate", "merge"}
	for _, kw := range writeKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}
