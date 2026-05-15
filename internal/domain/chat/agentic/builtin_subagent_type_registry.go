package agentic

// BuiltinSubagentTypeRegistry models the six built-in subagent types
// enumerated in §8.1 of the Claude Code architecture paper
// (arXiv:2604.14228v1, "The Agent Tool and Delegation Criteria").
//
// §8.1 states: "Claude Code provides up to six built-in subagent types,
// depending on feature flags and entrypoint" and then lists each by name
// with its distinguishing toolset policy, permission behaviour, and
// use-case purpose.
//
// The six types in PDF enumeration order:
//  1. Explore        — read/search-oriented; write+edit tools in deny-list
//  2. Plan           — creates structured plans; standard permission model
//  3. General-purpose — broadly capable; may fork to fork-subagent path
//  4. Claude Code Guide — onboarding/documentation; own permissionMode override
//  5. Verification   — runs validation checks (test suites, linting)
//  6. Statusline-setup — specialized for terminal status line configuration
//
// This registry is intentionally distinct from builtin_subagents.go
// (SUB-002: AgentHub's curated role roster) and subagent_isolation.go
// (§8.2: isolation modes). §8.1 captures the structural profile of each
// Claude Code built-in type: routing policy, toolset restrictions,
// permission override, background eligibility, and AgentHub mapping.

// BuiltinSubagentTypeSlug is the canonical slug for one of the six §8.1 types.
type BuiltinSubagentTypeSlug string

const (
	// BuiltinTypeExplore is type 1 (§8.1):
	// "Explore: primarily read/search-oriented investigation, with write and
	// edit tools in its deny-list."
	BuiltinTypeExplore BuiltinSubagentTypeSlug = "explore"

	// BuiltinTypePlan is type 2 (§8.1):
	// "Plan: creates structured plans; execution proceeds through the standard
	// permission model."
	BuiltinTypePlan BuiltinSubagentTypeSlug = "plan"

	// BuiltinTypeGeneralPurpose is type 3 (§8.1):
	// "General-purpose: broadly capable, used when explicitly requested
	// (note: omitting the type may route to the fork-subagent path instead)."
	BuiltinTypeGeneralPurpose BuiltinSubagentTypeSlug = "general_purpose"

	// BuiltinTypeClaudeCodeGuide is type 4 (§8.1):
	// "Claude Code Guide: onboarding and documentation assistance, with its
	// own permissionMode override."
	BuiltinTypeClaudeCodeGuide BuiltinSubagentTypeSlug = "claude_code_guide"

	// BuiltinTypeVerification is type 5 (§8.1):
	// "Verification: runs validation checks (test suites, linting)."
	BuiltinTypeVerification BuiltinSubagentTypeSlug = "verification"

	// BuiltinTypeStatuslineSetup is type 6 (§8.1):
	// "Statusline-setup: specialized for terminal status line configuration."
	BuiltinTypeStatuslineSetup BuiltinSubagentTypeSlug = "statusline_setup"
)

// BuiltinSubagentTypeProfile holds the immutable structural characteristics
// of one §8.1 built-in subagent type.
type BuiltinSubagentTypeProfile struct {
	// Slug is the canonical identifier from the PDF §8.1 enumeration.
	Slug BuiltinSubagentTypeSlug

	// PDFSection is the exact paper section reference.
	PDFSection string

	// Label is the human-readable name used in the paper.
	Label string

	// Description is the paper's one-line characterisation of the type.
	Description string

	// ToolsetCategory describes the category of tools available to this type.
	// Derived from the paper's deny-list or toolset notes.
	ToolsetCategory string // "read_only" | "plan_only" | "full" | "validation" | "docs" | "terminal"

	// HasWriteToolsDenied indicates that write and edit tools are placed in
	// the deny-list for this subagent type.
	// §8.1: only Explore explicitly has write+edit in its deny-list.
	HasWriteToolsDenied bool

	// PermissionModelOverride describes any permissionMode override that
	// this type carries. Empty string means the type uses the standard
	// inherited permission model.
	// §8.1: Claude Code Guide carries "its own permissionMode override".
	PermissionModelOverride string

	// MayRouteThroughForkSubagent indicates whether omitting this type
	// at invocation time may cause the harness to route to the fork-subagent
	// path instead.
	// §8.1: General-purpose notes this routing caveat explicitly.
	MayRouteThroughForkSubagent bool

	// IsFeatureGated indicates that the type's availability depends on a
	// feature flag or entrypoint condition.
	// §8.1: "Claude Code provides up to six built-in subagent types, depending
	// on feature flags and entrypoint."
	IsFeatureGated bool

	// UseCaseCategory classifies the type's primary purpose.
	UseCaseCategory string // "investigation" | "planning" | "general" | "documentation" | "validation" | "configuration"

	// AgenthubMapping is the AgentHub Go source location that implements
	// or approximates the equivalent behaviour.
	AgenthubMapping string
}

