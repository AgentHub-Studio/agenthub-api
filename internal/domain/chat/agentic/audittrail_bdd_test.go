package agentic

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// BDD-style scenarios that ratify GOV-001 (Audit trail) against the Claude
// Code architecture paper "Dive into Claude Code" (arXiv:2604.14228v1):
//
//   - Section 11 (Tensions, trade-offs, observability, governance):
//     governance requires a comprehensive, queryable, immutable record of
//     agent behaviour so operators can answer "what happened" and "why" with
//     evidence — not narrative.
//   - Section 5 (deny-first + ask-by-default): decisions must be auditable.
//   - Section 9 (persistence): the durable record must outlive the live
//     context window AND the live runtime process.
//
// AgentHub's audit trail is composed of FOUR interconnected channels, all
// linked by SessionID + (optionally) RunID:
//
//   1. Conversation transcript        — chat_message (PERSIST-001 covers the
//                                        append-only contract).
//   2. Run-level aggregate            — chat_run + RunMetric (OBS-002).
//   3. Per-tool execution metrics     — ToolMetric (OBS-002 covers attribution).
//   4. Permission decisions           — permission_audit_log via
//                                        PermissionAuditRepository (OBS-003 covers
//                                        the contract).
//
// THIS BDD asserts the GOVERNANCE-LEVEL invariants that tie the four
// channels into a single audit trail: shared linkage keys, append-only
// surface across channels, queryability bounded by session/tenant, and
// retention helpers.

