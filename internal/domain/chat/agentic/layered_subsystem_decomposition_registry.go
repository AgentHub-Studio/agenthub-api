package agentic

// LayeredSubsystemDecompositionRegistry models the five-layer subsystem
// decomposition defined in §3.3 of the Claude Code architecture paper
// (arXiv:2604.14228v1, "Layered Subsystem Decomposition").
//
// §3.3 states: "The five-layer decomposition (Figure 3) expands the
// seven-component model into a finer-grained view, mapping each layer
// to specific source directories."
//
// The five layers in PDF enumeration order (Figure 3, top-to-bottom):
//  1. Surface layer  — entry points and rendering (src/entrypoints/, src/screens/, src/components/)
//  2. Core layer     — agent loop and compaction pipeline (query.ts, compact.ts/query.ts:365-453)
//  3. Safety/Action layer — permission system, hooks, extensibility, tools, sandbox, subagents
//  4. State layer    — context assembly, runtime state, session persistence, CLAUDE.md + memory, sidechain transcripts
//  5. Backend layer  — execution backends, external resources (BashTool, MCP servers, src/tools/)
//
// This registry is intentionally distinct from:
//   - safety_layer.go (individual safety mechanism types)
//   - package_structure.go (source-level package conventions)
//   - compaction_pipeline_layer.go (§7 compaction pipeline stages)
// §3.3 captures the architectural decomposition view: which source
// directories live in which layer, what each layer is responsible for,
// and how the layers communicate (left-to-right spine: surface → core → safety/action → state → backend).

// SubsystemLayerSlug is the canonical slug for one of the five §3.3 layers.
type SubsystemLayerSlug string

const (
	// SubsystemLayerSurface is layer 1 (§3.3):
	// "Surface layer (entry points and rendering). The src/entrypoints/ directory contains
	// startup paths, including the SDK entry with coreTypes.ts, controlSchemas.ts, and
	// coreSchemas.ts. The src/screens/ directory composes full-screen layouts, and
	// src/components/ provides terminal UI building blocks via the ink framework."
	SubsystemLayerSurface SubsystemLayerSlug = "surface"

	// SubsystemLayerCore is layer 2 (§3.3):
	// "Core layer (agent loop, compaction pipeline). The queryLoop() async generator (query.ts)
	// implements the iterative agent loop, consuming assembled context from the surface layer
	// and dispatching tool requests to the safety/action layer. Before every model call, a
	// compaction pipeline of five sequential shapers (query.ts:365–453) manages context pressure."
	SubsystemLayerCore SubsystemLayerSlug = "core"

	// SubsystemLayerSafetyAction is layer 3 (§3.3):
	// "Safety/action layer (permission system, hooks, extensibility, tools, sandbox, subagents).
	// The permission system (permissions.ts) implements deny-first rule evaluation with up to
	// seven permission modes and an integrated auto-mode ML classifier (yoloClassifier.ts).
	// A hook pipeline spanning 27 event types can block, rewrite, or annotate tool requests.
	// An extensibility subsystem allows plugins and skills to register tools and hooks.
	// Tool pool assembly via assembleToolPool() merges built-in and MCP-provided tools.
	// Approved shell commands pass through the shell sandbox. Subagent spawning via AgentTool."
	SubsystemLayerSafetyAction SubsystemLayerSlug = "safety_action"

	// SubsystemLayerState is layer 4 (§3.3):
	// "State layer (context assembly, runtime state, persistence, memory, sidechains).
	// Context assembly is a memoized state loader, not a routing hub: getSystemContext()
	// computes session-level system context including git status and getUserContext() loads
	// the CLAUDE.md hierarchy and current date. The CLAUDE.md + memory subsystem provides
	// a four-level instruction hierarchy from managed settings to directory-specific files,
	// plus auto-memory entries. Sidechain transcripts store each subagent's conversation."
	SubsystemLayerState SubsystemLayerSlug = "state"

	// SubsystemLayerBackend is layer 5 (§3.3):
	// "Backend layer (execution backends, external resources). Shell command execution with
	// optional sandboxing (BashTool.tsx, PowerShellTool.tsx), remote execution support
	// (src/remote/), MCP server connections across multiple transport variants including
	// stdio, SSE, HTTP, WebSocket, SDK, and IDE-specific adapters (services/mcp/client.ts),
	// and 42 tool subdirectories in src/tools/ implement concrete tool logic."
	SubsystemLayerBackend SubsystemLayerSlug = "backend"
)

