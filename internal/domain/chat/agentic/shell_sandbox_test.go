package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellSandbox_IsValidKind(t *testing.T) {
	for _, k := range allShellSandboxViolationKinds {
		assert.True(t, IsValidShellSandboxViolationKind(k))
	}
	assert.False(t, IsValidShellSandboxViolationKind(ShellSandboxViolationKind("nope")))
}

func TestShellSandbox_CommandValidate(t *testing.T) {
	good := ShellCommand{Head: "ls"}
	assert.NoError(t, good.Validate())
	bad := ShellCommand{}
	assert.ErrorIs(t, bad.Validate(), ErrShellSandboxHeadRequired)
	whitespace := ShellCommand{Head: "   "}
	assert.ErrorIs(t, whitespace.Validate(), ErrShellSandboxHeadRequired)
}

func TestShellSandbox_PolicyValidateNegativeRuntime(t *testing.T) {
	p := ShellSandboxPolicy{MaxRuntimeSecs: -1}
	assert.ErrorIs(t, p.Validate(), ErrShellSandboxBadPolicy)
}

func TestShellSandbox_PolicyValidateNegativeOutput(t *testing.T) {
	p := ShellSandboxPolicy{MaxOutputBytes: -1}
	assert.ErrorIs(t, p.Validate(), ErrShellSandboxBadPolicy)
}

func TestShellSandbox_PolicyValidateRelativeRootRejected(t *testing.T) {
	p := ShellSandboxPolicy{AllowedPathRoots: []string{"workdir"}}
	assert.ErrorIs(t, p.Validate(), ErrShellSandboxBadPolicy)
}

func TestShellSandbox_NewRejectsBadPolicy(t *testing.T) {
	_, err := NewShellSandbox(ShellSandboxPolicy{MaxRuntimeSecs: -1}, "")
	assert.ErrorIs(t, err, ErrShellSandboxBadPolicy)
}

func TestShellSandbox_NewRejectsRelativeCwd(t *testing.T) {
	_, err := NewShellSandbox(ShellSandboxPolicy{}, "relative/path")
	assert.ErrorIs(t, err, ErrShellSandboxBadPolicy)
}

func TestShellSandbox_EvaluateAllowedCommandReturnsNil(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{AllowNetwork: true}, "")
	v := s.Evaluate(ShellCommand{Head: "ls"})
	assert.Nil(t, v)
}

func TestShellSandbox_EvaluateRejectsEmptyHead(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
	v := s.Evaluate(ShellCommand{})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationBlockedCommand, v.Kind)
}

func TestShellSandbox_BlockedCommandRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		BlockedCommands: []string{"sudo"},
	}, "")
	v := s.Evaluate(ShellCommand{Head: "sudo", Args: []string{"ls"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationBlockedCommand, v.Kind)
	assert.Equal(t, "sudo", v.Offender)
}

func TestShellSandbox_BlockedCommandCaseInsensitive(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		BlockedCommands: []string{"SUDO"},
	}, "")
	v := s.Evaluate(ShellCommand{Head: "sudo"})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationBlockedCommand, v.Kind)
}

func TestShellSandbox_BlockedArgPatternRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		BlockedArgPatterns: []string{"--privileged"},
	}, "")
	v := s.Evaluate(ShellCommand{Head: "docker", Args: []string{"run", "--privileged", "image"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationBlockedArg, v.Kind)
	assert.Equal(t, "--privileged", v.Offender)
}

func TestShellSandbox_BlockedArgCaseInsensitive(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		BlockedArgPatterns: []string{"--no-sandbox"},
	}, "")
	v := s.Evaluate(ShellCommand{Head: "chrome", Args: []string{"--NO-SANDBOX"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationBlockedArg, v.Kind)
}

func TestShellSandbox_PathEscapeRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		AllowedPathRoots: []string{"/workspace"},
	}, "/workspace")
	v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"../etc/passwd"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationPathEscape, v.Kind)
}

func TestShellSandbox_AbsolutePathEscapeRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		AllowedPathRoots: []string{"/workspace"},
	}, "/workspace")
	v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/etc/passwd"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationPathEscape, v.Kind)
}

func TestShellSandbox_PathInsideRootsAllowed(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		AllowedPathRoots: []string{"/workspace"},
	}, "/workspace")
	v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"./README.md"}})
	assert.Nil(t, v)
}

func TestShellSandbox_PathBoundaryNotPrefix(t *testing.T) {
	// "/workspace2/file" must NOT match a /workspace root.
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		AllowedPathRoots: []string{"/workspace"},
	}, "")
	v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/workspace2/file"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationPathEscape, v.Kind)
}

func TestShellSandbox_NonPathArgsSkipped(t *testing.T) {
	// "-la" doesn't look like a path so the path check skips it.
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		AllowedPathRoots: []string{"/workspace"},
	}, "/workspace")
	v := s.Evaluate(ShellCommand{Head: "ls", Args: []string{"-la"}})
	assert.Nil(t, v)
}

