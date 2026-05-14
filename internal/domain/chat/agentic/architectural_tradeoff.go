package agentic

// architectural_tradeoff.go — §11.3 Architectural Trade-offs + §11.7 Recurring Design Choices
//
// FEAT-023 — arXiv:2604.14228v1
//
// §11.3 identifies three concrete architectural trade-offs where value tensions (§11.2 / Table 4)
// manifest as measurable design costs:
//   1. safety_vs_autonomy     — permission-mode gradient erodes human oversight quality
//   2. context_efficiency_vs_transparency — five-layer compaction is largely invisible to the user
//   3. simplicity_vs_extensibility — four extension mechanisms create combinatorial interaction hazards
//
// §11.7 "Recurring Design Choices" surfaces three cross-cutting commitments that recur across
// all six subsystems (permissions, context management, extensibility, memory, subagents, sessions):
//   1. graduated_layering         — stacks of independent mechanisms, never a single integrated solution
//   2. append_only_auditability   — append-only state favoring auditability over query power
//   3. model_judgment_in_harness  — 1.6% decision logic / 98.4% deterministic harness ratio

// ArchitecturalTradeoffKind distinguishes §11.3 trade-offs from §11.7 recurring choices.
type ArchitecturalTradeoffKind string

const (
	// KindConcreteTradeoff — §11.3: a value tension that manifests as a measurable design cost.
	KindConcreteTradeoff ArchitecturalTradeoffKind = "concrete_tradeoff"
	// KindRecurringChoice — §11.7: a cross-cutting commitment that appears in every subsystem.
	KindRecurringChoice ArchitecturalTradeoffKind = "recurring_choice"
)

// ArchitecturalTradeoffID is a slug identifier for one of the six profiles.
type ArchitecturalTradeoffID string

const (
	// §11.3 trade-offs
	TradeoffSafetyVsAutonomy            ArchitecturalTradeoffID = "safety_vs_autonomy"
	TradeoffContextEfficiencyVsTransparency ArchitecturalTradeoffID = "context_efficiency_vs_transparency"
	TradeoffSimplicityVsExtensibility   ArchitecturalTradeoffID = "simplicity_vs_extensibility"

	// §11.7 recurring choices
	ChoiceGraduatedLayering        ArchitecturalTradeoffID = "graduated_layering"
	ChoiceAppendOnlyAuditability   ArchitecturalTradeoffID = "append_only_auditability"
	ChoiceModelJudgmentInHarness   ArchitecturalTradeoffID = "model_judgment_in_harness"
)

// SeedArchitecturalTradeoffCount is the total number of profiles pre-loaded in the registry.
const SeedArchitecturalTradeoffCount = 6

// SeedArchitecturalTradeoffSlugs lists all six profile IDs in canonical order.
var SeedArchitecturalTradeoffSlugs = []ArchitecturalTradeoffID{
	TradeoffSafetyVsAutonomy,
	TradeoffContextEfficiencyVsTransparency,
	TradeoffSimplicityVsExtensibility,
	ChoiceGraduatedLayering,
	ChoiceAppendOnlyAuditability,
	ChoiceModelJudgmentInHarness,
}

// ArchitecturalTradeoffProfile holds the structured data for one §11.3 or §11.7 entry.
type ArchitecturalTradeoffProfile struct {
	// ID is the canonical slug.
	ID ArchitecturalTradeoffID

	// Kind distinguishes §11.3 trade-offs from §11.7 recurring choices.
	Kind ArchitecturalTradeoffKind

	// Label is the short human-readable name used in the paper.
	Label string

	// PDFSection is the source section ("11.3" or "11.7").
	PDFSection string

	// ValuesInTension lists the design values (§2.1) whose conflict this entry captures.
	// For §11.7 recurring choices the values reflect the dominant design value served.
	ValuesInTension []DesignValue

	// ConsequenceDescription describes the measured or observed architectural cost or benefit.
	ConsequenceDescription string

	// SubsystemsAffected names the subsystems (§3–§9) where this trade-off or choice is visible.
	SubsystemsAffected []string

	// EvidenceSummary cites the empirical data or code observations supporting the entry.
	EvidenceSummary string
}

