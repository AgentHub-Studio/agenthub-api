package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for SessionPersistenceChannelRegistry2 — §9 ("Session Persistence
// and Recovery") durability-first four-channel model from arXiv:2604.14228v1.
//
// This registry extends session_persistence_channel.go (which models the three
// §9.1 channels from a write-mode perspective) by adding the §9.2 file-history
// checkpoint channel and capturing durability/recovery attributes.
//
// Four channels:
//   1. session_transcripts    (§9.1) — primary JSONL, one per session
//   2. global_prompt_history  (§9.1) — cross-session prompt store
//   3. subagent_sidechains    (§9.1 / §8.3) — per-subagent .jsonl + .meta.json
//   4. file_history_checkpoints (§9.2) — filesystem snapshots for --rewind-files

// --- Constructor and basic registry ---

func TestSessionPersistenceChannelRegistry2_NewReturnsNonNil(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	assert.NotNil(t, reg, "NewSessionPersistenceChannelRegistry2 must return a non-nil registry")
}

func TestSessionPersistenceChannelRegistry2_CountIsFour(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	assert.Equal(t, 4, reg.Count(),
		"§9 four-channel model: three from §9.1 plus one from §9.2")
}

func TestSessionPersistenceChannelRegistry2_SeedCountConstant(t *testing.T) {
	assert.Equal(t, 4, SeedPersistenceChannel2Count,
		"SeedPersistenceChannel2Count must equal 4")
}

func TestSessionPersistenceChannelRegistry2_AllChannelsLengthMatchesCount(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	all := reg.AllChannels()
	assert.Equal(t, reg.Count(), len(all),
		"AllChannels must return exactly Count() profiles")
}

func TestSessionPersistenceChannelRegistry2_AllChannelsReturnsDefensiveCopy(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	a := reg.AllChannels()
	b := reg.AllChannels()
	assert.NotSame(t, &a, &b, "AllChannels must return a fresh slice on each call")
	assert.Equal(t, len(a), len(b))
}

// --- FindChannelByID ---

func TestSessionPersistenceChannelRegistry2_FindChannelByID_Transcripts(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelTranscripts)
	require.True(t, ok, "session_transcripts channel must exist")
	require.NotNil(t, p)
	assert.Equal(t, PersistenceChannelTranscripts, p.ChannelID)
	assert.Equal(t, "9.1", p.PDFSection)
}

func TestSessionPersistenceChannelRegistry2_FindChannelByID_GlobalHistory(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelGlobalHistory)
	require.True(t, ok, "global_prompt_history channel must exist")
	require.NotNil(t, p)
	assert.Equal(t, PersistenceChannelGlobalHistory, p.ChannelID)
	assert.Equal(t, "9.1", p.PDFSection)
}

func TestSessionPersistenceChannelRegistry2_FindChannelByID_SubagentSidechains(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelSubagentSidechains)
	require.True(t, ok, "subagent_sidechains channel must exist")
	require.NotNil(t, p)
	assert.Equal(t, PersistenceChannelSubagentSidechains, p.ChannelID)
}

func TestSessionPersistenceChannelRegistry2_FindChannelByID_FileHistory(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelFileHistory)
	require.True(t, ok, "file_history_checkpoints channel must exist")
	require.NotNil(t, p)
	assert.Equal(t, PersistenceChannelFileHistory, p.ChannelID)
	assert.Equal(t, "9.2", p.PDFSection,
		"file_history_checkpoints are documented in §9.2, not §9.1")
}

func TestSessionPersistenceChannelRegistry2_FindChannelByID_Unknown(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannel2("nonexistent"))
	assert.False(t, ok, "unknown channel ID must return false")
	assert.Nil(t, p, "unknown channel ID must return nil profile")
}

// --- IsValidChannelID ---

func TestSessionPersistenceChannelRegistry2_IsValidChannelID_AllFourValid(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	ids := []PersistenceChannel2{
		PersistenceChannelTranscripts,
		PersistenceChannelGlobalHistory,
		PersistenceChannelSubagentSidechains,
		PersistenceChannelFileHistory,
	}
	for _, id := range ids {
		assert.True(t, reg.IsValidChannelID(id),
			"IsValidChannelID must return true for canonical ID %q", id)
	}
}

func TestSessionPersistenceChannelRegistry2_IsValidChannelID_EmptyStringInvalid(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	assert.False(t, reg.IsValidChannelID(""),
		"empty string must not be a valid channel ID")
}

// --- AutomaticChannels ---

func TestSessionPersistenceChannelRegistry2_AutomaticChannelsHasThree(t *testing.T) {
	// §9.1 names three channels that operate independently (always automatic).
	// §9.2's file_history_checkpoints require --rewind-files and are not automatic.
	reg := NewSessionPersistenceChannelRegistry2()
	auto := reg.AutomaticChannels()
	assert.Len(t, auto, 3,
		"exactly three of the four channels are automatic (§9.1 core channels)")
}

