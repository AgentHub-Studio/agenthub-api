package agentic

// ComparativeDimensionRegistry maps the six architectural comparison dimensions
// from arXiv:2604.14228v1 §10 / Table 3 ("Comparative Analysis: Claude Code and
// OpenClaw").  Each dimension is a recurring design question that both systems
// must answer; the two answers diverge because the deployment contexts differ.
//
// FEAT-022 — §10 Table 3 comparative design dimension registry.
//
// The registry is pure Go, no DB, no HTTP.  It is intended for introspection,
// documentation generation, and test-driven validation of architectural claims.

// ComparativeDimensionID is a stable slug identifying one of the six comparison
// dimensions from Table 3.
type ComparativeDimensionID string

const (
	// DimSystemScope — "What is the runtime scope and deployment model?"
	// Claude Code: ephemeral CLI process per session.
	// OpenClaw:    persistent WebSocket gateway daemon, multi-channel control plane.
	DimSystemScope ComparativeDimensionID = "system_scope"

	// DimTrustModel — "How are trust boundaries and permission policies structured?"
	// Claude Code: deny-first per-action rule evaluation; 7 modes; ML classifier.
	// OpenClaw:    single trusted operator per gateway; DM pairing; opt-in sandboxing.
	DimTrustModel ComparativeDimensionID = "trust_model"

	// DimAgentRuntime — "How is the agentic loop positioned in the architecture?"
	// Claude Code: queryLoop() async generator is the architectural center.
	// OpenClaw:    Pi-agent runner embedded inside gateway dispatch layer.
	DimAgentRuntime ComparativeDimensionID = "agent_runtime"

	// DimExtensionArchitecture — "How is the extension surface structured?"
	// Claude Code: 4 mechanisms at graduated context costs (MCP, plugins, skills, hooks).
	// OpenClaw:    manifest-first plugin system with 12 capability types; central registry.
	DimExtensionArchitecture ComparativeDimensionID = "extension_architecture"

	// DimMemoryAndContext — "How are memory and context managed across turns?"
	// Claude Code: CLAUDE.md 4-level hierarchy; 5-layer compaction pipeline; LLM memory scan.
	// OpenClaw:    workspace bootstrap files + separate memory system (MEMORY.md, daily notes,
	//              dreaming); optional hybrid search (vector + keyword).
	DimMemoryAndContext ComparativeDimensionID = "memory_and_context"

	// DimMultiAgentRouting — "How is multi-agent orchestration and routing structured?"
	// Claude Code: task-delegating subagents (Explore, Plan, general-purpose); worktree
	//              isolation; summary-only results returned to parent.
	// OpenClaw:    two separate concerns: multi-agent routing (isolated agents, distinct
	//              workspaces) and sub-agent delegation (configurable nesting depth ≤ 5).
	DimMultiAgentRouting ComparativeDimensionID = "multi_agent_routing"
)

// ComparativeDimensionCount is the total number of dimensions in Table 3.
const ComparativeDimensionCount = 6

// SystemSlug identifies which system a description belongs to.
type SystemSlug string

const (
	// SystemClaudeCode is Anthropic's production CLI coding agent.
	SystemClaudeCode SystemSlug = "claude_code"
	// SystemOpenClaw is the independent open-source multi-channel personal assistant gateway.
	SystemOpenClaw SystemSlug = "openclaw"
)

// ComparativeDimensionProfile is the full profile for one Table 3 row.
type ComparativeDimensionProfile struct {
	// ID is the stable slug for this dimension.
	ID ComparativeDimensionID

	// Label is the short human-readable column header from Table 3.
	Label string

	// DesignQuestion is the recurring architectural question both systems must answer.
	DesignQuestion string

	// ClaudeCodeAnswer summarises how Claude Code answers the design question.
	ClaudeCodeAnswer string

	// OpenClawAnswer summarises how OpenClaw answers the design question.
	OpenClawAnswer string

	// KeyDivergence is the one-sentence distillation of where the two answers differ most.
	// Drawn from §10.1 and §10.2 of the paper.
	KeyDivergence string

	// ConvergenceNote captures any shared approach noted in §10.2 ("What the Contrast
	// Reveals").  Empty when the systems diverge without common ground.
	ConvergenceNote string
}

