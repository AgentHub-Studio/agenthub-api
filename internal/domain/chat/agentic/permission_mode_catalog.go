package agentic

// PermissionModeCatalog is the authoritative §5.1 registry of all seven permission modes
// defined in the Claude Code architecture paper (arXiv:2604.14228v1).
//
// §5.1: "Seven permission modes exist across the type definitions (5 external modes at
// types/permissions.ts; auto added conditionally; bubble in the type union):
//  1. plan
//  2. default
//  3. acceptEdits
//  4. auto
//  5. dontAsk
//  6. bypassPermissions
//  7. bubble
//
// The five externally visible modes (acceptEdits, bypassPermissions, default, dontAsk,
// plan) are defined in the EXTERNAL_PERMISSION_MODES array. The auto mode is conditionally
// included only when the TRANSCRIPT_CLASSIFIER feature flag is active. The bubble mode
// exists in the type union but not in either mode array; it is used internally for
// subagent permission escalation (Section 8)."
//
// This catalog unifies all seven modes in one queryable structure, complementing:
//   - permission_mode_gradient.go: five-mode safety gradient (plan→bypassPermissions)
//   - permission.go: PermissionMode constants and rule evaluation
//   - permission_plan_mode.go: plan-mode plan session logic
//   - permission_bubble_mode.go: bubble-mode vertical escalation logic
//
// The catalog does NOT re-declare any PermissionMode constants. It solely provides
// the §5.1 metadata layer: visibility, activation conditions, autonomy rank, and
// prompt behavior.

// PermissionModeVisibility classifies how a permission mode appears to the user and SDK.
//
// §5.1 distinguishes three visibility tiers:
//   - External: listed in EXTERNAL_PERMISSION_MODES (types/permissions.ts) — user-selectable
//   - Conditional: included only when a feature flag is active (auto requires TRANSCRIPT_CLASSIFIER)
//   - Internal: present in the type union but not in any exported array (bubble — subagent-only)
type PermissionModeVisibility string

const (
	// PermissionModeVisibilityExternal means the mode is always present in EXTERNAL_PERMISSION_MODES.
	// Users and operators can select these modes directly.
	PermissionModeVisibilityExternal PermissionModeVisibility = "external"

	// PermissionModeVisibilityConditional means the mode is added to the available set only when
	// a specific feature flag is active. §5.1: "auto mode is conditionally included only when the
	// TRANSCRIPT_CLASSIFIER feature flag is active."
	PermissionModeVisibilityConditional PermissionModeVisibility = "conditional"

	// PermissionModeVisibilityInternal means the mode exists in the TypeScript type union but is
	// NOT listed in any exported mode array. §5.1: "bubble mode exists in the type union but not
	// in either mode array; it is used internally for subagent permission escalation."
	PermissionModeVisibilityInternal PermissionModeVisibility = "internal"
)

// PermissionModePromptBehavior describes how the mode handles tool invocations that would
// normally trigger a user confirmation prompt.
type PermissionModePromptBehavior string

const (
	// PermissionModePromptAlwaysAsk — every potentially dangerous operation shows a user prompt.
	// §5.1 plan: "The model must create a plan; execution proceeds only after user approval."
	// §5.1 default: "Standard interactive use. Most operations require user approval."
	PermissionModePromptAlwaysAsk PermissionModePromptBehavior = "always_ask"

	// PermissionModePromptAutoAcceptEdits — filesystem edits in the working directory are
	// auto-approved; shell commands still prompt.
	// §5.1 acceptEdits: "Edits within the working directory and certain filesystem shell commands
	// (mkdir, rmdir, touch, rm, mv, cp, sed) are auto-approved; other shell commands require approval."
	PermissionModePromptAutoAcceptEdits PermissionModePromptBehavior = "auto_accept_edits"

	// PermissionModePromptMLClassifier — an ML-based classifier evaluates requests that do not
	// pass fast-path checks; most actions proceed automatically.
	// §5.1 auto: "An ML-based classifier evaluates requests that do not pass fast-path checks
	// (gated by TRANSCRIPT_CLASSIFIER)."
	PermissionModePromptMLClassifier PermissionModePromptBehavior = "ml_classifier"

	// PermissionModePromptDenyOnAsk — no prompting occurs; any operation that would normally
	// prompt is auto-denied instead. Deny rules still apply.
	// §5.1 dontAsk: "No prompting, but deny rules are still enforced."
	PermissionModePromptDenyOnAsk PermissionModePromptBehavior = "deny_on_ask"

	// PermissionModePromptSkipMost — most permission prompts are skipped; safety-critical
	// checks and bypass-immune rules still apply.
	// §5.1 bypassPermissions: "Skips most permission prompts, but safety-critical checks and
	// bypass-immune rules still apply."
	PermissionModePromptSkipMost PermissionModePromptBehavior = "skip_most"

	// PermissionModePromptEscalate — the mode does not prompt locally; instead it escalates
	// confirmation requests to the parent agent in the subagent tree.
	// §5.1 bubble: "Internal-only mode for subagent permission escalation to the parent terminal."
	PermissionModePromptEscalate PermissionModePromptBehavior = "escalate_to_parent"
)

