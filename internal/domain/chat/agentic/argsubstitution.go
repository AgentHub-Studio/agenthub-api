package agentic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Argument substitution for skill/command prompt templates.
//
// Inspired by Claude Code's argumentSubstitution.ts — substitutes
// $ARGUMENTS placeholders in skill prompts with actual values.
//
// Supports:
//   - $ARGUMENTS — replaced with the full arguments string
//   - $ARGUMENTS[0], $ARGUMENTS[1], etc. — individual indexed arguments
//   - $0, $1, etc. — shorthand for indexed arguments
//   - Named arguments ($foo, $bar) — mapped to indexed positions

var (
	indexedArgRe   = regexp.MustCompile(`\$ARGUMENTS\[(\d+)\]`)
	shorthandArgRe = regexp.MustCompile(`\$(\d+)(?:\b|$)`)
)

// ParseArguments splits an arguments string into individual arguments.
// Handles simple quoting (double and single quotes). Falls back to
// whitespace splitting if quoting is unbalanced.
func ParseArguments(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}

	var result []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(args); i++ {
		ch := args[i]
		switch {
		case ch == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote
		case ch == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote
		case (ch == ' ' || ch == '\t') && !inSingleQuote && !inDoubleQuote:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// ParseArgumentNames parses argument name definitions from a
// space-separated string or slice. Filters out empty and numeric-only
// names (which would conflict with $0, $1 shorthand).
func ParseArgumentNames(names string) []string {
	if names == "" {
		return nil
	}
	parts := strings.Fields(names)
	var result []string
	for _, p := range parts {
		if p != "" && !isNumericOnly(p) {
			result = append(result, p)
		}
	}
	return result
}

// GenerateArgumentHint returns a hint string showing remaining
// unfilled arguments (e.g., "[arg2] [arg3]"). Returns empty string
// if all arguments are filled.
func GenerateArgumentHint(argNames []string, typedArgs []string) string {
	if len(typedArgs) >= len(argNames) {
		return ""
	}
	remaining := argNames[len(typedArgs):]
	if len(remaining) == 0 {
		return ""
	}
	parts := make([]string, len(remaining))
	for i, name := range remaining {
		parts[i] = "[" + name + "]"
	}
	return strings.Join(parts, " ")
}

// SubstituteArguments replaces argument placeholders in content with
// actual values from the args string.
//
// If appendIfNoPlaceholder is true and no placeholders were found in
// the content, the arguments are appended as "ARGUMENTS: {args}".
func SubstituteArguments(content, args string, appendIfNoPlaceholder bool, argumentNames []string) string {
	if args == "" && !strings.Contains(content, "$ARGUMENTS") {
		return content
	}

	parsedArgs := ParseArguments(args)
	original := content

	// Replace named arguments ($foo, $bar) mapped to positions
	for i, name := range argumentNames {
		if name == "" {
			continue
		}
		pattern := regexp.MustCompile(`\$` + regexp.QuoteMeta(name) + `(?:[^[\w]|$)`)
		content = pattern.ReplaceAllStringFunc(content, func(match string) string {
			value := ""
			if i < len(parsedArgs) {
				value = parsedArgs[i]
			}
			// Preserve trailing character that isn't part of the name
			suffix := ""
			trimmed := strings.TrimPrefix(match, "$"+name)
			if trimmed != "" {
				suffix = trimmed
			}
			return value + suffix
		})
	}

	// Replace indexed arguments: $ARGUMENTS[0], $ARGUMENTS[1], ...
	content = indexedArgRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := indexedArgRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx >= len(parsedArgs) {
			return ""
		}
		return parsedArgs[idx]
	})

	// Replace shorthand indexed arguments: $0, $1, ...
	content = shorthandArgRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := shorthandArgRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx >= len(parsedArgs) {
			return ""
		}
		return parsedArgs[idx]
	})

	// Replace $ARGUMENTS with the full arguments string
	content = strings.ReplaceAll(content, "$ARGUMENTS", args)

	// Append if no placeholders were found
	if content == original && appendIfNoPlaceholder && args != "" {
		content = content + fmt.Sprintf("\n\nARGUMENTS: %s", args)
	}

	return content
}

// isNumericOnly returns true if the string contains only digits.
func isNumericOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
