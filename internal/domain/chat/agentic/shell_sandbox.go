package agentic

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// PERM-008 — Shell sandbox.
//
// PDF arXiv:2604.14228v1 §4 (Permissions and Safety) — when an agent
// is allowed to call a shell tool (Bash/sh/exec), the harness must
// constrain what it can do. Static deny rules (PERM-001/002) catch
// gross matches like "Bash(rm -rf)" but cannot reason about path
// escapes, pipes-to-shell-from-curl, environment-variable injection,
// or runtime/output limits.
//
// This file implements pure-domain sandbox EVALUATION (policy → does
// this command violate?). The actual process spawning, cgroup setup,
// and PID namespacing live in the runtime adapter — out of scope for
// the domain layer.
//
// Distinct from existing AgentHub plumbing:
//   - shellrulematch.go = exact/prefix/wildcard MATCHING utilities for
//     permission rules.
//   - permission.go = the deny/allow/confirm DECISION engine.
//   - shell_sandbox.go (this file) = STRUCTURAL inspection of a parsed
//     command. Produces ShellSandboxViolation when the command would
//     escape the policy. The producer of PERM-006 FromSandboxViolation
//     feedback.

// ShellSandboxViolationKind bounded enum classifies why the sandbox
// rejected a command.
type ShellSandboxViolationKind string

const (
	// SandboxViolationPathEscape — argument references a path outside
	// the allowed roots (`..` or absolute path not in allowlist).
	SandboxViolationPathEscape ShellSandboxViolationKind = "path_escape"
	// SandboxViolationBlockedCommand — the head of the command (token
	// before any args) is on the blocklist.
	SandboxViolationBlockedCommand ShellSandboxViolationKind = "blocked_command"
	// SandboxViolationBlockedArg — an argument matches a blocklist
	// pattern (e.g., `--privileged`, `--no-sandbox`, `; rm`).
	SandboxViolationBlockedArg ShellSandboxViolationKind = "blocked_arg"
	// SandboxViolationNetwork — command attempts a network call and
	// the policy disallows network.
	SandboxViolationNetwork ShellSandboxViolationKind = "network_denied"
	// SandboxViolationRuntime — declared max runtime exceeds the
	// policy ceiling.
	SandboxViolationRuntime ShellSandboxViolationKind = "runtime_exceeded"
	// SandboxViolationOutput — declared max output exceeds policy.
	SandboxViolationOutput ShellSandboxViolationKind = "output_exceeded"
	// SandboxViolationShellSubstitution — command contains shell
	// substitution patterns (`$(...)`, backticks) that the parser
	// cannot statically analyze.
	SandboxViolationShellSubstitution ShellSandboxViolationKind = "shell_substitution"
)

var allShellSandboxViolationKinds = []ShellSandboxViolationKind{
	SandboxViolationPathEscape, SandboxViolationBlockedCommand,
	SandboxViolationBlockedArg, SandboxViolationNetwork,
	SandboxViolationRuntime, SandboxViolationOutput,
	SandboxViolationShellSubstitution,
}

// IsValidShellSandboxViolationKind returns true for the bounded set.
func IsValidShellSandboxViolationKind(k ShellSandboxViolationKind) bool {
	for _, v := range allShellSandboxViolationKinds {
		if k == v {
			return true
		}
	}
	return false
}

// ShellCommand is the parsed input the sandbox inspects.
//
// Head is the program (e.g., "bash", "git", "ls"). Args are tokens
// after the head, in order. Env is the proposed environment overlay.
// Stdin captures the textual stdin (for `cat <<EOF`-style invocations
// that the LLM might pre-build). RequestedRuntimeSecs and
// RequestedOutputBytes are caller-declared limits; the sandbox rejects
// if they exceed the policy ceilings.
type ShellCommand struct {
	Head                 string
	Args                 []string
	Env                  map[string]string
	Stdin                string
	RequestedRuntimeSecs int
	RequestedOutputBytes int
	HasNetworkIntent     bool
}

// Validate enforces the minimal invariants the sandbox relies on.
func (c ShellCommand) Validate() error {
	if strings.TrimSpace(c.Head) == "" {
		return ErrShellSandboxHeadRequired
	}
	return nil
}

// ShellSandboxPolicy declares what is allowed.
//
// AllowedPathRoots — canonical absolute paths agent commands may
// touch. Empty means "no filesystem access" (everything path-shaped
// becomes a violation).
//
// BlockedCommands — exact command heads denied at the structural layer
// (independent of PERM rules; this is a last-resort guard).
//
// BlockedArgPatterns — substring patterns (case-insensitive) checked
// against each Arg.
//
// MaxRuntimeSecs / MaxOutputBytes — declared-limit ceilings. Zero
// disables the check.
//
// AllowNetwork — if false, HasNetworkIntent commands violate.
type ShellSandboxPolicy struct {
	AllowedPathRoots   []string
	BlockedCommands    []string
	BlockedArgPatterns []string
	MaxRuntimeSecs     int
	MaxOutputBytes     int
	AllowNetwork       bool
}

