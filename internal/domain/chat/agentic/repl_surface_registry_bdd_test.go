package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD-style scenarios for REPLSurfaceRegistry — §3.2 interaction surfaces
// from arXiv:2604.14228v1 ("Dive into Claude Code").
//
// These scenarios validate the behavioral contracts of the four Claude Code
// entry surfaces that all converge on the same queryLoop() agent loop:
//   - interactive_repl: human-facing terminal CLI
//   - headless_ci:      non-interactive `claude -p` for automation
//   - agent_sdk:        programmatic async-generator event stream
//   - ide_extension:    IDE/Desktop/Browser integrations
//
// Core invariant (§3.2): "Claude Code uses a single queryLoop() function
// that executes regardless of surface; only the rendering and user-interaction
// layer varies."

func TestBDD_REPLSurfaceRegistry_SharedQueryLoop(t *testing.T) {
	t.Run("Scenario_AllFourSurfacesShareASingleExecutionCore", func(t *testing.T) {
		// Given the §3.2 REPL surface registry representing all entry points,
		reg := NewREPLSurfaceRegistry()

		// When retrieving all registered surfaces,
		all := reg.AllSurfaces()

		// Then there are exactly four surfaces — the same number identified
		// in Figure 3 and §3.2 as feeding into the shared queryLoop().
		// This confirms Claude Code's architectural decision: one engine,
		// multiple rendering surfaces.
		assert.Len(t, all, 4,
			"§3.2 identifies exactly four surfaces that share the queryLoop() core")
		for _, p := range all {
			assert.NotEmpty(t, p.SurfaceID,
				"every surface must have a non-empty SurfaceID")
			assert.Equal(t, "3.2", p.PDFSection,
				"all surfaces originate from §3.2 of the paper")
		}
	})

	t.Run("Scenario_InteractiveREPLIsTheDefaultHumanEntryPoint", func(t *testing.T) {
		// Given a developer launching Claude Code from a terminal,
		reg := NewREPLSurfaceRegistry()

		// When identifying the default surface,
		def := reg.DefaultSurface()

		// Then the interactive REPL is returned — it is the primary human-facing
		// entry point described in §3.2. Exactly one surface holds IsDefault.
		require.NotNil(t, def, "a default surface must exist")
		assert.Equal(t, REPLSurfaceInteractive, def.SurfaceID,
			"interactive_repl must be the §3.2 default surface")
		assert.True(t, def.IsInteractive,
			"the default surface must be interactive")
		assert.True(t, def.SupportsTTY,
			"the default surface must support TTY for terminal UI rendering")
		assert.True(t, ExactlyOneDefault(),
			"structural invariant: exactly one surface may be the default")
	})
}

func TestBDD_REPLSurfaceRegistry_HeadlessSurfaceContracts(t *testing.T) {
	t.Run("Scenario_HeadlessSurfacesCannotHaveTTY", func(t *testing.T) {
		// Given the §3.2 surface registry,
		reg := NewREPLSurfaceRegistry()

		// When enumerating headless surfaces (headless_ci, agent_sdk),
		headless := reg.HeadlessSurfaces()

		// Then none of them may have SupportsTTY == true — headless surfaces
		// run in environments without an attached terminal device.
		// §3.3: the headless CLI creates a QueryEngine for single-shot processing,
		// not a terminal UI. This invariant is load-bearing for permission escalation:
		// without a TTY the surface cannot display inline approval dialogs.
		require.NotEmpty(t, headless, "at least one headless surface must exist")
		for _, p := range headless {
			assert.False(t, p.SupportsTTY,
				"headless surface %q must not require a TTY", p.SurfaceID)
			assert.False(t, p.IsInteractive,
				"headless surface %q must not be interactive", p.SurfaceID)
		}
		assert.True(t, HeadlessSurfacesLackTTY(),
			"structural invariant HeadlessSurfacesLackTTY must hold")
	})

	t.Run("Scenario_HeadlessCIUsesBypassPermissionsMode", func(t *testing.T) {
		// Given a CI pipeline invoking `claude -p` for automated processing,
		reg := NewREPLSurfaceRegistry()

		// When retrieving the headless_ci surface,
		p, ok := reg.FindSurfaceByID(REPLSurfaceHeadlessCI)

		// Then its DefaultPermissionMode is bypassPermissions — CI environments
		// cannot present interactive approval dialogs; permissions must be
		// pre-approved via CLI args or disk settings (§9.2: resume does not
		// restore session-scoped permissions, so they must be re-granted).
		require.True(t, ok, "headless_ci surface must exist")
		require.NotNil(t, p)
		assert.Equal(t, "bypassPermissions", p.DefaultPermissionMode,
			"headless_ci must default to bypassPermissions for automated pipelines")
	})
}

