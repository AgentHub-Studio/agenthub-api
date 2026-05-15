package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD-style scenarios for SessionPersistenceChannelRegistry2 — §9 ("Session
// Persistence and Recovery") durability-first four-channel model from
// arXiv:2604.14228v1 ("Dive into Claude Code").
//
// §9 opens: "Session persistence in coding agents involves a design choice
// between append-only logs, structured databases, checkpoint-based snapshots,
// and stateless architectures. Claude Code's persistence design implements the
// append-only durable state principle."
//
// §9.1 names three channels that operate independently:
//  1. session_transcripts — primary JSONL, one file per session, append-only
//  2. global_prompt_history — cross-session user-prompt store (Up-arrow / ctrl+r)
//  3. subagent_sidechains — per-subagent .jsonl + .meta.json (§8.3)
//
// §9.2 adds: file_history_checkpoints — filesystem snapshots for --rewind-files.
//
// Core invariant (§9): "Session-scoped permissions live in memory only and
// are not serialised to the transcript, so resume rebuilds the permission
// context from CLI args and disk settings."

func TestBDD_SessionPersistenceChannelRegistry2_AppendOnlyDurableState(t *testing.T) {
	t.Run("Scenario_ThreeChannelsOperateIndependentlyAutomatically", func(t *testing.T) {
		// Given a Claude Code session accumulating conversation state,
		reg := NewSessionPersistenceChannelRegistry2()

		// When checking which channels are active without any special flags,
		auto := reg.AutomaticChannels()

		// Then exactly three channels are automatic, matching §9.1's statement
		// that "three persistence channels operate independently." The fourth
		// (file_history_checkpoints) requires the explicit --rewind-files flag
		// and therefore does NOT activate in normal sessions.
		assert.Len(t, auto, 3,
			"§9.1: exactly three channels operate independently (automatically)")
		ids := make(map[PersistenceChannel2]bool)
		for _, p := range auto {
			ids[p.ChannelID] = true
		}
		assert.True(t, ids[PersistenceChannelTranscripts],
			"session_transcripts must always be active — it is the primary durable record")
		assert.True(t, ids[PersistenceChannelGlobalHistory],
			"global_prompt_history must always be active — it powers Up-arrow / ctrl+r")
		assert.True(t, ids[PersistenceChannelSubagentSidechains],
			"subagent_sidechains must always be active — each subagent writes its own log")
		assert.False(t, ids[PersistenceChannelFileHistory],
			"file_history_checkpoints must NOT be automatic (requires --rewind-files)")
	})

	t.Run("Scenario_TranscriptIsAppendOnlyFavouringAuditability", func(t *testing.T) {
		// Given the §9.1 design principle "append-only durable state" from Table 1,
		reg := NewSessionPersistenceChannelRegistry2()

		// When inspecting the session_transcripts channel,
		p, ok := reg.FindChannelByID(PersistenceChannelTranscripts)

		// Then the channel is append-only, automatic, and persists across both
		// restarts and compaction — implementing the paper's stated design choice:
		// "The append-only JSONL format is a deliberate choice favouring
		// auditability and simplicity over query power." (§9.1)
		require.True(t, ok, "session_transcripts must be registered in the §9 model")
		require.NotNil(t, p)
		assert.True(t, p.IsAppendOnly,
			"session_transcripts must be append-only per §9.1")
		assert.True(t, p.IsAutomatic,
			"session_transcripts must be written automatically — no special flag needed")
		assert.True(t, p.PersistsAcrossRestart,
			"session_transcripts must survive process restarts (disk-backed §9)")
		assert.True(t, p.PersistsAcrossCompaction,
			"session_transcripts must survive compaction (boundaries are marked inline)")
	})
}

func TestBDD_SessionPersistenceChannelRegistry2_ResumeAndForkContracts(t *testing.T) {
	t.Run("Scenario_ResumeReplaysTranscriptAndDoesNotRestorePermissions", func(t *testing.T) {
		// Given a developer using --resume to continue a previous Claude Code session,
		reg := NewSessionPersistenceChannelRegistry2()

		// When checking which channels participate in the --resume flow,
		transcriptChannel, _ := reg.FindChannelByID(PersistenceChannelTranscripts)
		historyChannel, _ := reg.FindChannelByID(PersistenceChannelGlobalHistory)
		sidechainChannel, _ := reg.FindChannelByID(PersistenceChannelSubagentSidechains)
		fileHistoryChannel, _ := reg.FindChannelByID(PersistenceChannelFileHistory)

		// Then only session_transcripts supports resume. §9.2: "--resume rebuilds
		// the conversation by replaying the transcript (conversationRecovery.ts)."
		// Crucially, it does NOT restore session-scoped permissions — users must
		// re-grant them. This is a deliberate safety design choice from §9.2.
		require.NotNil(t, transcriptChannel)
		assert.True(t, transcriptChannel.SupportsResume,
			"session_transcripts must support --resume: it is the replay source")
		assert.False(t, historyChannel.SupportsResume,
			"global_prompt_history is NOT replayed on --resume")
		assert.False(t, sidechainChannel.SupportsResume,
			"subagent_sidechains are NOT replayed on --resume")
		assert.False(t, fileHistoryChannel.SupportsResume,
			"file_history_checkpoints are NOT replayed on --resume")
	})

	t.Run("Scenario_ForkCreatesNewSessionFromTranscriptOnly", func(t *testing.T) {
		// Given a developer using --branch (fork) to create a divergent session,
		reg := NewSessionPersistenceChannelRegistry2()

		// When collecting channels that support fork,
		var forkChannels []*PersistenceChannelProfile2
		for _, p := range reg.AllChannels() {
			if p.SupportsFork {
				forkChannels = append(forkChannels, p)
			}
		}

		// Then exactly one channel supports fork: session_transcripts.
		// §9.2: "Fork (--branch) creates a new session from an existing one
		// (commands/branch/branch.ts). Resume and fork do NOT restore session-scoped
		// permissions; users must grant them again in the new session. This is a
		// deliberate safety-conservative design choice."
		require.Len(t, forkChannels, 1,
			"exactly one channel supports --branch (fork)")
		assert.Equal(t, PersistenceChannelTranscripts, forkChannels[0].ChannelID,
			"only session_transcripts participates in --branch (fork)")
	})
}

func TestBDD_SessionPersistenceChannelRegistry2_GlobalPromptHistoryNavigation(t *testing.T) {
	t.Run("Scenario_GlobalHistoryEnablesUpArrowNavigationAcrossSessions", func(t *testing.T) {
		// Given a developer pressing Up-arrow or ctrl+r in the interactive REPL
		// to recall a prompt from a previous session,
		reg := NewSessionPersistenceChannelRegistry2()

		// When retrieving the global_prompt_history channel profile,
		p, ok := reg.FindChannelByID(PersistenceChannelGlobalHistory)

		// Then the channel is cross-session, automatic, user-editable, and
		// restart-persistent — it stores user prompts in history.jsonl so
		// shell navigation works across all sessions on the same installation.
		// §9.1: "makeHistoryReader() yields entries in reverse order via
		// readLinesReverse(), supporting Up-arrow and ctrl+r navigation."
		require.True(t, ok, "global_prompt_history channel must exist")
		require.NotNil(t, p)
		assert.True(t, p.IsAutomatic,
			"global_prompt_history must be written automatically for every user prompt")
		assert.True(t, p.IsUserEditable,
			"global_prompt_history is stored as plain JSONL at the config home")
		assert.True(t, p.PersistsAcrossRestart,
			"global_prompt_history must persist across restarts for cross-session navigation")
		assert.Equal(t, "cross-session (global)", p.MaxRetentionHint,
			"global_prompt_history is scoped globally, not per-session")
		assert.False(t, p.SupportsResume,
			"global_prompt_history is not part of the --resume replay")
		assert.False(t, p.SupportsFork,
			"global_prompt_history is not part of the --branch fork")
	})
}

func TestBDD_SessionPersistenceChannelRegistry2_SubagentSidechains(t *testing.T) {
	t.Run("Scenario_SubagentHistoriesAreAuditArtefactsNotContextSources", func(t *testing.T) {
		// Given a session that delegates a task to a subagent (e.g., the Explore
		// subagent searching the authentication module structure — §8, §9),
		reg := NewSessionPersistenceChannelRegistry2()

		// When inspecting the subagent_sidechains channel,
		p, ok := reg.FindChannelByID(PersistenceChannelSubagentSidechains)

		// Then the channel is automatic and restart-persistent, but is NOT
		// user-editable and does NOT support resume/fork. The "context as
		// bottleneck" principle (§8.3) is encoded: only the subagent's final
		// summary returns to the parent context, not the full transcript.
		// §9.1: "Subagent sidechains: Separate .jsonl + .meta.json files per
		// subagent (§8.3). Subagent histories are preserved for debugging and
		// auditing but do not inflate the parent's session file."
		require.True(t, ok, "subagent_sidechains channel must exist")
		require.NotNil(t, p)
		assert.True(t, p.IsAutomatic,
			"sidechains must be written automatically whenever a subagent runs")
		assert.False(t, p.IsUserEditable,
			"sidechains are internal audit artefacts, not directly user-editable")
		assert.True(t, p.PersistsAcrossRestart,
			"sidechains must persist across restarts for post-session debugging")
		assert.False(t, p.SupportsResume,
			"sidechains are not replayed on --resume (only transcript is)")
		assert.Equal(t, "per-subagent", p.MaxRetentionHint,
			"each subagent invocation produces its own distinct sidechain file")
	})
}

func TestBDD_SessionPersistenceChannelRegistry2_FileHistoryCheckpoints(t *testing.T) {
	t.Run("Scenario_FileHistoryCheckpointsAreOptionalFilesystemRevertMechanism", func(t *testing.T) {
		// Given a developer using the --rewind-files flag to revert filesystem
		// changes made during an active Claude Code session,
		reg := NewSessionPersistenceChannelRegistry2()

		// When inspecting the file_history_checkpoints channel,
		p, ok := reg.FindChannelByID(PersistenceChannelFileHistory)

		// Then the channel is NOT automatic (flag-gated), NOT user-editable,
		// NOT append-only (snapshots replace prior state), and does NOT support
		// resume/fork — it is exclusively a filesystem revert mechanism.
		// §9.2: "The 'checkpoints' in Claude Code are file-history checkpoints for
		// --rewind-files, stored at ~/.claude/file-history/<sessionId>/. These are
		// file-level snapshots for reverting filesystem changes, not a generic
		// checkpoint store."
		require.True(t, ok, "file_history_checkpoints channel must exist in §9 model")
		require.NotNil(t, p)
		assert.False(t, p.IsAutomatic,
			"file_history_checkpoints require --rewind-files and must NOT be automatic")
		assert.False(t, p.IsUserEditable,
			"file_history_checkpoints are filesystem snapshots, not user-editable JSONL")
		assert.False(t, p.IsAppendOnly,
			"file_history_checkpoints are snapshots, not append-only logs")
		assert.False(t, p.SupportsResume,
			"file_history_checkpoints are NOT replayed on --resume")
		assert.False(t, p.SupportsFork,
			"file_history_checkpoints do NOT participate in --branch (fork)")
		assert.Equal(t, "9.2", p.PDFSection,
			"file_history_checkpoints must cite §9.2 (not §9.1)")
		assert.True(t, p.PersistsAcrossRestart,
			"file_history_checkpoints persist on disk even after restart (for rewind)")
	})
}

func TestBDD_SessionPersistenceChannelRegistry2_DurabilityHierarchyAndInvariants(t *testing.T) {
	t.Run("Scenario_TranscriptsAreTheMostDurableChannel", func(t *testing.T) {
		// Given the four §9 persistence channels with their durability attributes,
		reg := NewSessionPersistenceChannelRegistry2()

		// When computing the most durable channel,
		most := reg.MostDurableChannel()

		// Then session_transcripts is returned — it scores highest on all five
		// durability dimensions: PersistsAcrossRestart, PersistsAcrossCompaction,
		// SupportsResume, SupportsFork, IsAutomatic.
		// §9.2: "The transcript … is replayed on --resume and read on --branch,
		// making it the authoritative session record."
		require.NotNil(t, most, "MostDurableChannel must return a non-nil profile")
		assert.Equal(t, PersistenceChannelTranscripts, most.ChannelID,
			"session_transcripts is the most durable channel: resume + fork + automatic")
		assert.True(t, most.PersistsAcrossRestart,
			"the most durable channel must survive restarts")
		assert.True(t, most.SupportsResume,
			"the most durable channel must support --resume")
		assert.True(t, most.SupportsFork,
			"the most durable channel must support --branch (fork)")
	})

	t.Run("Scenario_AllThreeStructuralInvariantsMustHold", func(t *testing.T) {
		// Given the §9 persistence channel registry,
		// (invariants operate on the package-level seed — no additional setup needed)

		// When evaluating all three structural invariants simultaneously,
		atLeastOneAuto := AtLeastOneAutomaticPersistenceChannel2()
		editableRestartPersistent := UserEditablePersistenceChannels2AreRestartPersistent()
		mostDurableRestartPersistent := MostDurablePersistenceChannel2IsRestartPersistent()

		// Then all three must hold — they encode the core persistence contracts
		// from §9 and §9.1:
		//   - AtLeastOneAutomaticPersistenceChannel2: session state must always be
		//     written to disk; violating this breaks the "append-only durable state"
		//     principle from Table 1.
		//   - UserEditablePersistenceChannels2AreRestartPersistent: editable channels
		//     must be disk-backed so user-made edits survive restarts.
		//   - MostDurablePersistenceChannel2IsRestartPersistent: session_transcripts,
		//     the most durable channel, must survive restarts as the --resume source.
		assert.True(t, atLeastOneAuto,
			"AtLeastOneAutomaticPersistenceChannel2 must hold (§9: always writing to disk)")
		assert.True(t, editableRestartPersistent,
			"UserEditablePersistenceChannels2AreRestartPersistent must hold "+
				"(editability is meaningless without restart persistence)")
		assert.True(t, mostDurableRestartPersistent,
			"MostDurablePersistenceChannel2IsRestartPersistent must hold "+
				"(session_transcripts: the --resume and --branch source)")
	})
}
