package agentic

// PermissionHandlerPath identifies one of the four execution paths that the
// useCanUseTool handler branches into at runtime, as described in §5.2 of the
// Claude Code architecture paper (arXiv:2604.14228v1).
//
// §5.2: "The handler in useCanUseTool.tsx branches into one of four paths
// based on runtime context."
//
// The four paths represent a progressive automation spectrum: Coordinator and
// SwarmWorker attempt fully automated resolution; SpeculativeClassifier races
// a pre-started classifier against a timeout for near-instant approval;
// Interactive falls back to the standard user approval dialog.
//
// When a denial occurs on any path, the system treats it as a routing signal
// rather than a hard stop: the model receives the denial reason, revises its
// approach, and attempts a safer alternative in the next loop iteration. This
// recovery-oriented design means permission enforcement shapes agent behavior
// rather than simply halting it (§5.2 final paragraph).
type PermissionHandlerPath string

const (
	// PermissionHandlerPathCoordinator is used in multi-agent coordination mode.
	// §5.2: "Coordinator: For multi-agent coordination mode. Attempts automated
	// resolution (classifier, hooks, rules) before falling back to user interaction."
	// Automated checks run first; user interaction is only a fallback.
	PermissionHandlerPathCoordinator PermissionHandlerPath = "coordinator"

	// PermissionHandlerPathSwarmWorker handles worker agents in a multi-agent swarm.
	// §5.2: "Swarm worker: Handles worker agents in a multi-agent swarm with their
	// own resolution logic."
	// Workers apply swarm-specific resolution without exposing a user dialog.
	PermissionHandlerPathSwarmWorker PermissionHandlerPath = "swarm_worker"

	// PermissionHandlerPathSpeculativeClassifier races a pre-started BASH_CLASSIFIER
	// against a timeout for instant approval of BashTool calls.
	// §5.2: "Speculative classifier: When BASH_CLASSIFIER is enabled and the tool is
	// BashTool, a speculative classifier races a pre-started classification result
	// against a timeout. If the classifier returns with high confidence, the tool is
	// approved instantly without user interaction."
	PermissionHandlerPathSpeculativeClassifier PermissionHandlerPath = "speculative_classifier"

	// PermissionHandlerPathInteractive is the standard fallback path.
	// §5.2: "Interactive: The fallback path. Presents the standard user approval
	// dialog through the terminal UI."
	// In the interactive path the user dialog is queued first and hooks run
	// asynchronously alongside it.
	PermissionHandlerPathInteractive PermissionHandlerPath = "interactive"
)

// permissionHandlerPaths is the canonical order: coordinator → swarm_worker →
// speculative_classifier → interactive. Listed from most-automated to least-automated,
// mirroring the §5.2 numbered list in the paper.
var permissionHandlerPaths = []PermissionHandlerPath{
	PermissionHandlerPathCoordinator,
	PermissionHandlerPathSwarmWorker,
	PermissionHandlerPathSpeculativeClassifier,
	PermissionHandlerPathInteractive,
}

// PermissionHandlerPathProfile holds the immutable characteristics of one
// authorization handler path.
type PermissionHandlerPathProfile struct {
	Path PermissionHandlerPath

	// AutomationLevel is the 1-based ranking of automated-ness (1 = most automated).
	// Coordinator=1, SwarmWorker=2, SpeculativeClassifier=3, Interactive=4.
	AutomationLevel int

	// RequiresUserDialog indicates that the path presents the standard user approval
	// dialog when no automated check resolves the request.
	RequiresUserDialog bool

	// SupportsAutomatedResolution indicates the path attempts classifier/hook/rule
	// evaluation before any user interaction.
	SupportsAutomatedResolution bool

	// IsBackgroundAgentPath indicates the path is used by background or coordinator
	// agents that await automated checks before showing any dialog (§5.2 paragraph 4).
	IsBackgroundAgentPath bool

	// FeatureFlag is the environment flag that enables the path, if any.
	// Empty string means the path is always available.
	FeatureFlag string

	// Description is a short human-readable summary of the path.
	Description string
}

