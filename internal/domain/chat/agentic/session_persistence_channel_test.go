package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for SessionPersistenceChannelRegistry (FEAT-021).
//
// Reference: arXiv:2604.14228v1 §9.1 ("Transcript Model") — three independent
// persistence channels: session_transcript, global_prompt_history,
// subagent_sidechain.

func TestSessionPersistenceChannelRegistry_AllChannels_ReturnsThree(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.AllChannels()
	assert.Len(t, channels, 3, "§9.1 defines exactly three persistence channels")
}

func TestSessionPersistenceChannelRegistry_AllChannels_ReturnsDefensiveCopy(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	first := r.AllChannels()
	first[0].Description = "MUTATED"
	second := r.AllChannels()
	assert.NotEqual(t, "MUTATED", second[0].Description,
		"AllChannels must return a copy — callers must not mutate registry state")
}

func TestSessionPersistenceChannelRegistry_AllChannels_FirstIsSessionTranscript(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.AllChannels()
	assert.Equal(t, ChannelSessionTranscript, channels[0].Channel,
		"§9.1 lists session transcripts first")
}

func TestSessionPersistenceChannelRegistry_AllChannels_SecondIsGlobalPromptHistory(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.AllChannels()
	assert.Equal(t, ChannelGlobalPromptHistory, channels[1].Channel,
		"§9.1 lists global prompt history second")
}

func TestSessionPersistenceChannelRegistry_AllChannels_ThirdIsSubagentSidechain(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.AllChannels()
	assert.Equal(t, ChannelSubagentSidechain, channels[2].Channel,
		"§9.1 lists subagent sidechains third")
}

func TestSessionPersistenceChannelRegistry_ChannelByID_KnownChannel(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelSessionTranscript)
	require.True(t, ok)
	assert.Equal(t, ChannelSessionTranscript, p.Channel)
}

func TestSessionPersistenceChannelRegistry_ChannelByID_UnknownChannel(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	_, ok := r.ChannelByID(SessionPersistenceChannel("nonexistent"))
	assert.False(t, ok, "unknown channel ID must return false")
}

func TestSessionPersistenceChannelRegistry_SessionTranscript_InflatesParentContext(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelSessionTranscript)
	require.True(t, ok)
	assert.True(t, p.InflatesParentContext,
		"session transcript is the only channel that inflates the parent context window")
}

func TestSessionPersistenceChannelRegistry_SubagentSidechain_DoesNotInflateParentContext(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelSubagentSidechain)
	require.True(t, ok)
	assert.False(t, p.InflatesParentContext,
		"§8.3 context-as-bottleneck: sidechain must NOT inflate parent context")
}

func TestSessionPersistenceChannelRegistry_SubagentSidechain_SubagentSummaryOnly(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelSubagentSidechain)
	require.True(t, ok)
	assert.True(t, p.SubagentSummaryOnly,
		"§8.3: only final summary returns to parent — full sidechain stays isolated")
}

func TestSessionPersistenceChannelRegistry_GlobalPromptHistory_IncludesUserPromptsOnly(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelGlobalPromptHistory)
	require.True(t, ok)
	assert.True(t, p.IncludesUserPromptsOnly,
		"§9.1: global prompt history stores user prompts only")
}

func TestSessionPersistenceChannelRegistry_GlobalPromptHistory_ReverseIterableForNavigation(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelGlobalPromptHistory)
	require.True(t, ok)
	assert.True(t, p.ReverseIterableForNavigation,
		"§9.1: history reader emits entries in reverse order for Up-arrow / ctrl+r")
}

func TestSessionPersistenceChannelRegistry_ContextInflatingChannels_ExactlyOne(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.ContextInflatingChannels()
	require.Len(t, channels, 1,
		"exactly one channel may inflate the parent context window")
	assert.Equal(t, ChannelSessionTranscript, channels[0].Channel)
}

func TestSessionPersistenceChannelRegistry_NavigationChannels_ExactlyOne(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.NavigationChannels()
	require.Len(t, channels, 1,
		"exactly one channel supports reverse navigation")
	assert.Equal(t, ChannelGlobalPromptHistory, channels[0].Channel)
}

func TestSessionPersistenceChannelRegistry_SidechainChannels_ExactlyOne(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.SidechainChannels()
	require.Len(t, channels, 1)
	assert.Equal(t, ChannelSubagentSidechain, channels[0].Channel)
}

func TestSessionPersistenceChannelRegistry_ChannelsByScope_Session(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.ChannelsByScope(ScopeSession)
	require.Len(t, channels, 1)
	assert.Equal(t, ChannelSessionTranscript, channels[0].Channel)
}

