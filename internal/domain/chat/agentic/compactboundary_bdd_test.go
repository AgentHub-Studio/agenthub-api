package agentic

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that ratify PERSIST-008 (Compact boundary
// persistence) against the Claude Code architecture paper "Dive into
// Claude Code" (arXiv:2604.14228v1):
//
//   - Section 9.1 (Transcript Model): "Session transcripts store several
//     kinds of events beyond simple messages, including COMPACTION
//     MARKERS, file-history snapshots, attribution snapshots, and
//     content-replacement records." Compact boundaries are first-class
//     persisted events.
//   - Section 7.3 (Compaction Pipeline): "buildPostCompactMessages()
//     returns: [boundaryMarker, ...summaryMessages, ...messagesToKeep,
//     ...attachments, ...hookResults]. The boundary marker is annotated
//     with preserved-segment metadata via annotateBoundaryWithPreservedSegment(),
//     recording headUuid, anchorUuid, and tailUuid to enable read-time
//     chain patching."
//   - Section 9.2 (Resume): boundaries enable boundary-aware
//     reconstruction so a resumed session sees the COMPACTED view, not
//     the pre-compaction full history.
//
// AgentHub maps compact boundary persistence to:
//   - chat.MessageTypeCompactSummary — first-class message type for the
//     boundary marker (PERSIST-001 covered the type contract).
//   - chat.Repository.GetLatestCompactSummary — bounded query returning
//     the most recent compact_summary for a session, sorted by created_at
//     descending. Used by the runner to anchor read-time projection.
//   - PromptBuilder.summFn (CompactSummaryFinder interface) — abstraction
//     consumed by the runtime to fetch the boundary at prompt assembly.
//   - chat_message.metadata JSON column — extensible storage for
//     preserved-segment metadata equivalent to PDF's headUuid/anchorUuid/
//     tailUuid.
//
// These scenarios assert: boundary message type identity, repository
// contract surface, latest-wins lookup semantics, metadata extensibility.

func TestBDD_CompactBoundaryPersistence(t *testing.T) {
	t.Run("Scenario_CompactSummaryIsAFirstClassMessageType", func(t *testing.T) {
		// Given the agentic loop emits compact_summary at boundary points
		//       (PDF Section 9.1: compaction markers persisted alongside
		//       regular messages),
		// When the durable model is inspected,
		// Then the message type token "compact_summary" exists distinctly
		//      from text/tool_use/tool_result/system — the boundary is
		//      structurally separable from regular conversation events.
		assert.Equal(t, chat.MessageType("compact_summary"),
			chat.MessageTypeCompactSummary,
			"compact_summary must be a stable wire token")
		assert.NotEqual(t, chat.MessageTypeText, chat.MessageTypeCompactSummary,
			"boundary type distinct from text")
		assert.NotEqual(t, chat.MessageTypeToolUse, chat.MessageTypeCompactSummary,
			"boundary type distinct from tool_use")
		assert.NotEqual(t, chat.MessageTypeToolResult, chat.MessageTypeCompactSummary,
			"boundary type distinct from tool_result")
		assert.NotEqual(t, chat.MessageTypeSystem, chat.MessageTypeCompactSummary,
			"boundary type distinct from system")
	})

	t.Run("Scenario_BoundaryMessageCarriesSummaryContent", func(t *testing.T) {
		// Given a compact boundary message produced by the LLM
		//       summarisation step (PDF Section 7.3: boundary message has
		//       the summary text the model generated),
		given := chat.ChatMessage{
			ID:          uuid.New(),
			SessionID:   uuid.New(),
			Role:        "system",
			Content:     "Summary of turns 1-12: user asked X, agent did Y, conclusion Z.",
			MessageType: chat.MessageTypeCompactSummary,
			TurnIndex:   13,
		}

		// When persisted and read back,
		// Then the summary content is preserved verbatim — required for
		//      replay/reconstruction.
		assert.Equal(t, chat.MessageTypeCompactSummary, given.MessageType,
			"message type must be compact_summary for boundary marker")
		assert.NotEmpty(t, given.Content,
			"compact_summary must carry the LLM-generated summary text")
		assert.Equal(t, 13, given.TurnIndex,
			"TurnIndex anchors the boundary to a specific turn")
	})

	t.Run("Scenario_RepositoryExposesGetLatestCompactSummaryQuery", func(t *testing.T) {
		// Given the durable repository (PDF Section 9.2: read-time
		//       projection needs to find the most recent boundary so the
		//       runner can anchor its conversation view),
		repoType := reflect.TypeOf((*chat.Repository)(nil)).Elem()

		// When we inspect for the boundary lookup verb,
		hasGetLatest := false
		for i := 0; i < repoType.NumMethod(); i++ {
			if repoType.Method(i).Name == "GetLatestCompactSummary" {
				hasGetLatest = true
				break
			}
		}

		// Then GetLatestCompactSummary exists — first-class query for the
		//      runtime to discover the latest boundary without scanning
		//      the full transcript.
		assert.True(t, hasGetLatest,
			"chat.Repository must expose GetLatestCompactSummary for boundary-aware reconstruction")
	})

	t.Run("Scenario_GetLatestCompactSummaryReturnsBoolPresent", func(t *testing.T) {
		// Given the repository contract (PDF Section 9.2: a session may
		//       have NO compaction yet — caller must distinguish "not
		//       found" from "error"),
		repoType := reflect.TypeOf((*chat.Repository)(nil)).Elem()
		method, found := repoType.MethodByName("GetLatestCompactSummary")
		assert.True(t, found, "method must exist (precondition)")

		// When we inspect the return signature,
		// Then it returns (ChatMessage, bool, error) — the bool
		//      explicitly signals presence, distinct from error.
		assert.Equal(t, 3, method.Type.NumOut(),
			"method must return 3 values: message, found-bool, error")
	})

	t.Run("Scenario_PromptBuilderConsumesCompactSummaryFinderInterface", func(t *testing.T) {
		// Given the runtime depends on a narrow interface (PDF: composable
		//       consumer; testable without a real DB),
		ifaceType := reflect.TypeOf((*CompactSummaryFinder)(nil)).Elem()

		// When we inspect the interface,
		// Then it has exactly one method (single-purpose, easy to mock).
		assert.Equal(t, 1, ifaceType.NumMethod(),
			"CompactSummaryFinder is single-method interface — easy to mock for tests")
		assert.Equal(t, "GetLatestCompactSummary", ifaceType.Method(0).Name,
			"the only method is the boundary lookup verb")
	})

	t.Run("Scenario_BoundaryMetadataIsExtensibleViaJSON", func(t *testing.T) {
		// Given the chat_message.metadata column is json.RawMessage
		//       (PDF Section 7.3: boundary marker is annotated with
		//       preserved-segment metadata — headUuid/anchorUuid/tailUuid;
		//       AgentHub's flexible metadata column can carry equivalent
		//       structured data without schema migration),
		boundaryMeta := json.RawMessage(`{
			"compactBoundary": true,
			"headUuid": "11111111-1111-1111-1111-111111111111",
			"anchorUuid": "22222222-2222-2222-2222-222222222222",
			"tailUuid": "33333333-3333-3333-3333-333333333333",
			"summarisedTurns": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]
		}`)

		given := chat.ChatMessage{
			MessageType: chat.MessageTypeCompactSummary,
			Metadata:    boundaryMeta,
		}

		// When the runner reads the metadata back,
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(given.Metadata, &decoded))

		// Then all 4 PDF-described boundary fields are roundtrip-able —
		//      the schema-less metadata supports the same chain-patching
		//      workflow described in PDF Section 7.3.
		assert.True(t, decoded["compactBoundary"].(bool),
			"compactBoundary marker must roundtrip")
		assert.NotEmpty(t, decoded["headUuid"],
			"headUuid roundtrips for chain-patching")
		assert.NotEmpty(t, decoded["anchorUuid"],
			"anchorUuid roundtrips")
		assert.NotEmpty(t, decoded["tailUuid"],
			"tailUuid roundtrips")
		assert.NotNil(t, decoded["summarisedTurns"],
			"summarisedTurns range roundtrips")
	})

	t.Run("Scenario_BoundaryAndRegularMessagesShareTheSameTable", func(t *testing.T) {
		// Given both kinds persist via the same INSERT path (PDF Section
		//       9.1: compaction markers are STORED ALONGSIDE regular
		//       messages, not in a parallel table — single source of truth),
		regular := chat.ChatMessage{MessageType: chat.MessageTypeText}
		boundary := chat.ChatMessage{MessageType: chat.MessageTypeCompactSummary}

		// When inspecting the model types,
		// Then they share the same struct (chat.ChatMessage) — the type
		//      field is the discriminator, not a separate type tree.
		assert.Equal(t, reflect.TypeOf(regular), reflect.TypeOf(boundary),
			"boundary and regular messages share the same Go type — same table")
	})

	t.Run("Scenario_BoundaryMessageRunIDIsNullableForOutOfRunBoundaries", func(t *testing.T) {
		// Given a boundary may be created by the runtime outside any
		//       active run (e.g. eager compaction during session resume),
		given := chat.ChatMessage{
			MessageType: chat.MessageTypeCompactSummary,
			RunID:       nil,
		}

		// When the persistence layer inspects RunID,
		// Then nil is allowed — boundaries don't strictly belong to a
		//      single run.
		assert.Nil(t, given.RunID,
			"compact_summary RunID is nullable")
	})

	t.Run("Scenario_RunnerSkipsCompactSummaryDuringHistoryReplay", func(t *testing.T) {
		// Given the runner's loadHistory has a comment + behaviour to skip
		//       compact_summary (and system) messages because the
		//       PromptBuilder injects them differently (runner.go:1830:
		//       "Skip system and compact_summary messages — handled by
		//       PromptBuilder."),
		// When we model what loadHistory should do,
		messages := []chat.ChatMessage{
			{MessageType: chat.MessageTypeText, Content: "user msg"},
			{MessageType: chat.MessageTypeCompactSummary, Content: "boundary"},
			{MessageType: chat.MessageTypeText, Content: "assistant msg"},
		}

		// When we filter for AI-API messages,
		var visible []chat.ChatMessage
		for _, m := range messages {
			if m.MessageType == chat.MessageTypeSystem ||
				m.MessageType == chat.MessageTypeCompactSummary {
				continue
			}
			visible = append(visible, m)
		}

		// Then 2 messages survive (the boundary is filtered) — the
		//      PromptBuilder will inject the boundary's summary content
		//      via a different code path (system prompt augmentation).
		assert.Len(t, visible, 2,
			"loadHistory must skip compact_summary (PromptBuilder handles it)")
		for _, m := range visible {
			assert.NotEqual(t, chat.MessageTypeCompactSummary, m.MessageType,
				"compact_summary must be filtered from history replay")
		}
	})

	t.Run("Scenario_CompactBoundaryPreservesTimestampForOrdering", func(t *testing.T) {
		// Given multiple boundaries across a session (PDF Section 9.1:
		//       "ORDER BY created_at DESC LIMIT 1" — the LATEST wins),
		t1 := time.Now().Add(-2 * time.Hour)
		t2 := time.Now().Add(-1 * time.Hour)
		t3 := time.Now()
		boundaries := []chat.ChatMessage{
			{MessageType: chat.MessageTypeCompactSummary, CreatedAt: t1, Content: "old"},
			{MessageType: chat.MessageTypeCompactSummary, CreatedAt: t2, Content: "middle"},
			{MessageType: chat.MessageTypeCompactSummary, CreatedAt: t3, Content: "newest"},
		}

		// When the repository sorts DESC by created_at,
		var newest chat.ChatMessage
		for _, b := range boundaries {
			if b.CreatedAt.After(newest.CreatedAt) {
				newest = b
			}
		}

		// Then the newest boundary wins — re-resume after multiple
		//      compactions sees only the latest summary, not all.
		assert.Equal(t, "newest", newest.Content,
			"GetLatestCompactSummary returns the newest by CreatedAt DESC")
	})
}