// seedBuiltinSubagentTypes is the canonical §8.1 registry in PDF enumeration order.
var seedBuiltinSubagentTypes = []BuiltinSubagentTypeProfile{
	{
		Slug:                        BuiltinTypeExplore,
		PDFSection:                  "§8.1",
		Label:                       "Explore",
		Description:                 "Primarily read/search-oriented investigation, with write and edit tools in its deny-list.",
		ToolsetCategory:             "read_only",
		HasWriteToolsDenied:         true,
		PermissionModelOverride:     "",
		MayRouteThroughForkSubagent: false,
		IsFeatureGated:              false,
		UseCaseCategory:             "investigation",
		AgenthubMapping:             "builtin_subagents.go: BuiltinSubagentExplorer; subagent_toolset_isolation.go: DenyWriteTools policy",
	},
	{
		Slug:                        BuiltinTypePlan,
		PDFSection:                  "§8.1",
		Label:                       "Plan",
		Description:                 "Creates structured plans; execution proceeds through the standard permission model.",
		ToolsetCategory:             "plan_only",
		HasWriteToolsDenied:         false,
		PermissionModelOverride:     "",
		MayRouteThroughForkSubagent: false,
		IsFeatureGated:              false,
		UseCaseCategory:             "planning",
		AgenthubMapping:             "builtin_subagents.go: BuiltinSubagentPlanner; permission_plan_mode.go: PlanModePermission",
	},
	{
		Slug:                        BuiltinTypeGeneralPurpose,
		PDFSection:                  "§8.1",
		Label:                       "General-purpose",
		Description:                 "Broadly capable, used when explicitly requested; omitting the type may route to the fork-subagent path instead.",
		ToolsetCategory:             "full",
		HasWriteToolsDenied:         false,
		PermissionModelOverride:     "",
		MayRouteThroughForkSubagent: true,
		IsFeatureGated:              false,
		UseCaseCategory:             "general",
		AgenthubMapping:             "builtin_subagents.go: BuiltinSubagentCoder; forkedagent.go: ForkSubagentDispatch",
	},
	{
		Slug:                        BuiltinTypeClaudeCodeGuide,
		PDFSection:                  "§8.1",
		Label:                       "Claude Code Guide",
		Description:                 "Onboarding and documentation assistance, with its own permissionMode override.",
		ToolsetCategory:             "docs",
		HasWriteToolsDenied:         false,
		PermissionModelOverride:     "docs_mode",
		MayRouteThroughForkSubagent: false,
		IsFeatureGated:              true,
		UseCaseCategory:             "documentation",
		AgenthubMapping:             "builtin_subagents.go: BuiltinSubagentDocumenter; subagent_permission_precedence.go: PermissionModeOverride",
	},
	{
		Slug:                        BuiltinTypeVerification,
		PDFSection:                  "§8.1",
		Label:                       "Verification",
		Description:                 "Runs validation checks (test suites, linting).",
		ToolsetCategory:             "validation",
		HasWriteToolsDenied:         false,
		PermissionModelOverride:     "",
		MayRouteThroughForkSubagent: false,
		IsFeatureGated:              false,
		UseCaseCategory:             "validation",
		AgenthubMapping:             "builtin_subagents.go: BuiltinSubagentReviewer; toolexec.go: validation tool dispatch",
	},
	{
		Slug:                        BuiltinTypeStatuslineSetup,
		PDFSection:                  "§8.1",
		Label:                       "Statusline-setup",
		Description:                 "Specialized for terminal status line configuration.",
		ToolsetCategory:             "terminal",
		HasWriteToolsDenied:         false,
		PermissionModelOverride:     "",
		MayRouteThroughForkSubagent: false,
		IsFeatureGated:              true,
		UseCaseCategory:             "configuration",
		AgenthubMapping:             "builtin_subagents.go: BuiltinSubagentCurator; config.go: terminal status configuration",
	},
}

// allBuiltinSubagentTypesBySlug is the lookup map built at init time.
var allBuiltinSubagentTypesBySlug map[BuiltinSubagentTypeSlug]BuiltinSubagentTypeProfile

func init() {
	allBuiltinSubagentTypesBySlug = make(map[BuiltinSubagentTypeSlug]BuiltinSubagentTypeProfile, len(seedBuiltinSubagentTypes))
	for _, t := range seedBuiltinSubagentTypes {
		allBuiltinSubagentTypesBySlug[t.Slug] = t
	}
}

// SeedBuiltinSubagentTypeCount is the number of §8.1 types captured in the registry.
// A compile-time-visible constant so tests can guard against silent deletions.
const SeedBuiltinSubagentTypeCount = 6

// SeedBuiltinSubagentTypeSlugs lists all slugs in PDF enumeration order.
// Tests use this to assert exhaustive coverage without hard-coding every slug.
var SeedBuiltinSubagentTypeSlugs = []BuiltinSubagentTypeSlug{
	BuiltinTypeExplore,
	BuiltinTypePlan,
	BuiltinTypeGeneralPurpose,
	BuiltinTypeClaudeCodeGuide,
	BuiltinTypeVerification,
	BuiltinTypeStatuslineSetup,
}

