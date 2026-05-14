package agentic

// design_space_question.go — §3.1 Design Questions and Running Example
//
// FEAT-029 — arXiv:2604.14228v1
//
// §3.1 (page 6) frames the architecture description around four recurring
// design questions that every production coding agent must answer.  For each
// question the paper:
//   (a) states the question,
//   (b) describes Claude Code's answer and the source-level evidence,
//   (c) names the Table 1 principle(s) the answer is grounded in,
//   (d) lists alternative approaches other systems use, and
//   (e) names the tradeoff the chosen answer accepts.
//
// The four questions are:
//
//   1. DesignQuestionWhereReasoningLives
//      "Where does reasoning live?"
//      Answer: in the model (harness executes); the harness never reasons.
//      Evidence: queryLoop() in query.ts; 1.6% AI logic / 98.4% harness ratio.
//      Alternatives: Devin (explicit planning in scaffolding), LangGraph
//      (control flow via developer-defined state graphs).
//
//   2. DesignQuestionHowManyEngines
//      "How many execution engines?"
//      Answer: one — queryLoop() regardless of surface (CLI, headless, SDK, IDE).
//      Evidence: query.ts shared by all entry points; QueryEngine delegates to query().
//      Alternatives: mode-specific engines (IDE vs. CLI separate code paths).
//
//   3. DesignQuestionDefaultSafetyPosture
//      "What is the default safety posture?"
//      Answer: deny-first with human escalation; multiple independent safety layers.
//      Evidence: permissions.ts; seven-layer safety stack (§3.5).
//      Alternatives: SWE-Agent / OpenHands container isolation, Aider git-rollback.
//
//   4. DesignQuestionBindingResourceConstraint
//      "What is the binding resource constraint?"
//      Answer: context window; five sequential shapers before every model call.
//      Evidence: query.ts:365-453; five-layer compaction pipeline (§4.3 + §7.3).
//      Alternatives: compute-budget (model-call count), working-memory (explicit
//      scratchpad), tool-invocation quota.
//
// The registry is pure Go, no DB, no HTTP.

// DesignSpaceQuestionSlug is a stable slug for one of the four §3.1 design questions.
type DesignSpaceQuestionSlug string

const (
	// DesignQuestionWhereReasoningLives — "Where does reasoning live?"
	// Claude's answer: in the model; the harness only executes.
	DesignQuestionWhereReasoningLives DesignSpaceQuestionSlug = "where_reasoning_lives"

	// DesignQuestionHowManyEngines — "How many execution engines?"
	// Claude's answer: one queryLoop() shared across all surfaces.
	DesignQuestionHowManyEngines DesignSpaceQuestionSlug = "how_many_engines"

	// DesignQuestionDefaultSafetyPosture — "What is the default safety posture?"
	// Claude's answer: deny-first with layered independent mechanisms.
	DesignQuestionDefaultSafetyPosture DesignSpaceQuestionSlug = "default_safety_posture"

	// DesignQuestionBindingResourceConstraint — "What is the binding resource constraint?"
	// Claude's answer: context window, managed by a five-layer compaction pipeline.
	DesignQuestionBindingResourceConstraint DesignSpaceQuestionSlug = "binding_resource_constraint"
)

// SeedDesignSpaceQuestionCount is the total number of §3.1 design questions.
const SeedDesignSpaceQuestionCount = 4

// SeedDesignSpaceQuestionSlugs lists the four slugs in §3.1 canonical order.
var SeedDesignSpaceQuestionSlugs = []DesignSpaceQuestionSlug{
	DesignQuestionWhereReasoningLives,
	DesignQuestionHowManyEngines,
	DesignQuestionDefaultSafetyPosture,
	DesignQuestionBindingResourceConstraint,
}

// DesignSpaceAlternative describes one alternative approach that another agent
// system takes for the same design question.
type DesignSpaceAlternative struct {
	// SystemName names the external system or pattern (e.g. "LangGraph", "Devin").
	SystemName string
	// Approach briefly describes how the alternative resolves the design question.
	Approach string
	// AlternativeType classifies the structural nature of the alternative.
	AlternativeType string // "scaffolding_reasoning" | "container_isolation" | "mode_specific_engine" | "alternate_resource"
}