// PermissionModeCatalogEntry holds the immutable §5.1 characteristics of one permission mode.
type PermissionModeCatalogEntry struct {
	// Mode is the canonical PermissionMode value (declared in permission.go or sibling files).
	Mode PermissionMode

	// Visibility classifies whether the mode is external, conditional, or internal-only.
	Visibility PermissionModeVisibility

	// ActivationFlag is the environment/feature flag that enables this mode.
	// Empty string means the mode is always available once its visibility allows it.
	// §5.1: auto requires TRANSCRIPT_CLASSIFIER.
	ActivationFlag string

	// AutonomyRank is the 1-based ordering of modes from most supervised (1=plan) to
	// most autonomous (6=bypassPermissions). Bubble (7) is not ranked in the autonomy
	// spectrum — it is an escalation mechanism, not a user-facing autonomy level.
	// 0 means "not ranked in the autonomy spectrum."
	AutonomyRank int

	// InGradient indicates the mode participates in the §11.3 five-mode safety gradient.
	// The gradient covers plan, default, acceptEdits, auto, bypassPermissions.
	// dontAsk and bubble are NOT in the gradient.
	InGradient bool

	// PromptBehavior describes how this mode handles confirmation-requiring tool calls.
	PromptBehavior PermissionModePromptBehavior

	// DenyRulesEnforced indicates whether deny rules from PermissionRules.Deny still apply
	// even when the mode would otherwise allow or skip confirmation.
	// §5.1: bypassPermissions still enforces "bypass-immune rules"; dontAsk enforces deny rules.
	DenyRulesEnforced bool

	// SubagentOnly indicates the mode is exclusively used by subagents, not top-level agents.
	// §5.1: bubble is subagent-only.
	SubagentOnly bool

	// PDFSection cites the paper section that names this mode.
	PDFSection string

	// Description is the verbatim or close-paraphrase description from §5.1.
	Description string
}

// permissionModeCatalog is the package-level seed data for all seven §5.1 modes.
// Canonical ordering: plan(1) → default(2) → acceptEdits(3) → auto(4) → dontAsk(5)
// → bypassPermissions(6) → bubble(7, unranked).
var permissionModeCatalogEntries = []*PermissionModeCatalogEntry{
	{
		Mode:              PermissionModePlan,
		Visibility:        PermissionModeVisibilityExternal,
		ActivationFlag:    "",
		AutonomyRank:      1,
		InGradient:        true,
		PromptBehavior:    PermissionModePromptAlwaysAsk,
		DenyRulesEnforced: true,
		SubagentOnly:      false,
		PDFSection:        "5.1",
		Description:       "The model must create a plan; execution proceeds only after user approval.",
	},
	{
		Mode:              PermissionModeDefault,
		Visibility:        PermissionModeVisibilityExternal,
		ActivationFlag:    "",
		AutonomyRank:      2,
		InGradient:        true,
		PromptBehavior:    PermissionModePromptAlwaysAsk,
		DenyRulesEnforced: true,
		SubagentOnly:      false,
		PDFSection:        "5.1",
		Description:       "Standard interactive use. Most operations require user approval.",
	},
	{
		Mode:              PermissionModeAcceptEdits,
		Visibility:        PermissionModeVisibilityExternal,
		ActivationFlag:    "",
		AutonomyRank:      3,
		InGradient:        true,
		PromptBehavior:    PermissionModePromptAutoAcceptEdits,
		DenyRulesEnforced: true,
		SubagentOnly:      false,
		PDFSection:        "5.1",
		Description:       "Edits within the working directory and certain filesystem shell commands are auto-approved; other shell commands require approval.",
	},
	{
		Mode:              PermissionModeAuto,
		Visibility:        PermissionModeVisibilityConditional,
		ActivationFlag:    "TRANSCRIPT_CLASSIFIER",
		AutonomyRank:      4,
		InGradient:        true,
		PromptBehavior:    PermissionModePromptMLClassifier,
		DenyRulesEnforced: true,
		SubagentOnly:      false,
		PDFSection:        "5.1",
		Description:       "An ML-based classifier evaluates requests that do not pass fast-path checks (gated by TRANSCRIPT_CLASSIFIER).",
	},
	{
		Mode:              PermissionModeDontAsk,
		Visibility:        PermissionModeVisibilityExternal,
		ActivationFlag:    "",
		AutonomyRank:      5,
		InGradient:        false,
		PromptBehavior:    PermissionModePromptDenyOnAsk,
		DenyRulesEnforced: true,
		SubagentOnly:      false,
		PDFSection:        "5.1",
		Description:       "No prompting, but deny rules are still enforced.",
	},
	{
		Mode:              PermissionModeBypassPermissions,
		Visibility:        PermissionModeVisibilityExternal,
		ActivationFlag:    "",
		AutonomyRank:      6,
		InGradient:        true,
		PromptBehavior:    PermissionModePromptSkipMost,
		DenyRulesEnforced: true, // bypass-immune rules still apply
		SubagentOnly:      false,
		PDFSection:        "5.1",
		Description:       "Skips most permission prompts, but safety-critical checks and bypass-immune rules still apply.",
	},
	{
		Mode:              PermissionModeBubble,
		Visibility:        PermissionModeVisibilityInternal,
		ActivationFlag:    "",
		AutonomyRank:      0, // not ranked in the autonomy spectrum
		InGradient:        false,
		PromptBehavior:    PermissionModePromptEscalate,
		DenyRulesEnforced: true, // local deny rules still fire before escalation
		SubagentOnly:      true,
		PDFSection:        "5.1",
		Description:       "Internal-only mode for subagent permission escalation to the parent terminal.",
	},
}

