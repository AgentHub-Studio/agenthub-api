package agentic

import "fmt"

// system_safety_comparison_registry.go — FEAT-047 — §13.2 Safety and Permissions Comparison
//
// arXiv:2604.14228v1, §13.2 "Agent Architecture Patterns" (page 36).
//
// The §13.2 "Safety and permissions" paragraph explicitly states that production
// coding agents adopt safety architectures that vary along three axes:
//
//  1. approval_model — per-action prompting, classifier-mediated automation, or
//     no prompting with post-hoc review.
//
//  2. isolation_boundary — OS-level container, filesystem sandbox,
//     permission-scoped tool pool, or none.
//
//  3. recovery_mechanism — version-control rollback, session-scoped permission
//     reset, or checkpoint-based rewind.
//
// The paragraph surveys four named systems:
//
//   - SWE-Agent / OpenHands (§13.2, also §2.2 and §5.1): rely primarily on Docker
//     container isolation, providing environment-level sandboxing that constrains
//     all agent actions.
//
//   - Aider (§13.2, also §2.2 and §5.1): uses Git as its primary safety mechanism,
//     making all changes reversible through version control.
//
//   - Codex CLI (§13.2): supports sandbox modes and approval policies for shell
//     commands.
//
//   - Claude Code (§13.2, §5.1, §2.2): combines per-action deny-first rules, an
//     ML-based classifier for automated approval, optional shell sandboxing, and
//     session-scoped permission non-restoration, layering multiple mechanisms
//     rather than relying on a single isolation boundary.
//
// Table 5 (page 35) classifies Devin and SWE-Agent as "Fully autonomous" (sandbox
// + planning) and Claude Code / Aider / Codex CLI as "Agentic CLI" (tool-use loop).
// This registry models the four systems named in the §13.2 safety paragraph.
//
// The registry is pure Go, no DB, no HTTP. It is intended for introspection,
// documentation generation, and test-driven validation of §13.2 claims.

// AgentSystemID is a stable slug for one of the four §13.2 agentic systems
// compared in the safety-and-permissions analysis.
//
// Note: the prefix "SafetySystem" is used to avoid collision with SystemSlug
// constants defined in comparative_dimension.go (which use "System" prefix for
// a different type).
type AgentSystemID string

const (
	// SafetySystemClaudeCode — Claude Code (Anthropic, agentic CLI).
	// §13.2: "Claude Code combines per-action deny-first rules, an ML-based
	// classifier for automated approval, optional shell sandboxing, and
	// session-scoped permission non-restoration, layering multiple mechanisms
	// rather than relying on a single isolation boundary."
	SafetySystemClaudeCode AgentSystemID = "claude_code"

	// SafetySystemSWEAgent — SWE-Agent (Yang et al., 2024; fully autonomous).
	// §13.2: "SWE-Agent and OpenHands rely primarily on Docker container
	// isolation, providing environment-level sandboxing that constrains all
	// agent actions."
	SafetySystemSWEAgent AgentSystemID = "swe_agent"

	// SafetySystemOpenHands — OpenHands (Wang et al., 2024b; fully autonomous).
	// §13.2: shares Docker-isolation safety model with SWE-Agent.
	SafetySystemOpenHands AgentSystemID = "open_hands"

	// SafetySystemAider — Aider (Gauthier, 2024; agentic CLI).
	// §13.2: "Aider uses Git as its primary safety mechanism, making all
	// changes reversible through version control."
	SafetySystemAider AgentSystemID = "aider"
)

// ApprovalModel describes how a system decides whether an action may proceed.
type ApprovalModel string

const (
	// ApprovalPerActionDenyFirst — every action is evaluated by deny-first rules
	// before execution; unrecognised actions are escalated to the user. Claude Code
	// additionally layers an ML auto-mode classifier and PreToolUse hooks on top of
	// rule evaluation. (§5.1, §13.2)
	ApprovalPerActionDenyFirst ApprovalModel = "per_action_deny_first"

	// ApprovalContainerBoundary — the container itself enforces the boundary;
	// no per-action prompting is applied inside the sandbox. (§13.2, §2.2)
	ApprovalContainerBoundary ApprovalModel = "container_boundary"

	// ApprovalShellApprovalPolicy — sandbox modes and approval policies for shell
	// commands; mid-point between per-action and container-only. (§13.2)
	ApprovalShellApprovalPolicy ApprovalModel = "shell_approval_policy"

	// ApprovalVersionControlAsNet — no per-action prompting; Git rollback is the
	// primary safety guarantee, not pre-action evaluation. (§13.2, §2.2)
	ApprovalVersionControlAsNet ApprovalModel = "version_control_as_net"
)