func TestShellSandbox_NetworkIntentDenied(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{AllowNetwork: false}, "")
	v := s.Evaluate(ShellCommand{Head: "curl", HasNetworkIntent: true})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationNetwork, v.Kind)
}

func TestShellSandbox_NetworkIntentAllowed(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{AllowNetwork: true}, "")
	v := s.Evaluate(ShellCommand{Head: "curl", HasNetworkIntent: true})
	assert.Nil(t, v)
}

func TestShellSandbox_RuntimeExceeded(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{MaxRuntimeSecs: 60}, "")
	v := s.Evaluate(ShellCommand{Head: "sleep", RequestedRuntimeSecs: 120})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationRuntime, v.Kind)
}

func TestShellSandbox_RuntimeWithinPolicy(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{MaxRuntimeSecs: 60}, "")
	v := s.Evaluate(ShellCommand{Head: "sleep", RequestedRuntimeSecs: 30})
	assert.Nil(t, v)
}

func TestShellSandbox_RuntimeZeroDisablesCheck(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
	v := s.Evaluate(ShellCommand{Head: "x", RequestedRuntimeSecs: 9999})
	assert.Nil(t, v)
}

func TestShellSandbox_OutputExceeded(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{MaxOutputBytes: 1024}, "")
	v := s.Evaluate(ShellCommand{Head: "x", RequestedOutputBytes: 4096})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationOutput, v.Kind)
}

func TestShellSandbox_ShellSubstitutionInHeadRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
	v := s.Evaluate(ShellCommand{Head: "$(whoami)"})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationShellSubstitution, v.Kind)
}

func TestShellSandbox_ShellSubstitutionInArgRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
	v := s.Evaluate(ShellCommand{Head: "echo", Args: []string{"`whoami`"}})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationShellSubstitution, v.Kind)
}

func TestShellSandbox_ShellSubstitutionInStdinRejected(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
	v := s.Evaluate(ShellCommand{Head: "cat", Stdin: "echo $(date)"})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationShellSubstitution, v.Kind)
}

func TestShellSandbox_CheckOrderShellSubstitutionBeforeBlocklist(t *testing.T) {
	// If both shell substitution and blocked command are present,
	// substitution is reported first (it covers the blocklist too).
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		BlockedCommands: []string{"sudo"},
	}, "")
	v := s.Evaluate(ShellCommand{Head: "$(sudo)"})
	require.NotNil(t, v)
	assert.Equal(t, SandboxViolationShellSubstitution, v.Kind)
}

func TestShellSandbox_ViolationString(t *testing.T) {
	v := ShellSandboxViolation{
		Kind: SandboxViolationPathEscape, Detail: "escape", Offender: "../etc",
	}
	got := v.String()
	assert.Contains(t, got, "path_escape")
	assert.Contains(t, got, "escape")
	assert.Contains(t, got, "../etc")
}

func TestShellSandbox_ViolationStringNoOffender(t *testing.T) {
	v := ShellSandboxViolation{Kind: SandboxViolationNetwork, Detail: "no net"}
	got := v.String()
	assert.True(t, strings.HasPrefix(got, "network_denied: "))
	assert.NotContains(t, got, `"`)
}

func TestShellSandbox_PolicyReturnsCopy(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		BlockedCommands: []string{"sudo"},
	}, "")
	p1 := s.Policy()
	p1.BlockedCommands[0] = "tampered"
	p2 := s.Policy()
	assert.Equal(t, "sudo", p2.BlockedCommands[0], "Policy() must return a copy")
}

func TestShellSandbox_NoPathRootsDisablesPathCheck(t *testing.T) {
	// With AllowedPathRoots empty, path-shaped args are NOT checked
	// (caller has opted out of filesystem containment).
	s, _ := NewShellSandbox(ShellSandboxPolicy{}, "")
	v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/etc/passwd"}})
	assert.Nil(t, v)
}

func TestShellSandbox_PathRootCanonicalisationCleansTrailingSlash(t *testing.T) {
	// Policy root "/workspace/" must match "/workspace/file" via canonicalization.
	s, _ := NewShellSandbox(ShellSandboxPolicy{
		AllowedPathRoots: []string{"/workspace/"},
	}, "/workspace")
	v := s.Evaluate(ShellCommand{Head: "cat", Args: []string{"/workspace/file"}})
	assert.Nil(t, v)
}

func TestShellSandbox_OffenderPopulatedOnRuntime(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{MaxRuntimeSecs: 10}, "")
	v := s.Evaluate(ShellCommand{Head: "sleep", RequestedRuntimeSecs: 99})
	require.NotNil(t, v)
	assert.Equal(t, "99", v.Offender)
}

func TestShellSandbox_OffenderPopulatedOnOutput(t *testing.T) {
	s, _ := NewShellSandbox(ShellSandboxPolicy{MaxOutputBytes: 1024}, "")
	v := s.Evaluate(ShellCommand{Head: "x", RequestedOutputBytes: 2048})
	require.NotNil(t, v)
	assert.Equal(t, "2048", v.Offender)
}
