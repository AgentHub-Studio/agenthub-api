package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// BDD scenarios for KairosHeartbeat — §11.6 proactive background agent loop
// (arXiv:2604.14228v1): tick-based heartbeat with act/sleep decisions,
// economic throttling via SleepTool, terminal focus awareness.

func TestBDD_KairosHeartbeat(t *testing.T) {
	base := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	away := agentic.PresenceSignal{} // zero = user away

	t.Run("Scenario_AgentActsWhenUserIsAway", func(t *testing.T) {
		// Given a KAIROS heartbeat scheduler and a user who has been absent > 30s
		h, err := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
		require.NoError(t, err)

		// When the tick fires with no recent user message
		decision := h.NextDecision(base, away)

		// Then the agent acts proactively
		assert.Equal(t, agentic.KairosAct, decision.Decision)
		assert.Equal(t, 1, decision.TickCount)
	})

	t.Run("Scenario_AgentSleepsWhenUserIsPresent", func(t *testing.T) {
		// Given a user who sent a message 10 seconds ago (within 30s window)
		h, _ := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
		present := agentic.PresenceSignal{LastMessageAt: base.Add(-10 * time.Second)}

		// When the tick fires with a recent user message
		decision := h.NextDecision(base, present)

		// Then the agent sleeps to preserve collaboration mode (terminal focus awareness)
		assert.Equal(t, agentic.KairosSleep, decision.Decision)
		assert.Contains(t, decision.Reason, "present")
	})

	t.Run("Scenario_ForcedSleepAfterConsecutiveActLimit", func(t *testing.T) {
		// Given a configured limit of 3 consecutive acts before mandatory sleep
		cfg := agentic.DefaultKairosConfig()
		cfg.MaxActTicksBeforeForcedSleep = 3
		cfg.SleepDuration = 10 * time.Minute
		h, _ := agentic.NewKairosHeartbeat(cfg)

		// When 3 act ticks fire (user always away)
		for i := 0; i < 3; i++ {
			d := h.NextDecision(base, away)
			assert.Equal(t, agentic.KairosAct, d.Decision)
		}

		// Then the 4th tick triggers a forced sleep (prevents runaway loops)
		d := h.NextDecision(base, away)
		assert.Equal(t, agentic.KairosSleep, d.Decision)
		assert.Contains(t, d.Reason, "consecutive")
		assert.False(t, d.SleepUntil.IsZero(), "sleep window must be set")
	})

	t.Run("Scenario_EconomicThrottleStopsBudgetOverrun", func(t *testing.T) {
		// Given an economic budget that covers exactly 1 act tick at $0.005 each
		// (budget = $0.004: first tick cost=0 → act; second tick cost=0.005 >= 0.004 → sleep)
		cfg := agentic.DefaultKairosConfig()
		cfg.EstimatedAPICallCostUSD = 0.005
		cfg.EconomicBudgetUSD = 0.004
		cfg.MaxActTicksBeforeForcedSleep = 0 // disable consecutive limit
		h, _ := agentic.NewKairosHeartbeat(cfg)

		// When the first tick fires
		d1 := h.NextDecision(base, away)
		assert.Equal(t, agentic.KairosAct, d1.Decision)

		// Then the second tick is blocked by the economic budget (SleepTool analog)
		d2 := h.NextDecision(base, away)
		assert.Equal(t, agentic.KairosSleep, d2.Decision)
		assert.Contains(t, d2.Reason, "budget")
	})

	t.Run("Scenario_SleepWindowBlocksAllTicksDuringCooldown", func(t *testing.T) {
		// Given a forced sleep window of 5 minutes starting at base
		cfg := agentic.DefaultKairosConfig()
		cfg.MaxActTicksBeforeForcedSleep = 1
		cfg.SleepDuration = 5 * time.Minute
		h, _ := agentic.NewKairosHeartbeat(cfg)

		h.NextDecision(base, away)            // act (triggers forced sleep next)
		forced := h.NextDecision(base, away)  // forced sleep
		require.Equal(t, agentic.KairosSleep, forced.Decision)

		// When ticks fire during the cooldown window
		midCooldown := h.NextDecision(base.Add(3*time.Minute), away)

		// Then the agent still sleeps (cooling down)
		assert.Equal(t, agentic.KairosSleep, midCooldown.Decision)
		assert.Contains(t, midCooldown.Reason, "cooling")
	})

	t.Run("Scenario_AgentResumesActingAfterSleepWindowExpires", func(t *testing.T) {
		// Given a completed forced sleep window
		cfg := agentic.DefaultKairosConfig()
		cfg.MaxActTicksBeforeForcedSleep = 1
		cfg.SleepDuration = 5 * time.Minute
		h, _ := agentic.NewKairosHeartbeat(cfg)

		h.NextDecision(base, away)
		h.NextDecision(base, away) // forced sleep until base+5m

		// When a tick fires after the sleep window expires
		afterSleep := h.NextDecision(base.Add(6*time.Minute), away)

		// Then the agent acts again (resuming proactive mode)
		assert.Equal(t, agentic.KairosAct, afterSleep.Decision)
	})
}
