package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validRoute() ComponentRoute {
	return ComponentRoute{
		Kind:             ExtensionComponentSkills,
		Slug:             "web-research",
		ExtensionSlug:    "vendor/research-pack",
		ExtensionVersion: "1.0.0",
		EntryPath:        "skills/web-research.yaml",
		InstalledAt:      time.Now(),
	}
}

func TestRoutingConflictPolicy_EnumIsBounded(t *testing.T) {
	for _, p := range AllRoutingConflictPolicies() {
		assert.True(t, IsValidRoutingConflictPolicy(p))
	}
	assert.False(t, IsValidRoutingConflictPolicy(RoutingConflictPolicy("random")))
	assert.Equal(t, 4, len(AllRoutingConflictPolicies()))
}

func TestIndexRoute_RegistersValidRoute(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.IndexRoute(context.Background(), "t", validRoute()))
	got, err := r.ListRoutes(context.Background(), "t", ExtensionComponentSkills)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestIndexRoute_RejectsEmptyTenant(t *testing.T) {
	r := NewInMemoryComponentRouter()
	err := r.IndexRoute(context.Background(), "", validRoute())
	assert.True(t, errors.Is(err, ErrRoutingTenantRequired))
}

func TestIndexRoute_RejectsInvalidKind(t *testing.T) {
	r := NewInMemoryComponentRouter()
	route := validRoute()
	route.Kind = "worktrees"
	err := r.IndexRoute(context.Background(), "t", route)
	assert.True(t, errors.Is(err, ErrRoutingInvalidComponentKind))
}

func TestIndexRoute_RejectsEmptyFields(t *testing.T) {
	r := NewInMemoryComponentRouter()
	for _, mutate := range []func(*ComponentRoute){
		func(c *ComponentRoute) { c.Slug = "" },
		func(c *ComponentRoute) { c.ExtensionSlug = "" },
	} {
		route := validRoute()
		mutate(&route)
		err := r.IndexRoute(context.Background(), "t", route)
		assert.Error(t, err)
	}
}

func TestIndexRoute_DeduplicatesByExtensionSlug(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r2 := validRoute()
	r2.ExtensionVersion = "2.0.0"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))
	got, _ := r.ListRoutes(context.Background(), "t", ExtensionComponentSkills)
	require.Len(t, got, 1, "same extension → deduped")
	assert.Equal(t, "2.0.0", got[0].ExtensionVersion, "newer version wins")
}

func TestIndexRoute_DifferentExtensionsProduceMultipleCandidates(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r1.ExtensionSlug = "vendor-a/pack"
	r2 := validRoute()
	r2.ExtensionSlug = "vendor-b/pack"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))
	got, _ := r.ListRoutes(context.Background(), "t", ExtensionComponentSkills)
	assert.Len(t, got, 2, "two extensions claim same (kind, slug)")
}

func TestIndexRoutes_BatchSucceeds(t *testing.T) {
	r := NewInMemoryComponentRouter()
	routes := []ComponentRoute{validRoute()}
	r2 := validRoute()
	r2.Slug = "kb-research"
	routes = append(routes, r2)
	require.NoError(t, r.IndexRoutes(context.Background(), "t", routes))
	got, _ := r.ListRoutes(context.Background(), "t", ExtensionComponentSkills)
	assert.Len(t, got, 2)
}

func TestResolve_SingleMatchReturnsRoute(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.IndexRoute(context.Background(), "t", validRoute()))
	res, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictFirstInstallWins)
	require.NoError(t, err)
	assert.Equal(t, "single_match", res.ResolvedBy)
	assert.False(t, res.ConflictDetected)
	assert.Equal(t, "vendor/research-pack", res.Route.ExtensionSlug)
}

func TestResolve_NoMatchReturnsError(t *testing.T) {
	r := NewInMemoryComponentRouter()
	_, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "missing",
		RoutingConflictFirstInstallWins)
	assert.True(t, errors.Is(err, ErrRoutingNoMatch))
}