// SubsystemLayerProfile holds the immutable structural characteristics of
// one §3.3 subsystem layer in the Claude Code architecture.
type SubsystemLayerProfile struct {
	// Slug is the canonical identifier for this layer.
	Slug SubsystemLayerSlug

	// Label is the human-readable name from the PDF ("Surface layer", etc.).
	Label string

	// PDFSection is the paper section where this layer is defined (always "§3.3").
	PDFSection string

	// LayerOrder is the positional index (1=surface, 5=backend), matching
	// Figure 3 top-to-bottom ordering in the paper.
	LayerOrder int

	// SubtitleInPaper is the parenthetical subtitle used in §3.3 to describe
	// the layer's primary concerns (e.g. "entry points and rendering").
	SubtitleInPaper string

	// PrimarySourceDirs lists the TypeScript source directories that §3.3
	// maps to this layer.
	PrimarySourceDirs []string

	// KeySourceFiles lists the named source files most representative of
	// this layer, as cited directly in §3.3.
	KeySourceFiles []string

	// ResponsibilityDomain is a short classification of what this layer owns:
	// "rendering" | "orchestration" | "safety_and_tools" | "state_and_memory" | "execution"
	ResponsibilityDomain string

	// DirectionInDataFlow describes this layer's role in the left-to-right spine
	// described in §3.3: "surface → agent loop → safety/action → state → backend".
	// Values: "ingress" | "dispatch" | "gate_and_route" | "memoized_state" | "egress"
	DirectionInDataFlow string

	// CommunicatesUpTo is the slug of the adjacent upstream layer (empty for surface).
	CommunicatesUpTo SubsystemLayerSlug

	// CommunicatesDownTo is the slug of the adjacent downstream layer (empty for backend).
	CommunicatesDownTo SubsystemLayerSlug

	// HasIsolatedSubcontext indicates whether this layer can spawn sub-contexts
	// that run with isolated state (only true for safety_action via subagent spawning).
	HasIsolatedSubcontext bool

	// IsStateless indicates whether the layer holds no durable mutable state of its own.
	// Surface and core are stateless in the sense that they consume state produced by
	// the state layer; the state layer itself is the mutable state holder.
	IsStateless bool

	// ContextPressureRole describes how this layer contributes to or relieves the
	// context-as-bottleneck constraint (§3.6):
	// "consumer" | "manager" | "gatekeeper" | "producer" | "executor"
	ContextPressureRole string

	// AgentHubLayerEquivalent describes the closest AgentHub web-service layer
	// that corresponds to this Claude Code layer.
	AgentHubLayerEquivalent string
}

