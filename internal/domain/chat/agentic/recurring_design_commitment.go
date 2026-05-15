package agentic

// recurring_design_commitment.go — §11.7 Recurring Design Choices
//
// FEAT-028 — arXiv:2604.14228v1
//
// §11.7 "Recurring Design Choices" (page 32) reads six subsystem analyses together
// and surfaces three cross-cutting commitments that recur across otherwise
// independent components:
//
//   1. graduated_layering          — stacks of independent mechanisms, never a
//      single integrated solution (permissions, context, extensibility all use
//      N-layer stacks not monolithic mechanisms).
//
//   2. append_only_auditability    — append-only state that favors auditability
//      over query power (transcripts, permission state, compaction projections).
//
//   3. model_judgment_in_harness   — model judgment within a deterministic harness;
//      1.6% decision-logic ratio / 98.4% operational harness ratio.
//
// This file models these three commitments as a standalone registry with richer
// fields than the combined §11.3+§11.7 ArchitecturalTradeoffRegistry (FEAT-023).
// The additional fields capture: CommitmentType, Manifestations (per-subsystem
// evidence), TradeoffAccepted (what is sacrificed), DesignValues (served), and
// IsStructural (baked into architecture vs. enforced by policy).
//
// The registry is pure Go, no DB, no HTTP.

// RecurringDesignCommitmentSlug is a stable slug for one of the three §11.7 commitments.
type RecurringDesignCommitmentSlug string

const (
	// CommitGraduatedLayering — safety, context management, and extensibility all
	// use graduated stacks of independent mechanisms rather than integrated solutions.
	CommitGraduatedLayering RecurringDesignCommitmentSlug = "graduated_layering"

	// CommitAppendOnlyAuditability — append-only state favoring auditability over
	// mutable query power (session transcripts, permission non-restoration, read-time
	// compaction projections).
	CommitAppendOnlyAuditability RecurringDesignCommitmentSlug = "append_only_auditability"

	// CommitModelJudgmentInHarness — model judgment within a rich deterministic
	// harness; 1.6% decision logic / 98.4% operational harness ratio.
	CommitModelJudgmentInHarness RecurringDesignCommitmentSlug = "model_judgment_in_harness"
)

// SeedRecurringDesignCommitmentCount is the total number of §11.7 commitments.
const SeedRecurringDesignCommitmentCount = 3

// SeedRecurringDesignCommitmentSlugs lists the three commitment slugs in §11.7
// canonical order (matches the prose sequence in the paper).
var SeedRecurringDesignCommitmentSlugs = []RecurringDesignCommitmentSlug{
	CommitGraduatedLayering,
	CommitAppendOnlyAuditability,
	CommitModelJudgmentInHarness,
}

// CommitmentType classifies the structural nature of a recurring design commitment.
type CommitmentType string

const (
	// CommitmentTypeLayering — the commitment takes the form of stacking independent
	// mechanisms rather than building a single integrated solution.
	CommitmentTypeLayering CommitmentType = "layering"

	// CommitmentTypeAuditability — the commitment takes the form of favoring
	// append-only, human-inspectable state over mutable, query-optimized state.
	CommitmentTypeAuditability CommitmentType = "auditability"

	// CommitmentTypeJudgmentDelegation — the commitment takes the form of delegating
	// judgment decisions to the model within a deterministic operational harness.
	CommitmentTypeJudgmentDelegation CommitmentType = "judgment_delegation"
)

// RecurringDesignCommitmentProfile is the full structured profile for one §11.7
// commitment.  Each field is directly sourced from the paper text.
type RecurringDesignCommitmentProfile struct {
	// Slug is the stable identifier for this commitment.
	Slug RecurringDesignCommitmentSlug

	// Label is the human-readable name used in the paper.
	Label string

	// PDFSection is the source section in arXiv:2604.14228v1 (always "11.7").
	PDFSection string

	// CommitmentType classifies the structural form of this commitment.
	CommitmentType CommitmentType

	// Manifestations lists per-subsystem evidence of the commitment in the
	// architecture.  Each entry names where in §3–§9 the commitment is visible.
	Manifestations []string

	// TradeoffAccepted describes what is explicitly sacrificed to honour this
	// commitment.  Drawn directly from the paper's consequence paragraphs.
	TradeoffAccepted string

	// DesignValues lists the §2.1 design values that this commitment primarily
	// serves.  Expressed as DesignValue constants.
	DesignValues []string

	// IsStructural is true when the commitment is baked into the architecture
	// (e.g. data-format decisions, layer ordering) rather than enforced only
	// by policy or configuration.
	IsStructural bool
}

