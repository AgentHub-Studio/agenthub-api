package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ToolResultBudget(t *testing.T) {
	t.Run("Scenario_HugeSQLResultDoesNotConsumeEntireContextWindow", func(t *testing.T) {
		// Given a tool returns a 100k-row SQL result (≈40k tokens),
		// And the budget caps per-call at 4k tokens,
		// When the enforcer runs,
		// Then the result is truncated (or summarized) to fit within
		// budget — the agent never sees a single tool result that
		// dominates its context window (PDF §7.6).
		cfg := DefaultToolResultBudgetConfig()
		e := NewToolResultBudgetEnforcer(cfg)
		hugeBody := strings.Repeat("row data...\n", 14000) // ~42k tokens
		d, err := e.Enforce("execute_sql", hugeBody)
		require.NoError(t, err)
		assert.NotEqual(t, ToolResultBudgetKeep, d.Action)
		assert.LessOrEqual(t, d.FinalTokens, cfg.ForceSummarizeOverTokens+200)
	})

	t.Run("Scenario_MultipleSmallToolCallsAccumulateAndEventuallyTriggerDrop", func(t *testing.T) {
		// Given a turn invokes 5 small tools each well under per-call cap,
		// When the cumulative crosses PerTurnMaxTokens,
		// Then the next call is DROPPED (one-line note in place of result)
		// — single-call cap doesn't catch death-by-a-thousand-cuts.
		cfg := ToolResultBudgetConfig{
			PerCallMaxTokens: 200,
			PerTurnMaxTokens: 250,
			PerRunMaxTokens:  100000,
		}
		e := NewToolResultBudgetEnforcer(cfg)
		_, _ = e.Enforce("a", strings.Repeat("x", 400)) // 100 tokens
		_, _ = e.Enforce("b", strings.Repeat("x", 400)) // cumulative 200
		d, _ := e.Enforce("c", strings.Repeat("x", 400)) // would push to 300
		assert.Equal(t, ToolResultBudgetDrop, d.Action)
	})

	t.Run("Scenario_PerRunBudgetSurvivesTurnBoundary", func(t *testing.T) {
		// Given an agent runs many turns over a long session,
		// When EndTurn fires (turn boundary),
		// Then per-turn cumulative resets but per-run KEEPS COUNTING —
		// total context spent across the entire run stays bounded.
		e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
		_, _ = e.Enforce("a", strings.Repeat("x", 400))
		assert.Equal(t, 100, e.PerTurnUsed())
		assert.Equal(t, 100, e.PerRunUsed())
		e.EndTurn()
		assert.Equal(t, 0, e.PerTurnUsed(), "turn reset")
		assert.Equal(t, 100, e.PerRunUsed(), "run keeps counting")
	})

	t.Run("Scenario_ForceSummarizeReplacesHugeResultWithPlaceholder", func(t *testing.T) {
		// Given results above ForceSummarizeOverTokens are too big to
		// usefully truncate (would lose 90% of content),
		// When the enforcer encounters one,
		// Then it returns a Summarize placeholder mentioning the tool
		// name + original size, so the agent can reason about WHAT was
		// dropped (vs blunt truncation that loses context entirely).
		cfg := ToolResultBudgetConfig{
			ForceSummarizeOverTokens: 100,
			PerCallMaxTokens:         100000,
		}
		e := NewToolResultBudgetEnforcer(cfg)
		d, _ := e.Enforce("execute_sql", strings.Repeat("x", 1000))
		assert.Equal(t, ToolResultBudgetSummarize, d.Action)
		assert.Contains(t, d.FinalBody, "execute_sql")
		assert.Contains(t, d.FinalBody, "summarized")
	})

	t.Run("Scenario_DropNoteIncludesAuditableReason", func(t *testing.T) {
		// Given GOV-001 audits every tool result that reached the agent,
		// When budget enforcement drops a result,
		// Then the drop note carries the actual budget number that was
		// exceeded — auditor can reconstruct WHY the agent was deprived
		// of that context.
		cfg := ToolResultBudgetConfig{PerRunMaxTokens: 50}
		e := NewToolResultBudgetEnforcer(cfg)
		d, _ := e.Enforce("q", strings.Repeat("x", 1000))
		assert.Equal(t, ToolResultBudgetDrop, d.Action)
		assert.Contains(t, d.Reason, "PerRunMaxTokens")
		assert.Contains(t, d.FinalBody, "PerRunMaxTokens")
	})

	t.Run("Scenario_DecisionTracksOriginalVsFinalForCostAnalytics", func(t *testing.T) {
		// Given cost analytics needs to know how much we SAVED by enforcing,
		// When a budget decision is recorded,
		// Then OriginalTokens vs FinalTokens diff is the savings — admin
		// dashboards can show "tool result budget saved 30k tokens this hour".
		cfg := ToolResultBudgetConfig{PerCallMaxTokens: 100}
		e := NewToolResultBudgetEnforcer(cfg)
		d, _ := e.Enforce("q", strings.Repeat("x", 4000))
		assert.Greater(t, d.OriginalTokens, d.FinalTokens)
		savings := d.OriginalTokens - d.FinalTokens
		assert.Greater(t, savings, 800)
	})

	t.Run("Scenario_KeepActionPreservesVerbatimBodyForCacheStability", func(t *testing.T) {
		// Given prompt cache stability requires byte-identical replay
		// (toolresultstorage.go ContentReplacementState pattern),
		// When the result fits all budgets,
		// Then enforcer keeps body verbatim (no transformation) so
		// cache hits work across turns.
		e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
		body := "small result that fits"
		d, _ := e.Enforce("q", body)
		assert.Equal(t, ToolResultBudgetKeep, d.Action)
		assert.Equal(t, body, d.FinalBody, "verbatim for cache stability")
	})

	t.Run("Scenario_AdminCanInspectActiveConfigForAudit", func(t *testing.T) {
		// Given audit + ops dashboards display the active enforcer config,
		// When admin queries the enforcer,
		// Then ConfigSummary returns a one-line string with all 4 budget
		// numbers — copy-pasteable into incident reports.
		e := NewToolResultBudgetEnforcer(DefaultToolResultBudgetConfig())
		summary := e.ConfigSummary()
		assert.Contains(t, summary, "per_call=4000")
		assert.Contains(t, summary, "per_turn=12000")
		assert.Contains(t, summary, "per_run=50000")
	})

	t.Run("Scenario_ZeroBudgetEffectivelyDisablesEnforcement", func(t *testing.T) {
		// Given dev-mode tenants want full visibility (no truncation),
		// When admin sets PerCallMaxTokens=0,
		// Then any size result is kept verbatim — useful for debugging
		// or for tenants with no token-cost concerns.
		cfg := ToolResultBudgetConfig{}
		e := NewToolResultBudgetEnforcer(cfg)
		huge := strings.Repeat("x", 100000)
		d, _ := e.Enforce("q", huge)
		assert.Equal(t, ToolResultBudgetKeep, d.Action)
	})

	t.Run("Scenario_FourActionsAreClosedSetForRunnerDispatch", func(t *testing.T) {
		// Given the runtime dispatches each ToolResultBudgetAction differently,
		// When the enforcer emits an action,
		// Then it's exactly one of: keep / truncate / summarize / drop
		// (no surprise actions runtime can't handle).
		expected := []ToolResultBudgetAction{
			ToolResultBudgetKeep, ToolResultBudgetTruncate,
			ToolResultBudgetSummarize, ToolResultBudgetDrop,
		}
		assert.Equal(t, expected, AllToolResultBudgetActions())
	})
}