// BuiltinSubagentTypeRegistry provides structured queries over the §8.1
// built-in subagent types. All methods are pure read operations; the
// registry carries no mutable state.
type BuiltinSubagentTypeRegistry struct{}

// NewBuiltinSubagentTypeRegistry returns a ready-to-use registry.
func NewBuiltinSubagentTypeRegistry() *BuiltinSubagentTypeRegistry {
	return &BuiltinSubagentTypeRegistry{}
}

// FindBuiltinSubagentTypeBySlug returns the profile for the given slug.
// Returns (zero-value, false) when the slug is not in the registry.
func (r *BuiltinSubagentTypeRegistry) FindBuiltinSubagentTypeBySlug(slug BuiltinSubagentTypeSlug) (BuiltinSubagentTypeProfile, bool) {
	p, ok := allBuiltinSubagentTypesBySlug[slug]
	return p, ok
}

// AllBuiltinSubagentTypes returns all six §8.1 types in PDF enumeration order.
func (r *BuiltinSubagentTypeRegistry) AllBuiltinSubagentTypes() []BuiltinSubagentTypeProfile {
	result := make([]BuiltinSubagentTypeProfile, len(seedBuiltinSubagentTypes))
	copy(result, seedBuiltinSubagentTypes)
	return result
}

// TypesWithWriteToolsDenied returns types whose write and edit tools are
// placed in the deny-list.
// §8.1: only Explore explicitly carries this restriction.
func (r *BuiltinSubagentTypeRegistry) TypesWithWriteToolsDenied() []BuiltinSubagentTypeProfile {
	var result []BuiltinSubagentTypeProfile
	for _, t := range seedBuiltinSubagentTypes {
		if t.HasWriteToolsDenied {
			result = append(result, t)
		}
	}
	return result
}

// TypesWithPermissionModeOverride returns types that carry a non-default
// permissionMode override.
// §8.1: Claude Code Guide carries its own permissionMode override.
func (r *BuiltinSubagentTypeRegistry) TypesWithPermissionModeOverride() []BuiltinSubagentTypeProfile {
	var result []BuiltinSubagentTypeProfile
	for _, t := range seedBuiltinSubagentTypes {
		if t.PermissionModelOverride != "" {
			result = append(result, t)
		}
	}
	return result
}

// TypesThatMayFork returns types where omitting the type at invocation may
// cause routing to the fork-subagent path.
// §8.1: General-purpose carries this routing caveat.
func (r *BuiltinSubagentTypeRegistry) TypesThatMayFork() []BuiltinSubagentTypeProfile {
	var result []BuiltinSubagentTypeProfile
	for _, t := range seedBuiltinSubagentTypes {
		if t.MayRouteThroughForkSubagent {
			result = append(result, t)
		}
	}
	return result
}

// FeatureGatedTypes returns types whose availability depends on a feature
// flag or entrypoint condition.
// §8.1: "Claude Code provides up to six built-in subagent types, depending
// on feature flags and entrypoint."
func (r *BuiltinSubagentTypeRegistry) FeatureGatedTypes() []BuiltinSubagentTypeProfile {
	var result []BuiltinSubagentTypeProfile
	for _, t := range seedBuiltinSubagentTypes {
		if t.IsFeatureGated {
			result = append(result, t)
		}
	}
	return result
}

// TypesByUseCaseCategory returns types belonging to the given use-case
// category. Valid categories: "investigation", "planning", "general",
// "documentation", "validation", "configuration".
func (r *BuiltinSubagentTypeRegistry) TypesByUseCaseCategory(category string) []BuiltinSubagentTypeProfile {
	var result []BuiltinSubagentTypeProfile
	for _, t := range seedBuiltinSubagentTypes {
		if t.UseCaseCategory == category {
			result = append(result, t)
		}
	}
	return result
}

// TypesByToolsetCategory returns types belonging to the given toolset
// category. Valid categories: "read_only", "plan_only", "full",
// "validation", "docs", "terminal".
func (r *BuiltinSubagentTypeRegistry) TypesByToolsetCategory(category string) []BuiltinSubagentTypeProfile {
	var result []BuiltinSubagentTypeProfile
	for _, t := range seedBuiltinSubagentTypes {
		if t.ToolsetCategory == category {
			result = append(result, t)
		}
	}
	return result
}

// ReadOnlyType returns the single type whose toolset restricts write and
// edit tools. §8.1 names only Explore in this category.
func (r *BuiltinSubagentTypeRegistry) ReadOnlyType() (BuiltinSubagentTypeProfile, bool) {
	for _, t := range seedBuiltinSubagentTypes {
		if t.HasWriteToolsDenied {
			return t, true
		}
	}
	return BuiltinSubagentTypeProfile{}, false
}

// IsValidBuiltinSubagentTypeSlug returns true if slug corresponds to one
// of the six §8.1 types.
func IsValidBuiltinSubagentTypeSlug(slug BuiltinSubagentTypeSlug) bool {
	_, ok := allBuiltinSubagentTypesBySlug[slug]
	return ok
}