// DesignSpaceQuestionProfile is the full structured profile for one §3.1 design question.
type DesignSpaceQuestionProfile struct {
	// Slug is the stable identifier for this design question.
	Slug DesignSpaceQuestionSlug

	// QuestionOrder is the 1-based index in the §3.1 canonical list.
	QuestionOrder int

	// QuestionText is the verbatim question from §3.1.
	QuestionText string

	// PDFSection is always "3.1" for all four questions.
	PDFSection string

	// ClaudeAnswer describes Claude Code's chosen answer to the question.
	ClaudeAnswer string

	// SourceEvidence names the source-level artefacts that implement the answer.
	// Drawn directly from the §3.1 prose (e.g. "query.ts", "permissions.ts").
	SourceEvidence []string

	// GroundingPrinciples lists the Table 1 principle IDs that motivate the answer.
	GroundingPrinciples []DesignPrincipleID

	// Alternatives lists the alternative approaches other systems take.
	Alternatives []DesignSpaceAlternative

	// TradeoffAccepted describes what is sacrificed by Claude's chosen answer.
	TradeoffAccepted string
}

// designSpaceQuestionProfiles is the canonical ordered data for §3.1.
var designSpaceQuestionProfiles = []DesignSpaceQuestionProfile{
	{
		Slug:          DesignQuestionWhereReasoningLives,
		QuestionOrder: 1,
		QuestionText:  "Where does reasoning live?",
		PDFSection:    "3.1",
		ClaudeAnswer: "Reasoning lives in the model; the harness is responsible only for " +
			"executing actions.  The model emits tool_use blocks; the harness parses them, " +
			"checks permissions, dispatches them to tool implementations, and collects results " +
			"(query.ts).  The model never directly accesses the filesystem, runs shell commands, " +
			"or makes network requests.  Community analysis estimates ~1.6% of the codebase is " +
			"AI decision logic; 98.4% is operational infrastructure.",
		SourceEvidence: []string{
			"query.ts (queryLoop() async generator)",
			"yoloClassifier.ts (ML safety, not model-side reasoning)",
			"tools.ts (assembleToolPool — flat pool exposed to model)",
		},
		GroundingPrinciples: []DesignPrincipleID{
			PrincipleMinimalScaffoldingMaximalHarness,
			PrincipleValuesOverRules,
		},
		Alternatives: []DesignSpaceAlternative{
			{
				SystemName:      "Devin",
				Approach:        "Maintains explicit planning structures and task-tracking state in the scaffolding layer; reasoning is partially offloaded to scaffolding-side state machines.",
				AlternativeType: "scaffolding_reasoning",
			},
			{
				SystemName:      "LangGraph",
				Approach:        "Routes control flow through developer-defined state graphs with typed edges; reasoning is encoded in graph structure, not solely in the model.",
				AlternativeType: "scaffolding_reasoning",
			},
		},
		TradeoffAccepted: "Search completeness is traded for simplicity and latency: each turn commits " +
			"to one action sequence without backtracking.  A compromised or adversarially manipulated " +
			"model cannot override sandboxing or deny-first rules because reasoning and enforcement " +
			"occupy separate code paths.",
	},
	{
		Slug:          DesignQuestionHowManyEngines,
		QuestionOrder: 2,
		QuestionText:  "How many execution engines are needed?",
		PDFSection:    "3.1",
		ClaudeAnswer: "Claude Code uses a single queryLoop() function that executes regardless of " +
			"whether the user is interacting through an interactive terminal, a headless CLI " +
			"invocation, the Agent SDK, or an IDE integration (query.ts).  Only the rendering and " +
			"user-interaction layer varies.  QueryEngine is a conversation wrapper — it delegates " +
			"to query(); the interactive CLI also calls query() directly, bypassing QueryEngine.",
		SourceEvidence: []string{
			"query.ts (shared queryLoop across all entry points)",
			"QueryEngine.ts (conversation wrapper; delegates to query())",
			"src/entrypoints/ (multiple surfaces, one shared loop)",
		},
		GroundingPrinciples: []DesignPrincipleID{
			PrincipleMinimalScaffoldingMaximalHarness,
			PrincipleGracefulRecoveryResilience,
		},
		Alternatives: []DesignSpaceAlternative{
			{
				SystemName:      "Mode-specific engine pattern",
				Approach:        "An IDE integration follows a different code path than a CLI tool, enabling surface-specific optimisation at the cost of divergent behaviour across deployment modes.",
				AlternativeType: "mode_specific_engine",
			},
		},
		TradeoffAccepted: "Uniformity across surfaces is bought at the cost of surface-specific " +
			"optimisation.  Every surface shares the same context-management pipeline, compaction " +
			"overhead, and recovery mechanisms even when the surface could be lighter-weight.",
	},
	{
		Slug:          DesignQuestionDefaultSafetyPosture,
		QuestionOrder: 3,
		QuestionText:  "What is the default safety posture?",
		PDFSection:    "3.1",
		ClaudeAnswer: "Deny-first with human escalation: deny rules override ask rules override " +
			"allow rules, and unrecognised actions are escalated to the user rather than allowed " +
			"silently (permissions.ts).  Multiple independent safety layers (permission rules, " +
			"PreToolUse hooks, ML auto-mode classifier, optional shell sandbox) apply in parallel, " +
			"so any one layer can block an action.  This combines deny-first with human escalation " +
			"and defense in depth with layered mechanisms from Table 1.",
		SourceEvidence: []string{
			"permissions.ts (deny-first rule evaluation; seven permission modes)",
			"shouldUseSandbox.ts (shell sandboxing; orthogonal to permission layer)",
			"yoloClassifier.ts (ML auto-mode classifier; TRANSCRIPT_CLASSIFIER feature flag)",
			"types/hooks.ts (27 hook event types; PreToolUse participates in permission flow)",
		},
		GroundingPrinciples: []DesignPrincipleID{
			PrincipleDenyFirstHumanEscalation,
			PrincipleDefenseInDepthLayered,
			PrincipleReversibilityWeightedRisk,
		},
		Alternatives: []DesignSpaceAlternative{
			{
				SystemName:      "SWE-Agent / OpenHands",
				Approach:        "Relies on Docker container isolation to sandbox the agent's entire execution environment rather than evaluating individual tool invocations.",
				AlternativeType: "container_isolation",
			},
			{
				SystemName:      "Aider",
				Approach:        "Uses Git as a safety net: all changes are reversible through version control, making Git rollback the primary safety mechanism rather than deny-first evaluation.",
				AlternativeType: "container_isolation",
			},
		},
		TradeoffAccepted: "Per-action evaluation introduces per-tool latency and complexity.  Layers " +
			"share common performance constraints: commands with more than 50 subcommands fall back to " +
			"a single generic approval prompt, producing a structural tension between safety and " +
			"performance (§5.4 / §11.3).  Users approve ~93% of prompts habitually, so the system " +
			"must maintain safety independently of human vigilance.",
	},
	{
		Slug:          DesignQuestionBindingResourceConstraint,
		QuestionOrder: 4,
		QuestionText:  "What is the binding resource constraint?",
		PDFSection:    "3.1",
		ClaudeAnswer: "The context window (200K tokens for older models; 1M for the 4.6 series) is " +
			"the binding constraint.  Five distinct context-reduction strategies execute before every " +
			"model call (query.ts:365-453): budget reduction (per-message size caps), snip (temporal " +
			"trim of older history), microcompact (fine-grained semantic compression), context " +
			"collapse (read-time full-history projection), and auto-compact (full model-generated " +
			"summary as last resort).  The five-layer pipeline exists because no single compaction " +
			"strategy addresses all types of context pressure.",
		SourceEvidence: []string{
			"query.ts:365-453 (five shapers execute in sequence before every model call)",
			"compact.ts (compactConversation — auto-compact fifth shaper)",
			"tools.ts (deferred tool schemas; ToolSearch lazy-loads on demand)",
			"runAgent.ts (summary-only subagent returns, §8)",
		},
		GroundingPrinciples: []DesignPrincipleID{
			PrincipleContextAsScarceResource,
			PrincipleAppendOnlyDurableState,
			PrincipleGracefulRecoveryResilience,
		},
		Alternatives: []DesignSpaceAlternative{
			{
				SystemName:      "Compute-budget-constrained systems",
				Approach:        "Treat the number of model calls or tool invocations as the primary bottleneck rather than the context window, optimising for call minimisation rather than context economy.",
				AlternativeType: "alternate_resource",
			},
			{
				SystemName:      "Working-memory-based systems",
				Approach:        "Maintain an explicit scratchpad outside the conversation history rather than relying on the context window as the primary store, reducing pressure but introducing scratchpad-management complexity.",
				AlternativeType: "alternate_resource",
			},
		},
		TradeoffAccepted: "Earlier, cheaper layers run before costlier ones; each layer operates at a " +
			"different cost-benefit tradeoff.  Context collapse does not mutate the REPL's stored " +
			"history — it applies a read-time projection — so the full history remains available for " +
			"reconstruction at the cost of processing overhead on every call.",
	},
}

