package agentic

// REPLSurfaceRegistry models the interaction surfaces described in
// arXiv:2604.14228v1 §3.2 ("High-Level System Structure") and Figure 3
// ("Expanded Layered Architecture"), as the "Surface Layer" entry points that
// all converge on the same shared queryLoop() agent loop.
//
// The paper enumerates four concrete surfaces in the Surface Layer (Figure 3,
// §3.2, §3.3):
//
//   - interactive_repl  — the Interactive CLI launched by `claude` with a live
//     terminal UI (ink framework, real-time streaming, permission dialogs).
//   - headless_ci       — the Headless CLI launched by `claude -p` that creates
//     a QueryEngine instance for single-shot non-interactive processing.
//   - agent_sdk         — the Agent SDK path that emits typed events via async
//     generators, consumed programmatically (no terminal UI).
//   - ide_extension     — the IDE/Desktop/Browser integration surface, routing
//     requests through the same queryLoop() with surface-specific rendering.
//
// The key architectural invariant (§3.2, §3.4): "Claude Code uses a single
// queryLoop() function that executes regardless of whether the user is
// interacting through an interactive terminal CLI invocation, a headless CLI
// invocation, the Agent SDK, or an IDE integration (query.ts). Only the
// rendering and user-interaction layer varies."
//
// Design principle alignment (Table 1):
//   - "Deny-first with human escalation" (§5, §8, §9) — interactive surfaces
//     prompt the user; headless/SDK surfaces suppress prompts (bubble mode,
//     async escalation).
//   - "Append-only durable state" (§4, §9) — all surfaces write to the same
//     append-only JSONL transcript via sessionStorage.ts.
//   - "Minimal scaffolding, maximal operational harness" (§3, §4) — a single
//     shared queryLoop() rather than mode-specific execution engines.
//
// Relationships to neighbouring constructs:
//   - permission_bubble_mode.go models the bubble-mode escalation path that
//     headless_ci and agent_sdk surfaces rely on when a permission prompt
//     cannot reach an interactive terminal.
//   - session_persistence_channel.go (§9) records durable state identically
//     across all four surfaces — surface identity does not change persistence.
//   - shell_sandbox.go applies the same sandboxing regardless of surface,
//     consistent with the shared queryLoop() architectural choice.

// REPLSurface is the type-safe identifier for a Claude Code interaction surface
// from arXiv:2604.14228v1 §3.2.
type REPLSurface string

const (
	// REPLSurfaceInteractive is the interactive terminal CLI surface.
	// Launched by the bare `claude` command; hosts the ink-based terminal UI with
	// real-time streaming output, permission dialogs, and progress indicators.
	// §3.2: "The interactive CLI launches a terminal UI with real-time streaming,
	// permission dialogs, and progress indicators."
	REPLSurfaceInteractive REPLSurface = "interactive_repl"

	// REPLSurfaceHeadlessCI is the headless CLI surface launched by `claude -p`.
	// Creates a QueryEngine instance for single-shot non-interactive processing.
	// No terminal UI; designed for CI/CD pipelines and scripted automation.
	// §3.2: "The headless CLI (claude -p) creates a QueryEngine instance for
	// single-shot processing."
	REPLSurfaceHeadlessCI REPLSurface = "headless_ci"

	// REPLSurfaceAgentSDK is the Agent SDK surface.
	// Emits typed events (StreamEvent, RequestStartEvent, etc.) via async
	// generators for programmatic consumption. No terminal UI rendering.
	// §3.2: "The Agent SDK emits typed events via async generators."
	REPLSurfaceAgentSDK REPLSurface = "agent_sdk"

	// REPLSurfaceIDEExtension is the IDE, desktop application, and browser
	// integration surface. Routes through the same queryLoop() but with
	// IDE-specific adapters and rendering.
	// §3.2: "IDE/Desktop/Browser" as a distinct entry point in Figure 3.
	REPLSurfaceIDEExtension REPLSurface = "ide_extension"
)

// REPLSurfaceProfile is the immutable §3.2 metadata for one Claude Code
// interaction surface from arXiv:2604.14228v1.
type REPLSurfaceProfile struct {
	// SurfaceID is the canonical identifier for this interaction surface.
	SurfaceID REPLSurface

	// Label is the short human-readable name for this surface.
	Label string

	// Description explains the surface's role and distinguishing characteristics
	// in the Claude Code architecture.
	Description string

	// PDFSection cites the paper section(s) that describe this surface.
	PDFSection string

	// IsInteractive is true when the surface supports direct human interaction
	// in real time (approval dialogs, progress indicators, streaming output to a
	// human-visible terminal). True only for interactive_repl.
	IsInteractive bool

	// IsHeadless is true when the surface is designed for non-interactive,
	// programmatic or automated use without a human watching in real time.
	// True for headless_ci and agent_sdk.
	IsHeadless bool

	// SupportsTTY is true when the surface is launched in a terminal context
	// with a real TTY device attached. Per §3.2 and §3.3 (surface layer),
	// the interactive CLI requires a TTY; headless and SDK surfaces do not.
	SupportsTTY bool

	// SupportsSSE is true when the surface can emit Server-Sent Events or
	// equivalent streaming event protocols. The Agent SDK emits typed async
	// generator events; the headless CLI pipes stdout.
	// Per §3.2: the backend layer supports "SSE, HTTP, WebSocket" transports.
	SupportsSSE bool

	// DefaultPermissionMode is the permission mode baseline for this surface.
	// Interactive REPL defaults to "default" (deny-first with human escalation).
	// Headless and SDK surfaces default to bubble mode, which escalates prompts
	// to the parent terminal rather than showing inline dialogs.
	// §5 and §8.2: async agents use bubble mode by default.
	DefaultPermissionMode string

	// TypicalUserContext describes who or what typically drives this surface
	// (human developer, CI bot, IDE plugin, etc.).
	TypicalUserContext string

	// IsDefault is true when this surface is the primary entry point for the
	// majority of human-initiated Claude Code sessions. Exactly one surface is
	// the default: interactive_repl.
	IsDefault bool
}