// IsolationBoundary describes the trust boundary that constrains agent actions.
type IsolationBoundary string

const (
	// IsolationLayeredPolicyEnforcement — multiple independent layers: deny-first
	// permission rules, ML classifier, optional shell sandbox, PreToolUse hooks.
	// No single boundary; any layer can block an action. (§5, §13.2)
	IsolationLayeredPolicyEnforcement IsolationBoundary = "layered_policy_enforcement"

	// IsolationDockerContainer — OS-level Docker container; the entire execution
	// environment is sandboxed at the container level. (§2.2, §13.2)
	IsolationDockerContainer IsolationBoundary = "docker_container"

	// IsolationSandboxModes — filesystem and network isolation via sandbox modes;
	// environment-level but configurable per invocation. (§13.2)
	IsolationSandboxModes IsolationBoundary = "sandbox_modes"

	// IsolationNone — no dedicated runtime isolation boundary; safety comes
	// entirely from version-control recoverability. (§13.2, §2.2)
	IsolationNone IsolationBoundary = "none"
)

// RecoveryMechanism describes the primary recovery strategy if an action causes harm.
type RecoveryMechanism string

const (
	// RecoverySessionScopedPermissionReset — session-scoped permissions are not
	// restored on resume; this is a deliberate safety choice (not a limitation).
	// The combination of deny-first rules and non-restoration means the permission
	// surface does not silently expand across sessions. (§9, §13.2)
	RecoverySessionScopedPermissionReset RecoveryMechanism = "session_scoped_permission_reset"

	// RecoveryContainerDispose — discard and recreate the container; stateless
	// recovery at the OS level. (§13.2)
	RecoveryContainerDispose RecoveryMechanism = "container_dispose"

	// RecoveryCheckpointRewind — rewind to a prior checkpoint; supported by
	// some sandbox-planning systems. (§13.2)
	RecoveryCheckpointRewind RecoveryMechanism = "checkpoint_rewind"

	// RecoveryGitRollback — Git revert / reset as the primary undo mechanism;
	// all changes are reversible through version control. (§2.2, §13.2)
	RecoveryGitRollback RecoveryMechanism = "git_rollback"
)

// PermissionGranularity classifies how finely the system scopes permissions.
type PermissionGranularity string

const (
	// PermGranularityPerAction — each tool invocation is individually evaluated.
	// Claude Code evaluates at the tool-name level and the content level
	// (e.g. Bash(prefix:npm)). (§5.1)
	PermGranularityPerAction PermissionGranularity = "per_action"

	// PermGranularityEnvironmentLevel — the container defines the boundary; actions
	// inside the container are not individually scoped. (§13.2)
	PermGranularityEnvironmentLevel PermissionGranularity = "environment_level"

	// PermGranularityCommandLevel — approval is at the shell-command level, not at
	// the individual-argument level. (§13.2)
	PermGranularityCommandLevel PermissionGranularity = "command_level"

	// PermGranularityFileSystem — permissions are scoped to filesystem changes;
	// all changes are reversible via Git. (§13.2)
	PermGranularityFileSystem PermissionGranularity = "file_system"
)

// SystemSafetyProfile is the structured safety profile for one §13.2 agentic system.
type SystemSafetyProfile struct {
	// SystemID is the stable slug for this system.
	SystemID AgentSystemID

	// SystemName is the human-readable display name.
	SystemName string

	// PDFSection is the primary §-reference where this system's safety properties
	// are described.
	PDFSection string

	// ApprovalModel describes how actions are approved before execution.
	ApprovalModel ApprovalModel

	// IsolationBoundary describes the primary trust boundary.
	IsolationBoundary IsolationBoundary

	// RecoveryMechanism describes how the system recovers from harmful actions.
	RecoveryMechanism RecoveryMechanism

	// PermissionGranularity classifies the scope at which permissions are enforced.
	PermissionGranularity PermissionGranularity

	// IsHumanInLoop reports whether the system uses interactive human approval as
	// part of its primary safety flow (not just as a fallback).
	IsHumanInLoop bool

	// DefaultsToSafe reports whether the system's out-of-box default posture is
	// deny-first (true) rather than permit-first or no-evaluation (false).
	DefaultsToSafe bool

	// SupportsGitRollback reports whether Git-based rollback is part of the
	// system's documented safety or recovery story.
	SupportsGitRollback bool

	// UsesMLClassifier reports whether an ML-based classifier participates in
	// the permission or approval decision.
	UsesMLClassifier bool

	// UsesContainerIsolation reports whether a container (Docker or similar)
	// is part of the system's isolation strategy.
	UsesContainerIsolation bool

	// PDFEvidence quotes the verbatim §13.2 sentence describing this system.
	PDFEvidence string
}

