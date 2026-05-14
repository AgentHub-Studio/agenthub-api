package agentic

// SessionPersistenceChannelRegistry2 models the complete §9 ("Session
// Persistence and Recovery") persistence model from arXiv:2604.14228v1,
// extending the three §9.1 channels documented in session_persistence_channel.go
// with the §9.2 file-history checkpoint channel and a richer durability profile.
//
// §9 opens: "Session persistence in coding agents involves a design choice
// between append-only logs, structured databases, checkpoint-based snapshots,
// and stateless architectures, each with different trade-offs in auditability,
// query power, and deployment complexity. Claude Code's persistence design
// implements the append-only durable state principle from Table 1."
//
// §9.1 explicitly names three channels that "operate independently":
//
//  1. PersistenceChannelTranscripts — Conversation records including user,
//     assistant, attachment, and system messages, plus compaction and other
//     metadata events. Project-scoped, one file per session. Append-only JSONL.
//  2. PersistenceChannelGlobalHistory — User prompts only, stored in
//     history.jsonl at the Claude configuration home directory (history.ts).
//     makeHistoryReader() yields entries in reverse order via readLinesReverse(),
//     supporting Up-arrow and ctrl+r navigation.
//  3. PersistenceChannelSubagentSidechains — Separate .jsonl + .meta.json
//     files per subagent (§8.3). Subagent histories are preserved for debugging
//     and auditing but do not inflate the parent's session file.
//
// §9.2 ("Resume, Fork, and Not Restoring Permissions") documents a fourth
// channel:
//
//  4. PersistenceChannelFileHistory — File-level checkpoints for --rewind-files,
//     stored at ~/.claude/file-history/<sessionId>/. These are file-level
//     snapshots for reverting filesystem changes, not a generic checkpoint store.
//
// Key §9 / §9.2 invariants encoded in this registry:
//   - Session-scoped permissions are NOT serialised to the transcript; resume
//     rebuilds the permission context from CLI args and disk settings.
//   - Resume (--resume) replays the transcript; Fork (--branch) creates a new
//     session from an existing one. Both do NOT restore session-scoped
//     permissions.
//   - The append-only JSONL format is a deliberate choice favouring auditability
//     and simplicity over query power (§9.1).
//   - Compaction rewrites are the only explicit exception to the append-only rule.
//
// Note on naming: This registry uses "PersistenceChannel2" prefixes to avoid
// collision with the SessionPersistenceChannel type in session_persistence_channel.go
// (which models the same §9.1 channels from a different angle — write-mode focus
// rather than durability/recovery focus). This file provides the durability-first
// four-channel view including §9.2.
//
// Relationships:
//   - session_persistence_channel.go (FEAT-021) models the same §9.1 channels
//     with WriteMode, Scope, InflatesParentContext, ReverseIterableForNavigation.
//   - repl_surface_registry.go (§3.2) documents the four surfaces that all write
//     to PersistenceChannelTranscripts identically.
//   - auto_memory.go (§7) models the compaction pipeline that inserts boundary
//     markers into the session transcript channel.
//   - subagent_spawning_registry.go (§8) documents the subagent isolation model
//     giving rise to PersistenceChannelSubagentSidechains.

// PersistenceChannel2 is the type-safe identifier for a Claude Code session
// persistence channel in the §9 four-channel model (arXiv:2604.14228v1).
//
// This type is distinct from SessionPersistenceChannel (session_persistence_channel.go)
// which models the same three §9.1 channels from a write-mode perspective.
// PersistenceChannel2 adds the §9.2 file-history checkpoint channel and
// captures durability/recovery attributes (SupportsResume, SupportsFork, etc.).
type PersistenceChannel2 string