// comparativeDimensionProfiles is the canonical registry keyed by ID.
var comparativeDimensionProfiles = map[ComparativeDimensionID]ComparativeDimensionProfile{
	DimSystemScope: {
		ID:    DimSystemScope,
		Label: "System scope",
		DesignQuestion: "What is the runtime scope and deployment model of the agent system?",
		ClaudeCodeAnswer: "CLI/IDE coding harness; ephemeral per-session process bound to a " +
			"single repository directory.",
		OpenClawAnswer: "Persistent WebSocket gateway daemon (default port 18789, " +
			"loopback-only); multi-channel control plane connecting ~two dozen messaging " +
			"surfaces (WhatsApp, Telegram, Slack, Discord, Signal, and others).",
		KeyDivergence: "Claude Code is session-scoped and repository-bound; OpenClaw is a " +
			"persistent daemon that owns all messaging surface connections and coordinates " +
			"clients, tools, and device nodes across a typed WebSocket protocol.",
		ConvergenceNote: "OpenClaw can host Claude Code via ACP integration, making the two " +
			"systems stackable rather than purely alternative.",
	},
	DimTrustModel: {
		ID:    DimTrustModel,
		Label: "Trust model",
		DesignQuestion: "How are trust boundaries, permission policies, and sandboxing structured?",
		ClaudeCodeAnswer: "deny-first per-action rule evaluation with hooks and optional ML " +
			"classifier; 7 permission modes; graduated trust spectrum from plan to " +
			"bypassPermissions.",
		OpenClawAnswer: "Single trusted operator per gateway; DM pairing codes and send " +
			"allowlists for inbound channel access; opt-in sandboxing with configurable " +
			"scope (per-agent, per-session, or shared) and multiple backends (Docker, SSH, " +
			"OpenShell).",
		KeyDivergence: "Claude Code places the trust boundary between the model and the " +
			"execution environment; OpenClaw places it at the gateway perimeter.",
		ConvergenceNote: "",
	},
	DimAgentRuntime: {
		ID:    DimAgentRuntime,
		Label: "Agent runtime",
		DesignQuestion: "How is the agentic loop positioned in the overall architecture?",
		ClaudeCodeAnswer: "Iterative async generator (queryLoop()) is the architectural center; " +
			"all interfaces feed into it; it directly manages context assembly, model calls, " +
			"tool dispatch, and recovery.",
		OpenClawAnswer: "Pi-agent runner embedded inside the gateway dispatch layer; the " +
			"gateway agent RPC validates parameters, resolves sessions, and returns " +
			"immediately; runs are serialized through per-session queues and an optional " +
			"global lane.",
		KeyDivergence: "Both follow the ReAct pattern; but Claude Code's loop is the " +
			"architectural center, while OpenClaw's loop is a component within a gateway " +
			"control plane rather than the control plane itself.",
		ConvergenceNote: "Both systems implement iterative agentic loops following the ReAct pattern.",
	},
	DimExtensionArchitecture: {
		ID:    DimExtensionArchitecture,
		Label: "Extension architecture",
		DesignQuestion: "How is the extension surface structured and at what context cost?",
		ClaudeCodeAnswer: "4 mechanisms at graduated context costs: MCP servers (high — tool " +
			"schemas), plugins (medium), skills (low — descriptions only), hooks (zero by " +
			"default); each extends a single agent's context window and tool surface.",
		OpenClawAnswer: "Manifest-first plugin system with 12 capability types and central " +
			"registry; separate skills layer with multiple sources (workspace, project-level, " +
			"personal, managed, bundled, extra) plus public ClawHub registry; built-in MCP " +
			"via openclaw mcp (server and outbound client registry).",
		KeyDivergence: "Claude Code's extensions modify one agent's action surface; OpenClaw's " +
			"plugins extend the gateway's capability surface across all agents.",
		ConvergenceNote: "",
	},
	DimMemoryAndContext: {
		ID:    DimMemoryAndContext,
		Label: "Memory and context",
		DesignQuestion: "How are memory and context assembled, compressed, and persisted?",
		ClaudeCodeAnswer: "CLAUDE.md 4-level hierarchy loaded at startup; 5-layer compaction " +
			"pipeline (budget-reduction, snip, microcompact, compact, full-compact) with " +
			"cache-aware behavior; LLM-based memory scan of file headers.",
		OpenClawAnswer: "Workspace bootstrap files (AGENTS.md, SOUL.md, TOOLS.md, IDENTITY.md, " +
			"USER.md plus conditional BOOTSTRAP.md, HEARTBEAT.md, MEM-ORY.md); separate " +
			"memory system (MEMORY.md, daily notes, optional DREAMS.md); auto-compaction " +
			"with pluggable providers; optional hybrid search (vector + keyword) when " +
			"embedding provider configured; experimental dreaming for long-term promotion.",
		KeyDivergence: "Claude Code invests more in graduated context compression (5 layers " +
			"with cache awareness); OpenClaw invests more in structured long-term memory " +
			"promotion (dreaming, daily notes, memory search).",
		ConvergenceNote: "Both systems use transparent file-based memory rather than opaque " +
			"databases.",
	},
	DimMultiAgentRouting: {
		ID:    DimMultiAgentRouting,
		Label: "Multi-agent and routing",
		DesignQuestion: "How is multi-agent orchestration, delegation, and routing structured?",
		ClaudeCodeAnswer: "Task-delegating subagents (Explore, Plan, general-purpose, custom " +
			"types) operate in isolated context windows with restricted tool sets and return " +
			"summary-only results; worktree isolation provides filesystem-level separation.",
		OpenClawAnswer: "Two separate concerns: (a) multi-agent routing with isolated agents, " +
			"distinct workspaces, authentication profiles, session store, and model " +
			"configuration routed via deterministic binding rules; (b) sub-agent delegation " +
			"with configurable nesting depth (maximum 5, default 1, recommended 2) and " +
			"thread-bound sessions on supported channels.",
		KeyDivergence: "Claude Code's subagents are subordinate workers within one user's " +
			"coding session; OpenClaw's multi-agent routing creates genuinely independent " +
			"agent instances serving different users or purposes through different channels.",
		ConvergenceNote: "",
	},
}

