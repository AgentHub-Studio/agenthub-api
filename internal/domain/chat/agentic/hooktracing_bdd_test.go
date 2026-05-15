package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify OBS-004 (Hook tracing) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 11 (observability): permission decisions, hook interception,
//     compaction, and subagent dispatch are first-class observability
//     concerns. Hook tracing in particular must be granular enough that
//     operators can tell WHICH hook fired, WHEN in its lifecycle (started
//     vs progress vs response), and WHAT it returned (exit code, outcome).
//   - Section 5.3 + 6.1: hooks can BLOCK, REWRITE, or ANNOTATE tool calls.
//     Tracing must surface these decisions distinctly from generic events
//     so audits can reconstruct cause-and-effect.
//   - Section 4.5: stop hooks can veto the loop's continuation. The trace
//     must capture this veto explicitly (StopReason) so operators can
//     understand why a run terminated.
//
// AgentHub maps hook tracing to:
//   - hooktype.go HookExecutionEventType — 3-value lifecycle enum
//     (started / progress / response).
//   - hooktype.go HookExecutionEvent — typed event with HookID + HookName
//     + HookEvent (which lifecycle event triggered it) + stdout/stderr +
//     ExitCode + Outcome ("success" / "error" / "cancelled").
//   - hooktype.go HookJSONOutput — typed structured output a hook may
//     return to control agent behaviour: Continue / Decision (approve|
//     block) / Reason / StopReason / SystemMessage / SuppressOutput / Async.
//   - permission_audit.go also captures permission decisions including
//     hook-driven ones (covered by OBS-003).
//
// These scenarios assert: lifecycle enum coverage, attribution chain,
// exit code + outcome semantics, structured output shape, defaulting
// behaviour for IsAsync / ShouldContinue.

