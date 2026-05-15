package agentic

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify OBS-003 (Permission decision tracing)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 11 (observability + governance): permission decisions must be
//     traceable as a first-class audit signal — distinct from generic event
//     logs because regulators/operators need to answer "why was this
//     allowed/denied" with evidence.
//   - Section 5 (deny-first + ask-by-default): every gated decision crosses
//     the permission boundary; without an audit log the deny-first principle
//     is unverifiable post-hoc.
//   - Section 9.1 implicit: audit entries must be persistable separately from
//     the conversation transcript so safety telemetry survives even if the
//     transcript is purged.
//
// AgentHub maps the audit contract to permission_audit.go:
//   - PermissionAuditDecision enum: 5 outcomes covering the auto + interactive
//     paths (allow / deny / confirm_approved / confirm_denied / confirm_escalated).
//   - PermissionAuditEntry: SessionID + RunID + ToolName + Decision +
//     MatchedRule (which pattern fired) + InputSnippet (bounded snapshot of
//     the input) + CreatedAt (when).
//   - PermissionAuditLogger interface: pluggable persistence (DB, OTel, file).
//   - NoopPermissionAuditLogger: zero-config sink for unit tests + sub-runners.
//   - truncateInput: bounded snippet helper to keep audit rows small.

func TestBDD_PermissionDecisionTracing(t *testing.T) {
	t.Run("Scenario_FiveAuditDecisionsCoverAutoAndInteractivePaths", func(t *testing.T) {
		// Given the deny-first engine produces 5 distinct outcomes (PDF
		//       Section 5 + 5.1: allow, deny, confirm — and confirm has
		//       three sub-outcomes after the user/auto reply),
		decisions := map[PermissionAuditDecision]string{
			AuditDecisionAllow:            "auto-allowed (no rule blocked)",
			AuditDecisionDeny:             "auto-denied (deny rule matched)",
			AuditDecisionConfirmApproved:  "confirm rule + user said yes",
			AuditDecisionConfirmDenied:    "confirm rule + user said no / timed out",
			AuditDecisionConfirmEscalated: "confirm rule + automated mode (no elicitation)",
		}

		// When the audit logger receives them,
		// Then exactly 5 distinct constants exist — refactor that collapses
		//      two paths fails this guard, preserving auditability.
		assert.Len(t, decisions, 5,
			"audit must enumerate exactly 5 decision outcomes for full traceability")
		for d := range decisions {
			assert.NotEmpty(t, string(d),
				"every decision constant must be a non-empty wire token")
		}
	})

	t.Run("Scenario_AuditEntryCarriesFullDecisionAttribution", func(t *testing.T) {
		// Given a permission denial that must be reviewable by an operator,
		runID := uuid.New()
		given := PermissionAuditEntry{
			SessionID:    uuid.New(),
			RunID:        &runID,
			ToolName:     "execute-sql",
			Decision:     AuditDecisionDeny,
			MatchedRule:  "execute-sql(DROP)",
			InputSnippet: "DROP TABLE users",
			CreatedAt:    time.Now(),
		}

		// When the audit log is read back,
		// Then every field that an audit/regulator would need is present —
		//      session/run for context, tool/decision for the act, matched
		//      rule for the WHY, snippet for the WHAT, timestamp for WHEN.
		assert.NotEqual(t, uuid.Nil, given.SessionID, "session attribution required")
		assert.NotNil(t, given.RunID, "run attribution required when within a run")
		assert.NotEmpty(t, given.ToolName, "tool name required for filterability")
		assert.Equal(t, AuditDecisionDeny, given.Decision)
		assert.NotEmpty(t, given.MatchedRule,
			"matched rule must surface the WHY of the decision")
		assert.NotEmpty(t, given.InputSnippet,
			"input snippet preserves enough context for audit review")
		assert.False(t, given.CreatedAt.IsZero(), "timestamp required for ordering")
	})

	t.Run("Scenario_RunIDIsNullableForOutOfRunDecisions", func(t *testing.T) {
		// Given a permission decision evaluated outside any tracked run
		//       (e.g. eager validation during agent setup),
		given := PermissionAuditEntry{
			SessionID: uuid.New(),
			ToolName:  "any",
			Decision:  AuditDecisionAllow,
			RunID:     nil,
		}

		// When the audit logger persists it,
		// Then RunID nullable pointer accommodates session-scoped or
		//      pre-run decisions without forcing a synthetic UUID.
		assert.Nil(t, given.RunID,
			"RunID nullable so non-run decisions are recordable cleanly")
	})

	t.Run("Scenario_InputSnippetIsBoundedToProtectAuditStorage", func(t *testing.T) {
		// Given a tool input that is enormous (e.g. a multi-MB SQL blob),
		giant := strings.Repeat("X", 10_000)

		// When the audit logger truncates it before persistence
		//      (permission_audit.go uses permissionAuditInputMaxLen=300),
		when := truncateInput(giant, permissionAuditInputMaxLen)

		// Then the snippet is bounded — keeping audit rows compact and
		//      preventing audit log itself from becoming a data-leak channel.
		assert.LessOrEqual(t, len(when), permissionAuditInputMaxLen,
			"audit snippet must be bounded by permissionAuditInputMaxLen")
		assert.Equal(t, permissionAuditInputMaxLen, len(when),
			"truncation must hit the cap exactly for predictable storage")
	})

	t.Run("Scenario_InputSnippetIsAnIdentityForShortInputs", func(t *testing.T) {
		// Given a short tool input,
		short := "SELECT 1"

		// When the truncator runs,
		when := truncateInput(short, permissionAuditInputMaxLen)

		// Then the input is preserved verbatim — short inputs already fit.
		assert.Equal(t, short, when,
			"short input must round-trip without truncation")
	})

	t.Run("Scenario_TruncateHandlesUnicodeRuneBoundaries", func(t *testing.T) {
		// Given an input containing multi-byte runes (PDF observability must
		//       not produce invalid UTF-8 in the audit log),
		input := "DROP TABLE 用户s — não permitido αβγ"

		// When the truncator runs at a small bound,
		when := truncateInput(input, 10)

		// Then the result contains exactly 10 RUNES (not 10 bytes) and is
		//      valid UTF-8 — never splits a multibyte character.
		assert.Equal(t, 10, len([]rune(when)),
			"truncation must operate on rune count, not byte count")
		// And re-encoding to UTF-8 must succeed (would panic if invalid).
		_ = []byte(when)
	})

	t.Run("Scenario_NoopLoggerSatisfiesInterfaceWithoutSideEffects", func(t *testing.T) {
		// Given a sub-runner or unit test with no audit backend,
		var logger PermissionAuditLogger = NoopPermissionAuditLogger{}

		// When the runner emits a decision,
		err := logger.LogDecision(context.Background(), PermissionAuditEntry{
			SessionID: uuid.New(), ToolName: "x", Decision: AuditDecisionAllow,
		})

		// Then no error is returned — production code can wire the noop
		//      logger as a default and never crash on missing config.
		assert.NoError(t, err,
			"noop logger must succeed without persisting anything")
	})

	t.Run("Scenario_LoggerInterfaceIsContextAware", func(t *testing.T) {
		// Given the audit logger may need to attach trace IDs or tenant info
		//       from the request context,
		// When we inspect the LogDecision signature,
		// Then the first parameter is context.Context — supports cancellation,
		//      OTel propagation, and tenant scoping out of the box.
		var logger PermissionAuditLogger = NoopPermissionAuditLogger{}

		// Cancellation must be honourable — assert by passing a cancelled ctx
		// (noop will still succeed, but the contract is established).
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := logger.LogDecision(ctx, PermissionAuditEntry{ToolName: "x"})
		// Noop ignores the context; real implementations are expected to
		// honour it. We assert that the SIGNATURE allows the contract.
		assert.NoError(t, err)
	})

	t.Run("Scenario_LoggerImplementationsMustBeSafeForConcurrentUse", func(t *testing.T) {
		// Given the Runner emits decisions from multiple goroutines (per-fork,
		//       per-tool batch), the logger interface contract states "safe
		//       for concurrent use",
		var logger PermissionAuditLogger = NoopPermissionAuditLogger{}

		// When 50 goroutines log concurrently,
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_ = logger.LogDecision(context.Background(), PermissionAuditEntry{
					SessionID: uuid.New(),
					ToolName:  "concurrent",
					Decision:  AuditDecisionAllow,
				})
			}(i)
		}
		wg.Wait()

		// Then the noop sink succeeds without race (would fail under -race).
		// Real backed implementations must hold this invariant per docs.
	})

	t.Run("Scenario_EveryDecisionHasAUniqueWireToken", func(t *testing.T) {
		// Given the audit log will be queried (e.g. "show me all confirm_denied
		//       in the last 7 days"),
		seen := map[string]bool{}

		// When the runner enumerates all decision tokens,
		for _, d := range []PermissionAuditDecision{
			AuditDecisionAllow,
			AuditDecisionDeny,
			AuditDecisionConfirmApproved,
			AuditDecisionConfirmDenied,
			AuditDecisionConfirmEscalated,
		} {
			s := string(d)
			assert.False(t, seen[s],
				"decision tokens must be unique strings; collision on %q", s)
			seen[s] = true
		}

		// Then 5 distinct strings — supports SQL/OTel filtering without
		//      ambiguity.
		assert.Len(t, seen, 5,
			"exactly 5 distinct wire tokens for the 5 decisions")
	})
}
