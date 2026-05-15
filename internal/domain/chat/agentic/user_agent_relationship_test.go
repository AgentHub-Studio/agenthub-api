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

func validRelationship() UserAgentRelationship {
	return UserAgentRelationship{
		TenantID: "tenant-x", UserID: "user-alice", AgentID: "agent-y",
	}
}

func TestRelationship_TrustLevelEnumIsBounded(t *testing.T) {
	for _, l := range AllTrustLevels() {
		assert.True(t, IsValidTrustLevel(l))
	}
	assert.False(t, IsValidTrustLevel(TrustLevel("vip")))
}

func TestRelationship_AllTrustLevelsCount(t *testing.T) {
	// 5 levels: unknown/probationary/established/trusted/mistrusted.
	assert.Equal(t, 5, len(AllTrustLevels()))
}

func TestRelationship_RapportEventEnumIsBounded(t *testing.T) {
	for _, e := range AllRapportEvents() {
		assert.True(t, IsValidRapportEvent(e))
	}
	assert.False(t, IsValidRapportEvent(RapportEvent("anger")))
}

func TestRelationship_AllRapportEventsCount(t *testing.T) {
	// 5 events.
	assert.Equal(t, 5, len(AllRapportEvents()))
}

func TestRelationship_CommunicationStyleEnumIsBounded(t *testing.T) {
	for _, s := range AllCommunicationStyles() {
		assert.True(t, IsValidCommunicationStyle(s))
	}
	assert.False(t, IsValidCommunicationStyle(CommunicationStyle("rude")))
}

func TestRelationship_Save_AssignsIDAndStartedAt(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	saved, err := store.Save(context.Background(), validRelationship())
	require.NoError(t, err)
	assert.NotEqual(t, "", saved.ID.String())
	assert.False(t, saved.StartedAt.IsZero())
}

func TestRelationship_Save_PreservesIDAndStartedAtOnUpdate(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	r1, _ := store.Save(context.Background(), validRelationship())
	originalID := r1.ID
	originalStarted := r1.StartedAt

	time.Sleep(10 * time.Millisecond)

	r2 := validRelationship()
	r2.RapportScore = 0.5
	saved, err := store.Save(context.Background(), r2)
	require.NoError(t, err)
	assert.Equal(t, originalID, saved.ID, "ID preserved on update")
	assert.Equal(t, originalStarted, saved.StartedAt, "StartedAt preserved on update")
	assert.InDelta(t, 0.5, saved.RapportScore, 0.001, "new value applied")
}

func TestRelationship_Save_RejectsEmptyTenantUserAgent(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	for _, missing := range []string{"tenant", "user", "agent"} {
		r := validRelationship()
		switch missing {
		case "tenant":
			r.TenantID = ""
		case "user":
			r.UserID = ""
		case "agent":
			r.AgentID = ""
		}
		_, err := store.Save(context.Background(), r)
		assert.Error(t, err, "missing %s must error", missing)
	}
}

func TestRelationship_Save_DefaultsTrustToUnknown(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	saved, _ := store.Save(context.Background(), validRelationship())
	assert.Equal(t, TrustLevelUnknown, saved.TrustLevel)
}

func TestRelationship_Save_DefaultsStyleToUnspecified(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	saved, _ := store.Save(context.Background(), validRelationship())
	assert.Equal(t, CommunicationStyleUnspecified, saved.CommunicationStyle)
}

func TestRelationship_Save_RejectsInvalidTrust(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	r := validRelationship()
	r.TrustLevel = TrustLevel("vip")
	_, err := store.Save(context.Background(), r)
	assert.True(t, errors.Is(err, ErrInvalidTrustLevel))
}

func TestRelationship_Save_RejectsInvalidStyle(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	r := validRelationship()
	r.CommunicationStyle = CommunicationStyle("rude")
	_, err := store.Save(context.Background(), r)
	assert.True(t, errors.Is(err, ErrInvalidCommunicationStyle))
}

func TestRelationship_FindByPair_RoundTrips(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	saved, _ := store.Save(context.Background(), validRelationship())
	got, err := store.FindByPair(context.Background(), "tenant-x", "user-alice", "agent-y")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

func TestRelationship_FindByPair_UnknownReturnsErrNotFound(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	_, err := store.FindByPair(context.Background(), "x", "y", "z")
	assert.True(t, errors.Is(err, ErrRelationshipNotFound))
}

func TestRelationship_FindByPair_TenantIsolation(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	_, _ = store.Save(context.Background(), validRelationship())
	_, err := store.FindByPair(context.Background(), "different", "user-alice", "agent-y")
	assert.True(t, errors.Is(err, ErrRelationshipNotFound))
}

func TestRelationship_RecordEvent_AutoCreatesOnFirstEvent(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	r, err := store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InteractionCount)
	assert.InDelta(t, 0.05, r.RapportScore, 0.001)
	assert.Equal(t, TrustLevelProbationary, r.TrustLevel,
		"first event promotes to probationary")
}

