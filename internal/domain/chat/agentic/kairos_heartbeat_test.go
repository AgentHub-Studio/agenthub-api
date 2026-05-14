package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

var testNow = time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

// --- DefaultKairosConfig ---

func TestDefaultKairosConfig_Values(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	assert.Equal(t, 5*time.Minute, cfg.TickInterval)
	assert.Equal(t, 30*time.Second, cfg.PresenceWindow)
	assert.Equal(t, 12, cfg.MaxActTicksBeforeForcedSleep)
	assert.Equal(t, 15*time.Minute, cfg.SleepDuration)
	assert.InDelta(t, 0.001, cfg.EstimatedAPICallCostUSD, 1e-6)
	assert.Equal(t, float64(0), cfg.EconomicBudgetUSD)
}

// --- KairosConfig.Validate ---

func TestKairosConfig_Validate_Valid(t *testing.T) {
	assert.NoError(t, agentic.DefaultKairosConfig().Validate())
}

func TestKairosConfig_Validate_ZeroTickInterval(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.TickInterval = 0
	assert.Error(t, cfg.Validate())
}

func TestKairosConfig_Validate_NegativePresenceWindow(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.PresenceWindow = -1
	assert.Error(t, cfg.Validate())
}

func TestKairosConfig_Validate_NegativeMaxActTicks(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.MaxActTicksBeforeForcedSleep = -1
	assert.Error(t, cfg.Validate())
}

func TestKairosConfig_Validate_NegativeCost(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.EstimatedAPICallCostUSD = -0.001
	assert.Error(t, cfg.Validate())
}

func TestKairosConfig_Validate_NegativeBudget(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.EconomicBudgetUSD = -1
	assert.Error(t, cfg.Validate())
}

// --- NewKairosHeartbeat ---

func TestNewKairosHeartbeat_Valid(t *testing.T) {
	h, err := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
	require.NoError(t, err)
	assert.NotNil(t, h)
	assert.Equal(t, 0, h.TickCount())
	assert.Equal(t, float64(0), h.CumulativeCostUSD())
}

func TestNewKairosHeartbeat_InvalidConfig(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.TickInterval = 0
	_, err := agentic.NewKairosHeartbeat(cfg)
	assert.Error(t, err)
}

func TestNewKairosHeartbeat_ZeroSleepDuration_DefaultsToThreeTickIntervals(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.SleepDuration = 0
	h, err := agentic.NewKairosHeartbeat(cfg)
	require.NoError(t, err)
	assert.NotNil(t, h)
}

// --- PresenceSignal ---

func TestPresenceSignal_IsPresent_UserRecent(t *testing.T) {
	signal := agentic.PresenceSignal{LastMessageAt: testNow.Add(-10 * time.Second)}
	assert.True(t, signal.IsPresent(testNow, 30*time.Second))
}

func TestPresenceSignal_IsPresent_UserOld(t *testing.T) {
	signal := agentic.PresenceSignal{LastMessageAt: testNow.Add(-5 * time.Minute)}
	assert.False(t, signal.IsPresent(testNow, 30*time.Second))
}

func TestPresenceSignal_IsPresent_ZeroTime(t *testing.T) {
	signal := agentic.PresenceSignal{}
	assert.False(t, signal.IsPresent(testNow, 30*time.Second))
}

// --- NextDecision: user present ---

func TestKairosHeartbeat_UserPresent_ReturnsSleep(t *testing.T) {
	h, _ := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
	signal := agentic.PresenceSignal{LastMessageAt: testNow.Add(-5 * time.Second)}
	d := h.NextDecision(testNow, signal)
	assert.Equal(t, agentic.KairosSleep, d.Decision)
	assert.Contains(t, d.Reason, "present")
	assert.Equal(t, 1, d.TickCount)
}

// --- NextDecision: user away → act ---

func TestKairosHeartbeat_UserAway_ReturnsAct(t *testing.T) {
	h, _ := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
	signal := agentic.PresenceSignal{} // zero = away
	d := h.NextDecision(testNow, signal)
	assert.Equal(t, agentic.KairosAct, d.Decision)
	assert.Contains(t, d.Reason, "away")
	assert.Equal(t, 1, d.TickCount)
}

// --- NextDecision: consecutive act limit ---

func TestKairosHeartbeat_MaxActTicks_ForcesSleep(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.MaxActTicksBeforeForcedSleep = 3
	cfg.SleepDuration = 10 * time.Minute
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{} // user away throughout

	// First 3 ticks should act
	for i := 0; i < 3; i++ {
		d := h.NextDecision(testNow, signal)
		assert.Equal(t, agentic.KairosAct, d.Decision, "tick %d should act", i+1)
	}
	// 4th tick triggers forced sleep
	d := h.NextDecision(testNow, signal)
	assert.Equal(t, agentic.KairosSleep, d.Decision)
	assert.Contains(t, d.Reason, "consecutive")
}

