package agentic

import "fmt"

// conditional_tool_availability_registry.go — FEAT-046 — Appendix A §A.2 Table 8
//
// arXiv:2604.14228v1, Appendix A §A.2 "Conditional Tool Availability" (page 44–45).
//
// §A.2 explains that getAllBaseTools() (tools.ts) constructs different tool sets
// depending on mode, build, environment, and feature flags.  Table 8 (page 45)
// enumerates four mutually-exclusive availability categories that cover all tools
// in the Claude Code package:
//
//  1. AlwaysIncluded  — tools present in every build and every mode, regardless
//     of environment or feature flags.  Examples: AgentTool, BashTool,
//     FileReadTool, FileEditTool, FileWriteTool, SkillTool, WebFetchTool,
//     WebSearchTool.
//
//  2. Environment     — tools whose presence depends on the runtime environment
//     or the target platform detected at build time.  Examples:
//     GlobTool/GrepTool (excluded when embedded), ConfigTool (ant-only),
//     PowerShellTool (Windows-only).
//
//  3. FeatureFlag     — tools gated behind a named feature flag that must be
//     explicitly enabled.  Examples: TaskCreate/Get/Update/List (todoV2),
//     EnterWorktreeTool (worktree), TeamTools (swarms), ToolSearchTool.
//
//  4. NullChecked     — tools whose slot in the tool array is null-checked at
//     assembly time; the tool is omitted when the underlying capability is
//     absent.  Examples: SuggestBackgroundPRTool, WebBrowserTool,
//     RemoteTriggerTool, MonitorTool, SleepTool.
//
// The registry is pure Go, no DB, no HTTP.  It is intended for introspection,
// documentation generation, and test-driven validation of Table 8 claims.

// ConditionalToolCategory is the availability category assigned to a tool or
// tool group in Table 8 of arXiv:2604.14228v1.
type ConditionalToolCategory string

const (
	// CategoryAlwaysIncluded — tools present in every build and mode.
	// Activation condition: unconditional.
	CategoryAlwaysIncluded ConditionalToolCategory = "always_included"

	// CategoryEnvironment — tools whose presence depends on the runtime
	// environment or target platform detected at build time.
	CategoryEnvironment ConditionalToolCategory = "environment"

	// CategoryFeatureFlag — tools gated behind a named feature flag.
	CategoryFeatureFlag ConditionalToolCategory = "feature_flag"

	// CategoryNullChecked — tools whose slot is null-checked at tool-pool
	// assembly time; omitted when the underlying capability is absent.
	CategoryNullChecked ConditionalToolCategory = "null_checked"
)

// conditionalToolCategories is the complete bounded set of categories.
var conditionalToolCategories = []ConditionalToolCategory{
	CategoryAlwaysIncluded,
	CategoryEnvironment,
	CategoryFeatureFlag,
	CategoryNullChecked,
}

// ConditionalToolAvailabilityRule represents one row (or logical group) from
// Table 8.  Each rule has a canonical RuleID, a category, a human-readable
// label, representative affected tools, and the conditions under which the
// tools enter or leave the active tool pool.
type ConditionalToolAvailabilityRule struct {
	// RuleID is the stable, kebab-case identifier for this rule.
	RuleID string

	// Category is one of the four Table 8 categories.
	Category ConditionalToolCategory

	// Label is a short display name for the rule.
	Label string

	// Description summarises the availability semantics in plain English.
	Description string

	// PDFSection is the source reference within arXiv:2604.14228v1.
	PDFSection string

	// ActivationCondition is a human-readable predicate that must be true for
	// the affected tools to be included in the active tool pool.
	ActivationCondition string

	// DeactivationCondition is the complement: when this predicate is true, the
	// tools are excluded.  Empty for CategoryAlwaysIncluded rules.
	DeactivationCondition string

	// AffectedTools lists the canonical tool names in this rule's examples.
	AffectedTools []string

	// IsDefault indicates whether the tools are present in the default
	// (no-flags, interactive) mode.
	IsDefault bool

	// RequiresUserGrant indicates whether a user must explicitly enable the
	// feature (e.g., pass a flag or toggle a setting) for the tools to appear.
	RequiresUserGrant bool

	// OverridableByUser indicates whether an interactive user can change the
	// activation state of these tools at runtime (e.g., by passing a CLI flag).
	OverridableByUser bool
}

// ConditionalToolAvailabilityRegistry holds all rules from Appendix A Table 8.
// It provides query methods for inspection and test-driven validation.
type ConditionalToolAvailabilityRegistry struct {
	rules []ConditionalToolAvailabilityRule
}

