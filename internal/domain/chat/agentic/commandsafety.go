package agentic

import (
	"strings"
)

// FlagArgType describes the kind of argument a CLI flag accepts.
// Inspired by Claude Code's FlagArgType in readOnlyCommandValidation.ts.
type FlagArgType string

const (
	// FlagArgNone means the flag takes no argument (e.g. --color, -n).
	FlagArgNone FlagArgType = "none"
	// FlagArgNumber means the flag takes a numeric argument (e.g. --context=3).
	FlagArgNumber FlagArgType = "number"
	// FlagArgString means the flag takes a string argument (e.g. --relative=path).
	FlagArgString FlagArgType = "string"
)

// CommandConfig defines the safety profile for a single command.
// SafeFlags lists the flags that are considered safe (read-only).
// If a command is in the allowlist and only uses safe flags, it can be
// auto-approved without user confirmation.
//
// Inspired by Claude Code's CommandConfig in readOnlyValidation.ts.
type CommandConfig struct {
	// SafeFlags maps flag names (e.g. "--help", "-n") to the type of argument they accept.
	SafeFlags map[string]FlagArgType
	// RespectsDoubleDash indicates the command treats "--" as end-of-options.
	// Default true for most POSIX commands. Set false for commands that don't
	// follow this convention (e.g. some Windows tools).
	RespectsDoubleDash bool
}

