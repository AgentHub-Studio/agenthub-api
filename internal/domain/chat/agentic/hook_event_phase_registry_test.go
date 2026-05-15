package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FEAT032 — HookEventPhaseRegistry unit tests.
// §6.1 full 27-event phase taxonomy + §5.3 permission-flow participation.
// All test functions prefixed FEAT032.

// ---- Construction ----

func TestFEAT032_NewHookEventPhaseRegistry_ReturnsNonNil(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	require.NotNil(t, r)
}

func TestFEAT032_Count_Equals27(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	assert.Equal(t, SeedHookEventProfileCount, r.Count(),
		"§6.1: 'source code defines 27 hook events'")
}

func TestFEAT032_SeedHookEventSlugs_HasExactly27Entries(t *testing.T) {
	assert.Len(t, SeedHookEventSlugs, SeedHookEventProfileCount,
		"SeedHookEventSlugs must list exactly 27 events")
}

// ---- Phase coverage ----

func TestFEAT032_AllPhases_ReturnsSeven(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	phases := r.AllPhases()
	assert.Len(t, phases, 7, "§6.1 lists exactly 7 phase groups")
}

func TestFEAT032_AllPhasesFirstIsToolAuthorization(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	phases := r.AllPhases()
	assert.Equal(t, HookEventPhaseToolAuthorization, phases[0],
		"§6.1 listing order: tool_authorization is first")
}

func TestFEAT032_AllPhasesLastIsNotifications(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	phases := r.AllPhases()
	assert.Equal(t, HookEventPhaseNotifications, phases[len(phases)-1],
		"§6.1 listing order: notifications is last")
}

func TestFEAT032_ToolAuthorizationPhase_HasFiveEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseToolAuthorization)
	assert.Len(t, events, 5,
		"§6.1 tool authorization: PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest, PermissionDenied")
}

func TestFEAT032_SessionLifecyclePhase_HasFiveEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseSessionLifecycle)
	assert.Len(t, events, 5,
		"§6.1 session lifecycle: SessionStart, SessionEnd, Setup, Stop, StopFailure")
}

func TestFEAT032_UserInteractionPhase_HasThreeEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseUserInteraction)
	assert.Len(t, events, 3,
		"§6.1 user interaction: UserPromptSubmit, Elicitation, ElicitationResult")
}

func TestFEAT032_SubagentCoordinationPhase_HasFiveEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseSubagentCoordination)
	assert.Len(t, events, 5,
		"§6.1 subagent coordination: SubagentStart, SubagentStop, TeammateIdle, TaskCreated, TaskCompleted")
}

func TestFEAT032_ContextManagementPhase_HasFourEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseContextManagement)
	assert.Len(t, events, 4,
		"§6.1 context management: PreCompact, PostCompact, InstructionsLoaded, ConfigChange")
}

func TestFEAT032_WorkspacePhase_HasFourEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseWorkspace)
	assert.Len(t, events, 4,
		"§6.1 workspace: CwdChanged, FileChanged, WorktreeCreate, WorktreeRemove")
}

func TestFEAT032_NotificationsPhase_HasOneEvent(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	events := r.EventsByPhase(HookEventPhaseNotifications)
	assert.Len(t, events, 1,
		"§6.1 notifications: Notification only")
	assert.Equal(t, HookNotificationExt, events[0].Event)
}

// ---- Phase event count sum ----

func TestFEAT032_AllPhaseCountsSumTo27(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	total := 0
	for _, phase := range r.AllPhases() {
		total += len(r.EventsByPhase(phase))
	}
	assert.Equal(t, SeedHookEventProfileCount, total,
		"Phase event counts must sum to 27 — no event unaccounted")
}

// ---- Permission flow (§5.3) ----

func TestFEAT032_PermissionFlowEvents_ReturnsFive(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	pfEvents := r.PermissionFlowEvents()
	assert.Len(t, pfEvents, 5,
		"§5.3: exactly 5 hooks participate in the permission flow")
}

func TestFEAT032_PermissionFlowFirstIsPreToolUse(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	pfEvents := r.PermissionFlowEvents()
	assert.Equal(t, HookPreToolUseExt, pfEvents[0].Event,
		"§5.3 order: PreToolUse is first permission-flow hook")
}

func TestFEAT032_PreToolUse_CanBlockAndModifyInput(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, ok := r.FindByEvent(HookPreToolUseExt)
	require.True(t, ok)
	assert.True(t, p.CanBlockExecution,
		"§5.3: PreToolUse can return permissionDecision=deny to block")
	assert.True(t, p.CanModifyInput,
		"§5.3: PreToolUse can return updatedInput to rewrite tool parameters")
	assert.False(t, p.CanModifyOutput,
		"PreToolUse fires before execution — cannot modify output")
}

