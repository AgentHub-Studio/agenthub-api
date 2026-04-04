package agentic

import "strings"

// MCP tool name parsing and construction utilities.
//
// Inspired by Claude Code's mcpStringUtils.ts — provides pure string
// operations for MCP (Model Context Protocol) tool/server name handling.
// Tool names follow the format "mcp__serverName__toolName" where the
// double-underscore delimiter separates the three components.
//
// Depends on NormalizeMCPName from mcpnormalization.go.

// ParsedMCPToolName holds parsed information from an MCP tool name string.
type ParsedMCPToolName struct {
	ServerName string
	ToolName   string // may be empty if only server prefix was matched
}

// MCPInfoFromString parses an MCP tool name string in the format
// "mcp__serverName__toolName". Returns nil if the string is not a
// valid MCP tool name.
//
// Known limitation: if a server name contains "__", parsing will be
// incorrect (e.g., "mcp__my__server__tool" parses as server="my").
func MCPInfoFromString(toolString string) *ParsedMCPToolName {
	parts := strings.SplitN(toolString, "__", 3)
	if len(parts) < 2 || parts[0] != "mcp" || parts[1] == "" {
		return nil
	}

	info := &ParsedMCPToolName{
		ServerName: parts[1],
	}
	if len(parts) == 3 && parts[2] != "" {
		info.ToolName = parts[2]
	}
	return info
}

// MCPPrefix returns the MCP tool name prefix for a server name.
// The server name is normalized before constructing the prefix.
// Example: "My Server" → "mcp__My_Server__"
func MCPPrefix(serverName string) string {
	return "mcp__" + NormalizeMCPName(serverName) + "__"
}

// BuildMCPToolName constructs a fully qualified MCP tool name from
// server and tool names. Both names are normalized.
// Example: ("My Server", "list_files") → "mcp__My_Server__list_files"
func BuildMCPToolName(serverName, toolName string) string {
	return MCPPrefix(serverName) + NormalizeMCPName(toolName)
}

// MCPDisplayName strips the MCP prefix from a fully qualified tool
// name, returning just the tool name portion.
// Example: ("mcp__github__list_repos", "github") → "list_repos"
func MCPDisplayName(fullName, serverName string) string {
	prefix := "mcp__" + NormalizeMCPName(serverName) + "__"
	return strings.TrimPrefix(fullName, prefix)
}

// IsMCPToolName returns true if the string follows the MCP tool name
// convention (starts with "mcp__").
func IsMCPToolName(name string) bool {
	return strings.HasPrefix(name, "mcp__")
}

// ExtractMCPToolDisplayName extracts the tool display name from a
// user-facing name format like "server - tool_name (MCP)".
// Returns the tool name without server prefix and (MCP) suffix.
func ExtractMCPToolDisplayName(userFacingName string) string {
	// Remove " (MCP)" suffix
	name := strings.TrimSpace(userFacingName)
	if idx := strings.LastIndex(name, " (MCP)"); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}

	// Remove "server - " prefix
	if idx := strings.Index(name, " - "); idx >= 0 {
		return strings.TrimSpace(name[idx+3:])
	}

	return name
}
