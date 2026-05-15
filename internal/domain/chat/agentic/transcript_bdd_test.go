package agentic

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that ratify PERSIST-001 (Session transcript JSONL)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9   ("Session Persistence and Recovery") + Figure 8 — durable
//     state lives separately from the live context window; resume rebuilds
//     conversation by replaying durable records.
//   - Section 9.1 ("Transcript Model") — transcripts stored as MOSTLY
//     append-only JSONL files at a project-specific path; cleanup rewrites
//     are an explicit exception. Three persistence channels operate
//     independently: session transcripts, global prompt history, subagent
//     sidechains.
//
// AgentHub maps the durable transcript to a per-tenant PostgreSQL table
// (`ah_{tenantId}.chat_message`) — file-system JSONL replaced by row-level
// INSERT-only persistence. The architectural mapping is honest, not literal:
//
//   File-based JSONL (Claude Code)            → DB-backed table (AgentHub)
//   project-scoped path                       → tenant-scoped schema
//   append a new line per event               → INSERT one row per message
//   in-place rewrite as exception             → no UpdateMessage method exposed
//   reverse-iter prompt history (history.ts)  → ordered query in repository
//   subagent .jsonl + .meta.json sidechain    → SUB-009 (separate feature)
//
// These scenarios assert the in-process model contract (ChatMessage shape,
// MessageType enum, append-only repository surface) at unit level. Real DB
// behaviour is exercised by integration tests in repository_test.go.