// seedLayeredSubsystemDecomposition holds the five profiles in §3.3 order.
var seedLayeredSubsystemDecomposition = []SubsystemLayerProfile{
	{
		Slug:            SubsystemLayerSurface,
		Label:           "Surface layer",
		PDFSection:      "§3.3",
		LayerOrder:      1,
		SubtitleInPaper: "entry points and rendering",
		PrimarySourceDirs: []string{
			"src/entrypoints/",
			"src/screens/",
			"src/components/",
		},
		KeySourceFiles: []string{
			"coreTypes.ts",
			"controlSchemas.ts",
			"coreSchemas.ts",
		},
		ResponsibilityDomain: "rendering",
		DirectionInDataFlow:  "ingress",
		CommunicatesUpTo:     "",
		CommunicatesDownTo:   SubsystemLayerCore,
		HasIsolatedSubcontext: false,
		IsStateless:          true,
		ContextPressureRole:  "consumer",
		AgentHubLayerEquivalent: "HTTP handler layer — chi routes that receive requests and render " +
			"SSE/JSON responses; Angular frontend chat UI for interactive surface",
	},
	{
		Slug:            SubsystemLayerCore,
		Label:           "Core layer",
		PDFSection:      "§3.3",
		LayerOrder:      2,
		SubtitleInPaper: "agent loop, compaction pipeline",
		PrimarySourceDirs: []string{
			"src/",
		},
		KeySourceFiles: []string{
			"query.ts",
			"compact.ts",
		},
		ResponsibilityDomain: "orchestration",
		DirectionInDataFlow:  "dispatch",
		CommunicatesUpTo:     SubsystemLayerSurface,
		CommunicatesDownTo:   SubsystemLayerSafetyAction,
		HasIsolatedSubcontext: false,
		IsStateless:          true,
		ContextPressureRole:  "manager",
		AgentHubLayerEquivalent: "AgentHub Runner — the agentic loop in runner.go that orchestrates " +
			"LLM calls, tool dispatch, and the five-layer compaction pipeline",
	},
	{
		Slug:            SubsystemLayerSafetyAction,
		Label:           "Safety/action layer",
		PDFSection:      "§3.3",
		LayerOrder:      3,
		SubtitleInPaper: "permission system, hooks, extensibility, tools, sandbox, subagents",
		PrimarySourceDirs: []string{
			"src/permissions/",
			"src/tools/",
			"src/services/",
		},
		KeySourceFiles: []string{
			"permissions.ts",
			"types/permissions.ts",
			"yoloClassifier.ts",
			"types/hooks.ts",
			"tools.ts",
			"shouldUseSandbox.ts",
			"AgentTool.tsx",
			"runAgent.ts",
		},
		ResponsibilityDomain: "safety_and_tools",
		DirectionInDataFlow:  "gate_and_route",
		CommunicatesUpTo:     SubsystemLayerCore,
		CommunicatesDownTo:   SubsystemLayerState,
		HasIsolatedSubcontext: true,
		IsStateless:          false,
		ContextPressureRole:  "gatekeeper",
		AgentHubLayerEquivalent: "AgentHub permission middleware + skill-runtime service layer — " +
			"permission evaluation (permission.go, policy_evaluator.go), hook pipeline (hook.go), " +
			"tool orchestration (toolorchestrator.go), and subagent spawning (forkedagent.go)",
	},
	{
		Slug:            SubsystemLayerState,
		Label:           "State layer",
		PDFSection:      "§3.3",
		LayerOrder:      4,
		SubtitleInPaper: "context assembly, runtime state, session persistence, CLAUDE.md + memory, sidechain transcripts",
		PrimarySourceDirs: []string{
			"src/state/",
		},
		KeySourceFiles: []string{
			"context.ts",
			"sessionStorage.ts",
			"claudemd.ts",
			"history.ts",
			"conversationRecovery.ts",
		},
		ResponsibilityDomain: "state_and_memory",
		DirectionInDataFlow:  "memoized_state",
		CommunicatesUpTo:     SubsystemLayerSafetyAction,
		CommunicatesDownTo:   SubsystemLayerBackend,
		HasIsolatedSubcontext: false,
		IsStateless:          false,
		ContextPressureRole:  "producer",
		AgentHubLayerEquivalent: "AgentHub session/memory subsystem — session persistence " +
			"(session_persistence_channel.go), CLAUDE.md analog (claudemd_memory_type.go), " +
			"cross-session memory (cross_session_memory.go), and context assembly (context_assembly.go)",
	},
	{
		Slug:            SubsystemLayerBackend,
		Label:           "Backend layer",
		PDFSection:      "§3.3",
		LayerOrder:      5,
		SubtitleInPaper: "execution backends, external resources",
		PrimarySourceDirs: []string{
			"src/tools/",
			"src/remote/",
			"services/mcp/",
		},
		KeySourceFiles: []string{
			"BashTool.tsx",
			"PowerShellTool.tsx",
			"services/mcp/client.ts",
		},
		ResponsibilityDomain: "execution",
		DirectionInDataFlow:  "egress",
		CommunicatesUpTo:     SubsystemLayerState,
		CommunicatesDownTo:   "",
		HasIsolatedSubcontext: false,
		IsStateless:          true,
		ContextPressureRole:  "executor",
		AgentHubLayerEquivalent: "AgentHub tool execution backends — skill-runtime HTTP/SQL/document " +
			"executors, MCP client runtime (agenthub-mcp-client-runtime), shell sandbox, and " +
			"42 concrete tool implementations analogous to src/tools/ subdirectories",
	},
}

// SeedLayeredSubsystemDecompositionCount is the exact number of §3.3 layers
// defined in the seed (matches Figure 3 layer count in the paper).
const SeedLayeredSubsystemDecompositionCount = 5

// SeedLayeredSubsystemDecompositionSlugs lists all five §3.3 layer slugs in
// LayerOrder sequence.
var SeedLayeredSubsystemDecompositionSlugs = []SubsystemLayerSlug{
	SubsystemLayerSurface,
	SubsystemLayerCore,
	SubsystemLayerSafetyAction,
	SubsystemLayerState,
	SubsystemLayerBackend,
}

// AllSubsystemLayers returns a copy of the five §3.3 layer profiles in
// LayerOrder sequence (surface first, backend last).
func AllSubsystemLayers() []SubsystemLayerProfile {
	out := make([]SubsystemLayerProfile, len(seedLayeredSubsystemDecomposition))
	copy(out, seedLayeredSubsystemDecomposition)
	return out
}

