package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBinding() ExtensionOutputStyleBinding {
	return ExtensionOutputStyleBinding{
		TenantID:        "t-1",
		StyleSlug:       "conversational",
		Format:          OutputStyleFormatMarkdown,
		Scope:           OutputStyleScopeTenant,
		Priority:        50,
		Enabled:         true,
		SourceExtension: "vendor/themes-pack",
	}
}

func TestOutputStyleFormat_EnumIsBounded(t *testing.T) {
	for _, f := range AllOutputStyleFormats() {
		assert.True(t, IsValidOutputStyleFormat(f))
	}
	assert.False(t, IsValidOutputStyleFormat(OutputStyleFormat("xml")))
	assert.Equal(t, 4, len(AllOutputStyleFormats()))
}

func TestOutputStyleScope_EnumIsBounded(t *testing.T) {
	for _, s := range AllOutputStyleScopes() {
		assert.True(t, IsValidOutputStyleScope(s))
	}
	assert.False(t, IsValidOutputStyleScope(OutputStyleScope("global")))
	assert.Equal(t, 4, len(AllOutputStyleScopes()))
}

func TestOutputStyleScope_CascadeOrderIsExplicitFirst(t *testing.T) {
	expected := []OutputStyleScope{
		OutputStyleScopeExplicit, OutputStyleScopeAgent,
		OutputStyleScopeTenant, OutputStyleScopePlatform,
	}
	assert.Equal(t, expected, AllOutputStyleScopes())
}

