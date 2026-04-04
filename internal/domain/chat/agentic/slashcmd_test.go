package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestParseSlashCommand_Basic(t *testing.T) {
	result := agentic.ParseSlashCommand("/search foo bar")
	require.NotNil(t, result)
	assert.Equal(t, "search", result.CommandName)
	assert.Equal(t, "foo bar", result.Args)
	assert.False(t, result.IsMCP)
}

func TestParseSlashCommand_NoArgs(t *testing.T) {
	result := agentic.ParseSlashCommand("/help")
	require.NotNil(t, result)
	assert.Equal(t, "help", result.CommandName)
	assert.Equal(t, "", result.Args)
	assert.False(t, result.IsMCP)
}

func TestParseSlashCommand_MCP(t *testing.T) {
	result := agentic.ParseSlashCommand("/mcp:tool (MCP) arg1 arg2")
	require.NotNil(t, result)
	assert.Equal(t, "mcp:tool (MCP)", result.CommandName)
	assert.Equal(t, "arg1 arg2", result.Args)
	assert.True(t, result.IsMCP)
}

func TestParseSlashCommand_MCPNoArgs(t *testing.T) {
	result := agentic.ParseSlashCommand("/mcp:tool (MCP)")
	require.NotNil(t, result)
	assert.Equal(t, "mcp:tool (MCP)", result.CommandName)
	assert.Equal(t, "", result.Args)
	assert.True(t, result.IsMCP)
}

func TestParseSlashCommand_NotSlashCommand(t *testing.T) {
	assert.Nil(t, agentic.ParseSlashCommand("hello world"))
}

func TestParseSlashCommand_Empty(t *testing.T) {
	assert.Nil(t, agentic.ParseSlashCommand(""))
}

func TestParseSlashCommand_JustSlash(t *testing.T) {
	assert.Nil(t, agentic.ParseSlashCommand("/"))
}

func TestParseSlashCommand_WithLeadingSpaces(t *testing.T) {
	result := agentic.ParseSlashCommand("  /commit message here")
	require.NotNil(t, result)
	assert.Equal(t, "commit", result.CommandName)
	assert.Equal(t, "message here", result.Args)
}

func TestParseSlashCommand_MultipleSpaces(t *testing.T) {
	result := agentic.ParseSlashCommand("/search  multiple   spaces")
	require.NotNil(t, result)
	assert.Equal(t, "search", result.CommandName)
	assert.Equal(t, "multiple spaces", result.Args)
}

// --- IsSlashCommand ---

func TestIsSlashCommand_True(t *testing.T) {
	assert.True(t, agentic.IsSlashCommand("/help"))
}

func TestIsSlashCommand_False(t *testing.T) {
	assert.False(t, agentic.IsSlashCommand("hello"))
}

func TestIsSlashCommand_LeadingWhitespace(t *testing.T) {
	assert.True(t, agentic.IsSlashCommand("  /command"))
}

func TestIsSlashCommand_Empty(t *testing.T) {
	assert.False(t, agentic.IsSlashCommand(""))
}