// FindSubsystemLayerBySlug returns the SubsystemLayerProfile for the given
// slug and true, or a zero value and false if not found.
func FindSubsystemLayerBySlug(slug SubsystemLayerSlug) (SubsystemLayerProfile, bool) {
	for _, p := range seedLayeredSubsystemDecomposition {
		if p.Slug == slug {
			return p, true
		}
	}
	return SubsystemLayerProfile{}, false
}

// IsValidSubsystemLayerSlug reports whether slug names one of the five §3.3 layers.
func IsValidSubsystemLayerSlug(slug SubsystemLayerSlug) bool {
	_, ok := FindSubsystemLayerBySlug(slug)
	return ok
}

// SubsystemLayerByOrder returns the layer at the given 1-based LayerOrder
// (1=surface … 5=backend), and true, or a zero value and false if order is
// out of [1,5].
func SubsystemLayerByOrder(order int) (SubsystemLayerProfile, bool) {
	for _, p := range seedLayeredSubsystemDecomposition {
		if p.LayerOrder == order {
			return p, true
		}
	}
	return SubsystemLayerProfile{}, false
}

// LayersWithIsolatedSubcontext returns only those layers that can spawn
// an isolated sub-context (i.e. the safety/action layer via subagent spawning).
func LayersWithIsolatedSubcontext() []SubsystemLayerProfile {
	var out []SubsystemLayerProfile
	for _, p := range seedLayeredSubsystemDecomposition {
		if p.HasIsolatedSubcontext {
			out = append(out, p)
		}
	}
	return out
}

// StatefulLayers returns the layers that hold durable mutable state
// (IsStateless == false), i.e. safety/action and state layers.
func StatefulLayers() []SubsystemLayerProfile {
	var out []SubsystemLayerProfile
	for _, p := range seedLayeredSubsystemDecomposition {
		if !p.IsStateless {
			out = append(out, p)
		}
	}
	return out
}

// StatelessLayers returns the layers that hold no durable mutable state of
// their own (IsStateless == true): surface, core, and backend.
func StatelessLayers() []SubsystemLayerProfile {
	var out []SubsystemLayerProfile
	for _, p := range seedLayeredSubsystemDecomposition {
		if p.IsStateless {
			out = append(out, p)
		}
	}
	return out
}

// LayersByResponsibilityDomain returns layers matching the given domain string.
// Valid domain values: "rendering", "orchestration", "safety_and_tools",
// "state_and_memory", "execution".
func LayersByResponsibilityDomain(domain string) []SubsystemLayerProfile {
	var out []SubsystemLayerProfile
	for _, p := range seedLayeredSubsystemDecomposition {
		if p.ResponsibilityDomain == domain {
			out = append(out, p)
		}
	}
	return out
}

// LayersByContextPressureRole returns all layers assigned the given
// ContextPressureRole.
// Valid roles: "consumer", "manager", "gatekeeper", "producer", "executor".
func LayersByContextPressureRole(role string) []SubsystemLayerProfile {
	var out []SubsystemLayerProfile
	for _, p := range seedLayeredSubsystemDecomposition {
		if p.ContextPressureRole == role {
			out = append(out, p)
		}
	}
	return out
}

// DataFlowSpine returns all five layers sorted by LayerOrder, representing
// the left-to-right data-flow spine described in §3.3:
//
//	surface → core → safety/action → state → backend
func DataFlowSpine() []SubsystemLayerProfile {
	return AllSubsystemLayers() // already in LayerOrder sequence
}

// DownstreamNeighbour returns the layer immediately downstream of the given
// layer in the §3.3 data-flow spine, and true — or a zero value and false if
// there is no downstream neighbour (i.e. the given layer is backend).
func DownstreamNeighbour(slug SubsystemLayerSlug) (SubsystemLayerProfile, bool) {
	p, ok := FindSubsystemLayerBySlug(slug)
	if !ok || p.CommunicatesDownTo == "" {
		return SubsystemLayerProfile{}, false
	}
	return FindSubsystemLayerBySlug(p.CommunicatesDownTo)
}

// UpstreamNeighbour returns the layer immediately upstream of the given layer
// in the §3.3 data-flow spine, and true — or a zero value and false if there
// is no upstream neighbour (i.e. the given layer is surface).
func UpstreamNeighbour(slug SubsystemLayerSlug) (SubsystemLayerProfile, bool) {
	p, ok := FindSubsystemLayerBySlug(slug)
	if !ok || p.CommunicatesUpTo == "" {
		return SubsystemLayerProfile{}, false
	}
	return FindSubsystemLayerBySlug(p.CommunicatesUpTo)
}