func TestResolve_ConflictFirstInstallWinsPicksOldest(t *testing.T) {
	r := NewInMemoryComponentRouter()
	now := time.Now()
	r1 := validRoute()
	r1.ExtensionSlug = "older"
	r1.InstalledAt = now.Add(-time.Hour)
	r2 := validRoute()
	r2.ExtensionSlug = "newer"
	r2.InstalledAt = now
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	res, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictFirstInstallWins)
	require.NoError(t, err)
	assert.True(t, res.ConflictDetected)
	assert.Equal(t, "older", res.Route.ExtensionSlug)
	assert.Equal(t, "policy:first_install_wins", res.ResolvedBy)
	assert.Len(t, res.ConflictCandidates, 2)
}

func TestResolve_ConflictLatestInstallWinsPicksNewest(t *testing.T) {
	r := NewInMemoryComponentRouter()
	now := time.Now()
	r1 := validRoute()
	r1.ExtensionSlug = "older"
	r1.InstalledAt = now.Add(-time.Hour)
	r2 := validRoute()
	r2.ExtensionSlug = "newer"
	r2.InstalledAt = now
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	res, _ := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictLatestInstallWins)
	assert.Equal(t, "newer", res.Route.ExtensionSlug)
}

func TestResolve_ConflictErrorOnConflictAborts(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r1.ExtensionSlug = "a"
	r2 := validRoute()
	r2.ExtensionSlug = "b"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	_, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictErrorOnConflict)
	assert.True(t, errors.Is(err, ErrRoutingConflictUnresolved))
}

func TestResolve_ConflictRequireExplicitPinErrorsWithoutPin(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r1.ExtensionSlug = "a"
	r2 := validRoute()
	r2.ExtensionSlug = "b"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	_, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictRequireExplicitPin)
	assert.True(t, errors.Is(err, ErrRoutingPinRequired))
}

func TestResolve_PinOverridesPolicyOnConflict(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r1.ExtensionSlug = "preferred"
	r2 := validRoute()
	r2.ExtensionSlug = "other"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	require.NoError(t, r.SetPin(context.Background(), ComponentPin{
		TenantID: "t", Kind: ExtensionComponentSkills,
		Slug: "web-research", ExtensionSlug: "preferred",
		Reason: "admin chose preferred for security audit",
	}))

	res, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictErrorOnConflict)
	require.NoError(t, err, "pin overrides error policy")
	assert.Equal(t, "preferred", res.Route.ExtensionSlug)
	assert.Equal(t, "pin", res.ResolvedBy)
}

func TestResolve_PinReferencingMissingExtensionFails(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r1.ExtensionSlug = "actual"
	r2 := validRoute()
	r2.ExtensionSlug = "other"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	require.NoError(t, r.SetPin(context.Background(), ComponentPin{
		TenantID: "t", Kind: ExtensionComponentSkills,
		Slug: "web-research", ExtensionSlug: "ghost-extension",
		Reason: "pin to non-existent extension",
	}))

	_, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research",
		RoutingConflictFirstInstallWins)
	assert.True(t, errors.Is(err, ErrRoutingPinInvalidExtension))
}

func TestResolve_RejectsInvalidPolicy(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.IndexRoute(context.Background(), "t", validRoute()))
	_, err := r.Resolve(context.Background(), "t",
		ExtensionComponentSkills, "web-research", "bogus")
	assert.True(t, errors.Is(err, ErrRoutingInvalidPolicy))
}

func TestSetPin_RejectsMissingFields(t *testing.T) {
	r := NewInMemoryComponentRouter()
	for _, mutate := range []func(*ComponentPin){
		func(p *ComponentPin) { p.TenantID = "" },
		func(p *ComponentPin) { p.Kind = "bogus" },
		func(p *ComponentPin) { p.Slug = "" },
		func(p *ComponentPin) { p.ExtensionSlug = "" },
		func(p *ComponentPin) { p.Reason = "" },
	} {
		pin := ComponentPin{
			TenantID: "t", Kind: ExtensionComponentSkills,
			Slug: "x", ExtensionSlug: "y", Reason: "test",
		}
		mutate(&pin)
		err := r.SetPin(context.Background(), pin)
		assert.Error(t, err)
	}
}