// architecturalTradeoffProfiles is the canonical data source for all six profiles.
var architecturalTradeoffProfiles = []ArchitecturalTradeoffProfile{
	{
		ID:         TradeoffSafetyVsAutonomy,
		Kind:       KindConcreteTradeoff,
		Label:      "Safety vs. autonomy",
		PDFSection: "11.3",
		ValuesInTension: []DesignValue{
			DesignValueSafety,
			DesignValueCapability,
			DesignValueHumanAuthority,
		},
		ConsequenceDescription: "The permission-mode gradient (plan → default → acceptEdits → auto → bypassPermissions) monotonically " +
			"decreases safety with increasing autonomy. Auto-approve rates rise from ~20% at <50 sessions to >40% " +
			"at 750 sessions via habituation, not deliberate mode selection — safety state is not restored on resume, " +
			"which is a deliberate architectural choice to err toward safety across session boundaries.",
		SubsystemsAffected: []string{"permissions", "sessions"},
		EvidenceSummary: "93% approval fatigue rate (Hughes 2026); sandboxing reduces prompt frequency by 84% " +
			"(Dworken & Weller-Davies 2025); >50 subcommands fall back to generic approval due to parsing overhead " +
			"(Adversa.ai 2026); defense-in-depth independence assumption violated when layers share common performance " +
			"and economic constraints.",
	},
	{
		ID:         TradeoffContextEfficiencyVsTransparency,
		Kind:       KindConcreteTradeoff,
		Label:      "Context efficiency vs. transparency",
		PDFSection: "11.3",
		ValuesInTension: []DesignValue{
			DesignValueCapability,
			DesignValueReliability,
			DesignValueHumanAuthority,
		},
		ConsequenceDescription: "The five-layer compaction pipeline achieves effective context management but is largely " +
			"invisible to the user. When budget reduction replaces a long tool output with a reference, or context " +
			"collapse substitutes messages with a summary ('read-time projection over the REPL full history'), or snip " +
			"trims older history, there is no easy way to inspect what was lost. Cache-aware microcompact adds further " +
			"opacity because compression decisions are influenced by prompt caching not visible to the user.",
		SubsystemsAffected: []string{"context_management", "compaction_pipeline"},
		EvidenceSummary: "Five-layer compaction pipeline (§7.3); cache-aware compression (§7.3 microcompact shaper); " +
			"read-time projection retains full history for reconstruction while presenting compressed view (§7.3); " +
			"no per-session user-visible compression log surfaced in source analysis.",
	},
	{
		ID:         TradeoffSimplicityVsExtensibility,
		Kind:       KindConcreteTradeoff,
		Label:      "Simplicity vs. extensibility",
		PDFSection: "11.3",
		ValuesInTension: []DesignValue{
			DesignValueAdaptability,
			DesignValueReliability,
			DesignValueSafety,
		},
		ConsequenceDescription: "The four extension mechanisms (MCP servers, plugins, skills, hooks) enable rich customization " +
			"but create combinatorial interactions. A plugin contributes a PreToolUse hook that modifies tool inputs. " +
			"The auto-mode classifier reads cached CLAUDE.md content. Path-scoped rules load lazily when new directories " +
			"are read, potentially changing classifier behavior mid-conversation. The permission handler interacts with " +
			"the hook pipeline at multiple points. These cross-cutting concerns create emergent behaviors difficult to " +
			"predict from any single configuration file. Extensibility also creates attack surface not only through " +
			"combinatorial complexity but through initialization ordering (pre-trust execution window).",
		SubsystemsAffected: []string{"extensibility", "permissions", "hooks"},
		EvidenceSummary: "Four extension mechanisms at different context costs (§6); pre-trust initialization ordering " +
			"CVE-2025-59536 (CVSS 8.7) and CVE-2026-21852 (CVSS 5.3) (Donenfeld & Vanunu 2026); CVE-2025-54794 " +
			"and CVE-2025-54795 exploit path validation and command parsing (Beber 2025).",
	},
	{
		ID:         ChoiceGraduatedLayering,
		Kind:       KindRecurringChoice,
		Label:      "Graduated layering over monolithic mechanisms",
		PDFSection: "11.7",
		ValuesInTension: []DesignValue{
			DesignValueSafety,
			DesignValueReliability,
		},
		ConsequenceDescription: "Safety, context management, and extensibility all use graduated stacks of independent " +
			"mechanisms rather than single integrated solutions. The permission architecture layers seven stages from " +
			"tool pre-filtering through deny-first rules, permission modes, the auto-mode classifier, shell sandboxing, " +
			"non-restoration on resume, and hook interception. Context management layers five compaction stages, " +
			"lazy-loaded CLAUDE.md files, deferred tool schemas, and summary-only subagent returns. Extensibility " +
			"layers four mechanisms (MCP servers, plugins, skills, hooks) at different context costs. The design " +
			"trades simplicity and debuggability for defense in depth, accepting that layer interactions can produce " +
			"emergent behaviors difficult to predict from any single configuration.",
		SubsystemsAffected: []string{"permissions", "context_management", "extensibility"},
		EvidenceSummary: "Seven-stage permission pipeline (§5); five-layer compaction (§7.3); four-mechanism extensibility " +
			"at different context costs (§6); independence assumption in defense-in-depth (§11.3).",
	},
	{
		ID:         ChoiceAppendOnlyAuditability,
		Kind:       KindRecurringChoice,
		Label:      "Append-only designs favoring auditability over query power",
		PDFSection: "11.7",
		ValuesInTension: []DesignValue{
			DesignValueReliability,
			DesignValueHumanAuthority,
		},
		ConsequenceDescription: "Session transcripts are append-only JSONL files with read-time chain patching. Permissions " +
			"are not restored across session boundaries. Context compaction applies read-time projections over a full " +
			"history rather than destructive edits. This commitment recurs because it preserves the ability to resume, " +
			"fork, and audit sessions without modifying previously written state. The cost is that richer structured " +
			"queries ('show me all tool calls that modified file X across sessions') require post-hoc reconstruction " +
			"rather than direct lookup.",
		SubsystemsAffected: []string{"sessions", "context_management", "permissions"},
		EvidenceSummary: "Append-only JSONL session transcripts (§9); read-time chain patching (§9); permissions non-" +
			"restoration on resume as deliberate choice (§9, §11.3); context compaction read-time projection (§7.3).",
	},
	{
		ID:         ChoiceModelJudgmentInHarness,
		Kind:       KindRecurringChoice,
		Label:      "Model judgment within a deterministic harness",
		PDFSection: "11.7",
		ValuesInTension: []DesignValue{
			DesignValueCapability,
			DesignValueReliability,
		},
		ConsequenceDescription: "Across all subsystems the architecture trusts the model's judgment within a rich deterministic " +
			"harness rather than constraining its choices. The estimated 1.6% decision-logic ratio (98.4% operational " +
			"harness) captures this quantitatively: the harness creates conditions (tool routing, permission enforcement, " +
			"context assembly, recovery logic) under which the model can decide well. Hierarchical permissions preserve " +
			"safety invariants across agent boundaries, and assembleToolPool() merges built-in and MCP tools into a " +
			"single unified interface, but the model retains full latitude over which tools to invoke and in what order. " +
			"The trade-off is that good local decisions can produce poor global outcomes when bounded context prevents " +
			"global awareness.",
		SubsystemsAffected: []string{"loop_architecture", "permissions", "tool_routing", "subagents"},
		EvidenceSummary: "1.6% decision logic / 98.4% operational harness ratio (§11.1); assembleToolPool() unified " +
			"interface (§6.2, Appendix A); subagent isolation limits cross-agent consistency (§8); empirical predictions " +
			"in §11.4 (pattern duplication, convention violation, complexity increase).",
	},
}

