package agentic

import "strings"

// Lightweight git config file parser.
//
// Inspired by Claude Code's gitConfigParser.ts — parses .git/config
// files following git's config.c rules: case-insensitive section/key
// names, case-sensitive quoted subsections, backslash escapes, inline
// comments. Useful for reading remote URLs, branch tracking, and
// other git configuration without shelling out to git.

// ParseGitConfigString parses a value from an in-memory git config
// string. Section and key matching is case-insensitive; subsection
// matching (if non-empty) is case-sensitive.
//
// Example: ParseGitConfigString(config, "remote", "origin", "url")
func ParseGitConfigString(config, section, subsection, key string) string {
	lines := strings.Split(config, "\n")
	sectionLower := strings.ToLower(section)
	keyLower := strings.ToLower(key)

	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' || trimmed[0] == ';' {
			continue
		}

		if trimmed[0] == '[' {
			inSection = gitConfigMatchesSectionHeader(trimmed, sectionLower, subsection)
			continue
		}

		if !inSection {
			continue
		}

		k, v, ok := gitConfigParseKeyValue(trimmed)
		if ok && strings.ToLower(k) == keyLower {
			return v
		}
	}
	return ""
}

// ParseAllGitConfigValues returns all values for a key in a section.
func ParseAllGitConfigValues(config, section, subsection, key string) []string {
	lines := strings.Split(config, "\n")
	sectionLower := strings.ToLower(section)
	keyLower := strings.ToLower(key)

	var results []string
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' || trimmed[0] == ';' {
			continue
		}
		if trimmed[0] == '[' {
			inSection = gitConfigMatchesSectionHeader(trimmed, sectionLower, subsection)
			continue
		}
		if !inSection {
			continue
		}
		k, v, ok := gitConfigParseKeyValue(trimmed)
		if ok && strings.ToLower(k) == keyLower {
			results = append(results, v)
		}
	}
	return results
}

// ListGitConfigSections returns all section headers (with subsections).
func ListGitConfigSections(config string) []string {
	lines := strings.Split(config, "\n")
	var sections []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			sec := gitConfigExtractSectionName(trimmed)
			if sec != "" {
				sections = append(sections, sec)
			}
		}
	}
	return sections
}

func gitConfigMatchesSectionHeader(line, sectionLower, subsection string) bool {
	i := 1 // skip '['

	// Read section name.
	for i < len(line) && line[i] != ']' && line[i] != ' ' && line[i] != '\t' && line[i] != '"' {
		i++
	}
	foundSection := strings.ToLower(line[1:i])
	if foundSection != sectionLower {
		return false
	}

	if subsection == "" {
		return i < len(line) && line[i] == ']'
	}

	// Skip whitespace before subsection quote.
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) || line[i] != '"' {
		return false
	}
	i++ // skip opening quote

	// Read subsection — case-sensitive, handle \\ and \" escapes.
	var foundSubsection strings.Builder
	for i < len(line) && line[i] != '"' {
		if line[i] == '\\' && i+1 < len(line) {
			next := line[i+1]
			if next == '\\' || next == '"' {
				foundSubsection.WriteByte(next)
				i += 2
				continue
			}
			foundSubsection.WriteByte(next)
			i += 2
			continue
		}
		foundSubsection.WriteByte(line[i])
		i++
	}

	if i >= len(line) || line[i] != '"' {
		return false
	}
	i++ // skip closing quote
	if i >= len(line) || line[i] != ']' {
		return false
	}

	return foundSubsection.String() == subsection
}

func gitConfigParseKeyValue(line string) (key, value string, ok bool) {
	i := 0
	for i < len(line) && isGitConfigKeyChar(line[i]) {
		i++
	}
	if i == 0 {
		return "", "", false
	}
	key = line[:i]

	// Skip whitespace.
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) || line[i] != '=' {
		return "", "", false
	}
	i++ // skip '='

	// Skip whitespace after '='.
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}

	value = gitConfigParseValue(line, i)
	return key, value, true
}

func gitConfigParseValue(line string, start int) string {
	var result strings.Builder
	inQuote := false
	i := start

	for i < len(line) {
		ch := line[i]

		if !inQuote && (ch == '#' || ch == ';') {
			break
		}

		if ch == '"' {
			inQuote = !inQuote
			i++
			continue
		}

		if ch == '\\' && i+1 < len(line) {
			next := line[i+1]
			if inQuote {
				switch next {
				case 'n':
					result.WriteByte('\n')
				case 't':
					result.WriteByte('\t')
				case 'b':
					result.WriteByte('\b')
				case '"':
					result.WriteByte('"')
				case '\\':
					result.WriteByte('\\')
				default:
					result.WriteByte(next)
				}
				i += 2
				continue
			}
			if next == '\\' {
				result.WriteByte('\\')
				i += 2
				continue
			}
		}

		result.WriteByte(ch)
		i++
	}

	s := result.String()
	if !inQuote {
		s = strings.TrimRight(s, " \t")
	}
	return s
}

func isGitConfigKeyChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '-'
}

func gitConfigExtractSectionName(line string) string {
	// line starts with '['
	end := strings.IndexByte(line, ']')
	if end == -1 {
		return ""
	}
	return line[1:end]
}
