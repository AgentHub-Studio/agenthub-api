package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for §8.2 subagent isolation modes.

func TestBDD_SubagentIsolationModes(t *testing.T) {
	t.Run("Scenario_ThreeNamedIsolationModesFromSpec", func(t *testing.T) {
		// Given §8.2 defines three named isolation contexts for subagents
		// When all modes are listed
		// Then exactly in_process, remote, and worktree are present
		assert.Equal(t, 3, len(AllSubagentIsolationModes))
		r := NewSubagentIsolationRegistry()
		modes := r.AllModes()
		assert.Contains(t, modes, SubagentIsolationInProcess)
		assert.Contains(t, modes, SubagentIsolationRemote)
		assert.Contains(t, modes, SubagentIsolationWorktree)
	})

	t.Run("Scenario_OnlyTwoModesApplicableToAgentHubWebPlatform", func(t *testing.T) {
		// Given AgentHub is a web platform without local git CLI access
		// When web-applicable modes are filtered
		// Then only in_process and remote are usable; worktree requires git CLI
		r := NewSubagentIsolationRegistry()
		webModes := r.WebApplicableModes()
		assert.Equal(t, 2, len(webModes))
		assert.NotContains(t, webModes, SubagentIsolationWorktree)
	})

	t.Run("Scenario_InProcessIsDefaultAndInheritsPermissions", func(t *testing.T) {
		// Given §8.2 two-tier permission scoping: session-level rules inherited by default
		// When in_process profile is inspected
		// Then it is the default mode and inherits parent session-level permissions
		r := NewSubagentIsolationRegistry()
		assert.Equal(t, SubagentIsolationInProcess, r.DefaultMode())
		p, _ := r.Profile(SubagentIsolationInProcess)
		assert.True(t, p.InheritsParentPermissions)
	})

	t.Run("Scenario_RemoteAlwaysRunsAsBackgroundAgent", func(t *testing.T) {
		// Given §8.2 remote isolation always launches a background agent
		// When remote profile is inspected
		// Then SupportsBackground is true and it does not inherit session permissions
		r := NewSubagentIsolationRegistry()
		p, ok := r.Profile(SubagentIsolationRemote)
		assert.True(t, ok)
		assert.True(t, p.SupportsBackground)
		assert.False(t, p.InheritsParentPermissions)
	})

	t.Run("Scenario_OnlyInProcessIsEligibleForBubbleMode", func(t *testing.T) {
		// Given §8.2 bubble mode escalates permission prompts to the parent terminal
		//      and applies only to in-process async subagents
		// When bubble-mode eligible modes are queried
		// Then only in_process is returned
		r := NewSubagentIsolationRegistry()
		bubble := r.BubbleModeEligibleModes()
		assert.Equal(t, 1, len(bubble))
		assert.Equal(t, SubagentIsolationInProcess, bubble[0])
	})
}
