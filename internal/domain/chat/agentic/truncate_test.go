package agentic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// Tests for truncateToolResult (ACT-F2-27: structured truncation marker).

func TestTruncate_BelowMax_NoMarker(t *testing.T) {
	result := ToolExecResult{Output: json.RawMessage(`{"data":"small"}`)}
	out := truncateToolResult(result, 1000)
	assert.Equal(t, `{"data":"small"}`, string(out.Output))
	assert.NotContains(t, string(out.Output), "TRUNCATED")
}

func TestTruncate_AtMaxChars_NoMarker(t *testing.T) {
	data := strings.Repeat("x", 100)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 100)
	assert.Equal(t, data, string(out.Output))
	assert.NotContains(t, string(out.Output), "TRUNCATED")
}

func TestTruncate_AddsMarker(t *testing.T) {
	data := strings.Repeat("x", 500)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 200)

	s := string(out.Output)
	assert.Contains(t, s, "[TRUNCATED:")
	assert.Contains(t, s, "200")  // shown size
	assert.Contains(t, s, "500")  // original size
	assert.Contains(t, s, "full result available on request")
}

func TestTruncate_ZeroMax_NoTruncation(t *testing.T) {
	data := strings.Repeat("x", 10000)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 0)
	assert.Equal(t, data, string(out.Output))
}

func TestTruncate_OutputLengthWithinBounds(t *testing.T) {
	data := strings.Repeat("y", 1000)
	result := ToolExecResult{Output: json.RawMessage(data)}
	out := truncateToolResult(result, 100)

	// Output length should not exceed maxChars + marker length
	marker := "\n[TRUNCATED: showing first 100 of 1000 chars — full result available on request]"
	assert.LessOrEqual(t, len(out.Output), 100+len(marker))
}

func TestTruncate_VerySmallMax_NoNegativeSlice(t *testing.T) {
	// maxChars smaller than marker length — should not panic
	data := strings.Repeat("z", 200)
	result := ToolExecResult{Output: json.RawMessage(data)}
	assert.NotPanics(t, func() {
		out := truncateToolResult(result, 5)
		assert.Contains(t, string(out.Output), "TRUNCATED")
	})
}

// --- ACT-F3-06: sliding window de histórico (DX-01-M) ---

// mockHistoryLoader implements HistoryLoader and MessagePersister for tests.
type mockHistoryLoader struct {
	msgs []chat.ChatMessage
}

func (m *mockHistoryLoader) FindAllMessages(_ context.Context, _ uuid.UUID) ([]chat.ChatMessage, error) {
	return m.msgs, nil
}

func (m *mockHistoryLoader) CreateMessage(_ context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	msg.ID = uuid.New()
	m.msgs = append(m.msgs, msg)
	return msg, nil
}

func makeChatMessages(n int) []chat.ChatMessage {
	msgs := make([]chat.ChatMessage, n)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = chat.ChatMessage{
			ID:        uuid.New(),
			Role:      role,
			Content:   "message",
			CreatedAt: time.Now(),
		}
	}
	return msgs
}

func TestLoadHistory_SlidingWindow_TruncatesOldMessages(t *testing.T) {
	cfg := DefaultRunConfig()
	cfg.MaxHistoryMessages = 5

	history := &mockHistoryLoader{msgs: makeChatMessages(10)}
	runner := &Runner{config: cfg, history: history}

	msgs, _, err := runner.loadHistory(context.Background(), uuid.New())
	require.NoError(t, err)
	// After filtering (no system/compact messages in mock), should have ≤ 5 messages.
	assert.LessOrEqual(t, len(msgs), 5)
}

func TestLoadHistory_BelowLimit_ReturnsAll(t *testing.T) {
	cfg := DefaultRunConfig()
	cfg.MaxHistoryMessages = 200

	history := &mockHistoryLoader{msgs: makeChatMessages(4)}
	runner := &Runner{config: cfg, history: history}

	msgs, _, err := runner.loadHistory(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 4, len(msgs))
}

func TestLoadHistory_ZeroLimit_NoTruncation(t *testing.T) {
	cfg := DefaultRunConfig()
	cfg.MaxHistoryMessages = 0 // no limit

	history := &mockHistoryLoader{msgs: makeChatMessages(300)}
	runner := &Runner{config: cfg, history: history}

	msgs, _, err := runner.loadHistory(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 300, len(msgs))
}
