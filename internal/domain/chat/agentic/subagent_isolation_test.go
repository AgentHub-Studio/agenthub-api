package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubagentIsolationModes_ThreeModes(t *testing.T) {
	assert.Equal(t, 3, len(AllSubagentIsolationModes))
}

func TestSubagentIsolationRegistry_Profile_KnownMode(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	p, ok := r.Profile(SubagentIsolationInProcess)
	require.True(t, ok)
	assert.Equal(t, SubagentIsolationInProcess, p.Mode)
	assert.True(t, p.IsWebApplicable)
	assert.True(t, p.IsDefault)
	assert.True(t, p.InheritsParentPermissions)
	assert.False(t, p.RequiresGitWorktree)
}

func TestSubagentIsolationRegistry_Profile_UnknownMode(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	_, ok := r.Profile("not_a_mode")
	assert.False(t, ok)
}

func TestSubagentIsolationRegistry_AllModes_Length(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	assert.Equal(t, 3, len(r.AllModes()))
}

func TestSubagentIsolationRegistry_AllModes_DefensiveCopy(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	modes := r.AllModes()
	original := modes[0]
	modes[0] = "mutated"
	assert.Equal(t, original, r.AllModes()[0])
}

func TestSubagentIsolationRegistry_WebApplicableModes(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	webModes := r.WebApplicableModes()
	assert.Equal(t, 2, len(webModes))
	assert.Contains(t, webModes, SubagentIsolationInProcess)
	assert.Contains(t, webModes, SubagentIsolationRemote)
	assert.NotContains(t, webModes, SubagentIsolationWorktree)
}

func TestSubagentIsolationRegistry_DefaultMode(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	assert.Equal(t, SubagentIsolationInProcess, r.DefaultMode())
}

func TestSubagentIsolationRegistry_BackgroundOnlyModes(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	bg := r.BackgroundOnlyModes()
	// remote and worktree are always background; in_process is not always background
	assert.Equal(t, 2, len(bg))
	assert.Contains(t, bg, SubagentIsolationRemote)
	assert.Contains(t, bg, SubagentIsolationWorktree)
}

func TestSubagentIsolationRegistry_BubbleModeEligibleModes(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	bubble := r.BubbleModeEligibleModes()
	assert.Equal(t, 1, len(bubble))
	assert.Contains(t, bubble, SubagentIsolationInProcess)
}

func TestSubagentIsolationProfiles_WorktreeIsNotWebApplicable(t *testing.T) {
	p := subagentIsolationProfiles[SubagentIsolationWorktree]
	assert.False(t, p.IsWebApplicable, "worktree requires git CLI, not applicable to web platform")
	assert.True(t, p.RequiresGitWorktree)
}

func TestSubagentIsolationProfiles_RemoteAlwaysBackground(t *testing.T) {
	p := subagentIsolationProfiles[SubagentIsolationRemote]
	assert.True(t, p.SupportsBackground)
	assert.True(t, p.IsWebApplicable)
	assert.False(t, p.InheritsParentPermissions)
}

func TestSubagentIsolationProfiles_InProcessInheritsPermissions(t *testing.T) {
	p := subagentIsolationProfiles[SubagentIsolationInProcess]
	assert.True(t, p.InheritsParentPermissions,
		"two-tier scoping: in_process inherits parent session-level rules")
}

func TestSubagentIsolationProfiles_ExactlyOneDefault(t *testing.T) {
	defaults := 0
	for _, mode := range AllSubagentIsolationModes {
		if subagentIsolationProfiles[mode].IsDefault {
			defaults++
		}
	}
	assert.Equal(t, 1, defaults, "exactly one isolation mode must be the default")
}

func TestSubagentIsolationProfiles_AllModesHaveProfiles(t *testing.T) {
	r := NewSubagentIsolationRegistry()
	for _, mode := range AllSubagentIsolationModes {
		_, ok := r.Profile(mode)
		assert.True(t, ok, "mode %q must have a profile", mode)
	}
}