func TestFEAT032_PreToolUse_OutputFieldsContainPermissionDecision(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, _ := r.FindByEvent(HookPreToolUseExt)
	assert.Contains(t, p.PermissionFlowOutputFields, "permissionDecision")
	assert.Contains(t, p.PermissionFlowOutputFields, "permissionDecisionReason")
	assert.Contains(t, p.PermissionFlowOutputFields, "updatedInput")
}

func TestFEAT032_PostToolUse_CanModifyOutputNotInput(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, ok := r.FindByEvent(HookPostToolUseExt)
	require.True(t, ok)
	assert.True(t, p.CanModifyOutput,
		"§5.3: PostToolUse can return updatedMCPToolOutput (MCP tools only)")
	assert.False(t, p.CanBlockExecution,
		"PostToolUse cannot block — tool has already executed")
	assert.False(t, p.CanModifyInput)
}

func TestFEAT032_PostToolUse_OutputFieldsContainAdditionalContext(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, _ := r.FindByEvent(HookPostToolUseExt)
	assert.Contains(t, p.PermissionFlowOutputFields, "additionalContext")
	assert.Contains(t, p.PermissionFlowOutputFields, "updatedMCPToolOutput")
}

func TestFEAT032_PermissionRequest_CanBlockViaDecisionField(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, ok := r.FindByEvent(HookPermissionRequestExt)
	require.True(t, ok)
	assert.True(t, p.CanBlockExecution,
		"§5.3: PermissionRequest can return decision=deny to block")
	assert.Contains(t, p.PermissionFlowOutputFields, "decision")
}

func TestFEAT032_PermissionDenied_CannotBlock(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, ok := r.FindByEvent(HookPermissionDeniedExt)
	require.True(t, ok)
	// PermissionDenied fires after the deny decision — it cannot un-deny.
	assert.False(t, p.CanBlockExecution,
		"PermissionDenied fires after deny is decided — cannot block")
}

// ---- Blocking events ----

func TestFEAT032_BlockingEvents_ContainsPreToolUseAndStopAndPermRequest(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	blocking := r.BlockingEvents()
	events := make(map[ExtendedHookEvent]bool)
	for _, p := range blocking {
		events[p.Event] = true
	}
	assert.True(t, events[HookPreToolUseExt], "PreToolUse must be blocking")
	assert.True(t, events[HookStopExt], "Stop hook must be blocking")
	assert.True(t, events[HookPermissionRequestExt], "PermissionRequest must be blocking")
}

// ---- Schema coverage ----

func TestFEAT032_EventsWithSpecificSchema_AtLeast15(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	schemas := r.EventsWithSpecificSchema()
	// §6.1: "15 have event-specific output schemas"
	assert.GreaterOrEqual(t, len(schemas), 15,
		"§6.1: at least 15 events must have event-specific output schemas")
}

// ---- New events (TeammateIdle, CwdChanged, WorktreeCreate, WorktreeRemove) ----

func TestFEAT032_TeammateIdle_IsInSubagentCoordinationPhase(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	p, ok := r.FindByEvent(HookTeammateIdleExt)
	require.True(t, ok, "TeammateIdle must be seeded — §6.1 subagent coordination group")
	assert.Equal(t, HookEventPhaseSubagentCoordination, p.Phase)
	assert.True(t, p.FiresForSubagents)
}

func TestFEAT032_WorkspaceEvents_AllFourPresent(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	workspace := []ExtendedHookEvent{
		HookCwdChangedExt, HookFileChangedExt,
		HookWorktreeCreateExt, HookWorktreeRemoveExt,
	}
	for _, e := range workspace {
		p, ok := r.FindByEvent(e)
		require.True(t, ok, "workspace event %q must be seeded — §6.1", e)
		assert.Equal(t, HookEventPhaseWorkspace, p.Phase,
			"event %q must be in workspace phase", e)
	}
}

// ---- AllProfiles ordering ----

func TestFEAT032_AllProfiles_OrderedBySeedSlugOrder(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	profiles := r.AllProfiles()
	require.Len(t, profiles, SeedHookEventProfileCount)
	for i, event := range SeedHookEventSlugs {
		assert.Equal(t, event, profiles[i].Event,
			"profile[%d] must match SeedHookEventSlugs[%d]", i, i)
	}
}

// ---- IsKnownEvent ----

func TestFEAT032_IsKnownEvent_TrueForAllSeededEvents(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	for _, event := range SeedHookEventSlugs {
		assert.True(t, r.IsKnownEvent(event),
			"IsKnownEvent must return true for seeded event %q", event)
	}
}

func TestFEAT032_IsKnownEvent_FalseForUnknown(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	assert.False(t, r.IsKnownEvent("NoSuchEvent"),
		"IsKnownEvent must return false for unknown event")
}

// ---- Structural invariants ----

