package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FUTURE-002 — Longitudinal user-agent relationship state BDD.
//
// PDF arXiv:2604.14228v1 §12 (long-term agent-user relationships beyond
// memory facts: trust, rapport, communication style accumulate over time).

func TestBDD_UserAgentRelationship(t *testing.T) {

	t.Run("Scenario_FirstInteractionStartsAtUnknownTrust", func(t *testing.T) {
		// Given a brand-new user-agent pair,
		store := NewInMemoryRelationshipStore()
		r, err := store.Save(context.Background(), UserAgentRelationship{
			TenantID: "t", UserID: "alice-new", AgentID: "support-bot",
		})
		require.NoError(t, err)
		assert.Equal(t, TrustLevelUnknown, r.TrustLevel,
			"fresh pair = unknown trust until interaction")
	})

	t.Run("Scenario_TrustPromotesGraduallyWithPositiveInteractions", func(t *testing.T) {
		// Given the user has 10 positive interactions (tutorial → established),
		store := NewInMemoryRelationshipStore()
		for i := 0; i < 10; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		}
		r, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.Equal(t, TrustLevelEstablished, r.TrustLevel,
			"10 positives reaches established")
	})

	t.Run("Scenario_FullyTrustedAfterExtendedPositiveHistory", func(t *testing.T) {
		// Given 50 sustained positive interactions = trusted user,
		store := NewInMemoryRelationshipStore()
		for i := 0; i < 50; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		}
		r, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.Equal(t, TrustLevelTrusted, r.TrustLevel)
	})

	t.Run("Scenario_SustainedEscalationsDemoteToMistrusted", func(t *testing.T) {
		// Given the user causes multiple escalations (jailbreak attempts,
		//       repeated abuse),
		store := NewInMemoryRelationshipStore()
		for i := 0; i < 5; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventEscalation)
		}
		r, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.Equal(t, TrustLevelMistrusted, r.TrustLevel)
	})

	t.Run("Scenario_MistrustedRequiresExplicitAdminResetNotAutoRecovery", func(t *testing.T) {
		// Given a mistrusted user later sends positive signals,
		// When the events accumulate,
		// Then trust does NOT auto-recover — requires admin intervention.
		store := NewInMemoryRelationshipStore()
		for i := 0; i < 5; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventEscalation)
		}
		// 20 positives → would normally promote to trusted.
		for i := 0; i < 20; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		}
		r, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.Equal(t, TrustLevelMistrusted, r.TrustLevel,
			"mistrusted = sticky; admin must reset explicitly")
	})

	t.Run("Scenario_RapportScoreClampedToValidRange", func(t *testing.T) {
		// Given many positive events would overflow,
		store := NewInMemoryRelationshipStore()
		for i := 0; i < 100; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		}
		r, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.LessOrEqual(t, r.RapportScore, 1.0)
		assert.GreaterOrEqual(t, r.RapportScore, -1.0)
	})

	t.Run("Scenario_ConflictResolvedAcceleratesTrustRecovery", func(t *testing.T) {
		// Given a negative interaction was followed by conflict_resolved,
		store := NewInMemoryRelationshipStore()
		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventNegative)
		mid, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.Less(t, mid.RapportScore, 0.0)

		_, _ = store.RecordEvent(context.Background(), "t", "u", "a", RapportEventConflictResolved)
		after, _ := store.FindByPair(context.Background(), "t", "u", "a")
		assert.InDelta(t, 0.0, after.RapportScore, 0.001,
			"conflict_resolved offsets the prior negative")
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantLeakage", func(t *testing.T) {
		// Given two tenants share user-id 'alice' (different humans),
		store := NewInMemoryRelationshipStore()
		_, _ = store.RecordEvent(context.Background(), "tenant-A", "alice", "bot", RapportEventPositive)
		_, err := store.FindByPair(context.Background(), "tenant-B", "alice", "bot")
		assert.Error(t, err, "tenant B's alice doesn't see tenant A's relationship")
	})

	t.Run("Scenario_FiveTrustLevelsCoverLifecycle", func(t *testing.T) {
		// Stable wire strings.
		expected := map[string]bool{
			"unknown": true, "probationary": true, "established": true,
			"trusted": true, "mistrusted": true,
		}
		for _, l := range AllTrustLevels() {
			assert.True(t, expected[string(l)])
		}
	})

	t.Run("Scenario_FiveRapportEventsClassifyInteractionOutcomes", func(t *testing.T) {
		expected := map[string]bool{
			"positive": true, "neutral": true, "negative": true,
			"conflict_resolved": true, "escalation": true,
		}
		for _, e := range AllRapportEvents() {
			assert.True(t, expected[string(e)])
		}
	})

	t.Run("Scenario_FiveCommunicationStylesPersonalizeAgentResponses", func(t *testing.T) {
		// Given the agent uses CommunicationStyle to personalize tone,
		expected := map[string]bool{
			"unspecified": true, "formal": true, "casual": true,
			"terse": true, "verbose": true,
		}
		for _, s := range AllCommunicationStyles() {
			assert.True(t, expected[string(s)])
		}
	})

	t.Run("Scenario_HistogramByTrustEnablesAdminDashboard", func(t *testing.T) {
		// Given an admin wants to see distribution of users by trust,
		store := NewInMemoryRelationshipStore()
		_, _ = store.RecordEvent(context.Background(), "t", "u1", "a", RapportEventPositive)
		_, _ = store.RecordEvent(context.Background(), "t", "u2", "a", RapportEventPositive)
		// Drive u3 to mistrusted.
		for i := 0; i < 5; i++ {
			_, _ = store.RecordEvent(context.Background(), "t", "u3", "a", RapportEventEscalation)
		}
		hist, _ := store.CountByTrust(context.Background(), "t")
		assert.Equal(t, 2, hist[TrustLevelProbationary])
		assert.Equal(t, 1, hist[TrustLevelMistrusted])
		assert.Equal(t, 0, hist[TrustLevelTrusted], "stable axes")
	})

	t.Run("Scenario_UpdatePreservesIDAndStartedAtForAuditTrail", func(t *testing.T) {
		// Given the relationship is the audit trail's anchor,
		store := NewInMemoryRelationshipStore()
		first, _ := store.Save(context.Background(), validRelationship())
		// Mutate.
		updated := validRelationship()
		updated.RapportScore = 0.7
		updated.CommunicationStyle = CommunicationStyleCasual
		got, _ := store.Save(context.Background(), updated)
		assert.Equal(t, first.ID, got.ID)
		assert.Equal(t, first.StartedAt, got.StartedAt)
	})

	t.Run("Scenario_RecordEventAutoCreatesRelationshipFirstTime", func(t *testing.T) {
		// Given the runtime calls RecordEvent without prior Save,
		store := NewInMemoryRelationshipStore()
		r, err := store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		require.NoError(t, err)
		assert.Equal(t, 1, r.InteractionCount)
		assert.False(t, r.StartedAt.IsZero())
	})

	t.Run("Scenario_LastInteractionAtTouchedOnEachEvent", func(t *testing.T) {
		// Given dashboards show "users active in last 30 days",
		store := NewInMemoryRelationshipStore()
		r, _ := store.RecordEvent(context.Background(), "t", "u", "a", RapportEventPositive)
		assert.False(t, r.LastInteractionAt.IsZero())
	})
}
