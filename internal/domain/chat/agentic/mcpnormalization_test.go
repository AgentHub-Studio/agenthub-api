package agentic_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NormalizeMCPName ---

func TestNormalizeMCPName_AlreadyValid(t *testing.T) {
	assert.Equal(t, "my-server", agentic.NormalizeMCPName("my-server"))
}

func TestNormalizeMCPName_WithSpaces(t *testing.T) {
	assert.Equal(t, "my_cool_server", agentic.NormalizeMCPName("my cool server"))
}

func TestNormalizeMCPName_WithDots(t *testing.T) {
	assert.Equal(t, "api_example_com", agentic.NormalizeMCPName("api.example.com"))
}

func TestNormalizeMCPName_ClaudeAI_CollapsesUnderscores(t *testing.T) {
	result := agentic.NormalizeMCPName("claude.ai My Server")
	assert.Equal(t, "claude_ai_My_Server", result)
}

func TestNormalizeMCPName_ClaudeAI_StripsLeadingTrailing(t *testing.T) {
	result := agentic.NormalizeMCPName("claude.ai  test ")
	// "claude.ai  test " → "claude_ai__test_" → collapse → "claude_ai_test_" → strip → "claude_ai_test"
	assert.Equal(t, "claude_ai_test", result)
}

func TestNormalizeMCPName_NonClaudeAI_KeepsUnderscores(t *testing.T) {
	result := agentic.NormalizeMCPName("my..server")
	assert.Equal(t, "my__server", result) // consecutive underscores kept
}

func TestNormalizeMCPName_SpecialChars(t *testing.T) {
	result := agentic.NormalizeMCPName("server@v2#beta!")
	assert.Equal(t, "server_v2_beta_", result)
}

func TestNormalizeMCPName_Truncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	result := agentic.NormalizeMCPName(long)
	assert.Len(t, result, 64)
}

func TestNormalizeMCPName_Hyphens(t *testing.T) {
	assert.Equal(t, "my-server-v2", agentic.NormalizeMCPName("my-server-v2"))
}

func TestNormalizeMCPName_Numbers(t *testing.T) {
	assert.Equal(t, "server123", agentic.NormalizeMCPName("server123"))
}

// --- IsValidMCPName ---

func TestIsValidMCPName_Valid(t *testing.T) {
	assert.True(t, agentic.IsValidMCPName("my-server"))
	assert.True(t, agentic.IsValidMCPName("server_v2"))
	assert.True(t, agentic.IsValidMCPName("A"))
	assert.True(t, agentic.IsValidMCPName(strings.Repeat("x", 64)))
}

func TestIsValidMCPName_Invalid(t *testing.T) {
	assert.False(t, agentic.IsValidMCPName(""))
	assert.False(t, agentic.IsValidMCPName("has spaces"))
	assert.False(t, agentic.IsValidMCPName("has.dots"))
	assert.False(t, agentic.IsValidMCPName(strings.Repeat("x", 65)))
}