// systemSafetyProfiles is the canonical dataset populated from §13.2 + §5.1 + §2.2.
var systemSafetyProfiles = map[AgentSystemID]SystemSafetyProfile{
	SafetySystemClaudeCode: {
		SystemID:   SafetySystemClaudeCode,
		SystemName: "Claude Code",
		PDFSection: "13.2",
		ApprovalModel: ApprovalPerActionDenyFirst,
		IsolationBoundary: IsolationLayeredPolicyEnforcement,
		RecoveryMechanism: RecoverySessionScopedPermissionReset,
		PermissionGranularity: PermGranularityPerAction,
		IsHumanInLoop:         true,
		DefaultsToSafe:        true,
		SupportsGitRollback:   false,
		UsesMLClassifier:      true,
		UsesContainerIsolation: false,
		PDFEvidence: "Claude Code combines per-action deny-first rules, an ML-based classifier " +
			"for automated approval, optional shell sandboxing, and session-scoped permission " +
			"non-restoration, layering multiple mechanisms rather than relying on a single " +
			"isolation boundary. (§13.2)",
	},
	SafetySystemSWEAgent: {
		SystemID:   SafetySystemSWEAgent,
		SystemName: "SWE-Agent",
		PDFSection: "13.2",
		ApprovalModel: ApprovalContainerBoundary,
		IsolationBoundary: IsolationDockerContainer,
		RecoveryMechanism: RecoveryContainerDispose,
		PermissionGranularity: PermGranularityEnvironmentLevel,
		IsHumanInLoop:         false,
		DefaultsToSafe:        false,
		SupportsGitRollback:   false,
		UsesMLClassifier:      false,
		UsesContainerIsolation: true,
		PDFEvidence: "SWE-Agent and OpenHands rely primarily on Docker container isolation, " +
			"providing environment-level sandboxing that constrains all agent actions. (§13.2)",
	},
	SafetySystemOpenHands: {
		SystemID:   SafetySystemOpenHands,
		SystemName: "OpenHands",
		PDFSection: "13.2",
		ApprovalModel: ApprovalContainerBoundary,
		IsolationBoundary: IsolationDockerContainer,
		RecoveryMechanism: RecoveryContainerDispose,
		PermissionGranularity: PermGranularityEnvironmentLevel,
		IsHumanInLoop:         false,
		DefaultsToSafe:        false,
		SupportsGitRollback:   false,
		UsesMLClassifier:      false,
		UsesContainerIsolation: true,
		PDFEvidence: "SWE-Agent and OpenHands rely primarily on Docker container isolation, " +
			"providing environment-level sandboxing that constrains all agent actions. (§13.2)",
	},
	SafetySystemAider: {
		SystemID:   SafetySystemAider,
		SystemName: "Aider",
		PDFSection: "13.2",
		ApprovalModel: ApprovalVersionControlAsNet,
		IsolationBoundary: IsolationNone,
		RecoveryMechanism: RecoveryGitRollback,
		PermissionGranularity: PermGranularityFileSystem,
		IsHumanInLoop:         false,
		DefaultsToSafe:        false,
		SupportsGitRollback:   true,
		UsesMLClassifier:      false,
		UsesContainerIsolation: false,
		PDFEvidence: "Aider uses Git as its primary safety mechanism, making all changes " +
			"reversible through version control. (§13.2)",
	},
}

// systemSafetySequence is the canonical §13.2 order: Claude Code first (primary
// subject), then comparators in Table-5 category order (SWE-Agent, OpenHands,
// Aider).
var systemSafetySequence = []AgentSystemID{
	SafetySystemClaudeCode,
	SafetySystemSWEAgent,
	SafetySystemOpenHands,
	SafetySystemAider,
}

// SystemSafetyComparisonRegistry provides structured access to the four §13.2
// agentic system safety profiles and cross-cutting comparison methods.
type SystemSafetyComparisonRegistry struct{}

// NewSystemSafetyComparisonRegistry returns a ready-to-use registry.
func NewSystemSafetyComparisonRegistry() *SystemSafetyComparisonRegistry {
	return &SystemSafetyComparisonRegistry{}
}

// FindByID returns the safety profile for the given system ID.
// Returns (zero, false) if the ID is not registered.
func (r *SystemSafetyComparisonRegistry) FindByID(id AgentSystemID) (SystemSafetyProfile, bool) {
	p, ok := systemSafetyProfiles[id]
	return p, ok
}

