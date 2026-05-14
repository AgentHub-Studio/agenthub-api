package agentic

// SubagentIsolationMode identifies one of the three named subagent execution contexts.
// §8.2: each subagent declares an isolation mode that governs filesystem access,
// permission inheritance, and background execution eligibility.
type SubagentIsolationMode string

const (
	// SubagentIsolationInProcess is the default mode. The subagent shares the
	// parent's filesystem but operates in an isolated conversation context.
	// Always web-applicable: AgentHub uses this for standard sub-runs.
	SubagentIsolationInProcess SubagentIsolationMode = "in_process"

	// SubagentIsolationRemote launches the subagent in a remote execution
	// environment; always a background agent. Applicable to AgentHub's
	// background and KAIROS agent execution model.
	SubagentIsolationRemote SubagentIsolationMode = "remote"

	// SubagentIsolationWorktree creates a temporary git worktree so the
	// subagent gets its own copy of the repository. Git/CLI-specific;
	// NOT applicable to the AgentHub web platform.
	SubagentIsolationWorktree SubagentIsolationMode = "worktree"
)

// SubagentIsolationProfile is the immutable characteristics of one isolation mode.
type SubagentIsolationProfile struct {
	Mode SubagentIsolationMode

	// IsWebApplicable marks modes usable in the AgentHub web platform.
	// worktree requires a local git CLI and is excluded.
	IsWebApplicable bool

	// SupportsBackground is true when the mode always runs as a background agent.
	// remote is always background; in_process can be sync or async.
	SupportsBackground bool

	// InheritsParentPermissions is true when the subagent inherits the parent's
	// session-level permission rules (no explicit allowedTools provided).
	// Two-tier scoping §8.2: SDK-level permissions always flow through;
	// session-level rules are inherited in in_process and replaced in remote.
	InheritsParentPermissions bool

	// RequiresGitWorktree is true when the mode depends on the git CLI.
	RequiresGitWorktree bool

	// IsDefault is true for the mode used when no isolation is specified.
	IsDefault bool

	// BubbleModeEligible is true when async subagents running in this mode can
	// escalate permission prompts to the parent terminal via bubble mode.
	BubbleModeEligible bool
}

var subagentIsolationProfiles = map[SubagentIsolationMode]SubagentIsolationProfile{
	SubagentIsolationInProcess: {
		Mode:                      SubagentIsolationInProcess,
		IsWebApplicable:           true,
		SupportsBackground:        false, // sync or async, not always background
		InheritsParentPermissions: true,
		RequiresGitWorktree:       false,
		IsDefault:                 true,
		BubbleModeEligible:        true,
	},
	SubagentIsolationRemote: {
		Mode:                      SubagentIsolationRemote,
		IsWebApplicable:           true,
		SupportsBackground:        true, // always background
		InheritsParentPermissions: false,
		RequiresGitWorktree:       false,
		IsDefault:                 false,
		BubbleModeEligible:        false,
	},
	SubagentIsolationWorktree: {
		Mode:                      SubagentIsolationWorktree,
		IsWebApplicable:           false, // CLI/git-specific
		SupportsBackground:        true,
		InheritsParentPermissions: false,
		RequiresGitWorktree:       true,
		IsDefault:                 false,
		BubbleModeEligible:        false,
	},
}

// AllSubagentIsolationModes is the canonical ordered slice of all three modes.
var AllSubagentIsolationModes = []SubagentIsolationMode{
	SubagentIsolationInProcess,
	SubagentIsolationRemote,
	SubagentIsolationWorktree,
}

// SubagentIsolationRegistry provides queries over the §8.2 isolation modes.
type SubagentIsolationRegistry struct{}

// NewSubagentIsolationRegistry returns a ready-to-use registry.
func NewSubagentIsolationRegistry() *SubagentIsolationRegistry {
	return &SubagentIsolationRegistry{}
}

// Profile returns the immutable profile for the given mode.
// Returns false if the mode is unknown.
func (r *SubagentIsolationRegistry) Profile(mode SubagentIsolationMode) (SubagentIsolationProfile, bool) {
	p, ok := subagentIsolationProfiles[mode]
	return p, ok
}

// AllModes returns all three modes as a defensive copy.
func (r *SubagentIsolationRegistry) AllModes() []SubagentIsolationMode {
	result := make([]SubagentIsolationMode, len(AllSubagentIsolationModes))
	copy(result, AllSubagentIsolationModes)
	return result
}

// WebApplicableModes returns only the modes usable in the AgentHub web platform.
func (r *SubagentIsolationRegistry) WebApplicableModes() []SubagentIsolationMode {
	var result []SubagentIsolationMode
	for _, mode := range AllSubagentIsolationModes {
		if subagentIsolationProfiles[mode].IsWebApplicable {
			result = append(result, mode)
		}
	}
	return result
}

// DefaultMode returns the mode used when no isolation is explicitly specified.
func (r *SubagentIsolationRegistry) DefaultMode() SubagentIsolationMode {
	for _, mode := range AllSubagentIsolationModes {
		if subagentIsolationProfiles[mode].IsDefault {
			return mode
		}
	}
	return SubagentIsolationInProcess
}

// BackgroundOnlyModes returns modes that always run as background agents.
func (r *SubagentIsolationRegistry) BackgroundOnlyModes() []SubagentIsolationMode {
	var result []SubagentIsolationMode
	for _, mode := range AllSubagentIsolationModes {
		if subagentIsolationProfiles[mode].SupportsBackground &&
			mode != SubagentIsolationInProcess {
			result = append(result, mode)
		}
	}
	return result
}

// BubbleModeEligibleModes returns modes where async subagents can escalate
// permission prompts to the parent terminal via bubble mode.
func (r *SubagentIsolationRegistry) BubbleModeEligibleModes() []SubagentIsolationMode {
	var result []SubagentIsolationMode
	for _, mode := range AllSubagentIsolationModes {
		if subagentIsolationProfiles[mode].BubbleModeEligible {
			result = append(result, mode)
		}
	}
	return result
}
