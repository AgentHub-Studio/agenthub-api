package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestNewTurnState_Initial(t *testing.T) {
	ts := agentic.NewTurnState()
	assert.Equal(t, 0, ts.TurnCount)
	assert.Equal(t, agentic.TransitionNone, ts.Transition)
	assert.Equal(t, 0, ts.MaxOutputTokensRecoveryCount)
	assert.False(t, ts.HasAttemptedReactiveCompact)
	assert.Equal(t, 0, ts.MaxOutputTokensOverride)
	assert.False(t, ts.StopHookActive)
	assert.False(t, ts.StartedAt.IsZero())
}

func TestTurnState_NextTurn(t *testing.T) {
	ts := agentic.NewTurnState()
	ts.NextTurn(agentic.TransitionToolUse)
	assert.Equal(t, 1, ts.TurnCount)
	assert.Equal(t, agentic.TransitionToolUse, ts.Transition)

	ts.NextTurn(agentic.TransitionBudgetContinue)
	assert.Equal(t, 2, ts.TurnCount)
	assert.Equal(t, agentic.TransitionBudgetContinue, ts.Transition)
}

func TestTurnState_RecordMaxTokensRecovery(t *testing.T) {
	ts := agentic.NewTurnState()
	ts.RecordMaxTokensRecovery(8000)
	assert.Equal(t, 1, ts.MaxOutputTokensRecoveryCount)
	assert.Equal(t, 8000, ts.MaxOutputTokensOverride)
	assert.Equal(t, agentic.TransitionMaxTokensRecovery, ts.Transition)
}

func TestTurnState_ShouldRetryMaxTokens(t *testing.T) {
	ts := agentic.NewTurnState()
	assert.True(t, ts.ShouldRetryMaxTokens())

	ts.RecordMaxTokensRecovery(8000)
	assert.True(t, ts.ShouldRetryMaxTokens())

	ts.RecordMaxTokensRecovery(6000)
	assert.True(t, ts.ShouldRetryMaxTokens())

	ts.RecordMaxTokensRecovery(4000)
	assert.False(t, ts.ShouldRetryMaxTokens(), "should stop after 3 recoveries")
}

func TestTurnState_RecordReactiveCompact(t *testing.T) {
	ts := agentic.NewTurnState()
	assert.True(t, ts.CanAttemptReactiveCompact())

	ts.RecordReactiveCompact()
	assert.True(t, ts.HasAttemptedReactiveCompact)
	assert.Equal(t, agentic.TransitionReactiveCompact, ts.Transition)
	assert.False(t, ts.CanAttemptReactiveCompact())
}

func TestTurnState_RecordContextOverflow(t *testing.T) {
	ts := agentic.NewTurnState()
	ts.RecordContextOverflow(5000)
	assert.Equal(t, 5000, ts.MaxOutputTokensOverride)
	assert.Equal(t, agentic.TransitionContextOverflow, ts.Transition)
}

func TestTurnState_EffectiveMaxTokens_Default(t *testing.T) {
	ts := agentic.NewTurnState()
	assert.Equal(t, 4096, ts.EffectiveMaxTokens(4096))
}

func TestTurnState_EffectiveMaxTokens_Override(t *testing.T) {
	ts := agentic.NewTurnState()
	ts.RecordContextOverflow(3000)
	assert.Equal(t, 3000, ts.EffectiveMaxTokens(4096))
}

func TestTurnState_RunDuration(t *testing.T) {
	ts := agentic.NewTurnState()
	time.Sleep(10 * time.Millisecond)
	dur := ts.RunDuration()
	assert.GreaterOrEqual(t, dur.Milliseconds(), int64(10))
}

func TestTurnState_TurnDuration(t *testing.T) {
	ts := agentic.NewTurnState()
	time.Sleep(10 * time.Millisecond)
	ts.NextTurn(agentic.TransitionToolUse) // resets TurnStartedAt
	dur := ts.TurnDuration()
	assert.Less(t, dur.Milliseconds(), int64(10), "turn duration should reset on NextTurn")
}

func TestTurnTransition_Constants(t *testing.T) {
	assert.Equal(t, agentic.TurnTransition(""), agentic.TransitionNone)
	assert.Equal(t, agentic.TurnTransition("tool_use"), agentic.TransitionToolUse)
	assert.Equal(t, agentic.TurnTransition("budget_continue"), agentic.TransitionBudgetContinue)
	assert.Equal(t, agentic.TurnTransition("max_tokens_recovery"), agentic.TransitionMaxTokensRecovery)
	assert.Equal(t, agentic.TurnTransition("reactive_compact"), agentic.TransitionReactiveCompact)
	assert.Equal(t, agentic.TurnTransition("prompt_too_long"), agentic.TransitionPromptTooLong)
	assert.Equal(t, agentic.TurnTransition("context_overflow"), agentic.TransitionContextOverflow)
	assert.Equal(t, agentic.TurnTransition("stop_hook"), agentic.TransitionStopHook)
}

func TestTurnState_MultipleRecoveryPaths(t *testing.T) {
	ts := agentic.NewTurnState()

	// Turn 1: normal tool use
	ts.NextTurn(agentic.TransitionToolUse)
	assert.Equal(t, 1, ts.TurnCount)

	// Turn 2: context overflow → adjust max tokens
	ts.RecordContextOverflow(3000)
	ts.NextTurn(agentic.TransitionContextOverflow)
	assert.Equal(t, 2, ts.TurnCount)
	assert.Equal(t, 3000, ts.EffectiveMaxTokens(4096))

	// Turn 3: reactive compact
	ts.RecordReactiveCompact()
	ts.NextTurn(agentic.TransitionReactiveCompact)
	assert.Equal(t, 3, ts.TurnCount)
	assert.False(t, ts.CanAttemptReactiveCompact())
}

// --- QueryDeps ---

func TestDefaultQueryDeps(t *testing.T) {
	deps := agentic.DefaultQueryDeps()
	assert.Nil(t, deps.CallModel)
	assert.Nil(t, deps.Compact)
	assert.Nil(t, deps.GenerateID)
}