// ArchitecturalTradeoffRegistry provides structured access to the six §11.3 and §11.7 profiles.
type ArchitecturalTradeoffRegistry struct {
	profiles []ArchitecturalTradeoffProfile
	index    map[ArchitecturalTradeoffID]*ArchitecturalTradeoffProfile
}

// NewArchitecturalTradeoffRegistry constructs a registry pre-loaded with all six profiles.
func NewArchitecturalTradeoffRegistry() *ArchitecturalTradeoffRegistry {
	r := &ArchitecturalTradeoffRegistry{
		profiles: make([]ArchitecturalTradeoffProfile, len(architecturalTradeoffProfiles)),
		index:    make(map[ArchitecturalTradeoffID]*ArchitecturalTradeoffProfile, len(architecturalTradeoffProfiles)),
	}
	copy(r.profiles, architecturalTradeoffProfiles)
	for i := range r.profiles {
		r.index[r.profiles[i].ID] = &r.profiles[i]
	}
	return r
}

// FindArchitecturalTradeoffBySlug returns the profile for the given slug ID.
// Returns (profile, true) on success, (zero, false) if the slug is unknown.
func (r *ArchitecturalTradeoffRegistry) FindArchitecturalTradeoffBySlug(id ArchitecturalTradeoffID) (*ArchitecturalTradeoffProfile, bool) {
	p, ok := r.index[id]
	return p, ok
}