// Validate ensures the policy is internally consistent.
func (p ShellSandboxPolicy) Validate() error {
	if p.MaxRuntimeSecs < 0 {
		return fmt.Errorf("%w: max_runtime_secs negative", ErrShellSandboxBadPolicy)
	}
	if p.MaxOutputBytes < 0 {
		return fmt.Errorf("%w: max_output_bytes negative", ErrShellSandboxBadPolicy)
	}
	for _, root := range p.AllowedPathRoots {
		if !filepath.IsAbs(root) {
			return fmt.Errorf("%w: allowed_path_root %q not absolute", ErrShellSandboxBadPolicy, root)
		}
	}
	return nil
}

// ShellSandboxViolation is the structured rejection record.
type ShellSandboxViolation struct {
	Kind    ShellSandboxViolationKind
	Detail  string
	Offender string // the specific arg/path/pattern that triggered the violation
}

// String renders the violation as a one-line audit string.
func (v ShellSandboxViolation) String() string {
	if v.Offender != "" {
		return fmt.Sprintf("%s: %s (%q)", v.Kind, v.Detail, v.Offender)
	}
	return fmt.Sprintf("%s: %s", v.Kind, v.Detail)
}

// Sentinel errors.
var (
	ErrShellSandboxHeadRequired = errors.New("shell sandbox: command head required")
	ErrShellSandboxBadPolicy    = errors.New("shell sandbox: invalid policy")
)

// shellSubstitutionMarkers catches $(...) and backtick command
// substitution which the sandbox cannot statically analyse.
var shellSubstitutionMarkers = []string{"$(", "`"}

// pathLikeArgRE catches arguments that look like paths (start with /,
// ./, or ../). Used to know when to check against AllowedPathRoots.
func looksLikePath(arg string) bool {
	if arg == "" {
		return false
	}
	if strings.HasPrefix(arg, "/") {
		return true
	}
	if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") {
		return true
	}
	if arg == ".." || arg == "." {
		return true
	}
	return false
}

// canonicalizePath returns an absolute, lexically cleaned path.
// Relative paths are joined against cwd. Returns the cleaned path and
// a bool true when canonicalisation succeeded.
func canonicalizePath(cwd, arg string) (string, bool) {
	if arg == "" {
		return "", false
	}
	candidate := arg
	if !filepath.IsAbs(candidate) {
		if cwd == "" {
			return "", false
		}
		candidate = filepath.Join(cwd, candidate)
	}
	return filepath.Clean(candidate), true
}

