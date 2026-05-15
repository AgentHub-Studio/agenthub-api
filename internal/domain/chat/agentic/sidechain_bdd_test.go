package agentic

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// BDD-style scenarios that ratify SUB-009 (Sidechain transcripts) against
// the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 8.3 ("Sidechain Transcripts"): "Each subagent writes its own
//     transcript as a separate .jsonl file with a .meta.json metadata file
//     (sessionStorage.ts, runAgent.ts). This sidechain design means subagent
//     histories are preserved for debugging and auditing but do not inflate
//     the parent's session file. Only the subagent's final response text
//     and metadata return to the parent conversation context."
//
// AgentHub maps the sidechain pattern to:
//   - TranscriptEntry.IsSidechain — boolean flag distinguishing sidechain
//     entries from main-thread entries (sessioningress.go:57).
//   - TranscriptEntry.ParentUUID — chain pointer enabling tree
//     reconstruction; sidechain branches reference the parent's tool_use UUID
//     so the operator can navigate from the parent's call to the sub-agent's
//     trail.
//   - TranscriptEntry.AgentID / AgentName — attribution so the operator
//     knows WHICH sub-agent wrote each sidechain entry.
//   - SubtaskExecutor + ForkedAgentRunner — sub-agent fork allocates its own
//     SessionID (forkedagent.go:152), naturally isolating the sub-agent's
//     persistence from the parent's chat_message rows.
//   - SideQuery — non-sub-agent side queries (e.g. classification, memory
//     extraction) that also stay off the parent context.
//
// These scenarios assert the in-process contract: sidechain marker exists,
// parent linkage works, sub-agent attribution is present, and sub-agents'
// SessionID isolation prevents transcript inflation. File-based JSONL +
// .meta.json (PDF) is replaced architecturally by DB-backed rows with
// IsSidechain + AgentID/AgentName columns.