// SeedPermissionModeCatalogCount is the number of entries seeded by §5.1.
const SeedPermissionModeCatalogCount = 7

// permissionModeCatalogByMode is the fast-lookup index built at init time.
var permissionModeCatalogByMode map[PermissionMode]*PermissionModeCatalogEntry

func init() {
	permissionModeCatalogByMode = make(map[PermissionMode]*PermissionModeCatalogEntry, SeedPermissionModeCatalogCount)
	for _, e := range permissionModeCatalogEntries {
		permissionModeCatalogByMode[e.Mode] = e
	}
}

// PermissionModeCatalogRegistry provides §5.1 queries over all seven permission modes.
// It complements PermissionModeGradientManager (which covers only the five-mode safety
// gradient) by adding visibility, activation conditions, and prompt-behavior semantics.
type PermissionModeCatalogRegistry struct{}

// NewPermissionModeCatalogRegistry returns a ready-to-use registry.
func NewPermissionModeCatalogRegistry() *PermissionModeCatalogRegistry {
	return &PermissionModeCatalogRegistry{}
}

// FindPermissionModeCatalogEntryBySlug looks up a catalog entry by its PermissionMode string.
// Returns (entry, true) if found; (nil, false) if unknown.
func (r *PermissionModeCatalogRegistry) FindPermissionModeCatalogEntryBySlug(slug string) (*PermissionModeCatalogEntry, bool) {
	e, ok := permissionModeCatalogByMode[PermissionMode(slug)]
	return e, ok
}

// AllModes returns all seven catalog entries in §5.1 enumeration order.
// This is a defensive copy of the slice (pointers share the immutable structs).
func (r *PermissionModeCatalogRegistry) AllModes() []*PermissionModeCatalogEntry {
	result := make([]*PermissionModeCatalogEntry, len(permissionModeCatalogEntries))
	copy(result, permissionModeCatalogEntries)
	return result
}

// ExternalModes returns the five modes that appear in EXTERNAL_PERMISSION_MODES.
// §5.1: acceptEdits, bypassPermissions, default, dontAsk, plan.
// Returned in autonomy-rank order.
func (r *PermissionModeCatalogRegistry) ExternalModes() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.Visibility == PermissionModeVisibilityExternal {
			result = append(result, e)
		}
	}
	return result
}

// ConditionalModes returns modes that require a feature flag to be active.
// §5.1: only auto (requires TRANSCRIPT_CLASSIFIER).
func (r *PermissionModeCatalogRegistry) ConditionalModes() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.Visibility == PermissionModeVisibilityConditional {
			result = append(result, e)
		}
	}
	return result
}

// InternalModes returns modes that exist in the type union but not in any exported array.
// §5.1: only bubble.
func (r *PermissionModeCatalogRegistry) InternalModes() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.Visibility == PermissionModeVisibilityInternal {
			result = append(result, e)
		}
	}
	return result
}

// GradientModes returns only the modes that participate in the §11.3 safety gradient,
// in gradient order (plan→default→acceptEdits→auto→bypassPermissions).
func (r *PermissionModeCatalogRegistry) GradientModes() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.InGradient {
			result = append(result, e)
		}
	}
	return result
}

// NonGradientModes returns modes that are NOT part of the §11.3 gradient.
// §5.1: dontAsk and bubble fall outside the graduated autonomy spectrum.
func (r *PermissionModeCatalogRegistry) NonGradientModes() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if !e.InGradient {
			result = append(result, e)
		}
	}
	return result
}