// pathWithinRoots returns true if path (already canonical) is under
// any of the canonical roots.
func pathWithinRoots(path string, roots []string) bool {
	for _, root := range roots {
		clean := filepath.Clean(root)
		if path == clean {
			return true
		}
		// Boundary: ensure path is under clean and not just a string-prefix.
		prefix := clean
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// ShellSandbox evaluates commands against a policy.
type ShellSandbox struct {
	policy ShellSandboxPolicy
	cwd    string // canonical working directory for relative path resolution
}

// NewShellSandbox builds a sandbox. Validates the policy eagerly. The
// cwd is required when AllowedPathRoots is non-empty AND the caller
// expects relative-path resolution; pass "" if all checks go against
// absolute paths only.
func NewShellSandbox(policy ShellSandboxPolicy, cwd string) (*ShellSandbox, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if cwd != "" && !filepath.IsAbs(cwd) {
		return nil, fmt.Errorf("%w: cwd %q not absolute", ErrShellSandboxBadPolicy, cwd)
	}
	return &ShellSandbox{policy: policy, cwd: filepath.Clean(cwd)}, nil
}

// Evaluate runs the sandbox checks and returns the FIRST violation
// detected, or nil when the command is allowed.
//
// Check order (deterministic — order matters for audit):
//   1. Validate the command itself.
//   2. Reject shell substitution patterns (we cannot reason about them).
//   3. Blocked command head.
//   4. Blocked arg patterns.
//   5. Path-escape from path-shaped args.
//   6. Network intent vs policy.
//   7. Declared runtime exceeds policy.
//   8. Declared output exceeds policy.
func (s *ShellSandbox) Evaluate(cmd ShellCommand) *ShellSandboxViolation {
	if err := cmd.Validate(); err != nil {
		return &ShellSandboxViolation{
			Kind:   SandboxViolationBlockedCommand,
			Detail: err.Error(),
		}
	}

	// 2. Shell substitution patterns.
	for _, marker := range shellSubstitutionMarkers {
		if strings.Contains(cmd.Head, marker) {
			return &ShellSandboxViolation{
				Kind:     SandboxViolationShellSubstitution,
				Detail:   "command head contains shell substitution",
				Offender: cmd.Head,
			}
		}
	}
	for _, arg := range cmd.Args {
		for _, marker := range shellSubstitutionMarkers {
			if strings.Contains(arg, marker) {
				return &ShellSandboxViolation{
					Kind:     SandboxViolationShellSubstitution,
					Detail:   "argument contains shell substitution",
					Offender: arg,
				}
			}
		}
	}
	for _, marker := range shellSubstitutionMarkers {
		if strings.Contains(cmd.Stdin, marker) {
			return &ShellSandboxViolation{
				Kind:   SandboxViolationShellSubstitution,
				Detail: "stdin contains shell substitution",
			}
		}
	}

	// 3. Blocked command head (case-insensitive exact match).
	headLower := strings.ToLower(cmd.Head)
	for _, blocked := range s.policy.BlockedCommands {
		if strings.ToLower(blocked) == headLower {
			return &ShellSandboxViolation{
				Kind:     SandboxViolationBlockedCommand,
				Detail:   "command head is on blocklist",
				Offender: cmd.Head,
			}
		}
	}

	// 4. Blocked arg patterns (case-insensitive substring).
	for _, arg := range cmd.Args {
		argLower := strings.ToLower(arg)
		for _, pattern := range s.policy.BlockedArgPatterns {
			if pattern == "" {
				continue
			}
			if strings.Contains(argLower, strings.ToLower(pattern)) {
				return &ShellSandboxViolation{
					Kind:     SandboxViolationBlockedArg,
					Detail:   fmt.Sprintf("argument matches blocked pattern %q", pattern),
					Offender: arg,
				}
			}
		}
	}

	// 5. Path-escape detection.
	if len(s.policy.AllowedPathRoots) > 0 {
		for _, arg := range cmd.Args {
			if !looksLikePath(arg) {
				continue
			}
			clean, ok := canonicalizePath(s.cwd, arg)
			if !ok {
				continue
			}
			if !pathWithinRoots(clean, s.policy.AllowedPathRoots) {
				return &ShellSandboxViolation{
					Kind:     SandboxViolationPathEscape,
					Detail:   "argument resolves outside allowed roots",
					Offender: arg,
				}
			}
		}
	}

	// 6. Network intent.
	if cmd.HasNetworkIntent && !s.policy.AllowNetwork {
		return &ShellSandboxViolation{
			Kind:   SandboxViolationNetwork,
			Detail: "command declares network intent but policy denies it",
		}
	}

	// 7. Runtime ceiling.
	if s.policy.MaxRuntimeSecs > 0 && cmd.RequestedRuntimeSecs > s.policy.MaxRuntimeSecs {
		return &ShellSandboxViolation{
			Kind:     SandboxViolationRuntime,
			Detail:   fmt.Sprintf("requested %ds exceeds policy ceiling %ds", cmd.RequestedRuntimeSecs, s.policy.MaxRuntimeSecs),
			Offender: fmt.Sprintf("%d", cmd.RequestedRuntimeSecs),
		}
	}

	// 8. Output ceiling.
	if s.policy.MaxOutputBytes > 0 && cmd.RequestedOutputBytes > s.policy.MaxOutputBytes {
		return &ShellSandboxViolation{
			Kind:     SandboxViolationOutput,
			Detail:   fmt.Sprintf("requested %dB exceeds policy ceiling %dB", cmd.RequestedOutputBytes, s.policy.MaxOutputBytes),
			Offender: fmt.Sprintf("%d", cmd.RequestedOutputBytes),
		}
	}

	return nil
}

// Policy returns a copy of the active policy. Useful for audit emission.
func (s *ShellSandbox) Policy() ShellSandboxPolicy {
	roots := append([]string(nil), s.policy.AllowedPathRoots...)
	blockedCmds := append([]string(nil), s.policy.BlockedCommands...)
	blockedArgs := append([]string(nil), s.policy.BlockedArgPatterns...)
	return ShellSandboxPolicy{
		AllowedPathRoots:   roots,
		BlockedCommands:    blockedCmds,
		BlockedArgPatterns: blockedArgs,
		MaxRuntimeSecs:     s.policy.MaxRuntimeSecs,
		MaxOutputBytes:     s.policy.MaxOutputBytes,
		AllowNetwork:       s.policy.AllowNetwork,
	}
}
