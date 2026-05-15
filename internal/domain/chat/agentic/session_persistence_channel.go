package agentic

// SessionPersistenceChannel is a typed string identifying one of the three
// independent persistence channels described in §9.1 ("Transcript Model") of
// the Claude Code architecture paper (arXiv:2604.14228v1).
//
// The three channels operate independently: each has its own storage path,
// scope, writer queue, and reader API. No channel merges into another at
// write time; readers compose them as needed (e.g. resume hydrates session
// transcripts, tooling may load sidechain entries for debugging).
//
// PDF quote (§9.1): "Three persistence channels operate independently:
//  1. Session transcripts: Conversation records including user, assistant,
//     attachment, and system messages, plus compaction and other metadata
//     events. Project-scoped, one file per session.
//  2. Global prompt history: User prompts only, stored in history.jsonl at
//     the Claude configuration home directory. Reverse-iterable for
//     Up-arrow / ctrl+r navigation.
//  3. Subagent sidechains: Separate .jsonl + .meta.json files per subagent."
type SessionPersistenceChannel string

const (
	// ChannelSessionTranscript is channel 1: the primary append-only log of a
	// single session — user messages, assistant responses, tool calls/results,
	// system events, and compaction boundary markers.
	// Scoped per-project, one logical file (or DB-backed row sequence) per session.
	ChannelSessionTranscript SessionPersistenceChannel = "session_transcript"

	// ChannelGlobalPromptHistory is channel 2: user-typed prompts only,
	// persisted globally (not per-project) to support reverse-iteration via
	// Up-arrow / ctrl+r navigation across all sessions.
	// Writes are additive; the reader emits entries in reverse-chronological order.
	ChannelGlobalPromptHistory SessionPersistenceChannel = "global_prompt_history"

	// ChannelSubagentSidechain is channel 3: per-subagent isolation — each
	// subagent writes its full transcript to a separate sidechain file.
	// Sidechain entries NEVER inflate the parent session transcript;
	// only the subagent's final summary returns to the parent context
	// (the "context-as-bottleneck" principle from §8.3).
	ChannelSubagentSidechain SessionPersistenceChannel = "subagent_sidechain"
)

// PersistenceChannelScope classifies how broadly a channel's storage is shared.
type PersistenceChannelScope string

const (
	// ScopeSession — storage is bound to a single session (one logical unit
	// per session execution). The most common scope for runtime transcript data.
	ScopeSession PersistenceChannelScope = "session"

	// ScopeGlobal — storage is shared across all sessions and projects on the
	// same installation. Only ChannelGlobalPromptHistory uses this scope.
	ScopeGlobal PersistenceChannelScope = "global"

	// ScopeSubagent — storage is bound to a single subagent invocation.
	// A session may produce many subagent sidechain segments.
	ScopeSubagent PersistenceChannelScope = "subagent"
)

// PersistenceWriteMode classifies the durability contract of a channel.
type PersistenceWriteMode string

const (
	// WriteModeAppendOnly — new records are only ever appended; no update or
	// in-place rewrite occurs under normal operation. This is the standard
	// mode for session transcripts and sidechains (§9.1: "mostly append-only").
	WriteModeAppendOnly PersistenceWriteMode = "append_only"

	// WriteModeAppendWithCleanupRewrite — append-only with a single, explicit
	// exception: a cleanup/compaction rewrite may replace the entire file to
	// remove old tool outputs. The PDF notes this as a deliberate exception
	// to the append-only principle.
	WriteModeAppendWithCleanupRewrite PersistenceWriteMode = "append_with_cleanup_rewrite"
)

// SessionPersistenceChannelProfile describes the static characteristics of
// one persistence channel.
//
// Profiles are immutable and registered in [SessionPersistenceChannelRegistry].
type SessionPersistenceChannelProfile struct {
	// Channel is the canonical identifier.
	Channel SessionPersistenceChannel

	// Scope classifies the storage sharing model for this channel.
	Scope PersistenceChannelScope

	// WriteMode classifies the durability / mutability contract.
	WriteMode PersistenceWriteMode

	// InflatesParentContext reports whether entries from this channel are
	// loaded into the parent session's live context window.
	// Only ChannelSessionTranscript inflates the parent context.
	// ChannelSubagentSidechain explicitly DOES NOT (§8.3 "context-as-bottleneck").
	// ChannelGlobalPromptHistory is not loaded into context at all (it is a
	// navigation-only store).
	InflatesParentContext bool

	// IncludesUserPromptsOnly reports whether this channel is restricted to
	// user-typed prompt text (true only for ChannelGlobalPromptHistory).
	IncludesUserPromptsOnly bool

	// ReverseIterableForNavigation reports whether the channel's reader
	// supports reverse-chronological iteration for shell navigation
	// (Up-arrow / ctrl+r). True only for ChannelGlobalPromptHistory.
	ReverseIterableForNavigation bool

	// SubagentSummaryOnly reports whether only the final summary of a
	// subagent is returned to the parent context (true for
	// ChannelSubagentSidechain — §8.3: "only the subagent's final response
	// text and metadata return to the parent conversation context").
	SubagentSummaryOnly bool

	// Description is a human-readable summary derived from §9.1.
	Description string
}

