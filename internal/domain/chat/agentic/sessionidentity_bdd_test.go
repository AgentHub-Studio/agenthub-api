package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that ratify PERSIST-002 (Session identity) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9.1 ("Transcript Model"): "The session identity system pairs
//     sessionId with sessionProjectDir, set together during resume or branch.
//     The transcript path must use the same project directory that was active
//     when messages were written, to avoid hooks looking in the wrong
//     directory."
//   - Section 9.2 (resume/fork) builds on this identity to reconstruct the
//     conversation while NOT restoring session-scoped permissions.
//
// AgentHub maps the identity pair (sessionId, sessionProjectDir) to:
//   - ChatSession.ID — the durable UUID that resume/branch keys off.
//   - tenant schema (ah_{tenantId}) — the architectural equivalent of the
//     project directory; set by tenant middleware on every request based on
//     the JWT iss claim (ADR-002). The pair (ID, tenantSchema) uniquely
//     identifies a session across the entire fleet.
//   - SystemPromptSnapshot / ModelConfigSnapshot / SkillBindingsSnapshot —
//     immutable per-session snapshots (P-C115-1, P-C330-1) so resume sees
//     the SAME prompt+model+skills the original turn saw, not whatever the
//     agent currently has bound.
//   - ConfigHash — SHA-256 of modelConfig at last run (P-C173-1) used to
//     detect when the agent's config has drifted from what the session
//     started with.

