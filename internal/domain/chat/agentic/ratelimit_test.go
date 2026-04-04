package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestRateLimitState_String(t *testing.T) {
	assert.Equal(t, "ok", agentic.RateLimitOK.String())
	assert.Equal(t, "warning", agentic.RateLimitWarning.String())
	assert.Equal(t, "exceeded", agentic.RateLimitExceeded.String())
}

func TestRateLimitThresholds(t *testing.T) {
	assert.Equal(t, 0.8, agentic.RateLimitWarningThreshold)
	assert.Equal(t, 1.0, agentic.RateLimitExceededThreshold)
}

func TestRateLimitWindow_Values(t *testing.T) {
	assert.Equal(t, agentic.RateLimitWindow("five_hour"), agentic.WindowFiveHour)
	assert.Equal(t, agentic.RateLimitWindow("seven_day"), agentic.WindowSevenDay)
}

// --- NewRateLimitManager ---

func TestNewRateLimitManager(t *testing.T) {
	m := agentic.NewRateLimitManager()
	assert.Equal(t, agentic.RateLimitOK, m.GetState())
	assert.Nil(t, m.GetUtilization())
	assert.False(t, m.IsExceeded())
}

// --- Update ---

func TestRateLimitManager_Update(t *testing.T) {
	m := agentic.NewRateLimitManager()
	u := floatPtr(0.5)
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: u},
		},
	})

	util := m.GetUtilization()
	require.NotNil(t, util)
	assert.NotNil(t, util.Windows[agentic.WindowFiveHour].Utilization)
	assert.Equal(t, 1, m.HistoryLen())
}

// --- UpdateWindow ---

func TestRateLimitManager_UpdateWindow(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.UpdateWindow(agentic.WindowFiveHour, agentic.RateLimit{Utilization: floatPtr(0.3)})

	assert.Equal(t, agentic.RateLimitOK, m.GetWindowState(agentic.WindowFiveHour))
}

func TestRateLimitManager_UpdateWindow_CreatesUtilization(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.UpdateWindow(agentic.WindowSevenDay, agentic.RateLimit{Utilization: floatPtr(0.9)})

	util := m.GetUtilization()
	require.NotNil(t, util)
	assert.Equal(t, agentic.RateLimitWarning, m.GetWindowState(agentic.WindowSevenDay))
}

// --- GetState ---

func TestRateLimitManager_GetState_OK(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.5)},
		},
	})
	assert.Equal(t, agentic.RateLimitOK, m.GetState())
}

func TestRateLimitManager_GetState_Warning(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.85)},
		},
	})
	assert.Equal(t, agentic.RateLimitWarning, m.GetState())
}

func TestRateLimitManager_GetState_Exceeded(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(1.0)},
		},
	})
	assert.Equal(t, agentic.RateLimitExceeded, m.GetState())
}

func TestRateLimitManager_GetState_WorstCase(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.5)}, // OK
			agentic.WindowSevenDay: {Utilization: floatPtr(1.2)}, // Exceeded
		},
	})
	assert.Equal(t, agentic.RateLimitExceeded, m.GetState())
}

func TestRateLimitManager_GetState_NilUtilization(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {}, // nil utilization
		},
	})
	assert.Equal(t, agentic.RateLimitOK, m.GetState())
}

// --- GetWindowState ---

func TestRateLimitManager_GetWindowState_Unknown(t *testing.T) {
	m := agentic.NewRateLimitManager()
	assert.Equal(t, agentic.RateLimitOK, m.GetWindowState(agentic.WindowFiveHour))
}

// --- IsExceeded ---

func TestRateLimitManager_IsExceeded(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(1.5)},
		},
	})
	assert.True(t, m.IsExceeded())
}

// --- TimeUntilReset ---

func TestRateLimitManager_TimeUntilReset(t *testing.T) {
	m := agentic.NewRateLimitManager()
	reset := time.Now().Add(30 * time.Minute)
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(1.0), ResetsAt: &reset},
		},
	})

	d := m.TimeUntilReset()
	assert.Greater(t, d, 25*time.Minute)
	assert.Less(t, d, 31*time.Minute)
}

func TestRateLimitManager_TimeUntilReset_NoExceeded(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.5)},
		},
	})
	assert.Equal(t, time.Duration(0), m.TimeUntilReset())
}

// --- Summary ---

func TestRateLimitManager_Summary_NoData(t *testing.T) {
	m := agentic.NewRateLimitManager()
	assert.Contains(t, m.Summary(), "No rate limit data")
}

func TestRateLimitManager_Summary_OK(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.3)},
		},
	})
	assert.Contains(t, m.Summary(), "OK")
}

func TestRateLimitManager_Summary_Warning(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.9)},
		},
	})
	assert.Contains(t, m.Summary(), "Approaching")
}

func TestRateLimitManager_Summary_Exceeded(t *testing.T) {
	m := agentic.NewRateLimitManager()
	reset := time.Now().Add(10 * time.Minute)
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(1.0), ResetsAt: &reset},
		},
	})
	assert.Contains(t, m.Summary(), "exceeded")
}

// --- History ---

func TestRateLimitManager_History(t *testing.T) {
	m := agentic.NewRateLimitManager()
	for i := 0; i < 5; i++ {
		m.Update(agentic.Utilization{
			Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
				agentic.WindowFiveHour: {Utilization: floatPtr(float64(i) * 0.1)},
			},
		})
	}
	assert.Equal(t, 5, m.HistoryLen())
}

// --- Reset ---

func TestRateLimitManager_Reset(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{
			agentic.WindowFiveHour: {Utilization: floatPtr(0.5)},
		},
	})
	m.Reset()
	assert.Nil(t, m.GetUtilization())
	assert.Equal(t, 0, m.HistoryLen())
}

// --- ExtraUsage ---

func TestRateLimitManager_ExtraUsage(t *testing.T) {
	m := agentic.NewRateLimitManager()
	m.Update(agentic.Utilization{
		Windows: map[agentic.RateLimitWindow]agentic.RateLimit{},
		ExtraUsage: &agentic.ExtraUsage{
			Enabled:     true,
			MonthlyLimit: floatPtr(100.0),
			UsedCredits:  floatPtr(42.5),
			Utilization:  floatPtr(0.425),
		},
	})

	util := m.GetUtilization()
	require.NotNil(t, util.ExtraUsage)
	assert.True(t, util.ExtraUsage.Enabled)
	assert.Equal(t, 42.5, *util.ExtraUsage.UsedCredits)
}

func floatPtr(f float64) *float64 {
	return &f
}