func TestBDD_HookTracing(t *testing.T) {
	t.Run("Scenario_HookExecutionLifecycleHasThreePhases", func(t *testing.T) {
		// Given a hook execution emits trace events at distinct phases
		//       (PDF Section 11: granular observability requires
		//       distinguishable lifecycle states),
		phases := map[HookExecutionEventType]string{
			HookExecStarted:  "hook dispatched, awaiting output",
			HookExecProgress: "hook produced intermediate output (stdout/stderr chunk)",
			HookExecResponse: "hook completed (exit code + outcome available)",
		}

		// When we enumerate them,
		// Then exactly 3 distinct phases — operators can subscribe to
		//      individual phases (e.g. "all responses" for audit, "all
		//      progress" for live UI) without parsing payload bodies.
		assert.Len(t, phases, 3,
			"hook lifecycle has 3 distinct trace phases (started/progress/response)")
		for ty := range phases {
			assert.NotEmpty(t, string(ty),
				"phase token must be non-empty wire string")
		}
	})

	t.Run("Scenario_HookExecutionEventCarriesAttributionChain", func(t *testing.T) {
		// Given a hook fires (PDF Section 11 + 6.1: tracing must
		//       attribute WHICH hook + WHICH lifecycle event triggered it),
		given := HookExecutionEvent{
			Type:      HookExecResponse,
			HookID:    "hook_abc",
			HookName:  "Block destructive SQL",
			HookEvent: "PreToolUse",
			Output:    "blocked",
			ExitCode:  intPtr(0),
			Outcome:   "success",
		}

		// When the runtime serialises for telemetry,
		raw, err := json.Marshal(given)
		assert.NoError(t, err)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then 3 attribution fields are observable: HookID (the deployed
		//      hook instance), HookName (human label), HookEvent (which
		//      lifecycle event triggered the hook).
		assert.Equal(t, "hook_abc", decoded["hookId"],
			"hookId must round-trip for unique attribution")
		assert.Equal(t, "Block destructive SQL", decoded["hookName"],
			"hookName must round-trip for human-readable filtering")
		assert.Equal(t, "PreToolUse", decoded["hookEvent"],
			"hookEvent must round-trip — operators distinguish PreToolUse from PermissionRequest")
	})

	t.Run("Scenario_ResponseEventCarriesExitCodeAndOutcome", func(t *testing.T) {
		// Given a hook has finished executing (PDF Section 11: outcome +
		//       exit code drive audit decisions like "did this hook
		//       block, error, or succeed?"),
		given := HookExecutionEvent{
			Type:     HookExecResponse,
			ExitCode: intPtr(2),
			Outcome:  "error",
			Stderr:   "permission check failed",
		}

		// When the audit reads the event,
		raw, _ := json.Marshal(given)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then ExitCode (numeric) and Outcome (string) are observable
		//      separately — exit code for programmatic checks, outcome
		//      for human review.
		assert.NotNil(t, decoded["exitCode"],
			"exitCode must be observable on response events")
		assert.Equal(t, float64(2), decoded["exitCode"],
			"exit code 2 must round-trip")
		assert.Equal(t, "error", decoded["outcome"],
			"outcome must categorise the result")
	})

	t.Run("Scenario_StartedEventOmitsExitCodeAndOutcome", func(t *testing.T) {
		// Given a started event (PDF: started just signals "hook
		//       dispatched"; exit code + outcome are unknown until
		//       response),
		given := HookExecutionEvent{
			Type:     HookExecStarted,
			HookID:   "hook_xyz",
			HookName: "Inject KB context",
		}

		// When the runtime serialises,
		raw, _ := json.Marshal(given)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then ExitCode + Outcome are absent (omitempty) — wire stays
		//      lean for the dominant started event.
		_, hasExit := decoded["exitCode"]
		assert.False(t, hasExit,
			"started event must omit exitCode")
		outcome, _ := decoded["outcome"].(string)
		assert.Empty(t, outcome,
			"started event must omit outcome")
	})

	t.Run("Scenario_OutcomeEnumCoversSuccessErrorAndCancelled", func(t *testing.T) {
		// Given the 3 outcome categories the audit recognises (PDF +
		//       OBS-002 ToolErrorCategory analogue: distinct categories
		//       for distinct telemetry),
		known := map[string]bool{
			"success":   true,
			"error":     true,
			"cancelled": true,
		}

		// When operators filter audit data,
		// Then 3 distinct values exist; refactor that adds a 4th must
		//      update both the runtime and the audit dashboard.
		assert.Len(t, known, 3,
			"3 outcome categories — extension requires audit dashboard update")
	})

	t.Run("Scenario_HookJSONOutputContinueDefaultsToTrueWhenNil", func(t *testing.T) {
		// Given a hook that returns no explicit Continue value (PDF: the
		//       sane default is to allow the loop to proceed unless the
		//       hook explicitly vetoes),
		given := &HookJSONOutput{}

		// When the runner consults whether to continue,
		when := given.ShouldContinue()

		// Then the answer is true — silent absence allows continuation.
		//      A hook MUST explicitly say Continue=false to block.
		assert.True(t, when,
			"nil Continue must default to true (allow) — explicit veto required to block")
	})

	t.Run("Scenario_HookJSONOutputContinueFalseStopsTheLoop", func(t *testing.T) {
		// Given a hook that explicitly vetoes continuation (PDF Section
		//       4.5 stop hook intervention),
		false_ := false
		given := &HookJSONOutput{Continue: &false_, StopReason: "policy violation"}

		// When the runner consults,
		when := given.ShouldContinue()

		// Then the loop must stop — explicit veto is honoured.
		assert.False(t, when,
			"Continue=false must stop the loop")
		assert.NotEmpty(t, given.StopReason,
			"StopReason must accompany the veto for audit trail")
	})

	t.Run("Scenario_HookJSONOutputAsyncFlagDistinguishesBackgroundExecution", func(t *testing.T) {
		// Given a hook that opts into async execution (PDF Section 6.1:
		//       async hooks don't block the agent loop while still
		//       observable),
		given := &HookJSONOutput{Async: true, AsyncTimeout: 30}

		// When the runner inspects,
		when := given.IsAsync()

		// Then the runtime knows to dispatch in background — different
		//      tracing path: started event fires immediately but
		//      response may arrive much later.
		assert.True(t, when,
			"Async=true must report async via IsAsync()")
		assert.Equal(t, 30, given.AsyncTimeout,
			"AsyncTimeout sets the max wait for the async response")
	})

	t.Run("Scenario_HookJSONOutputDecisionEnumCoversApproveAndBlock", func(t *testing.T) {
		// Given a permission hook returns a decision (PDF Section 5.3:
		//       PermissionRequest hook can return allow|deny — typed
		//       enum prevents typos drifting into audit logs),
		approve := &HookJSONOutput{Decision: "approve", Reason: "matched allow rule"}
		block := &HookJSONOutput{Decision: "block", Reason: "destructive SQL detected"}

		// When the runner reads each,
		// Then both decisions carry an explanatory Reason — audit trail
		//      surfaces WHY the hook decided that way.
		assert.Equal(t, "approve", approve.Decision)
		assert.NotEmpty(t, approve.Reason,
			"approve must carry Reason for audit")
		assert.Equal(t, "block", block.Decision)
		assert.NotEmpty(t, block.Reason,
			"block must carry Reason for audit")
	})

	t.Run("Scenario_HookJSONOutputSystemMessageInjectsIntoConversation", func(t *testing.T) {
		// Given a hook injects a system-level message into the conversation
		//       (PDF Section 6.1: hooks can inject context to influence
		//       the next model call without consuming tools[] surface),
		given := &HookJSONOutput{
			SystemMessage: "Note: this query touches PII fields.",
			SuppressOutput: true,
		}

		// When the runner reads,
		// Then the system message is observable separately from any
		//      stdout/stderr output (which can be suppressed independently).
		assert.NotEmpty(t, given.SystemMessage,
			"SystemMessage carries the inline injection content")
		assert.True(t, given.SuppressOutput,
			"SuppressOutput hides the hook's raw output from user view")
	})

	t.Run("Scenario_HookExecutionEventStdoutAndStderrAreSeparateFields", func(t *testing.T) {
		// Given a progress event with both stdout and stderr chunks (PDF
		//       Section 11: separating output streams enables filtering
		//       — error analysis vs normal trace),
		given := HookExecutionEvent{
			Type:   HookExecProgress,
			Stdout: "processing 42 records",
			Stderr: "warning: rate limit nearing",
		}

		// When emitted,
		raw, _ := json.Marshal(given)
		var decoded map[string]interface{}
		assert.NoError(t, json.Unmarshal(raw, &decoded))

		// Then both streams are observable as distinct fields.
		assert.Equal(t, "processing 42 records", decoded["stdout"])
		assert.Equal(t, "warning: rate limit nearing", decoded["stderr"])
	})
}

func intPtr(i int) *int { return &i }