func TestExtOutStyle_Bind_AcceptsValid(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	saved, err := r.Bind(context.Background(), validBinding())
	require.NoError(t, err)
	assert.NotEqual(t, "", saved.ID.String())
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestExtOutStyle_Bind_RejectsInvalidFormat(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	b := validBinding()
	b.Format = "xml"
	_, err := r.Bind(context.Background(), b)
	assert.True(t, errors.Is(err, ErrExtOutputStyleInvalidFormat))
}

func TestExtOutStyle_Bind_RejectsInvalidScope(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	b := validBinding()
	b.Scope = "global"
	_, err := r.Bind(context.Background(), b)
	assert.True(t, errors.Is(err, ErrExtOutputStyleInvalidScope))
}

func TestExtOutStyle_Bind_RejectsMissingFields(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	for name, mutate := range map[string]func(*ExtensionOutputStyleBinding){
		"tenant": func(b *ExtensionOutputStyleBinding) { b.TenantID = "" },
		"style":  func(b *ExtensionOutputStyleBinding) { b.StyleSlug = "" },
	} {
		b := validBinding()
		mutate(&b)
		_, err := r.Bind(context.Background(), b)
		assert.Error(t, err, "missing %s must error", name)
	}
}

func TestExtOutStyle_Bind_RequiresScopeIDForAgentAndExplicit(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	for _, scope := range []OutputStyleScope{OutputStyleScopeAgent, OutputStyleScopeExplicit} {
		b := validBinding()
		b.Scope = scope
		b.ScopeID = ""
		_, err := r.Bind(context.Background(), b)
		assert.True(t, errors.Is(err, ErrExtOutputStyleScopeIDRequired),
			"scope %q must require scope_id", scope)
	}
}

func TestExtOutStyle_Bind_AllowsEmptyScopeIDForTenantAndPlatform(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	for _, scope := range []OutputStyleScope{OutputStyleScopeTenant, OutputStyleScopePlatform} {
		b := validBinding()
		b.Scope = scope
		b.ScopeID = ""
		// Use unique slug to avoid duplicate.
		b.StyleSlug = "style-" + string(scope)
		_, err := r.Bind(context.Background(), b)
		assert.NoError(t, err, "scope %q must allow empty scope_id", scope)
	}
}

func TestExtOutStyle_Bind_RejectsDuplicate(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	_, err := r.Bind(context.Background(), validBinding())
	require.NoError(t, err)
	_, err = r.Bind(context.Background(), validBinding())
	assert.True(t, errors.Is(err, ErrExtOutputStyleDuplicate))
}

func TestExtOutStyle_Find_RoundTrips(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	saved, _ := r.Bind(context.Background(), validBinding())
	got, err := r.Find(context.Background(), saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

func TestExtOutStyle_Find_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	_, err := r.Find(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrExtOutputStyleNotFound))
}

func TestExtOutStyle_Resolve_PicksExplicitOverAgentTenantPlatform(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	// Platform binding.
	p := validBinding()
	p.StyleSlug = "platform-style"
	p.Scope = OutputStyleScopePlatform
	p.ScopeID = ""
	_, _ = r.Bind(context.Background(), p)

	// Tenant binding.
	tnt := validBinding()
	tnt.StyleSlug = "tenant-style"
	tnt.Scope = OutputStyleScopeTenant
	tnt.ScopeID = ""
	_, _ = r.Bind(context.Background(), tnt)

	// Agent binding.
	a := validBinding()
	a.StyleSlug = "agent-style"
	a.Scope = OutputStyleScopeAgent
	a.ScopeID = "agent-x"
	_, _ = r.Bind(context.Background(), a)

	// Explicit binding.
	e := validBinding()
	e.StyleSlug = "explicit-style"
	e.Scope = OutputStyleScopeExplicit
	e.ScopeID = "req-1"
	_, _ = r.Bind(context.Background(), e)

	res, err := r.Resolve(context.Background(), ResolutionContext{
		TenantID: "t-1", AgentID: "agent-x", ExplicitRequestID: "req-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "explicit-style", res.StyleSlug)
}

func TestExtOutStyle_Resolve_FallsBackToAgentWhenNoExplicit(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	tnt := validBinding()
	tnt.StyleSlug = "tenant-style"
	_, _ = r.Bind(context.Background(), tnt)

	a := validBinding()
	a.StyleSlug = "agent-style"
	a.Scope = OutputStyleScopeAgent
	a.ScopeID = "agent-x"
	_, _ = r.Bind(context.Background(), a)

	res, _ := r.Resolve(context.Background(), ResolutionContext{
		TenantID: "t-1", AgentID: "agent-x",
	})
	assert.Equal(t, "agent-style", res.StyleSlug)
}

func TestExtOutStyle_Resolve_FallsBackToTenantWhenAgentMissing(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	tnt := validBinding()
	tnt.StyleSlug = "tenant-style"
	_, _ = r.Bind(context.Background(), tnt)

	res, _ := r.Resolve(context.Background(), ResolutionContext{
		TenantID: "t-1", AgentID: "no-binding-for-this-agent",
	})
	assert.Equal(t, "tenant-style", res.StyleSlug)
}

func TestExtOutStyle_Resolve_FallsBackToPlatformDefault(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	p := validBinding()
	p.StyleSlug = "platform-default"
	p.Scope = OutputStyleScopePlatform
	p.ScopeID = ""
	_, _ = r.Bind(context.Background(), p)

	res, _ := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
	assert.Equal(t, "platform-default", res.StyleSlug)
}

func TestExtOutStyle_Resolve_NoMatchReturnsError(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	_, err := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
	assert.True(t, errors.Is(err, ErrExtOutputStyleNoMatch))
}

func TestExtOutStyle_Resolve_RejectsEmptyTenant(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	_, err := r.Resolve(context.Background(), ResolutionContext{})
	assert.True(t, errors.Is(err, ErrExtOutputStyleTenantRequired))
}

func TestExtOutStyle_Resolve_PriorityBreaksTiesWithinScope(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	low := validBinding()
	low.StyleSlug = "low-prio"
	low.Priority = 10
	_, _ = r.Bind(context.Background(), low)

	high := validBinding()
	high.StyleSlug = "high-prio"
	high.Priority = 100
	_, _ = r.Bind(context.Background(), high)

	res, _ := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
	assert.Equal(t, "high-prio", res.StyleSlug)
}

func TestExtOutStyle_Resolve_DisabledBindingsExcluded(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	b := validBinding()
	b.Enabled = false
	_, _ = r.Bind(context.Background(), b)

	_, err := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
	assert.True(t, errors.Is(err, ErrExtOutputStyleNoMatch))
}

func TestExtOutStyle_Resolve_TenantIsolation(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	_, _ = r.Bind(context.Background(), validBinding())
	_, err := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-2"})
	assert.True(t, errors.Is(err, ErrExtOutputStyleNoMatch))
}

func TestExtOutStyle_ListByTenant_OrderedByScopeCascadeThenSlug(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	for _, scope := range []OutputStyleScope{
		OutputStyleScopePlatform, OutputStyleScopeTenant,
	} {
		b := validBinding()
		b.StyleSlug = "style-" + string(scope)
		b.Scope = scope
		b.ScopeID = ""
		_, _ = r.Bind(context.Background(), b)
	}

	got, _ := r.ListByTenant(context.Background(), "t-1")
	require.Len(t, got, 2)
	// Tenant has rank 2, Platform has rank 3 — tenant first.
	assert.Equal(t, OutputStyleScopeTenant, got[0].Scope)
	assert.Equal(t, OutputStyleScopePlatform, got[1].Scope)
}

func TestExtOutStyle_ListByScope_FiltersStrictly(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	a := validBinding()
	a.Scope = OutputStyleScopeAgent
	a.ScopeID = "agent-x"
	_, _ = r.Bind(context.Background(), a)

	tnt := validBinding()
	tnt.StyleSlug = "tenant-style"
	_, _ = r.Bind(context.Background(), tnt)

	agents, _ := r.ListByScope(context.Background(), "t-1", OutputStyleScopeAgent)
	assert.Len(t, agents, 1)
}

func TestExtOutStyle_ListByScope_RejectsInvalidScope(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	_, err := r.ListByScope(context.Background(), "t-1", "global")
	assert.True(t, errors.Is(err, ErrExtOutputStyleInvalidScope))
}

func TestExtOutStyle_Disable_IsIdempotent(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	saved, _ := r.Bind(context.Background(), validBinding())
	require.NoError(t, r.Disable(context.Background(), saved.ID))
	require.NoError(t, r.Disable(context.Background(), saved.ID))
	got, _ := r.Find(context.Background(), saved.ID)
	assert.False(t, got.Enabled)
}

func TestExtOutStyle_Disable_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	err := r.Disable(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrExtOutputStyleNotFound))
}

func TestExtOutStyle_Delete_RemovesBinding(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	saved, _ := r.Bind(context.Background(), validBinding())
	require.NoError(t, r.Delete(context.Background(), saved.ID))
	_, err := r.Find(context.Background(), saved.ID)
	assert.True(t, errors.Is(err, ErrExtOutputStyleNotFound))
}

func TestExtOutStyle_Delete_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	err := r.Delete(context.Background(), uuid.New())
	assert.True(t, errors.Is(err, ErrExtOutputStyleNotFound))
}

