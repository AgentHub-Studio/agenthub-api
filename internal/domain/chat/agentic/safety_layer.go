package agentic

// SafetyLayerRegistry models the seven independent permission/safety layers
// from arXiv:2604.14228v1 §3.5 "Permission and Safety Layers".
//
// The paper's key property: a request MUST pass through ALL applicable layers,
// and ANY SINGLE LAYER can block it. The layers are independent — no layer
// relies on another having already run.
//
// Each layer corresponds to a specific source artefact in the original
// TypeScript codebase (permissions.ts, types/permissions.ts, etc.).

// SafetyLayerID identifies one of the seven independent safety layers (§3.5).
type SafetyLayerID string

const (
	// SafetyLayerToolPreFilter — layer 1.
	// Blanket-denied tools are removed from the model's view before any call,
	// preventing the model from attempting to invoke them.
	// Source: tools.ts.
	SafetyLayerToolPreFilter SafetyLayerID = "tool_pre_filter"

	// SafetyLayerDenyFirstRuleEval — layer 2.
	// Deny rules always take precedence over allow rules, even when the allow
	// rule is more specific.
	// Source: permissions.ts.
	SafetyLayerDenyFirstRuleEval SafetyLayerID = "deny_first_rule_eval"

	// SafetyLayerPermissionModeConstraint — layer 3.
	// The active mode determines baseline handling for requests matching no
	// explicit rule.
	// Source: types/permissions.ts.
	SafetyLayerPermissionModeConstraint SafetyLayerID = "permission_mode_constraint"

	// SafetyLayerAutoModeClassifier — layer 4.
	// An ML-based classifier evaluates tool safety, potentially denying requests
	// the rule system would allow.
	// Source: yoloClassifier.ts (internal name from §3.3).
	SafetyLayerAutoModeClassifier SafetyLayerID = "auto_mode_classifier"

	// SafetyLayerShellSandbox — layer 5.
	// Approved shell commands may still execute inside a sandbox restricting
	// filesystem and network access.
	// Source: shouldUseSandbox.ts.
	SafetyLayerShellSandbox SafetyLayerID = "shell_sandbox"

	// SafetyLayerNoResumePermissions — layer 6.
	// Session-scoped permissions are NOT restored on resume or fork, preventing
	// stale permission grants from persisting across sessions.
	// Source: conversationRecovery.ts.
	SafetyLayerNoResumePermissions SafetyLayerID = "no_resume_permissions"

	// SafetyLayerHookInterception — layer 7.
	// PreToolUse hooks can modify permission decisions; PermissionRequest hooks
	// can resolve decisions asynchronously alongside the user dialog (or before
	// it, in coordinator mode).
	// Source: types/hooks.ts.
	SafetyLayerHookInterception SafetyLayerID = "hook_interception"
)

// safetylayerOrder is the canonical §3.5 enumeration order (1..7).
var safetyLayerOrder = []SafetyLayerID{
	SafetyLayerToolPreFilter,
	SafetyLayerDenyFirstRuleEval,
	SafetyLayerPermissionModeConstraint,
	SafetyLayerAutoModeClassifier,
	SafetyLayerShellSandbox,
	SafetyLayerNoResumePermissions,
	SafetyLayerHookInterception,
}

// SafetyLayerProfile holds the §3.5 metadata for one safety layer.
type SafetyLayerProfile struct {
	ID              SafetyLayerID
	LayerIndex      int    // 1-based, matching the PDF's numbered list
	Label           string // short label used in the paper
	Description     string // what the layer does
	SourceArtefact  string // TypeScript file cited in the paper
	CanBlock        bool   // always true; included for explicit documentation
}

