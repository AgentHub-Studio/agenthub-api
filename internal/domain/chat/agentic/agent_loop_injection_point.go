package agentic

// AgentLoopInjectionPoint identifies one of the three named injection points
// through which extension mechanisms plug into Claude Code's agent loop.
//
// §6.1 (Figure 5, page 16) of the Claude Code architecture paper (arXiv:2604.14228v1)
// describes these three points explicitly in the pseudocode caption:
//
//	"Every agent loop has three injection points:
//	(a) assemble() controls what the model sees,
//	(b) model() controls what it can reach,
//	(c) execute() controls whether and how an action actually runs."
//
// Each injection point has a distinct set of elements that contribute to it,
// detailed in the three side-tables of Figure 5. The four extension mechanisms
// (MCP servers, plugins, skills, hooks) plug into these points at different
// context costs — from zero (hooks at execute) to high (MCP servers at model).
type AgentLoopInjectionPoint string

const (
	// AgentLoopInjectionPointAssemble is the (a) assemble() phase.
	// Controls what the model sees — the context window composition.
	// §6.1 Figure 5 table (a): CLAUDE.md files, skill descriptions,
	// MCP resources & prompts, output style, UserPromptSubmit hook, SessionStart hook.
	AgentLoopInjectionPointAssemble AgentLoopInjectionPoint = "assemble"

	// AgentLoopInjectionPointModel is the (b) model() phase.
	// Controls what the model can reach — the flat tool pool.
	// §6.1 Figure 5 table (b): built-in tools, MCP tools, SkillTool, AgentTool.
	AgentLoopInjectionPointModel AgentLoopInjectionPoint = "model"

	// AgentLoopInjectionPointExecute is the (c) execute() phase.
	// Controls whether and how an action actually runs.
	// §6.1 Figure 5 table (c): permission rules, PreToolUse hook, PostToolUse hook,
	// Stop hook, SubagentStop hook, Notification hook.
	AgentLoopInjectionPointExecute AgentLoopInjectionPoint = "execute"
)

// agentLoopInjectionOrder is the canonical order matching Figure 5 labels (a)→(b)→(c).
var agentLoopInjectionOrder = []AgentLoopInjectionPoint{
	AgentLoopInjectionPointAssemble,
	AgentLoopInjectionPointModel,
	AgentLoopInjectionPointExecute,
}

// AgentLoopInjectionElement represents a single named element that contributes
// to one injection point, as listed in the Figure 5 side-tables (§6.1).
type AgentLoopInjectionElement struct {
	// Name is the element identifier as named in the Figure 5 table.
	Name string

	// WhatItDoes is the concise description from the table's "What it does" column.
	WhatItDoes string

	// InjectionPoint is the phase this element belongs to.
	InjectionPoint AgentLoopInjectionPoint

	// ExtensionMechanism is the primary extension mechanism that introduces this element,
	// or "" for built-in elements always present.
	// Values: "mcp" | "plugin" | "skill" | "hook" | "builtin"
	ExtensionMechanism string

	// IsHook indicates the element is a hook event type (fires at runtime).
	IsHook bool

	// ContextCost classifies the context-window footprint: "zero" | "low" | "medium" | "high".
	// §6.1 Table 2 / §6.3: hooks=zero, skills=low, plugins=medium, mcp=high.
	ContextCost string
}

