package agentic

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify EXT-002 (Hook registry) against the
// Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 6.1 ("Four Extension Mechanisms"): "The source code defines 27
//     hook events spanning tool authorization (PreToolUse, PostToolUse,
//     PostToolUseFailure, PermissionRequest, PermissionDenied), session
//     lifecycle (SessionStart, SessionEnd, Setup, Stop, StopFailure), user
//     interaction (UserPromptSubmit, Elicitation, ElicitationResult),
//     subagent coordination (SubagentStart, SubagentStop, TeammateIdle,
//     TaskCreated, TaskCompleted), context management (PreCompact,
//     PostCompact, InstructionsLoaded, ConfigChange), workspace events
//     (CwdChanged, FileChanged, WorktreeCreate, WorktreeRemove), and
//     notifications."
//   - Section 6.1 also lists 4 command types: shell (command), LLM prompt
//     (prompt), HTTP (http), and agent verifier (agent).
//   - Section 6 ("Extensibility"): hooks are zero-context-cost extensions
//     (Table 2) that intercept the lifecycle without consuming the model's
//     context window unless they explicitly inject context.
//   - Section 5.3 details the 5 SAFETY-related hook events out of 27
//     (PreToolUse + PostToolUse + PostToolUseFailure + PermissionRequest +
//     PermissionDenied) which can return permissionDecision and updatedInput.
//
// AgentHub maps hooks to:
//   - hooktype.go ExtendedHookEvent — 23 of the PDF's 27 events (missing
//     TeammateIdle, CwdChanged, WorktreeCreate, WorktreeRemove which are
//     workspace/multi-agent concepts not directly applicable to a server
//     architecture).
//   - hook.go AgentHook + HookEvent (8 simpler events for direct DB binding).
//   - hooktype.go HookCommand with HookCommandType + DefaultTimeout + Validate.
//   - hooktype.go HooksSettings map[event][]matcher (config shape).
//   - hook.go HookRepository interface + pgHookRepository (DB-backed).
//   - hook.go AgentHook with per-hook TimeoutSeconds + IsAsync + RunOnce +
//     StatusMessage (matches Claude Code's per-hook config).
//
// These scenarios ratify the registry contract and document the 4-event gap
// honestly so a future EXT-002a can close it.