func TestBDD_REPLSurfaceRegistry_AgentSDKEventStreaming(t *testing.T) {
	t.Run("Scenario_AgentSDKEmitsTypedEventsViaSSE", func(t *testing.T) {
		// Given an application consuming Claude Code via the Agent SDK,
		reg := NewREPLSurfaceRegistry()

		// When retrieving the agent_sdk surface profile,
		p, ok := reg.FindSurfaceByID(REPLSurfaceAgentSDK)

		// Then the surface supports SSE-style event emission and uses bubble mode —
		// §3.2: "The Agent SDK emits typed events via async generators."
		// §8.2 and §5: async agents that cannot display prompts escalate via
		// bubble mode to the parent terminal.
		require.True(t, ok, "agent_sdk surface must exist")
		require.NotNil(t, p)
		assert.True(t, p.SupportsSSE,
			"agent_sdk must support SSE/async-generator event emission")
		assert.True(t, p.IsHeadless,
			"agent_sdk is a programmatic surface — not interactive")
		assert.Equal(t, "bubble", p.DefaultPermissionMode,
			"agent_sdk must use bubble mode to escalate permission prompts")
		assert.False(t, p.IsDefault,
			"agent_sdk must not be the default surface")
	})

	t.Run("Scenario_SDKSurfaceCanBeFoundByID", func(t *testing.T) {
		// Given the registry and a programmatic lookup by canonical ID,
		reg := NewREPLSurfaceRegistry()

		// When calling FindSurfaceByID with the agent_sdk constant,
		p, ok := reg.FindSurfaceByID(REPLSurfaceAgentSDK)

		// Then the lookup succeeds and the returned profile is self-consistent.
		require.True(t, ok)
		require.NotNil(t, p)
		assert.Equal(t, REPLSurfaceAgentSDK, p.SurfaceID)
		assert.True(t, reg.IsValidSurfaceID(p.SurfaceID),
			"IsValidSurfaceID must confirm the returned SurfaceID")
	})
}

func TestBDD_REPLSurfaceRegistry_IDEExtensionProfile(t *testing.T) {
	t.Run("Scenario_IDEExtensionIsInteractiveWithSSETransport", func(t *testing.T) {
		// Given a developer using Claude Code through an IDE or browser,
		reg := NewREPLSurfaceRegistry()

		// When retrieving the ide_extension surface,
		p, ok := reg.FindSurfaceByID(REPLSurfaceIDEExtension)

		// Then the surface is interactive (human in the loop), supports SSE
		// transport (§3.3: MCP transport variants include SSE, HTTP, WebSocket),
		// but does not require a TTY — it uses IDE-native rendering, not ink.
		require.True(t, ok, "ide_extension surface must exist")
		require.NotNil(t, p)
		assert.True(t, p.IsInteractive,
			"ide_extension is interactive — a human developer drives the session")
		assert.True(t, p.SupportsSSE,
			"ide_extension supports SSE/HTTP/WebSocket transports per §3.3")
		assert.False(t, p.SupportsTTY,
			"ide_extension does not use a TTY — it has its own rendering layer")
		assert.False(t, p.IsHeadless,
			"ide_extension is not headless — a human is present")
	})
}