// AllSystems returns all four §13.2 system profiles in canonical order.
func (r *SystemSafetyComparisonRegistry) AllSystems() []SystemSafetyProfile {
	out := make([]SystemSafetyProfile, len(systemSafetySequence))
	for i, id := range systemSafetySequence {
		out[i] = systemSafetyProfiles[id]
	}
	return out
}

// Count returns the number of registered systems (always 4).
func (r *SystemSafetyComparisonRegistry) Count() int {
	return len(systemSafetySequence)
}

// IsValidSystemID returns true if the ID is one of the four §13.2 systems.
func (r *SystemSafetyComparisonRegistry) IsValidSystemID(id AgentSystemID) bool {
	_, ok := systemSafetyProfiles[id]
	return ok
}

// SystemsWithHumanInLoop returns all profiles where IsHumanInLoop is true.
// Per §5.1: only Claude Code uses interactive human approval in its primary
// permission flow; the others do not.
func (r *SystemSafetyComparisonRegistry) SystemsWithHumanInLoop() []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.IsHumanInLoop {
			out = append(out, p)
		}
	}
	return out
}

// SystemsDefaultingSafe returns all profiles where DefaultsToSafe is true.
// Per §5.1: Claude Code is the only §13.2 system whose out-of-box posture is
// deny-first evaluation.
func (r *SystemSafetyComparisonRegistry) SystemsDefaultingSafe() []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.DefaultsToSafe {
			out = append(out, p)
		}
	}
	return out
}

// SystemsWithGitRollback returns all profiles where SupportsGitRollback is true.
// Per §13.2 + §2.2: Aider is the only §13.2 system that uses Git rollback as its
// primary safety mechanism.
func (r *SystemSafetyComparisonRegistry) SystemsWithGitRollback() []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.SupportsGitRollback {
			out = append(out, p)
		}
	}
	return out
}

// SystemsWithContainerIsolation returns all profiles where UsesContainerIsolation
// is true. Per §13.2: SWE-Agent and OpenHands rely on Docker container isolation.
func (r *SystemSafetyComparisonRegistry) SystemsWithContainerIsolation() []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.UsesContainerIsolation {
			out = append(out, p)
		}
	}
	return out
}

// SystemsWithMLClassifier returns all profiles where UsesMLClassifier is true.
// Per §5.3: only Claude Code employs an ML-based classifier (yoloClassifier.ts)
// among the four §13.2 systems.
func (r *SystemSafetyComparisonRegistry) SystemsWithMLClassifier() []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.UsesMLClassifier {
			out = append(out, p)
		}
	}
	return out
}

// SafestSystem returns the profile with the most conservative default safety
// posture. Per §13.2 + §5.1, that is Claude Code: it is the only system that
// (a) defaults to deny-first evaluation, (b) involves the human in the primary
// permission flow, and (c) layers multiple independent mechanisms.
func (r *SystemSafetyComparisonRegistry) SafestSystem() SystemSafetyProfile {
	return systemSafetyProfiles[SafetySystemClaudeCode]
}

// MostAutonomousSystem returns the profile that minimises human-in-the-loop
// involvement while still producing software changes. Per §13.2 and Table 5,
// SWE-Agent and OpenHands are classified as "Fully autonomous" (sandbox +
// planning). Among the two, this method returns SWE-Agent as the canonical
// exemplar cited first in §13.2.
func (r *SystemSafetyComparisonRegistry) MostAutonomousSystem() SystemSafetyProfile {
	return systemSafetyProfiles[SafetySystemSWEAgent]
}

// CompareApprovalModels returns a slice of (SystemName, ApprovalModel) strings
// for all four systems in canonical order — suitable for tabular display or
// documentation generation.
func (r *SystemSafetyComparisonRegistry) CompareApprovalModels() []string {
	out := make([]string, len(systemSafetySequence))
	for i, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		out[i] = fmt.Sprintf("%s: %s", p.SystemName, string(p.ApprovalModel))
	}
	return out
}

// CompareIsolationBoundaries returns a slice of (SystemName, IsolationBoundary)
// strings for all four systems in canonical order.
func (r *SystemSafetyComparisonRegistry) CompareIsolationBoundaries() []string {
	out := make([]string, len(systemSafetySequence))
	for i, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		out[i] = fmt.Sprintf("%s: %s", p.SystemName, string(p.IsolationBoundary))
	}
	return out
}

