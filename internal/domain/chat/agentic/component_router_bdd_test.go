package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ComponentRouter(t *testing.T) {
	t.Run("Scenario_RuntimeResolvesSkillToInstalledExtension", func(t *testing.T) {
		// Given an extension is installed providing skills/web-research,
		// And the agentic runtime needs that skill,
		// When it asks the router,
		// Then it gets back the route pointing at the extension's
		// entry path — single source of truth for "what implements this".
		r := NewInMemoryComponentRouter()
		require.NoError(t, r.IndexRoute(context.Background(), "t", validRoute()))
		res, err := r.Resolve(context.Background(), "t",
			ExtensionComponentSkills, "web-research",
			RoutingConflictFirstInstallWins)
		require.NoError(t, err)
		assert.Equal(t, "vendor/research-pack", res.Route.ExtensionSlug)
		assert.Equal(t, "skills/web-research.yaml", res.Route.EntryPath)
	})

	t.Run("Scenario_TwoExtensionsClaimSameComponentTriggersConflict", func(t *testing.T) {
		// Given two competing vendors both ship a skill named web-research,
		// When admin installs both,
		// Then the router records both as candidates — visible to admin
		// for explicit resolution.
		r := NewInMemoryComponentRouter()
		r1 := validRoute()
		r1.ExtensionSlug = "vendor-a/pack"
		r2 := validRoute()
		r2.ExtensionSlug = "vendor-b/pack"
		require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
		require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

		res, _ := r.Resolve(context.Background(), "t",
			ExtensionComponentSkills, "web-research",
			RoutingConflictFirstInstallWins)
		assert.True(t, res.ConflictDetected)
		assert.Len(t, res.ConflictCandidates, 2)
	})

	t.Run("Scenario_AdminPinOverridesAutomaticPolicyForSecurityAudit", func(t *testing.T) {
		// Given conflict between vendor-a (untrusted) and vendor-b (trusted),
		// And admin wants vendor-b regardless of install order,
		// When admin sets a pin pointing at vendor-b,
		// Then Resolve returns vendor-b even when policy=error_on_conflict
		// — pin trumps everything.
		r := NewInMemoryComponentRouter()
		r1 := validRoute()
		r1.ExtensionSlug = "vendor-a/pack"
		r2 := validRoute()
		r2.ExtensionSlug = "vendor-b/pack"
		require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
		require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

		require.NoError(t, r.SetPin(context.Background(), ComponentPin{
			TenantID: "t", Kind: ExtensionComponentSkills,
			Slug: "web-research", ExtensionSlug: "vendor-b/pack",
			Reason: "vendor-a flagged in security audit",
		}))

		res, err := r.Resolve(context.Background(), "t",
			ExtensionComponentSkills, "web-research",
			RoutingConflictErrorOnConflict)
		require.NoError(t, err)
		assert.Equal(t, "vendor-b/pack", res.Route.ExtensionSlug)
		assert.Equal(t, "pin", res.ResolvedBy)
	})

	t.Run("Scenario_StrictPolicyForcesAdminToResolveBeforeRuntime", func(t *testing.T) {
		// Given regulated tenants want zero ambiguity,
		// When policy=require_explicit_pin and conflict exists with no pin,
		// Then resolution fails — forces admin to pin BEFORE runtime
		// hits the missing decision (no silent random winner).
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
	})

	t.Run("Scenario_LatestInstallWinsForOptimisticPlatformUpdates", func(t *testing.T) {
		// Given platform team prefers the most recent extension version
		// (assumes newer = better),
		// When policy=latest_install_wins,
		// Then newer InstalledAt wins — automatic upgrade path.
		r := NewInMemoryComponentRouter()
		now := time.Now()
		r1 := validRoute()
		r1.ExtensionSlug = "v1"
		r1.InstalledAt = now.Add(-time.Hour)
		r2 := validRoute()
		r2.ExtensionSlug = "v2"
		r2.InstalledAt = now
		require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
		require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

		res, _ := r.Resolve(context.Background(), "t",
			ExtensionComponentSkills, "web-research",
			RoutingConflictLatestInstallWins)
		assert.Equal(t, "v2", res.Route.ExtensionSlug)
	})

	t.Run("Scenario_FirstInstallWinsForStabilityPlatform", func(t *testing.T) {
		// Given platform team prefers stability — incumbent wins (avoid
		// auto-upgrade surprises),
		// When policy=first_install_wins,
		// Then earliest InstalledAt wins.
		r := NewInMemoryComponentRouter()
		now := time.Now()
		r1 := validRoute()
		r1.ExtensionSlug = "incumbent"
		r1.InstalledAt = now.Add(-time.Hour)
		r2 := validRoute()
		r2.ExtensionSlug = "newcomer"
		r2.InstalledAt = now
		require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
		require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

		res, _ := r.Resolve(context.Background(), "t",
			ExtensionComponentSkills, "web-research",
			RoutingConflictFirstInstallWins)
		assert.Equal(t, "incumbent", res.Route.ExtensionSlug)
	})

	t.Run("Scenario_PinReasonRequiredForGOV001Audit", func(t *testing.T) {
		// Given GOV-001 audits every routing override,
		// When admin tries to pin without a reason,
		// Then SetPin rejects — every pin has a justification in the
		// audit trail.
		r := NewInMemoryComponentRouter()
		err := r.SetPin(context.Background(), ComponentPin{
			TenantID: "t", Kind: ExtensionComponentSkills,
			Slug: "x", ExtensionSlug: "y", Reason: "",
		})
		assert.Error(t, err)
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantRouteBleed", func(t *testing.T) {
		// Given tenant-A installs a route,
		// When tenant-B asks for the same (kind, slug),
		// Then tenant-B sees no route — per-tenant index keying.
		r := NewInMemoryComponentRouter()
		require.NoError(t, r.IndexRoute(context.Background(), "t-a", validRoute()))
		_, err := r.Resolve(context.Background(), "t-b",
			ExtensionComponentSkills, "web-research",
			RoutingConflictFirstInstallWins)
		assert.True(t, errors.Is(err, ErrRoutingNoMatch))
	})

	t.Run("Scenario_UninstallTenantClearsRoutesAndPinsAtomically", func(t *testing.T) {
		// Given tenant requests deactivation,
		// When admin runs ClearTenant,
		// Then ALL routes + pins for that tenant disappear, leaving
		// other tenants intact (clean uninstall).
		r := NewInMemoryComponentRouter()
		require.NoError(t, r.IndexRoute(context.Background(), "t-a", validRoute()))
		require.NoError(t, r.IndexRoute(context.Background(), "t-b", validRoute()))
		require.NoError(t, r.ClearTenant(context.Background(), "t-a"))

		a, _ := r.ListRoutes(context.Background(), "t-a", ExtensionComponentSkills)
		b, _ := r.ListRoutes(context.Background(), "t-b", ExtensionComponentSkills)
		assert.Empty(t, a)
		assert.Len(t, b, 1)
	})

	t.Run("Scenario_ConflictTraceFeedsGOV001AuditTrail", func(t *testing.T) {
		// Given audit needs to know when conflicts happened + which
		// candidates competed,
		// When Resolve returns,
		// Then ConflictCandidates lists all competitors AND ResolvedBy
		// names the resolution mechanism (pin / policy / single).
		r := NewInMemoryComponentRouter()
		r1 := validRoute()
		r1.ExtensionSlug = "a"
		r2 := validRoute()
		r2.ExtensionSlug = "b"
		require.NoError(t, r.IndexRoute(context.Background(), "t", r1))
		require.NoError(t, r.IndexRoute(context.Background(), "t", r2))

		res, _ := r.Resolve(context.Background(), "t",
			ExtensionComponentSkills, "web-research",
			RoutingConflictFirstInstallWins)
		assert.True(t, res.ConflictDetected)
		assert.Len(t, res.ConflictCandidates, 2)
		assert.Contains(t, res.ResolvedBy, "first_install_wins")
	})
}