const (
	// PersistenceChannelTranscripts is the primary session transcript channel.
	// Conversation records (user, assistant, attachment, and system messages)
	// plus compaction markers, file-history snapshots, attribution snapshots,
	// and content-replacement records are stored as append-only JSONL at a
	// project-specific path computed by getTranscriptPath() (sessionStorage.ts).
	// One file per session. The only channel that supports --resume and --branch.
	// §9.1: "Session transcripts: Conversation records including user, assistant,
	// attachment, and system messages, plus compaction and other metadata events.
	// Project-scoped, one file per session."
	PersistenceChannelTranscripts PersistenceChannel2 = "session_transcripts"

	// PersistenceChannelGlobalHistory is the global prompt history channel.
	// Stores user prompts only in history.jsonl at the Claude configuration home
	// directory. makeHistoryReader() yields entries in reverse order via
	// readLinesReverse(), powering Up-arrow and ctrl+r navigation across all
	// sessions.
	// §9.1: "Global prompt history: User prompts only, stored in history.jsonl
	// at the Claude configuration home directory (history.ts)."
	PersistenceChannelGlobalHistory PersistenceChannel2 = "global_prompt_history"

	// PersistenceChannelSubagentSidechains is the subagent sidechain channel.
	// Each subagent writes its own transcript as a separate .jsonl file with a
	// .meta.json metadata file. Histories are preserved for debugging and auditing
	// but do not inflate the parent's session file; the full subagent history
	// never enters the parent's context window (context-as-bottleneck principle).
	// §9.1 (referencing §8.3): "Subagent sidechains: Separate .jsonl +
	// .meta.json files per subagent."
	PersistenceChannelSubagentSidechains PersistenceChannel2 = "subagent_sidechains"

	// PersistenceChannelFileHistory is the file-history checkpoint channel used
	// by --rewind-files. Stored at ~/.claude/file-history/<sessionId>/. These are
	// file-level snapshots for reverting filesystem changes, not a generic
	// checkpoint store. Only active when --rewind-files is enabled.
	// §9.2: "The 'checkpoints' in Claude Code are file-history checkpoints for
	// --rewind-files, stored at ~/.claude/file-history/<sessionId>/."
	PersistenceChannelFileHistory PersistenceChannel2 = "file_history_checkpoints"
)

// PersistenceChannelProfile2 is the immutable §9 durability/recovery metadata
// for one Claude Code session persistence channel from arXiv:2604.14228v1.
//
// This struct is distinct from SessionPersistenceChannelProfile, which captures
// write-mode, scope, and context-inflation properties. PersistenceChannelProfile2
// focuses on durability attributes: restart survival, compaction survival,
// resume/fork support, and automatic vs. opt-in activation.
type PersistenceChannelProfile2 struct {
	// ChannelID is the canonical identifier for this persistence channel.
	ChannelID PersistenceChannel2

	// Label is the short human-readable name for this channel.
	Label string

	// Description explains the channel's role, storage format, and
	// distinguishing characteristics in the Claude Code persistence model.
	Description string

	// PDFSection cites the paper section(s) that describe this channel.
	PDFSection string

	// StorageLayer identifies where on the filesystem this channel stores data.
	StorageLayer string

	// IsAutomatic is true when the harness writes to this channel automatically
	// without a special flag. The three §9.1 channels are automatic; the §9.2
	// file-history channel requires --rewind-files.
	IsAutomatic bool

	// IsUserEditable is true when the channel's artefacts can be meaningfully
	// read or inspected by the user without breaking consistency (plain JSONL).
	IsUserEditable bool

	// IsAppendOnly is true when the channel follows the append-only durable
	// state principle from Table 1. §9.1: compaction rewrites are the only
	// explicit exception for session_transcripts.
	IsAppendOnly bool

	// PersistsAcrossCompaction is true when the channel's data survives a
	// compaction event. All four channels persist across compaction.
	PersistsAcrossCompaction bool

	// PersistsAcrossRestart is true when data written to this channel is
	// available after a process restart or a new --resume session. All
	// disk-backed channels satisfy this; session-scoped permissions do not.
	PersistsAcrossRestart bool

	// SupportsResume is true when this channel participates in the --resume
	// flow. §9.2: --resume replays the transcript (conversationRecovery.ts).
	// Only session_transcripts supports resume.
	SupportsResume bool

	// SupportsFork is true when this channel participates in the --branch
	// (fork) flow. §9.2: fork creates a new session from an existing one.
	// Only session_transcripts supports fork.
	SupportsFork bool

	// MaxRetentionHint describes the scope or lifetime of data in this channel.
	MaxRetentionHint string
}

