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

func validHierFact() HierarchicalFact {
	return HierarchicalFact{
		Scope:   MemoryHierarchyScopeAgent,
		ScopeID: "agent-7",
		Key:     "tone",
		Value:   "concise",
		Source:  "agent_default",
	}
}

func TestMemoryHierarchy_ScopeEnumIsBounded(t *testing.T) {
	for _, s := range AllMemoryHierarchyScopes() {
		assert.True(t, IsValidMemoryHierarchyScope(s))
	}
	assert.False(t, IsValidMemoryHierarchyScope(MemoryHierarchyScope("workspace")))
	assert.Equal(t, 5, len(AllMemoryHierarchyScopes()))
}

func TestMemoryHierarchy_ScopeOrderIsNarrowToWide(t *testing.T) {
	expected := []MemoryHierarchyScope{
		MemoryHierarchyScopeSession,
		MemoryHierarchyScopeUser,
		MemoryHierarchyScopeAgent,
		MemoryHierarchyScopeTenant,
		MemoryHierarchyScopeGlobal,
	}
	assert.Equal(t, expected, AllMemoryHierarchyScopes())
}

func TestHierarchicalFact_IsStale_ZeroMaxAgeNeverExpires(t *testing.T) {
	f := HierarchicalFact{SetAt: time.Now().Add(-100 * 24 * time.Hour), MaxAge: 0}
	assert.False(t, f.IsStale(time.Now()))
}

func TestHierarchicalFact_IsStale_ExpiresAfterMaxAge(t *testing.T) {
	now := time.Now()
	f := HierarchicalFact{SetAt: now.Add(-2 * time.Hour), MaxAge: time.Hour}
	assert.True(t, f.IsStale(now))
}

func TestHierarchy_Set_AssignsTimestampWhenZero(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	saved, err := s.Set(context.Background(), validHierFact())
	require.NoError(t, err)
	assert.False(t, saved.SetAt.IsZero())
}

func TestHierarchy_Set_RejectsInvalidScope(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	f := validHierFact()
	f.Scope = "rogue"
	_, err := s.Set(context.Background(), f)
	assert.True(t, errors.Is(err, ErrMemoryHierarchyInvalidScope))
}

func TestHierarchy_Set_RejectsEmptyKey(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	f := validHierFact()
	f.Key = " "
	_, err := s.Set(context.Background(), f)
	assert.True(t, errors.Is(err, ErrMemoryHierarchyKeyEmpty))
}

func TestHierarchy_Set_RejectsEmptyValue(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	f := validHierFact()
	f.Value = " "
	_, err := s.Set(context.Background(), f)
	assert.True(t, errors.Is(err, ErrMemoryHierarchyValueEmpty))
}

func TestHierarchy_Set_RejectsMissingScopeIDExceptGlobal(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	for _, scope := range []MemoryHierarchyScope{
		MemoryHierarchyScopeSession, MemoryHierarchyScopeUser,
		MemoryHierarchyScopeAgent, MemoryHierarchyScopeTenant,
	} {
		f := validHierFact()
		f.Scope = scope
		f.ScopeID = ""
		_, err := s.Set(context.Background(), f)
		assert.True(t, errors.Is(err, ErrMemoryHierarchyScopeIDEmpty),
			"scope %q must require scope_id", scope)
	}

	// Global allows empty scope_id.
	f := validHierFact()
	f.Scope = MemoryHierarchyScopeGlobal
	f.ScopeID = ""
	_, err := s.Set(context.Background(), f)
	assert.NoError(t, err)
}

func TestHierarchy_Set_RejectsNegativeMaxAge(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	f := validHierFact()
	f.MaxAge = -time.Hour
	_, err := s.Set(context.Background(), f)
	assert.Error(t, err)
}

func TestHierarchy_Lookup_MostSpecificWins(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	// Layered facts on same key:
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeGlobal, Key: "tone", Value: "neutral",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeTenant, ScopeID: "t-1", Key: "tone", Value: "formal",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeAgent, ScopeID: "a-1", Key: "tone", Value: "casual",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeUser, ScopeID: "u-1", Key: "tone", Value: "terse",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeSession, ScopeID: "s-1", Key: "tone", Value: "verbose",
	})
	lc := LookupContext{SessionID: "s-1", UserID: "u-1", AgentID: "a-1", TenantID: "t-1"}
	got, trace, err := s.Lookup(context.Background(), lc, "tone")
	require.NoError(t, err)
	assert.Equal(t, "verbose", got.Value, "session-scope wins")
	assert.Equal(t, MemoryHierarchyScopeSession, trace.Matched)
}