// newConditionalToolAvailabilityRegistry constructs the registry with every
// rule derived from Table 8 (arXiv:2604.14228v1, page 45).
func newConditionalToolAvailabilityRegistry() *ConditionalToolAvailabilityRegistry {
	rules := []ConditionalToolAvailabilityRule{
		// ── Category: AlwaysIncluded ──────────────────────────────────────────
		{
			RuleID:      "always-core-agent-tools",
			Category:    CategoryAlwaysIncluded,
			Label:       "Core Agent Tools",
			Description: "The foundational tools that are present in every build and every execution mode, including simple (Bash/Read/Edit only), full interactive, and headless SDK modes. These tools constitute the minimum viable tool set for agentic operation.",
			PDFSection:  "Appendix A §A.2 Table 8",
			ActivationCondition:   "unconditional — always included",
			DeactivationCondition: "",
			AffectedTools: []string{
				"AgentTool",
				"BashTool",
				"FileReadTool",
				"FileEditTool",
				"FileWriteTool",
				"SkillTool",
				"WebFetchTool",
				"WebSearchTool",
			},
			IsDefault:         true,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},

		// ── Category: Environment ─────────────────────────────────────────────
		{
			RuleID:   "env-glob-grep-not-embedded",
			Category: CategoryEnvironment,
			Label:    "Glob/Grep Tools (non-embedded)",
			Description: "GlobTool and GrepTool are included in standard CLI and headless SDK builds but excluded when Claude Code is embedded inside another application (e.g., an IDE plugin). The embedding host typically provides its own file-search primitives.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "runtime environment is not embedded",
			DeactivationCondition: "EMBEDDED build flag is set; host provides equivalent search",
			AffectedTools: []string{
				"GlobTool",
				"GrepTool",
			},
			IsDefault:         true,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
		{
			RuleID:   "env-config-tool-ant-only",
			Category: CategoryEnvironment,
			Label:    "ConfigTool (Anthropic-internal only)",
			Description: "ConfigTool is restricted to Anthropic-internal (ant) builds. It exposes configuration management capabilities that are not intended for external users or standard deployments.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "build target is ant (Anthropic-internal)",
			DeactivationCondition: "build target is external or community",
			AffectedTools: []string{
				"ConfigTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
		{
			RuleID:   "env-powershell-windows",
			Category: CategoryEnvironment,
			Label:    "PowerShellTool (Windows)",
			Description: "PowerShellTool is included only on Windows platforms, where PowerShell is the native shell. On macOS and Linux the BashTool covers the shell-execution role.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "runtime platform is Windows",
			DeactivationCondition: "runtime platform is macOS or Linux",
			AffectedTools: []string{
				"PowerShellTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},

		// ── Category: FeatureFlag ─────────────────────────────────────────────
		{
			RuleID:   "flag-todo-v2",
			Category: CategoryFeatureFlag,
			Label:    "Task Management Tools (todoV2 flag)",
			Description: "The four task-management tools (TaskCreate, TaskGet, TaskUpdate, TaskList) are enabled only when the todoV2 feature flag is active. This flag controls the second-generation task-tracking subsystem.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "feature flag todoV2 is enabled",
			DeactivationCondition: "todoV2 flag absent or disabled",
			AffectedTools: []string{
				"TaskCreateTool",
				"TaskGetTool",
				"TaskUpdateTool",
				"TaskListTool",
			},
			IsDefault:         false,
			RequiresUserGrant: true,
			OverridableByUser: true,
		},
		{
			RuleID:   "flag-worktree",
			Category: CategoryFeatureFlag,
			Label:    "EnterWorktreeTool (worktree flag)",
			Description: "EnterWorktreeTool is gated behind the worktree feature flag, enabling git-worktree-based parallel session isolation when the flag is explicitly enabled.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "feature flag worktree is enabled",
			DeactivationCondition: "worktree flag absent or disabled",
			AffectedTools: []string{
				"EnterWorktreeTool",
			},
			IsDefault:         false,
			RequiresUserGrant: true,
			OverridableByUser: true,
		},
		{
			RuleID:   "flag-swarms",
			Category: CategoryFeatureFlag,
			Label:    "TeamTools (swarms flag)",
			Description: "The multi-agent team coordination tools are enabled only when the swarms feature flag is active. This flag gates the experimental parallel-subagent coordination subsystem.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "feature flag swarms is enabled",
			DeactivationCondition: "swarms flag absent or disabled",
			AffectedTools: []string{
				"TeamTools",
			},
			IsDefault:         false,
			RequiresUserGrant: true,
			OverridableByUser: true,
		},
		{
			RuleID:   "flag-tool-search",
			Category: CategoryFeatureFlag,
			Label:    "ToolSearchTool (feature flag)",
			Description: "ToolSearchTool is gated behind a feature flag. It provides dynamic tool discovery within a running session; its inclusion is controlled via a named flag rather than environment detection.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "ToolSearch feature flag is enabled",
			DeactivationCondition: "ToolSearch flag absent or disabled",
			AffectedTools: []string{
				"ToolSearchTool",
			},
			IsDefault:         false,
			RequiresUserGrant: true,
			OverridableByUser: true,
		},

		// ── Category: NullChecked ─────────────────────────────────────────────
		{
			RuleID:   "null-suggest-background-pr",
			Category: CategoryNullChecked,
			Label:    "SuggestBackgroundPRTool (null-checked)",
			Description: "SuggestBackgroundPRTool is null-checked at tool-pool assembly time. Its slot in the tool array is populated only when the underlying background-PR capability is available; otherwise the slot resolves to null and is excluded.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "background-PR capability is non-null at assembly time",
			DeactivationCondition: "background-PR capability is null (unavailable)",
			AffectedTools: []string{
				"SuggestBackgroundPRTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
		{
			RuleID:   "null-web-browser",
			Category: CategoryNullChecked,
			Label:    "WebBrowserTool (null-checked)",
			Description: "WebBrowserTool is null-checked at assembly time. It is included only when a browser runtime is available to the process; headless or embedded environments without a browser receive null.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "browser runtime capability is non-null at assembly time",
			DeactivationCondition: "browser runtime is null (headless or embedded)",
			AffectedTools: []string{
				"WebBrowserTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
		{
			RuleID:   "null-remote-trigger",
			Category: CategoryNullChecked,
			Label:    "RemoteTriggerTool (null-checked)",
			Description: "RemoteTriggerTool is null-checked at assembly time. The tool's availability depends on a remote-execution backend being configured; without it the slot is null.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "remote-execution backend capability is non-null at assembly time",
			DeactivationCondition: "remote-execution backend is null (not configured)",
			AffectedTools: []string{
				"RemoteTriggerTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
		{
			RuleID:   "null-monitor",
			Category: CategoryNullChecked,
			Label:    "MonitorTool (null-checked)",
			Description: "MonitorTool is null-checked at assembly time. It is present only when a background-process monitoring capability is available in the current build or session configuration.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "background-process monitoring capability is non-null",
			DeactivationCondition: "background-process monitoring capability is null",
			AffectedTools: []string{
				"MonitorTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
		{
			RuleID:   "null-sleep",
			Category: CategoryNullChecked,
			Label:    "SleepTool (null-checked)",
			Description: "SleepTool is null-checked at assembly time. Its slot is populated only when the sleep/delay capability is available (e.g., in proactivity-enabled builds with KAIROS integration); otherwise null.",
			PDFSection:          "Appendix A §A.2 Table 8",
			ActivationCondition: "sleep capability is non-null at assembly time",
			DeactivationCondition: "sleep capability is null (not enabled)",
			AffectedTools: []string{
				"SleepTool",
			},
			IsDefault:         false,
			RequiresUserGrant: false,
			OverridableByUser: false,
		},
	}

	return &ConditionalToolAvailabilityRegistry{rules: rules}
}

// globalConditionalToolAvailabilityRegistry is the package-level singleton.
var globalConditionalToolAvailabilityRegistry = newConditionalToolAvailabilityRegistry()

// ConditionalToolAvailabilityRegistryInstance returns the package-level
// singleton registry for Table 8 rules.
func ConditionalToolAvailabilityRegistryInstance() *ConditionalToolAvailabilityRegistry {
	return globalConditionalToolAvailabilityRegistry
}

// AllRules returns a copy of all rules in the registry.
func (r *ConditionalToolAvailabilityRegistry) AllRules() []ConditionalToolAvailabilityRule {
	out := make([]ConditionalToolAvailabilityRule, len(r.rules))
	copy(out, r.rules)
	return out
}

// Count returns the total number of rules.
func (r *ConditionalToolAvailabilityRegistry) Count() int {
	return len(r.rules)
}

// FindByID returns the rule with the given RuleID and true, or a zero value
// and false if no such rule exists.
func (r *ConditionalToolAvailabilityRegistry) FindByID(ruleID string) (ConditionalToolAvailabilityRule, bool) {
	for _, rule := range r.rules {
		if rule.RuleID == ruleID {
			return rule, true
		}
	}
	return ConditionalToolAvailabilityRule{}, false
}

// IsValidRuleID reports whether ruleID matches a rule in the registry.
func (r *ConditionalToolAvailabilityRegistry) IsValidRuleID(ruleID string) bool {
	_, ok := r.FindByID(ruleID)
	return ok
}

// RulesByCategory returns all rules belonging to the given category.
func (r *ConditionalToolAvailabilityRegistry) RulesByCategory(cat ConditionalToolCategory) []ConditionalToolAvailabilityRule {
	var out []ConditionalToolAvailabilityRule
	for _, rule := range r.rules {
		if rule.Category == cat {
			out = append(out, rule)
		}
	}
	return out
}

// DefaultRules returns all rules for which IsDefault is true.  These are the
// rules whose tools appear in a default interactive session with no flags.
func (r *ConditionalToolAvailabilityRegistry) DefaultRules() []ConditionalToolAvailabilityRule {
	var out []ConditionalToolAvailabilityRule
	for _, rule := range r.rules {
		if rule.IsDefault {
			out = append(out, rule)
		}
	}
	return out
}

// UserGrantableRules returns all rules for which RequiresUserGrant is true.
// These are the rules where a user must explicitly opt-in for the tools to
// appear (e.g., by enabling a feature flag).
func (r *ConditionalToolAvailabilityRegistry) UserGrantableRules() []ConditionalToolAvailabilityRule {
	var out []ConditionalToolAvailabilityRule
	for _, rule := range r.rules {
		if rule.RequiresUserGrant {
			out = append(out, rule)
		}
	}
	return out
}

// OverridableRules returns all rules for which OverridableByUser is true.
func (r *ConditionalToolAvailabilityRegistry) OverridableRules() []ConditionalToolAvailabilityRule {
	var out []ConditionalToolAvailabilityRule
	for _, rule := range r.rules {
		if rule.OverridableByUser {
			out = append(out, rule)
		}
	}
	return out
}

// RulesAffectingTool returns all rules whose AffectedTools list contains
// toolSlug (exact string match).
func (r *ConditionalToolAvailabilityRegistry) RulesAffectingTool(toolSlug string) []ConditionalToolAvailabilityRule {
	var out []ConditionalToolAvailabilityRule
	for _, rule := range r.rules {
		for _, t := range rule.AffectedTools {
			if t == toolSlug {
				out = append(out, rule)
				break
			}
		}
	}
	return out
}

// AllToolSlugs returns a deduplicated, sorted slice of all tool names mentioned
// across all rules.
func (r *ConditionalToolAvailabilityRegistry) AllToolSlugs() []string {
	seen := make(map[string]struct{})
	for _, rule := range r.rules {
		for _, t := range rule.AffectedTools {
			seen[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	// deterministic order
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i] > out[j] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// StructuralInvariants validates the hard constraints encoded in Table 8 and
// §A.2.  Returns nil when all invariants hold, or a descriptive error
// otherwise.
func (r *ConditionalToolAvailabilityRegistry) StructuralInvariants() error {
	// Invariant 1: exactly four categories must be represented.
	catSet := make(map[ConditionalToolCategory]int)
	for _, rule := range r.rules {
		catSet[rule.Category]++
	}
	if len(catSet) != 4 {
		return fmt.Errorf("invariant FourCategories: expected 4 distinct categories, got %d", len(catSet))
	}
	for _, cat := range conditionalToolCategories {
		if catSet[cat] == 0 {
			return fmt.Errorf("invariant FourCategories: category %q has no rules", cat)
		}
	}

	// Invariant 2: AlwaysIncluded rules must not require a user grant (they are
	// unconditional by definition).
	for _, rule := range r.rules {
		if rule.Category == CategoryAlwaysIncluded && rule.RequiresUserGrant {
			return fmt.Errorf("invariant AlwaysIncludedNotUserGrantable: rule %q is always-included but RequiresUserGrant=true", rule.RuleID)
		}
	}

	// Invariant 3: every rule must have a non-empty ActivationCondition.
	for _, rule := range r.rules {
		if rule.ActivationCondition == "" {
			return fmt.Errorf("invariant AllRulesHaveActivationCondition: rule %q has empty ActivationCondition", rule.RuleID)
		}
	}

	// Invariant 4: FeatureFlag rules must require a user grant (flags are
	// opt-in by definition).
	for _, rule := range r.rules {
		if rule.Category == CategoryFeatureFlag && !rule.RequiresUserGrant {
			return fmt.Errorf("invariant FeatureFlagRulesRequireUserGrant: rule %q is feature-flag but RequiresUserGrant=false", rule.RuleID)
		}
	}

	// Invariant 5: AlwaysIncluded rules must be IsDefault=true (they are always
	// present, hence default).
	for _, rule := range r.rules {
		if rule.Category == CategoryAlwaysIncluded && !rule.IsDefault {
			return fmt.Errorf("invariant AlwaysIncludedIsDefault: rule %q is always-included but IsDefault=false", rule.RuleID)
		}
	}

	// Invariant 6: every rule must name at least one affected tool.
	for _, rule := range r.rules {
		if len(rule.AffectedTools) == 0 {
			return fmt.Errorf("invariant AllRulesHaveAffectedTools: rule %q has no AffectedTools", rule.RuleID)
		}
	}

	return nil
}
