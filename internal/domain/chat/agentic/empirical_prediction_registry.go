package agentic

// empirical_prediction_registry.go — FEAT-043 — §11.4 Empirical Predictions and Early Signals
//
// arXiv:2604.14228v1, §11.4 "Empirical Predictions and Early Signals" (pages 30–31).
//
// The architectural properties documented in the paper generate testable predictions
// about code-quality outcomes that are not derivable from source-code analysis alone.
// §11.4 states three first-order predictions arising directly from the architecture,
// each accompanied by published empirical signals from adjacent tools:
//
//   1. PatternDuplication   — bounded context → higher pattern duplication & convention
//      violations than code with full-codebase visibility (§7 compaction pipeline)
//
//   2. SubagentRedundancy   — subagent isolation (§8) compounds the effect: parallel
//      agents can independently re-implement solutions already existing elsewhere
//
//   3. LocalGoodGlobalBad   — §11.1 design philosophy trusts local model decisions,
//      but good local decisions can produce poor global outcomes when context is bounded
//
// Published early signals (adjacent tools, not Claude Code specifically):
//
//   - He et al. (2025): +40.7% code-complexity increase across 807 Cursor repos
//     (initial velocity spike +281%, dissipated to baseline by month 3)
//   - Liu et al. (2026): 304,000 AI-authored commits / 6,275 repos; ~25% of
//     AI-introduced issues persist to the latest revision at higher rates
//
// The context-management pipeline's mitigations (graduated compression, cache-aware
// compaction, read-time projection, subagent summary isolation) are documented as
// architectural responses, but whether they close the gap is explicitly left open as
// "a directly measurable empirical question that source-level analysis cannot resolve."

// PredictionID is a stable slug for one of the §11.4 architectural predictions.
type PredictionID string

const (
	// PredictionPatternDuplication — bounded context causes higher rates of pattern
	// duplication and convention violation than full-codebase-visibility code.
	// Architectural root: §7 five-layer compaction pipeline introduces lossy compression.
	PredictionPatternDuplication PredictionID = "pattern_duplication"

	// PredictionSubagentRedundancy — subagent isolation compounds the duplication effect:
	// parallel agents independently re-implement solutions that exist elsewhere.
	// Architectural root: §8 subagent isolation (independent context + tool pool per agent).
	PredictionSubagentRedundancy PredictionID = "subagent_redundancy"

	// PredictionLocalGoodGlobalBad — model makes good local decisions but bounded context
	// prevents global awareness, producing poor global outcomes.
	// Architectural root: §11.1 design philosophy (model autonomy within harness).
	PredictionLocalGoodGlobalBad PredictionID = "local_good_global_bad"
)

// PredictionCategory classifies the output dimension that an architectural prediction
// targets.
type PredictionCategory string

const (
	// PredictionCategoryCodeQuality — the prediction targets observable code-quality metrics
	// (complexity, duplication, convention adherence, technical debt).
	PredictionCategoryCodeQuality PredictionCategory = "code_quality"

	// PredictionCategoryCoordination — the prediction targets coordination failures between
	// concurrent or sequential agents operating with isolated contexts.
	PredictionCategoryCoordination PredictionCategory = "coordination"

	// PredictionCategorySystemCoherence — the prediction targets system-level coherence:
	// the gap between locally-correct decisions and globally consistent outcomes.
	PredictionCategorySystemCoherence PredictionCategory = "system_coherence"
)

// EmpiricalSignal records one published empirical data point that supports or is
// consistent with an architectural prediction.  All signals in this registry are from
// adjacent tools (not Claude Code specifically), as acknowledged in §11.4.
type EmpiricalSignal struct {
	// Citation is the author-year reference as it appears in §11.4.
	Citation string

	// Finding is the key quantitative or qualitative result from the study.
	Finding string

	// DataScope describes the dataset or experimental context of the signal.
	DataScope string

	// IsDirectClaudeCodeEvidence is false when the study targets adjacent tools
	// (Cursor, AI-authored commits broadly); true would mean Claude Code itself
	// was the subject.  §11.4 explicitly notes all listed signals are from adjacent tools.
	IsDirectClaudeCodeEvidence bool
}

