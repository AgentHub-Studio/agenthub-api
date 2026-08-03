package agentic

import "fmt"

// design_philosophy_registry.go — FEAT-045 — §11.1 Design Philosophy
//
// arXiv:2604.14228v1, §11.1 "Design Philosophy" (page 27).
//
// §11.1 synthesises the preceding sections (§3–9) into a coherent design
// philosophy for Claude Code.  It names five distinct, modelable concepts:
//
//  1. DecisionLogicRatio (1.6% / 98.4%) — quantitative measure of how much
//     of the codebase constitutes decision logic vs. operational harness.
//
//  2. MinimalScaffoldingThesis — the architectural choice to invest in
//     operational infrastructure (harness) rather than decision scaffolding;
//     the harness creates conditions under which the model can decide well,
//     rather than constraining its choices.
//
//  3. OSKernelAnalogy — the agent loop (queryLoop()) as architectural kernel;
//     permission system, context management, extensibility, subagents, and
//     session persistence as the surrounding OS.
//
//  4. ConvergenceThesis — as frontier models improve in practical coding
//     capability, coding agents converge toward OS-like abstractions where
//     the quality of the operational harness is the principal differentiator.
//
//  5. AlternativeDesignFamilies — the three named design families that
//     §2.2 contrasts with Claude Code's approach: rule-based orchestration
//     (LangGraph), container-isolated execution (SWEAgent / OpenHands), and
//     version-control-as-safety (Aider).
//
// The registry is pure Go, no DB, no HTTP.  It is intended for introspection,
// documentation generation, and test-driven validation of §11.1 claims.

// PhilosophyConceptID is a stable slug for one of the five §11.1 concepts.
type PhilosophyConceptID string

const (
	// ConceptDecisionLogicRatio — the 1.6% decision-logic / 98.4% operational
	// harness split documented in §11.1.  The ratio is not accidental: it is the
	// intended consequence of the minimal-scaffolding principle.
	ConceptDecisionLogicRatio PhilosophyConceptID = "decision_logic_ratio"

	// ConceptMinimalScaffoldingThesis — the architectural commitment to invest
	// in operational infrastructure rather than decision scaffolding.  Harness
	// creates conditions under which the model can decide well, rather than
	// constraining its choices via explicit planners or state graphs.
	ConceptMinimalScaffoldingThesis PhilosophyConceptID = "minimal_scaffolding_thesis"

	// ConceptOSKernelAnalogy — the framing of the agent loop (queryLoop()) as
	// an architectural kernel with permission system, context management,
	// extensibility, subagents, and persistence as the surrounding OS.
	ConceptOSKernelAnalogy PhilosophyConceptID = "os_kernel_analogy"

	// ConceptConvergenceThesis — the claim that as frontier models converge in
	// practical coding capability, the quality of the surrounding operational
	// harness becomes the principal differentiator, validating the investment in
	// deterministic infrastructure over decision scaffolding.
	ConceptConvergenceThesis PhilosophyConceptID = "convergence_thesis"

	// ConceptAlternativeDesignFamilies — the three named alternative design
	// families that §2.2 contrasts with Claude Code: rule-based orchestration,
	// container-isolated execution, and version-control-as-safety.
	ConceptAlternativeDesignFamilies PhilosophyConceptID = "alternative_design_families"
)

// AlternativeDesignFamilyID is a slug for one of the three §2.2 design families
// that Claude Code does NOT adopt.
type AlternativeDesignFamilyID string

const (
	// FamilyRuleBasedOrchestration — LangGraph-style frameworks that encode
	// decision logic as explicit state graphs with typed edges, choosing
	// scaffolding over minimal harness.
	FamilyRuleBasedOrchestration AlternativeDesignFamilyID = "rule_based_orchestration"

	// FamilyContainerIsolatedExecution — SWEAgent and OpenHands rely on Docker
	// container isolation rather than layered policy enforcement.
	FamilyContainerIsolatedExecution AlternativeDesignFamilyID = "container_isolated_execution"

	// FamilyVersionControlAsSafety — Aider uses Git rollback as the primary
	// safety mechanism rather than deny-first evaluation.
	FamilyVersionControlAsSafety AlternativeDesignFamilyID = "version_control_as_safety"
)

// AlternativeDesignFamily describes one of the three design families that
// §2.2 explicitly names as contrasting with Claude Code's approach.
type AlternativeDesignFamily struct {
	ID              AlternativeDesignFamilyID
	Label           string   // short human-readable name
	ExemplarSystems []string // systems cited in §2.2 as exemplars
	CoreMechanism   string   // what they invest in (instead of harness)
	Contrast        string   // how Claude Code differs
	PDFSection      string   // PDF section where this family is named
}

