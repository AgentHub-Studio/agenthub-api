package agentic

import (
	"regexp"
	"strings"
)

// MCP server name normalization.
//
// Inspired by Claude Code's mcp/normalization.ts — normalizes server
// names to be compatible with the MCP API pattern ^[a-zA-Z0-9_-]{1,64}$.
// Replaces invalid characters (dots, spaces, etc.) with underscores.
// For claude.ai servers, also collapses consecutive underscores and
// strips leading/trailing underscores to prevent interference with
// the __ delimiter used in MCP tool names.

const claudeAIServerPrefix = "claude.ai "

var (
	invalidMCPCharsRe    = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	consecutiveUnderRe   = regexp.MustCompile(`_+`)
	leadTrailUnderRe     = regexp.MustCompile(`^_|_$`)
	validMCPNameRe       = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

// NormalizeMCPName normalizes a server name to be compatible with the
// MCP API pattern ^[a-zA-Z0-9_-]{1,64}$. Invalid characters are
// replaced with underscores. For claude.ai servers, consecutive
// underscores are collapsed and leading/trailing underscores are stripped.
func NormalizeMCPName(name string) string {
	normalized := invalidMCPCharsRe.ReplaceAllString(name, "_")

	if strings.HasPrefix(name, claudeAIServerPrefix) {
		normalized = consecutiveUnderRe.ReplaceAllString(normalized, "_")
		normalized = leadTrailUnderRe.ReplaceAllString(normalized, "")
	}

	// Truncate to 64 characters
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}

	return normalized
}

// IsValidMCPName returns true if the name matches the MCP API pattern.
func IsValidMCPName(name string) bool {
	return validMCPNameRe.MatchString(name)
}