// commandAllowlist is the registry of commands that can be auto-approved when
// they only use safe (read-only) flags. Commands not in this list always require
// confirmation for execution.
//
// Inspired by Claude Code's COMMAND_ALLOWLIST in readOnlyValidation.ts and
// EXTERNAL_READONLY_COMMANDS in readOnlyCommandValidation.ts.
var commandAllowlist = map[string]CommandConfig{
	// --- File listing / search ---
	"ls": {
		SafeFlags: map[string]FlagArgType{
			"-l": FlagArgNone, "-a": FlagArgNone, "-h": FlagArgNone,
			"-R": FlagArgNone, "-t": FlagArgNone, "-S": FlagArgNone,
			"-r": FlagArgNone, "-1": FlagArgNone, "--color": FlagArgNone,
			"--human-readable": FlagArgNone, "--all": FlagArgNone,
			"--recursive": FlagArgNone, "--sort": FlagArgString,
		},
		RespectsDoubleDash: true,
	},
	"find": {
		SafeFlags: map[string]FlagArgType{
			"-name": FlagArgString, "-iname": FlagArgString,
			"-type": FlagArgString, "-maxdepth": FlagArgNumber,
			"-mindepth": FlagArgNumber, "-path": FlagArgString,
			"-not": FlagArgNone, "-print": FlagArgNone,
			"-size": FlagArgString, "-newer": FlagArgString,
			"-mtime": FlagArgString, "-ctime": FlagArgString,
			// -exec and -delete deliberately EXCLUDED (arbitrary code execution / deletion)
		},
		RespectsDoubleDash: true,
	},
	"fd": {
		SafeFlags: map[string]FlagArgType{
			"-h": FlagArgNone, "--help": FlagArgNone, "-V": FlagArgNone,
			"-d": FlagArgNumber, "--max-depth": FlagArgNumber,
			"-t": FlagArgString, "--type": FlagArgString,
			"-E": FlagArgString, "--exclude": FlagArgString,
			"-e": FlagArgString, "--extension": FlagArgString,
			"-H": FlagArgNone, "--hidden": FlagArgNone,
			"-I": FlagArgNone, "--no-ignore": FlagArgNone,
			"-s": FlagArgNone, "--case-sensitive": FlagArgNone,
			"-i": FlagArgNone, "--ignore-case": FlagArgNone,
			"-g": FlagArgString, "--glob": FlagArgString,
			"-p": FlagArgNone, "--full-path": FlagArgNone,
			"--color": FlagArgString,
			// -x/--exec and -X/--exec-batch deliberately EXCLUDED
		},
		RespectsDoubleDash: true,
	},
	"tree": {
		SafeFlags: map[string]FlagArgType{
			"-L": FlagArgNumber, "-d": FlagArgNone, "-a": FlagArgNone,
			"-f": FlagArgNone, "-I": FlagArgString, "--noreport": FlagArgNone,
			"-h": FlagArgNone, "-s": FlagArgNone, "-P": FlagArgString,
			"--charset": FlagArgString, "-C": FlagArgNone,
		},
		RespectsDoubleDash: true,
	},

	// --- Text search ---
	"grep": {
		SafeFlags: map[string]FlagArgType{
			"-r": FlagArgNone, "-R": FlagArgNone, "-i": FlagArgNone,
			"-l": FlagArgNone, "-L": FlagArgNone, "-n": FlagArgNone,
			"-c": FlagArgNone, "-v": FlagArgNone, "-w": FlagArgNone,
			"-e": FlagArgString, "-f": FlagArgString, "-m": FlagArgNumber,
			"-A": FlagArgNumber, "-B": FlagArgNumber, "-C": FlagArgNumber,
			"--include": FlagArgString, "--exclude": FlagArgString,
			"--exclude-dir": FlagArgString, "--color": FlagArgString,
			"-h": FlagArgNone, "-H": FlagArgNone, "-o": FlagArgNone,
			"-E": FlagArgNone, "-P": FlagArgNone, "-F": FlagArgNone,
			"--line-buffered": FlagArgNone,
		},
		RespectsDoubleDash: true,
	},
	"rg": {
		SafeFlags: map[string]FlagArgType{
			"-i": FlagArgNone, "--ignore-case": FlagArgNone,
			"-n": FlagArgNone, "--line-number": FlagArgNone,
			"-l": FlagArgNone, "--files-with-matches": FlagArgNone,
			"-c": FlagArgNone, "--count": FlagArgNone,
			"-v": FlagArgNone, "--invert-match": FlagArgNone,
			"-w": FlagArgNone, "--word-regexp": FlagArgNone,
			"-t": FlagArgString, "--type": FlagArgString,
			"-T": FlagArgString, "--type-not": FlagArgString,
			"-g": FlagArgString, "--glob": FlagArgString,
			"-m": FlagArgNumber, "--max-count": FlagArgNumber,
			"-A": FlagArgNumber, "--after-context": FlagArgNumber,
			"-B": FlagArgNumber, "--before-context": FlagArgNumber,
			"-C": FlagArgNumber, "--context": FlagArgNumber,
			"--max-depth": FlagArgNumber, "-d": FlagArgNumber,
			"--hidden": FlagArgNone, "--no-ignore": FlagArgNone,
			"--color": FlagArgString, "-e": FlagArgString,
			"--heading": FlagArgNone, "--no-heading": FlagArgNone,
			"-F": FlagArgNone, "--fixed-strings": FlagArgNone,
			"-U": FlagArgNone, "--multiline": FlagArgNone,
			"--json": FlagArgNone, "-o": FlagArgNone,
			"--sort": FlagArgString, "--sortr": FlagArgString,
		},
		RespectsDoubleDash: true,
	},

	// --- File reading ---
	"cat":  {SafeFlags: map[string]FlagArgType{"-n": FlagArgNone, "-b": FlagArgNone, "-s": FlagArgNone, "-A": FlagArgNone, "-e": FlagArgNone, "-t": FlagArgNone, "-v": FlagArgNone}, RespectsDoubleDash: true},
	"head": {SafeFlags: map[string]FlagArgType{"-n": FlagArgNumber, "-c": FlagArgNumber, "-q": FlagArgNone, "-v": FlagArgNone}, RespectsDoubleDash: true},
	"tail": {SafeFlags: map[string]FlagArgType{"-n": FlagArgNumber, "-c": FlagArgNumber, "-q": FlagArgNone, "-v": FlagArgNone, "-f": FlagArgNone, "--follow": FlagArgNone}, RespectsDoubleDash: true},
	"less": {SafeFlags: map[string]FlagArgType{"-N": FlagArgNone, "-S": FlagArgNone, "-R": FlagArgNone, "-X": FlagArgNone, "-F": FlagArgNone}, RespectsDoubleDash: true},
	"wc":   {SafeFlags: map[string]FlagArgType{"-l": FlagArgNone, "-w": FlagArgNone, "-c": FlagArgNone, "-m": FlagArgNone, "-L": FlagArgNone}, RespectsDoubleDash: true},
	"file": {SafeFlags: map[string]FlagArgType{"-b": FlagArgNone, "--brief": FlagArgNone, "-i": FlagArgNone, "--mime": FlagArgNone, "-L": FlagArgNone}, RespectsDoubleDash: true},
	"stat": {SafeFlags: map[string]FlagArgType{"-f": FlagArgString, "--format": FlagArgString, "-t": FlagArgNone, "-L": FlagArgNone}, RespectsDoubleDash: true},

	// --- Text processing (read-only) ---
	"sort":   {SafeFlags: map[string]FlagArgType{"-n": FlagArgNone, "-r": FlagArgNone, "-u": FlagArgNone, "-k": FlagArgString, "-t": FlagArgString, "-f": FlagArgNone, "-h": FlagArgNone, "-V": FlagArgNone}, RespectsDoubleDash: true},
	"uniq":   {SafeFlags: map[string]FlagArgType{"-c": FlagArgNone, "-d": FlagArgNone, "-u": FlagArgNone, "-i": FlagArgNone, "-f": FlagArgNumber, "-s": FlagArgNumber}, RespectsDoubleDash: true},
	"cut":    {SafeFlags: map[string]FlagArgType{"-d": FlagArgString, "-f": FlagArgString, "-c": FlagArgString, "-b": FlagArgString, "--complement": FlagArgNone, "-s": FlagArgNone}, RespectsDoubleDash: true},
	"diff":   {SafeFlags: map[string]FlagArgType{"-u": FlagArgNone, "-r": FlagArgNone, "-q": FlagArgNone, "--color": FlagArgString, "-N": FlagArgNone, "-b": FlagArgNone, "-w": FlagArgNone, "-y": FlagArgNone, "-i": FlagArgNone}, RespectsDoubleDash: true},
	"jq":     {SafeFlags: map[string]FlagArgType{"-r": FlagArgNone, "--raw-output": FlagArgNone, "-c": FlagArgNone, "--compact-output": FlagArgNone, "-e": FlagArgNone, "-S": FlagArgNone, "--sort-keys": FlagArgNone, "--tab": FlagArgNone, "--indent": FlagArgNumber}, RespectsDoubleDash: true},
	"column": {SafeFlags: map[string]FlagArgType{"-t": FlagArgNone, "-s": FlagArgString}, RespectsDoubleDash: true},

	// --- Version control (read-only subcommands) ---
	"git": {
		SafeFlags: map[string]FlagArgType{
			// Global git flags
			"--no-pager": FlagArgNone, "-C": FlagArgString,
		},
		RespectsDoubleDash: true,
	},

	// --- System info ---
	"pwd":   {SafeFlags: map[string]FlagArgType{"-L": FlagArgNone, "-P": FlagArgNone}, RespectsDoubleDash: true},
	"date":  {SafeFlags: map[string]FlagArgType{"-u": FlagArgNone, "--utc": FlagArgNone, "-R": FlagArgNone, "--rfc-email": FlagArgNone, "-I": FlagArgString, "+%*": FlagArgNone}, RespectsDoubleDash: true},
	"whoami": {SafeFlags: map[string]FlagArgType{}, RespectsDoubleDash: true},
	"uname": {SafeFlags: map[string]FlagArgType{"-a": FlagArgNone, "-s": FlagArgNone, "-r": FlagArgNone, "-m": FlagArgNone, "-n": FlagArgNone}, RespectsDoubleDash: true},
	"which": {SafeFlags: map[string]FlagArgType{"-a": FlagArgNone}, RespectsDoubleDash: true},
	"echo":  {SafeFlags: map[string]FlagArgType{"-n": FlagArgNone, "-e": FlagArgNone, "-E": FlagArgNone}, RespectsDoubleDash: false},
	"du":    {SafeFlags: map[string]FlagArgType{"-h": FlagArgNone, "-s": FlagArgNone, "-a": FlagArgNone, "-d": FlagArgNumber, "--max-depth": FlagArgNumber, "-c": FlagArgNone, "--total": FlagArgNone, "-k": FlagArgNone, "-m": FlagArgNone}, RespectsDoubleDash: true},
	"df":    {SafeFlags: map[string]FlagArgType{"-h": FlagArgNone, "-k": FlagArgNone, "-T": FlagArgNone, "-i": FlagArgNone, "--total": FlagArgNone}, RespectsDoubleDash: true},

	// --- Docker (read-only) ---
	"docker": {
		SafeFlags: map[string]FlagArgType{
			"--format": FlagArgString, "-f": FlagArgString,
			"--no-trunc": FlagArgNone, "-q": FlagArgNone,
		},
		RespectsDoubleDash: true,
	},

	// --- Kubernetes (read-only) ---
	"kubectl": {
		SafeFlags: map[string]FlagArgType{
			"-n": FlagArgString, "--namespace": FlagArgString,
			"-o": FlagArgString, "--output": FlagArgString,
			"-l": FlagArgString, "--selector": FlagArgString,
			"--no-headers": FlagArgNone, "--show-labels": FlagArgNone,
			"-A": FlagArgNone, "--all-namespaces": FlagArgNone,
			"--tail": FlagArgNumber, "-f": FlagArgNone, "--follow": FlagArgNone,
			"--context": FlagArgString, "--kubeconfig": FlagArgString,
			"-w": FlagArgNone, "--watch": FlagArgNone,
		},
		RespectsDoubleDash: true,
	},
}