var alternativeDesignFamilies = map[AlternativeDesignFamilyID]AlternativeDesignFamily{
	FamilyRuleBasedOrchestration: {
		ID:              FamilyRuleBasedOrchestration,
		Label:           "Rule-based orchestration",
		ExemplarSystems: []string{"LangGraph"},
		CoreMechanism:   "Explicit state graphs with typed edges encode decision logic; scaffolding constrains model choices.",
		Contrast:        "Claude Code gives the model maximum decision latitude within a rich operational harness; no explicit planning graph is imposed.",
		PDFSection:      "2.2",
	},
	FamilyContainerIsolatedExecution: {
		ID:              FamilyContainerIsolatedExecution,
		Label:           "Container-isolated execution",
		ExemplarSystems: []string{"SWEAgent", "OpenHands"},
		CoreMechanism:   "Docker container isolation as the primary trust boundary; per-action safety classification is minimal.",
		Contrast:        "Claude Code uses layered policy enforcement (deny-first rules, ML classifier, sandboxing) rather than container isolation as the sole boundary.",
		PDFSection:      "2.2",
	},
	FamilyVersionControlAsSafety: {
		ID:              FamilyVersionControlAsSafety,
		Label:           "Version-control-as-safety",
		ExemplarSystems: []string{"Aider"},
		CoreMechanism:   "Git rollback as the primary safety mechanism; changes are reversible via version control rather than prevented by evaluation.",
		Contrast:        "Claude Code uses deny-first evaluation at time of action rather than relying on post-hoc rollback as the primary safety guarantee.",
		PDFSection:      "2.2",
	},
}

// alternativeDesignFamilySequence is the canonical §2.2 ordering.
var alternativeDesignFamilySequence = []AlternativeDesignFamilyID{
	FamilyRuleBasedOrchestration,
	FamilyContainerIsolatedExecution,
	FamilyVersionControlAsSafety,
}

// PhilosophyConcept is the structured profile for one of the five §11.1 concepts.
type PhilosophyConcept struct {
	// ID is the stable slug.
	ID PhilosophyConceptID
	// Label is a short human-readable name.
	Label string
	// Description is the §11.1 textual content for this concept.
	Description string
	// PDFSection is the primary PDF section where this concept appears.
	PDFSection string
	// GroundedInPrinciple is the Table 1 principle that this concept most directly
	// instantiates, if any.  Empty string means it is a synthesis concept.
	GroundedInPrinciple DesignPrincipleID
	// QuantitativeClaim holds a measurable assertion from §11.1, if the concept
	// carries one (e.g. "1.6%" for DecisionLogicRatio).  Empty for qualitative concepts.
	QuantitativeClaim string
	// DesignImplication is what §11.1 says this concept means for agent builders.
	DesignImplication string
	// IsQuantified reports whether the concept carries a numeric claim from the paper.
	IsQuantified bool
}

