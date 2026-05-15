package agentic

// HookEventPhaseRegistry is the §6.1 + §5.3 authoritative taxonomy of all 27 hook
// events defined in Claude Code (arXiv:2604.14228v1).
//
// §6.1 (page 17) groups the 27 events into seven phases:
//
//	"The source code defines 27 hook events spanning tool authorization
//	(PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest, PermissionDenied),
//	session lifecycle (SessionStart, SessionEnd, Setup, Stop, StopFailure),
//	user interaction (UserPromptSubmit, Elicitation, ElicitationResult),
//	subagent coordination (SubagentStart, SubagentStop, TeammateIdle, TaskCreated,
//	TaskCompleted), context management (PreCompact, PostCompact, InstructionsLoaded,
//	ConfigChange), workspace events (CwdChanged, FileChanged, WorktreeCreate,
//	WorktreeRemove), and notifications (coreTypes.ts, coreSchemas.ts)."
//
// §5.3 (page 14) identifies 5 events that participate directly in the permission flow,
// each with a Zod-validated output schema (types/hooks.ts):
//
//   - PreToolUse: permissionDecision, permissionDecisionReason, updatedInput
//   - PostToolUse: additionalContext, updatedMCPToolOutput (MCP tools only)
//   - PostToolUseFailure: additionalContext
//   - PermissionDenied: retry guidance fields
//   - PermissionRequest: decision ("allow" | "deny") — can resolve before user dialog
//
// Of the 15 events that have event-specific output schemas (§6.1), these 5 are the
// subset that can directly affect authorization decisions in the permission pipeline.
//
// The remaining 12 permission-agnostic events with rich schemas cover session lifecycle,
// context injection, result transformation, and retry control (types/hooks.ts).
//
// This registry is a pure read-only catalog — no mutation methods.

// HookEventPhase identifies one of the 7 taxonomic groups from §6.1.
type HookEventPhase string

const (
	// HookEventPhaseToolAuthorization covers events that participate in tool
	// dispatch and the authorization pipeline (§5, §6.1).
	HookEventPhaseToolAuthorization HookEventPhase = "tool_authorization"

	// HookEventPhaseSessionLifecycle covers session-scoped lifecycle events
	// from start through stop (§6.1, §9).
	HookEventPhaseSessionLifecycle HookEventPhase = "session_lifecycle"

	// HookEventPhaseUserInteraction covers events that fire on user input
	// and elicitation flows (§6.1).
	HookEventPhaseUserInteraction HookEventPhase = "user_interaction"

	// HookEventPhaseSubagentCoordination covers events related to subagent
	// spawning, termination, and team coordination (§6.1, §8).
	HookEventPhaseSubagentCoordination HookEventPhase = "subagent_coordination"

	// HookEventPhaseContextManagement covers events fired by the compaction
	// pipeline and instruction-loading subsystem (§6.1, §7.3).
	HookEventPhaseContextManagement HookEventPhase = "context_management"

	// HookEventPhaseWorkspace covers filesystem and worktree change events
	// (§6.1).
	HookEventPhaseWorkspace HookEventPhase = "workspace"

	// HookEventPhaseNotifications covers the general notification event
	// that fires for external side-effects on user notifications (§6.1).
	HookEventPhaseNotifications HookEventPhase = "notifications"
)

// allHookEventPhases is the canonical ordering matching §6.1's listing order.
var allHookEventPhases = []HookEventPhase{
	HookEventPhaseToolAuthorization,
	HookEventPhaseSessionLifecycle,
	HookEventPhaseUserInteraction,
	HookEventPhaseSubagentCoordination,
	HookEventPhaseContextManagement,
	HookEventPhaseWorkspace,
	HookEventPhaseNotifications,
}