// ModesByPromptBehavior returns all catalog entries with the given prompt behavior.
func (r *PermissionModeCatalogRegistry) ModesByPromptBehavior(behavior PermissionModePromptBehavior) []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.PromptBehavior == behavior {
			result = append(result, e)
		}
	}
	return result
}

// SubagentOnlyModes returns modes reserved exclusively for subagent contexts.
// §5.1: only bubble.
func (r *PermissionModeCatalogRegistry) SubagentOnlyModes() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.SubagentOnly {
			result = append(result, e)
		}
	}
	return result
}

// MostAutonomousRankedMode returns the catalog entry with the highest AutonomyRank
// among modes that have a rank (AutonomyRank > 0). Per §5.1 that is bypassPermissions (rank 6).
func (r *PermissionModeCatalogRegistry) MostAutonomousRankedMode() *PermissionModeCatalogEntry {
	var best *PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.AutonomyRank > 0 && (best == nil || e.AutonomyRank > best.AutonomyRank) {
			best = e
		}
	}
	return best
}

// MostSupervisedRankedMode returns the catalog entry with the lowest AutonomyRank > 0.
// Per §5.1 that is plan (rank 1).
func (r *PermissionModeCatalogRegistry) MostSupervisedRankedMode() *PermissionModeCatalogEntry {
	var best *PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.AutonomyRank > 0 && (best == nil || e.AutonomyRank < best.AutonomyRank) {
			best = e
		}
	}
	return best
}

// ModesRequiringFeatureFlag returns modes with a non-empty ActivationFlag.
func (r *PermissionModeCatalogRegistry) ModesRequiringFeatureFlag() []*PermissionModeCatalogEntry {
	var result []*PermissionModeCatalogEntry
	for _, e := range permissionModeCatalogEntries {
		if e.ActivationFlag != "" {
			result = append(result, e)
		}
	}
	return result
}

// IsKnownPermissionMode returns true if the slug corresponds to one of the seven §5.1 modes.
func (r *PermissionModeCatalogRegistry) IsKnownPermissionMode(slug string) bool {
	_, ok := permissionModeCatalogByMode[PermissionMode(slug)]
	return ok
}

// --- Structural invariants ---

// PermissionModeCatalogHasSevenEntries validates the §5.1 count invariant.
func PermissionModeCatalogHasSevenEntries() bool {
	return len(permissionModeCatalogEntries) == SeedPermissionModeCatalogCount
}

// PermissionModeCatalogExternalCountIsFive validates the §5.1 statement that exactly
// five modes are in EXTERNAL_PERMISSION_MODES.
func PermissionModeCatalogExternalCountIsFive() bool {
	count := 0
	for _, e := range permissionModeCatalogEntries {
		if e.Visibility == PermissionModeVisibilityExternal {
			count++
		}
	}
	return count == 5
}

// PermissionModeCatalogOneBubbleMode validates that exactly one mode is internal-only (bubble).
func PermissionModeCatalogOneBubbleMode() bool {
	count := 0
	for _, e := range permissionModeCatalogEntries {
		if e.Visibility == PermissionModeVisibilityInternal {
			count++
		}
	}
	return count == 1
}

// PermissionModeCatalogOneConditionalMode validates that exactly one mode is conditional (auto).
func PermissionModeCatalogOneConditionalMode() bool {
	count := 0
	for _, e := range permissionModeCatalogEntries {
		if e.Visibility == PermissionModeVisibilityConditional {
			count++
		}
	}
	return count == 1
}

// PermissionModeCatalogGradientCountIsFive validates that exactly five modes are InGradient.
func PermissionModeCatalogGradientCountIsFive() bool {
	count := 0
	for _, e := range permissionModeCatalogEntries {
		if e.InGradient {
			count++
		}
	}
	return count == 5
}

// PermissionModeCatalogAutonomyRanksAreUnique validates that all ranked modes (rank > 0)
// have distinct AutonomyRank values.
func PermissionModeCatalogAutonomyRanksAreUnique() bool {
	seen := make(map[int]bool)
	for _, e := range permissionModeCatalogEntries {
		if e.AutonomyRank == 0 {
			continue
		}
		if seen[e.AutonomyRank] {
			return false
		}
		seen[e.AutonomyRank] = true
	}
	return true
}

// PermissionModeCatalogOnlySubagentModeIsBubble validates that bubble is the sole
// subagent-only mode. §5.1: bubble is exclusively for subagent escalation.
func PermissionModeCatalogOnlySubagentModeIsBubble() bool {
	for _, e := range permissionModeCatalogEntries {
		if e.SubagentOnly && e.Mode != PermissionModeBubble {
			return false
		}
	}
	// Also ensure bubble itself IS marked subagent-only.
	entry, ok := permissionModeCatalogByMode[PermissionModeBubble]
	return ok && entry.SubagentOnly
}
