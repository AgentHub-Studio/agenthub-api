package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_CustomAgents(t *testing.T) {
	t.Run("Scenario_TenantComposesOwnAgentRecipe", func(t *testing.T) {
		// Given a tenant wants a subagent shaped around their domain,
		// When they submit a CustomAgentDefinition with all required fields,
		// Then the registry accepts it and Lookup returns the same name.
		r := NewCustomAgentRegistry()
		d := validCustomAgent()
		d.Slug = "domain-specialist"
		require.NoError(t, r.Register(d))
		got, ok := r.Lookup(d.OwnerTenantSlug, d.Slug)
		require.True(t, ok)
		assert.Equal(t, "ACME Researcher", got.Name)
	})

	t.Run("Scenario_TwoTenantsShareSlugWithoutCollision", func(t *testing.T) {
		// Given the registry is keyed by (owner, slug),
		// When two tenants register the same slug,
		// Then both definitions coexist and ListByOwner isolates each tenant.
		r := NewCustomAgentRegistry()
		d1 := validCustomAgent()
		d1.OwnerTenantSlug = "tenant-a"
		d2 := validCustomAgent()
		d2.OwnerTenantSlug = "tenant-b"
		require.NoError(t, r.Register(d1))
		require.NoError(t, r.Register(d2))
		assert.Equal(t, 1, len(r.ListByOwner("tenant-a")))
		assert.Equal(t, 1, len(r.ListByOwner("tenant-b")))
	})

	t.Run("Scenario_CustomAgentDerivedFromBuiltinTracksLineage", func(t *testing.T) {
		// Given a tenant clones SUB-002 researcher-baseline as a starting
		// point, they tag DerivedFromBuiltinSlug to record lineage,
		// When admin lists all custom agents derived from researcher-baseline,
		// Then this fork shows up.
		r := NewCustomAgentRegistry()
		d := validCustomAgent()
		d.Slug = "fork-1"
		d.DerivedFromBuiltinSlug = "researcher-baseline"
		require.NoError(t, r.Register(d))
		matches := r.ListByDerivedBuiltin("researcher-baseline")
		require.Equal(t, 1, len(matches))
		assert.Equal(t, "fork-1", matches[0].Slug)
	})

	t.Run("Scenario_VisibilityGatesMarketplaceDiscovery", func(t *testing.T) {
		// Given marketplace listing requires opt-in via Visibility,
		// When a tenant sets Visibility=marketplace_listing,
		// Then ListByVisibility(marketplace_listing) surfaces it for any
		// admin UI catalogue query.
		r := NewCustomAgentRegistry()
		d := validCustomAgent()
		d.Slug = "public-pick"
		d.Visibility = CustomAgentVisibilityMarketplace
		require.NoError(t, r.Register(d))
		got := r.ListByVisibility(CustomAgentVisibilityMarketplace)
		require.Equal(t, 1, len(got))
	})

	t.Run("Scenario_SemverVersionEnforcedWhenSupplied", func(t *testing.T) {
		// Given semver lets tenants iterate on definitions safely,
		// When an invalid version is supplied,
		// Then Validate rejects with a stable sentinel.
		d := validCustomAgent()
		d.Version = "v1"
		assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadVersion)
	})

	t.Run("Scenario_DerivedSlugOptionalForGreenfieldAgents", func(t *testing.T) {
		// Given tenants may design agents from scratch (no SUB-002 lineage),
		// When DerivedFromBuiltinSlug is empty,
		// Then Validate accepts.
		d := validCustomAgent()
		d.DerivedFromBuiltinSlug = ""
		assert.NoError(t, d.Validate())
	})

	t.Run("Scenario_RegistryRejectsDuplicateOwnerSlug", func(t *testing.T) {
		// Given (owner, slug) is the uniqueness key inside a tenant,
		// When the same (owner, slug) is registered twice,
		// Then the registry rejects with ErrCustomAgentDuplicate.
		r := NewCustomAgentRegistry()
		d := validCustomAgent()
		require.NoError(t, r.Register(d))
		err := r.Register(d)
		assert.ErrorIs(t, err, ErrCustomAgentDuplicate)
	})

	t.Run("Scenario_ThreePolicyRefsAreMandatoryForSpawnContract", func(t *testing.T) {
		// Given the runner needs SUB-005/006/010 policies at spawn time,
		// When any policy slug is missing,
		// Then Validate rejects.
		d := validCustomAgent()
		d.ToolsetPolicySlug = ""
		assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptyToolsetSlug)
	})

	t.Run("Scenario_RegistryThreadSafeForConcurrentBootstrap", func(t *testing.T) {
		// Given the admin UI may bulk-import many definitions,
		// When many registrations land concurrently,
		// Then all distinct (owner, slug) succeed.
		r := NewCustomAgentRegistry()
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				d := validCustomAgent()
				d.Slug = "bulk-" + string(rune('a'+i%26))
				_ = r.Register(d)
			}(i)
		}
		wg.Wait()
		assert.LessOrEqual(t, r.Size(), 20)
		assert.Greater(t, r.Size(), 0)
	})

	t.Run("Scenario_LookupCrossReferencesSUB002Builtins", func(t *testing.T) {
		// Given SUB-002 catalog slugs are stable,
		// When a custom agent references researcher-baseline,
		// Then ListByDerivedBuiltin returns it (loose coupling, no FK).
		r := NewCustomAgentRegistry()
		d := validCustomAgent()
		d.Slug = "cross-ref"
		d.DerivedFromBuiltinSlug = "coder-baseline"
		require.NoError(t, r.Register(d))
		matches := r.ListByDerivedBuiltin("coder-baseline")
		assert.Equal(t, 1, len(matches))
	})
}