// HookEventProfile is the complete §6.1 + §5.3 descriptor for one hook event.
type HookEventProfile struct {
	// Event is the ExtendedHookEvent constant this profile describes.
	Event ExtendedHookEvent

	// Phase is the §6.1 taxonomic group this event belongs to.
	Phase HookEventPhase

	// PDFSection is the primary PDF section that defines this event.
	// "6.1" for the base taxonomy; "5.3" for permission-flow details.
	PDFSection string

	// HasEventSpecificSchema is true when types/hooks.ts defines a Zod schema
	// with fields beyond the base HookOutput type. §6.1: "15 have event-specific
	// output schemas with rich fields supporting permission decisions, context
	// injection, input modification, MCP result transformation, and retry control."
	HasEventSpecificSchema bool

	// ParticipatesInPermissionFlow is true for the 5 events §5.3 identifies as
	// participating directly in the permission pipeline with Zod-validated outputs.
	ParticipatesInPermissionFlow bool

	// PermissionFlowOutputFields lists the output schema fields specific to
	// permission-flow participation. Empty for non-permission-flow events.
	//
	// §5.3 defines these for the 5 participating hooks:
	//   PreToolUse:          permissionDecision, permissionDecisionReason, updatedInput
	//   PostToolUse:         additionalContext, updatedMCPToolOutput
	//   PostToolUseFailure:  additionalContext
	//   PermissionDenied:    (retry guidance — provider-defined retry fields)
	//   PermissionRequest:   decision
	PermissionFlowOutputFields []string

	// CanBlockExecution is true when the hook can prevent a tool call from
	// proceeding. §5.3: PreToolUse can return permissionDecision=deny to block;
	// PermissionRequest can return decision=deny to block.
	CanBlockExecution bool

	// CanModifyInput is true when the hook may rewrite the tool's input
	// parameters before execution. §5.3: only PreToolUse via updatedInput.
	CanModifyInput bool

	// CanModifyOutput is true when the hook may rewrite the tool's result
	// after execution. §5.3: PostToolUse via updatedMCPToolOutput (MCP only).
	CanModifyOutput bool

	// FiresForSubagents is true when this event also fires in subagent context.
	// §6.1: subagent-coordination events fire in subagent context by definition.
	// §8: subagents inherit hook registration from their spawning parent.
	FiresForSubagents bool
}

// SeedHookEventProfileCount is the exact number of hook event profiles seeded.
// §6.1: "The source code defines 27 hook events" (23 core + 4 web-adapted from EXT-002a).
const SeedHookEventProfileCount = 27

// SeedHookEventSlugs is the canonical ordered list matching §6.1's phase groupings.
var SeedHookEventSlugs = []ExtendedHookEvent{
	// Tool authorization (§5.3 + §6.1)
	HookPreToolUseExt, HookPostToolUseExt, HookPostToolUseFailureExt,
	HookPermissionRequestExt, HookPermissionDeniedExt,
	// Session lifecycle
	HookSessionStartExt, HookSessionEndExt, HookSetupExt,
	HookStopExt, HookStopFailureExt,
	// User interaction
	HookUserPromptSubmitExt, HookElicitationExt, HookElicitationResultExt,
	// Subagent coordination
	HookSubagentStartExt, HookSubagentStopExt,
	HookTeammateIdleExt, HookTaskCreatedExt, HookTaskCompletedExt,
	// Context management
	HookPreCompactExt, HookPostCompactExt,
	HookInstructionsLoadedExt, HookConfigChangeExt,
	// Workspace
	HookCwdChangedExt, HookFileChangedExt,
	HookWorktreeCreateExt, HookWorktreeRemoveExt,
	// Notifications
	HookNotificationExt,
}

// Additional ExtendedHookEvent constants for events present in §6.1 but not yet
// declared in hooktype.go (TeammateIdle, CwdChanged, WorktreeCreate, WorktreeRemove).
// These complete the 27-event set described by the paper.
const (
	// HookTeammateIdleExt fires when a teammate agent has no more pending tasks.
	// §6.1 subagent coordination group; stophooks.go calls this StopHookTeammateIdle.
	HookTeammateIdleExt ExtendedHookEvent = "TeammateIdle"

	// HookCwdChangedExt fires when the agent's working directory changes.
	// §6.1 workspace group.
	HookCwdChangedExt ExtendedHookEvent = "CwdChanged"

	// HookWorktreeCreateExt fires when a new git worktree is created.
	// §6.1 workspace group.
	HookWorktreeCreateExt ExtendedHookEvent = "WorktreeCreate"

	// HookWorktreeRemoveExt fires when a git worktree is removed.
	// §6.1 workspace group.
	HookWorktreeRemoveExt ExtendedHookEvent = "WorktreeRemove"
)