var philosophyConcepts = map[PhilosophyConceptID]PhilosophyConcept{
	ConceptDecisionLogicRatio: {
		ID:                  ConceptDecisionLogicRatio,
		Label:               "Decision-logic ratio (1.6% / 98.4%)",
		Description:         "An estimated 1.6% of the Claude Code codebase constitutes decision logic; the remaining 98.4% is operational harness (permission gates, tool routing, context management, recovery logic). §11.1 states this ratio is not accidental — it is the intended consequence of the minimal-scaffolding design principle.",
		PDFSection:          "11.1",
		GroundedInPrinciple: PrincipleMinimalScaffoldingMaximalHarness,
		QuantitativeClaim:   "decision_logic=1.6%, operational_harness=98.4%",
		DesignImplication:   "Agent builders can measure how much of their codebase is scaffolding vs. harness; a high scaffolding fraction suggests the design is constraining model choices rather than enabling them.",
		IsQuantified:        true,
	},
	ConceptMinimalScaffoldingThesis: {
		ID:                  ConceptMinimalScaffoldingThesis,
		Label:               "Minimal scaffolding, maximal harness thesis",
		Description:         "The architecture intentionally invests in operational infrastructure — permission gates, context assembly, recovery mechanisms, tool routing — rather than decision scaffolding (explicit planners, state graphs, typed edges). The harness creates conditions under which the model can decide well, rather than constraining its choices. The model is invoked as a stateless completion endpoint inside a rich deterministic context.",
		PDFSection:          "11.1",
		GroundedInPrinciple: PrincipleMinimalScaffoldingMaximalHarness,
		QuantitativeClaim:   "",
		DesignImplication:   "Investing in deterministic infrastructure (context management, safety layering, recovery mechanisms) may yield greater reliability gains than adding planning scaffolding around increasingly capable models.",
		IsQuantified:        false,
	},
	ConceptOSKernelAnalogy: {
		ID:                  ConceptOSKernelAnalogy,
		Label:               "OS-kernel analogy",
		Description:         "§11.1 raises the question of whether coding tools are converging toward operating-system-like abstractions in which the agent loop serves as the kernel and everything else constitutes the OS. In this framing: queryLoop() is the kernel; the permission system, context management pipeline, extensibility architecture, subagent delegation, and session persistence are the surrounding OS layers. All entry surfaces (CLI, API, IDE extension) converge on the same agent loop, consistent with the kernel framing.",
		PDFSection:          "11.1",
		GroundedInPrinciple: "",
		QuantitativeClaim:   "",
		DesignImplication:   "The kernel framing suggests architectural priorities: the loop must be minimal and stable (kernel), while surrounding subsystems can evolve independently (OS modules).",
		IsQuantified:        false,
	},
	ConceptConvergenceThesis: {
		ID:                  ConceptConvergenceThesis,
		Label:               "Frontier-model convergence thesis",
		Description:         "§11.1 argues that as frontier models converge in practical coding capability, the quality of the surrounding operational harness becomes the principal differentiator. An architecture that invests in infrastructure over decision scaffolding is validated when model capability is no longer the bottleneck — what remains is the harness's ability to route, recover, compress, and enforce policy correctly. This validates the minimal-scaffolding investment as a long-term architectural bet.",
		PDFSection:          "11.1",
		GroundedInPrinciple: PrincipleMinimalScaffoldingMaximalHarness,
		QuantitativeClaim:   "",
		DesignImplication:   "Agent builders should prioritise harness quality over scaffolding sophistication, because harness quality compounds as model capability improves.",
		IsQuantified:        false,
	},
	ConceptAlternativeDesignFamilies: {
		ID:                  ConceptAlternativeDesignFamilies,
		Label:               "Alternative design families",
		Description:         "§2.2 names three alternative design families that make different bets: (1) rule-based orchestration (LangGraph) — encode decision logic as explicit state graphs, scaffolding constrains model choices; (2) container-isolated execution (SWEAgent, OpenHands) — Docker isolation as primary trust boundary; (3) version-control-as-safety (Aider) — Git rollback as primary safety mechanism. Claude Code's principle set is distinctive in combining minimal decision scaffolding with layered policy enforcement, values-based judgment with deny-first defaults, and progressive context management with composable extensibility.",
		PDFSection:          "2.2",
		GroundedInPrinciple: "",
		QuantitativeClaim:   "",
		DesignImplication:   "The three families represent distinct positions in the design space; the choice between them follows from deployment context (trust model, user expectations, operational constraints) rather than from model capability alone.",
		IsQuantified:        false,
	},
}

// philosophyConceptSequence is the canonical §11.1 ordering.
var philosophyConceptSequence = []PhilosophyConceptID{
	ConceptDecisionLogicRatio,
	ConceptMinimalScaffoldingThesis,
	ConceptOSKernelAnalogy,
	ConceptConvergenceThesis,
	ConceptAlternativeDesignFamilies,
}

// DesignPhilosophyRegistry provides structured access to the five §11.1
// design-philosophy concepts and the three §2.2 alternative design families.
type DesignPhilosophyRegistry struct{}

// NewDesignPhilosophyRegistry returns a ready-to-use registry.
func NewDesignPhilosophyRegistry() *DesignPhilosophyRegistry {
	return &DesignPhilosophyRegistry{}
}

// FindByID returns the concept for the given ID.  Returns (zero, false) if unknown.
func (r *DesignPhilosophyRegistry) FindByID(id PhilosophyConceptID) (PhilosophyConcept, bool) {
	c, ok := philosophyConcepts[id]
	return c, ok
}

// AllConcepts returns all five §11.1 concepts in canonical order.
func (r *DesignPhilosophyRegistry) AllConcepts() []PhilosophyConcept {
	out := make([]PhilosophyConcept, len(philosophyConceptSequence))
	for i, id := range philosophyConceptSequence {
		out[i] = philosophyConcepts[id]
	}
	return out
}

// Count returns the number of §11.1 philosophy concepts (always 5).
func (r *DesignPhilosophyRegistry) Count() int {
	return len(philosophyConceptSequence)
}

// IsValidConcept returns true if the ID is one of the five §11.1 concepts.
func (r *DesignPhilosophyRegistry) IsValidConcept(id PhilosophyConceptID) bool {
	_, ok := philosophyConcepts[id]
	return ok
}