func TestBDD_REPLSurfaceRegistry_PermissionModeDistribution(t *testing.T) {
	t.Run("Scenario_EachSurfaceHasADistinctPermissionContext", func(t *testing.T) {
		// Given the four §3.2 surfaces with their default permission modes,
		reg := NewREPLSurfaceRegistry()

		// When grouping surfaces by DefaultPermissionMode,
		defaultMode := reg.SurfacesByPermissionMode("default")
		bubbleMode := reg.SurfacesByPermissionMode("bubble")
		bypassMode := reg.SurfacesByPermissionMode("bypassPermissions")

		// Then the distribution reflects §5's permission mode spectrum and
		// §8.2's async-agent escalation design:
		//   - "default" surfaces (interactive_repl, ide_extension): deny-first
		//     with human approval dialogs per §5.
		//   - "bubble" surface (agent_sdk): escalates prompts to parent terminal
		//     without blocking the SDK consumer (§8.2).
		//   - "bypassPermissions" surface (headless_ci): pre-approved for
		//     automated pipelines; cannot surface interactive prompts.
		assert.Len(t, defaultMode, 2,
			"interactive_repl and ide_extension both default to 'default' mode")
		assert.Len(t, bubbleMode, 1,
			"agent_sdk alone uses bubble mode for permission escalation")
		assert.Len(t, bypassMode, 1,
			"headless_ci alone uses bypassPermissions for automated pipelines")

		// Total must equal all registered surfaces.
		total := len(defaultMode) + len(bubbleMode) + len(bypassMode)
		assert.Equal(t, reg.Count(), total,
			"permission mode groups must partition all registered surfaces")
	})
}

func TestBDD_REPLSurfaceRegistry_StructuralInvariants(t *testing.T) {
	t.Run("Scenario_AllThreeStructuralInvariantsMustHold", func(t *testing.T) {
		// Given the §3.2 surface registry,
		// (no additional setup needed — invariants operate on package-level seed)

		// When checking all three structural invariants simultaneously,
		exactlyOneDefault := ExactlyOneDefault()
		headlessLackTTY := HeadlessSurfacesLackTTY()
		interactiveHaveTTY := InteractiveSurfacesHaveTTY()

		// Then all three must pass — they encode the core architectural contracts
		// of §3.2:
		//   - ExactlyOneDefault: one unambiguous primary entry point.
		//   - HeadlessSurfacesLackTTY: headless = no TTY, no inline dialogs.
		//   - InteractiveSurfacesHaveTTY: at least the interactive_repl has a TTY.
		assert.True(t, exactlyOneDefault,
			"ExactlyOneDefault must hold: interactive_repl is the unique default")
		assert.True(t, headlessLackTTY,
			"HeadlessSurfacesLackTTY must hold: no headless surface may require a TTY")
		assert.True(t, interactiveHaveTTY,
			"InteractiveSurfacesHaveTTY must hold: interactive_repl supports TTY")
	})

	t.Run("Scenario_InteractiveAndHeadleseSetsAreDisjoint", func(t *testing.T) {
		// Given the surface registry,
		reg := NewREPLSurfaceRegistry()

		// When collecting interactive and headless surface IDs,
		interactiveIDs := make(map[REPLSurface]bool)
		for _, p := range reg.InteractiveSurfaces() {
			interactiveIDs[p.SurfaceID] = true
		}
		headlessIDs := make(map[REPLSurface]bool)
		for _, p := range reg.HeadlessSurfaces() {
			headlessIDs[p.SurfaceID] = true
		}

		// Then the two sets are disjoint — a surface cannot be both interactive
		// and headless simultaneously (mutual exclusion encoded in the profiles).
		for id := range interactiveIDs {
			assert.False(t, headlessIDs[id],
				"surface %q appears in both interactive and headless sets — violation", id)
		}

		// And together they cover all four registered surfaces.
		assert.Equal(t, reg.Count(), len(interactiveIDs)+len(headlessIDs),
			"interactive + headless surfaces must cover all registered surfaces")
	})
}
