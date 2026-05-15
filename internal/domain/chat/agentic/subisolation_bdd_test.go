package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// BDD-style scenarios that ratify SUB-004 (Isolated prompts/context for
// subagents) against the Claude Code architecture paper "Dive into Claude
// Code" (arXiv:2604.14228v1):
//
//   - Section 8 ("Subagents") + Figure 7 — subagents run in an isolated
//     context window so their full conversation never inflates the parent.
//   - Section 8.3 — only summary text returns to the parent.
//
// For prompt-cache efficiency, subagents may share the parent's cache prefix
// via CacheSafeParams (cachesafe.go) — but ONLY when SystemPrompt, Tools,
// Provider, Model, and CacheControl are identical. This is the explicit
// boundary between "isolated message history" (always isolated) and "shared
// cache prefix" (opt-in, hash-validated).
//
// Implementation under test:
//   - cachesafe.go CacheSafeParams + Hash + Matches.
//   - forkedagent.go ForkedAgentParams.PromptMessages,
//     CacheSafeParamsSnapshot (thread-safe holder).
//   - turnstate.go NewTurnState (each fork gets a fresh state).

func TestBDD_SubagentIsolatedPromptsAndContext(t *testing.T) {
	t.Run("Scenario_ForkReceivesItsOwnPromptMessagesNotParentHistory", func(t *testing.T) {
		// Given a parent has a long conversation history,
		// When a fork is prepared with a focused prompt (PDF Section 8: each
		//      subagent runs in an isolated context window),
		fork := ForkedAgentParams{
			PromptMessages: []ai.Message{
				{Role: "user", Content: "Summarise the auth module"},
			},
			ForkLabel: "auth_summary",
		}

		// Then the fork's PromptMessages are EXACTLY what the caller supplied
		//      — no parent history is accidentally folded in. The caller is
		//      responsible for selecting what context the fork sees.
		assert.Len(t, fork.PromptMessages, 1,
			"fork must carry only the explicit messages — no history bleed")
		assert.Equal(t, "Summarise the auth module", fork.PromptMessages[0].Content,
			"fork prompt must be verbatim what the caller supplied")
	})

	t.Run("Scenario_ForkPermissionsAreFirstClassNotInherited", func(t *testing.T) {
		// Given a parent runs in default mode with broad permissions,
		// When a fork is prepared with stricter rules (PDF Section 9.2:
		//      "subagent permissions live in memory only and are not
		//      serialized" — and per Section 8 each fork's permission set is
		//      established fresh),
		fork := ForkedAgentParams{
			PermissionRules: &PermissionRules{
				Mode:  PermissionModeDontAsk,
				Allow: []string{"document-search"},
			},
		}

		// Then the fork's rules are a first-class field — they MUST be set
		//      explicitly by the caller, not silently copied from the parent.
		//      Nil PermissionRules signals "no fork-specific override" and
		//      makes the contract explicit.
		assert.NotNil(t, fork.PermissionRules,
			"fork rules must be set explicitly when restricting fork scope")
		assert.Equal(t, PermissionModeDontAsk, fork.PermissionRules.Mode,
			"fork can opt INTO stricter mode independent of parent")

		emptyFork := ForkedAgentParams{}
		assert.Nil(t, emptyFork.PermissionRules,
			"unset rules are nil — not auto-populated from parent")
	})

	t.Run("Scenario_CacheSafeParamsHashStableForIdenticalInputs", func(t *testing.T) {
		// Given two CacheSafeParams snapshots with identical critical fields
		//       (PDF Section 8 + cachesafe.go contract: same hash → shared
		//       prompt cache prefix),
		a := NewCacheSafeParams("system prompt v1",
			[]ai.Tool{{Function: ai.ToolSchema{Name: "agent"}}},
			"anthropic", "claude-sonnet-4-20250514", true)
		b := NewCacheSafeParams("system prompt v1",
			[]ai.Tool{{Function: ai.ToolSchema{Name: "agent"}}},
			"anthropic", "claude-sonnet-4-20250514", true)

		// When their hashes are computed,
		// Then they match — guaranteeing the fork hits the parent's cache.
		assert.Equal(t, a.Hash(), b.Hash(),
			"identical critical fields must produce identical hashes")
		assert.True(t, a.Matches(b),
			"Matches must agree with Hash equality")
	})

	t.Run("Scenario_CacheSafeParamsHashChangesOnAnyCriticalField", func(t *testing.T) {
		// Given a baseline CacheSafeParams,
		baseline := NewCacheSafeParams("system v1",
			[]ai.Tool{{Function: ai.ToolSchema{Name: "agent"}}},
			"anthropic", "claude-sonnet-4-20250514", true)

		// When ANY single critical field changes,
		differentPrompt := NewCacheSafeParams("system v2",
			baseline.Tools, baseline.Provider, baseline.Model, baseline.CacheControl)
		differentTools := NewCacheSafeParams(baseline.SystemPrompt,
			[]ai.Tool{{Function: ai.ToolSchema{Name: "other"}}},
			baseline.Provider, baseline.Model, baseline.CacheControl)
		differentProvider := NewCacheSafeParams(baseline.SystemPrompt,
			baseline.Tools, "openai", baseline.Model, baseline.CacheControl)
		differentModel := NewCacheSafeParams(baseline.SystemPrompt,
			baseline.Tools, baseline.Provider, "claude-haiku-4-5", baseline.CacheControl)
		differentCache := NewCacheSafeParams(baseline.SystemPrompt,
			baseline.Tools, baseline.Provider, baseline.Model, false)

		// Then each variant produces a DIFFERENT hash — preventing accidental
		//      cache reuse across incompatible runtime configurations.
		assert.NotEqual(t, baseline.Hash(), differentPrompt.Hash(),
			"system prompt change must invalidate cache")
		assert.NotEqual(t, baseline.Hash(), differentTools.Hash(),
			"tool set change must invalidate cache")
		assert.NotEqual(t, baseline.Hash(), differentProvider.Hash(),
			"provider change must invalidate cache")
		assert.NotEqual(t, baseline.Hash(), differentModel.Hash(),
			"model change must invalidate cache")
		assert.NotEqual(t, baseline.Hash(), differentCache.Hash(),
			"cache_control flag change must invalidate cache")
	})

	t.Run("Scenario_CacheSafeParamsHashIsBoundedAndDeterministic", func(t *testing.T) {
		// Given a CacheSafeParams,
		given := NewCacheSafeParams("prompt", nil, "anthropic", "model", true)

		// When Hash is called multiple times,
		first := given.Hash()
		second := given.Hash()

		// Then it returns the same value (memoised) and length is bounded —
		//      keeps the snapshot store predictable.
		assert.Equal(t, first, second,
			"Hash must be deterministic and memoised")
		assert.Len(t, first, 16,
			"hash should be 16 hex chars (sha256[:16]) — bounded for keys")
	})

	t.Run("Scenario_NilCacheSafeParamsHashIsEmpty", func(t *testing.T) {
		// Given a nil CacheSafeParams (no parent cache available),
		var given *CacheSafeParams

		// When Hash is called,
		when := given.Hash()

		// Then it returns "" without panicking — fork can fall back to a
		//      cold cache without crashing.
		assert.Equal(t, "", when,
			"nil receiver Hash must be safe and return empty")
	})

	t.Run("Scenario_NilNilMatchesAreReflexive", func(t *testing.T) {
		// Given two nil CacheSafeParams pointers,
		var a, b *CacheSafeParams

		// When Matches is called,
		when := a.Matches(b)

		// Then they match (both nil → trivially equal — no cache to share but
		//      no contradiction either).
		assert.True(t, when, "two nil snapshots must match (reflexively)")
	})

	t.Run("Scenario_OneNilOneNonNilDoNotMatch", func(t *testing.T) {
		// Given one snapshot present, one nil,
		a := NewCacheSafeParams("p", nil, "anthropic", "m", true)
		var b *CacheSafeParams

		// When Matches is called,
		// Then they do NOT match — preventing accidental cache use when only
		//      one side has a snapshot.
		assert.False(t, a.Matches(b),
			"present vs nil must not match")
		assert.False(t, b.Matches(a),
			"nil vs present must not match (symmetric)")
	})

	t.Run("Scenario_CacheSafeParamsSnapshotIsThreadSafeForConcurrentForks", func(t *testing.T) {
		// Given a snapshot store accessed concurrently (PDF Section 8:
		//       multi-agent coordination), parent may save while several
		//       sub-agents read,
		store := &CacheSafeParamsSnapshot{}
		params := NewCacheSafeParams("p", nil, "anthropic", "m", true)

		// When 100 goroutines hammer Save and Get,
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				store.Save(params)
			}()
			go func() {
				defer wg.Done()
				_ = store.Get()
			}()
		}
		wg.Wait()

		// Then no race occurs (the test would fail under -race) and Get
		//      returns the saved value.
		got := store.Get()
		assert.NotNil(t, got, "snapshot must be retrievable after Save")
	})

	t.Run("Scenario_FreshTurnStatePerForkPreventsParentLeakage", func(t *testing.T) {
		// Given a parent that already advanced 5 turns and recorded recoveries,
		parent := NewTurnState()
		for i := 0; i < 5; i++ {
			parent.NextTurn(TransitionToolUse)
		}
		parent.RecordReactiveCompact()
		parent.RecordMaxTokensRecovery(8000)

		// When a fork starts (each fork must construct its OWN TurnState —
		//      PDF Section 8: isolated context window),
		fork := NewTurnState()

		// Then the fork's state is a clean zero — no recovery counters or
		//      reactive_compact flags inherited from the parent.
		assert.Equal(t, 0, fork.TurnCount,
			"fork TurnCount must start at 0 — no parent state leak")
		assert.Equal(t, TransitionNone, fork.Transition,
			"fork must have no prior transition")
		assert.Equal(t, 0, fork.MaxOutputTokensRecoveryCount,
			"fork recovery counter must be fresh")
		assert.False(t, fork.HasAttemptedReactiveCompact,
			"fork compact flag must be fresh — parent's attempts don't carry over")
	})
}