var permissionHandlerPathProfiles = map[PermissionHandlerPath]PermissionHandlerPathProfile{
	PermissionHandlerPathCoordinator: {
		Path:                        PermissionHandlerPathCoordinator,
		AutomationLevel:             1,
		RequiresUserDialog:          false,
		SupportsAutomatedResolution: true,
		IsBackgroundAgentPath:       true,
		FeatureFlag:                 "",
		Description:                 "Multi-agent coordination mode: attempts classifier+hooks+rules before falling back to user interaction.",
	},
	PermissionHandlerPathSwarmWorker: {
		Path:                        PermissionHandlerPathSwarmWorker,
		AutomationLevel:             2,
		RequiresUserDialog:          false,
		SupportsAutomatedResolution: true,
		IsBackgroundAgentPath:       true,
		FeatureFlag:                 "",
		Description:                 "Swarm worker agent: applies swarm-specific resolution logic without a user dialog.",
	},
	PermissionHandlerPathSpeculativeClassifier: {
		Path:                        PermissionHandlerPathSpeculativeClassifier,
		AutomationLevel:             3,
		RequiresUserDialog:          false,
		SupportsAutomatedResolution: true,
		IsBackgroundAgentPath:       false,
		FeatureFlag:                 "BASH_CLASSIFIER",
		Description:                 "Races a pre-started BashTool classifier against a timeout; approves instantly at high confidence.",
	},
	PermissionHandlerPathInteractive: {
		Path:                        PermissionHandlerPathInteractive,
		AutomationLevel:             4,
		RequiresUserDialog:          true,
		SupportsAutomatedResolution: false,
		IsBackgroundAgentPath:       false,
		FeatureFlag:                 "",
		Description:                 "Fallback path: presents the standard user approval dialog through the terminal UI.",
	},
}

// PermissionHandlerPathRegistry provides queries over the §5.2 four authorization
// handler paths.
type PermissionHandlerPathRegistry struct{}

// NewPermissionHandlerPathRegistry returns a ready-to-use registry.
func NewPermissionHandlerPathRegistry() *PermissionHandlerPathRegistry {
	return &PermissionHandlerPathRegistry{}
}

// Profile returns the immutable profile for the given handler path.
// Returns (zero-value, false) if the path is unknown.
func (r *PermissionHandlerPathRegistry) Profile(p PermissionHandlerPath) (PermissionHandlerPathProfile, bool) {
	prof, ok := permissionHandlerPathProfiles[p]
	return prof, ok
}

// AllPaths returns all four handler paths in automation order (most→least automated).
// This is a defensive copy.
func (r *PermissionHandlerPathRegistry) AllPaths() []PermissionHandlerPath {
	result := make([]PermissionHandlerPath, len(permissionHandlerPaths))
	copy(result, permissionHandlerPaths)
	return result
}

// AutomatedPaths returns paths that support automated resolution without
// requiring a user dialog as the primary resolution mechanism.
// §5.2: "In coordinator and some background paths, automated resolution is
// attempted before user interaction."
func (r *PermissionHandlerPathRegistry) AutomatedPaths() []PermissionHandlerPath {
	var result []PermissionHandlerPath
	for _, p := range permissionHandlerPaths {
		if permissionHandlerPathProfiles[p].SupportsAutomatedResolution {
			result = append(result, p)
		}
	}
	return result
}

// BackgroundAgentPaths returns paths used by background or coordinator agents that
// await automated checks before any user dialog.
func (r *PermissionHandlerPathRegistry) BackgroundAgentPaths() []PermissionHandlerPath {
	var result []PermissionHandlerPath
	for _, p := range permissionHandlerPaths {
		if permissionHandlerPathProfiles[p].IsBackgroundAgentPath {
			result = append(result, p)
		}
	}
	return result
}

// FallbackPath returns the interactive path, which is always the last resort.
// §5.2: "Interactive: The fallback path."
func (r *PermissionHandlerPathRegistry) FallbackPath() PermissionHandlerPath {
	return PermissionHandlerPathInteractive
}

// MostAutomatedPath returns the path with the lowest AutomationLevel (most automated).
func (r *PermissionHandlerPathRegistry) MostAutomatedPath() PermissionHandlerPath {
	return PermissionHandlerPathCoordinator
}

// PathsRequiringFeatureFlag returns paths that are gated by an environment feature flag.
func (r *PermissionHandlerPathRegistry) PathsRequiringFeatureFlag() []PermissionHandlerPath {
	var result []PermissionHandlerPath
	for _, p := range permissionHandlerPaths {
		if permissionHandlerPathProfiles[p].FeatureFlag != "" {
			result = append(result, p)
		}
	}
	return result
}

// IsValidPermissionHandlerPath returns true for the four recognized path strings.
func IsValidPermissionHandlerPath(s PermissionHandlerPath) bool {
	_, ok := permissionHandlerPathProfiles[s]
	return ok
}

// PermissionHandlerPathAutomationOrderIsAscending validates that
// permissionHandlerPaths has strictly ascending AutomationLevel values (1..4).
// This is a structural invariant of the §5.2 architecture ordering.
func PermissionHandlerPathAutomationOrderIsAscending() bool {
	for i, p := range permissionHandlerPaths {
		if permissionHandlerPathProfiles[p].AutomationLevel != i+1 {
			return false
		}
	}
	return true
}