func TestSessionPersistenceChannelRegistry2_AutomaticChannelsExcludesFileHistory(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	for _, p := range reg.AutomaticChannels() {
		assert.NotEqual(t, PersistenceChannelFileHistory, p.ChannelID,
			"file_history_checkpoints must not be automatic (requires --rewind-files flag)")
	}
}

func TestSessionPersistenceChannelRegistry2_AutomaticChannelsIncludesCoreThree(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	auto := reg.AutomaticChannels()
	ids := make(map[PersistenceChannel2]bool)
	for _, p := range auto {
		ids[p.ChannelID] = true
	}
	assert.True(t, ids[PersistenceChannelTranscripts], "session_transcripts must be automatic")
	assert.True(t, ids[PersistenceChannelGlobalHistory], "global_prompt_history must be automatic")
	assert.True(t, ids[PersistenceChannelSubagentSidechains], "subagent_sidechains must be automatic")
}

// --- UserEditableChannels ---

func TestSessionPersistenceChannelRegistry2_UserEditableChannelsNotEmpty(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	editable := reg.UserEditableChannels()
	assert.NotEmpty(t, editable, "at least one channel must be user-editable")
}

func TestSessionPersistenceChannelRegistry2_UserEditableChannelsAllHaveFlag(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	for _, p := range reg.UserEditableChannels() {
		assert.True(t, p.IsUserEditable,
			"UserEditableChannels must only return channels with IsUserEditable == true")
	}
}

func TestSessionPersistenceChannelRegistry2_UserEditableChannelsAllPersistAcrossRestart(t *testing.T) {
	// Editability is meaningless without restart persistence.
	reg := NewSessionPersistenceChannelRegistry2()
	for _, p := range reg.UserEditableChannels() {
		assert.True(t, p.PersistsAcrossRestart,
			"user-editable channel %q must persist across restarts", p.ChannelID)
	}
}

// --- ChannelsPersistingAcrossRestart ---

func TestSessionPersistenceChannelRegistry2_AllChannelsPersistAcrossRestart(t *testing.T) {
	// §9: all disk-backed channels survive process restarts.
	reg := NewSessionPersistenceChannelRegistry2()
	restartPersistent := reg.ChannelsPersistingAcrossRestart()
	assert.Equal(t, reg.Count(), len(restartPersistent),
		"all four channels must persist across restarts (all are disk-backed)")
}

// --- ChannelsPersistingAcrossCompaction ---

func TestSessionPersistenceChannelRegistry2_AllChannelsPersistAcrossCompaction(t *testing.T) {
	// §9.1: compaction inserts boundary markers but does not delete channels.
	reg := NewSessionPersistenceChannelRegistry2()
	compactionPersistent := reg.ChannelsPersistingAcrossCompaction()
	assert.Equal(t, reg.Count(), len(compactionPersistent),
		"all four channels must persist across compaction events")
}

// --- Resume support ---

func TestSessionPersistenceChannelRegistry2_OnlyTranscriptsSupportsResume(t *testing.T) {
	// §9.2: --resume replays the transcript (conversationRecovery.ts).
	reg := NewSessionPersistenceChannelRegistry2()
	var resumeChannels []PersistenceChannel2
	for _, p := range reg.AllChannels() {
		if p.SupportsResume {
			resumeChannels = append(resumeChannels, p.ChannelID)
		}
	}
	require.Len(t, resumeChannels, 1, "exactly one channel must support --resume")
	assert.Equal(t, PersistenceChannelTranscripts, resumeChannels[0],
		"only session_transcripts supports --resume")
}

// --- Fork support ---

func TestSessionPersistenceChannelRegistry2_OnlyTranscriptsSupportsFork(t *testing.T) {
	// §9.2: --branch (fork) creates a new session from an existing transcript.
	reg := NewSessionPersistenceChannelRegistry2()
	var forkChannels []PersistenceChannel2
	for _, p := range reg.AllChannels() {
		if p.SupportsFork {
			forkChannels = append(forkChannels, p.ChannelID)
		}
	}
	require.Len(t, forkChannels, 1, "exactly one channel must support --branch (fork)")
	assert.Equal(t, PersistenceChannelTranscripts, forkChannels[0],
		"only session_transcripts supports --branch (fork)")
}

// --- Append-only ---

func TestSessionPersistenceChannelRegistry2_TranscriptsIsAppendOnly(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelTranscripts)
	require.True(t, ok)
	assert.True(t, p.IsAppendOnly,
		"session_transcripts must be append-only (§9.1)")
}

func TestSessionPersistenceChannelRegistry2_FileHistoryIsNotAppendOnly(t *testing.T) {
	// File-history checkpoints are filesystem snapshots, not append-only logs.
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelFileHistory)
	require.True(t, ok)
	assert.False(t, p.IsAppendOnly,
		"file_history_checkpoints are snapshots, not append-only logs (§9.2)")
}

// --- MostDurableChannel ---