// persistenceChannelProfiles2 is the package-level seed for all four §9
// persistence channels (three from §9.1, one from §9.2).
var persistenceChannelProfiles2 = []*PersistenceChannelProfile2{
	{
		ChannelID: PersistenceChannelTranscripts,
		Label:     "Session Transcripts",
		Description: "The primary durable record of a Claude Code session. Stored as " +
			"mostly append-only JSONL at a project-specific path computed by " +
			"getTranscriptPath() (sessionStorage.ts) as " +
			"join(projectDir, ${getSessionId()}.jsonl). Includes user, assistant, " +
			"attachment, and system messages plus compaction markers, file-history " +
			"snapshots, attribution snapshots, and content-replacement records. " +
			"The session identity system pairs sessionId with sessionProjectDir, set " +
			"together during resume or branch (§9.1). Compaction rewrites are the " +
			"only explicit exception to the append-only rule (cleanup rewrites in " +
			"sessionStorage.ts). Session-scoped permissions are NOT serialised here; " +
			"resume rebuilds the permission context from CLI args and disk settings " +
			"(§9, §9.2). The append-only JSONL format favours auditability and " +
			"simplicity over query power — database-backed alternatives would enable " +
			"richer queries but introduce deployment dependencies (§9.1).",
		PDFSection:               "9.1",
		StorageLayer:             "project-specific path (join(projectDir, sessionId.jsonl))",
		IsAutomatic:              true,
		IsUserEditable:           true,
		IsAppendOnly:             true,
		PersistsAcrossCompaction: true,
		PersistsAcrossRestart:    true,
		SupportsResume:           true,
		SupportsFork:             true,
		MaxRetentionHint:         "per-session",
	},
	{
		ChannelID: PersistenceChannelGlobalHistory,
		Label:     "Global Prompt History",
		Description: "A cross-session store of user prompts only, written to history.jsonl " +
			"at the Claude configuration home directory. Managed by history.ts. " +
			"makeHistoryReader() is an async generator that yields entries in reverse " +
			"chronological order via readLinesReverse(), directly powering Up-arrow and " +
			"ctrl+r navigation in the interactive REPL (§9.1). Unlike session_transcripts, " +
			"this channel accumulates prompts across all sessions and is scoped globally " +
			"to the user's Claude installation, not to any single project or session. " +
			"Not replayed on --resume or --branch; unaffected by compaction events.",
		PDFSection:               "9.1",
		StorageLayer:             "claude config home (history.jsonl)",
		IsAutomatic:              true,
		IsUserEditable:           true,
		IsAppendOnly:             true,
		PersistsAcrossCompaction: true,
		PersistsAcrossRestart:    true,
		SupportsResume:           false,
		SupportsFork:             false,
		MaxRetentionHint:         "cross-session (global)",
	},
	{
		ChannelID: PersistenceChannelSubagentSidechains,
		Label:     "Subagent Sidechains",
		Description: "Separate .jsonl + .meta.json transcript pairs written by each " +
			"subagent invocation (§8.3, sessionStorage.ts, runAgent.ts). The sidechain " +
			"design means subagent histories are preserved for debugging and auditing " +
			"but do not inflate the parent's session file; the full subagent history " +
			"never enters the parent's context window, respecting the 'context as " +
			"bottleneck' principle. The runAgent() function accepts 21 parameters " +
			"covering agent definition, prompts, permissions, tools, model settings, " +
			"isolation, and callbacks (§8.3). The summary-only return model — where " +
			"only the subagent's final response text and metadata return to the parent " +
			"conversation context — is a deliberate context-conservation choice. " +
			"Isolation mode (worktree / remote / in-process) does not change the " +
			"sidechain's storage structure.",
		PDFSection:               "9.1 (referencing §8.3)",
		StorageLayer:             "project-specific path (per-subagent .jsonl + .meta.json)",
		IsAutomatic:              true,
		IsUserEditable:           false,
		IsAppendOnly:             true,
		PersistsAcrossCompaction: true,
		PersistsAcrossRestart:    true,
		SupportsResume:           false,
		SupportsFork:             false,
		MaxRetentionHint:         "per-subagent",
	},
	{
		ChannelID: PersistenceChannelFileHistory,
		Label:     "File-History Checkpoints",
		Description: "File-level filesystem snapshots stored at " +
			"~/.claude/file-history/<sessionId>/ for the --rewind-files flag. " +
			"These checkpoints are distinct from the session transcript: they capture " +
			"the state of modified files so the user can revert filesystem changes " +
			"to any prior state during an active session (§9.2). The annotateBoundary" +
			"WithPreservedSegment() function (compact.ts) records headUuid, anchorUuid, " +
			"and tailUuid in compact_boundary events; these UUIDs let the session " +
			"loader link messages correctly after rewind. File-history checkpoints are " +
			"NOT a generic checkpoint store and are NOT replayed on --resume or " +
			"--branch; they are solely for filesystem revert within an existing session " +
			"(§9.2). This channel is only active when --rewind-files is enabled; " +
			"normal sessions do not populate it.",
		PDFSection:               "9.2",
		StorageLayer:             "~/.claude/file-history/<sessionId>/",
		IsAutomatic:              false,
		IsUserEditable:           false,
		IsAppendOnly:             false,
		PersistsAcrossCompaction: true,
		PersistsAcrossRestart:    true,
		SupportsResume:           false,
		SupportsFork:             false,
		MaxRetentionHint:         "per-session-rewind",
	},
}