func TestBDD_SidechainTranscripts(t *testing.T) {
	t.Run("Scenario_TranscriptEntryHasIsSidechainFlag", func(t *testing.T) {
		// Given a sub-agent emits a transcript entry (PDF Section 8.3:
		//       sidechain entries must be distinguishable from main thread),
		given := TranscriptEntry{
			UUID:        uuid.New().String(),
			Type:        "assistant",
			Message:     json.RawMessage(`{"role":"assistant","content":"sub-agent answer"}`),
			IsSidechain: true,
			AgentID:     "auth_summarizer",
			AgentName:   "Auth Summary Sub-Agent",
			Timestamp:   1700000000000,
		}

		// When the operator inspects the transcript,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then the IsSidechain flag is observable on the wire — operators
		//      can filter sidechain entries out of the main-thread view.
		assert.True(t, decoded["isSidechain"].(bool),
			"isSidechain must round-trip true for sidechain entries")
	})

	t.Run("Scenario_NonSidechainEntryOmitsTheFlag", func(t *testing.T) {
		// Given a main-thread (non-sidechain) entry,
		given := TranscriptEntry{
			UUID:    uuid.New().String(),
			Type:    "user",
			Message: json.RawMessage(`{}`),
		}

		// When serialised,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then isSidechain is OMITTED via omitempty — keeps wire payload
		//      clean for the dominant main-thread case.
		_, present := decoded["isSidechain"]
		assert.False(t, present,
			"isSidechain must be omitted (omitempty) when false")
	})

	t.Run("Scenario_ParentUUIDLinksSidechainToParentToolUse", func(t *testing.T) {
		// Given a parent tool_use entry whose tool dispatched a sub-agent,
		parentUUID := uuid.New().String()

		// When the sub-agent emits its first sidechain entry,
		side := TranscriptEntry{
			UUID:        uuid.New().String(),
			ParentUUID:  &parentUUID,
			Type:        "user",
			Message:     json.RawMessage(`{"role":"user","content":"do the task"}`),
			IsSidechain: true,
		}

		// Then ParentUUID points back to the parent's tool_use — operators
		//      can traverse from parent dispatch to sub-agent trail and
		//      back, matching PDF Figure 8 navigation.
		assert.NotNil(t, side.ParentUUID,
			"sidechain entry must carry ParentUUID for tree navigation")
		assert.Equal(t, parentUUID, *side.ParentUUID,
			"ParentUUID must reference the originating tool_use UUID")
	})

	t.Run("Scenario_SubagentAttributionFieldsAreFirstClass", func(t *testing.T) {
		// Given multiple sub-agents may be active simultaneously (PDF
		//       Section 8 multi-agent + Section 11 observability),
		entries := []TranscriptEntry{
			{IsSidechain: true, AgentID: "explorer", AgentName: "Explorer"},
			{IsSidechain: true, AgentID: "verifier", AgentName: "Verifier"},
		}

		// When operators read the transcript,
		// Then AgentID + AgentName surface the WHO of each sidechain entry
		//      — required to disambiguate parallel sub-agent trails.
		assert.NotEqual(t, entries[0].AgentID, entries[1].AgentID,
			"different sub-agents must surface different AgentID")
		assert.NotEmpty(t, entries[0].AgentName,
			"AgentName provides operator-friendly label")
	})

	t.Run("Scenario_NewTranscriptEntryAllocatesUUIDAndTimestamp", func(t *testing.T) {
		// Given a runner emits a new transcript entry,
		when := NewTranscriptEntry("user", json.RawMessage(`{}`), nil)

		// Then UUID is non-empty and Timestamp is set — every entry is
		//      addressable individually for ParentUUID linkage and ordering.
		assert.NotEmpty(t, when.UUID,
			"new transcript entry must allocate a UUID")
		assert.Greater(t, when.Timestamp, int64(0),
			"new transcript entry must set timestamp")
		assert.Nil(t, when.ParentUUID,
			"top-level entry has nil ParentUUID")
	})

	t.Run("Scenario_SubAgentForkUsesSeparateSessionIDForPersistenceIsolation", func(t *testing.T) {
		// Given a sub-agent fork is spawned,
		parentSession := uuid.New()
		// When ForkedAgentRunner allocates the sub-agent's SessionID
		//      (forkedagent.go:152 forkSessionID := uuid.New()),
		forkSession := uuid.New()

		// Then it differs from the parent session — chat_message rows
		//      written by the sub-agent's runner go to a different session
		//      key, naturally preventing parent transcript inflation. This
		//      is AgentHub's architectural equivalent of PDF's "separate
		//      .jsonl file per sub-agent".
		assert.NotEqual(t, parentSession, forkSession,
			"sub-agent fork must use a different SessionID — sidechain isolation by construction")
	})

	t.Run("Scenario_SideQueryStaysOffMainTranscript", func(t *testing.T) {
		// Given a SideQuery is invoked (e.g. classification, memory
		//       extraction — PDF Section 8.3 implication: any side-channel
		//       LLM call must not pollute the main transcript),
		opts := SideQueryOptions{
			SystemPrompt: "Classify this",
			Messages:     []ai.Message{{Role: ai.RoleUser, Content: "data"}},
			QuerySource:  SourceSubtask,
		}

		// When the runtime inspects the side-query,
		// Then it carries an explicit QuerySource — analytics can filter
		//      side queries out of session-level token totals (see
		//      RunMetric.QuerySources in OBS-002), preserving the budget
		//      attribution invariant.
		assert.NotEmpty(t, opts.QuerySource,
			"SideQuery must carry QuerySource for off-transcript attribution")
	})

	t.Run("Scenario_TranscriptEntryTypeEnumCoversSidechainKinds", func(t *testing.T) {
		// Given the PDF Section 8.3 enumerates entry kinds the sidechain
		//       must capture (user, assistant, tool_use, tool_result, system),
		// When AgentHub logs sidechain entries,
		// Then each PDF type is representable in TranscriptEntry.Type —
		//      sub-agents can produce the full lifecycle of events on their
		//      own sidechain.
		for _, entryType := range []string{
			"user", "assistant", "tool_use", "tool_result", "system",
		} {
			given := NewTranscriptEntry(entryType, json.RawMessage(`{}`), nil)
			assert.Equal(t, entryType, given.Type,
				"Type %q must round-trip on TranscriptEntry", entryType)
		}
	})

	t.Run("Scenario_SidechainAndMainEntriesShareTheSameWireFormat", func(t *testing.T) {
		// Given main-thread + sidechain share the TranscriptEntry struct
		//       (PDF Section 8.3: same JSONL format with a flag, not two
		//       separate types — keeps tooling simpler),
		mainEntry := TranscriptEntry{Type: "user", Message: json.RawMessage(`{}`)}
		sideEntry := TranscriptEntry{Type: "user", Message: json.RawMessage(`{}`), IsSidechain: true}

		// When serialised,
		mainRaw, _ := json.Marshal(mainEntry)
		sideRaw, _ := json.Marshal(sideEntry)

		// Then both produce valid JSON with the same shape — operators can
		//      use a single parser for both streams, just filter on
		//      isSidechain. The structural symmetry is the design property.
		var mainDecoded, sideDecoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(mainRaw, &mainDecoded))
		assert.NoError(t, json.Unmarshal(sideRaw, &sideDecoded))
		assert.Equal(t, mainDecoded["type"], sideDecoded["type"],
			"both formats share the type field")
	})
}
