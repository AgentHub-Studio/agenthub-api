package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ShellSandbox(t *testing.T) {
	t.Run("Scenario_PathEscapeBlockedBeforeShellRuns", func(t *testing.T) {
		// Given a tenant scopes shell access to /workspace,
		// And the LLM proposes `cat /etc/passwd`,
		// When the sandbox evaluates,
		// Then path_escape violation is raised — the process never spawns.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			AllowedPathRoots: []string{"/workspace"},
		}, "/workspace")
		v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/etc/passwd"}})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationPathEscape, v.Kind)
	})

	t.Run("Scenario_ParentRelativeEscapeAlsoBlocked", func(t *testing.T) {
		// Given cwd is /workspace and the LLM tries `cat ../etc/passwd`,
		// When the sandbox canonicalises the path,
		// Then the resolved /etc/passwd is rejected.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			AllowedPathRoots: []string{"/workspace"},
		}, "/workspace")
		v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"../etc/passwd"}})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationPathEscape, v.Kind)
	})

	t.Run("Scenario_BlockedCommandRejectsSudoElevation", func(t *testing.T) {
		// Given the platform never wants `sudo` to be invoked,
		// When the LLM proposes it,
		// Then blocked_command is returned regardless of args.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			BlockedCommands: []string{"sudo"},
		}, "")
		v := s.Evaluate(ShellCommand{Head: "sudo", Args: []string{"ls"}})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationBlockedCommand, v.Kind)
	})

	t.Run("Scenario_DockerPrivilegedFlagBlocked", func(t *testing.T) {
		// Given the tenant disallows `--privileged` docker invocations,
		// When the LLM proposes `docker run --privileged image`,
		// Then blocked_arg is reported with the specific flag.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			BlockedArgPatterns: []string{"--privileged"},
		}, "")
		v := s.Evaluate(ShellCommand{Head: "docker", Args: []string{"run", "--privileged", "alpine"}})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationBlockedArg, v.Kind)
		assert.Equal(t, "--privileged", v.Offender)
	})

	t.Run("Scenario_NetworkIntentBlockedForAirgappedTenant", func(t *testing.T) {
		// Given an air-gapped tenant cannot reach the internet,
		// When the LLM proposes `curl https://example.com`,
		// Then network_denied is raised; the shell tool driver sets
		// HasNetworkIntent based on the head.
		s, _ := NewShellSandbox(ShellSandboxPolicy{AllowNetwork: false}, "")
		v := s.Evaluate(ShellCommand{Head: "curl", HasNetworkIntent: true})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationNetwork, v.Kind)
	})

	t.Run("Scenario_RuntimeAndOutputLimitsRespected", func(t *testing.T) {
		// Given the tenant caps shell calls at 60s and 1MB output,
		// When the LLM declares 120s or 4MB,
		// Then the corresponding violation kind is returned.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			MaxRuntimeSecs: 60, MaxOutputBytes: 1024 * 1024,
		}, "")
		v1 := s.Evaluate(ShellCommand{Head: "sleep", RequestedRuntimeSecs: 120})
		require.NotNil(t, v1)
		assert.Equal(t, SandboxViolationRuntime, v1.Kind)
		v2 := s.Evaluate(ShellCommand{Head: "x", RequestedOutputBytes: 4 * 1024 * 1024})
		require.NotNil(t, v2)
		assert.Equal(t, SandboxViolationOutput, v2.Kind)
	})

	t.Run("Scenario_ShellSubstitutionRefusedNotAnalysed", func(t *testing.T) {
		// Given `$(...)` and backticks can mask anything inside,
		// When the LLM proposes them,
		// Then the sandbox refuses the command entirely — static
		// analysis cannot reason about subshells.
		s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
		v := s.Evaluate(ShellCommand{Head: "echo", Args: []string{"$(whoami)"}})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationShellSubstitution, v.Kind)
	})

	t.Run("Scenario_PrefixCollisionNotAllowed", func(t *testing.T) {
		// Given the allowed root is /workspace,
		// And a path /workspace2/file would look like a string-prefix,
		// When the sandbox evaluates,
		// Then the boundary check rejects it — prefix matching is path-aware.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			AllowedPathRoots: []string{"/workspace"},
		}, "")
		v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/workspace2/file"}})
		require.NotNil(t, v)
		assert.Equal(t, SandboxViolationPathEscape, v.Kind)
	})

	t.Run("Scenario_ViolationFeedsPERM006Builder", func(t *testing.T) {
		// Given a sandbox violation occurred,
		// When the harness builds LLM feedback via PERM-006,
		// Then FromSandboxViolation produces a payload carrying the
		// violation detail so the LLM can adjust input.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			AllowedPathRoots: []string{"/workspace"},
		}, "/workspace")
		v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/etc/shadow"}})
		require.NotNil(t, v)
		b := PermissionDeniedFeedbackBuilder{}
		feedback := b.FromSandboxViolation("Bash", "cat /etc/shadow", v.String())
		assert.Equal(t, PermissionDenialSandboxViolation, feedback.Reason)
		assert.Contains(t, feedback.Explanation, "path_escape")
	})

	t.Run("Scenario_AllowedCommandPassesThroughSilently", func(t *testing.T) {
		// Given a legitimate command stays within policy,
		// When the sandbox evaluates,
		// Then nil is returned — no audit noise.
		s, _ := NewShellSandbox(ShellSandboxPolicy{
			AllowedPathRoots: []string{"/workspace"},
			AllowNetwork:     true,
			MaxRuntimeSecs:   60,
			MaxOutputBytes:   1024 * 1024,
		}, "/workspace")
		v := s.Evaluate(ShellCommand{
			Head: "ls", Args: []string{"-la", "/workspace/src"},
			RequestedRuntimeSecs: 30, RequestedOutputBytes: 4096,
		})
		assert.Nil(t, v)
	})
}
