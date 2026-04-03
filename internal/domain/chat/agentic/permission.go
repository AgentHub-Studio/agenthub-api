package agentic

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// PermissionDecision represents the outcome of evaluating a tool call against permission rules.
type PermissionDecision string

const (
	PermissionAllow   PermissionDecision = "allow"
	PermissionDeny    PermissionDecision = "deny"
	PermissionConfirm PermissionDecision = "confirm"
)

// PermissionRules defines the allow/deny/confirm patterns for tool usage.
// JSON shape: {"allow":["tool(pattern)"],"deny":["tool(pattern)"],"confirm":["tool(pattern)"]}
type PermissionRules struct {
	Allow   []string `json:"allow,omitempty"`
	Deny    []string `json:"deny,omitempty"`
	Confirm []string `json:"confirm,omitempty"`
}

// ParsePermissionRules parses raw JSON into PermissionRules.
// Returns nil if the input is empty or invalid.
func ParsePermissionRules(raw json.RawMessage) *PermissionRules {
	if len(raw) == 0 {
		return nil
	}
	var rules PermissionRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil
	}
	if len(rules.Allow) == 0 && len(rules.Deny) == 0 && len(rules.Confirm) == 0 {
		return nil
	}
	return &rules
}

// EvaluatePermission checks a tool call against the permission rules.
// Evaluation order: deny (checked first) > confirm > allow > default allow.
//
// Pattern syntax: "tool_name" or "tool_name(input_pattern)"
// - "execute-sql" matches any call to execute-sql
// - "execute-sql(SELECT *)" matches calls where input contains "SELECT *"
// - "*" matches all tools
// - "http-*" matches tools starting with "http-"
func EvaluatePermission(rules *PermissionRules, toolName string, toolInput string) PermissionDecision {
	if rules == nil {
		return PermissionAllow
	}

	// Deny rules checked first — highest priority.
	for _, pattern := range rules.Deny {
		if matchesPermissionPattern(pattern, toolName, toolInput) {
			return PermissionDeny
		}
	}

	// Confirm rules checked next.
	for _, pattern := range rules.Confirm {
		if matchesPermissionPattern(pattern, toolName, toolInput) {
			return PermissionConfirm
		}
	}

	// Allow rules — if any are defined, only matching tools are allowed.
	if len(rules.Allow) > 0 {
		for _, pattern := range rules.Allow {
			if matchesPermissionPattern(pattern, toolName, toolInput) {
				return PermissionAllow
			}
		}
		// If allow rules exist but none matched, deny.
		return PermissionDeny
	}

	// No allow rules defined = everything is allowed by default.
	return PermissionAllow
}

// matchesPermissionPattern checks if a tool call matches a permission pattern.
// Pattern: "tool_pattern" or "tool_pattern(input_pattern)"
func matchesPermissionPattern(pattern, toolName, toolInput string) bool {
	toolPattern, inputPattern := parsePattern(pattern)

	// Match tool name with glob.
	matched, err := filepath.Match(toolPattern, toolName)
	if err != nil || !matched {
		return false
	}

	// If no input pattern specified, tool name match is sufficient.
	if inputPattern == "" {
		return true
	}

	// Match input pattern as substring (case-insensitive).
	return strings.Contains(
		strings.ToLower(toolInput),
		strings.ToLower(inputPattern),
	)
}

// parsePattern splits "tool(input)" into ("tool", "input").
// Returns ("pattern", "") if no parentheses.
func parsePattern(pattern string) (string, string) {
	idx := strings.IndexByte(pattern, '(')
	if idx < 0 {
		return strings.TrimSpace(pattern), ""
	}
	toolPart := strings.TrimSpace(pattern[:idx])
	rest := pattern[idx+1:]
	// Find closing paren.
	end := strings.LastIndexByte(rest, ')')
	if end < 0 {
		return toolPart, strings.TrimSpace(rest)
	}
	return toolPart, strings.TrimSpace(rest[:end])
}

// FormatDeniedError creates a user-friendly error message for denied tool calls.
func FormatDeniedError(toolName string) string {
	return fmt.Sprintf("Tool '%s' is not permitted by the agent's permission rules.", toolName)
}
