package agentic

import "strings"

// Slash command parsing utilities.
//
// Inspired by Claude Code's slashCommandParsing.ts — parses "/command
// args" input strings into command name, arguments, and MCP flag.
// Useful for chat interfaces where users invoke skills or tools via
// slash-prefixed commands.

// ParsedSlashCommand holds the result of parsing a slash command.
type ParsedSlashCommand struct {
	CommandName string // the command name without the leading '/'
	Args        string // remaining arguments after the command name
	IsMCP       bool   // true if the command has the "(MCP)" flag
}

// ParseSlashCommand parses a slash command input string into its
// component parts. Returns nil if the input is not a valid slash
// command (doesn't start with '/').
//
// Examples:
//
//	"/search foo bar"      → {CommandName:"search", Args:"foo bar", IsMCP:false}
//	"/mcp:tool (MCP) arg"  → {CommandName:"mcp:tool (MCP)", Args:"arg", IsMCP:true}
//	"/help"                → {CommandName:"help", Args:"", IsMCP:false}
func ParseSlashCommand(input string) *ParsedSlashCommand {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	withoutSlash := trimmed[1:]
	words := strings.Fields(withoutSlash)
	if len(words) == 0 {
		return nil
	}

	commandName := words[0]
	isMCP := false
	argsStart := 1

	// Check for MCP commands (second word is "(MCP)")
	if len(words) > 1 && words[1] == "(MCP)" {
		commandName = commandName + " (MCP)"
		isMCP = true
		argsStart = 2
	}

	args := ""
	if argsStart < len(words) {
		args = strings.Join(words[argsStart:], " ")
	}

	return &ParsedSlashCommand{
		CommandName: commandName,
		Args:        args,
		IsMCP:       isMCP,
	}
}

// IsSlashCommand returns true if the input starts with '/'.
func IsSlashCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}
