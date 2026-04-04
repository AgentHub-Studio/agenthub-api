package agentic

import (
	"regexp"
	"strings"
)

// Shell permission rule matching utilities.
//
// Inspired by Claude Code's shellRuleMatching.ts — parses
// permission rules (exact, prefix, wildcard) and matches
// commands against them. Supports escape sequences (\*, \\)
// and legacy prefix syntax (cmd:*).

// ShellRuleType discriminates the kind of permission rule.
type ShellRuleType string

const (
	ShellRuleExact    ShellRuleType = "exact"
	ShellRulePrefix   ShellRuleType = "prefix"
	ShellRuleWildcard ShellRuleType = "wildcard"
)

// ShellPermissionRule is a parsed permission rule.
type ShellPermissionRule struct {
	Type    ShellRuleType
	Command string // populated for exact
	Prefix  string // populated for prefix
	Pattern string // populated for wildcard
}

// legacyPrefixRe matches the legacy :* suffix (e.g. "npm:*" → prefix "npm").
var legacyPrefixRe = regexp.MustCompile(`^(.+):\*$`)

// PermissionRuleExtractPrefix extracts the prefix from a legacy :* rule.
// Returns the prefix and true, or empty string and false if not a legacy rule.
func PermissionRuleExtractPrefix(rule string) (string, bool) {
	m := legacyPrefixRe.FindStringSubmatch(rule)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// HasWildcards checks if a pattern contains unescaped wildcards
// (not the legacy :* suffix syntax). An asterisk is unescaped
// if preceded by an even number of backslashes (including zero).
func HasWildcards(pattern string) bool {
	if strings.HasSuffix(pattern, ":*") {
		return false
	}
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			backslashes := 0
			j := i - 1
			for j >= 0 && pattern[j] == '\\' {
				backslashes++
				j--
			}
			if backslashes%2 == 0 {
				return true
			}
		}
	}
	return false
}

// regexMetaChars are the characters to escape when building
// a regex from a wildcard pattern (everything except *).
var regexMetaChars = regexp.MustCompile(`[.+?^${}()|[\]\\'"` + "`]")

// MatchWildcardPattern matches a command against a wildcard pattern.
// Unescaped * matches any sequence of characters (including newlines).
// Use \* for a literal asterisk and \\ for a literal backslash.
// When the pattern has a single trailing " *", the space+args become
// optional so "git *" matches both "git add" and bare "git".
func MatchWildcardPattern(pattern, command string, caseInsensitive bool) bool {
	trimmed := strings.TrimSpace(pattern)

	// Phase 1: process escape sequences into placeholders.
	const starPH = "\x00ESCAPED_STAR\x00"
	const bsPH = "\x00ESCAPED_BACKSLASH\x00"

	var b strings.Builder
	unescapedStars := 0
	i := 0
	for i < len(trimmed) {
		ch := trimmed[i]
		if ch == '\\' && i+1 < len(trimmed) {
			next := trimmed[i+1]
			if next == '*' {
				b.WriteString(starPH)
				i += 2
				continue
			}
			if next == '\\' {
				b.WriteString(bsPH)
				i += 2
				continue
			}
		}
		if ch == '*' {
			unescapedStars++
		}
		b.WriteByte(ch)
		i++
	}
	processed := b.String()

	// Phase 2: escape regex meta characters (except *).
	escaped := regexMetaChars.ReplaceAllString(processed, `\$0`)

	// Phase 3: convert unescaped * to .* for wildcard matching.
	withWildcards := strings.ReplaceAll(escaped, "*", ".*")

	// Phase 4: restore placeholders to regex-escaped literals.
	regexPattern := strings.ReplaceAll(withWildcards, starPH, `\*`)
	regexPattern = strings.ReplaceAll(regexPattern, bsPH, `\\`)

	// Phase 5: trailing " *" with single wildcard → optional space+args.
	if strings.HasSuffix(regexPattern, " .*") && unescapedStars == 1 {
		regexPattern = regexPattern[:len(regexPattern)-3] + "( .*)?"
	}

	// Phase 6: compile with dotAll ((?s)) and optional case-insensitive.
	flags := "(?s)"
	if caseInsensitive {
		flags = "(?si)"
	}
	re, err := regexp.Compile("^" + flags + regexPattern + "$")
	if err != nil {
		return false
	}
	return re.MatchString(command)
}

// ParseShellPermissionRule parses a permission rule string into
// a structured ShellPermissionRule.
func ParseShellPermissionRule(rule string) ShellPermissionRule {
	if prefix, ok := PermissionRuleExtractPrefix(rule); ok {
		return ShellPermissionRule{
			Type:   ShellRulePrefix,
			Prefix: prefix,
		}
	}
	if HasWildcards(rule) {
		return ShellPermissionRule{
			Type:    ShellRuleWildcard,
			Pattern: rule,
		}
	}
	return ShellPermissionRule{
		Type:    ShellRuleExact,
		Command: rule,
	}
}

// MatchShellPermissionRule checks if a command matches a parsed
// permission rule.
func MatchShellPermissionRule(rule ShellPermissionRule, command string) bool {
	switch rule.Type {
	case ShellRuleExact:
		return command == rule.Command
	case ShellRulePrefix:
		return command == rule.Prefix || strings.HasPrefix(command, rule.Prefix+" ")
	case ShellRuleWildcard:
		return MatchWildcardPattern(rule.Pattern, command, false)
	}
	return false
}