// gitReadOnlySubcommands lists git subcommands that are read-only.
// Commands not in this list (e.g. push, commit, reset) are not auto-approved.
//
// Inspired by Claude Code's GIT_READ_ONLY_COMMANDS.
var gitReadOnlySubcommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true,
	"branch": true, "tag": true, "describe": true,
	"rev-parse": true, "rev-list": true, "ls-files": true,
	"ls-tree": true, "cat-file": true, "shortlog": true,
	"blame": true, "stash": true, "remote": true,
	"config": true, "reflog": true, "name-rev": true,
	"for-each-ref": true, "count-objects": true,
}

// dockerReadOnlySubcommands lists docker subcommands that are read-only.
var dockerReadOnlySubcommands = map[string]bool{
	"ps": true, "images": true, "inspect": true, "logs": true,
	"stats": true, "top": true, "port": true, "version": true,
	"info": true, "network": true, "volume": true,
}

// kubectlReadOnlySubcommands lists kubectl subcommands that are read-only.
var kubectlReadOnlySubcommands = map[string]bool{
	"get": true, "describe": true, "logs": true, "top": true,
	"explain": true, "api-resources": true, "api-versions": true,
	"cluster-info": true, "config": true, "version": true,
}

// subcommandAllowlists maps parent commands to their read-only subcommand sets.
var subcommandAllowlists = map[string]map[string]bool{
	"git":     gitReadOnlySubcommands,
	"docker":  dockerReadOnlySubcommands,
	"kubectl": kubectlReadOnlySubcommands,
}