// replSurfaceProfiles is the package-level seed for all four §3.2 surfaces.
var replSurfaceProfiles = []*REPLSurfaceProfile{
	{
		SurfaceID:             REPLSurfaceInteractive,
		Label:                 "Interactive REPL",
		Description:           "The primary human-facing terminal surface. Launched by `claude` with a TTY attached; renders a real-time streaming terminal UI via the ink framework, shows permission approval dialogs inline, and displays progress indicators. This is the default entry point for most developer sessions. The interactive CLI calls query() directly, bypassing QueryEngine — the shared code path is the loop function, not the engine class (§3.4).",
		PDFSection:            "3.2",
		IsInteractive:         true,
		IsHeadless:            false,
		SupportsTTY:           true,
		SupportsSSE:           false,
		DefaultPermissionMode: "default",
		TypicalUserContext:    "Human software developer in a terminal session",
		IsDefault:             true,
	},
	{
		SurfaceID:             REPLSurfaceHeadlessCI,
		Label:                 "Headless CLI",
		Description:           "The non-interactive CLI surface launched by `claude -p`. Creates a QueryEngine instance for single-shot processing without a terminal UI. Designed for CI/CD pipelines, scripted automation, and batch processing. Session-scoped permissions are not restored on resume (§9.2); requests must grant permissions programmatically via CLI args and disk settings.",
		PDFSection:            "3.2",
		IsInteractive:         false,
		IsHeadless:            true,
		SupportsTTY:           false,
		SupportsSSE:           false,
		DefaultPermissionMode: "bypassPermissions",
		TypicalUserContext:    "CI/CD pipeline, shell script, or automated toolchain",
		IsDefault:             false,
	},
	{
		SurfaceID:             REPLSurfaceAgentSDK,
		Label:                 "Agent SDK",
		Description:           "The programmatic SDK surface. Emits typed events (StreamEvent, RequestStartEvent, TombstoneMessage, ToolUseSummaryMessage) via async generators for consumption by host applications. No terminal rendering. Permission prompts are escalated via bubble mode to the parent process. Enables composable multi-agent systems where Claude Code acts as a component within a larger orchestration layer.",
		PDFSection:            "3.2",
		IsInteractive:         false,
		IsHeadless:            true,
		SupportsTTY:           false,
		SupportsSSE:           true,
		DefaultPermissionMode: "bubble",
		TypicalUserContext:    "SDK consumer application, orchestration framework, or agent harness",
		IsDefault:             false,
	},
	{
		SurfaceID:             REPLSurfaceIDEExtension,
		Label:                 "IDE / Desktop / Browser",
		Description:           "The IDE, desktop application, and web browser integration surface. Routes requests through the same queryLoop() as all other surfaces but uses IDE-specific adapters and rendering. Listed as a distinct entry point in Figure 3 alongside Interactive CLI and Headless CLI. Supports MCP transport variants including SSE, HTTP, and WebSocket (§3.3 backend layer). The surface-specific adapter layer handles rendering while the shared loop handles execution.",
		PDFSection:            "3.2",
		IsInteractive:         true,
		IsHeadless:            false,
		SupportsTTY:           false,
		SupportsSSE:           true,
		DefaultPermissionMode: "default",
		TypicalUserContext:    "Developer using Cursor, VS Code extension, Claude.ai web, or desktop app",
		IsDefault:             false,
	},
}

// SeedREPLSurfaceCount is the number of §3.2 interaction surfaces.
const SeedREPLSurfaceCount = 4

// replSurfaceByID is the fast-lookup index built at init time.
var replSurfaceByID map[REPLSurface]*REPLSurfaceProfile

func init() {
	replSurfaceByID = make(map[REPLSurface]*REPLSurfaceProfile, SeedREPLSurfaceCount)
	for _, p := range replSurfaceProfiles {
		replSurfaceByID[p.SurfaceID] = p
	}
}

