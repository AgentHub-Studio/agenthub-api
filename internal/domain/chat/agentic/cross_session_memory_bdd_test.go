package agentic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FUTURE-001 — Cross-session memory substrate BDD.
//
// PDF arXiv:2604.14228v1 §12 (Future Directions: persistent memory
// across sessions; agent recalls across runs).

func TestBDD_CrossSessionMemorySubstrate(t *testing.T) {

	t.Run("Scenario_AgentRecallsUserPreferenceFromPriorSession", func(t *testing.T) {
		// Given a user told the agent "I prefer responses in Portuguese"
		//       in session A,
		// When session B starts and the agent recalls user preferences,
		// Then the Portuguese preference is available — agent doesn't
		//      have to re-ask.
		sub := NewInMemoryCrossSessionMemorySubstrate()
		_, _ = sub.Save(context.Background(), CrossSessionMemoryEntry{
			TenantID: "tenant-acme", Scope: MemoryScopeUser,
			Subject: "user-alice", Kind: MemoryKindPreference,
			Topic: "language", Content: "Alice prefers responses in Portuguese (BR)",
			SourceRunID: "run-A",
		})
		got, err := sub.Recall(context.Background(), MemoryRecallQuery{
			TenantID: "tenant-acme", Scope: MemoryScopeUser, Subject: "user-alice",
		})
		require.NoError(t, err)
		require.NotEmpty(t, got)
		assert.Contains(t, got[0].Content, "Portuguese")
	})

	t.Run("Scenario_TenantGlobalFactSharedAcrossAllAgentsAndUsers", func(t *testing.T) {
		// Given a tenant-wide fact (e.g. "fiscal year Jul-Jun"),
		// When ANY agent recalls tenant_global memory,
		// Then the fact is available regardless of subject.
		sub := NewInMemoryCrossSessionMemorySubstrate()
		_, _ = sub.Save(context.Background(), CrossSessionMemoryEntry{
			TenantID: "t", Scope: MemoryScopeTenantGlobal,
			Kind: MemoryKindFact, Topic: "fiscal_year",
			Content: "Tenant uses fiscal year Jul-Jun",
		})
		got, _ := sub.Recall(context.Background(), MemoryRecallQuery{
			TenantID: "t", Scope: MemoryScopeTenantGlobal,
		})
		require.NotEmpty(t, got)
		assert.Contains(t, got[0].Content, "Jul-Jun")
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantRecall", func(t *testing.T) {
		sub := NewInMemoryCrossSessionMemorySubstrate()
		_, _ = sub.Save(context.Background(), validUserMemory())
		got, _ := sub.Recall(context.Background(), MemoryRecallQuery{
			TenantID: "other-tenant", Scope: MemoryScopeUser, Subject: "user-alice",
		})
		assert.Empty(t, got, "cross-tenant leak forbidden")
	})

	t.Run("Scenario_ScopeBoundEnforcesUserVsTenantSemanticDistinction", func(t *testing.T) {
		// Given user scope requires Subject (whose preference?),
		//       tenant_global forbids Subject (it's everyone's),
		sub := NewInMemoryCrossSessionMemorySubstrate()

		// User without subject → reject.
		userNoSubject := validUserMemory()
		userNoSubject.Subject = ""
		_, err := sub.Save(context.Background(), userNoSubject)
		assert.Error(t, err)

		// Tenant global with subject → reject.
		globalWithSubject := validTenantGlobalMemory()
		globalWithSubject.Subject = "should-not"
		_, err = sub.Save(context.Background(), globalWithSubject)
		assert.Error(t, err)
	})

	t.Run("Scenario_NewerEvidenceSupersedesOldMemory", func(t *testing.T) {
		// Given user originally said "I prefer Postgres",
		// When user later says "actually I prefer MySQL now",
		// Then the new memory supersedes the old; recall returns ONLY new.
		sub := NewInMemoryCrossSessionMemorySubstrate()
		old, _ := sub.Save(context.Background(), CrossSessionMemoryEntry{
			TenantID: "t", Scope: MemoryScopeUser, Subject: "u",
			Kind: MemoryKindPreference, Topic: "db", Content: "Postgres",
		})
		newE, _ := sub.Save(context.Background(), CrossSessionMemoryEntry{
			TenantID: "t", Scope: MemoryScopeUser, Subject: "u",
			Kind: MemoryKindPreference, Topic: "db", Content: "MySQL",
		})
		require.NoError(t, sub.Supersede(context.Background(), old.ID, newE.ID))

		got, _ := sub.Recall(context.Background(), MemoryRecallQuery{
			TenantID: "t", Scope: MemoryScopeUser, Subject: "u", Topic: "db",
		})
		require.Len(t, got, 1)
		assert.Equal(t, "MySQL", got[0].Content,
			"recall returns the current memory, not the superseded one")
	})

	t.Run("Scenario_FirstSupersedeWinsAuditGuarantee", func(t *testing.T) {
		// Given audit requires once-superseded-stays-superseded,
		sub := NewInMemoryCrossSessionMemorySubstrate()
		old, _ := sub.Save(context.Background(), validUserMemory())
		first, _ := sub.Save(context.Background(), validUserMemory())
		second, _ := sub.Save(context.Background(), validUserMemory())
		require.NoError(t, sub.Supersede(context.Background(), old.ID, first.ID))
		err := sub.Supersede(context.Background(), old.ID, second.ID)
		assert.True(t, errors.Is(err, ErrMemoryAlreadySuperseded))
	})

	t.Run("Scenario_ArchivedMemoriesAreInvisibleButPreserved", func(t *testing.T) {
		// Given retention policy archives stale memories,
		sub := NewInMemoryCrossSessionMemorySubstrate()
		saved, _ := sub.Save(context.Background(), validUserMemory())
		_, _ = sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))

		// Recall excludes archived.
		got, _ := sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x"})
		assert.Empty(t, got)

		// FindByID still returns it (audit trail intact).
		archived, err := sub.FindByID(context.Background(), saved.ID)
		require.NoError(t, err)
		assert.True(t, archived.Archived)
	})

	t.Run("Scenario_RecallTouchesLastRecalledAtForRetentionPolicy", func(t *testing.T) {
		// Given retention archives entries not recalled in N months,
		// When the agent recalls a memory,
		// Then LastRecalledAt is updated — recently-used memories
		//      survive future archive sweeps.
		sub := NewInMemoryCrossSessionMemorySubstrate()
		saved, _ := sub.Save(context.Background(), validUserMemory())
		_, _ = sub.Recall(context.Background(), MemoryRecallQuery{TenantID: "tenant-x"})

		got, _ := sub.FindByID(context.Background(), saved.ID)
		assert.False(t, got.LastRecalledAt.IsZero(),
			"recall must touch LastRecalledAt — drives retention policy")
	})

	t.Run("Scenario_ConfidenceDefaultsTo1ButCanBeLowered", func(t *testing.T) {
		// Given the agent isn't sure of a fact,
		sub := NewInMemoryCrossSessionMemorySubstrate()
		uncertain := validUserMemory()
		uncertain.Confidence = 0.6
		saved, _ := sub.Save(context.Background(), uncertain)
		assert.InDelta(t, 0.6, saved.Confidence, 0.001)

		certain := validUserMemory()
		certain.Confidence = 0 // omitted → default
		saved2, _ := sub.Save(context.Background(), certain)
		assert.InDelta(t, 1.0, saved2.Confidence, 0.001,
			"omitted confidence defaults to 1.0")
	})

	t.Run("Scenario_HistogramByScopeProvidesUsageDashboard", func(t *testing.T) {
		// Given an admin wants to see "how many user vs tenant memories",
		sub := NewInMemoryCrossSessionMemorySubstrate()
		_, _ = sub.Save(context.Background(), validUserMemory())
		_, _ = sub.Save(context.Background(), validUserMemory())
		_, _ = sub.Save(context.Background(), validTenantGlobalMemory())

		hist, _ := sub.CountByScope(context.Background(), "tenant-x")
		assert.Equal(t, 2, hist[MemoryScopeUser])
		assert.Equal(t, 1, hist[MemoryScopeTenantGlobal])
		assert.Equal(t, 0, hist[MemoryScopeAgent], "stable axes — agent kind shows 0")
	})

	t.Run("Scenario_CompactRecallSummaryFitsAgentSystemPromptBudget", func(t *testing.T) {
		// Given the runner prepends a compact recall summary to the
		//       system prompt at session start (token budget critical),
		entries := []CrossSessionMemoryEntry{}
		for i := 0; i < 20; i++ {
			entries = append(entries, CrossSessionMemoryEntry{
				Scope: MemoryScopeUser, Kind: MemoryKindFact, Topic: "x", Content: "y",
			})
		}
		out := CompactRecallSummary(entries, 5)
		assert.Contains(t, out, "Cross-session memory:")
		assert.Contains(t, out, "(... 15 more entries elided)",
			"summary respects max-entries limit + signals truncation")
	})

	t.Run("Scenario_FiveMemoryKindsCoverCommonExtractionCategories", func(t *testing.T) {
		// Mirror auto-memory taxonomy.
		set := map[string]bool{}
		for _, k := range AllMemoryKinds() {
			set[string(k)] = true
		}
		for _, want := range []string{
			"fact", "preference", "feedback", "reference", "relationship",
		} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_ThreeScopesCoverUserAgentAndTenant", func(t *testing.T) {
		set := map[string]bool{}
		for _, s := range AllMemoryScopes() {
			set[string(s)] = true
		}
		for _, want := range []string{"user", "agent", "tenant_global"} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_ArchiveIdempotentAcrossPasses", func(t *testing.T) {
		sub := NewInMemoryCrossSessionMemorySubstrate()
		_, _ = sub.Save(context.Background(), validUserMemory())
		first, err := sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 1, first)

		second, err := sub.ArchiveExpired(context.Background(), "tenant-x", time.Now().Add(time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0, second, "second pass = no-op (idempotent)")
	})
}