// codeExecutionCommands lists commands that can execute arbitrary code.
// These are never auto-approved regardless of flags.
//
// Inspired by Claude Code's CROSS_PLATFORM_CODE_EXEC and DANGEROUS_BASH_PATTERNS.
var codeExecutionCommands = map[string]bool{
	"python": true, "python3": true, "python2": true,
	"node": true, "deno": true, "tsx": true,
	"ruby": true, "perl": true, "php": true, "lua": true,
	"bash": true, "sh": true, "zsh": true, "fish": true,
	"eval": true, "exec": true, "env": true, "xargs": true,
	"sudo": true, "su": true,
	"ssh": true, "scp": true,
	"curl": true, "wget": true,
	"dd": true, "mkfs": true,
}

// shellMetacharacters are characters that indicate shell command chaining,
// piping, or redirection. Commands containing these are never safe to auto-approve
// without full analysis.
var shellMetacharacters = []string{
	"|", "&&", "||", ";", "`",
	"$(", "$((", "$[",
	">>", ">", "<(",
}

// IsCommandReadOnly determines whether a shell command is safe to auto-approve
// (read-only, no side effects). Returns true only if:
// 1. The base command is in the allowlist
// 2. No shell metacharacters are present (no piping/chaining)
// 3. The command is not a code execution tool
// 4. For commands with subcommands (git, docker, kubectl), the subcommand is read-only
//
// This is a conservative check — it returns false for anything it can't prove safe.
// Inspired by Claude Code's readOnlyValidation.ts isReadOnlyCommand().
func IsCommandReadOnly(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}

	// Reject commands with shell metacharacters.
	for _, meta := range shellMetacharacters {
		if strings.Contains(trimmed, meta) {
			return false
		}
	}

	// Parse the base command.
	parts := splitCommandParts(trimmed)
	if len(parts) == 0 {
		return false
	}

	baseCmd := extractBaseCommand(parts[0])

	// Reject code execution commands.
	if codeExecutionCommands[baseCmd] {
		return false
	}

	// Check if command is in the allowlist.
	_, inAllowlist := commandAllowlist[baseCmd]
	if !inAllowlist {
		return false
	}

	// For commands with subcommand allowlists, verify the subcommand.
	if subcommands, hasSubcmds := subcommandAllowlists[baseCmd]; hasSubcmds {
		if len(parts) < 2 {
			return false // no subcommand = not provably safe
		}
		subcmd := parts[1]
		if !subcommands[subcmd] {
			return false
		}
	}

	return true
}