func TestSessionPersistenceChannelRegistry2_MostDurableChannelIsTranscripts(t *testing.T) {
	// session_transcripts: restart-persistent + compaction-persistent + resume + fork + automatic
	// = highest durability score among all four channels.
	reg := NewSessionPersistenceChannelRegistry2()
	most := reg.MostDurableChannel()
	require.NotNil(t, most, "MostDurableChannel must return a non-nil profile")
	assert.Equal(t, PersistenceChannelTranscripts, most.ChannelID,
		"session_transcripts must be the most durable channel")
}

func TestSessionPersistenceChannelRegistry2_MostDurableChannelPersistsAcrossRestart(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	most := reg.MostDurableChannel()
	require.NotNil(t, most)
	assert.True(t, most.PersistsAcrossRestart,
		"MostDurableChannel must persist across restarts")
}

// --- ChannelsByStorageLayer ---

func TestSessionPersistenceChannelRegistry2_ChannelsByStorageLayer_ConfigHome(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	results := reg.ChannelsByStorageLayer("config home")
	require.NotEmpty(t, results, "at least one channel uses the claude config home")
	found := false
	for _, p := range results {
		if p.ChannelID == PersistenceChannelGlobalHistory {
			found = true
		}
	}
	assert.True(t, found, "global_prompt_history must be found via 'config home' query")
}

func TestSessionPersistenceChannelRegistry2_ChannelsByStorageLayer_EmptySubstring(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	results := reg.ChannelsByStorageLayer("")
	assert.Equal(t, reg.Count(), len(results),
		"empty substring must match all channels")
}

// --- Structural invariants ---

func TestSessionPersistenceChannelRegistry2_InvariantAtLeastOneAutomatic(t *testing.T) {
	assert.True(t, AtLeastOneAutomaticPersistenceChannel2(),
		"AtLeastOneAutomaticPersistenceChannel2 must hold: §9 requires at least one automatic channel")
}

func TestSessionPersistenceChannelRegistry2_InvariantUserEditableAreRestartPersistent(t *testing.T) {
	assert.True(t, UserEditablePersistenceChannels2AreRestartPersistent(),
		"UserEditablePersistenceChannels2AreRestartPersistent must hold")
}

func TestSessionPersistenceChannelRegistry2_InvariantMostDurableIsRestartPersistent(t *testing.T) {
	assert.True(t, MostDurablePersistenceChannel2IsRestartPersistent(),
		"MostDurablePersistenceChannel2IsRestartPersistent must hold")
}

// --- Profile completeness ---

func TestSessionPersistenceChannelRegistry2_AllProfilesHaveNonEmptyFields(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	for _, p := range reg.AllChannels() {
		assert.NotEmpty(t, string(p.ChannelID), "ChannelID must not be empty")
		assert.NotEmpty(t, p.Label, "Label must not be empty for channel %q", p.ChannelID)
		assert.NotEmpty(t, p.Description, "Description must not be empty for channel %q", p.ChannelID)
		assert.NotEmpty(t, p.PDFSection, "PDFSection must not be empty for channel %q", p.ChannelID)
		assert.NotEmpty(t, p.StorageLayer, "StorageLayer must not be empty for channel %q", p.ChannelID)
		assert.NotEmpty(t, p.MaxRetentionHint, "MaxRetentionHint must not be empty for channel %q", p.ChannelID)
	}
}

// --- Permission non-persistence (§9) ---

func TestSessionPersistenceChannelRegistry2_TranscriptDescriptionMentionsPermissions(t *testing.T) {
	// §9: "Session-scoped permissions live in memory only and are not serialized
	// to the transcript." The transcript channel description must reflect this.
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelTranscripts)
	require.True(t, ok)
	assert.Contains(t, p.Description, "permissions",
		"session_transcripts description must reference the permission non-persistence contract (§9)")
}

// --- persistenceContainsSubstring helper ---

func TestPersistenceContainsSubstringHelper(t *testing.T) {
	assert.True(t, persistenceContainsSubstring("hello world", "world"))
	assert.True(t, persistenceContainsSubstring("hello world", "hello"))
	assert.True(t, persistenceContainsSubstring("hello world", ""))
	assert.False(t, persistenceContainsSubstring("hello", "hello world"))
	assert.False(t, persistenceContainsSubstring("hello", "xyz"))
	assert.True(t, persistenceContainsSubstring("abc", "abc"))
	assert.True(t, persistenceContainsSubstring("config home (history.jsonl)", "config home"))
}

// --- File-history is not automatic ---

func TestSessionPersistenceChannelRegistry2_FileHistoryIsNotAutomatic(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelFileHistory)
	require.True(t, ok)
	assert.False(t, p.IsAutomatic,
		"file_history_checkpoints require --rewind-files and must not be automatic")
}

// --- Subagent sidechains are not user-editable ---

func TestSessionPersistenceChannelRegistry2_SubagentSidechainsNotUserEditable(t *testing.T) {
	reg := NewSessionPersistenceChannelRegistry2()
	p, ok := reg.FindChannelByID(PersistenceChannelSubagentSidechains)
	require.True(t, ok)
	assert.False(t, p.IsUserEditable,
		"subagent_sidechains are internal audit artefacts, not directly user-editable")
}
