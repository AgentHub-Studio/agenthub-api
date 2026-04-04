package agentic

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Unicode sanitization for hidden character attack mitigation.
//
// Inspired by Claude Code's sanitization.ts — protects against
// ASCII Smuggling and Hidden Prompt Injection attacks that use
// invisible Unicode characters (Tag characters, format controls,
// private use areas, noncharacters) to hide malicious instructions
// invisible to users but processed by AI models.
//
// Reference: https://embracethered.com/blog/posts/2024/hiding-and-finding-text-with-unicode-tags/

// maxSanitizationIterations is the safety limit to prevent infinite loops.
const maxSanitizationIterations = 10

// SanitizeUnicode applies iterative Unicode sanitization to a string.
// It applies NFKC normalization and removes dangerous Unicode categories
// (format chars, private use, unassigned, zero-width, directional, BOM).
// Returns an error if the sanitization does not converge within the
// iteration limit (indicates a bug or adversarial input).
func SanitizeUnicode(input string) (string, error) {
	current := input
	previous := ""

	for i := 0; i < maxSanitizationIterations; i++ {
		if current == previous {
			return current, nil
		}
		previous = current

		// NFKC normalization to decompose composed character sequences
		current = norm.NFKC.String(current)

		// Remove dangerous Unicode categories
		current = removeDangerousRunes(current)

		// Remove specific known dangerous ranges as fallback
		current = removeKnownDangerousRanges(current)
	}

	return "", fmt.Errorf(
		"unicode sanitization reached maximum iterations (%d) for input: %s",
		maxSanitizationIterations, truncateForError(input, 100),
	)
}

// MustSanitizeUnicode is like SanitizeUnicode but returns the input
// unchanged if sanitization fails (for use in non-critical paths).
func MustSanitizeUnicode(input string) string {
	result, err := SanitizeUnicode(input)
	if err != nil {
		return input
	}
	return result
}

// IsDangerousUnicode returns true if the string contains any
// dangerous Unicode characters that would be removed by sanitization.
func IsDangerousUnicode(s string) bool {
	sanitized, err := SanitizeUnicode(s)
	if err != nil {
		return true
	}
	return sanitized != s
}

// removeDangerousRunes removes Unicode characters in dangerous categories:
// Cf (format), Co (private use), Cn (unassigned/noncharacter).
func removeDangerousRunes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isDangerousCategory(r) {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// isDangerousCategory returns true if the rune is in a dangerous
// Unicode general category (Cf, Co, or Cn).
func isDangerousCategory(r rune) bool {
	return unicode.Is(unicode.Cf, r) || // Format characters
		unicode.Is(unicode.Co, r) || // Private use area
		unicode.Is(unicode.Cs, r) // Surrogates (stand-in for unassigned/noncharacters)
}

// removeKnownDangerousRanges removes specific known dangerous Unicode
// ranges as a fallback for environments where category-based filtering
// might miss characters.
func removeKnownDangerousRanges(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isKnownDangerousRange(r) {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// isKnownDangerousRange checks specific Unicode ranges known to be
// used in hidden character attacks.
func isKnownDangerousRange(r rune) bool {
	switch {
	case r >= 0x200B && r <= 0x200F: // Zero-width spaces, LTR/RTL marks
		return true
	case r >= 0x202A && r <= 0x202E: // Directional formatting
		return true
	case r >= 0x2066 && r <= 0x2069: // Directional isolates
		return true
	case r == 0xFEFF: // Byte order mark
		return true
	case r >= 0xE000 && r <= 0xF8FF: // BMP private use area
		return true
	default:
		return false
	}
}

// truncateForError truncates a string for inclusion in error messages.
func truncateForError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