// DesignSpaceQuestionRegistry provides structured access to the four §3.1
// recurring design questions with Claude's answers and alternatives.
type DesignSpaceQuestionRegistry struct {
	questions []DesignSpaceQuestionProfile
	index     map[DesignSpaceQuestionSlug]*DesignSpaceQuestionProfile
}

// NewDesignSpaceQuestionRegistry constructs a registry pre-loaded with the
// four §3.1 design questions in canonical order.
func NewDesignSpaceQuestionRegistry() *DesignSpaceQuestionRegistry {
	r := &DesignSpaceQuestionRegistry{
		questions: make([]DesignSpaceQuestionProfile, len(designSpaceQuestionProfiles)),
		index:     make(map[DesignSpaceQuestionSlug]*DesignSpaceQuestionProfile, len(designSpaceQuestionProfiles)),
	}
	copy(r.questions, designSpaceQuestionProfiles)
	for i := range r.questions {
		r.index[r.questions[i].Slug] = &r.questions[i]
	}
	return r
}

// FindDesignSpaceQuestionBySlug returns the profile for the given slug.
// Returns (profile, true) on success and (nil, false) when the slug is unknown.
func (r *DesignSpaceQuestionRegistry) FindDesignSpaceQuestionBySlug(
	slug DesignSpaceQuestionSlug,
) (*DesignSpaceQuestionProfile, bool) {
	p, ok := r.index[slug]
	return p, ok
}