// sessionPersistenceChannelProfiles is the canonical registry of all three
// §9.1 persistence channels.
var sessionPersistenceChannelProfiles = []SessionPersistenceChannelProfile{
	{
		Channel:                      ChannelSessionTranscript,
		Scope:                        ScopeSession,
		WriteMode:                    WriteModeAppendWithCleanupRewrite,
		InflatesParentContext:         true,
		IncludesUserPromptsOnly:       false,
		ReverseIterableForNavigation: false,
		SubagentSummaryOnly:          false,
		Description: "Primary per-session log: user, assistant, attachments, system events, " +
			"compaction boundaries. Project-scoped; one logical file per session. " +
			"Loaded on resume to rebuild the conversation. Cleanup rewrite is the " +
			"only non-append mutation allowed (§9.1).",
	},
	{
		Channel:                      ChannelGlobalPromptHistory,
		Scope:                        ScopeGlobal,
		WriteMode:                    WriteModeAppendOnly,
		InflatesParentContext:         false,
		IncludesUserPromptsOnly:       true,
		ReverseIterableForNavigation: true,
		SubagentSummaryOnly:          false,
		Description: "User prompts only, persisted globally across all sessions. " +
			"Stored at the Claude configuration home directory. " +
			"Supports reverse-chronological iteration for Up-arrow / ctrl+r " +
			"shell navigation (§9.1).",
	},
	{
		Channel:                      ChannelSubagentSidechain,
		Scope:                        ScopeSubagent,
		WriteMode:                    WriteModeAppendOnly,
		InflatesParentContext:         false,
		IncludesUserPromptsOnly:       false,
		ReverseIterableForNavigation: false,
		SubagentSummaryOnly:          true,
		Description: "Per-subagent isolation: each subagent writes its full transcript to " +
			"a separate sidechain. Sidechain content NEVER enters the parent context " +
			"window (context-as-bottleneck §8.3). Only the subagent's final summary " +
			"returns to the parent conversation context (§8.3 + §9.1).",
	},
}

// SessionPersistenceChannelRegistry provides read-only access to the three
// §9.1 persistence channel profiles.
//
// The registry is the authoritative source for channel metadata; the actual
// storage implementations (SessionIngress, SidechainLog, HistoryLog) are
// separate components that reference these profiles.
type SessionPersistenceChannelRegistry struct {
	profiles []SessionPersistenceChannelProfile
}

// NewSessionPersistenceChannelRegistry returns a registry initialised with the
// three canonical §9.1 persistence channel profiles.
func NewSessionPersistenceChannelRegistry() *SessionPersistenceChannelRegistry {
	cp := make([]SessionPersistenceChannelProfile, len(sessionPersistenceChannelProfiles))
	copy(cp, sessionPersistenceChannelProfiles)
	return &SessionPersistenceChannelRegistry{profiles: cp}
}

// AllChannels returns all three profiles in canonical §9.1 order.
func (r *SessionPersistenceChannelRegistry) AllChannels() []SessionPersistenceChannelProfile {
	result := make([]SessionPersistenceChannelProfile, len(r.profiles))
	copy(result, r.profiles)
	return result
}

// ChannelByID returns the profile whose Channel equals id.
// Returns the zero value and false if id is not recognised.
func (r *SessionPersistenceChannelRegistry) ChannelByID(id SessionPersistenceChannel) (SessionPersistenceChannelProfile, bool) {
	for _, p := range r.profiles {
		if p.Channel == id {
			return p, true
		}
	}
	return SessionPersistenceChannelProfile{}, false
}

// ChannelsByScope returns all profiles whose Scope matches s.
func (r *SessionPersistenceChannelRegistry) ChannelsByScope(s PersistenceChannelScope) []SessionPersistenceChannelProfile {
	var out []SessionPersistenceChannelProfile
	for _, p := range r.profiles {
		if p.Scope == s {
			out = append(out, p)
		}
	}
	return out
}

// ContextInflatingChannels returns profiles that load content into the parent
// session's live context window.
// Under the §9.1 model only ChannelSessionTranscript does this.
func (r *SessionPersistenceChannelRegistry) ContextInflatingChannels() []SessionPersistenceChannelProfile {
	var out []SessionPersistenceChannelProfile
	for _, p := range r.profiles {
		if p.InflatesParentContext {
			out = append(out, p)
		}
	}
	return out
}

// NavigationChannels returns profiles that support reverse-chronological
// iteration for shell history navigation (Up-arrow / ctrl+r).
func (r *SessionPersistenceChannelRegistry) NavigationChannels() []SessionPersistenceChannelProfile {
	var out []SessionPersistenceChannelProfile
	for _, p := range r.profiles {
		if p.ReverseIterableForNavigation {
			out = append(out, p)
		}
	}
	return out
}

// SidechainChannels returns profiles that use subagent sidechain isolation
// (InflatesParentContext == false && SubagentSummaryOnly == true).
func (r *SessionPersistenceChannelRegistry) SidechainChannels() []SessionPersistenceChannelProfile {
	var out []SessionPersistenceChannelProfile
	for _, p := range r.profiles {
		if !p.InflatesParentContext && p.SubagentSummaryOnly {
			out = append(out, p)
		}
	}
	return out
}

// IsValidChannel reports whether id is a recognised SessionPersistenceChannel value.
func (r *SessionPersistenceChannelRegistry) IsValidChannel(id SessionPersistenceChannel) bool {
	for _, p := range r.profiles {
		if p.Channel == id {
			return true
		}
	}
	return false
}

// AppendOnlyChannels returns profiles whose write mode is strictly append-only
// (excluding the cleanup-rewrite exception).
func (r *SessionPersistenceChannelRegistry) AppendOnlyChannels() []SessionPersistenceChannelProfile {
	var out []SessionPersistenceChannelProfile
	for _, p := range r.profiles {
		if p.WriteMode == WriteModeAppendOnly {
			out = append(out, p)
		}
	}
	return out
}