// AllProfiles returns all six profiles in canonical order.
func (r *ArchitecturalTradeoffRegistry) AllProfiles() []ArchitecturalTradeoffProfile {
	out := make([]ArchitecturalTradeoffProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// ByKind returns profiles filtered by ArchitecturalTradeoffKind.
func (r *ArchitecturalTradeoffRegistry) ByKind(kind ArchitecturalTradeoffKind) []ArchitecturalTradeoffProfile {
	var result []ArchitecturalTradeoffProfile
	for i := range r.profiles {
		if r.profiles[i].Kind == kind {
			result = append(result, r.profiles[i])
		}
	}
	return result
}

// ConcreteTradeoffs returns the three §11.3 trade-off profiles.
func (r *ArchitecturalTradeoffRegistry) ConcreteTradeoffs() []ArchitecturalTradeoffProfile {
	return r.ByKind(KindConcreteTradeoff)
}

// RecurringChoices returns the three §11.7 recurring design choice profiles.
func (r *ArchitecturalTradeoffRegistry) RecurringChoices() []ArchitecturalTradeoffProfile {
	return r.ByKind(KindRecurringChoice)
}

// InvolvingValue returns all profiles where value appears in ValuesInTension.
func (r *ArchitecturalTradeoffRegistry) InvolvingValue(v DesignValue) []ArchitecturalTradeoffProfile {
	var result []ArchitecturalTradeoffProfile
	for i := range r.profiles {
		for _, tv := range r.profiles[i].ValuesInTension {
			if tv == v {
				result = append(result, r.profiles[i])
				break
			}
		}
	}
	return result
}

// AffectingSubsystem returns all profiles where subsystem appears in SubsystemsAffected.
func (r *ArchitecturalTradeoffRegistry) AffectingSubsystem(subsystem string) []ArchitecturalTradeoffProfile {
	var result []ArchitecturalTradeoffProfile
	for i := range r.profiles {
		for _, s := range r.profiles[i].SubsystemsAffected {
			if s == subsystem {
				result = append(result, r.profiles[i])
				break
			}
		}
	}
	return result
}

// IsValidSlug reports whether id is one of the six registered profile IDs.
func (r *ArchitecturalTradeoffRegistry) IsValidSlug(id ArchitecturalTradeoffID) bool {
	_, ok := r.index[id]
	return ok
}