func TestHierarchy_Lookup_FallsBackThroughLayers(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeTenant, ScopeID: "t-1", Key: "tone", Value: "formal",
	})
	lc := LookupContext{SessionID: "s-1", UserID: "u-1", AgentID: "a-1", TenantID: "t-1"}
	got, trace, err := s.Lookup(context.Background(), lc, "tone")
	require.NoError(t, err)
	assert.Equal(t, "formal", got.Value)
	assert.Equal(t, MemoryHierarchyScopeTenant, trace.Matched)
}

func TestHierarchy_Lookup_StaleSkipped(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	now := time.Now()
	s.SetClock(func() time.Time { return now })

	// Stale session-level (would normally win).
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeSession, ScopeID: "s-1",
		Key: "tone", Value: "stale_value",
		SetAt:  now.Add(-2 * time.Hour),
		MaxAge: time.Hour,
	})
	// Live tenant-level (fallback).
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeTenant, ScopeID: "t-1",
		Key: "tone", Value: "live_value",
		SetAt: now,
	})

	lc := LookupContext{SessionID: "s-1", TenantID: "t-1"}
	got, trace, err := s.Lookup(context.Background(), lc, "tone")
	require.NoError(t, err)
	assert.Equal(t, "live_value", got.Value)
	assert.Equal(t, MemoryHierarchyScopeTenant, trace.Matched)

	// Trace shows session probe was found-but-stale.
	for _, p := range trace.Scopes {
		if p.Scope == MemoryHierarchyScopeSession {
			assert.True(t, p.Found)
			assert.True(t, p.Stale)
		}
	}
}

func TestHierarchy_Lookup_SkipsScopesWithoutScopeID(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeAgent, ScopeID: "a-1", Key: "tone", Value: "x",
	})
	// Lookup with no SessionID, UserID — those scopes should be skipped, not error.
	lc := LookupContext{AgentID: "a-1"}
	got, trace, err := s.Lookup(context.Background(), lc, "tone")
	require.NoError(t, err)
	assert.Equal(t, "x", got.Value)
	for _, p := range trace.Scopes {
		switch p.Scope {
		case MemoryHierarchyScopeSession, MemoryHierarchyScopeUser, MemoryHierarchyScopeTenant:
			assert.True(t, p.Skipped)
		}
	}
}

func TestHierarchy_Lookup_NotFoundWhenNoLayerMatches(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	lc := LookupContext{TenantID: "t-1"}
	_, _, err := s.Lookup(context.Background(), lc, "missing")
	assert.True(t, errors.Is(err, ErrMemoryHierarchyNotFound))
}

func TestHierarchy_Lookup_RejectsEmptyKey(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _, err := s.Lookup(context.Background(), LookupContext{}, "")
	assert.True(t, errors.Is(err, ErrMemoryHierarchyKeyEmpty))
}

func TestHierarchy_Lookup_GlobalIsAlwaysAvailable(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeGlobal, Key: "platform_name", Value: "AgentHub",
	})
	got, _, err := s.Lookup(context.Background(), LookupContext{}, "platform_name")
	require.NoError(t, err)
	assert.Equal(t, "AgentHub", got.Value)
}

func TestHierarchy_LookupAll_ReturnsLiveLayersNarrowToWide(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeGlobal, Key: "k", Value: "g",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeUser, ScopeID: "u-1", Key: "k", Value: "u",
	})
	got, err := s.LookupAll(context.Background(), LookupContext{UserID: "u-1"}, "k")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, MemoryHierarchyScopeUser, got[0].Scope)
	assert.Equal(t, MemoryHierarchyScopeGlobal, got[1].Scope)
}

func TestHierarchy_LookupAll_ExcludesStale(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	now := time.Now()
	s.SetClock(func() time.Time { return now })
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeUser, ScopeID: "u", Key: "k", Value: "stale",
		SetAt: now.Add(-2 * time.Hour), MaxAge: time.Hour,
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeGlobal, Key: "k", Value: "live",
	})
	got, _ := s.LookupAll(context.Background(), LookupContext{UserID: "u"}, "k")
	require.Len(t, got, 1)
	assert.Equal(t, "live", got[0].Value)
}

func TestHierarchy_Delete_RemovesFact(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), validHierFact())
	require.NoError(t, s.Delete(context.Background(), MemoryHierarchyScopeAgent, "agent-7", "tone"))
	_, _, err := s.Lookup(context.Background(), LookupContext{AgentID: "agent-7"}, "tone")
	assert.True(t, errors.Is(err, ErrMemoryHierarchyNotFound))
}

func TestHierarchy_Delete_UnknownReturnsNotFound(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	err := s.Delete(context.Background(), MemoryHierarchyScopeAgent, "missing", "x")
	assert.True(t, errors.Is(err, ErrMemoryHierarchyNotFound))
}

