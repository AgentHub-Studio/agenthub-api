package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelContextRegistry_AllModelsPresent(t *testing.T) {
	required := []string{
		"claude-haiku-4-5-20251001",
		"claude-sonnet-4-6",
		"claude-opus-4-7",
	}
	for _, id := range required {
		spec, ok := LookupModel(id)
		assert.True(t, ok, "model %q must be in registry", id)
		assert.Equal(t, id, spec.ModelID)
		assert.Greater(t, spec.ContextWindow, 0)
		assert.Greater(t, spec.MaxOutputTokens, 0)
	}
}

func TestModelContextRegistry_AllSupportedModelIDs(t *testing.T) {
	ids := AllSupportedModelIDs()
	assert.GreaterOrEqual(t, len(ids), 3)
}

func TestLookupModel_UnknownReturnsFalse(t *testing.T) {
	_, ok := LookupModel("gpt-4-turbo")
	assert.False(t, ok)
}

func TestContextWindowAlertLevel_IsValid(t *testing.T) {
	assert.True(t, ContextWindowAlertWarning.IsValid())
	assert.True(t, ContextWindowAlertCritical.IsValid())
	assert.True(t, ContextWindowAlertBlocking.IsValid())
	assert.False(t, ContextWindowAlertLevel("info").IsValid())
}

func TestContextWindowAlertLevel_AllCount(t *testing.T) {
	assert.Equal(t, 3, len(AllContextWindowAlertLevels))
}

func TestNewContextWindowMonitor_Happy(t *testing.T) {
	m, err := NewContextWindowMonitor("sess-1", "claude-sonnet-4-6", 80)
	require.NoError(t, err)
	_, _, limit := m.Snapshot()
	assert.Greater(t, limit, 0)
}

func TestNewContextWindowMonitor_UnknownModel(t *testing.T) {
	_, err := NewContextWindowMonitor("sess-1", "unknown-model", 80)
	assert.ErrorIs(t, err, ErrCWMonitorModelUnknown)
}

func TestNewContextWindowMonitor_PctZero(t *testing.T) {
	_, err := NewContextWindowMonitor("sess-1", "claude-sonnet-4-6", 0)
	assert.ErrorIs(t, err, ErrCWMonitorLimitPctOutOfRange)
}

func TestNewContextWindowMonitor_PctOver100(t *testing.T) {
	_, err := NewContextWindowMonitor("sess-1", "claude-sonnet-4-6", 101)
	assert.ErrorIs(t, err, ErrCWMonitorLimitPctOutOfRange)
}

func TestContextWindowMonitor_RecordTurn_NegativeTokens(t *testing.T) {
	m, _ := NewContextWindowMonitor("sess-1", "claude-sonnet-4-6", 100)
	_, err := m.RecordTurn(-1)
	assert.ErrorIs(t, err, ErrCWMonitorNegativeTokens)
}

func TestContextWindowMonitor_RecordTurn_NoAlertUnderThreshold(t *testing.T) {
	m, err := NewContextWindowMonitor("sess-1", "claude-sonnet-4-6", 100)
	require.NoError(t, err)
	alert, err := m.RecordTurn(1000)
	require.NoError(t, err)
	assert.Nil(t, alert, "no alert expected well below threshold")
}

func TestContextWindowMonitor_RecordTurn_EmitsWarning(t *testing.T) {
	// sonnet: contextWindow=200k, maxOutput=8192 → effective = 191808
	// limitPct=100 → effectiveLimit=191808
	// warning threshold = effectiveLimit - 20000 = 171808
	m, err := NewContextWindowMonitor("sess-1", "claude-sonnet-4-6", 100)
	require.NoError(t, err)

	_, _, limit := m.Snapshot()
	// warning zone: limit - 40000 + delta (above warning but below critical at 20000)
	warningTokens := limit - CWMonitorWarningBufferTokens + 100

	alert, err := m.RecordTurn(warningTokens)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.Equal(t, ContextWindowAlertWarning, alert.Level)
	assert.Equal(t, "claude-sonnet-4-6", alert.ModelID)
	assert.Equal(t, "sess-1", alert.SessionID)
	assert.Equal(t, 1, alert.TurnNumber)
}

func TestContextWindowMonitor_RecordTurn_EmitsBlocking(t *testing.T) {
	m, err := NewContextWindowMonitor("sess-1", "claude-haiku-4-5-20251001", 100)
	require.NoError(t, err)

	_, _, limit := m.Snapshot()
	blockingTokens := limit - ManualCompactBufferTokens + 1

	alert, err := m.RecordTurn(blockingTokens)
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.Equal(t, ContextWindowAlertBlocking, alert.Level)
}

func TestContextWindowMonitor_LimitPct_ReducesEffectiveWindow(t *testing.T) {
	m100, _ := NewContextWindowMonitor("s", "claude-sonnet-4-6", 100)
	m80, _ := NewContextWindowMonitor("s", "claude-sonnet-4-6", 80)

	_, _, limit100 := m100.Snapshot()
	_, _, limit80 := m80.Snapshot()

	assert.Greater(t, limit100, limit80)
	assert.InDelta(t, float64(limit100)*0.8, float64(limit80), 1)
}

func TestContextWindowMonitor_TurnNumberIncrements(t *testing.T) {
	m, _ := NewContextWindowMonitor("s", "claude-opus-4-7", 100)
	m.RecordTurn(0)
	m.RecordTurn(0)
	m.RecordTurn(0)
	turn, _, _ := m.Snapshot()
	assert.Equal(t, 3, turn)
}

func TestContextWindowMonitor_LastAlert_NilUntilThreshold(t *testing.T) {
	m, _ := NewContextWindowMonitor("s", "claude-sonnet-4-6", 100)
	m.RecordTurn(1000)
	assert.Nil(t, m.LastAlert())
}

func TestContextWindowMonitor_LastAlert_SetAfterAlert(t *testing.T) {
	m, _ := NewContextWindowMonitor("s", "claude-sonnet-4-6", 100)
	_, _, limit := m.Snapshot()
	m.RecordTurn(limit - ManualCompactBufferTokens + 1)
	assert.NotNil(t, m.LastAlert())
}