func TestSessionPersistenceChannelRegistry_ChannelsByScope_Global(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.ChannelsByScope(ScopeGlobal)
	require.Len(t, channels, 1)
	assert.Equal(t, ChannelGlobalPromptHistory, channels[0].Channel)
}

func TestSessionPersistenceChannelRegistry_ChannelsByScope_Subagent(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	channels := r.ChannelsByScope(ScopeSubagent)
	require.Len(t, channels, 1)
	assert.Equal(t, ChannelSubagentSidechain, channels[0].Channel)
}

func TestSessionPersistenceChannelRegistry_AppendOnlyChannels_TwoChannels(t *testing.T) {
	// §9.1: global prompt history and sidechain are strictly append-only.
	// session transcript allows cleanup rewrites (WriteModeAppendWithCleanupRewrite).
	r := NewSessionPersistenceChannelRegistry()
	channels := r.AppendOnlyChannels()
	assert.Len(t, channels, 2, "two channels are strictly append-only")

	ids := make(map[SessionPersistenceChannel]bool, 2)
	for _, c := range channels {
		ids[c.Channel] = true
	}
	assert.True(t, ids[ChannelGlobalPromptHistory])
	assert.True(t, ids[ChannelSubagentSidechain])
}

func TestSessionPersistenceChannelRegistry_SessionTranscript_AllowsCleanupRewrite(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelSessionTranscript)
	require.True(t, ok)
	assert.Equal(t, WriteModeAppendWithCleanupRewrite, p.WriteMode,
		"§9.1: session transcript is 'mostly append-only' — cleanup rewrite is the explicit exception")
}

func TestSessionPersistenceChannelRegistry_IsValidChannel_AllThreeKnown(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	assert.True(t, r.IsValidChannel(ChannelSessionTranscript))
	assert.True(t, r.IsValidChannel(ChannelGlobalPromptHistory))
	assert.True(t, r.IsValidChannel(ChannelSubagentSidechain))
}

func TestSessionPersistenceChannelRegistry_IsValidChannel_UnknownReturnsFalse(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	assert.False(t, r.IsValidChannel(SessionPersistenceChannel("unknown_channel")))
}

func TestSessionPersistenceChannelRegistry_AllProfilesHaveNonEmptyDescription(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	for _, p := range r.AllChannels() {
		assert.NotEmpty(t, p.Description,
			"every channel profile must carry a human-readable description")
	}
}

func TestSessionPersistenceChannel_ConstantValues(t *testing.T) {
	// Assert the canonical string values used in serialized config/logs.
	assert.Equal(t, SessionPersistenceChannel("session_transcript"), ChannelSessionTranscript)
	assert.Equal(t, SessionPersistenceChannel("global_prompt_history"), ChannelGlobalPromptHistory)
	assert.Equal(t, SessionPersistenceChannel("subagent_sidechain"), ChannelSubagentSidechain)
}

func TestPersistenceChannelScope_ConstantValues(t *testing.T) {
	assert.Equal(t, PersistenceChannelScope("session"), ScopeSession)
	assert.Equal(t, PersistenceChannelScope("global"), ScopeGlobal)
	assert.Equal(t, PersistenceChannelScope("subagent"), ScopeSubagent)
}

func TestPersistenceWriteMode_ConstantValues(t *testing.T) {
	assert.Equal(t, PersistenceWriteMode("append_only"), WriteModeAppendOnly)
	assert.Equal(t, PersistenceWriteMode("append_with_cleanup_rewrite"), WriteModeAppendWithCleanupRewrite)
}

func TestSessionPersistenceChannelRegistry_GlobalPromptHistory_DoesNotInflateContext(t *testing.T) {
	r := NewSessionPersistenceChannelRegistry()
	p, ok := r.ChannelByID(ChannelGlobalPromptHistory)
	require.True(t, ok)
	assert.False(t, p.InflatesParentContext,
		"global prompt history is a navigation store — it is not loaded into session context")
}

func TestSessionPersistenceChannelRegistry_NewRegistryIsIndependentOfPackageState(t *testing.T) {
	r1 := NewSessionPersistenceChannelRegistry()
	r2 := NewSessionPersistenceChannelRegistry()
	ch1 := r1.AllChannels()
	ch2 := r2.AllChannels()
	ch1[0].Description = "MUTATED"
	assert.NotEqual(t, "MUTATED", ch2[0].Description,
		"each registry instance is independent — no shared mutable state")
}