// NewComparativeDimensionRegistry returns all six Table 3 profiles in stable
// dimension order (matching the table row order in the paper).
func NewComparativeDimensionRegistry() []ComparativeDimensionProfile {
	order := []ComparativeDimensionID{
		DimSystemScope,
		DimTrustModel,
		DimAgentRuntime,
		DimExtensionArchitecture,
		DimMemoryAndContext,
		DimMultiAgentRouting,
	}
	out := make([]ComparativeDimensionProfile, 0, len(order))
	for _, id := range order {
		out = append(out, comparativeDimensionProfiles[id])
	}
	return out
}

// FindComparativeDimensionBySlug returns the profile for the given ID, or
// (zero-value, false) when not found.
func FindComparativeDimensionBySlug(id ComparativeDimensionID) (ComparativeDimensionProfile, bool) {
	p, ok := comparativeDimensionProfiles[id]
	return p, ok
}

// SystemAnswer returns the answer string for the requested system, or an empty
// string when the system slug is unrecognised.
func (p ComparativeDimensionProfile) SystemAnswer(sys SystemSlug) string {
	switch sys {
	case SystemClaudeCode:
		return p.ClaudeCodeAnswer
	case SystemOpenClaw:
		return p.OpenClawAnswer
	default:
		return ""
	}
}

// HasConvergence reports whether the two systems share any common ground on this
// dimension (i.e. ConvergenceNote is non-empty).
func (p ComparativeDimensionProfile) HasConvergence() bool {
	return p.ConvergenceNote != ""
}
