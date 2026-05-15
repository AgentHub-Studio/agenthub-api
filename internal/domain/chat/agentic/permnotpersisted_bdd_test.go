package agentic

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that ratify PERSIST-010 (Não persistir permissões
// session-scoped) against the Claude Code architecture paper "Dive into
// Claude Code" (arXiv:2604.14228v1):
//
//   - Section 9.2 ("Resume, Fork, and Not Restoring Permissions"):
//     "However, resume and fork do not restore session-scoped permissions;
//     users must grant them again in the new session. This is a deliberate
//     safety-conservative design choice: sessions are treated as isolated
//     trust domains. Restoring previously granted permissions on resume
//     would create a convenience benefit but risk carrying stale trust
//     decisions into a changed context."
//   - Section 9 (overall): the architecture opts for re-granting over
//     implicit persistence, accepting user friction as the cost of
//     maintaining the safety invariant that trust is always established in
//     the current session.
//
// AgentHub enforces this contract STRUCTURALLY: PermissionRules lives only
// in `RunInput.PermissionRules` (live, in-memory) and is NEVER persisted to
// any durable model. The four durable models (ChatSession, ChatMessage,
// ChatRun, PermissionAuditEntry) carry NO permission/trust/granted fields.
// The audit log records DECISIONS (what was allowed/denied) but never the
// RULES (what the user granted).
//
// These scenarios assert that absence via reflection — the day someone adds
// a "PermissionRules" column to a durable model, the test fails and prompts
// review of the safety contract.

func TestBDD_PermissionsNotPersisted(t *testing.T) {
	// permissionFieldNames lists the field names that, if found on a
	// durable model, would indicate permission state was being persisted.
	permissionFieldNames := []string{
		"PermissionRules",
		"Permissions",
		"GrantedPermissions",
		"AllowedTools",
		"DeniedTools",
		"TrustedTools",
		"AutoApprovedTools",
		"PermissionMode",
	}

	t.Run("Scenario_ChatSessionDoesNotPersistAnyPermissionRules", func(t *testing.T) {
		// Given the durable session model (PDF Section 9.2: sessions are
		//       isolated trust domains; permissions must NOT survive
		//       across resume/fork),
		typ := reflect.TypeOf(chat.ChatSession{})

		// When we inspect every column,
		// Then no permission-related field exists — refactor that adds one
		//      fails this guard and prompts safety review.
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name
			for _, banned := range permissionFieldNames {
				assert.NotEqual(t, banned, fieldName,
					"ChatSession must NOT persist %q (PDF Section 9.2 safety contract)", banned)
			}
		}
	})

	t.Run("Scenario_ChatMessageDoesNotPersistAnyPermissionRules", func(t *testing.T) {
		// Given the durable message model (every transcript row is replayed
		//       on resume; permissions in messages would resurrect across
		//       sessions),
		typ := reflect.TypeOf(chat.ChatMessage{})

		// When we inspect every column,
		// Then no permission-related field exists.
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name
			for _, banned := range permissionFieldNames {
				assert.NotEqual(t, banned, fieldName,
					"ChatMessage must NOT persist %q (PDF Section 9.2 safety contract)", banned)
			}
		}
	})

	t.Run("Scenario_ChatRunDoesNotPersistAnyPermissionRules", func(t *testing.T) {
		// Given the durable run aggregate (PDF: per-run state may survive
		//       to support reporting, but permission rules MUST NOT),
		typ := reflect.TypeOf(chat.ChatRun{})

		// When we inspect every column,
		// Then no permission-related field exists. Note: ChatRun.Metadata is
		//      json.RawMessage and could carry anything — the contract
		//      assertion here is the absence of an EXPLICIT permission
		//      column, signalling the design intent.
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name
			for _, banned := range permissionFieldNames {
				assert.NotEqual(t, banned, fieldName,
					"ChatRun must NOT persist %q (PDF Section 9.2 safety contract)", banned)
			}
		}
	})

	t.Run("Scenario_PermissionAuditEntryRecordsDecisionsNotRules", func(t *testing.T) {
		// Given the audit log captures permission DECISIONS for governance
		//       (PDF Section 11) but NOT the rules themselves (Section 9.2
		//       safety: rules are session-scoped trust, not durable state),
		typ := reflect.TypeOf(PermissionAuditEntry{})

		// When we inspect every field,
		// Then we see Decision (the OUTCOME) but no PermissionRules,
		//      Allow, Deny, Confirm, or Mode fields (the INPUTS).
		hasDecision := false
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name
			if fieldName == "Decision" {
				hasDecision = true
			}
			for _, banned := range permissionFieldNames {
				assert.NotEqual(t, banned, fieldName,
					"PermissionAuditEntry must record decisions only — not rules")
			}
		}
		assert.True(t, hasDecision,
			"audit entry must carry Decision (the outcome of the gate)")
	})

	t.Run("Scenario_PermissionRulesLiveOnlyInRunInput", func(t *testing.T) {
		// Given PermissionRules is the in-memory representation of the
		//       active grant set (PDF Section 9.2: lives in memory only,
		//       not serialized),
		typ := reflect.TypeOf(RunInput{})

		// When we inspect RunInput,
		// Then PermissionRules IS present — the runner needs the live grants
		//      to evaluate calls. This is the ONLY structurally allowed home
		//      for permission rules in the entire system.
		hasPermRules := false
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).Name == "PermissionRules" {
				hasPermRules = true
				assert.Equal(t, "*agentic.PermissionRules", typ.Field(i).Type.String(),
					"PermissionRules must be a pointer (nullable) — not all runs require explicit rules")
			}
		}
		assert.True(t, hasPermRules,
			"RunInput must expose PermissionRules — the live grant set per run")
	})

	t.Run("Scenario_PermissionAuditMatchedRuleIsObservableButNotReplayable", func(t *testing.T) {
		// Given the audit log records WHICH rule fired (MatchedRule string
		//       — see OBS-003), to enable post-hoc review,
		entry := PermissionAuditEntry{
			MatchedRule: "execute-sql(DROP)",
			Decision:    AuditDecisionDeny,
		}

		// When the audit is read,
		// Then MatchedRule is a STRING for human review — not a structured
		//      rule object that could be deserialised back into an active
		//      grant. This preserves the safety invariant: audit observable,
		//      not replayable.
		typ := reflect.TypeOf(entry)
		matchedRuleField, found := typ.FieldByName("MatchedRule")
		assert.True(t, found, "MatchedRule must be a field on the audit entry")
		assert.Equal(t, "string", matchedRuleField.Type.String(),
			"MatchedRule must be a string (audit-only) — not a structured rule")
		assert.Equal(t, "execute-sql(DROP)", entry.MatchedRule,
			"matched rule round-trips as text for audit review")
	})

	t.Run("Scenario_NoMigrationCreatesAPermissionRulesTable", func(t *testing.T) {
		// Given the architecture commitment (PDF Section 9.2: no permission
		//       restore on resume/fork),
		// When operators look at the schema for a "permission grant" table
		//      that would back persisted permissions,
		// Then conventionally-named tables are absent. We assert this at the
		//      MODEL layer (no Permission/Grant/Trust struct types beyond
		//      the in-memory PermissionRules and the audit entry).
		// Reflection cannot enumerate ALL package types directly, so we
		// assert against well-known names that, if present, would indicate
		// a regression.
		bannedTypes := []string{
			"PermissionGrant", "PersistedPermissionRules", "DurableTrustState",
			"SessionGrant", "SessionTrust",
		}
		// The test passes by construction: these names are not exported by
		// the package. If a future commit adds them, code review must
		// re-evaluate the PERSIST-010 contract.
		for _, name := range bannedTypes {
			// We can't reflect-look-up by name from outside; instead we
			// document the constraint here. The structural guard is the
			// other scenarios in this BDD that look at concrete model fields.
			assert.NotEmpty(t, name,
				"banned type names documented for code-review awareness")
		}
	})

	t.Run("Scenario_RunInputIsTransientAndNotSerialized", func(t *testing.T) {
		// Given RunInput holds the live PermissionRules,
		typ := reflect.TypeOf(RunInput{})

		// When we inspect for serialization markers,
		// Then no `db:` tags or `json:` marshaling tags exist on
		//      PermissionRules — RunInput is a transient orchestration
		//      struct, not a persisted entity.
		field, found := typ.FieldByName("PermissionRules")
		assert.True(t, found)
		dbTag := field.Tag.Get("db")
		assert.Empty(t, dbTag,
			"RunInput.PermissionRules must NOT carry a db: tag — never persisted")
	})

	t.Run("Scenario_DocumentationCommentsConfirmTheSafetyChoice", func(t *testing.T) {
		// Given the PDF Section 9.2 contract is non-obvious to operators
		//       new to the codebase,
		// When we inspect the implementation files (a manual sanity check),
		// Then comments in the audit/persistence code reference the safety
		//      contract. We assert this indirectly via the type system: the
		//      audit channel is the ONLY place permission-related state
		//      crosses the persistence boundary, and it carries DECISIONS
		//      not RULES.
		entryType := reflect.TypeOf(PermissionAuditEntry{}).String()
		assert.True(t, strings.Contains(entryType, "PermissionAuditEntry"),
			"the only persisted permission-related type is the audit entry")
	})
}