// recurringDesignCommitmentProfiles is the canonical ordered data for §11.7.
var recurringDesignCommitmentProfiles = []RecurringDesignCommitmentProfile{
	{
		Slug:           CommitGraduatedLayering,
		Label:          "Graduated layering over monolithic mechanisms",
		PDFSection:     "11.7",
		CommitmentType: CommitmentTypeLayering,
		Manifestations: []string{
			"permissions: seven-stage pipeline (tool pre-filter → deny-first rules → " +
				"permission modes → auto-mode ML classifier → shell sandboxing → " +
				"non-restoration on session resume → hook interception)",
			"context_management: five-layer compaction pipeline (budget-reduction → " +
				"snip → microcompact → compact → full-compact) plus lazy-loaded " +
				"CLAUDE.md files and deferred tool schemas",
			"extensibility: four mechanisms at graduated context costs — MCP servers " +
				"(high), plugins (medium), skills (low), hooks (zero by default)",
			"subagents: graduated delegation (built-in types → general-purpose → " +
				"custom agents) with per-level context and tool isolation",
		},
		TradeoffAccepted: "Simplicity and debuggability are sacrificed for defense in depth. " +
			"Layer interactions can produce emergent behaviors that are difficult to predict " +
			"from any single configuration file or layer in isolation.",
		DesignValues: []string{
			string(DesignValueSafety),
			string(DesignValueReliability),
		},
		IsStructural: true,
	},
	{
		Slug:           CommitAppendOnlyAuditability,
		Label:          "Append-only designs that favor auditability over query power",
		PDFSection:     "11.7",
		CommitmentType: CommitmentTypeAuditability,
		Manifestations: []string{
			"sessions: transcripts are append-only JSONL files; the only mutation " +
				"allowed is read-time chain patching for fork/resume (§9)",
			"permissions: permission state is not restored across session boundaries; " +
				"the deliberate choice errs toward safety rather than continuity (§9, §11.3)",
			"context_management: compaction applies read-time projections over the full " +
				"history rather than destructive edits — the full history is retained " +
				"for reconstruction (§7.3)",
			"memory: CLAUDE.md hierarchy is file-based and human-readable; no opaque " +
				"vector-only store is the primary memory substrate (§7.1)",
		},
		TradeoffAccepted: "Richer structured queries (e.g. 'show all tool calls that modified " +
			"file X across sessions') require post-hoc reconstruction from the append-only " +
			"log rather than direct lookup.  Query power and cross-session aggregation are " +
			"secondary concerns to resumability and auditability.",
		DesignValues: []string{
			string(DesignValueReliability),
			string(DesignValueHumanAuthority),
		},
		IsStructural: true,
	},
	{
		Slug:           CommitModelJudgmentInHarness,
		Label:          "Model judgment within a deterministic harness",
		PDFSection:     "11.7",
		CommitmentType: CommitmentTypeJudgmentDelegation,
		Manifestations: []string{
			"loop_architecture: estimated 1.6% decision-logic ratio / 98.4% operational " +
				"harness ratio — the harness creates conditions under which the model can " +
				"decide well rather than constraining which decisions it may make (§11.1)",
			"tool_routing: assembleToolPool() merges built-in and MCP tools into a single " +
				"unified interface; the model retains full latitude over which tools to " +
				"invoke and in what order (§6.2, Appendix A)",
			"permissions: hierarchical permission settings preserve safety invariants " +
				"across agent boundaries without constraining model tool selection within " +
				"those boundaries (§5)",
			"subagents: model decides when to delegate to subagents, what task description " +
				"to pass, and when to synthesize results — the harness provides isolation " +
				"and summary-only return, not decision scripts (§8)",
		},
		TradeoffAccepted: "Good local decisions by the model can produce poor global outcomes " +
			"when bounded context prevents global awareness. Subagent isolation means " +
			"parallel agents can independently re-implement solutions already existing " +
			"elsewhere; the architecture accepts this as an empirical prediction (§11.4).",
		DesignValues: []string{
			string(DesignValueCapability),
			string(DesignValueReliability),
		},
		IsStructural: true,
	},
}

