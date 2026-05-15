package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for REPLSurfaceRegistry — §3.2 interaction surfaces from
// arXiv:2604.14228v1.

func TestREPLSurfaceRegistry_NewReturnsNonNil(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	assert.NotNil(t, r, "NewREPLSurfaceRegistry must return a non-nil registry")
}

func TestREPLSurfaceRegistry_Count(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	assert.Equal(t, SeedREPLSurfaceCount, r.Count(),
		"registry must contain exactly SeedREPLSurfaceCount surfaces")
}

func TestREPLSurfaceRegistry_SeedConstantIs4(t *testing.T) {
	assert.Equal(t, 4, SeedREPLSurfaceCount,
		"§3.2 defines exactly four surfaces: interactive_repl, headless_ci, agent_sdk, ide_extension")
}

func TestREPLSurfaceRegistry_FindSurfaceByID_AllKnownSurfaces(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	known := []REPLSurface{
		REPLSurfaceInteractive,
		REPLSurfaceHeadlessCI,
		REPLSurfaceAgentSDK,
		REPLSurfaceIDEExtension,
	}
	for _, id := range known {
		p, ok := r.FindSurfaceByID(id)
		assert.True(t, ok, "FindSurfaceByID must find registered surface %q", id)
		require.NotNil(t, p, "profile must be non-nil for %q", id)
		assert.Equal(t, id, p.SurfaceID, "SurfaceID must match query key for %q", id)
	}
}

func TestREPLSurfaceRegistry_FindSurfaceByID_UnknownReturnsNilFalse(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	p, ok := r.FindSurfaceByID("nonexistent_surface")
	assert.False(t, ok, "FindSurfaceByID must return false for unknown surface")
	assert.Nil(t, p, "profile must be nil for unknown surface")
}

func TestREPLSurfaceRegistry_AllSurfaces_Length(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	all := r.AllSurfaces()
	assert.Len(t, all, SeedREPLSurfaceCount, "AllSurfaces must return all registered surfaces")
}

func TestREPLSurfaceRegistry_AllSurfaces_IsDefensiveCopy(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	a := r.AllSurfaces()
	b := r.AllSurfaces()
	assert.NotSame(t, &a, &b, "AllSurfaces must return a new slice each call")
	a[0] = nil
	c := r.AllSurfaces()
	assert.NotNil(t, c[0], "mutation of returned slice must not affect registry internals")
}

func TestREPLSurfaceRegistry_AllSurfaces_NilEntriesAbsent(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	for i, p := range r.AllSurfaces() {
		assert.NotNil(t, p, "AllSurfaces must not contain nil entries at index %d", i)
	}
}

func TestREPLSurfaceRegistry_IsValidSurfaceID_TrueForKnown(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	assert.True(t, r.IsValidSurfaceID(REPLSurfaceInteractive))
	assert.True(t, r.IsValidSurfaceID(REPLSurfaceHeadlessCI))
	assert.True(t, r.IsValidSurfaceID(REPLSurfaceAgentSDK))
	assert.True(t, r.IsValidSurfaceID(REPLSurfaceIDEExtension))
}

func TestREPLSurfaceRegistry_IsValidSurfaceID_FalseForUnknown(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	assert.False(t, r.IsValidSurfaceID("unknown"))
	assert.False(t, r.IsValidSurfaceID(""))
	assert.False(t, r.IsValidSurfaceID("INTERACTIVE_REPL"))
}

func TestREPLSurfaceRegistry_InteractiveSurfaces_ContainsInteractiveAndIDE(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	interactive := r.InteractiveSurfaces()
	require.Len(t, interactive, 2,
		"§3.2 defines two interactive surfaces: interactive_repl and ide_extension")
	ids := make(map[REPLSurface]bool)
	for _, p := range interactive {
		ids[p.SurfaceID] = true
	}
	assert.True(t, ids[REPLSurfaceInteractive],
		"interactive_repl must be in InteractiveSurfaces()")
	assert.True(t, ids[REPLSurfaceIDEExtension],
		"ide_extension must be in InteractiveSurfaces()")
}

