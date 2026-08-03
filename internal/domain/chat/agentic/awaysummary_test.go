package agentic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Config ---

func TestDefaultAwaySummaryConfig(t *testing.T) {
	cfg := agentic.DefaultAwaySummaryConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30, cfg.MessageWindow)
	assert.Equal(t, agentic.AwayBlurDelay, cfg.BlurDelay)
}

// --- BuildAwaySummaryPrompt ---

func TestBuildAwaySummaryPrompt_NoMemory(t *testing.T) {
	prompt := agentic.BuildAwaySummaryPrompt("")
	assert.Contains(t, prompt, "stepped away")
	assert.Contains(t, prompt, "1-3 short sentences")
	assert.NotContains(t, prompt, "Session memory")
}

func TestBuildAwaySummaryPrompt_WithMemory(t *testing.T) {
	prompt := agentic.BuildAwaySummaryPrompt("Working on auth middleware refactor")
	assert.Contains(t, prompt, "Session memory (broader context)")
	assert.Contains(t, prompt, "auth middleware refactor")
	assert.Contains(t, prompt, "1-3 short sentences")
}

// --- WindowRecentMessages ---

func TestWindowRecentMessages_Empty(t *testing.T) {
	result, err := agentic.WindowRecentMessages(nil, 30)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestWindowRecentMessages_UnderWindow(t *testing.T) {
	msgs := []map[string]string{
		{"role": "user", "content": "hi"},
		{"role": "assistant", "content": "hello"},
	}
	raw, _ := json.Marshal(msgs)

	result, err := agentic.WindowRecentMessages(raw, 30)
	require.NoError(t, err)
	assert.JSONEq(t, string(raw), string(result), "should return all when under window size")
}

func TestWindowRecentMessages_OverWindow(t *testing.T) {
	msgs := make([]map[string]string, 50)
	for i := range msgs {
		msgs[i] = map[string]string{"role": "user", "content": "msg"}
	}
	raw, _ := json.Marshal(msgs)

	result, err := agentic.WindowRecentMessages(raw, 10)
	require.NoError(t, err)

	var windowed []map[string]string
	require.NoError(t, json.Unmarshal(result, &windowed))
	assert.Len(t, windowed, 10, "should truncate to window size")
}

func TestWindowRecentMessages_DefaultWindow(t *testing.T) {
	msgs := make([]map[string]string, 50)
	for i := range msgs {
		msgs[i] = map[string]string{"role": "user", "content": "msg"}
	}
	raw, _ := json.Marshal(msgs)

	result, err := agentic.WindowRecentMessages(raw, 0)
	require.NoError(t, err)

	var windowed []map[string]string
	require.NoError(t, json.Unmarshal(result, &windowed))
	assert.Len(t, windowed, 30, "should use default window")
}

func TestWindowRecentMessages_InvalidJSON(t *testing.T) {
	_, err := agentic.WindowRecentMessages(json.RawMessage(`not json`), 10)
	assert.Error(t, err)
}

// --- HasSummarySinceLastUserTurn ---

func TestHasSummarySinceLastUserTurn_Empty(t *testing.T) {
	assert.False(t, agentic.HasSummarySinceLastUserTurn(nil))
}

func TestHasSummarySinceLastUserTurn_NoSummary(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi"},
	}
	raw, _ := json.Marshal(msgs)
	assert.False(t, agentic.HasSummarySinceLastUserTurn(raw))
}

func TestHasSummarySinceLastUserTurn_SummaryAfterUser(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"type": "system", "subtype": "away_summary", "content": "You were working on auth."},
	}
	raw, _ := json.Marshal(msgs)
	assert.True(t, agentic.HasSummarySinceLastUserTurn(raw))
}

func TestHasSummarySinceLastUserTurn_UserAfterSummary(t *testing.T) {
	msgs := []map[string]interface{}{
		{"type": "system", "subtype": "away_summary", "content": "recap"},
		{"role": "user", "content": "thanks"},
		{"role": "assistant", "content": "welcome back"},
	}
	raw, _ := json.Marshal(msgs)
	assert.False(t, agentic.HasSummarySinceLastUserTurn(raw),
		"summary before last user message should not count")
}

// --- AwaySummaryMessage ---

func TestNewAwaySummaryMessage(t *testing.T) {
	msg := agentic.NewAwaySummaryMessage("You were building the auth module.")
	assert.Equal(t, "system", msg.Type)
	assert.Equal(t, "away_summary", msg.Subtype)
	assert.Equal(t, "You were building the auth module.", msg.Content)
	assert.False(t, msg.Timestamp.IsZero())
}

// --- PrepareAwaySummary ---

func TestPrepareAwaySummary_EmptyMessages(t *testing.T) {
	req := agentic.GenerateAwaySummaryRequest{}
	result, _, _, err := agentic.PrepareAwaySummary(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Skipped)
	assert.Equal(t, "empty conversation", result.SkipReason)
}

func TestPrepareAwaySummary_AlreadyHasSummary(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"type": "system", "subtype": "away_summary", "content": "recap"},
	}
	raw, _ := json.Marshal(msgs)

	req := agentic.GenerateAwaySummaryRequest{Messages: raw}
	result, _, _, err := agentic.PrepareAwaySummary(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "already exists")
}

func TestPrepareAwaySummary_Success(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "help me refactor auth"},
		{"role": "assistant", "content": "I'll help with that."},
	}
	raw, _ := json.Marshal(msgs)

	req := agentic.GenerateAwaySummaryRequest{
		Messages:      raw,
		SessionMemory: "Working on auth middleware",
	}
	result, prompt, windowed, err := agentic.PrepareAwaySummary(context.Background(), req)
	require.NoError(t, err)
	assert.Nil(t, result, "should not be skipped")
	assert.Contains(t, prompt, "stepped away")
	assert.Contains(t, prompt, "auth middleware")
	assert.NotEmpty(t, windowed)
}

func TestPrepareAwaySummary_CustomWindowSize(t *testing.T) {
	msgs := make([]map[string]interface{}, 50)
	for i := range msgs {
		msgs[i] = map[string]interface{}{"role": "user", "content": "msg"}
	}
	raw, _ := json.Marshal(msgs)

	req := agentic.GenerateAwaySummaryRequest{Messages: raw, WindowSize: 5}
	_, _, windowed, err := agentic.PrepareAwaySummary(context.Background(), req)
	require.NoError(t, err)

	var parsed []map[string]interface{}
	require.NoError(t, json.Unmarshal(windowed, &parsed))
	assert.Len(t, parsed, 5)
}

// --- CompleteAwaySummary ---

func TestCompleteAwaySummary(t *testing.T) {
	result := agentic.CompleteAwaySummary("You were building the auth module. Next: add JWT validation.")
	assert.False(t, result.Skipped)
	assert.Equal(t, "You were building the auth module. Next: add JWT validation.", result.Summary)
	assert.Equal(t, "away_summary", result.Message.Subtype)
}