func TestRelationship_RecordEvent_RejectsInvalidEvent(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	_, err := store.RecordEvent(context.Background(), "t", "u", "a", RapportEvent("anger"))
	assert.True(t, errors.Is(err, ErrInvalidRapportEvent))
}

func TestRelationship_RecordEvent_PromotionPath(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	// Drive 10 positive events.
	for i := 0; i < 10; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
	}
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.Equal(t, 10, r.InteractionCount)
	// 10 events × +0.05 = 0.5 rapport
	assert.InDelta(t, 0.5, r.RapportScore, 0.001)
	assert.Equal(t, TrustLevelEstablished, r.TrustLevel,
		"10 positive interactions → established")
}

func TestRelationship_RecordEvent_DriveToTrusted(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	for i := 0; i < 50; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
	}
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.Equal(t, TrustLevelTrusted, r.TrustLevel,
		"50 positive interactions → trusted")
	assert.LessOrEqual(t, r.RapportScore, 1.0, "clamped at +1.0")
}

func TestRelationship_RecordEvent_EscalationDemotesToMistrusted(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	// Drive multiple escalations to push score below -0.5.
	for i := 0; i < 5; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventEscalation)
	}
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.Equal(t, TrustLevelMistrusted, r.TrustLevel,
		"sustained escalations demote to mistrusted")
}

func TestRelationship_RecordEvent_MistrustedDoesNotPromoteOnPositive(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	for i := 0; i < 5; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventEscalation)
	}
	// Now flood with positives — mistrusted should NOT auto-recover.
	for i := 0; i < 20; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
	}
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.Equal(t, TrustLevelMistrusted, r.TrustLevel,
		"mistrusted requires explicit admin reset, not auto-recovery")
}

func TestRelationship_RecordEvent_RapportClampedAtOne(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	for i := 0; i < 100; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
	}
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.LessOrEqual(t, r.RapportScore, 1.0)
}

func TestRelationship_RecordEvent_RapportClampedAtNegativeOne(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	for i := 0; i < 50; i++ {
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventNegative)
	}
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.GreaterOrEqual(t, r.RapportScore, -1.0)
}

func TestRelationship_RecordEvent_NeutralIncrementsCountOnly(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	r, _ := store.RecordEvent(context.Background(), "t", "u", "a", RapportEventNeutral)
	assert.Equal(t, 1, r.InteractionCount)
	assert.InDelta(t, 0.0, r.RapportScore, 0.001)
}

func TestRelationship_RecordEvent_ConflictResolvedRecoversRapport(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventNegative)
	_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventConflictResolved)
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	// -0.10 + +0.10 = 0
	assert.InDelta(t, 0.0, r.RapportScore, 0.001)
}

func TestRelationship_RecordEvent_TouchesLastInteractionAt(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	r, _ := store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
	assert.False(t, r.LastInteractionAt.IsZero())
}

func TestRelationship_CountByTrust_AlwaysAllLevels(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	_, _ = store.RecordEvent(context.Background(), "t", "u1", "a", RapportEventPositive)
	_, _ = store.RecordEvent(context.Background(), "t", "u2", "a", RapportEventPositive)

	hist, _ := store.CountByTrust(context.Background(), "t")
	for _, l := range AllTrustLevels() {
		_, ok := hist[l]
		assert.True(t, ok)
	}
	assert.Equal(t, 2, hist[TrustLevelProbationary])
	assert.Equal(t, 0, hist[TrustLevelTrusted])
}

func TestRelationship_ConcurrentRecordIsSafe(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		}()
	}
	wg.Wait()
	r, _ := store.FindByPair(context.Background(), "t", "u", "a")
	assert.Equal(t, 50, r.InteractionCount,
		"all concurrent events recorded — no lost updates")
}

func TestRelationship_ContextCancelledOperationsError(t *testing.T) {
	store := NewInMemoryRelationshipStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Save(ctx, validRelationship())
	assert.Error(t, err)

	_, err = store.FindByPair(ctx, "t", "u", "a")
	assert.Error(t, err)

	_, err = store.RecordEvent(ctx, "t", "u", "a", RapportEventPositive)
	assert.Error(t, err)

	_, err = store.CountByTrust(ctx, "t")
	assert.Error(t, err)
}
