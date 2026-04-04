package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- TipContext ---

func TestTipContext_Values(t *testing.T) {
	assert.Equal(t, agentic.TipContext("spinner"), agentic.TipContextSpinner)
	assert.Equal(t, agentic.TipContext("idle"), agentic.TipContextIdle)
	assert.Equal(t, agentic.TipContext("error"), agentic.TipContextError)
	assert.Equal(t, agentic.TipContext("first_run"), agentic.TipContextFirstRun)
	assert.Equal(t, agentic.TipContext("tool_use"), agentic.TipContextToolUse)
}

// --- NewTipScheduler ---

func TestNewTipScheduler(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "tip-1", Content: "Use /help for commands", Contexts: []agentic.TipContext{agentic.TipContextSpinner}},
	}
	s := agentic.NewTipScheduler(tips)
	assert.Equal(t, 1, s.TotalTips())
	assert.True(t, s.IsEnabled())
}

// --- SetEnabled ---

func TestTipScheduler_SetEnabled(t *testing.T) {
	s := agentic.NewTipScheduler(nil)
	s.SetEnabled(false)
	assert.False(t, s.IsEnabled())
}

// --- GetTip ---

func TestTipScheduler_GetTip_MatchesContext(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "spinner-tip", Content: "Wait for it", Contexts: []agentic.TipContext{agentic.TipContextSpinner}},
		{ID: "error-tip", Content: "Try again", Contexts: []agentic.TipContext{agentic.TipContextError}},
	}
	s := agentic.NewTipScheduler(tips)

	tip := s.GetTip(agentic.TipContextSpinner)
	require.NotNil(t, tip)
	assert.Equal(t, "spinner-tip", tip.ID)
}

func TestTipScheduler_GetTip_NoContextRestriction(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "universal", Content: "General tip", Contexts: nil},
	}
	s := agentic.NewTipScheduler(tips)

	tip := s.GetTip(agentic.TipContextIdle)
	require.NotNil(t, tip)
	assert.Equal(t, "universal", tip.ID)
}

func TestTipScheduler_GetTip_NoMatch(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "spinner-only", Content: "Wait", Contexts: []agentic.TipContext{agentic.TipContextSpinner}},
	}
	s := agentic.NewTipScheduler(tips)

	tip := s.GetTip(agentic.TipContextError)
	assert.Nil(t, tip)
}

func TestTipScheduler_GetTip_Disabled(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "tip-1", Content: "x"},
	}
	s := agentic.NewTipScheduler(tips)
	s.SetEnabled(false)

	assert.Nil(t, s.GetTip(agentic.TipContextSpinner))
}

func TestTipScheduler_GetTip_Empty(t *testing.T) {
	s := agentic.NewTipScheduler(nil)
	assert.Nil(t, s.GetTip(agentic.TipContextIdle))
}

// --- LRU selection ---

func TestTipScheduler_GetTip_LRU_NeverShownFirst(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "shown", Content: "a"},
		{ID: "never", Content: "b"},
	}
	s := agentic.NewTipScheduler(tips)
	s.RecordShown("shown")

	tip := s.GetTip(agentic.TipContextSpinner)
	require.NotNil(t, tip)
	assert.Equal(t, "never", tip.ID, "never-shown tip should be selected first")
}

func TestTipScheduler_GetTip_LRU_OldestShownFirst(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "recent", Content: "a"},
		{ID: "old", Content: "b"},
	}
	s := agentic.NewTipScheduler(tips)

	s.RecordShown("old")
	time.Sleep(5 * time.Millisecond)
	s.RecordShown("recent")

	tip := s.GetTip(agentic.TipContextSpinner)
	require.NotNil(t, tip)
	assert.Equal(t, "old", tip.ID, "oldest-shown tip should be selected")
}

// --- Cooldown ---

func TestTipScheduler_GetTip_Cooldown(t *testing.T) {
	tips := []agentic.Tip{
		{ID: "cooldown-tip", Content: "wait", CooldownSessions: 3},
		{ID: "no-cooldown", Content: "always"},
	}
	s := agentic.NewTipScheduler(tips)

	// Show the cooldown tip in session 0.
	s.RecordShown("cooldown-tip")

	// Session 1 — still in cooldown.
	s.AdvanceSession()
	tip := s.GetTip(agentic.TipContextSpinner)
	require.NotNil(t, tip)
	assert.Equal(t, "no-cooldown", tip.ID, "cooldown tip should be skipped")

	// Advance past cooldown (session 3+).
	s.AdvanceSession()
	s.AdvanceSession()

	// Record no-cooldown as shown so the LRU prefers cooldown-tip.
	s.RecordShown("no-cooldown")

	tip = s.GetTip(agentic.TipContextSpinner)
	require.NotNil(t, tip)
	assert.Equal(t, "cooldown-tip", tip.ID, "cooldown tip should be available again")
}

// --- RecordShown ---

func TestTipScheduler_RecordShown(t *testing.T) {
	s := agentic.NewTipScheduler(nil)
	s.RecordShown("tip-1")
	s.RecordShown("tip-1")

	rec := s.GetRecord("tip-1")
	require.NotNil(t, rec)
	assert.Equal(t, 2, rec.ShowCount)
	assert.False(t, rec.LastShownAt.IsZero())
}

func TestTipScheduler_GetRecord_NotFound(t *testing.T) {
	s := agentic.NewTipScheduler(nil)
	assert.Nil(t, s.GetRecord("nonexistent"))
}

// --- AddTip ---

func TestTipScheduler_AddTip(t *testing.T) {
	s := agentic.NewTipScheduler(nil)
	s.AddTip(agentic.Tip{ID: "new", Content: "fresh tip"})
	assert.Equal(t, 1, s.TotalTips())
}

// --- Reset ---

func TestTipScheduler_Reset(t *testing.T) {
	s := agentic.NewTipScheduler([]agentic.Tip{{ID: "tip-1"}})
	s.RecordShown("tip-1")
	s.Reset()
	assert.Nil(t, s.GetRecord("tip-1"))
}

// --- AdvanceSession ---

func TestTipScheduler_AdvanceSession(t *testing.T) {
	s := agentic.NewTipScheduler(nil)
	s.AdvanceSession()
	s.AdvanceSession()
	s.RecordShown("tip-1")

	rec := s.GetRecord("tip-1")
	require.NotNil(t, rec)
	assert.Equal(t, 2, rec.SessionsShown)
}