// allSafetyLayerProfiles is the authoritative §3.5 layer data.
var allSafetyLayerProfiles = map[SafetyLayerID]SafetyLayerProfile{
	SafetyLayerToolPreFilter: {
		ID:             SafetyLayerToolPreFilter,
		LayerIndex:     1,
		Label:          "Tool pre-filtering",
		Description:    "Blanket-denied tools are removed from the model's view before any call, preventing the model from attempting to invoke them.",
		SourceArtefact: "tools.ts",
		CanBlock:       true,
	},
	SafetyLayerDenyFirstRuleEval: {
		ID:             SafetyLayerDenyFirstRuleEval,
		LayerIndex:     2,
		Label:          "Deny-first rule evaluation",
		Description:    "Deny rules always take precedence over allow rules, even when the allow rule is more specific.",
		SourceArtefact: "permissions.ts",
		CanBlock:       true,
	},
	SafetyLayerPermissionModeConstraint: {
		ID:             SafetyLayerPermissionModeConstraint,
		LayerIndex:     3,
		Label:          "Permission mode constraints",
		Description:    "The active mode determines baseline handling for requests matching no explicit rule.",
		SourceArtefact: "types/permissions.ts",
		CanBlock:       true,
	},
	SafetyLayerAutoModeClassifier: {
		ID:             SafetyLayerAutoModeClassifier,
		LayerIndex:     4,
		Label:          "Auto-mode classifier",
		Description:    "An ML-based classifier evaluates tool safety, potentially denying requests the rule system would allow.",
		SourceArtefact: "yoloClassifier.ts",
		CanBlock:       true,
	},
	SafetyLayerShellSandbox: {
		ID:             SafetyLayerShellSandbox,
		LayerIndex:     5,
		Label:          "Shell sandboxing",
		Description:    "Approved shell commands may still execute inside a sandbox restricting filesystem and network access.",
		SourceArtefact: "shouldUseSandbox.ts",
		CanBlock:       true,
	},
	SafetyLayerNoResumePermissions: {
		ID:             SafetyLayerNoResumePermissions,
		LayerIndex:     6,
		Label:          "Not restoring permissions on resume",
		Description:    "Session-scoped permissions are not restored on resume or fork.",
		SourceArtefact: "conversationRecovery.ts",
		CanBlock:       true,
	},
	SafetyLayerHookInterception: {
		ID:             SafetyLayerHookInterception,
		LayerIndex:     7,
		Label:          "Hook-based interception",
		Description:    "PreToolUse hooks can modify permission decisions; PermissionRequest hooks can resolve decisions asynchronously.",
		SourceArtefact: "types/hooks.ts",
		CanBlock:       true,
	},
}

// SafetyLayerRegistry provides structured access to the §3.5 seven-layer model.
type SafetyLayerRegistry struct{}

// NewSafetyLayerRegistry returns a ready-to-use registry.
func NewSafetyLayerRegistry() *SafetyLayerRegistry {
	return &SafetyLayerRegistry{}
}

// Profile returns the §3.5 profile for a given layer ID. ok=false if unknown.
func (r *SafetyLayerRegistry) Profile(id SafetyLayerID) (SafetyLayerProfile, bool) {
	p, ok := allSafetyLayerProfiles[id]
	return p, ok
}

// AllLayers returns all seven profiles in §3.5 enumeration order (layer 1..7).
func (r *SafetyLayerRegistry) AllLayers() []SafetyLayerProfile {
	result := make([]SafetyLayerProfile, len(safetyLayerOrder))
	for i, id := range safetyLayerOrder {
		result[i] = allSafetyLayerProfiles[id]
	}
	return result
}

// Count returns the number of registered safety layers (always 7 per §3.5).
func (r *SafetyLayerRegistry) Count() int {
	return len(safetyLayerOrder)
}

// IsValidSafetyLayer returns true iff id is one of the seven §3.5 layer IDs.
func (r *SafetyLayerRegistry) IsValidSafetyLayer(id SafetyLayerID) bool {
	_, ok := allSafetyLayerProfiles[id]
	return ok
}

// LayerByIndex returns the profile for a 1-based index (1..7).
// Returns false if the index is out of range.
func (r *SafetyLayerRegistry) LayerByIndex(index int) (SafetyLayerProfile, bool) {
	if index < 1 || index > len(safetyLayerOrder) {
		return SafetyLayerProfile{}, false
	}
	return allSafetyLayerProfiles[safetyLayerOrder[index-1]], true
}

// LayersBySourceArtefact returns all layers whose SourceArtefact matches the
// given filename (exact match). Useful for tracing which artefact implements
// which safety concerns.
func (r *SafetyLayerRegistry) LayersBySourceArtefact(artefact string) []SafetyLayerProfile {
	var result []SafetyLayerProfile
	for _, id := range safetyLayerOrder {
		p := allSafetyLayerProfiles[id]
		if p.SourceArtefact == artefact {
			result = append(result, p)
		}
	}
	return result
}

// SafetyKey is the core §3.5 guarantee: ALL layers apply, ANY single layer
// can block. This property is modelled as a named constant for documentation
// and assertion purposes.
const (
	// SafetyPropertyAllLayersApply documents the §3.5 guarantee that a request
	// must pass through all applicable layers.
	SafetyPropertyAllLayersApply = "all_layers_apply"

	// SafetyPropertyAnySingleLayerBlocks documents the §3.5 guarantee that any
	// single layer can block a request independently.
	SafetyPropertyAnySingleLayerBlocks = "any_single_layer_blocks"
)

// SafetyProperties returns the two core §3.5 guarantees as a slice.
func SafetyProperties() []string {
	return []string{SafetyPropertyAllLayersApply, SafetyPropertyAnySingleLayerBlocks}
}
