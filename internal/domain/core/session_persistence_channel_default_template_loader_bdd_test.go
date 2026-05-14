package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for session_persistence_channel_template seed.
// Maps §9.1 three-channel persistence model to ah_core default templates.

func TestBDD_AhCoreSessionPersistenceChannelSeed(t *testing.T) {
	t.Run("Scenario_ThreeChannelsCoverAllSessionStateFromSpec", func(t *testing.T) {
		// Given §9.1 specifies three persistence channels for session state
		// When the platform loads channel presets
		// Then exactly session_transcripts, global_prompt_history, subagent_sidechains exist
		assert.Equal(t, 3, SeedExpectedSessionPersistenceChannelRowCount)
		assert.Contains(t, SeedExpectedSessionPersistenceChannelSlugs, "session_transcripts")
		assert.Contains(t, SeedExpectedSessionPersistenceChannelSlugs, "global_prompt_history")
		assert.Contains(t, SeedExpectedSessionPersistenceChannelSlugs, "subagent_sidechains")
	})

	t.Run("Scenario_AllChannelsAreAppendOnly", func(t *testing.T) {
		// Given §9.1 uses mostly-append design: compaction never mutates prior entries
		// When the append-only invariant is checked
		// Then all three channels are append-only
		assert.True(t, SeedSessionPersistenceAllAppendOnly)
	})

	t.Run("Scenario_TwoChannelsAlwaysActiveOneConditional", func(t *testing.T) {
		// Given subagent_sidechains only exist when subagents are invoked
		// When always-active channels are counted
		// Then session_transcripts and global_prompt_history are always active
		assert.Equal(t, 2, len(SeedSessionPersistenceAlwaysActiveSlugs))
		assert.Contains(t, SeedSessionPersistenceAlwaysActiveSlugs, "session_transcripts")
		assert.Contains(t, SeedSessionPersistenceAlwaysActiveSlugs, "global_prompt_history")
	})

	t.Run("Scenario_GlobalPromptHistoryIsNotProjectScoped", func(t *testing.T) {
		// Given global_prompt_history is shared across sessions for a user (not per-project)
		// When project-scoped channels are checked
		// Then global_prompt_history is not project-scoped
		found := false
		for _, s := range SeedSessionPersistenceProjectScopedSlugs {
			if s == "global_prompt_history" {
				found = true
			}
		}
		assert.False(t, found, "global_prompt_history must not be project-scoped")
	})

	t.Run("Scenario_SubagentSidechainsAreProjectScopedButConditional", func(t *testing.T) {
		// Given sidechains are scoped to a project session but only exist when subagents run
		// When the project-scoped list is checked
		// Then subagent_sidechains appears there but not in always-active list
		assert.Contains(t, SeedSessionPersistenceProjectScopedSlugs, "subagent_sidechains")
		inAlwaysActive := false
		for _, s := range SeedSessionPersistenceAlwaysActiveSlugs {
			if s == "subagent_sidechains" {
				inAlwaysActive = true
			}
		}
		assert.False(t, inAlwaysActive)
	})
}
