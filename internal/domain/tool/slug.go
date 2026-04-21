package tool

import (
	"strings"

	"github.com/google/uuid"
)

// ToSlug converts a human-readable tool name into a stable, LLM-safe slug.
// The slug is exposed to LLMs as the tool name in OpenAI function-calling
// schemas (tool_use), so it must only contain characters that providers
// accept for function names: [a-z0-9_.-].
//
// Rules:
//   - Lowercased
//   - Any run of non-[a-z0-9_.-] characters collapses to a single underscore
//   - Leading/trailing underscores, dots and hyphens are trimmed
//   - If the result is empty (e.g. all punctuation), a deterministic
//     "tool-<uuid-prefix>" fallback is returned so INSERTs never fail the
//     NOT NULL constraint on the slug column.
func ToSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prev := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_', c == '.', c == '-':
			b.WriteByte(c)
			prev = c
		default:
			if prev != '_' && b.Len() > 0 {
				b.WriteByte('_')
				prev = '_'
			}
		}
	}
	result := strings.Trim(b.String(), "_.-")
	if result == "" {
		return "tool-" + uuid.New().String()[:8]
	}
	return result
}