// CompareRecoveryMechanisms returns a slice of (SystemName, RecoveryMechanism)
// strings for all four systems in canonical order.
func (r *SystemSafetyComparisonRegistry) CompareRecoveryMechanisms() []string {
	out := make([]string, len(systemSafetySequence))
	for i, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		out[i] = fmt.Sprintf("%s: %s", p.SystemName, string(p.RecoveryMechanism))
	}
	return out
}

// SystemsByIsolationBoundary returns all systems whose IsolationBoundary matches
// the given value, in canonical order.
func (r *SystemSafetyComparisonRegistry) SystemsByIsolationBoundary(b IsolationBoundary) []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.IsolationBoundary == b {
			out = append(out, p)
		}
	}
	return out
}

// SystemsByApprovalModel returns all systems whose ApprovalModel matches the given
// value, in canonical order.
func (r *SystemSafetyComparisonRegistry) SystemsByApprovalModel(m ApprovalModel) []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.ApprovalModel == m {
			out = append(out, p)
		}
	}
	return out
}

// SystemsByRecoveryMechanism returns all systems whose RecoveryMechanism matches
// the given value, in canonical order.
func (r *SystemSafetyComparisonRegistry) SystemsByRecoveryMechanism(m RecoveryMechanism) []SystemSafetyProfile {
	var out []SystemSafetyProfile
	for _, id := range systemSafetySequence {
		p := systemSafetyProfiles[id]
		if p.RecoveryMechanism == m {
			out = append(out, p)
		}
	}
	return out
}

// --- Structural invariants ---

// SafetyInvariantResult reports whether a §13.2 structural invariant holds.
type SafetyInvariantResult struct {
	Name    string
	Holds   bool
	Message string
}

// StructuralInvariants verifies the four invariants that §13.2 implies:
//
//  1. ExactlyFourSystems — the registry contains exactly four §13.2 systems.
//
//  2. ClaudeCodeIsHumanInLoop — Claude Code is the only system where IsHumanInLoop
//     is true; §5.1 documents that the 93% auto-approval rate motivated sandboxing
//     and ML classification rather than more per-action prompts, but interactive
//     human escalation remains part of the primary permission flow.
//
//  3. GitRollbackSystemsAreSafe — every system where SupportsGitRollback is true
//     must have RecoveryMechanism == RecoveryGitRollback; the paper treats git
//     rollback as the safety mechanism, not merely an ancillary feature.
//
//  4. ContainerSystemsNotDenyFirst — systems relying on Docker container isolation
//     (SWE-Agent, OpenHands) do NOT use per-action deny-first approval; §13.2
//     presents these as distinct safety strategies.
func (r *SystemSafetyComparisonRegistry) StructuralInvariants() []SafetyInvariantResult {
	results := []SafetyInvariantResult{}

	// Invariant 1: exactly four systems.
	n := r.Count()
	results = append(results, SafetyInvariantResult{
		Name:    "ExactlyFourSystems",
		Holds:   n == 4,
		Message: fmt.Sprintf("expected 4 §13.2 systems, got %d", n),
	})

	// Invariant 2: Claude Code is human-in-loop; all others are not.
	humanLoop := r.SystemsWithHumanInLoop()
	claudeOK := len(humanLoop) == 1 && humanLoop[0].SystemID == SafetySystemClaudeCode
	results = append(results, SafetyInvariantResult{
		Name:    "ClaudeCodeIsHumanInLoop",
		Holds:   claudeOK,
		Message: fmt.Sprintf("expected exactly ClaudeCode as human-in-loop system, got %d systems", len(humanLoop)),
	})

	// Invariant 3: git-rollback systems have RecoveryGitRollback.
	gitRollbackOK := true
	for _, p := range r.SystemsWithGitRollback() {
		if p.RecoveryMechanism != RecoveryGitRollback {
			gitRollbackOK = false
			break
		}
	}
	results = append(results, SafetyInvariantResult{
		Name:    "GitRollbackSystemsAreSafe",
		Holds:   gitRollbackOK,
		Message: "all systems with SupportsGitRollback=true must have RecoveryMechanism=git_rollback",
	})

	// Invariant 4: container-isolation systems do not use per-action deny-first.
	containerNotDenyFirst := true
	for _, p := range r.SystemsWithContainerIsolation() {
		if p.ApprovalModel == ApprovalPerActionDenyFirst {
			containerNotDenyFirst = false
			break
		}
	}
	results = append(results, SafetyInvariantResult{
		Name:    "ContainerSystemsNotDenyFirst",
		Holds:   containerNotDenyFirst,
		Message: "container-isolation systems must not use per-action deny-first approval",
	})

	return results
}