// QuantifiedConcepts returns all concepts that carry a numeric claim from the paper.
func (r *DesignPhilosophyRegistry) QuantifiedConcepts() []PhilosophyConcept {
	var out []PhilosophyConcept
	for _, id := range philosophyConceptSequence {
		c := philosophyConcepts[id]
		if c.IsQuantified {
			out = append(out, c)
		}
	}
	return out
}

// ConceptsGroundedInPrinciple returns all concepts whose GroundedInPrinciple
// field matches the given principle ID.
func (r *DesignPhilosophyRegistry) ConceptsGroundedInPrinciple(p DesignPrincipleID) []PhilosophyConcept {
	var out []PhilosophyConcept
	for _, id := range philosophyConceptSequence {
		c := philosophyConcepts[id]
		if c.GroundedInPrinciple == p {
			out = append(out, c)
		}
	}
	return out
}

// ConceptsWithDesignImplication returns all concepts that have a non-empty
// DesignImplication field (all five do, but this allows filtering by content).
func (r *DesignPhilosophyRegistry) ConceptsWithDesignImplication() []PhilosophyConcept {
	var out []PhilosophyConcept
	for _, id := range philosophyConceptSequence {
		c := philosophyConcepts[id]
		if c.DesignImplication != "" {
			out = append(out, c)
		}
	}
	return out
}

// --- Alternative Design Families ---

// AllAlternativeDesignFamilies returns all three §2.2 alternative design families
// in canonical order.
func (r *DesignPhilosophyRegistry) AllAlternativeDesignFamilies() []AlternativeDesignFamily {
	out := make([]AlternativeDesignFamily, len(alternativeDesignFamilySequence))
	for i, id := range alternativeDesignFamilySequence {
		out[i] = alternativeDesignFamilies[id]
	}
	return out
}

// FindFamilyByID returns the alternative design family for the given ID.
// Returns (zero, false) if unknown.
func (r *DesignPhilosophyRegistry) FindFamilyByID(id AlternativeDesignFamilyID) (AlternativeDesignFamily, bool) {
	f, ok := alternativeDesignFamilies[id]
	return f, ok
}

// FamilyCount returns the number of alternative design families (always 3).
func (r *DesignPhilosophyRegistry) FamilyCount() int {
	return len(alternativeDesignFamilySequence)
}

// IsValidFamilyID returns true if the ID is one of the three §2.2 families.
func (r *DesignPhilosophyRegistry) IsValidFamilyID(id AlternativeDesignFamilyID) bool {
	_, ok := alternativeDesignFamilies[id]
	return ok
}

// FamilyByExemplar returns the design family whose ExemplarSystems slice contains
// the given system name (case-sensitive).  Returns (zero, false) if not found.
func (r *DesignPhilosophyRegistry) FamilyByExemplar(system string) (AlternativeDesignFamily, bool) {
	for _, id := range alternativeDesignFamilySequence {
		f := alternativeDesignFamilies[id]
		for _, s := range f.ExemplarSystems {
			if s == system {
				return f, true
			}
		}
	}
	return AlternativeDesignFamily{}, false
}

// --- Structural invariants ---

// PhilosophyInvariantResult reports whether a §11.1 structural invariant holds.
type PhilosophyInvariantResult struct {
	Name    string
	Holds   bool
	Message string
}

// StructuralInvariants verifies the three invariants that §11.1 implies:
//
//  1. FiveConcepts — the registry contains exactly five §11.1 concepts.
//  2. OneQuantifiedConcept — exactly one concept carries a numeric claim
//     (DecisionLogicRatio at 1.6%/98.4%).
//  3. ThreeAlternativeFamilies — the registry contains exactly three
//     §2.2 alternative design families.
func (r *DesignPhilosophyRegistry) StructuralInvariants() []PhilosophyInvariantResult {
	results := []PhilosophyInvariantResult{}

	// Invariant 1: five concepts.
	n := r.Count()
	results = append(results, PhilosophyInvariantResult{
		Name:    "FiveConcepts",
		Holds:   n == 5,
		Message: fmt.Sprintf("expected 5 §11.1 concepts, got %d", n),
	})

	// Invariant 2: exactly one quantified concept.
	q := r.QuantifiedConcepts()
	results = append(results, PhilosophyInvariantResult{
		Name:    "OneQuantifiedConcept",
		Holds:   len(q) == 1 && q[0].ID == ConceptDecisionLogicRatio,
		Message: fmt.Sprintf("expected exactly 1 quantified concept (DecisionLogicRatio), got %d", len(q)),
	})

	// Invariant 3: three alternative design families.
	fn := r.FamilyCount()
	results = append(results, PhilosophyInvariantResult{
		Name:    "ThreeAlternativeFamilies",
		Holds:   fn == 3,
		Message: fmt.Sprintf("expected 3 §2.2 alternative families, got %d", fn),
	})

	return results
}