func TestExtOutStyle_ClearExtension_RemovesAllForExtension(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	for _, slug := range []string{"a", "b", "c"} {
		b := validBinding()
		b.StyleSlug = slug
		_, _ = r.Bind(context.Background(), b)
	}
	other := validBinding()
	other.StyleSlug = "other"
	other.SourceExtension = "different/pack"
	_, _ = r.Bind(context.Background(), other)

	count, err := r.ClearExtension(context.Background(), "t-1", "vendor/themes-pack")
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	remaining, _ := r.ListByTenant(context.Background(), "t-1")
	assert.Len(t, remaining, 1)
	assert.Equal(t, "other", remaining[0].StyleSlug)
}

func TestExtOutStyle_Resolve_NewerCreatedAtBreaksPriorityTie(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	now := time.Now()
	r.SetClock(func() time.Time { return now })

	b1 := validBinding()
	b1.StyleSlug = "older"
	_, _ = r.Bind(context.Background(), b1)

	now = now.Add(time.Hour)
	b2 := validBinding()
	b2.StyleSlug = "newer"
	_, _ = r.Bind(context.Background(), b2)

	// Same priority + scope; newer wins.
	res, _ := r.Resolve(context.Background(), ResolutionContext{TenantID: "t-1"})
	assert.Equal(t, "newer", res.StyleSlug)
}

func TestExtOutStyle_ConcurrentBindIsSafe(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := validBinding()
			b.StyleSlug = "slug-" + string(rune('a'+(i%26)))
			_, _ = r.Bind(context.Background(), b)
		}()
	}
	wg.Wait()
}

func TestExtOutStyle_ContextCancelled(t *testing.T) {
	r := NewInMemoryExtensionOutputStyleRegistry()
	saved, _ := r.Bind(context.Background(), validBinding())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Bind(ctx, validBinding())
	assert.Error(t, err)
	_, err = r.Find(ctx, saved.ID)
	assert.Error(t, err)
	_, err = r.Resolve(ctx, ResolutionContext{TenantID: "t-1"})
	assert.Error(t, err)
	_, err = r.ListByTenant(ctx, "t-1")
	assert.Error(t, err)
	_, err = r.ListByScope(ctx, "t-1", OutputStyleScopeTenant)
	assert.Error(t, err)
	err = r.Disable(ctx, saved.ID)
	assert.Error(t, err)
	err = r.Delete(ctx, saved.ID)
	assert.Error(t, err)
	_, err = r.ClearExtension(ctx, "t-1", "x")
	assert.Error(t, err)
}
