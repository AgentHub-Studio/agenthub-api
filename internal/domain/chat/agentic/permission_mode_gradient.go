package agentic

// §11.3: five permission modes form a monotonically decreasing safety gradient.
// plan → default → acceptEdits → auto → bypassPermissions
//
// PermissionMode, PermissionModeDefault, and PermissionModePlan are declared in
// permission.go and permission_plan_mode.go respectively. This file adds the
// three remaining canonical modes and the gradient manager.

const (
	// PermissionModeAcceptEdits auto-accepts file edits; shell commands still prompt.
	PermissionModeAcceptEdits PermissionMode = "acceptEdits"
	// PermissionModeAuto uses an ML classifier; most actions proceed automatically.
	// Background-capable (KAIROS-compatible). SafetyScore = 40.
	PermissionModeAuto PermissionMode = "auto"
	// PermissionModeBypassPermissions skips most permission prompts.
	// Reserved for trusted automation pipelines and service agents. SafetyScore = 0.
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

// PermissionGradient is the canonical ordered slice from safest (index 0) to most autonomous.
// Invariant (§11.3): safety scores are strictly decreasing along this slice.
var PermissionGradient = []PermissionMode{
	PermissionModePlan,
	PermissionModeDefault,
	PermissionModeAcceptEdits,
	PermissionModeAuto,
	PermissionModeBypassPermissions,
}

// PermissionModeProfile holds the immutable characteristics of one permission mode.
type PermissionModeProfile struct {
	Mode                 PermissionMode
	GradientIndex        int  // 0=plan (safest) → 4=bypassPermissions (most autonomous)
	SafetyScore          int  // 0–100; strictly decreasing along the gradient
	RequiresConfirmation bool // user must approve before each action
	AutoAcceptsEdits     bool // file-system edits accepted without prompt
	AllowsBackgroundRun  bool // agent may run without user present (KAIROS-compatible)
	AllowsToolBypass     bool // skips most permission prompts (bypassPermissions only)
}

var permissionModeProfiles = map[PermissionMode]PermissionModeProfile{
	PermissionModePlan: {
		Mode: PermissionModePlan, GradientIndex: 0, SafetyScore: 100,
		RequiresConfirmation: true,
	},
	PermissionModeDefault: {
		Mode: PermissionModeDefault, GradientIndex: 1, SafetyScore: 80,
		RequiresConfirmation: true,
	},
	PermissionModeAcceptEdits: {
		Mode: PermissionModeAcceptEdits, GradientIndex: 2, SafetyScore: 60,
		AutoAcceptsEdits: true,
	},
	PermissionModeAuto: {
		Mode: PermissionModeAuto, GradientIndex: 3, SafetyScore: 40,
		AutoAcceptsEdits: true, AllowsBackgroundRun: true,
	},
	PermissionModeBypassPermissions: {
		Mode: PermissionModeBypassPermissions, GradientIndex: 4, SafetyScore: 0,
		AutoAcceptsEdits: true, AllowsBackgroundRun: true, AllowsToolBypass: true,
	},
}

// PermissionModeGradientManager provides safe navigation of the five-mode gradient.
type PermissionModeGradientManager struct{}

// NewPermissionModeGradientManager returns a ready-to-use manager.
func NewPermissionModeGradientManager() *PermissionModeGradientManager {
	return &PermissionModeGradientManager{}
}

// Profile returns the immutable profile for the given mode.
// Returns false if the mode is not a known gradient position.
func (m *PermissionModeGradientManager) Profile(mode PermissionMode) (PermissionModeProfile, bool) {
	p, ok := permissionModeProfiles[mode]
	return p, ok
}

// Compare returns -1 if a is safer than b, +1 if a is more autonomous, 0 if equal.
// Unknown modes compare equal to everything.
func (m *PermissionModeGradientManager) Compare(a, b PermissionMode) int {
	pa, aok := permissionModeProfiles[a]
	pb, bok := permissionModeProfiles[b]
	if !aok || !bok {
		return 0
	}
	switch {
	case pa.GradientIndex < pb.GradientIndex:
		return -1
	case pa.GradientIndex > pb.GradientIndex:
		return 1
	default:
		return 0
	}
}

// IsSaferThan returns true when a has a lower gradient index than b.
func (m *PermissionModeGradientManager) IsSaferThan(a, b PermissionMode) bool {
	return m.Compare(a, b) < 0
}

// Escalate returns the next mode toward more autonomy.
// Returns (mode, false) if already at bypassPermissions.
func (m *PermissionModeGradientManager) Escalate(mode PermissionMode) (PermissionMode, bool) {
	p, ok := permissionModeProfiles[mode]
	if !ok || p.GradientIndex >= len(PermissionGradient)-1 {
		return mode, false
	}
	return PermissionGradient[p.GradientIndex+1], true
}

// Deescalate returns the next mode toward more safety.
// Returns (mode, false) if already at plan.
func (m *PermissionModeGradientManager) Deescalate(mode PermissionMode) (PermissionMode, bool) {
	p, ok := permissionModeProfiles[mode]
	if !ok || p.GradientIndex == 0 {
		return mode, false
	}
	return PermissionGradient[p.GradientIndex-1], true
}

// AllModes returns the full gradient as a defensive copy.
func (m *PermissionModeGradientManager) AllModes() []PermissionMode {
	result := make([]PermissionMode, len(PermissionGradient))
	copy(result, PermissionGradient)
	return result
}

// SafestBackgroundCapable returns the mode with lowest gradient index that AllowsBackgroundRun.
// Per §11.3, that is PermissionModeAuto.
func (m *PermissionModeGradientManager) SafestBackgroundCapable() (PermissionMode, bool) {
	for _, mode := range PermissionGradient {
		if permissionModeProfiles[mode].AllowsBackgroundRun {
			return mode, true
		}
	}
	return "", false
}

// ModesRequiringConfirmation returns all modes where RequiresConfirmation is true, in gradient order.
func (m *PermissionModeGradientManager) ModesRequiringConfirmation() []PermissionMode {
	var result []PermissionMode
	for _, mode := range PermissionGradient {
		if permissionModeProfiles[mode].RequiresConfirmation {
			result = append(result, mode)
		}
	}
	return result
}

// IsPermissionGradientMonotonicallyDecreasing validates the §11.3 safety invariant:
// each mode in the gradient must have a strictly lower safety score than the previous.
func IsPermissionGradientMonotonicallyDecreasing() bool {
	for i := 1; i < len(PermissionGradient); i++ {
		prev := permissionModeProfiles[PermissionGradient[i-1]]
		curr := permissionModeProfiles[PermissionGradient[i]]
		if curr.SafetyScore >= prev.SafetyScore {
			return false
		}
	}
	return true
}