// --- NextDecision: economic budget ---

func TestKairosHeartbeat_EconomicBudget_ForcesSleepWhenExhausted(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.EstimatedAPICallCostUSD = 0.005
	// Budget 0.009: allows 2 acts (cumulative 0.010 after tick 2),
	// tick 3 sees cumulative=0.010 >= 0.009 → sleep.
	cfg.EconomicBudgetUSD = 0.009
	cfg.MaxActTicksBeforeForcedSleep = 0 // disabled
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}

	d1 := h.NextDecision(testNow, signal)
	assert.Equal(t, agentic.KairosAct, d1.Decision)

	d2 := h.NextDecision(testNow, signal)
	assert.Equal(t, agentic.KairosAct, d2.Decision)
	assert.InDelta(t, 0.010, d2.EstimatedCostUSD, 1e-6)

	d3 := h.NextDecision(testNow, signal)
	assert.Equal(t, agentic.KairosSleep, d3.Decision)
	assert.Contains(t, d3.Reason, "budget")
}

// --- NextDecision: forced sleep window ---

func TestKairosHeartbeat_SleepWindow_BlocksActDuringCooldown(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.MaxActTicksBeforeForcedSleep = 1
	cfg.SleepDuration = 10 * time.Minute
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}

	// Act once → triggers forced sleep on next tick
	h.NextDecision(testNow, signal)
	d := h.NextDecision(testNow, signal)
	require.Equal(t, agentic.KairosSleep, d.Decision)
	require.False(t, d.SleepUntil.IsZero())

	// During the sleep window, all ticks should still sleep
	d2 := h.NextDecision(testNow.Add(5*time.Minute), signal)
	assert.Equal(t, agentic.KairosSleep, d2.Decision)
	assert.Contains(t, d2.Reason, "cooling")
}

// --- NextDecision: sleep window expires ---

func TestKairosHeartbeat_SleepWindowExpires_ResumesAct(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.MaxActTicksBeforeForcedSleep = 1
	cfg.SleepDuration = 5 * time.Minute
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}

	h.NextDecision(testNow, signal)
	h.NextDecision(testNow, signal) // triggers sleep until testNow+5m

	// After sleep window expires, should act again
	d := h.NextDecision(testNow.Add(6*time.Minute), signal)
	assert.Equal(t, agentic.KairosAct, d.Decision)
}

// --- IsSleeping ---

func TestKairosHeartbeat_IsSleeping_FalseInitially(t *testing.T) {
	h, _ := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
	assert.False(t, h.IsSleeping(testNow))
}

func TestKairosHeartbeat_IsSleeping_TrueAfterForcedSleep(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.MaxActTicksBeforeForcedSleep = 1
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}
	h.NextDecision(testNow, signal) // act
	h.NextDecision(testNow, signal) // force sleep
	assert.True(t, h.IsSleeping(testNow.Add(1*time.Minute)))
}

// --- CumulativeCost ---

func TestKairosHeartbeat_CumulativeCost_AccumulatesOnAct(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.EstimatedAPICallCostUSD = 0.01
	cfg.MaxActTicksBeforeForcedSleep = 0 // disabled
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}

	h.NextDecision(testNow, signal)
	h.NextDecision(testNow, signal)
	assert.InDelta(t, 0.02, h.CumulativeCostUSD(), 1e-6)
}

func TestKairosHeartbeat_CumulativeCost_NoAccumulateOnSleep(t *testing.T) {
	h, _ := agentic.NewKairosHeartbeat(agentic.DefaultKairosConfig())
	signal := agentic.PresenceSignal{LastMessageAt: testNow.Add(-1 * time.Second)}
	h.NextDecision(testNow, signal) // sleep
	assert.Equal(t, float64(0), h.CumulativeCostUSD())
}

// --- Reset ---

func TestKairosHeartbeat_Reset_ClearsState(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.EstimatedAPICallCostUSD = 0.01
	cfg.MaxActTicksBeforeForcedSleep = 0
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}
	h.NextDecision(testNow, signal)
	h.NextDecision(testNow, signal)
	assert.Equal(t, 2, h.TickCount())

	h.Reset()
	assert.Equal(t, 0, h.TickCount())
	assert.Equal(t, float64(0), h.CumulativeCostUSD())
	assert.False(t, h.IsSleeping(testNow))
}

// --- MaxActTicks = 0 means unlimited ---

func TestKairosHeartbeat_MaxActTicks_Zero_Unlimited(t *testing.T) {
	cfg := agentic.DefaultKairosConfig()
	cfg.MaxActTicksBeforeForcedSleep = 0
	cfg.EconomicBudgetUSD = 0
	h, _ := agentic.NewKairosHeartbeat(cfg)
	signal := agentic.PresenceSignal{}

	for i := 0; i < 50; i++ {
		d := h.NextDecision(testNow, signal)
		assert.Equal(t, agentic.KairosAct, d.Decision, "tick %d", i)
	}
}
