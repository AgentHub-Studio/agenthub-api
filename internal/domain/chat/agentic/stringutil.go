package agentic

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// General string utility functions.
//
// Inspired by Claude Code's stringUtils.ts — regex escaping, case
// manipulation, pluralization, character counting, and CJK full-width
// normalization. Complements the truncation utilities in
// truncaccumulator.go.

var regexpSpecialChars = regexp.MustCompile(`[.*+?^${}()|[\]\\]`)

// EscapeRegExp escapes special regex characters in a string so it
// can be used as a literal pattern in a regexp.
func EscapeRegExp(s string) string {
	return regexpSpecialChars.ReplaceAllString(s, `\$0`)
}

// Capitalize uppercases the first character of a string, leaving the
// rest unchanged. Unlike some implementations, this does NOT
// lowercase remaining characters.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// Plural returns the singular or plural form of a word based on count.
// If pluralWord is empty, appends "s" to form the plural.
func Plural(n int, word, pluralWord string) string {
	if n == 1 {
		return word
	}
	if pluralWord != "" {
		return pluralWord
	}
	return word + "s"
}

// FirstLineOf returns the first line of a string without allocating
// a full split array.
func FirstLineOf(s string) string {
	idx := strings.IndexByte(s, '\n')
	if idx == -1 {
		return s
	}
	return s[:idx]
}

// CountCharInString counts occurrences of char in s using IndexByte
// jumps instead of per-character iteration.
func CountCharInString(s string, char byte) int {
	count := 0
	start := 0
	for {
		idx := strings.IndexByte(s[start:], char)
		if idx == -1 {
			break
		}
		count++
		start += idx + 1
	}
	return count
}

// NormalizeFullWidthDigits converts CJK full-width digits (U+FF10-FF19)
// to ASCII digits. Useful for accepting input from Japanese/CJK IMEs.
func NormalizeFullWidthDigits(s string) string {
	var sb strings.Builder
	changed := false
	for _, r := range s {
		if r >= '０' && r <= '９' {
			sb.WriteRune(r - 0xFEE0)
			changed = true
		} else {
			sb.WriteRune(r)
		}
	}
	if !changed {
		return s
	}
	return sb.String()
}

// NormalizeFullWidthSpace converts CJK ideographic spaces (U+3000)
// to ASCII spaces.
func NormalizeFullWidthSpace(s string) string {
	return strings.ReplaceAll(s, "\u3000", " ")
}
