package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD-style scenarios that ratify PERSIST-011 (Three independent persistence
// channels) against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9.1 ("Transcript Model"): "Three persistence channels operate
//     independently: 1. Session transcripts … 2. Global prompt history …
//     3. Subagent sidechains …"
//   - Section 8.3 ("Sidechain Transcripts"): "The sidechain design means
//     subagent histories are preserved for debugging and auditing but do not
//     inflate the parent's session file."
//
// AgentHub maps these three channels to a typed registry so the runtime can
// reason about channel properties without hard-coding them in control-flow
// scattered across the codebase.

func TestBDD_SessionPersistenceChannels(t *testing.T) {

	t.Run("Scenario_ThreeChannelsOperateIndependently", func(t *testing.T) {
		// Given the §9.1 architecture defines exactly three persistence channels
		//       that operate independently (no channel merges into another at
		//       write time),
		registry := NewSessionPersistenceChannelRegistry()

		// When the registry is queried for all channels,
		channels := registry.AllChannels()

		// Then exactly three channels are present and each has a distinct scope —
		//      confirming independence at the storage-scoping level.
		assert.Len(t, channels, 3, "§9.1 defines exactly three channels")
		scopes := make(map[PersistenceChannelScope]int)
		for _, c := range channels {
			scopes[c.Scope]++
		}
		assert.Len(t, scopes, 3,
			"each of the three channels must occupy a distinct storage scope")
		assert.Equal(t, 1, scopes[ScopeSession])
		assert.Equal(t, 1, scopes[ScopeGlobal])
		assert.Equal(t, 1, scopes[ScopeSubagent])
	})

	t.Run("Scenario_SubagentSidechainDoesNotInflateParentContextWindow", func(t *testing.T) {
		// Given the §8.3 "context-as-bottleneck" principle: subagent histories
		//       are preserved for debugging and auditing but MUST NOT inflate the
		//       parent's session file,
		registry := NewSessionPersistenceChannelRegistry()

		// When a runtime component checks the sidechain channel profile,
		sidechain, ok := registry.ChannelByID(ChannelSubagentSidechain)
		require.True(t, ok, "sidechain channel must be registered")

		// Then:
		//   - InflatesParentContext is false (sidechain entries stay off parent).
		//   - SubagentSummaryOnly is true (only the final summary crosses back).
		assert.False(t, sidechain.InflatesParentContext,
			"§8.3: sidechain MUST NOT inflate the parent context window")
		assert.True(t, sidechain.SubagentSummaryOnly,
			"§8.3: only final response text returns to the parent conversation context")
	})

	t.Run("Scenario_OnlySessionTranscriptLoadedOnResume", func(t *testing.T) {
		// Given a session resume operation (§9.2): the runtime replays the
		//       session transcript to rebuild the conversation — it must NOT
		//       replay global prompt history or subagent sidechains into the
		//       context window,
		registry := NewSessionPersistenceChannelRegistry()

		// When the runtime queries which channels inflate the parent context,
		inflating := registry.ContextInflatingChannels()

		// Then exactly one channel qualifies — ChannelSessionTranscript.
		// This is the structural guarantee that resume is safe (no accidental
		// context pollution from other channels).
		require.Len(t, inflating, 1,
			"resume must only load session transcripts into context")
		assert.Equal(t, ChannelSessionTranscript, inflating[0].Channel)
	})

	t.Run("Scenario_GlobalPromptHistorySupportsReverseNavigationOnly", func(t *testing.T) {
		// Given the §9.1 description of global prompt history: "The
		//       makeHistoryReader() generator yields entries in reverse order via
		//       readLinesReverse(), supporting Up-arrow and ctrl+r navigation",
		registry := NewSessionPersistenceChannelRegistry()

		// When the runtime inspects the global_prompt_history channel,
		gph, ok := registry.ChannelByID(ChannelGlobalPromptHistory)
		require.True(t, ok)

		// Then the channel:
		//   - Stores user prompts only (not full conversation turns).
		//   - Supports reverse-chronological iteration.
		//   - Does NOT inflate the parent context window.
		//   - Has global scope (shared across all sessions and projects).
		assert.True(t, gph.IncludesUserPromptsOnly,
			"§9.1: global prompt history stores user prompts only")
		assert.True(t, gph.ReverseIterableForNavigation,
			"§9.1: reader supports Up-arrow / ctrl+r reverse navigation")
		assert.False(t, gph.InflatesParentContext,
			"global prompt history is a navigation store — never loaded into context")
		assert.Equal(t, ScopeGlobal, gph.Scope,
			"global prompt history is shared across all sessions and projects")
	})

	t.Run("Scenario_SessionTranscriptAllowsCleanupRewriteAsExplicitException", func(t *testing.T) {
		// Given §9.1 characterises the transcript model as "mostly append-only"
		//       with a deliberate exception: "cleanup rewrites as an exception"
		//       (tool output removal during compaction),
		registry := NewSessionPersistenceChannelRegistry()

		// When the runtime checks all three channels' write modes,
		allChannels := registry.AllChannels()
		appendOnlyChannels := registry.AppendOnlyChannels()

		// Then:
		//   - Two channels (global_prompt_history, subagent_sidechain) are
		//     strictly append-only.
		//   - One channel (session_transcript) permits cleanup rewrites.
		assert.Len(t, appendOnlyChannels, 2,
			"global_prompt_history and subagent_sidechain are strictly append-only")
		assert.Len(t, allChannels, 3)

		sessionTranscript, ok := registry.ChannelByID(ChannelSessionTranscript)
		require.True(t, ok)
		assert.Equal(t, WriteModeAppendWithCleanupRewrite, sessionTranscript.WriteMode,
			"§9.1: session transcript is 'mostly append-only' — cleanup rewrite is the exception")
	})

	t.Run("Scenario_AllChannelIDsAreValidAndNonEmpty", func(t *testing.T) {
		// Given the channel constants are used as discriminators in log rows,
		//       metrics labels, and config — they must be stable, non-empty
		//       string values,
		registry := NewSessionPersistenceChannelRegistry()

		// When each channel ID is validated,
		// Then every constant is a non-empty string recognised by the registry,
		//      and no unknown string is accepted.
		knownIDs := []SessionPersistenceChannel{
			ChannelSessionTranscript,
			ChannelGlobalPromptHistory,
			ChannelSubagentSidechain,
		}
		for _, id := range knownIDs {
			assert.NotEmpty(t, string(id), "channel ID must be a non-empty string")
			assert.True(t, registry.IsValidChannel(id),
				"each channel constant must be recognised by the registry")
		}
		assert.False(t, registry.IsValidChannel(""),
			"empty string must not be a valid channel ID")
		assert.False(t, registry.IsValidChannel("unknown_channel"),
			"arbitrary strings must not be valid channel IDs")
	})
}