// REPLSurfaceRegistry provides §3.2 queries over the four Claude Code
// interaction surfaces from arXiv:2604.14228v1.
//
// All four surfaces share the same queryLoop() agent loop; only the
// rendering and user-interaction layer varies between them (§3.2).
// The registry is read-only; callers must not modify the returned profiles.
type REPLSurfaceRegistry struct{}

// NewREPLSurfaceRegistry returns a ready-to-use registry of §3.2 surfaces.
func NewREPLSurfaceRegistry() *REPLSurfaceRegistry {
	return &REPLSurfaceRegistry{}
}

// FindSurfaceByID looks up a surface profile by its canonical ID.
// Returns (profile, true) if found; (nil, false) if unknown.
func (r *REPLSurfaceRegistry) FindSurfaceByID(id REPLSurface) (*REPLSurfaceProfile, bool) {
	p, ok := replSurfaceByID[id]
	return p, ok
}

// AllSurfaces returns all four §3.2 surface profiles.
// Returns a defensive copy of the slice; element order matches the seed order
// (interactive_repl first).
func (r *REPLSurfaceRegistry) AllSurfaces() []*REPLSurfaceProfile {
	result := make([]*REPLSurfaceProfile, len(replSurfaceProfiles))
	copy(result, replSurfaceProfiles)
	return result
}

// Count returns the number of registered §3.2 surfaces.
func (r *REPLSurfaceRegistry) Count() int {
	return len(replSurfaceProfiles)
}

// IsValidSurfaceID returns true if id is one of the four canonical §3.2
// surface identifiers.
func (r *REPLSurfaceRegistry) IsValidSurfaceID(id REPLSurface) bool {
	_, ok := replSurfaceByID[id]
	return ok
}

// InteractiveSurfaces returns all surfaces where IsInteractive == true.
// §3.2: interactive_repl and ide_extension interact with a human in real time.
func (r *REPLSurfaceRegistry) InteractiveSurfaces() []*REPLSurfaceProfile {
	var result []*REPLSurfaceProfile
	for _, p := range replSurfaceProfiles {
		if p.IsInteractive {
			result = append(result, p)
		}
	}
	return result
}

// HeadlessSurfaces returns all surfaces where IsHeadless == true.
// §3.2: headless_ci and agent_sdk are designed for non-interactive use.
func (r *REPLSurfaceRegistry) HeadlessSurfaces() []*REPLSurfaceProfile {
	var result []*REPLSurfaceProfile
	for _, p := range replSurfaceProfiles {
		if p.IsHeadless {
			result = append(result, p)
		}
	}
	return result
}

// SurfacesWithSSE returns all surfaces where SupportsSSE == true.
// §3.3 (backend layer): SSE, HTTP, and WebSocket transports apply to
// agent_sdk and ide_extension surfaces.
func (r *REPLSurfaceRegistry) SurfacesWithSSE() []*REPLSurfaceProfile {
	var result []*REPLSurfaceProfile
	for _, p := range replSurfaceProfiles {
		if p.SupportsSSE {
			result = append(result, p)
		}
	}
	return result
}

// SurfacesByPermissionMode returns all surfaces whose DefaultPermissionMode
// matches the given mode string.
func (r *REPLSurfaceRegistry) SurfacesByPermissionMode(mode string) []*REPLSurfaceProfile {
	var result []*REPLSurfaceProfile
	for _, p := range replSurfaceProfiles {
		if p.DefaultPermissionMode == mode {
			result = append(result, p)
		}
	}
	return result
}

// DefaultSurface returns the surface marked IsDefault == true.
// §3.2: the interactive_repl is the primary entry point for human-initiated
// developer sessions. Exactly one surface is the default.
func (r *REPLSurfaceRegistry) DefaultSurface() *REPLSurfaceProfile {
	for _, p := range replSurfaceProfiles {
		if p.IsDefault {
			return p
		}
	}
	return nil
}

// --- Structural invariants ---

// ExactlyOneDefault validates that exactly one §3.2 surface has IsDefault == true.
// §3.2: the interactive_repl is unambiguously the primary entry point.
func ExactlyOneDefault() bool {
	count := 0
	for _, p := range replSurfaceProfiles {
		if p.IsDefault {
			count++
		}
	}
	return count == 1
}

// HeadlessSurfacesLackTTY validates that every surface with IsHeadless == true
// also has SupportsTTY == false.
// §3.2 and §3.3: headless surfaces (headless_ci, agent_sdk) are launched
// without an attached TTY — they cannot display interactive terminal UI.
func HeadlessSurfacesLackTTY() bool {
	for _, p := range replSurfaceProfiles {
		if p.IsHeadless && p.SupportsTTY {
			return false
		}
	}
	return true
}

// InteractiveSurfacesHaveTTY validates that at least one surface with
// IsInteractive == true also has SupportsTTY == true.
// §3.2: the interactive_repl is fundamentally a TTY-based terminal surface.
// The ide_extension is interactive but uses a non-TTY rendering channel.
func InteractiveSurfacesHaveTTY() bool {
	for _, p := range replSurfaceProfiles {
		if p.IsInteractive && p.SupportsTTY {
			return true
		}
	}
	return false
}
