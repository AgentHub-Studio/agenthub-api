package agentic

import "strings"

// Command exit code semantic interpretation.
//
// Inspired by Claude Code's commandSemantics.ts — many CLI commands
// use non-zero exit codes for informational purposes (not errors).
// For example, grep returns 1 for "no matches found", diff returns
// 1 for "files differ". This module interprets exit codes based on
// which command was run.

// CommandSemanticResult holds the semantic interpretation of a command's exit.
type CommandSemanticResult struct {
	IsError bool
	Message string
}

// commandSemantic is a function that interprets an exit code.
type commandSemantic func(exitCode int, stdout, stderr string) CommandSemanticResult

// commandSemantics maps command names to their exit code interpreters.
var commandSemantics = map[string]commandSemantic{
	// grep: 0=matches found, 1=no matches, 2+=error
	"grep": func(exitCode int, _, _ string) CommandSemanticResult {
		if exitCode == 1 {
			return CommandSemanticResult{IsError: false, Message: "No matches found"}
		}
		return CommandSemanticResult{IsError: exitCode >= 2}
	},
	// ripgrep: same semantics as grep
	"rg": func(exitCode int, _, _ string) CommandSemanticResult {
		if exitCode == 1 {
			return CommandSemanticResult{IsError: false, Message: "No matches found"}
		}
		return CommandSemanticResult{IsError: exitCode >= 2}
	},
	// find: 0=success, 1=some dirs inaccessible, 2+=error
	"find": func(exitCode int, _, _ string) CommandSemanticResult {
		if exitCode == 1 {
			return CommandSemanticResult{IsError: false, Message: "Some directories were inaccessible"}
		}
		return CommandSemanticResult{IsError: exitCode >= 2}
	},
	// diff: 0=no differences, 1=differences found, 2+=error
	"diff": func(exitCode int, _, _ string) CommandSemanticResult {
		if exitCode == 1 {
			return CommandSemanticResult{IsError: false, Message: "Files differ"}
		}
		return CommandSemanticResult{IsError: exitCode >= 2}
	},
	// test/[: 0=condition true, 1=condition false, 2+=error
	"test": func(exitCode int, _, _ string) CommandSemanticResult {
		if exitCode == 1 {
			return CommandSemanticResult{IsError: false, Message: "Condition is false"}
		}
		return CommandSemanticResult{IsError: exitCode >= 2}
	},
	"[": func(exitCode int, _, _ string) CommandSemanticResult {
		if exitCode == 1 {
			return CommandSemanticResult{IsError: false, Message: "Condition is false"}
		}
		return CommandSemanticResult{IsError: exitCode >= 2}
	},
}

// InterpretCommandSemanticResult interprets a command's exit code using
// command-specific semantics. Commands like grep, diff, test use
// exit code 1 for non-error conditions.
func InterpretCommandSemanticResult(command string, exitCode int, stdout, stderr string) CommandSemanticResult {
	base := heuristicallyExtractBaseCommand(command)
	if sem, ok := commandSemantics[base]; ok {
		return sem(exitCode, stdout, stderr)
	}
	// Default: only 0 is success.
	if exitCode != 0 {
		return CommandSemanticResult{IsError: true, Message: "Command failed with exit code " + itoa(exitCode)}
	}
	return CommandSemanticResult{}
}

// heuristicallyExtractBaseCommand extracts the base command name
// from a possibly piped/chained command line. Takes the last
// segment (which determines the exit code in a pipe).
func heuristicallyExtractBaseCommand(command string) string {
	// Split on pipes — last command determines exit code.
	segments := strings.Split(command, "|")
	last := strings.TrimSpace(segments[len(segments)-1])

	// Also handle && and ;
	for _, sep := range []string{"&&", ";"} {
		parts := strings.Split(last, sep)
		last = strings.TrimSpace(parts[len(parts)-1])
	}

	// Extract first word.
	fields := strings.Fields(last)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