// ArchitecturalMitigation names one mechanism the architecture provides as a
// response to the predicted effect.  These are documented in §11.4's closing
// paragraph on the compaction pipeline.
type ArchitecturalMitigation struct {
	// Slug is a stable lowercase identifier for this mitigation.
	Slug string

	// Description explains what the mitigation does and how it addresses the
	// predicted negative effect.
	Description string

	// PDFSection gives the cross-reference within the paper.
	PDFSection string
}

// EmpiricalPredictionProfile is the full descriptor for one §11.4 architectural
// prediction.
type EmpiricalPredictionProfile struct {
	// ID is the stable prediction identifier.
	ID PredictionID

	// Title is the short human-readable name for the prediction.
	Title string

	// PDFSection is the section reference in arXiv:2604.14228v1.
	PDFSection string

	// Category classifies the output dimension the prediction targets.
	Category PredictionCategory

	// ArchitecturalRoot identifies the subsection(s) whose design choice generates this
	// prediction.
	ArchitecturalRoot string

	// Prediction is the precisely stated testable claim from §11.4.
	Prediction string

	// CausalChain describes the step-by-step mechanism from architectural choice to
	// predicted outcome.
	CausalChain string

	// EarlySignals lists published empirical signals consistent with this prediction.
	EarlySignals []EmpiricalSignal

	// Mitigations lists the architectural responses that §11.4 documents as partially
	// addressing the predicted effect.
	Mitigations []ArchitecturalMitigation

	// IsDirectlyMeasurable is true when §11.4 explicitly calls the question
	// "a directly measurable empirical question."
	IsDirectlyMeasurable bool

	// MitigationClaimedSufficient is false when the paper explicitly leaves open whether
	// the architectural mitigations are sufficient to overcome the structural limitation.
	// §11.4 marks all three predictions as unresolved by source-level analysis.
	MitigationClaimedSufficient bool
}

// EmpiricalPredictionRegistry is an in-memory registry of the three §11.4
// architectural predictions and their associated empirical signals.
type EmpiricalPredictionRegistry struct {
	profiles []EmpiricalPredictionProfile
	index    map[PredictionID]*EmpiricalPredictionProfile
}

// NewEmpiricalPredictionRegistry constructs the registry pre-seeded with all three
// §11.4 predictions in the order they appear in the paper.
func NewEmpiricalPredictionRegistry() *EmpiricalPredictionRegistry {
	profiles := seedEmpiricalPredictionProfiles()
	r := &EmpiricalPredictionRegistry{
		profiles: profiles,
		index:    make(map[PredictionID]*EmpiricalPredictionProfile, len(profiles)),
	}
	for i := range r.profiles {
		r.index[r.profiles[i].ID] = &r.profiles[i]
	}
	return r
}

// FindByID returns the profile for the given PredictionID, or (nil, false) when not found.
func (r *EmpiricalPredictionRegistry) FindByID(id PredictionID) (*EmpiricalPredictionProfile, bool) {
	p, ok := r.index[id]
	return p, ok
}

