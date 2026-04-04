package agentic

import (
	"regexp"
	"strings"
)

// Frontmatter parsing, brace expansion, and YAML-safe quoting.
//
// Inspired by Claude Code's frontmatterParser.ts — extracts YAML
// frontmatter from markdown, splits comma-separated patterns while
// respecting brace nesting, expands {a,b}/{c,d} into Cartesian
// products, and quotes YAML-special characters in values. Useful
// for skill/command configuration files and glob pattern lists.

var frontmatterRegex = regexp.MustCompile(`(?s)^---\s*\n(.*?)---\s*\n?`)

// yamlSpecialChars detects characters that require quoting in YAML.
var yamlSpecialChars = regexp.MustCompile(`[{}\[\]*&#!|>%@` + "`" + `]|: `)

// ParsedFrontmatter holds extracted frontmatter text and body content.
type ParsedFrontmatter struct {
	RawYAML string            // the raw YAML text between --- delimiters
	Fields  map[string]string // simple key-value pairs (flat)
	Content string            // remaining markdown after frontmatter
}

// ExtractFrontmatter splits markdown into frontmatter (raw YAML text)
// and content. It does NOT parse YAML — callers should use a YAML
// library for complex structures. For simple key: value lines, the
// Fields map is populated.
func ExtractFrontmatter(markdown string) ParsedFrontmatter {
	match := frontmatterRegex.FindStringSubmatch(markdown)
	if match == nil {
		return ParsedFrontmatter{Content: markdown, Fields: map[string]string{}}
	}

	rawYAML := match[1]
	content := markdown[len(match[0]):]
	fields := parseSimpleYAML(rawYAML)

	return ParsedFrontmatter{
		RawYAML: rawYAML,
		Fields:  fields,
		Content: content,
	}
}

// parseSimpleYAML parses flat key: value YAML lines (no nesting).
func parseSimpleYAML(yaml string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		fields[key] = val
	}
	return fields
}

// QuoteYAMLSpecialValues pre-processes frontmatter text to quote
// values containing special YAML characters. This allows glob
// patterns like **/*.{ts,tsx} to be parsed correctly.
func QuoteYAMLSpecialValues(frontmatterText string) string {
	kvPattern := regexp.MustCompile(`^([a-zA-Z_-]+):\s+(.+)$`)
	lines := strings.Split(frontmatterText, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		m := kvPattern.FindStringSubmatch(line)
		if m != nil {
			key, value := m[1], m[2]
			// Skip already-quoted values.
			if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
				(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
				result = append(result, line)
				continue
			}
			if yamlSpecialChars.MatchString(value) {
				escaped := strings.ReplaceAll(value, `\`, `\\`)
				escaped = strings.ReplaceAll(escaped, `"`, `\"`)
				result = append(result, key+`: "`+escaped+`"`)
				continue
			}
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// SplitPathInFrontmatter splits a comma-separated string and expands
// brace patterns. Commas inside braces are not treated as separators.
//
// Examples:
//
//	"a, b"                → ["a", "b"]
//	"a, src/*.{ts,tsx}"   → ["a", "src/*.ts", "src/*.tsx"]
//	"{a,b}/{c,d}"         → ["a/c", "a/d", "b/c", "b/d"]
func SplitPathInFrontmatter(input string) []string {
	var parts []string
	var current strings.Builder
	braceDepth := 0

	for _, ch := range input {
		switch ch {
		case '{':
			braceDepth++
			current.WriteRune(ch)
		case '}':
			braceDepth--
			current.WriteRune(ch)
		case ',':
			if braceDepth == 0 {
				if trimmed := strings.TrimSpace(current.String()); trimmed != "" {
					parts = append(parts, trimmed)
				}
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if trimmed := strings.TrimSpace(current.String()); trimmed != "" {
		parts = append(parts, trimmed)
	}

	var result []string
	for _, p := range parts {
		result = append(result, ExpandBraces(p)...)
	}
	return result
}

// ExpandBraces expands brace patterns in a glob string.
//
// Examples:
//
//	"src/*.{ts,tsx}"  → ["src/*.ts", "src/*.tsx"]
//	"{a,b}/{c,d}"     → ["a/c", "a/d", "b/c", "b/d"]
func ExpandBraces(pattern string) []string {
	braceMatch := regexp.MustCompile(`^([^{]*)\{([^}]+)\}(.*)$`)
	m := braceMatch.FindStringSubmatch(pattern)
	if m == nil {
		return []string{pattern}
	}

	prefix := m[1]
	alternatives := m[2]
	suffix := m[3]

	parts := strings.Split(alternatives, ",")
	var expanded []string
	for _, part := range parts {
		combined := prefix + strings.TrimSpace(part) + suffix
		expanded = append(expanded, ExpandBraces(combined)...)
	}
	return expanded
}

// ParseBooleanFrontmatter returns true only for literal "true".
func ParseBooleanFrontmatter(value string) bool {
	return value == "true"
}

// ParsePositiveIntFrontmatter parses a positive integer from a
// frontmatter value string. Returns 0 if invalid or not positive.
func ParsePositiveIntFrontmatter(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return 0
	}
	return n
}