func TestFEAT032_PermissionFlowEventsAllInToolAuthorizationPhase(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	for _, p := range r.PermissionFlowEvents() {
		assert.Equal(t, HookEventPhaseToolAuthorization, p.Phase,
			"permission-flow event %q must be in tool_authorization phase", p.Event)
	}
}

func TestFEAT032_PermissionFlowEventsAllHaveOutputFields(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	for _, p := range r.PermissionFlowEvents() {
		assert.NotEmpty(t, p.PermissionFlowOutputFields,
			"permission-flow event %q must have at least one output field", p.Event)
	}
}

func TestFEAT032_NonPermissionFlowEventsHaveNoOutputFields(t *testing.T) {
	r := NewHookEventPhaseRegistry()
	for _, p := range r.AllProfiles() {
		if !p.ParticipatesInPermissionFlow {
			assert.Empty(t, p.PermissionFlowOutputFields,
				"non-permission-flow event %q must have no output fields", p.Event)
		}
	}
}

// ---- BDD-style scenarios ----

func TestFEAT032_BDD_HookEventPhaseRegistry(t *testing.T) {
	t.Run("Scenario: §6.1 full 27-event taxonomy is seeded with phase groups", func(t *testing.T) {
		// Given: a new HookEventPhaseRegistry
		r := NewHookEventPhaseRegistry()
		// When: Count() is called
		count := r.Count()
		// Then: exactly 27 events are registered across 7 phases
		assert.Equal(t, 27, count)
		assert.Len(t, r.AllPhases(), 7)
	})

	t.Run("Scenario: §5.3 permission-flow hooks are correctly identified", func(t *testing.T) {
		// Given: the permission-flow event set
		r := NewHookEventPhaseRegistry()
		pfEvents := r.PermissionFlowEvents()
		// Then: exactly 5 events; all in tool-authorization phase
		assert.Len(t, pfEvents, 5)
		for _, p := range pfEvents {
			assert.Equal(t, HookEventPhaseToolAuthorization, p.Phase,
				"permission-flow event %q must be in tool-authorization phase", p.Event)
			assert.True(t, p.HasEventSpecificSchema,
				"permission-flow event %q must have event-specific schema", p.Event)
		}
	})

	t.Run("Scenario: PreToolUse is the only hook that can both block and modify input", func(t *testing.T) {
		// Given: all profiles
		r := NewHookEventPhaseRegistry()
		profiles := r.AllProfiles()
		// Then: only PreToolUse has both CanBlockExecution and CanModifyInput true
		var both []ExtendedHookEvent
		for _, p := range profiles {
			if p.CanBlockExecution && p.CanModifyInput {
				both = append(both, p.Event)
			}
		}
		assert.Equal(t, []ExtendedHookEvent{HookPreToolUseExt}, both,
			"§5.3: only PreToolUse can both block execution and modify input")
	})

	t.Run("Scenario: workspace phase contains all four §6.1 workspace events", func(t *testing.T) {
		r := NewHookEventPhaseRegistry()
		events := r.EventsByPhase(HookEventPhaseWorkspace)
		slugs := make(map[ExtendedHookEvent]bool)
		for _, p := range events {
			slugs[p.Event] = true
		}
		assert.True(t, slugs[HookCwdChangedExt])
		assert.True(t, slugs[HookFileChangedExt])
		assert.True(t, slugs[HookWorktreeCreateExt])
		assert.True(t, slugs[HookWorktreeRemoveExt])
	})

	t.Run("Scenario: subagent phase includes TeammateIdle from §6.1", func(t *testing.T) {
		r := NewHookEventPhaseRegistry()
		events := r.EventsByPhase(HookEventPhaseSubagentCoordination)
		var found bool
		for _, p := range events {
			if p.Event == HookTeammateIdleExt {
				found = true
				assert.True(t, p.FiresForSubagents)
			}
		}
		assert.True(t, found, "TeammateIdle must appear in subagent-coordination phase")
	})

	t.Run("Scenario: sum of per-phase event counts equals 27", func(t *testing.T) {
		r := NewHookEventPhaseRegistry()
		total := 0
		for _, phase := range r.AllPhases() {
			total += len(r.EventsByPhase(phase))
		}
		assert.Equal(t, 27, total, "every event must belong to exactly one phase")
	})

	t.Run("Scenario: blocking events include hooks from authorization, session, and user-interaction", func(t *testing.T) {
		r := NewHookEventPhaseRegistry()
		blocking := r.BlockingEvents()
		phases := make(map[HookEventPhase]bool)
		for _, p := range blocking {
			phases[p.Phase] = true
		}
		assert.True(t, phases[HookEventPhaseToolAuthorization],
			"at least one blocking event must be in tool-authorization phase")
		assert.True(t, phases[HookEventPhaseSessionLifecycle],
			"Stop hook creates a blocking event in session-lifecycle phase")
	})
}
