package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- MCPInfoFromString ---

func TestMCPInfoFromString_Valid(t *testing.T) {
	info := agentic.MCPInfoFromString("mcp__github__list_repos")
	assert.NotNil(t, info)
	assert.Equal(t, "github", info.ServerName)
	assert.Equal(t, "list_repos", info.ToolName)
}

func TestMCPInfoFromString_NoToolName(t *testing.T) {
	info := agentic.MCPInfoFromString("mcp__github")
	assert.NotNil(t, info)
	assert.Equal(t, "github", info.ServerName)
	assert.Equal(t, "", info.ToolName)
}

func TestMCPInfoFromString_NotMCP(t *testing.T) {
	assert.Nil(t, agentic.MCPInfoFromString("builtin__read"))
}

func TestMCPInfoFromString_Empty(t *testing.T) {
	assert.Nil(t, agentic.MCPInfoFromString(""))
}

func TestMCPInfoFromString_OnlyMCP(t *testing.T) {
	assert.Nil(t, agentic.MCPInfoFromString("mcp"))
}

func TestMCPInfoFromString_EmptyServer(t *testing.T) {
	assert.Nil(t, agentic.MCPInfoFromString("mcp__"))
}

func TestMCPInfoFromString_ToolWithUnderscores(t *testing.T) {
	info := agentic.MCPInfoFromString("mcp__server__my__complex__tool")
	assert.NotNil(t, info)
	assert.Equal(t, "server", info.ServerName)
	assert.Equal(t, "my__complex__tool", info.ToolName)
}

// --- MCPPrefix ---

func TestMCPPrefix_Simple(t *testing.T) {
	assert.Equal(t, "mcp__github__", agentic.MCPPrefix("github"))
}

func TestMCPPrefix_WithSpaces(t *testing.T) {
	assert.Equal(t, "mcp__My_Server__", agentic.MCPPrefix("My Server"))
}

// --- BuildMCPToolName ---

func TestBuildMCPToolName(t *testing.T) {
	result := agentic.BuildMCPToolName("github", "list_repos")
	assert.Equal(t, "mcp__github__list_repos", result)
}

func TestBuildMCPToolName_Normalized(t *testing.T) {
	result := agentic.BuildMCPToolName("My Server", "list files")
	assert.Equal(t, "mcp__My_Server__list_files", result)
}

// --- MCPDisplayName ---

func TestMCPDisplayName(t *testing.T) {
	result := agentic.MCPDisplayName("mcp__github__list_repos", "github")
	assert.Equal(t, "list_repos", result)
}

func TestMCPDisplayName_NoMatch(t *testing.T) {
	result := agentic.MCPDisplayName("mcp__other__tool", "github")
	assert.Equal(t, "mcp__other__tool", result)
}

// --- IsMCPToolName ---

func TestIsMCPToolName_True(t *testing.T) {
	assert.True(t, agentic.IsMCPToolName("mcp__github__list"))
}

func TestIsMCPToolName_False(t *testing.T) {
	assert.False(t, agentic.IsMCPToolName("Read"))
	assert.False(t, agentic.IsMCPToolName(""))
}

// --- ExtractMCPToolDisplayName ---

func TestExtractMCPToolDisplayName_Full(t *testing.T) {
	result := agentic.ExtractMCPToolDisplayName("github - Add comment to issue (MCP)")
	assert.Equal(t, "Add comment to issue", result)
}

func TestExtractMCPToolDisplayName_NoMCPSuffix(t *testing.T) {
	result := agentic.ExtractMCPToolDisplayName("github - list_repos")
	assert.Equal(t, "list_repos", result)
}

func TestExtractMCPToolDisplayName_NoPrefix(t *testing.T) {
	result := agentic.ExtractMCPToolDisplayName("list_repos (MCP)")
	assert.Equal(t, "list_repos", result)
}

func TestExtractMCPToolDisplayName_Plain(t *testing.T) {
	result := agentic.ExtractMCPToolDisplayName("simple_tool")
	assert.Equal(t, "simple_tool", result)
}