func TestHierarchy_ListByScope_OrderedByKey(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	for _, k := range []string{"zeta", "alpha", "mid"} {
		_, _ = s.Set(context.Background(), HierarchicalFact{
			Scope: MemoryHierarchyScopeAgent, ScopeID: "a", Key: k, Value: "v",
		})
	}
	got, err := s.ListByScope(context.Background(), MemoryHierarchyScopeAgent, "a")
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Less(t, got[i-1].Key, got[i].Key)
	}
}

func TestHierarchy_ListByScope_TenantIsolation(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeTenant, ScopeID: "t-a", Key: "k", Value: "v1",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeTenant, ScopeID: "t-b", Key: "k", Value: "v2",
	})
	got, _ := s.ListByScope(context.Background(), MemoryHierarchyScopeTenant, "t-a")
	require.Len(t, got, 1)
	assert.Equal(t, "v1", got[0].Value)
}

func TestHierarchy_PurgeStale_RemovesExpired(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	now := time.Now()
	s.SetClock(func() time.Time { return now })

	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeAgent, ScopeID: "a", Key: "live", Value: "x",
	})
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope:  MemoryHierarchyScopeAgent, ScopeID: "a", Key: "stale", Value: "x",
		SetAt:  now.Add(-2 * time.Hour),
		MaxAge: time.Hour,
	})
	count, err := s.PurgeStale(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	remaining, _ := s.ListByScope(context.Background(), MemoryHierarchyScopeAgent, "a")
	assert.Len(t, remaining, 1)
}

func TestHierarchy_TraceRecordsAllProbesWhenNoMatch(t *testing.T) {
	// Lookup short-circuits on the first live match. To verify all
	// scopes ARE probed in narrow→wide order, query a key that exists
	// at no layer — trace then walks the full chain.
	s := NewInMemoryMemoryHierarchyStore()
	lc := LookupContext{SessionID: "s-1", UserID: "u-1", AgentID: "a-1", TenantID: "t-1"}
	_, trace, _ := s.Lookup(context.Background(), lc, "never_set")
	assert.Equal(t, 5, len(trace.Scopes))
	scopes := []MemoryHierarchyScope{}
	for _, p := range trace.Scopes {
		scopes = append(scopes, p.Scope)
	}
	assert.Equal(t, AllMemoryHierarchyScopes(), scopes)
}

func TestHierarchy_TraceShortCircuitsOnFirstMatch(t *testing.T) {
	// Documented behavior: once Lookup finds a live fact, it returns
	// immediately. The trace records only scopes WALKED up to and
	// including the matching one — wider scopes are not probed.
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), HierarchicalFact{
		Scope: MemoryHierarchyScopeAgent, ScopeID: "a-1", Key: "k", Value: "v",
	})
	lc := LookupContext{SessionID: "s-1", UserID: "u-1", AgentID: "a-1", TenantID: "t-1"}
	_, trace, _ := s.Lookup(context.Background(), lc, "k")
	// session, user, agent (matched) — tenant + global skipped.
	assert.Equal(t, 3, len(trace.Scopes))
	assert.Equal(t, MemoryHierarchyScopeAgent, trace.Matched)
	assert.Equal(t, MemoryHierarchyScopeAgent, trace.Scopes[len(trace.Scopes)-1].Scope)
}

func TestHierarchy_ConcurrentSetIsSafe(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Set(context.Background(), HierarchicalFact{
				Scope:   MemoryHierarchyScopeAgent,
				ScopeID: "a",
				Key:     "k",
				Value:   "v",
				SetAt:   time.Now().Add(time.Duration(i) * time.Millisecond),
			})
		}()
	}
	wg.Wait()
	got, _ := s.ListByScope(context.Background(), MemoryHierarchyScopeAgent, "a")
	assert.Equal(t, 1, len(got), "same key→last write wins; no panic")
}

func TestHierarchy_ContextCancelled(t *testing.T) {
	s := NewInMemoryMemoryHierarchyStore()
	_, _ = s.Set(context.Background(), validHierFact())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Set(ctx, validHierFact())
	assert.Error(t, err)
	_, _, err = s.Lookup(ctx, LookupContext{}, "k")
	assert.Error(t, err)
	_, err = s.LookupAll(ctx, LookupContext{}, "k")
	assert.Error(t, err)
	err = s.Delete(ctx, MemoryHierarchyScopeAgent, "a", "k")
	assert.Error(t, err)
	_, err = s.ListByScope(ctx, MemoryHierarchyScopeAgent, "a")
	assert.Error(t, err)
	_, err = s.PurgeStale(ctx)
	assert.Error(t, err)
}
