package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD: CONTEXT-001 ContextWindowMonitor

func TestBDD_CONTEXT001_FrontmatterPctCapsTotalWindow(t *testing.T) {
	// GIVEN an agent with context_window_pct = 80 in frontmatter
	// WHEN the monitor is created for that session
	// THEN the effective limit is 80% of the full effective context window
	m80, err := NewContextWindowMonitor("s", "claude-sonnet-4-6", 80)
	require.NoError(t, err)
	m100, _ := NewContextWindowMonitor("s", "claude-sonnet-4-6", 100)

	_, _, limit80 := m80.Snapshot()
	_, _, limit100 := m100.Snapshot()

	assert.Less(t, limit80, limit100)
	// within 1% tolerance
	assert.InDelta(t, float64(limit100)*0.8, float64(limit80), float64(limit100)*0.01)
}

func TestBDD_CONTEXT001_NoAlertBelowWarningThreshold(t *testing.T) {
	// GIVEN a fresh session with 10k tokens used
	// WHEN turn is recorded
	// THEN no alert is emitted (threshold is much higher)
	m, _ := NewContextWindowMonitor("sess-fresh", "claude-opus-4-7", 100)
	alert, err := m.RecordTurn(10_000)
	require.NoError(t, err)
	assert.Nil(t, alert, "10k tokens << warning threshold; no alert expected")
}

func TestBDD_CONTEXT001_AlertLevelEscalatesWithUsage(t *testing.T) {
	// GIVEN a model with a large context window
	// WHEN successive turns consume more and more tokens
	// THEN alert level escalates: warning → critical → blocking
	m, _ := NewContextWindowMonitor("sess-escalate", "claude-sonnet-4-6", 100)
	_, _, limit := m.Snapshot()

	// warning zone: limit - 40000 + small delta (above warning but below critical)
	a1, _ := m.RecordTurn(limit - CWMonitorWarningBufferTokens + 500)
	require.NotNil(t, a1)
	assert.Equal(t, ContextWindowAlertWarning, a1.Level)

	// blocking zone: limit - 3000 + 1
	a2, _ := m.RecordTurn(limit - ManualCompactBufferTokens + 1)
	require.NotNil(t, a2)
	assert.Equal(t, ContextWindowAlertBlocking, a2.Level)
}

func TestBDD_CONTEXT001_AlertContainsSessionAndModel(t *testing.T) {
	// GIVEN a specific session and model
	// WHEN a blocking alert fires
	// THEN the alert payload includes the session ID and model ID for routing
	m, _ := NewContextWindowMonitor("session-xyz", "claude-haiku-4-5-20251001", 100)
	_, _, limit := m.Snapshot()
	alert, _ := m.RecordTurn(limit - ManualCompactBufferTokens + 1)
	require.NotNil(t, alert)
	assert.Equal(t, "session-xyz", alert.SessionID)
	assert.Equal(t, "claude-haiku-4-5-20251001", alert.ModelID)
	assert.False(t, alert.EmittedAt.IsZero())
}

func TestBDD_CONTEXT001_TurnCounterTracksConversationDepth(t *testing.T) {
	// GIVEN a multi-turn conversation
	// WHEN each turn is recorded
	// THEN the turn counter increments so the alert includes turn depth
	m, _ := NewContextWindowMonitor("s", "claude-sonnet-4-6", 100)
	_, _, limit := m.Snapshot()

	_, err := m.RecordTurn(100)
	require.NoError(t, err)
	_, err = m.RecordTurn(200)
	require.NoError(t, err)
	alert, _ := m.RecordTurn(limit - CWMonitorWarningBufferTokens + 1)

	require.NotNil(t, alert)
	assert.Equal(t, 3, alert.TurnNumber)
}

func TestBDD_CONTEXT001_UnknownModelRejectedAtConstruction(t *testing.T) {
	// GIVEN an unknown model ID (e.g., from a user-configured preset)
	// WHEN the monitor is constructed
	// THEN it returns an error rather than silently using a zero limit
	_, err := NewContextWindowMonitor("s", "open-ai-gpt-4o", 100)
	assert.ErrorIs(t, err, ErrCWMonitorModelUnknown)
}
