package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedSessionPersistenceChannel_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 3, SeedExpectedSessionPersistenceChannelRowCount)
}

func TestSeedSessionPersistenceChannel_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedSessionPersistenceChannelRowCount, len(SeedExpectedSessionPersistenceChannelSlugs))
}

func TestSeedSessionPersistenceChannel_SlugsContainSessionTranscripts(t *testing.T) {
	assert.Contains(t, SeedExpectedSessionPersistenceChannelSlugs, "session_transcripts")
}

func TestSeedSessionPersistenceChannel_SlugsContainGlobalPromptHistory(t *testing.T) {
	assert.Contains(t, SeedExpectedSessionPersistenceChannelSlugs, "global_prompt_history")
}

func TestSeedSessionPersistenceChannel_SlugsContainSubagentSidechains(t *testing.T) {
	assert.Contains(t, SeedExpectedSessionPersistenceChannelSlugs, "subagent_sidechains")
}

func TestSeedSessionPersistenceChannel_TwoAlwaysActiveSlugs(t *testing.T) {
	assert.Equal(t, 2, len(SeedSessionPersistenceAlwaysActiveSlugs))
}

func TestSeedSessionPersistenceChannel_TwoProjectScopedSlugs(t *testing.T) {
	assert.Equal(t, 2, len(SeedSessionPersistenceProjectScopedSlugs))
}

func TestSeedSessionPersistenceChannel_AllAppendOnly(t *testing.T) {
	assert.True(t, SeedSessionPersistenceAllAppendOnly)
}

func TestSeedSessionPersistenceChannel_AlwaysActiveSlugsAreSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedSessionPersistenceChannelSlugs {
		all[s] = true
	}
	for _, s := range SeedSessionPersistenceAlwaysActiveSlugs {
		assert.True(t, all[s], "always-active slug %q not in canonical list", s)
	}
}

func TestSeedSessionPersistenceChannel_SubagentSidechainsIsConditional(t *testing.T) {
	found := false
	for _, s := range SeedSessionPersistenceAlwaysActiveSlugs {
		if s == "subagent_sidechains" {
			found = true
		}
	}
	assert.False(t, found, "subagent_sidechains is conditional (only active when subagents run)")
}