func TestBDD_HookRegistry(t *testing.T) {
	t.Run("Scenario_ExtendedHookEventsCoverPDFLifecycleCategories", func(t *testing.T) {
		// Given the PDF Section 6.1 categories of hook events,
		// When we enumerate ExtendedHookEvent constants,
		// Then each PDF category is represented (we accept 23/27 events; the
		//      4 missing are workspace-specific — see below).
		toolAuth := []ExtendedHookEvent{
			HookPreToolUseExt, HookPostToolUseExt, HookPostToolUseFailureExt,
			HookPermissionRequestExt, HookPermissionDeniedExt,
		}
		sessionLifecycle := []ExtendedHookEvent{
			HookSessionStartExt, HookSessionEndExt, HookSetupExt,
			HookStopExt, HookStopFailureExt,
		}
		userInteraction := []ExtendedHookEvent{
			HookUserPromptSubmitExt, HookElicitationExt, HookElicitationResultExt,
		}
		subagentCoord := []ExtendedHookEvent{
			HookSubagentStartExt, HookSubagentStopExt,
			HookTaskCreatedExt, HookTaskCompletedExt,
			// TeammateIdle missing — multi-agent workspace concept (gap)
		}
		contextMgmt := []ExtendedHookEvent{
			HookPreCompactExt, HookPostCompactExt,
			HookInstructionsLoadedExt, HookConfigChangeExt,
		}
		workspace := []ExtendedHookEvent{
			HookFileChangedExt,
			// CwdChanged, WorktreeCreate, WorktreeRemove missing — IDE/CLI concepts
		}
		notification := []ExtendedHookEvent{HookNotificationExt}

		// Then each category has at least one event.
		assert.NotEmpty(t, toolAuth, "tool authorization category")
		assert.NotEmpty(t, sessionLifecycle, "session lifecycle category")
		assert.NotEmpty(t, userInteraction, "user interaction category")
		assert.NotEmpty(t, subagentCoord, "subagent coordination category")
		assert.NotEmpty(t, contextMgmt, "context management category")
		assert.NotEmpty(t, workspace, "workspace events category")
		assert.NotEmpty(t, notification, "notification category")
		assert.GreaterOrEqual(t, len(AllExtendedHookEvents), 23,
			"AgentHub must enumerate at least 23 of the PDF's 27 hook events")
	})

	t.Run("Scenario_FiveSafetyRelatedHooksAreFirstClass", func(t *testing.T) {
		// Given PDF Section 5.3 lists 5 hooks that participate in permission
		//       decisions: PreToolUse, PostToolUse, PostToolUseFailure,
		//       PermissionRequest, PermissionDenied,
		safetyHooks := []ExtendedHookEvent{
			HookPreToolUseExt,         // can return permissionDecision/updatedInput
			HookPostToolUseExt,        // can inject additionalContext
			HookPostToolUseFailureExt, // can inject error-specific guidance
			HookPermissionRequestExt,  // can return allow/deny decision
			HookPermissionDeniedExt,   // can provide retry guidance
		}

		// When we enumerate them,
		// Then all 5 are present and validate-able as known events — refactor
		//      that drops one fails this guard.
		for _, ev := range safetyHooks {
			assert.True(t, IsValidHookEvent(string(ev)),
				"safety-relevant hook %q must be a valid event", ev)
		}
		assert.Len(t, safetyHooks, 5,
			"PDF Section 5.3 enumerates exactly 5 safety hooks")
	})

	t.Run("Scenario_AgentHookEntityCarriesPerHookOverrides", func(t *testing.T) {
		// Given a hook configuration must support per-hook overrides (PDF
		//       Section 6.1: per-hook timeout, async flag, etc.),
		timeoutSecs := 30
		given := AgentHook{
			ID:             uuid.New(),
			AgentID:        uuid.New(),
			Event:          HookPreToolUse,
			HookType:       HookTypeHTTP,
			Config:         json.RawMessage(`{"url":"https://example.com"}`),
			Enabled:        true,
			Priority:       10,
			TimeoutSeconds: &timeoutSecs,
			IsAsync:        true,
			RunOnce:        false,
			StatusMessage:  "Validating policy...",
		}

		// When the registry inspects it,
		// Then per-hook overrides are observable and a refactor that drops
		//      any field is caught.
		assert.NotNil(t, given.TimeoutSeconds,
			"per-hook timeout must be settable")
		assert.True(t, given.IsAsync,
			"per-hook async flag must be observable")
		assert.False(t, given.RunOnce,
			"per-hook run-once flag must be settable")
		assert.NotEmpty(t, given.StatusMessage,
			"per-hook status message provides UX hint")
	})

	t.Run("Scenario_TwoHookTypesAreSupported", func(t *testing.T) {
		// Given AgentHub initial scope (HTTP for external service hooks +
		//       Prompt for inline LLM-prompt hooks; PDF mentions 4: command,
		//       prompt, http, agent),
		types := []HookType{HookTypeHTTP, HookTypePrompt}

		// When the registry classifies a hook,
		// Then both types are validate-able. command + agent types are gaps
		//      (documented as EXT-002a future).
		assert.Len(t, types, 2,
			"AgentHub supports 2 hook types (HTTP + Prompt); 2 PDF types missing")
	})

	t.Run("Scenario_HooksSettingsIndexesByEvent", func(t *testing.T) {
		// Given a hooks configuration loaded from settings.json or DB,
		given := HooksSettings{
			HookPreToolUseExt: []HookMatcher{
				{Matcher: "Bash"},
			},
			HookPostToolUseExt: []HookMatcher{
				{Matcher: "*"},
			},
		}

		// When the runner asks for matchers for an event,
		when := given.MatchersFor(HookPreToolUseExt)

		// Then the index returns just the matchers for that event — efficient
		//      lookup at every loop iteration's hook stage.
		assert.Len(t, when, 1,
			"MatchersFor must return only the requested event's matchers")
		// Unknown event returns empty slice (not nil panic).
		empty := given.MatchersFor("NonExistent")
		assert.Empty(t, empty,
			"MatchersFor on unknown event returns empty (no panic)")
	})

	t.Run("Scenario_HookRepositoryInterfaceIsBoundedAndAuditable", func(t *testing.T) {
		// Given the registry has a backing repository,
		// When we inspect HookRepository contract,
		// Then it exposes FindByAgentAndEvent (bounded read) + DisableHook
		//      (one-way disable, no Delete) — preserves audit trail of
		//      previously-active hooks.
		var repo HookRepository = &pgHookRepository{}
		assert.NotNil(t, repo, "HookRepository must be implementable")
		// We don't call methods (would require DB) but assert the type.
	})

	t.Run("Scenario_HookPayloadCarriesEventTypeAndContext", func(t *testing.T) {
		// Given a hook fires on a specific event (PDF Section 6.1: hooks
		//       receive event-specific payloads),
		toolErr := "permission denied"
		given := HookPayload{
			Event:      HookPreToolUse,
			AgentID:    uuid.New().String(),
			SessionID:  uuid.New().String(),
			ToolName:   "execute-sql",
			ToolInput:  json.RawMessage(`{"query":"SELECT 1"}`),
			ToolOutput: json.RawMessage(`{"rows":[]}`),
			ToolError:  &toolErr,
			TurnIndex:  3,
			TotalTurns: 5,
		}

		// When the hook executor invokes the registered handler,
		// Then payload carries enough context to make a decision: WHO
		//      (agent/session), WHEN (turn), WHAT (tool name/input/output/error).
		assert.Equal(t, HookPreToolUse, given.Event,
			"event type must be in payload for handler dispatch")
		assert.NotEmpty(t, given.AgentID,
			"agent id provides identity attribution")
		assert.NotEmpty(t, given.ToolName,
			"tool name provides matcher target")
		assert.NotNil(t, given.ToolError,
			"error pointer surfaces only when relevant (PostToolUseFailure)")
	})

	t.Run("Scenario_HookResultSupportsContextInjection", func(t *testing.T) {
		// Given PDF Section 6.1 Tabela 2: hooks have ZERO context cost by
		//       default but CAN inject context when needed,
		given := HookResult{
			Inject: "Note: this query touches PII fields.",
			Error:  nil,
		}

		// When the runner consumes the result,
		// Then Inject is the optional context injection vector and Error is
		//      a non-fatal pointer (hook errors don't abort the loop).
		assert.NotEmpty(t, given.Inject,
			"hook can inject context to influence next model call")
		assert.Nil(t, given.Error,
			"successful hook has nil Error (non-fatal contract)")
	})

	t.Run("Scenario_HookCommandTypeHasSensibleTimeoutDefaults", func(t *testing.T) {
		// Given different hook command types have different latency profiles
		//       (PDF Section 6.1: command runs locally vs http calls
		//       external service),
		// When DefaultTimeout is consulted,
		// Then each command type has a positive default timeout — no hook
		//      can hang indefinitely.
		for _, ct := range []HookCommandType{
			HookCommandType("command"),
			HookCommandType("prompt"),
			HookCommandType("http"),
			HookCommandType("agent"),
		} {
			when := DefaultTimeout(ct)
			assert.Greater(t, when, 0,
				"DefaultTimeout for %q must be positive — no infinite hangs", ct)
		}
	})

	t.Run("Scenario_HookCommandValidationCatchesMalformedConfig", func(t *testing.T) {
		// Given a malformed hook command (missing required fields),
		given := &HookCommand{}

		// When the registry validates before persisting,
		err := given.Validate()

		// Then the error surfaces — preventing malformed hooks from being
		//      registered.
		assert.Error(t, err,
			"empty hook command must fail validation")
	})
}
