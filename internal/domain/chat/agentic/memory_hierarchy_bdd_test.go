package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_MemoryHierarchy(t *testing.T) {
	t.Run("Scenario_SessionOverrideTrumpsUserAgentTenantGlobalDefaults", func(t *testing.T) {
		// Given a tenant default tone=formal, an agent default tone=
		// casual, a user preference tone=terse, and the user's current
		// session sets tone=verbose for one-off,
		// When the runtime looks up "tone" with full LookupContext,
		// Then session value wins (most specific layer takes precedence).
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeTenant, ScopeID: "t", Key: "tone", Value: "formal"})
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeAgent, ScopeID: "a", Key: "tone", Value: "casual"})
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeUser, ScopeID: "u", Key: "tone", Value: "terse"})
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeSession, ScopeID: "s", Key: "tone", Value: "verbose"})

		got, _, err := s.Lookup(context.Background(), LookupContext{
			SessionID: "s", UserID: "u", AgentID: "a", TenantID: "t",
		}, "tone")
		require.NoError(t, err)
		assert.Equal(t, "verbose", got.Value)
	})

	t.Run("Scenario_MissingScopeIDFallsThroughToWiderLayer", func(t *testing.T) {
		// Given a request without a session_id (e.g. background_run),
		// When the runtime looks up tone,
		// Then session scope is SKIPPED (not error) and the next layer
		// is consulted — useful for cron jobs that have no user session.
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeTenant, ScopeID: "t-1", Key: "tone", Value: "from_tenant",
		})
		got, trace, err := s.Lookup(context.Background(), LookupContext{TenantID: "t-1"}, "tone")
		require.NoError(t, err)
		assert.Equal(t, "from_tenant", got.Value)
		// Session probe was skipped.
		for _, p := range trace.Scopes {
			if p.Scope == MemoryHierarchyScopeSession {
				assert.True(t, p.Skipped)
			}
		}
	})

	t.Run("Scenario_StaleFactsAreSkippedWithoutErrorPreservingFreshness", func(t *testing.T) {
		// Given a session-scope cached preference older than its max_age,
		// When the runtime looks up,
		// Then the stale fact is skipped (not returned as the "most
		// specific" answer) and lookup falls through to a fresher layer.
		s := NewInMemoryMemoryHierarchyStore()
		now := time.Now()
		s.SetClock(func() time.Time { return now })

		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeSession, ScopeID: "s", Key: "k", Value: "stale",
			SetAt: now.Add(-2 * time.Hour), MaxAge: time.Hour,
		})
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeTenant, ScopeID: "t", Key: "k", Value: "live", SetAt: now,
		})

		got, _, err := s.Lookup(context.Background(), LookupContext{SessionID: "s", TenantID: "t"}, "k")
		require.NoError(t, err)
		assert.Equal(t, "live", got.Value)
	})

	t.Run("Scenario_TraceRecordsResolutionPathForAuditability", func(t *testing.T) {
		// Given GOV-001 audits which scope provided each value,
		// When Lookup runs,
		// Then trace records every scope probed (with skipped/found/
		// stale flags) so auditor can reconstruct decisions.
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeAgent, ScopeID: "a", Key: "k", Value: "v",
		})
		_, trace, _ := s.Lookup(context.Background(), LookupContext{
			SessionID: "s", AgentID: "a", TenantID: "t",
		}, "k")
		assert.Equal(t, MemoryHierarchyScopeAgent, trace.Matched)
		assert.NotEmpty(t, trace.Scopes)
	})

	t.Run("Scenario_GlobalScopeSurvivesEvenWithEmptyContext", func(t *testing.T) {
		// Given platform-wide constants live at global scope,
		// When a daemon with no session/user/agent/tenant looks up,
		// Then global facts are still reachable (e.g. platform_name).
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeGlobal, Key: "platform_name", Value: "AgentHub",
		})
		got, _, err := s.Lookup(context.Background(), LookupContext{}, "platform_name")
		require.NoError(t, err)
		assert.Equal(t, "AgentHub", got.Value)
	})

	t.Run("Scenario_LookupAllReturnsEveryLiveLayerForDebugging", func(t *testing.T) {
		// Given debugging needs to see all overrides for a key,
		// When admin calls LookupAll,
		// Then it returns one fact per matching layer narrow→wide so
		// admin can see "user wants X but tenant default is Y".
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeUser, ScopeID: "u", Key: "k", Value: "user_v"})
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeTenant, ScopeID: "t", Key: "k", Value: "tenant_v"})
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeGlobal, Key: "k", Value: "global_v"})

		got, err := s.LookupAll(context.Background(), LookupContext{UserID: "u", TenantID: "t"}, "k")
		require.NoError(t, err)
		require.Len(t, got, 3)
		// Order narrow→wide.
		assert.Equal(t, MemoryHierarchyScopeUser, got[0].Scope)
		assert.Equal(t, MemoryHierarchyScopeTenant, got[1].Scope)
		assert.Equal(t, MemoryHierarchyScopeGlobal, got[2].Scope)
	})

	t.Run("Scenario_PurgeStaleClearsExpiredFactsForBackgroundCleanup", func(t *testing.T) {
		// Given a background cleanup job runs nightly to drop old caches,
		// When PurgeStale runs,
		// Then it returns the count removed and stale entries are gone
		// from subsequent lookups.
		s := NewInMemoryMemoryHierarchyStore()
		now := time.Now()
		s.SetClock(func() time.Time { return now })
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeUser, ScopeID: "u", Key: "old", Value: "x",
			SetAt: now.Add(-100 * 24 * time.Hour), MaxAge: 30 * 24 * time.Hour,
		})
		count, err := s.PurgeStale(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantBleed", func(t *testing.T) {
		// Given tenant A and tenant B both have key "tone",
		// When agent in tenant A looks up,
		// Then it sees A's value, never B's (per-tenant scope_id keying).
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeTenant, ScopeID: "t-a", Key: "tone", Value: "A_value"})
		_, _ = s.Set(context.Background(), HierarchicalFact{Scope: MemoryHierarchyScopeTenant, ScopeID: "t-b", Key: "tone", Value: "B_value"})

		got, _, _ := s.Lookup(context.Background(), LookupContext{TenantID: "t-a"}, "tone")
		assert.Equal(t, "A_value", got.Value)
	})

	t.Run("Scenario_NoMatchAtAnyLayerReturnsExplicitNotFound", func(t *testing.T) {
		// Given a key has no fact at any reachable layer,
		// When Lookup runs,
		// Then it returns ErrMemoryHierarchyNotFound (sentinel for
		// callers to handle gracefully — fall back to default, etc).
		s := NewInMemoryMemoryHierarchyStore()
		_, _, err := s.Lookup(context.Background(), LookupContext{TenantID: "t"}, "never_set")
		assert.True(t, errors.Is(err, ErrMemoryHierarchyNotFound))
	})

	t.Run("Scenario_OverridingAtNarrowerLayerDoesNotMutateBroaderDefault", func(t *testing.T) {
		// Given session sets a temporary override of tenant default,
		// When the session ends and another session looks up without
		// any session-level fact,
		// Then the tenant default is intact (override was scoped, not
		// global).
		s := NewInMemoryMemoryHierarchyStore()
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeTenant, ScopeID: "t", Key: "k", Value: "tenant_default",
		})
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeSession, ScopeID: "s-1", Key: "k", Value: "session_override",
		})

		// Different session (no session-level fact for this session).
		got, _, _ := s.Lookup(context.Background(), LookupContext{SessionID: "s-2", TenantID: "t"}, "k")
		assert.Equal(t, "tenant_default", got.Value,
			"session override on s-1 must not bleed to s-2")
	})
}