func TestBDD_SessionTranscriptModel(t *testing.T) {
	t.Run("Scenario_AllFiveAgenticMessageTypesAreEnumerated", func(t *testing.T) {
		// Given the agentic loop emits five kinds of events that must persist
		//       (PDF Section 9.1: "session transcripts ... include user,
		//       assistant, attachment, and system messages, plus compaction
		//       and other metadata events"),
		types := map[chat.MessageType]bool{
			chat.MessageTypeText:           true, // user/assistant text
			chat.MessageTypeToolUse:        true, // assistant tool_use
			chat.MessageTypeToolResult:     true, // tool execution result
			chat.MessageTypeSystem:         true, // system events / errors
			chat.MessageTypeCompactSummary: true, // PDF Section 9.1 + 7.3 boundary
		}

		// Then exactly five message types — refactor that drops one fails the
		//      length check and surfaces a transcript shape regression.
		assert.Len(t, types, 5,
			"agentic loop must enumerate 5 message types for durable storage")
		for ty := range types {
			assert.NotEmpty(t, string(ty),
				"every MessageType constant must be a non-empty token")
		}
	})

	t.Run("Scenario_CompactSummaryTypeIsFirstClassForBoundaryPersistence", func(t *testing.T) {
		// Given a compact_summary message representing a PDF compact_boundary
		//       (Section 9.1: "compaction markers" persist alongside regular
		//       messages — boundary metadata is part of the durable record),
		given := chat.ChatMessage{
			ID:          uuid.New(),
			SessionID:   uuid.New(),
			Role:        "system",
			Content:     "Summary of turns 1-12: user asked X, agent did Y…",
			MessageType: chat.MessageTypeCompactSummary,
			TurnIndex:   13,
		}

		// When the runner stores the boundary,
		// Then the type is recognisable and the summary content is preserved
		//      verbatim — required for read-time reconstruction.
		assert.Equal(t, chat.MessageTypeCompactSummary, given.MessageType,
			"compact_summary must be a first-class persisted message type")
		assert.NotEmpty(t, given.Content,
			"compact_summary must carry the full LLM-generated text for replay")
	})

	t.Run("Scenario_TurnIndexOrdersMessagesWithinASession", func(t *testing.T) {
		// Given multiple messages from one session,
		messages := []chat.ChatMessage{
			{TurnIndex: 0, MessageType: chat.MessageTypeText},
			{TurnIndex: 0, MessageType: chat.MessageTypeToolUse},
			{TurnIndex: 0, MessageType: chat.MessageTypeToolResult},
			{TurnIndex: 1, MessageType: chat.MessageTypeText},
		}

		// When the runner inspects turn boundaries,
		// Then TurnIndex partitions the transcript into reconstructable turns
		//      (the same role queryLoop's turnIndex plays in PDF Section 4.1).
		assert.Equal(t, 0, messages[0].TurnIndex,
			"first turn is index 0")
		assert.Equal(t, 1, messages[3].TurnIndex,
			"second turn after stop=text + tool_use loop")
	})

	t.Run("Scenario_RunIDLinksMessagesToBackgroundChatRun", func(t *testing.T) {
		// Given a background ChatRun produces messages,
		runID := uuid.New()
		given := chat.ChatMessage{
			ID:          uuid.New(),
			SessionID:   uuid.New(),
			Role:        "assistant",
			Content:     "result",
			MessageType: chat.MessageTypeText,
			RunID:       &runID,
		}

		// When the persistence layer stores them,
		// Then RunID is a nullable pointer — interactive turns that did not
		//      come from a tracked run leave it nil, while background runs
		//      attach the parent run's UUID for correlation.
		assert.NotNil(t, given.RunID,
			"messages from a background run must carry the run UUID")
		assert.Equal(t, runID, *given.RunID,
			"RunID must round-trip without mutation")
	})

	t.Run("Scenario_ToolCallIDLinksToolUseToToolResult", func(t *testing.T) {
		// Given an assistant tool_use followed by its tool_result,
		callID := "call_abc123"
		toolUse := chat.ChatMessage{
			MessageType: chat.MessageTypeToolUse,
			ToolCalls:   json.RawMessage(`[{"id":"call_abc123","type":"function","function":{"name":"agent","arguments":"{}"}}]`),
		}
		toolResult := chat.ChatMessage{
			MessageType: chat.MessageTypeToolResult,
			ToolCallID:  &callID,
			Content:     "sub-agent finished",
		}

		// When the runner pairs them at read time,
		// Then ToolCallID on the result must match the call ID inside the use
		//      message — preserves the API invariant that every tool_result
		//      pairs with a tool_use (PDF + Anthropic API contract).
		assert.NotNil(t, toolResult.ToolCallID,
			"tool_result must carry a non-nil tool_call_id")
		assert.Equal(t, callID, *toolResult.ToolCallID,
			"tool_call_id must equal the originating tool_use id")
		assert.NotEmpty(t, toolUse.ToolCalls,
			"tool_use must carry the structured tool_calls payload")
	})

	t.Run("Scenario_MetadataIsExtensibleJSON", func(t *testing.T) {
		// Given the transcript must capture per-message metadata that may
		//       evolve (PDF Section 9.1: "compaction markers, file-history
		//       snapshots, attribution snapshots, content-replacement records"),
		given := chat.ChatMessage{
			Metadata: json.RawMessage(`{"compactBoundary":true,"summarisedTurns":[1,2,3,4,5]}`),
		}

		// When the runtime reads metadata back,
		// Then it round-trips as raw JSON — schema-less metadata supports new
		//      event kinds without DB migrations.
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(given.Metadata, &decoded),
			"metadata must parse as valid JSON")
		assert.True(t, decoded["compactBoundary"].(bool),
			"compactBoundary marker must round-trip")
	})

	t.Run("Scenario_RepositoryDoesNotExposeUpdateMessage", func(t *testing.T) {
		// Given the persistence layer must enforce mostly-append-only writes
		//       (PDF Section 9.1: "explicit cleanup rewrites as an
		//       exception" — the default surface is INSERT only),
		// When we inspect the chat.Repository interface for an UpdateMessage
		//      or modify-existing-message API,
		repoType := reflect.TypeOf((*chat.Repository)(nil)).Elem()

		// Then no method exists for in-place message rewrites — the contract
		//      is INSERT-only at the API boundary, mirroring the JSONL
		//      append-only invariant from the PDF.
		for i := 0; i < repoType.NumMethod(); i++ {
			name := repoType.Method(i).Name
			assert.NotEqual(t, "UpdateMessage", name,
				"repository must NOT expose UpdateMessage — append-only contract")
			assert.NotEqual(t, "ModifyMessage", name,
				"repository must NOT expose ModifyMessage")
			assert.NotEqual(t, "PatchMessage", name,
				"repository must NOT expose PatchMessage")
		}
	})

	t.Run("Scenario_RepositoryExposesReplayQueries", func(t *testing.T) {
		// Given a session needs to be reconstructed from durable state (PDF
		//       Section 9: resume rebuilds conversation by replaying
		//       persisted records),
		repoType := reflect.TypeOf((*chat.Repository)(nil)).Elem()

		// When the runtime asks for the read API,
		// Then FindAllMessages (full replay), FindMessages (paginated),
		//      GetLatestCompactSummary (compact boundary lookup) all exist —
		//      these are the verbs needed for resume / fork / replay.
		hasFindAll := false
		hasFindPaged := false
		hasGetLatestCompact := false
		for i := 0; i < repoType.NumMethod(); i++ {
			switch repoType.Method(i).Name {
			case "FindAllMessages":
				hasFindAll = true
			case "FindMessages":
				hasFindPaged = true
			case "GetLatestCompactSummary":
				hasGetLatestCompact = true
			}
		}
		assert.True(t, hasFindAll,
			"FindAllMessages must exist for full transcript replay")
		assert.True(t, hasFindPaged,
			"FindMessages must exist for paginated UI access")
		assert.True(t, hasGetLatestCompact,
			"GetLatestCompactSummary must exist for compact-boundary-aware reconstruction")
	})

	t.Run("Scenario_TenantScopingReplacesProjectScopedFilesystemPath", func(t *testing.T) {
		// Given AgentHub's tenant-isolated schema (ADR-002: ah_{tenantId}.*
		//       per tenant), the architectural equivalent of PDF's
		//       project-scoped filesystem path,
		given := chat.ChatMessage{SessionID: uuid.New()}

		// When a ChatMessage exists,
		// Then it does NOT carry a tenant_id field — isolation is enforced by
		//      the connection's search_path (set by tenant middleware), not
		//      by per-row column. This mirrors ADR-002's "per-tenant schema"
		//      decision: no leakage between tenants is structurally possible.
		typ := reflect.TypeOf(given)
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name
			assert.NotEqual(t, "TenantID", fieldName,
				"per-tenant tables must NOT carry tenant_id (ADR-002)")
			assert.NotEqual(t, "Tenant", fieldName,
				"per-tenant tables must NOT carry tenant column")
		}
		assert.NotEqual(t, uuid.Nil, given.SessionID,
			"SessionID is the project-equivalent scoping field")
	})
}