func TestRemovePin_NotFoundReturnsError(t *testing.T) {
	r := NewInMemoryComponentRouter()
	err := r.RemovePin(context.Background(), "t", ExtensionComponentSkills, "missing")
	assert.Error(t, err)
}

func TestListPins_ReturnsAllForTenant(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.SetPin(context.Background(), ComponentPin{
		TenantID: "t", Kind: ExtensionComponentSkills,
		Slug: "a", ExtensionSlug: "x", Reason: "test",
	}))
	require.NoError(t, r.SetPin(context.Background(), ComponentPin{
		TenantID: "t", Kind: ExtensionComponentTools,
		Slug: "b", ExtensionSlug: "y", Reason: "test",
	}))
	got, _ := r.ListPins(context.Background(), "t")
	assert.Len(t, got, 2)
}

func TestListPins_TenantIsolation(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.SetPin(context.Background(), ComponentPin{
		TenantID: "t-a", Kind: ExtensionComponentSkills,
		Slug: "x", ExtensionSlug: "y", Reason: "test",
	}))
	got, _ := r.ListPins(context.Background(), "t-b")
	assert.Empty(t, got)
}

func TestClearTenant_RemovesEverythingForTenant(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.IndexRoute(context.Background(), "t", validRoute()))
	require.NoError(t, r.SetPin(context.Background(), ComponentPin{
		TenantID: "t", Kind: ExtensionComponentSkills,
		Slug: "x", ExtensionSlug: "y", Reason: "test",
	}))
	require.NoError(t, r.ClearTenant(context.Background(), "t"))
	routes, _ := r.ListRoutes(context.Background(), "t", ExtensionComponentSkills)
	pins, _ := r.ListPins(context.Background(), "t")
	assert.Empty(t, routes)
	assert.Empty(t, pins)
}

func TestClearTenant_PreservesOtherTenants(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.IndexRoute(context.Background(), "t-a", validRoute()))
	require.NoError(t, r.IndexRoute(context.Background(), "t-b", validRoute()))
	require.NoError(t, r.ClearTenant(context.Background(), "t-a"))

	a, _ := r.ListRoutes(context.Background(), "t-a", ExtensionComponentSkills)
	b, _ := r.ListRoutes(context.Background(), "t-b", ExtensionComponentSkills)
	assert.Empty(t, a)
	assert.Len(t, b, 1)
}

func TestListRoutes_TenantAndKindIsolation(t *testing.T) {
	r := NewInMemoryComponentRouter()
	r1 := validRoute()
	r2 := validRoute()
	r2.Kind = ExtensionComponentTools
	r2.Slug = "tool-x"
	require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
	require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

	skills, _ := r.ListRoutes(context.Background(), "t", ExtensionComponentSkills)
	tools, _ := r.ListRoutes(context.Background(), "t", ExtensionComponentTools)
	assert.Len(t, skills, 1)
	assert.Len(t, tools, 1)
}

func TestRouter_ConcurrentIndexResolveIsSafe(t *testing.T) {
	r := NewInMemoryComponentRouter()
	require.NoError(t, r.IndexRoute(context.Background(), "t", validRoute()))
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Resolve(context.Background(), "t",
				ExtensionComponentSkills, "web-research",
				RoutingConflictFirstInstallWins)
		}()
	}
	wg.Wait()
}

func TestRouter_ContextCancelled(t *testing.T) {
	r := NewInMemoryComponentRouter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := r.IndexRoute(ctx, "t", validRoute())
	assert.Error(t, err)
	_, err = r.Resolve(ctx, "t", ExtensionComponentSkills, "x", RoutingConflictFirstInstallWins)
	assert.Error(t, err)
	err = r.SetPin(ctx, ComponentPin{TenantID: "t", Kind: ExtensionComponentSkills, Slug: "x", ExtensionSlug: "y", Reason: "z"})
	assert.Error(t, err)
	err = r.RemovePin(ctx, "t", ExtensionComponentSkills, "x")
	assert.Error(t, err)
	_, err = r.ListRoutes(ctx, "t", ExtensionComponentSkills)
	assert.Error(t, err)
	_, err = r.ListPins(ctx, "t")
	assert.Error(t, err)
	err = r.ClearTenant(ctx, "t")
	assert.Error(t, err)
}
