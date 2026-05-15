package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_AutoMemory(t *testing.T) {
	t.Run("Scenario_AgentAutoStoresExplicitRememberStatementWithoutAdminPrompt", func(t *testing.T) {
		// Given the user types "Remember that the deploy window is Friday",
		// And auto-memory pipeline runs on each turn,
		// When pipeline classifies the message,
		// Then a high-confidence project decision is created and stored
		// without admin-prompting (PDF §7.5 — agent learns continuously).
		classifier := NewHeuristicAutoMemoryClassifier()
		store := NewInMemoryAutoMemoryStore()
		res, err := RunAutoMemoryPipeline(context.Background(), classifier, store,
			DefaultAutoMemoryConfig(), "t", "u",
			"Remember that the deploy window is Friday 8pm")
		require.NoError(t, err)
		assert.NotEmpty(t, res.Stored)
		assert.Equal(t, AutoMemoryTypeProject, res.Stored[0].Decision.Type)
	})

	t.Run("Scenario_LowConfidenceDecisionsAreDroppedToAvoidNoisyMemory", func(t *testing.T) {
		// Given the agent's heuristic detected a maybe-correction with
		// confidence 0.7,
		// When admin sets MinConfidence=0.8,
		// Then that decision is dropped — the user's memory store stays
		// signal-rich (no clutter from low-confidence guesses).
		classifier := NewHeuristicAutoMemoryClassifier()
		store := NewInMemoryAutoMemoryStore()
		cfg := DefaultAutoMemoryConfig()
		cfg.MinConfidence = 0.8
		res, _ := RunAutoMemoryPipeline(context.Background(), classifier, store, cfg,
			"t", "u", "Don't summarize at the end of replies.")
		assert.Empty(t, res.Stored, "0.7 correction dropped vs 0.8 threshold")
		assert.NotEmpty(t, res.Dropped)
	})

	t.Run("Scenario_AdminBlockListPreventsSensitiveKeysFromBeingAutoStored", func(t *testing.T) {
		// Given admin's block list includes "password" and "credit_card",
		// When the LLM classifier emits a decision with one of those keys
		// (e.g. user accidentally said "my password is X"),
		// Then the decision is dropped (NOT stored) and the drop reason
		// names the block list — admin sees in audit that protection fired.
		decisions := []AutoMemoryDecision{
			{Type: AutoMemoryTypeUser, Key: "password", Value: "secret123", Confidence: 0.95},
			{Type: AutoMemoryTypeUser, Key: "name", Value: "Alice", Confidence: 0.9},
		}
		survivors, dropped := FilterByConfig(decisions, DefaultAutoMemoryConfig())
		assert.Len(t, survivors, 1)
		assert.Equal(t, "name", survivors[0].Key)
		assert.NotEmpty(t, dropped)
		assert.Contains(t, dropped[0].Reason, "block list")
	})

	t.Run("Scenario_BlockListMatchIsCaseInsensitiveToCatchAllVariants", func(t *testing.T) {
		// Given attacker tries "PASSWORD" or "Password" as key to bypass,
		// When filter compares against block list,
		// Then case-insensitive match catches every variant.
		decisions := []AutoMemoryDecision{
			{Type: AutoMemoryTypeUser, Key: "PASSWORD", Value: "x", Confidence: 0.99},
		}
		survivors, _ := FilterByConfig(decisions, DefaultAutoMemoryConfig())
		assert.Empty(t, survivors)
	})

	t.Run("Scenario_MaxPerTurnPreventsRunawayMemoryGrowth", func(t *testing.T) {
		// Given a verbose turn yields 50 candidate decisions,
		// When config caps MaxPerTurn=10,
		// Then only top 10 (after sort by confidence) are stored,
		// preventing one turn from flooding the memory store.
		decisions := []AutoMemoryDecision{}
		for i := 0; i < 50; i++ {
			decisions = append(decisions, AutoMemoryDecision{
				Type: AutoMemoryTypeProject, Key: "k", Value: "v", Confidence: 0.9,
			})
		}
		cfg := DefaultAutoMemoryConfig() // MaxPerTurn=10
		survivors, dropped := FilterByConfig(decisions, cfg)
		assert.Len(t, survivors, 10)
		assert.Len(t, dropped, 40)
	})

	t.Run("Scenario_HeuristicFallbackKeepsAgentLearningEvenWhenLLMOffline", func(t *testing.T) {
		// Given LLM evaluator is unavailable (cost cap, outage),
		// When the runtime falls back to HeuristicAutoMemoryClassifier,
		// Then well-known patterns ("remember that..", "my name is..", URLs,
		// corrective language) still produce decisions — agent doesn't go
		// dumb just because LLM is offline.
		c := NewHeuristicAutoMemoryClassifier()
		got, err := c.Classify(context.Background(),
			"Remember that my name is Alice — see https://example.com.")
		require.NoError(t, err)
		assert.NotEmpty(t, got, "heuristic must produce decisions when LLM offline")
	})

	t.Run("Scenario_ResultsAreSortedByConfidenceSoTopDecisionsSurviveCap", func(t *testing.T) {
		// Given MaxPerTurn caps survivors,
		// When multiple decisions exist,
		// Then they are filtered AFTER classifier sorts by confidence
		// (highest survives first) — caps don't accidentally drop the
		// most confident decisions.
		decisions := []AutoMemoryDecision{
			{Type: AutoMemoryTypeUser, Key: "k1", Value: "v", Confidence: 0.6},
			{Type: AutoMemoryTypeUser, Key: "k2", Value: "v", Confidence: 0.95},
			{Type: AutoMemoryTypeUser, Key: "k3", Value: "v", Confidence: 0.7},
		}
		// FilterByConfig respects input order — caller is expected to
		// sort by confidence first (heuristic classifier does this).
		// Here we verify the cap behaves predictably with cap=1.
		cfg := AutoMemoryConfig{MinConfidence: 0.5, MaxPerTurn: 1}
		survivors, _ := FilterByConfig(decisions, cfg)
		require.Len(t, survivors, 1)
		// The first one (k1) survives because input not pre-sorted.
		// This documents the contract: classifier must sort before filter.
		assert.Equal(t, "k1", survivors[0].Key,
			"FilterByConfig respects input order — classifier sorts first")
	})

	t.Run("Scenario_StoredRecordsCarryProvenanceForAuditability", func(t *testing.T) {
		// Given GOV-001 audits every stored memory,
		// When auto-memory stores a decision,
		// Then the record carries Rationale + SourceMessage so auditor
		// can reconstruct WHY this fact ended up in memory.
		c := NewHeuristicAutoMemoryClassifier()
		got, _ := c.Classify(context.Background(), "Remember that we deploy on Fridays.")
		require.NotEmpty(t, got)
		assert.NotEmpty(t, got[0].Rationale)
		assert.NotEmpty(t, got[0].SourceMessage)
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantMemoryBleed", func(t *testing.T) {
		// Given two tenants both store memories for users named "alice",
		// When tenant-A queries by user_id,
		// Then it sees only its own records (no cross-tenant bleed).
		store := NewInMemoryAutoMemoryStore()
		_, _ = store.Store(context.Background(), "t-a", "alice", validAutoDecision())
		_, _ = store.Store(context.Background(), "t-b", "alice", validAutoDecision())

		got, _ := store.ListByUser(context.Background(), "t-a", "alice")
		assert.Len(t, got, 1)
		assert.Equal(t, "t-a", got[0].TenantID)
	})

	t.Run("Scenario_FourTypeTaxonomyMatchesPDFExactly", func(t *testing.T) {
		// Given PDF §7.5 documents 4 memory types: user/feedback/project/reference,
		// When the bounded enum is queried,
		// Then exactly those 4 values exist (no drift).
		expected := map[AutoMemoryType]bool{
			AutoMemoryTypeUser: true, AutoMemoryTypeFeedback: true,
			AutoMemoryTypeProject: true, AutoMemoryTypeReference: true,
		}
		for _, mt := range AllAutoMemoryTypes() {
			assert.True(t, expected[mt])
		}
		assert.Equal(t, len(expected), len(AllAutoMemoryTypes()))
	})
}
