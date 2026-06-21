package tool

import (
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

// ToSlug converts a human-readable tool name into a stable, LLM-safe slug.
// The slug is exposed to LLMs as the tool name in OpenAI function-calling
// schemas (tool_use), so it uses the product v1 canonical slug contract.
//
// Rules:
//   - Lowercased
//   - Any run of non-alphanumeric characters collapses to a single hyphen
//   - Leading/trailing hyphens are trimmed
//   - If the result is empty (e.g. all punctuation), a deterministic
//     "tool-<uuid-prefix>" fallback is returned so INSERTs never fail the
//     NOT NULL constraint on the slug column.
func ToSlug(name string) string {
	return sanitize.ToSlug(name, "tool")
}
