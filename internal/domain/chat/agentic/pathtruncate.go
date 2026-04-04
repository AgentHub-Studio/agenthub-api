package agentic

import (
	"strings"
	"unicode/utf8"
)

// Path truncation utilities for display in constrained contexts.
//
// Inspired by Claude Code's truncate.ts — truncates file paths in
// the middle to preserve both directory context and filename.
// Simplified version without terminal-width awareness (uses rune
// count instead of display width).

// TruncatePathMiddle truncates a file path in the middle to fit
// within maxLen runes, preserving directory context and filename.
// Example: "src/components/deeply/nested/folder/MyComponent.tsx"
// becomes "src/com…/MyComponent.tsx" when maxLen is 25.
func TruncatePathMiddle(path string, maxLen int) string {
	if utf8.RuneCountInString(path) <= maxLen {
		return path
	}

	if maxLen <= 0 {
		return "…"
	}

	if maxLen < 5 {
		return truncateRuneEnd(path, maxLen)
	}

	// Find the filename (last path segment)
	lastSlash := strings.LastIndex(path, "/")
	var filename, directory string
	if lastSlash >= 0 {
		filename = path[lastSlash:] // include leading slash
		directory = path[:lastSlash]
	} else {
		filename = path
		directory = ""
	}

	filenameLen := utf8.RuneCountInString(filename)

	// If filename alone is too long, truncate from start
	if filenameLen >= maxLen-1 {
		return truncateRuneStart(path, maxLen)
	}

	// Calculate space for directory prefix: dir + "…" + filename
	availableForDir := maxLen - 1 - filenameLen // -1 for ellipsis
	if availableForDir <= 0 {
		return truncateRuneStart(filename, maxLen)
	}

	truncatedDir := truncateRuneEndNoEllipsis(directory, availableForDir)
	return truncatedDir + "…" + filename
}

// TruncateEnd truncates a string at the end if it exceeds maxLen
// runes, appending "…" as indicator.
func TruncateEnd(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	return truncateRuneEnd(s, maxLen)
}

// TruncateStart truncates a string from the start if it exceeds
// maxLen runes, prepending "…" as indicator.
func TruncateStart(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	return truncateRuneStart(s, maxLen)
}

// truncateRuneEnd truncates to maxLen runes with trailing ellipsis.
func truncateRuneEnd(s string, maxLen int) string {
	if maxLen <= 1 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// truncateRuneStart truncates from start to maxLen runes with leading ellipsis.
func truncateRuneStart(s string, maxLen int) string {
	if maxLen <= 1 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return "…" + string(runes[len(runes)-(maxLen-1):])
}

// truncateRuneEndNoEllipsis truncates to maxLen runes without adding ellipsis.
func truncateRuneEndNoEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