// agentLoopInjectionElements is the complete set of elements across all three
// injection points, derived directly from Figure 5 (§6.1, page 16).
var agentLoopInjectionElements = []AgentLoopInjectionElement{
	// ── (a) assemble(): what the model sees ───────────────────────────────────
	{
		Name:               "CLAUDE.md files",
		WhatItDoes:         "Loaded into context; files above the working directory load at startup, and subdirectory files load on demand.",
		InjectionPoint:     AgentLoopInjectionPointAssemble,
		ExtensionMechanism: "builtin",
		IsHook:             false,
		ContextCost:        "low",
	},
	{
		Name:               "Skill descriptions",
		WhatItDoes:         "Advertises skills so the model calls SkillTool.",
		InjectionPoint:     AgentLoopInjectionPointAssemble,
		ExtensionMechanism: "skill",
		IsHook:             false,
		ContextCost:        "low",
	},
	{
		Name:               "MCP resources & prompts",
		WhatItDoes:         "Non-tool content an MCP server pushes.",
		InjectionPoint:     AgentLoopInjectionPointAssemble,
		ExtensionMechanism: "mcp",
		IsHook:             false,
		ContextCost:        "high",
	},
	{
		Name:               "Output style",
		WhatItDoes:         "Replaces the response-formatting system block.",
		InjectionPoint:     AgentLoopInjectionPointAssemble,
		ExtensionMechanism: "plugin",
		IsHook:             false,
		ContextCost:        "medium",
	},
	{
		Name:               "UserPromptSubmit hook",
		WhatItDoes:         "Inject context, or block, on every user turn.",
		InjectionPoint:     AgentLoopInjectionPointAssemble,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
	{
		Name:               "SessionStart hook",
		WhatItDoes:         "One-shot context injection at session start.",
		InjectionPoint:     AgentLoopInjectionPointAssemble,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
	// ── (b) model(): what the model can reach ─────────────────────────────────
	{
		Name:               "Built-in tools",
		WhatItDoes:         "Read / Edit / Bash / … shipped with the CLI.",
		InjectionPoint:     AgentLoopInjectionPointModel,
		ExtensionMechanism: "builtin",
		IsHook:             false,
		ContextCost:        "high",
	},
	{
		Name:               "MCP tools",
		WhatItDoes:         "Tools from any MCP server, in the same flat pool.",
		InjectionPoint:     AgentLoopInjectionPointModel,
		ExtensionMechanism: "mcp",
		IsHook:             false,
		ContextCost:        "high",
	},
	{
		Name:               "SkillTool",
		WhatItDoes:         "Meta-tool that launches a skill by name.",
		InjectionPoint:     AgentLoopInjectionPointModel,
		ExtensionMechanism: "skill",
		IsHook:             false,
		ContextCost:        "low",
	},
	{
		Name:               "AgentTool",
		WhatItDoes:         "Meta-tool that spawns a sub-agent recursively.",
		InjectionPoint:     AgentLoopInjectionPointModel,
		ExtensionMechanism: "builtin",
		IsHook:             false,
		ContextCost:        "high",
	},
	// ── (c) execute(): whether / how an action runs ───────────────────────────
	{
		Name:               "Permission rules",
		WhatItDoes:         "Declarative allow / deny / ask per call.",
		InjectionPoint:     AgentLoopInjectionPointExecute,
		ExtensionMechanism: "builtin",
		IsHook:             false,
		ContextCost:        "zero",
	},
	{
		Name:               "PreToolUse hook",
		WhatItDoes:         "Approve / block / rewrite a tool call.",
		InjectionPoint:     AgentLoopInjectionPointExecute,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
	{
		Name:               "PostToolUse hook",
		WhatItDoes:         "Mutate output or inject context after a call.",
		InjectionPoint:     AgentLoopInjectionPointExecute,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
	{
		Name:               "Stop hook",
		WhatItDoes:         "Force the loop to keep going at model stop.",
		InjectionPoint:     AgentLoopInjectionPointExecute,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
	{
		Name:               "SubagentStop hook",
		WhatItDoes:         "Same as Stop hook, for sub-agents spawned via AgentTool.",
		InjectionPoint:     AgentLoopInjectionPointExecute,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
	{
		Name:               "Notification hook",
		WhatItDoes:         "External side effects on user notifications.",
		InjectionPoint:     AgentLoopInjectionPointExecute,
		ExtensionMechanism: "hook",
		IsHook:             true,
		ContextCost:        "zero",
	},
}

// AgentLoopInjectionPointProfile holds the immutable characteristics of one
// injection point, as described in §6.1 / Figure 5.
type AgentLoopInjectionPointProfile struct {
	// Point is the slug identifier.
	Point AgentLoopInjectionPoint

	// PhaseLabel is the single-word label used in the Figure 5 pseudocode.
	// Values: "assemble" | "model" | "execute"
	PhaseLabel string

	// FigureAnnotation is the letter annotation used in Figure 5: "a", "b", or "c".
	FigureAnnotation string

	// PDFSection identifies the section in arXiv:2604.14228v1 where this is described.
	PDFSection string

	// ControlsSummary is the brief description from the Figure 5 caption.
	ControlsSummary string

	// PrimaryExtensionMechanisms lists the extension mechanism types that contribute
	// the most elements at this injection point.
	PrimaryExtensionMechanisms []string

	// LoopPhaseOrder is the 1-based execution order within a single agent loop turn.
	LoopPhaseOrder int

	// TypicalContextCost is the dominant context-cost classification for this phase.
	// Based on §6.3 Table 2: hooks=zero cost at execute; MCP/tools=high at model;
	// skills/CLAUDE.md=low/medium at assemble.
	TypicalContextCost string
}

var agentLoopInjectionPointProfiles = map[AgentLoopInjectionPoint]AgentLoopInjectionPointProfile{
	AgentLoopInjectionPointAssemble: {
		Point:              AgentLoopInjectionPointAssemble,
		PhaseLabel:         "assemble",
		FigureAnnotation:   "a",
		PDFSection:         "6.1",
		ControlsSummary:    "Controls what the model sees — the context window composition.",
		PrimaryExtensionMechanisms: []string{"builtin", "skill", "mcp", "plugin", "hook"},
		LoopPhaseOrder:     1,
		TypicalContextCost: "low",
	},
	AgentLoopInjectionPointModel: {
		Point:              AgentLoopInjectionPointModel,
		PhaseLabel:         "model",
		FigureAnnotation:   "b",
		PDFSection:         "6.1",
		ControlsSummary:    "Controls what the model can reach — the flat tool pool.",
		PrimaryExtensionMechanisms: []string{"builtin", "mcp", "skill"},
		LoopPhaseOrder:     2,
		TypicalContextCost: "high",
	},
	AgentLoopInjectionPointExecute: {
		Point:              AgentLoopInjectionPointExecute,
		PhaseLabel:         "execute",
		FigureAnnotation:   "c",
		PDFSection:         "6.1",
		ControlsSummary:    "Controls whether and how an action actually runs.",
		PrimaryExtensionMechanisms: []string{"builtin", "hook"},
		LoopPhaseOrder:     3,
		TypicalContextCost: "zero",
	},
}

// SeedAgentLoopInjectionPointCount is the number of injection points seeded (always 3).
const SeedAgentLoopInjectionPointCount = 3

// SeedAgentLoopInjectionPointSlugs lists the three canonical injection-point slugs.
var SeedAgentLoopInjectionPointSlugs = []string{"assemble", "model", "execute"}

// SeedAgentLoopInjectionElementCount is the total number of named elements across all
// three injection points as enumerated in Figure 5 (§6.1).
const SeedAgentLoopInjectionElementCount = 16

// AgentLoopInjectionPointRegistry provides typed queries over the three agent-loop
// injection points and their contributing elements as described in §6.1 / Figure 5
// of arXiv:2604.14228v1.
type AgentLoopInjectionPointRegistry struct{}

// NewAgentLoopInjectionPointRegistry returns a ready-to-use registry.
func NewAgentLoopInjectionPointRegistry() *AgentLoopInjectionPointRegistry {
	return &AgentLoopInjectionPointRegistry{}
}

// FindInjectionPointBySlug returns the profile for the given slug.
// Returns (zero-value, false) if the slug is not one of the three canonical values.
func (r *AgentLoopInjectionPointRegistry) FindInjectionPointBySlug(slug string) (AgentLoopInjectionPointProfile, bool) {
	p, ok := agentLoopInjectionPointProfiles[AgentLoopInjectionPoint(slug)]
	return p, ok
}

// AllInjectionPoints returns all three profiles in loop-phase order (assemble→model→execute).
// The result is a defensive copy.
func (r *AgentLoopInjectionPointRegistry) AllInjectionPoints() []AgentLoopInjectionPointProfile {
	result := make([]AgentLoopInjectionPointProfile, 0, len(agentLoopInjectionOrder))
	for _, pt := range agentLoopInjectionOrder {
		result = append(result, agentLoopInjectionPointProfiles[pt])
	}
	return result
}

// ElementsAt returns all named elements that contribute to the specified injection point,
// preserving the order they appear in Figure 5.
func (r *AgentLoopInjectionPointRegistry) ElementsAt(point AgentLoopInjectionPoint) []AgentLoopInjectionElement {
	var result []AgentLoopInjectionElement
	for _, el := range agentLoopInjectionElements {
		if el.InjectionPoint == point {
			result = append(result, el)
		}
	}
	return result
}

// AllElements returns all 16 named elements across all injection points, in Figure 5 order.
// The result is a defensive copy.
func (r *AgentLoopInjectionPointRegistry) AllElements() []AgentLoopInjectionElement {
	result := make([]AgentLoopInjectionElement, len(agentLoopInjectionElements))
	copy(result, agentLoopInjectionElements)
	return result
}

// HookElements returns all elements that are hook events (IsHook == true), across all phases.
// §6.1: hooks participate at both assemble (UserPromptSubmit, SessionStart) and execute
// (PreToolUse, PostToolUse, Stop, SubagentStop, Notification).
func (r *AgentLoopInjectionPointRegistry) HookElements() []AgentLoopInjectionElement {
	var result []AgentLoopInjectionElement
	for _, el := range agentLoopInjectionElements {
		if el.IsHook {
			result = append(result, el)
		}
	}
	return result
}

// ElementsByMechanism returns all elements contributed by the given extension mechanism.
// mechanism is one of: "mcp" | "plugin" | "skill" | "hook" | "builtin".
func (r *AgentLoopInjectionPointRegistry) ElementsByMechanism(mechanism string) []AgentLoopInjectionElement {
	var result []AgentLoopInjectionElement
	for _, el := range agentLoopInjectionElements {
		if el.ExtensionMechanism == mechanism {
			result = append(result, el)
		}
	}
	return result
}

// ElementsByContextCost returns all elements whose ContextCost matches the given cost tier.
// cost is one of: "zero" | "low" | "medium" | "high".
func (r *AgentLoopInjectionPointRegistry) ElementsByContextCost(cost string) []AgentLoopInjectionElement {
	var result []AgentLoopInjectionElement
	for _, el := range agentLoopInjectionElements {
		if el.ContextCost == cost {
			result = append(result, el)
		}
	}
	return result
}

// LowestCostInjectionPoint returns the injection point whose TypicalContextCost is "zero".
// §6.1/§6.3: execute() has zero default context cost because hooks consume no context tokens
// unless they explicitly inject additional context.
func (r *AgentLoopInjectionPointRegistry) LowestCostInjectionPoint() AgentLoopInjectionPoint {
	return AgentLoopInjectionPointExecute
}

// IsValidInjectionPointSlug returns true for "assemble", "model", and "execute".
func IsValidInjectionPointSlug(slug string) bool {
	_, ok := agentLoopInjectionPointProfiles[AgentLoopInjectionPoint(slug)]
	return ok
}

// AgentLoopInjectionPointPhaseOrderIsAscending validates that the three profiles have
// strictly increasing LoopPhaseOrder values (1, 2, 3) matching Figure 5 annotation order.
func AgentLoopInjectionPointPhaseOrderIsAscending() bool {
	for i, pt := range agentLoopInjectionOrder {
		if agentLoopInjectionPointProfiles[pt].LoopPhaseOrder != i+1 {
			return false
		}
	}
	return true
}

// AgentLoopFigureAnnotationsAreCanonical validates that the three profiles carry
// the exact annotation letters "a", "b", "c" from Figure 5.
func AgentLoopFigureAnnotationsAreCanonical() bool {
	expected := map[AgentLoopInjectionPoint]string{
		AgentLoopInjectionPointAssemble: "a",
		AgentLoopInjectionPointModel:    "b",
		AgentLoopInjectionPointExecute:  "c",
	}
	for pt, want := range expected {
		if agentLoopInjectionPointProfiles[pt].FigureAnnotation != want {
			return false
		}
	}
	return true
}