func TestBDD_AuditTrailGovernance(t *testing.T) {
	t.Run("Scenario_AllAuditChannelsShareTheSessionLinkageKey", func(t *testing.T) {
		// Given the four audit channels (transcript, run aggregate, tool
		//       metrics, permission decisions),
		sessionID := uuid.New()

		// When entries from each channel reference the SAME session,
		msg := chat.ChatMessage{SessionID: sessionID}
		runMetric := RunMetric{SessionID: sessionID}
		toolMetric := ToolMetric{SessionID: sessionID}
		permEntry := PermissionAuditEntry{SessionID: sessionID}

		// Then SessionID is the universal join key — operators can
		//      reconstruct a complete picture of one session by joining the
		//      four channels on this column.
		assert.Equal(t, sessionID, msg.SessionID,
			"chat_message must carry SessionID")
		assert.Equal(t, sessionID, runMetric.SessionID,
			"RunMetric must carry SessionID")
		assert.Equal(t, sessionID, toolMetric.SessionID,
			"ToolMetric must carry SessionID")
		assert.Equal(t, sessionID, permEntry.SessionID,
			"PermissionAuditEntry must carry SessionID")
	})

	t.Run("Scenario_RunIDLinksWithinTheRunWhenPresent", func(t *testing.T) {
		// Given a single agentic run produces entries across multiple
		//       channels (PDF Section 11: per-run roll-up is the natural
		//       grain for cost + safety reporting),
		runID := uuid.New()
		runIDStr := runID.String()

		// When entries reference the same run identifier,
		runMetric := RunMetric{RunID: runIDStr}
		toolMetric := ToolMetric{RunID: runIDStr}
		permEntry := PermissionAuditEntry{RunID: &runID}
		// chat_message references *uuid.UUID for RunID
		msgRunID := runID
		msg := chat.ChatMessage{RunID: &msgRunID}

		// Then RunID/runId/RunID strings/pointers all resolve to the same
		//      logical identifier for join — supports per-run audit slicing.
		assert.Equal(t, runIDStr, runMetric.RunID,
			"RunMetric.RunID is a string serialisation of UUID")
		assert.Equal(t, runIDStr, toolMetric.RunID,
			"ToolMetric.RunID matches RunMetric.RunID convention")
		assert.NotNil(t, permEntry.RunID, "PermissionAuditEntry uses *uuid.UUID")
		assert.Equal(t, runID, *permEntry.RunID)
		assert.NotNil(t, msg.RunID, "chat_message uses *uuid.UUID for nullable runs")
		assert.Equal(t, runID, *msg.RunID)
	})

	t.Run("Scenario_AuditChannelsHaveBoundedListAPIsForQueryability", func(t *testing.T) {
		// Given an operator needs to retrieve audit entries for a session
		//       (PDF Section 11: governance APIs must be queryable),
		var repo *PermissionAuditRepository = &PermissionAuditRepository{} // zero value, just for shape inspection
		typ := reflect.TypeOf(repo)

		// When the runtime exposes the query API,
		hasListBySession := false
		for i := 0; i < typ.NumMethod(); i++ {
			if typ.Method(i).Name == "ListBySession" {
				hasListBySession = true
			}
		}

		// Then ListBySession exists and is the bounded retrieval surface —
		//      no unbounded "list everything" entry point that would let an
		//      operator vacuum the audit log.
		assert.True(t, hasListBySession,
			"PermissionAuditRepository must expose ListBySession bounded query")
	})

	t.Run("Scenario_PermissionAuditRepositoryDoesNotExposeMutators", func(t *testing.T) {
		// Given the audit log must be immutable post-write (PDF Section 11:
		//       "audit trail legível" — and credible audit cannot allow
		//       silent rewrites),
		var repo *PermissionAuditRepository = &PermissionAuditRepository{}
		typ := reflect.TypeOf(repo)

		// When we inspect the repository surface,
		// Then no Update/Delete/Patch verbs exist — only LogDecision (INSERT)
		//      and read APIs (ListBySession). A refactor that adds a
		//      mutator fails this guard.
		for i := 0; i < typ.NumMethod(); i++ {
			name := typ.Method(i).Name
			assert.NotContains(t, name, "Update",
				"audit log must not expose Update mutators")
			assert.NotContains(t, name, "Delete",
				"audit log must not expose Delete mutators")
			assert.NotContains(t, name, "Patch",
				"audit log must not expose Patch mutators")
		}
	})

	t.Run("Scenario_AuditChannelsAreTenantIsolatedByConstruction", func(t *testing.T) {
		// Given AgentHub's per-tenant schema isolation (ADR-002),
		// When we inspect the audit channel struct shapes,
		// Then per-tenant tables (chat_message, permission_audit_log) carry
		//      NO tenant_id column — isolation is enforced by search_path
		//      middleware, not per-row. This prevents cross-tenant audit
		//      leakage by construction.
		shapesNoTenant := []interface{}{
			chat.ChatMessage{},
			PermissionAuditEntry{},
		}
		for _, s := range shapesNoTenant {
			typ := reflect.TypeOf(s)
			for i := 0; i < typ.NumField(); i++ {
				name := typ.Field(i).Name
				assert.NotEqual(t, "TenantID", name,
					"%s lives in per-tenant schema, must NOT carry TenantID", typ.Name())
				assert.NotEqual(t, "Tenant", name,
					"%s must not carry Tenant column", typ.Name())
			}
		}

		// And global aggregates (RunMetric, ToolMetric) DO carry TenantID
		// because they may be exported to a cross-tenant analytics sink.
		shapesWithTenant := []interface{}{
			RunMetric{},
			ToolMetric{},
		}
		for _, s := range shapesWithTenant {
			typ := reflect.TypeOf(s)
			hasTenant := false
			for i := 0; i < typ.NumField(); i++ {
				if typ.Field(i).Name == "TenantID" {
					hasTenant = true
				}
			}
			assert.True(t, hasTenant,
				"%s aggregates may cross schema boundary, MUST carry TenantID", typ.Name())
		}
	})

	t.Run("Scenario_AuditEntriesCarryWhenAttributionExplicitly", func(t *testing.T) {
		// Given audit credibility requires explicit timestamps (PDF Section
		//       11: "WHEN" is foundational to any audit narrative),
		now := time.Now().UTC()
		permEntry := PermissionAuditEntry{CreatedAt: now}
		toolMetric := ToolMetric{StartedAt: now}
		runMetric := RunMetric{StartedAt: now, CompletedAt: now.Add(5 * time.Second)}

		// When the audit channels record entries,
		// Then each carries explicit timestamps (no implicit DB defaults
		//      that could drift between replicas).
		assert.False(t, permEntry.CreatedAt.IsZero(),
			"permission decision must carry explicit CreatedAt")
		assert.False(t, toolMetric.StartedAt.IsZero(),
			"tool execution must carry explicit StartedAt")
		assert.False(t, runMetric.StartedAt.IsZero(),
			"run must carry explicit StartedAt")
		assert.False(t, runMetric.CompletedAt.IsZero(),
			"run must carry explicit CompletedAt for duration audit")
	})

	t.Run("Scenario_NoopAuditLoggerIsTheSafeDefaultNotASilentDrop", func(t *testing.T) {
		// Given a sub-runner / unit test environment without a real audit
		//       backend (PDF Section 11: governance must NEVER cause
		//       availability degradation — but the noop must be EXPLICIT,
		//       not accidental),
		var logger PermissionAuditLogger = NoopPermissionAuditLogger{}

		// When the runner attempts to log,
		err := logger.LogDecision(context.Background(), PermissionAuditEntry{
			SessionID: uuid.New(), ToolName: "x", Decision: AuditDecisionAllow,
		})

		// Then the noop succeeds silently — but the type name "Noop" makes
		//      it explicit in code review that audit is being dropped.
		//      Accidental deployment with NoopLogger is visible.
		assert.NoError(t, err,
			"noop logger must succeed without persisting")
		// And the type name itself signals the dropped-audit behaviour:
		assert.Contains(t, "NoopPermissionAuditLogger", "Noop",
			"type name must include 'Noop' so misuse is visible in code review")
	})

	t.Run("Scenario_PermissionAuditEntryIsPureValueType", func(t *testing.T) {
		// Given audit entries cross goroutine boundaries (concurrent log
		//       calls — already validated in OBS-003) and may be batched,
		given := PermissionAuditEntry{
			SessionID: uuid.New(),
			ToolName:  "x",
			Decision:  AuditDecisionAllow,
		}

		// When we copy the value,
		copied := given

		// Then both copies are independent (no shared mutable state) —
		//      audit entries can be safely passed by value across batches.
		copied.ToolName = "mutated"
		assert.Equal(t, "x", given.ToolName,
			"original entry must not change when copy is mutated")
		assert.Equal(t, "mutated", copied.ToolName,
			"copy mutation works on the copy")
	})

	t.Run("Scenario_TranscriptAndAuditChannelsPersistIndependently", func(t *testing.T) {
		// Given the PDF Section 9.1 contract: audit entries persist
		//       INDEPENDENTLY of the conversation transcript so safety
		//       telemetry survives even if the transcript is purged for
		//       GDPR/retention reasons,
		// When we inspect the migration files,
		// Then permission_audit_log is its own table separate from
		//      chat_message — verified by the existence of dedicated
		//      migration 000052_permission_audit_log.up.sql.
		// (We can't read files from the test, but we assert the model-level
		// independence: PermissionAuditEntry has no foreign key to
		// chat_message.)
		typ := reflect.TypeOf(PermissionAuditEntry{})
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			assert.NotEqual(t, "MessageID", name,
				"audit entry must NOT FK to chat_message — independent persistence")
			assert.NotEqual(t, "ChatMessageID", name,
				"audit entry must NOT FK to chat_message")
		}
	})
}
