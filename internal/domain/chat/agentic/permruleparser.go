package agentic

import "strings"

// Permission rule string parsing with escape handling.
//
// Inspired by Claude Code's permissionRuleParser.ts — parses
// "ToolName(content)" permission rule format with backslash-escaped
// parentheses, legacy tool name normalization, and roundtrip
// serialization. Useful for persisting and evaluating tool permission
// rules in agent configurations.

// PermissionRuleValue holds a parsed permission rule.
type PermissionRuleValue struct {
	ToolName    string
	RuleContent string // empty means tool-wide rule (no content filter)
}

// LegacyToolNameAliases maps old tool names to their canonical names.
// When a tool is renamed, add old → new here so permission rules,
// hooks, and persisted wire names resolve correctly.
var LegacyToolNameAliases = map[string]string{
	"Task":            "Agent",
	"KillShell":       "TaskStop",
	"AgentOutputTool": "TaskOutput",
	"BashOutputTool":  "TaskOutput",
}

// NormalizeLegacyToolName maps a potentially legacy tool name to its
// canonical name. Returns the input unchanged if not a known alias.
func NormalizeLegacyToolName(name string) string {
	if canonical, ok := LegacyToolNameAliases[name]; ok {
		return canonical
	}
	return name
}

// GetLegacyToolNames returns all legacy aliases that map to the given
// canonical name.
func GetLegacyToolNames(canonicalName string) []string {
	var result []string
	for legacy, canonical := range LegacyToolNameAliases {
		if canonical == canonicalName {
			result = append(result, legacy)
		}
	}
	return result
}

// EscapeRuleContent escapes special characters for safe storage in
// permission rules. Order matters: backslashes first, then parens.
func EscapeRuleContent(content string) string {
	s := strings.ReplaceAll(content, `\`, `\\`)
	s = strings.ReplaceAll(s, `(`, `\(`)
	s = strings.ReplaceAll(s, `)`, `\)`)
	return s
}

// UnescapeRuleContent reverses EscapeRuleContent. Order matters:
// parens first, then backslashes.
func UnescapeRuleContent(content string) string {
	s := strings.ReplaceAll(content, `\(`, `(`)
	s = strings.ReplaceAll(s, `\)`, `)`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// PermissionRuleFromString parses a permission rule string into its
// components. Handles escaped parentheses in the content portion.
//
// Format: "ToolName" or "ToolName(content)"
// Content may contain escaped parentheses: \( and \)
func PermissionRuleFromString(ruleString string) PermissionRuleValue {
	openIdx := findFirstUnescapedChar(ruleString, '(')
	if openIdx == -1 {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	closeIdx := findLastUnescapedChar(ruleString, ')')
	if closeIdx == -1 || closeIdx <= openIdx {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	// Closing paren must be at the end.
	if closeIdx != len(ruleString)-1 {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	toolName := ruleString[:openIdx]
	rawContent := ruleString[openIdx+1 : closeIdx]

	// Missing tool name (e.g., "(foo)") is malformed.
	if toolName == "" {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	// Empty content or standalone wildcard means tool-wide rule.
	if rawContent == "" || rawContent == "*" {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(toolName)}
	}

	return PermissionRuleValue{
		ToolName:    NormalizeLegacyToolName(toolName),
		RuleContent: UnescapeRuleContent(rawContent),
	}
}

// PermissionRuleToString converts a permission rule value to its
// string representation, escaping parentheses in the content.
func PermissionRuleToString(rule PermissionRuleValue) string {
	if rule.RuleContent == "" {
		return rule.ToolName
	}
	return rule.ToolName + "(" + EscapeRuleContent(rule.RuleContent) + ")"
}

// findFirstUnescapedChar finds the index of the first occurrence of
// char that is not preceded by an odd number of backslashes.
func findFirstUnescapedChar(s string, char byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == char {
			backslashes := 0
			for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				return i
			}
		}
	}
	return -1
}

// findLastUnescapedChar finds the index of the last occurrence of
// char that is not preceded by an odd number of backslashes.
func findLastUnescapedChar(s string, char byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == char {
			backslashes := 0
			for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				return i
			}
		}
	}
	return -1
}