// RecurringDesignCommitmentRegistry provides structured access to the three
// §11.7 recurring design commitment profiles.
type RecurringDesignCommitmentRegistry struct {
	profiles []RecurringDesignCommitmentProfile
	index    map[RecurringDesignCommitmentSlug]*RecurringDesignCommitmentProfile
}

// NewRecurringDesignCommitmentRegistry constructs a registry pre-loaded with
// the three §11.7 commitment profiles in canonical order.
func NewRecurringDesignCommitmentRegistry() *RecurringDesignCommitmentRegistry {
	r := &RecurringDesignCommitmentRegistry{
		profiles: make([]RecurringDesignCommitmentProfile, len(recurringDesignCommitmentProfiles)),
		index:    make(map[RecurringDesignCommitmentSlug]*RecurringDesignCommitmentProfile, len(recurringDesignCommitmentProfiles)),
	}
	copy(r.profiles, recurringDesignCommitmentProfiles)
	for i := range r.profiles {
		r.index[r.profiles[i].Slug] = &r.profiles[i]
	}
	return r
}

// FindRecurringDesignCommitmentBySlug returns the profile for the given slug.
// Returns (profile, true) on success and (nil, false) when the slug is unknown.
func (r *RecurringDesignCommitmentRegistry) FindRecurringDesignCommitmentBySlug(
	slug RecurringDesignCommitmentSlug,
) (*RecurringDesignCommitmentProfile, bool) {
	p, ok := r.index[slug]
	return p, ok
}

// AllCommitments returns all three profiles in canonical §11.7 order.
func (r *RecurringDesignCommitmentRegistry) AllCommitments() []RecurringDesignCommitmentProfile {
	out := make([]RecurringDesignCommitmentProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// ByCommitmentType returns all profiles whose CommitmentType matches the given type.
func (r *RecurringDesignCommitmentRegistry) ByCommitmentType(
	ct CommitmentType,
) []RecurringDesignCommitmentProfile {
	var result []RecurringDesignCommitmentProfile
	for i := range r.profiles {
		if r.profiles[i].CommitmentType == ct {
			result = append(result, r.profiles[i])
		}
	}
	return result
}

// StructuralCommitments returns all profiles where IsStructural is true —
// i.e. commitments baked into the architecture rather than enforced by policy.
func (r *RecurringDesignCommitmentRegistry) StructuralCommitments() []RecurringDesignCommitmentProfile {
	var result []RecurringDesignCommitmentProfile
	for i := range r.profiles {
		if r.profiles[i].IsStructural {
			result = append(result, r.profiles[i])
		}
	}
	return result
}

// ServingDesignValue returns all profiles that list the given design value
// (as a string matching a DesignValue constant) in their DesignValues slice.
func (r *RecurringDesignCommitmentRegistry) ServingDesignValue(
	v DesignValue,
) []RecurringDesignCommitmentProfile {
	target := string(v)
	var result []RecurringDesignCommitmentProfile
	for i := range r.profiles {
		for _, dv := range r.profiles[i].DesignValues {
			if dv == target {
				result = append(result, r.profiles[i])
				break
			}
		}
	}
	return result
}

// ManifestationCount returns the total number of per-subsystem manifestation
// entries across all three commitments.
func (r *RecurringDesignCommitmentRegistry) ManifestationCount() int {
	total := 0
	for i := range r.profiles {
		total += len(r.profiles[i].Manifestations)
	}
	return total
}

// IsValidSlug reports whether slug is one of the three registered commitment slugs.
func (r *RecurringDesignCommitmentRegistry) IsValidSlug(slug RecurringDesignCommitmentSlug) bool {
	_, ok := r.index[slug]
	return ok
}