// SeedPersistenceChannel2Count is the number of §9 persistence channels in
// the durability-first four-channel model (three §9.1 + one §9.2).
const SeedPersistenceChannel2Count = 4

// persistenceChannel2ByID is the fast-lookup index built at init time.
var persistenceChannel2ByID map[PersistenceChannel2]*PersistenceChannelProfile2

func init() {
	persistenceChannel2ByID = make(
		map[PersistenceChannel2]*PersistenceChannelProfile2,
		SeedPersistenceChannel2Count,
	)
	for _, p := range persistenceChannelProfiles2 {
		persistenceChannel2ByID[p.ChannelID] = p
	}
}

// SessionPersistenceChannelRegistry2 provides §9 queries over the four Claude
// Code session persistence channels from arXiv:2604.14228v1 (three from §9.1,
// one from §9.2). It focuses on durability and recovery attributes.
//
// For the write-mode and context-inflation view of the three §9.1 channels,
// see SessionPersistenceChannelRegistry in session_persistence_channel.go.
//
// All returned profiles are read-only; callers must not modify them.
type SessionPersistenceChannelRegistry2 struct{}

// NewSessionPersistenceChannelRegistry2 returns a ready-to-use registry of the
// four §9 persistence channels (durability-first model).
func NewSessionPersistenceChannelRegistry2() *SessionPersistenceChannelRegistry2 {
	return &SessionPersistenceChannelRegistry2{}
}

// FindChannelByID looks up a channel profile by its canonical ID.
// Returns (profile, true) if found; (nil, false) if unknown.
func (r *SessionPersistenceChannelRegistry2) FindChannelByID(
	id PersistenceChannel2,
) (*PersistenceChannelProfile2, bool) {
	p, ok := persistenceChannel2ByID[id]
	return p, ok
}

// AllChannels returns all §9 persistence channel profiles.
// Returns a defensive copy of the slice; element order matches the seed order
// (session_transcripts first).
func (r *SessionPersistenceChannelRegistry2) AllChannels() []*PersistenceChannelProfile2 {
	result := make([]*PersistenceChannelProfile2, len(persistenceChannelProfiles2))
	copy(result, persistenceChannelProfiles2)
	return result
}

// Count returns the number of registered §9 persistence channels.
func (r *SessionPersistenceChannelRegistry2) Count() int {
	return len(persistenceChannelProfiles2)
}

// IsValidChannelID returns true if id is one of the canonical §9 channel
// identifiers (three §9.1 + one §9.2).
func (r *SessionPersistenceChannelRegistry2) IsValidChannelID(id PersistenceChannel2) bool {
	_, ok := persistenceChannel2ByID[id]
	return ok
}

// AutomaticChannels returns all channels where IsAutomatic == true.
// §9.1: the harness writes to session_transcripts, global_prompt_history, and
// subagent_sidechains automatically during normal operation. File-history
// checkpoints require the explicit --rewind-files flag and are not automatic.
func (r *SessionPersistenceChannelRegistry2) AutomaticChannels() []*PersistenceChannelProfile2 {
	var result []*PersistenceChannelProfile2
	for _, p := range persistenceChannelProfiles2 {
		if p.IsAutomatic {
			result = append(result, p)
		}
	}
	return result
}