// AllPredictions returns all three profiles in §11.4 prose order.
func (r *EmpiricalPredictionRegistry) AllPredictions() []EmpiricalPredictionProfile {
	out := make([]EmpiricalPredictionProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// Count returns the number of registered predictions.
func (r *EmpiricalPredictionRegistry) Count() int {
	return len(r.profiles)
}

// ByCategory returns all profiles whose Category matches the given value.
func (r *EmpiricalPredictionRegistry) ByCategory(cat PredictionCategory) []EmpiricalPredictionProfile {
	var out []EmpiricalPredictionProfile
	for _, p := range r.profiles {
		if p.Category == cat {
			out = append(out, p)
		}
	}
	return out
}

// DirectlyMeasurable returns predictions that §11.4 explicitly marks as "directly
// measurable empirical questions."
func (r *EmpiricalPredictionRegistry) DirectlyMeasurable() []EmpiricalPredictionProfile {
	var out []EmpiricalPredictionProfile
	for _, p := range r.profiles {
		if p.IsDirectlyMeasurable {
			out = append(out, p)
		}
	}
	return out
}

// WithEarlySignals returns profiles that have at least one published empirical signal.
func (r *EmpiricalPredictionRegistry) WithEarlySignals() []EmpiricalPredictionProfile {
	var out []EmpiricalPredictionProfile
	for _, p := range r.profiles {
		if len(p.EarlySignals) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// OpenQuestions returns predictions whose mitigation sufficiency is explicitly left
// unresolved by the paper (MitigationClaimedSufficient == false).
func (r *EmpiricalPredictionRegistry) OpenQuestions() []EmpiricalPredictionProfile {
	var out []EmpiricalPredictionProfile
	for _, p := range r.profiles {
		if !p.MitigationClaimedSufficient {
			out = append(out, p)
		}
	}
	return out
}

// TotalEarlySignalCount returns the sum of EarlySignal entries across all profiles.
func (r *EmpiricalPredictionRegistry) TotalEarlySignalCount() int {
	total := 0
	for _, p := range r.profiles {
		total += len(p.EarlySignals)
	}
	return total
}

// TotalMitigationCount returns the sum of Mitigation entries across all profiles.
func (r *EmpiricalPredictionRegistry) TotalMitigationCount() int {
	total := 0
	for _, p := range r.profiles {
		total += len(p.Mitigations)
	}
	return total
}

// IsValidPredictionID reports whether id names a registered prediction.
func (r *EmpiricalPredictionRegistry) IsValidPredictionID(id PredictionID) bool {
	_, ok := r.index[id]
	return ok
}

// SeedEmpiricalPredictionCount is the number of §11.4 architectural predictions.
const SeedEmpiricalPredictionCount = 3

// SeedEmpiricalPredictionIDs lists all prediction IDs in §11.4 prose order.
var SeedEmpiricalPredictionIDs = []PredictionID{
	PredictionPatternDuplication,
	PredictionSubagentRedundancy,
	PredictionLocalGoodGlobalBad,
}

// seedEmpiricalPredictionProfiles returns the canonical data for §11.4.
func seedEmpiricalPredictionProfiles() []EmpiricalPredictionProfile {
	return []EmpiricalPredictionProfile{
		{
			ID:         PredictionPatternDuplication,
			Title:      "Pattern Duplication and Convention Violation from Bounded Context",
			PDFSection: "11.4",
			Category:   PredictionCategoryCodeQuality,
			ArchitecturalRoot: "§7 five-layer compaction pipeline (budget-reduction, snip, " +
				"micro-compact, compact, full-compact) introduces lossy compression at each stage",
			Prediction: "Agent-generated code will exhibit higher rates of pattern duplication " +
				"and convention violation than code produced with full codebase visibility, because " +
				"the five-layer compaction pipeline preserves useful information but introduces lossy " +
				"compression at each stage.",
			CausalChain: "Bounded context window (§7) → five-layer compaction introduces lossy " +
				"compression → agent cannot maintain simultaneous awareness of the full codebase → " +
				"pattern duplication and convention violations emerge in generated code.",
			EarlySignals: []EmpiricalSignal{
				{
					Citation: "He et al. (2025)",
					Finding: "Statistically significant +40.7% code-complexity increase (p < 0.001) " +
						"across 807 Cursor repositories; initial velocity spike +281% in month 1, " +
						"dissipated to baseline by month 3; rising complexity associated with " +
						"proportional decrease in future development velocity (self-cancelling gains).",
					DataScope:                  "807 Cursor-adopting repositories; causal analysis of adoption vs. control group",
					IsDirectClaudeCodeEvidence: false,
				},
				{
					Citation: "Liu et al. (2026)",
					Finding: "304,000 AI-authored commits across 6,275 repositories found measurable " +
						"technical debt; approximately one-quarter of AI-introduced issues persist to " +
						"the latest revision at substantially higher rates than human-introduced issues.",
					DataScope:                  "6,275 GitHub repositories; large-scale audit of AI-authored commits",
					IsDirectClaudeCodeEvidence: false,
				},
			},
			Mitigations: []ArchitecturalMitigation{
				{
					Slug: "graduated_compression",
					Description: "Graduated compression preserves the most recent and most relevant " +
						"context, reducing information loss at the most critical compaction stages.",
					PDFSection: "7",
				},
				{
					Slug: "cache_aware_compaction",
					Description: "Cache-aware compaction avoids invalidating prompt caches during " +
						"compression, preserving continuity of the compressed context.",
					PDFSection: "7",
				},
				{
					Slug: "read_time_projection",
					Description: "Read-time projection maintains full append-only history for " +
						"reconstruction while presenting a compressed view to the model.",
					PDFSection: "7",
				},
			},
			IsDirectlyMeasurable:        true,
			MitigationClaimedSufficient: false,
		},
		{
			ID:         PredictionSubagentRedundancy,
			Title:      "Subagent Redundant Re-implementation from Context Isolation",
			PDFSection: "11.4",
			Category:   PredictionCategoryCoordination,
			ArchitecturalRoot: "§8 subagent isolation: each subagent operates in its own context " +
				"window with an independently assembled tool pool",
			Prediction: "Parallel agents can independently re-implement solutions that already exist " +
				"elsewhere in the codebase, because each subagent operates in its own context window " +
				"with an independently assembled tool pool and has no mechanism to observe peer agents' " +
				"ongoing work.",
			CausalChain: "Subagent isolation (§8) → each parallel agent assembles its own context " +
				"independently → agents lack visibility into sibling agents' in-progress or completed " +
				"work → redundant re-implementation of existing solutions in parallel subtask execution.",
			EarlySignals: []EmpiricalSignal{
				{
					Citation: "He et al. (2025) — indirect signal",
					Finding: "Complexity increases observed in adjacent tools suggest that " +
						"architectural parallels (bounded context, tool-use loops, single-pass generation) " +
						"are architecturally relevant to subagent coordination quality.",
					DataScope:                  "807 Cursor repositories; indirect architectural parallel, not direct subagent study",
					IsDirectClaudeCodeEvidence: false,
				},
			},
			Mitigations: []ArchitecturalMitigation{
				{
					Slug: "subagent_summary_isolation",
					Description: "Subagent summary isolation prevents exploratory noise from " +
						"accumulating in the parent context; summary-only returns give the parent " +
						"agent a condensed view of subagent outcomes.",
					PDFSection: "8",
				},
			},
			IsDirectlyMeasurable:        true,
			MitigationClaimedSufficient: false,
		},
		{
			ID:         PredictionLocalGoodGlobalBad,
			Title:      "Local-Good Global-Bad Decisions from Model Autonomy Under Bounded Context",
			PDFSection: "11.4",
			Category:   PredictionCategorySystemCoherence,
			ArchitecturalRoot: "§11.1 design philosophy: architecture trusts the model to make good " +
				"local decisions within a rich deterministic harness; 1.6% decision-logic / " +
				"98.4% operational harness ratio",
			Prediction: "Good local decisions can produce poor global outcomes when bounded context " +
				"prevents global awareness. The design philosophy of §11.1 trusts the model to make " +
				"good local decisions, but the structural limitation of bounded context means the " +
				"model lacks the information required to ensure global consistency.",
			CausalChain: "§11.1 design philosophy grants model maximum decision latitude within the " +
				"harness → bounded context (§7) prevents the model from maintaining full codebase " +
				"awareness → model makes individually-correct choices that are globally inconsistent → " +
				"poor global outcomes despite individually-rational local decisions.",
			EarlySignals: []EmpiricalSignal{
				{
					Citation: "He et al. (2025)",
					Finding: "Self-cancelling gains: velocity spike in month 1 dissipated by month 3 " +
						"as rising complexity caused proportional decrease in future development velocity, " +
						"suggesting locally-helpful AI changes compound into globally-degrading complexity.",
					DataScope:                  "807 Cursor repositories; longitudinal analysis",
					IsDirectClaudeCodeEvidence: false,
				},
				{
					Citation: "Liu et al. (2026)",
					Finding: "304,000 AI-authored commits across 6,275 repositories: AI-introduced " +
						"issues persist at substantially higher rates than human-introduced issues to " +
						"the latest revision, suggesting that individually plausible AI commits create " +
						"lasting system-level coherence problems.",
					DataScope:                  "304,000 AI-authored commits across 6,275 repositories",
					IsDirectClaudeCodeEvidence: false,
				},
			},
			Mitigations: []ArchitecturalMitigation{
				{
					Slug: "graduated_compression_global",
					Description: "Graduated compression with cache-aware compaction preserves the most " +
						"relevant context, mitigating (but not eliminating) bounded-context global blindness.",
					PDFSection: "7",
				},
				{
					Slug: "read_time_projection_global",
					Description: "Read-time projection maintains full history for reconstruction, allowing " +
						"post-hoc global reconstruction even when real-time global awareness is bounded.",
					PDFSection: "7",
				},
			},
			IsDirectlyMeasurable:        true,
			MitigationClaimedSufficient: false,
		},
	}
}