func TestREPLSurfaceRegistry_HeadlessSurfaces_ContainsCIAndSDK(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	headless := r.HeadlessSurfaces()
	require.Len(t, headless, 2,
		"§3.2 defines two headless surfaces: headless_ci and agent_sdk")
	ids := make(map[REPLSurface]bool)
	for _, p := range headless {
		ids[p.SurfaceID] = true
	}
	assert.True(t, ids[REPLSurfaceHeadlessCI],
		"headless_ci must be in HeadlessSurfaces()")
	assert.True(t, ids[REPLSurfaceAgentSDK],
		"agent_sdk must be in HeadlessSurfaces()")
}

func TestREPLSurfaceRegistry_SurfacesWithSSE_ContainsSDKAndIDE(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	sse := r.SurfacesWithSSE()
	require.Len(t, sse, 2,
		"§3.3 backend layer: agent_sdk and ide_extension support SSE/streaming events")
	ids := make(map[REPLSurface]bool)
	for _, p := range sse {
		ids[p.SurfaceID] = true
	}
	assert.True(t, ids[REPLSurfaceAgentSDK], "agent_sdk must support SSE")
	assert.True(t, ids[REPLSurfaceIDEExtension], "ide_extension must support SSE")
}

func TestREPLSurfaceRegistry_SurfacesByPermissionMode_Default(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	defaults := r.SurfacesByPermissionMode("default")
	require.Len(t, defaults, 2,
		"interactive_repl and ide_extension default to the 'default' permission mode")
	ids := make(map[REPLSurface]bool)
	for _, p := range defaults {
		ids[p.SurfaceID] = true
	}
	assert.True(t, ids[REPLSurfaceInteractive])
	assert.True(t, ids[REPLSurfaceIDEExtension])
}

func TestREPLSurfaceRegistry_SurfacesByPermissionMode_Bubble(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	bubble := r.SurfacesByPermissionMode("bubble")
	require.Len(t, bubble, 1, "agent_sdk uses bubble mode for permission escalation")
	assert.Equal(t, REPLSurfaceAgentSDK, bubble[0].SurfaceID)
}

func TestREPLSurfaceRegistry_SurfacesByPermissionMode_BypassPermissions(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	bypass := r.SurfacesByPermissionMode("bypassPermissions")
	require.Len(t, bypass, 1, "headless_ci defaults to bypassPermissions mode")
	assert.Equal(t, REPLSurfaceHeadlessCI, bypass[0].SurfaceID)
}

func TestREPLSurfaceRegistry_SurfacesByPermissionMode_Unknown(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	results := r.SurfacesByPermissionMode("nonexistent_mode")
	assert.Empty(t, results, "unknown permission mode must yield empty result")
}

func TestREPLSurfaceRegistry_DefaultSurface_IsInteractiveREPL(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	def := r.DefaultSurface()
	require.NotNil(t, def, "DefaultSurface must return a non-nil profile")
	assert.Equal(t, REPLSurfaceInteractive, def.SurfaceID,
		"§3.2: interactive_repl is the primary entry point for developer sessions")
}

func TestREPLSurfaceRegistry_DefaultSurface_HasTTY(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	def := r.DefaultSurface()
	require.NotNil(t, def)
	assert.True(t, def.SupportsTTY,
		"the default interactive_repl surface must have a TTY")
}

func TestREPLSurfaceRegistry_InvariantExactlyOneDefault(t *testing.T) {
	assert.True(t, ExactlyOneDefault(),
		"structural invariant ExactlyOneDefault must hold: exactly one surface is the default")
}

