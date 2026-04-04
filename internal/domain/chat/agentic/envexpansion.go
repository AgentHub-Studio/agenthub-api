package agentic

import (
	"os"
	"regexp"
	"strings"
)

// Environment variable expansion in configuration strings.
//
// Inspired by Claude Code's envExpansion.ts — expands ${VAR} and
// ${VAR:-default} placeholders in configuration values (e.g., MCP
// server configs, tool definitions). Reports missing variables for
// error handling rather than silently substituting empty strings.

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// EnvExpansionResult holds the expanded string and any variables
// that were referenced but not found in the environment.
type EnvExpansionResult struct {
	Expanded    string
	MissingVars []string
}

// ExpandEnvVars expands ${VAR} and ${VAR:-default} placeholders in a
// string using os.Getenv. Returns the expanded string and a list of
// any referenced variables that were not found (and had no default).
func ExpandEnvVars(value string) EnvExpansionResult {
	return ExpandEnvVarsWithLookup(value, os.Getenv)
}

// ExpandEnvVarsWithLookup is like ExpandEnvVars but uses a custom
// lookup function instead of os.Getenv. Useful for testing.
func ExpandEnvVarsWithLookup(value string, lookup func(string) string) EnvExpansionResult {
	var missingVars []string

	expanded := envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Strip ${ and }
		inner := match[2 : len(match)-1]

		// Split on :- to support default values (limit 2 parts)
		varName, defaultVal, hasDefault := splitDefault(inner)

		envValue := lookup(varName)
		if envValue != "" {
			return envValue
		}
		if hasDefault {
			return defaultVal
		}

		missingVars = append(missingVars, varName)
		return match // preserve original for debugging
	})

	return EnvExpansionResult{
		Expanded:    expanded,
		MissingVars: missingVars,
	}
}

// splitDefault splits "VAR:-default" into name, default, true.
// Returns name, "", false if no :- separator.
func splitDefault(s string) (name, defaultVal string, hasDefault bool) {
	idx := strings.Index(s, ":-")
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+2:], true
}