// CommandSafetyLevel classifies a command's safety.
type CommandSafetyLevel string

const (
	// CommandSafe means the command is read-only and can be auto-approved.
	CommandSafe CommandSafetyLevel = "safe"
	// CommandNeedsConfirm means the command may have side effects and needs user approval.
	CommandNeedsConfirm CommandSafetyLevel = "needs_confirm"
	// CommandDangerous means the command is explicitly dangerous and should be denied or require strong confirmation.
	CommandDangerous CommandSafetyLevel = "dangerous"
)

// ClassifyCommand determines the safety level of a shell command.
// It combines the read-only check with the dangerous command detection
// from permission.go to provide a three-level classification.
//
// Inspired by Claude Code's multi-layer command safety checks.
func ClassifyCommand(command string) CommandSafetyLevel {
	if ContainsDangerousCommand(command) {
		return CommandDangerous
	}
	if IsCommandReadOnly(command) {
		return CommandSafe
	}
	return CommandNeedsConfirm
}

// splitCommandParts does a simple whitespace split, respecting basic quoting.
// This is a simplified version — production shell parsing is far more complex.
func splitCommandParts(command string) []string {
	var parts []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// extractBaseCommand strips path prefixes to get the bare command name.
// e.g. "/usr/bin/git" → "git", "./run.sh" → "run.sh"
func extractBaseCommand(cmd string) string {
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		return cmd[idx+1:]
	}
	return cmd
}