// seedHookEventProfiles is the immutable set of profiles, initialized once.
var seedHookEventProfiles = buildSeedHookEventProfiles()

func buildSeedHookEventProfiles() map[ExtendedHookEvent]HookEventProfile {
	m := make(map[ExtendedHookEvent]HookEventProfile, SeedHookEventProfileCount)

	// ---- Tool Authorization Phase (§5.3 + §6.1) ----

	m[HookPreToolUseExt] = HookEventProfile{
		Event:                        HookPreToolUseExt,
		Phase:                        HookEventPhaseToolAuthorization,
		PDFSection:                   "5.3",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: true,
		// §5.3: "Can return permissionDecision to deny or ask, or an updatedInput
		// that modifies the tool's input parameters (types/hooks.ts). A hook allow
		// does not bypass subsequent rule-based denies or safety checks."
		PermissionFlowOutputFields: []string{
			"permissionDecision",
			"permissionDecisionReason",
			"updatedInput",
		},
		CanBlockExecution: true,
		CanModifyInput:    true,
		CanModifyOutput:   false,
		FiresForSubagents: true,
	}

	m[HookPostToolUseExt] = HookEventProfile{
		Event:                        HookPostToolUseExt,
		Phase:                        HookEventPhaseToolAuthorization,
		PDFSection:                   "5.3",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: true,
		// §5.3: "Can inject additionalContext and, for MCP tools, return
		// updatedMCPToolOutput to modify results before they enter the context."
		PermissionFlowOutputFields: []string{
			"additionalContext",
			"updatedMCPToolOutput",
		},
		CanBlockExecution: false,
		CanModifyInput:    false,
		CanModifyOutput:   true, // MCP tools only — updatedMCPToolOutput
		FiresForSubagents: true,
	}

	m[HookPostToolUseFailureExt] = HookEventProfile{
		Event:                        HookPostToolUseFailureExt,
		Phase:                        HookEventPhaseToolAuthorization,
		PDFSection:                   "5.3",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: true,
		// §5.3: "Can inject additionalContext for error-specific guidance."
		PermissionFlowOutputFields: []string{"additionalContext"},
		CanBlockExecution:          false,
		CanModifyInput:             false,
		CanModifyOutput:            false,
		FiresForSubagents:          true,
	}

	m[HookPermissionRequestExt] = HookEventProfile{
		Event:                        HookPermissionRequestExt,
		Phase:                        HookEventPhaseToolAuthorization,
		PDFSection:                   "5.3",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: true,
		// §5.3: "Can return a decision of allow or deny. In coordinator and similar
		// paths, this can resolve before the user dialog. In the standard interactive
		// path, it can also run alongside the dialog."
		PermissionFlowOutputFields: []string{"decision"},
		CanBlockExecution:          true,
		CanModifyInput:             false,
		CanModifyOutput:            false,
		FiresForSubagents:          false, // coordinator/swarm paths only
	}

	m[HookPermissionDeniedExt] = HookEventProfile{
		Event:                        HookPermissionDeniedExt,
		Phase:                        HookEventPhaseToolAuthorization,
		PDFSection:                   "5.3",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: true,
		// §5.3: "Can provide retry guidance after auto-mode denials."
		// The retry guidance fields are provider-defined in the hook output schema.
		PermissionFlowOutputFields: []string{"retryGuidance"},
		CanBlockExecution:          false, // fires after deny is already decided
		CanModifyInput:             false,
		CanModifyOutput:            false,
		FiresForSubagents:          true,
	}

	// ---- Session Lifecycle Phase (§6.1) ----

	m[HookSessionStartExt] = HookEventProfile{
		Event:                        HookSessionStartExt,
		Phase:                        HookEventPhaseSessionLifecycle,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookSessionEndExt] = HookEventProfile{
		Event:                        HookSessionEndExt,
		Phase:                        HookEventPhaseSessionLifecycle,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookSetupExt] = HookEventProfile{
		Event:                        HookSetupExt,
		Phase:                        HookEventPhaseSessionLifecycle,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookStopExt] = HookEventProfile{
		Event:                        HookStopExt,
		Phase:                        HookEventPhaseSessionLifecycle,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		// §4.5: Stop hook can set hook_stopped_continuation to terminate the loop.
		CanBlockExecution: true,
		FiresForSubagents: false,
	}

	m[HookStopFailureExt] = HookEventProfile{
		Event:                        HookStopFailureExt,
		Phase:                        HookEventPhaseSessionLifecycle,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	// ---- User Interaction Phase (§6.1) ----

	m[HookUserPromptSubmitExt] = HookEventProfile{
		Event:                        HookUserPromptSubmitExt,
		Phase:                        HookEventPhaseUserInteraction,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		// UserPromptSubmit can inject context or block the turn.
		CanBlockExecution: true,
		FiresForSubagents: false,
	}

	m[HookElicitationExt] = HookEventProfile{
		Event:                        HookElicitationExt,
		Phase:                        HookEventPhaseUserInteraction,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookElicitationResultExt] = HookEventProfile{
		Event:                        HookElicitationResultExt,
		Phase:                        HookEventPhaseUserInteraction,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	// ---- Subagent Coordination Phase (§6.1, §8) ----

	m[HookSubagentStartExt] = HookEventProfile{
		Event:                        HookSubagentStartExt,
		Phase:                        HookEventPhaseSubagentCoordination,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            true,
	}

	m[HookSubagentStopExt] = HookEventProfile{
		Event:                        HookSubagentStopExt,
		Phase:                        HookEventPhaseSubagentCoordination,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		// §6.1: "SubagentStop hook: Same [as Stop], for sub-agents spawned via AgentTool."
		CanBlockExecution: true,
		FiresForSubagents: true,
	}

	m[HookTeammateIdleExt] = HookEventProfile{
		Event:                        HookTeammateIdleExt,
		Phase:                        HookEventPhaseSubagentCoordination,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		// §6.1 subagent coordination group; stophooks.go StopHookTeammateIdle.
		FiresForSubagents: true,
	}

	m[HookTaskCreatedExt] = HookEventProfile{
		Event:                        HookTaskCreatedExt,
		Phase:                        HookEventPhaseSubagentCoordination,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            true,
	}

	m[HookTaskCompletedExt] = HookEventProfile{
		Event:                        HookTaskCompletedExt,
		Phase:                        HookEventPhaseSubagentCoordination,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            true,
	}

	// ---- Context Management Phase (§6.1, §7.3) ----

	m[HookPreCompactExt] = HookEventProfile{
		Event:                        HookPreCompactExt,
		Phase:                        HookEventPhaseContextManagement,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookPostCompactExt] = HookEventProfile{
		Event:                        HookPostCompactExt,
		Phase:                        HookEventPhaseContextManagement,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       true,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookInstructionsLoadedExt] = HookEventProfile{
		Event:                        HookInstructionsLoadedExt,
		Phase:                        HookEventPhaseContextManagement,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookConfigChangeExt] = HookEventProfile{
		Event:                        HookConfigChangeExt,
		Phase:                        HookEventPhaseContextManagement,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	// ---- Workspace Phase (§6.1) ----

	m[HookCwdChangedExt] = HookEventProfile{
		Event:                        HookCwdChangedExt,
		Phase:                        HookEventPhaseWorkspace,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookFileChangedExt] = HookEventProfile{
		Event:                        HookFileChangedExt,
		Phase:                        HookEventPhaseWorkspace,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookWorktreeCreateExt] = HookEventProfile{
		Event:                        HookWorktreeCreateExt,
		Phase:                        HookEventPhaseWorkspace,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	m[HookWorktreeRemoveExt] = HookEventProfile{
		Event:                        HookWorktreeRemoveExt,
		Phase:                        HookEventPhaseWorkspace,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		FiresForSubagents:            false,
	}

	// ---- Notifications Phase (§6.1) ----

	m[HookNotificationExt] = HookEventProfile{
		Event:                        HookNotificationExt,
		Phase:                        HookEventPhaseNotifications,
		PDFSection:                   "6.1",
		HasEventSpecificSchema:       false,
		ParticipatesInPermissionFlow: false,
		// §6.1: "Notification hook: External side effects on user notifications."
		FiresForSubagents: false,
	}

	return m
}

// HookEventPhaseRegistry is the read-only §6.1 + §5.3 catalog.
// Constructed once; all queries are O(1) or O(n) over the sealed set.
type HookEventPhaseRegistry struct {
	profiles map[ExtendedHookEvent]HookEventProfile
}

// NewHookEventPhaseRegistry returns a ready-to-use registry seeded with all
// 27 hook events from §6.1. The registry is immutable after construction.
func NewHookEventPhaseRegistry() *HookEventPhaseRegistry {
	return &HookEventPhaseRegistry{profiles: seedHookEventProfiles}
}

// FindByEvent returns the profile for the given event, or (zero, false) if unknown.
func (r *HookEventPhaseRegistry) FindByEvent(event ExtendedHookEvent) (HookEventProfile, bool) {
	p, ok := r.profiles[event]
	return p, ok
}

// AllProfiles returns all 27 profiles ordered by SeedHookEventSlugs (phase order).
func (r *HookEventPhaseRegistry) AllProfiles() []HookEventProfile {
	out := make([]HookEventProfile, 0, len(r.profiles))
	for _, event := range SeedHookEventSlugs {
		if p, ok := r.profiles[event]; ok {
			out = append(out, p)
		}
	}
	return out
}

// EventsByPhase returns all events in the given phase, ordered by SeedHookEventSlugs.
func (r *HookEventPhaseRegistry) EventsByPhase(phase HookEventPhase) []HookEventProfile {
	var out []HookEventProfile
	for _, event := range SeedHookEventSlugs {
		p, ok := r.profiles[event]
		if ok && p.Phase == phase {
			out = append(out, p)
		}
	}
	return out
}

// PermissionFlowEvents returns the 5 events from §5.3 that participate
// directly in the authorization pipeline, in §5.3 listing order.
func (r *HookEventPhaseRegistry) PermissionFlowEvents() []HookEventProfile {
	// §5.3 explicit order: PreToolUse, PostToolUse, PostToolUseFailure,
	// PermissionDenied, PermissionRequest.
	order := []ExtendedHookEvent{
		HookPreToolUseExt, HookPostToolUseExt, HookPostToolUseFailureExt,
		HookPermissionDeniedExt, HookPermissionRequestExt,
	}
	out := make([]HookEventProfile, 0, 5)
	for _, e := range order {
		if p, ok := r.profiles[e]; ok && p.ParticipatesInPermissionFlow {
			out = append(out, p)
		}
	}
	return out
}

// BlockingEvents returns events where CanBlockExecution is true.
// §4.5 lists 5 stop conditions; hooks account for "Hook intervention" condition.
func (r *HookEventPhaseRegistry) BlockingEvents() []HookEventProfile {
	var out []HookEventProfile
	for _, event := range SeedHookEventSlugs {
		p, ok := r.profiles[event]
		if ok && p.CanBlockExecution {
			out = append(out, p)
		}
	}
	return out
}

// EventsWithSpecificSchema returns events that have event-specific Zod
// output schemas beyond the base HookOutput. §6.1: "15 have event-specific
// output schemas."
func (r *HookEventPhaseRegistry) EventsWithSpecificSchema() []HookEventProfile {
	var out []HookEventProfile
	for _, event := range SeedHookEventSlugs {
		p, ok := r.profiles[event]
		if ok && p.HasEventSpecificSchema {
			out = append(out, p)
		}
	}
	return out
}

// SubagentEvents returns events that fire in subagent context.
func (r *HookEventPhaseRegistry) SubagentEvents() []HookEventProfile {
	var out []HookEventProfile
	for _, event := range SeedHookEventSlugs {
		p, ok := r.profiles[event]
		if ok && p.FiresForSubagents {
			out = append(out, p)
		}
	}
	return out
}

// AllPhases returns the 7 phases in canonical §6.1 order.
func (r *HookEventPhaseRegistry) AllPhases() []HookEventPhase {
	out := make([]HookEventPhase, len(allHookEventPhases))
	copy(out, allHookEventPhases)
	return out
}

// IsKnownEvent returns true when event is in the seeded 27-event set.
func (r *HookEventPhaseRegistry) IsKnownEvent(event ExtendedHookEvent) bool {
	_, ok := r.profiles[event]
	return ok
}

// Count returns the total number of seeded profiles. Must equal SeedHookEventProfileCount.
func (r *HookEventPhaseRegistry) Count() int {
	return len(r.profiles)
}