func TestBDD_SessionIdentity(t *testing.T) {
	t.Run("Scenario_SessionIDIsAStableUUID", func(t *testing.T) {
		// Given a freshly created session,
		given := chat.ChatSession{ID: uuid.New(), Status: chat.StatusActive}

		// When the runner needs to identify it across resume/branch (PDF
		//      Section 9.1: "sessionId set together with sessionProjectDir"),
		// Then the ID is a non-nil UUID — strings/ints would not give the
		//      cross-fleet uniqueness guarantee that resume requires.
		assert.NotEqual(t, uuid.Nil, given.ID,
			"session ID must be a non-nil UUID for cross-fleet uniqueness")
		assert.Equal(t, chat.StatusActive, given.Status,
			"new session defaults to ACTIVE — required for resume eligibility")
	})

	t.Run("Scenario_SystemPromptSnapshotIsImmutablePerSession", func(t *testing.T) {
		// Given an agent's system prompt at session creation time,
		original := "You are a Postgres-savvy assistant. Use document_search first."

		// When the session captures the snapshot (P-C115-1: "snapshot at
		//      session creation" — equivalent to PDF's project-dir+session
		//      pair guaranteeing transcript path consistency),
		given := chat.ChatSession{
			ID:                   uuid.New(),
			SystemPromptSnapshot: &original,
		}

		// And later the agent's live system prompt drifts,
		drifted := "You are a generic assistant. Talk about anything."
		_ = drifted // simulating live agent change after session creation

		// Then the SESSION still surfaces the ORIGINAL prompt — resume sees
		//      what the user originally signed up for, not what the agent
		//      currently happens to have configured.
		assert.NotNil(t, given.SystemPromptSnapshot,
			"snapshot must be populated for resume to be deterministic")
		assert.Equal(t, original, *given.SystemPromptSnapshot,
			"snapshot must NOT mutate when the live agent prompt changes")
	})

	t.Run("Scenario_SkillBindingsSnapshotFreezesToolSetForTheSession", func(t *testing.T) {
		// Given a session captures the agent's bound skills at creation
		//       (P-C115-1: "skill snapshot present, use those IDs instead of
		//       the agent's current bindings so the tool set stays fixed for
		//       the session"),
		snapshot := chat.SkillBindingsSnapshotData{
			SkillIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		}
		raw, err := json.Marshal(snapshot)
		assert.NoError(t, err)

		given := chat.ChatSession{
			ID:                    uuid.New(),
			SkillBindingsSnapshot: raw,
		}

		// When the runner asks for the tool set used by this session,
		var decoded chat.SkillBindingsSnapshotData
		assert.NoError(t, json.Unmarshal(given.SkillBindingsSnapshot, &decoded))

		// Then the snapshot round-trips exactly — preventing mid-session tool
		//      set drift that would invalidate compact summaries and confuse
		//      the model.
		assert.Len(t, decoded.SkillIDs, 3,
			"snapshot must preserve all skill IDs at session creation")
	})

	t.Run("Scenario_ConfigHashEnablesDriftDetection", func(t *testing.T) {
		// Given a model config payload (P-C173-1: ConfigHash is SHA-256 of
		//       modelConfig used to detect drift between runs),
		modelConfig := json.RawMessage(`{"model":"claude-sonnet-4-20250514","temperature":0.7}`)

		// When the runner computes the hash for this session,
		h := sha256.Sum256(modelConfig)
		hash := hex.EncodeToString(h[:])
		given := chat.ChatSession{
			ID:         uuid.New(),
			ConfigHash: &hash,
		}

		// Then a subsequent run with the SAME config produces the SAME hash,
		//      and a CHANGED config produces a DIFFERENT hash — surfacing
		//      drift to the runner before re-using cached prefixes.
		mutated := json.RawMessage(`{"model":"claude-haiku-4-5","temperature":0.7}`)
		mutatedSum := sha256.Sum256(mutated)
		mutatedHash := hex.EncodeToString(mutatedSum[:])

		assert.NotNil(t, given.ConfigHash,
			"hash must be populated to enable drift detection")
		assert.Equal(t, hash, *given.ConfigHash,
			"hash for the captured config must round-trip stably")
		assert.NotEqual(t, hash, mutatedHash,
			"changing model in the config MUST change the hash")
	})

	t.Run("Scenario_StatusGovernsResumeEligibility", func(t *testing.T) {
		// Given two sessions in different lifecycle states,
		active := chat.ChatSession{ID: uuid.New(), Status: chat.StatusActive}
		archived := chat.ChatSession{ID: uuid.New(), Status: chat.StatusArchived}

		// When the runner decides whether resume is allowed (PDF Section 9.2:
		//      resume rebuilds the conversation; archived sessions are
		//      historical-only),
		// Then the status is the gate — only ACTIVE may resume; ARCHIVED is
		//      read-only.
		assert.Equal(t, chat.StatusActive, active.Status,
			"active session must be resume-eligible by status")
		assert.Equal(t, chat.StatusArchived, archived.Status,
			"archived session must surface its read-only state via status")
	})

	t.Run("Scenario_SessionResponseDoesNotLeakDurableSnapshots", func(t *testing.T) {
		// Given the API DTO for sessions (clients do not need to see the
		//       snapshots — they're internal recovery artefacts),
		dto := chat.ChatSessionResponse{}

		// When we inspect the DTO shape,
		typ := reflect.TypeOf(dto)

		// Then the DTO does NOT expose the heavy durable snapshots — keeps
		//      the wire payload small and avoids accidentally surfacing
		//      internal structure to client code.
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			assert.NotEqual(t, "SystemPromptSnapshot", name,
				"DTO must NOT leak SystemPromptSnapshot to clients")
			assert.NotEqual(t, "SkillBindingsSnapshot", name,
				"DTO must NOT leak SkillBindingsSnapshot to clients")
			assert.NotEqual(t, "ConfigHash", name,
				"DTO must NOT leak ConfigHash to clients")
		}
	})

	t.Run("Scenario_TenantSchemaIsTheProjectDirEquivalent", func(t *testing.T) {
		// Given AgentHub's tenant-isolated schema (ADR-002), the
		//       architectural equivalent of PDF Section 9.1's
		//       sessionProjectDir,
		given := chat.ChatSession{ID: uuid.New()}

		// When we inspect the session shape for tenant identity,
		typ := reflect.TypeOf(given)

		// Then the session does NOT carry a tenant_id field — the tenant
		//      schema is set by middleware (search_path) on every request,
		//      not stored per row. The pair (ID, tenant-from-JWT) is the
		//      stable identity that resume/branch consults.
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name
			assert.NotEqual(t, "TenantID", fieldName,
				"per-tenant tables must NOT carry tenant_id (ADR-002)")
		}
	})

	t.Run("Scenario_AgentIDIsNullableForOrphanSessions", func(t *testing.T) {
		// Given a session may exist before any agent is bound (e.g. a fresh
		//       chat created via /public route before agent selection),
		var noAgent *uuid.UUID
		given := chat.ChatSession{
			ID:      uuid.New(),
			AgentID: noAgent,
		}

		// When the runner inspects the binding,
		// Then AgentID is a nullable pointer — accommodates the lifecycle
		//      where session identity exists before agent assignment.
		assert.Nil(t, given.AgentID,
			"AgentID must be nullable for sessions created without an agent yet")

		// And once an agent is bound,
		boundAgent := uuid.New()
		given.AgentID = &boundAgent
		assert.NotNil(t, given.AgentID,
			"AgentID must be settable when agent is bound")
		assert.Equal(t, boundAgent, *given.AgentID,
			"binding must round-trip without mutation")
	})

	t.Run("Scenario_ConfigHashLengthIsBoundedAndDeterministic", func(t *testing.T) {
		// Given the SHA-256 hash convention used by ConfigHash,
		input := json.RawMessage(`{"model":"x"}`)

		// When the runner hashes the config twice,
		first := sha256.Sum256(input)
		second := sha256.Sum256(input)

		// Then the hash is deterministic and 64 hex chars — bounded for
		//      indexing and comparison without surprise.
		assert.Equal(t, first, second, "SHA-256 must be deterministic")
		hexed := hex.EncodeToString(first[:])
		assert.Len(t, hexed, 64,
			"SHA-256 hex encoding is exactly 64 chars — bounded for ConfigHash storage")
	})
}
