package agentic

import (
	"fmt"
	"strconv"
	"strings"
)

// Validation path formatting and tool validation error messages.
//
// Inspired by Claude Code's toolErrors.ts — formats validation
// paths (e.g., ["todos", 0, "activeForm"] → "todos[0].activeForm")
// and produces human-readable validation error messages for LLMs
// when tool input validation fails.

// ValidationIssueType classifies a validation issue.
type ValidationIssueType string

const (
	ValidationMissing       ValidationIssueType = "missing"
	ValidationUnexpected    ValidationIssueType = "unexpected"
	ValidationTypeMismatch  ValidationIssueType = "type_mismatch"
)

// ValidationIssue describes a single validation problem.
type ValidationIssue struct {
	Type     ValidationIssueType
	Path     string // formatted path (e.g., "todos[0].name")
	Expected string // expected type (for type mismatch)
	Received string // received type (for type mismatch)
}

// FormatValidationPath converts a path of property keys and array
// indices into a readable dot/bracket notation string.
// Example: ["todos", "0", "activeForm"] → "todos[0].activeForm"
func FormatValidationPath(segments []string) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder
	for i, seg := range segments {
		// Check if segment is a numeric index
		if _, err := strconv.Atoi(seg); err == nil {
			b.WriteString("[")
			b.WriteString(seg)
			b.WriteString("]")
		} else {
			if i > 0 {
				b.WriteString(".")
			}
			b.WriteString(seg)
		}
	}
	return b.String()
}

// FormatValidationError produces a human-readable, LLM-friendly
// error message from a list of validation issues for a given tool.
func FormatValidationError(toolName string, issues []ValidationIssue) string {
	if len(issues) == 0 {
		return ""
	}

	var parts []string
	for _, issue := range issues {
		switch issue.Type {
		case ValidationMissing:
			parts = append(parts, fmt.Sprintf("The required parameter `%s` is missing", issue.Path))
		case ValidationUnexpected:
			parts = append(parts, fmt.Sprintf("An unexpected parameter `%s` was provided", issue.Path))
		case ValidationTypeMismatch:
			parts = append(parts, fmt.Sprintf(
				"The parameter `%s` type is expected as `%s` but provided as `%s`",
				issue.Path, issue.Expected, issue.Received,
			))
		}
	}

	noun := "issue"
	if len(parts) > 1 {
		noun = "issues"
	}

	return fmt.Sprintf("%s failed due to the following %s:\n%s",
		toolName, noun, strings.Join(parts, "\n"))
}