// UserEditableChannels returns all channels where IsUserEditable == true.
// §9.1: session_transcripts and global_prompt_history are human-readable
// JSONL files that users can inspect or modify.
func (r *SessionPersistenceChannelRegistry2) UserEditableChannels() []*PersistenceChannelProfile2 {
	var result []*PersistenceChannelProfile2
	for _, p := range persistenceChannelProfiles2 {
		if p.IsUserEditable {
			result = append(result, p)
		}
	}
	return result
}

// ChannelsPersistingAcrossRestart returns all channels where
// PersistsAcrossRestart == true. §9: all disk-backed channels survive process
// restarts; session-scoped permissions (memory-only) do not.
func (r *SessionPersistenceChannelRegistry2) ChannelsPersistingAcrossRestart() []*PersistenceChannelProfile2 {
	var result []*PersistenceChannelProfile2
	for _, p := range persistenceChannelProfiles2 {
		if p.PersistsAcrossRestart {
			result = append(result, p)
		}
	}
	return result
}

// ChannelsPersistingAcrossCompaction returns all channels where
// PersistsAcrossCompaction == true. §9.1: session_transcripts survive
// compaction (boundaries are marked inline); global_prompt_history and
// subagent_sidechains are unaffected by compaction.
func (r *SessionPersistenceChannelRegistry2) ChannelsPersistingAcrossCompaction() []*PersistenceChannelProfile2 {
	var result []*PersistenceChannelProfile2
	for _, p := range persistenceChannelProfiles2 {
		if p.PersistsAcrossCompaction {
			result = append(result, p)
		}
	}
	return result
}

// ChannelsByStorageLayer returns all channels whose StorageLayer contains the
// given substring. Useful for grouping channels by filesystem location.
func (r *SessionPersistenceChannelRegistry2) ChannelsByStorageLayer(
	layerSubstring string,
) []*PersistenceChannelProfile2 {
	var result []*PersistenceChannelProfile2
	for _, p := range persistenceChannelProfiles2 {
		if persistenceContainsSubstring(p.StorageLayer, layerSubstring) {
			result = append(result, p)
		}
	}
	return result
}

// persistenceContainsSubstring is a simple substring search without importing
// the strings package.
func persistenceContainsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// MostDurableChannel returns the channel profile with the broadest persistence
// scope. §9.2: session_transcripts support resume, fork, compaction survival,
// and restart survival — the highest durability score among all channels.
func (r *SessionPersistenceChannelRegistry2) MostDurableChannel() *PersistenceChannelProfile2 {
	var best *PersistenceChannelProfile2
	bestScore := -1
	for _, p := range persistenceChannelProfiles2 {
		score := 0
		if p.PersistsAcrossRestart {
			score++
		}
		if p.PersistsAcrossCompaction {
			score++
		}
		if p.SupportsResume {
			score++
		}
		if p.SupportsFork {
			score++
		}
		if p.IsAutomatic {
			score++
		}
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}

// --- Structural invariants ---

// AtLeastOneAutomaticPersistenceChannel2 validates that at least one §9
// persistence channel has IsAutomatic == true. The harness must always be
// writing session state to disk; a configuration with no automatic channels
// would violate the "append-only durable state" principle from Table 1.
func AtLeastOneAutomaticPersistenceChannel2() bool {
	for _, p := range persistenceChannelProfiles2 {
		if p.IsAutomatic {
			return true
		}
	}
	return false
}

// UserEditablePersistenceChannels2AreRestartPersistent validates that every
// channel with IsUserEditable == true also has PersistsAcrossRestart == true.
// §9.1: the channels that users can meaningfully read or edit (session
// transcripts, global prompt history) are disk-backed and survive restarts —
// making user edits durable is a prerequisite for editability to be meaningful.
func UserEditablePersistenceChannels2AreRestartPersistent() bool {
	for _, p := range persistenceChannelProfiles2 {
		if p.IsUserEditable && !p.PersistsAcrossRestart {
			return false
		}
	}
	return true
}

// MostDurablePersistenceChannel2IsRestartPersistent validates that the channel
// returned by MostDurableChannel() has PersistsAcrossRestart == true.
// §9: by definition the most durable channel must survive restarts — a channel
// that disappears on restart cannot be the most durable one.
func MostDurablePersistenceChannel2IsRestartPersistent() bool {
	reg := &SessionPersistenceChannelRegistry2{}
	most := reg.MostDurableChannel()
	if most == nil {
		return false
	}
	return most.PersistsAcrossRestart
}