// AllQuestions returns all four profiles in canonical §3.1 order.
func (r *DesignSpaceQuestionRegistry) AllQuestions() []DesignSpaceQuestionProfile {
	out := make([]DesignSpaceQuestionProfile, len(r.questions))
	copy(out, r.questions)
	return out
}

// QuestionByOrder returns the profile at the given 1-based order position.
// Returns (nil, false) if the order is out of range.
func (r *DesignSpaceQuestionRegistry) QuestionByOrder(order int) (*DesignSpaceQuestionProfile, bool) {
	for i := range r.questions {
		if r.questions[i].QuestionOrder == order {
			return &r.questions[i], true
		}
	}
	return nil, false
}

// QuestionsGroundedIn returns all profiles that list the given principle ID in
// their GroundingPrinciples slice.
func (r *DesignSpaceQuestionRegistry) QuestionsGroundedIn(id DesignPrincipleID) []DesignSpaceQuestionProfile {
	var result []DesignSpaceQuestionProfile
	for i := range r.questions {
		for _, p := range r.questions[i].GroundingPrinciples {
			if p == id {
				result = append(result, r.questions[i])
				break
			}
		}
	}
	return result
}

// AllAlternatives returns the flat list of all DesignSpaceAlternative entries
// across all four questions, in canonical question order.
func (r *DesignSpaceQuestionRegistry) AllAlternatives() []DesignSpaceAlternative {
	var result []DesignSpaceAlternative
	for i := range r.questions {
		result = append(result, r.questions[i].Alternatives...)
	}
	return result
}

// AlternativesByType returns all alternatives whose AlternativeType matches
// the given type string, across all four questions.
func (r *DesignSpaceQuestionRegistry) AlternativesByType(alternativeType string) []DesignSpaceAlternative {
	var result []DesignSpaceAlternative
	for i := range r.questions {
		for _, alt := range r.questions[i].Alternatives {
			if alt.AlternativeType == alternativeType {
				result = append(result, alt)
			}
		}
	}
	return result
}

// TotalAlternativeCount returns the sum of all alternative entries across all
// four questions.
func (r *DesignSpaceQuestionRegistry) TotalAlternativeCount() int {
	total := 0
	for i := range r.questions {
		total += len(r.questions[i].Alternatives)
	}
	return total
}

// IsValidSlug reports whether slug is one of the four registered question slugs.
func (r *DesignSpaceQuestionRegistry) IsValidSlug(slug DesignSpaceQuestionSlug) bool {
	_, ok := r.index[slug]
	return ok
}
