package agentic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ExtensionOutputStyle(t *testing.T) {
	t.Run("Scenario_ExplicitRequestStyleOverridesAgentTenantPlatform", func(t *testing.T) {
		// Given platform default style + tenant override + agent
		// override + caller-supplied explicit override,
		// When the renderer resolves the active style,
		// Then the explicit caller request wins (highest cascade
		// precedence per PDF §6.5).
		r := NewInMemoryExtensionOutputStyleRegistry()

		platform := validBinding()
		platform.StyleSlug = "platform-default"
		platform.Scope = OutputStyleScopePlatform
		platform.ScopeID = ""
		_, _ = r.Bind(context.Background(), platform)

		tnt := validBinding()
		tnt.StyleSlug = "tenant-default"
		_, _ = r.Bind(context.Background(), tnt)

		ag := validBinding()
		ag.StyleSlug = "agent-style"
		ag.Scope = OutputStyleScopeAgent
		ag.ScopeID = "agent-x"
		_, _ = r.Bind(context.Background(), ag)

		ex := validBinding()
		ex.StyleSlug = "explicit-style"
		ex.Scope = OutputStyleScopeExplicit
		ex.ScopeID = "req-1"
		_, _ = r.Bind(context.Background(), ex)

		res, err := r.Resolve(context.Background(), ResolutionContext{
			TenantID: "t-1", AgentID: "agent-x", ExplicitRequestID: "req-1",
		})
		require.NoError(t, err)
		assert.Equal(t, "explicit-style", res.StyleSlug)
	})

	t.Run("Scenario_AgentDefaultBeatsTenantWhenNoExplicit", func(t *testing.T) {
		// Given no explicit request override,
		// When agent has its own style binding,
		// Then agent default beats tenant default (per-agent override
		// rules).
		r := NewInMemoryExtensionOutputStyleRegistry()
		tnt := validBinding()
		tnt.StyleSlug = "tenant"
		_, _ = r.Bind(context.Background(), tnt)

		ag := validBinding()
		ag.StyleSlug = "agent"
		ag.Scope = OutputStyleScopeAgent
		ag.ScopeID = "agent-x"
		_, _ = r.Bind(context.Background(), ag)

		res, _ := r.Resolve(context.Background(), ResolutionContext{
			TenantID: "t-1", AgentID: "agent-x",
		})
		assert.Equal(t, "agent", res.StyleSlug)
	})

	t.Run("Scenario_PlatformDefaultIsAlwaysFallback", func(t *testing.T) {
		// Given a tenant has no explicit/agent/tenant binding,
		// When the renderer resolves,
		// Then platform default catches the request — no tenant ever
		// has zero rendering options.
		r := NewInMemoryExtensionOutputStyleRegistry()
		p := validBinding()
		p.StyleSlug = "platform-default"
		p.Scope = OutputStyleScopePlatform
		p.ScopeID = ""
		_, _ = r.Bind(context.Background(), p)

		res, _ := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
		assert.Equal(t, "platform-default", res.StyleSlug)
	})

	t.Run("Scenario_TenantWithNoBindingsHasNoMatch", func(t *testing.T) {
		// Given a brand-new tenant that hasn't installed anything yet
		// AND no platform fallback was registered,
		// When the renderer tries to resolve,
		// Then it errors loudly — caller knows no style exists rather
		// than silently rendering plain text.
		r := NewInMemoryExtensionOutputStyleRegistry()
		_, err := r.Resolve(context.Background(), ResolutionContext{TenantID: "fresh"})
		assert.True(t, errors.Is(err, ErrExtOutputStyleNoMatch))
	})

	t.Run("Scenario_DisabledBindingsExcludedFromResolutionForAdminPause", func(t *testing.T) {
		// Given admin temporarily disabled a vendor's style binding
		// (without deleting),
		// When resolution runs,
		// Then disabled binding is excluded — admin can pause without
		// losing the configuration.
		r := NewInMemoryExtensionOutputStyleRegistry()
		b := validBinding()
		b.Enabled = false
		_, _ = r.Bind(context.Background(), b)
		_, err := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
		assert.True(t, errors.Is(err, ErrExtOutputStyleNoMatch))
	})

	t.Run("Scenario_PriorityBreaksTiesWithinSameScope", func(t *testing.T) {
		// Given two tenant-scope bindings exist (admin chose two
		// styles for tenant-default),
		// When the resolver picks within the tenant scope,
		// Then higher Priority wins (admin can force their preferred
		// without changing scope).
		r := NewInMemoryExtensionOutputStyleRegistry()
		low := validBinding()
		low.StyleSlug = "low"
		low.Priority = 10
		_, _ = r.Bind(context.Background(), low)

		high := validBinding()
		high.StyleSlug = "high"
		high.Priority = 100
		_, _ = r.Bind(context.Background(), high)

		res, _ := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
		assert.Equal(t, "high", res.StyleSlug)
	})

	t.Run("Scenario_AgentScopeRequiresScopeIDToPreventGlobalLeak", func(t *testing.T) {
		// Given agent-scope without scope_id is ambiguous,
		// When admin tries to bind agent-scope without ID,
		// Then bind fails — prevents accidental global-leak.
		r := NewInMemoryExtensionOutputStyleRegistry()
		b := validBinding()
		b.Scope = OutputStyleScopeAgent
		b.ScopeID = ""
		_, err := r.Bind(context.Background(), b)
		assert.True(t, errors.Is(err, ErrExtOutputStyleScopeIDRequired))
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantStyleBleed", func(t *testing.T) {
		// Given tenant-A binds a style,
		// When tenant-B asks for resolution,
		// Then tenant-B does NOT see tenant-A's binding.
		r := NewInMemoryExtensionOutputStyleRegistry()
		_, _ = r.Bind(context.Background(), validBinding())
		_, err := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-2"})
		assert.True(t, errors.Is(err, ErrExtOutputStyleNoMatch))
	})

	t.Run("Scenario_ClearExtensionRemovesAllStyleBindingsAtomically", func(t *testing.T) {
		// Given an extension contributed 3 style bindings,
		// When admin uninstalls the extension,
		// Then all 3 disappear atomically (no orphan style bindings
		// pointing at uninstalled extensions).
		r := NewInMemoryExtensionOutputStyleRegistry()
		for _, slug := range []string{"a", "b", "c"} {
			b := validBinding()
			b.StyleSlug = slug
			_, _ = r.Bind(context.Background(), b)
		}
		count, _ := r.ClearExtension(context.Background(), "t-1", "vendor/themes-pack")
		assert.Equal(t, 3, count)
	})

	t.Run("Scenario_FourFormatsCoverWebOutputSpectrum", func(t *testing.T) {
		// Given web platform needs markdown / json / plain / html,
		// When format enum is queried,
		// Then exactly 4 formats — html is sanitized variant (XSS
		// guard built into the type contract).
		assert.Equal(t, 4, len(AllOutputStyleFormats()))
		set := map[OutputStyleFormat]bool{}
		for _, f := range AllOutputStyleFormats() {
			set[f] = true
		}
		assert.True(t, set[OutputStyleFormatHTMLSanitized],
			"html_sanitized variant — XSS guard at boundary")
	})
}