func TestREPLSurfaceRegistry_InvariantHeadlessSurfacesLackTTY(t *testing.T) {
	assert.True(t, HeadlessSurfacesLackTTY(),
		"structural invariant HeadlessSurfacesLackTTY must hold: no headless surface has a TTY")
}

func TestREPLSurfaceRegistry_InvariantInteractiveSurfacesHaveTTY(t *testing.T) {
	assert.True(t, InteractiveSurfacesHaveTTY(),
		"structural invariant InteractiveSurfacesHaveTTY must hold: at least one interactive surface has a TTY")
}

func TestREPLSurfaceRegistry_InteractiveSurfaceNotHeadless(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	p, ok := r.FindSurfaceByID(REPLSurfaceInteractive)
	require.True(t, ok)
	assert.False(t, p.IsHeadless, "interactive_repl must not be headless")
	assert.True(t, p.IsInteractive, "interactive_repl must be interactive")
}

func TestREPLSurfaceRegistry_HeadlessCINotInteractive(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	p, ok := r.FindSurfaceByID(REPLSurfaceHeadlessCI)
	require.True(t, ok)
	assert.True(t, p.IsHeadless, "headless_ci must be headless")
	assert.False(t, p.IsInteractive, "headless_ci must not be interactive")
	assert.False(t, p.SupportsTTY, "headless_ci must not support TTY")
}

func TestREPLSurfaceRegistry_AgentSDK_IsHeadlessWithSSE(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	p, ok := r.FindSurfaceByID(REPLSurfaceAgentSDK)
	require.True(t, ok)
	assert.True(t, p.IsHeadless, "agent_sdk must be headless")
	assert.True(t, p.SupportsSSE, "agent_sdk must support SSE event emission")
	assert.False(t, p.SupportsTTY, "agent_sdk must not require a TTY")
}

func TestREPLSurfaceRegistry_IDEExtension_IsInteractiveWithSSE(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	p, ok := r.FindSurfaceByID(REPLSurfaceIDEExtension)
	require.True(t, ok)
	assert.True(t, p.IsInteractive, "ide_extension must be interactive")
	assert.True(t, p.SupportsSSE, "ide_extension must support SSE/HTTP/WebSocket transports")
	assert.False(t, p.SupportsTTY, "ide_extension uses non-TTY rendering channel")
}

func TestREPLSurfaceRegistry_AllSurfacesHaveNonEmptyFields(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	for _, p := range r.AllSurfaces() {
		assert.NotEmpty(t, p.SurfaceID, "SurfaceID must not be empty")
		assert.NotEmpty(t, p.Label, "Label must not be empty for surface %q", p.SurfaceID)
		assert.NotEmpty(t, p.Description, "Description must not be empty for surface %q", p.SurfaceID)
		assert.NotEmpty(t, p.PDFSection, "PDFSection must not be empty for surface %q", p.SurfaceID)
		assert.NotEmpty(t, p.DefaultPermissionMode, "DefaultPermissionMode must not be empty for surface %q", p.SurfaceID)
		assert.NotEmpty(t, p.TypicalUserContext, "TypicalUserContext must not be empty for surface %q", p.SurfaceID)
	}
}

func TestREPLSurfaceRegistry_IsHeadlessAndIsInteractiveMutuallyExclusive(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	for _, p := range r.AllSurfaces() {
		assert.False(t, p.IsHeadless && p.IsInteractive,
			"surface %q cannot be both headless and interactive", p.SurfaceID)
	}
}

func TestREPLSurfaceRegistry_HeadlessSurfaceCount(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	assert.Len(t, r.HeadlessSurfaces(), 2,
		"exactly two surfaces are headless: headless_ci and agent_sdk")
}

func TestREPLSurfaceRegistry_InteractiveSurfaceCount(t *testing.T) {
	r := NewREPLSurfaceRegistry()
	assert.Len(t, r.InteractiveSurfaces(), 2,
		"exactly two surfaces are interactive: interactive_repl and ide_extension")
}
