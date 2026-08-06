package agentic_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// simpleSummaryChatModel returns a fixed response for Chat calls.
type simpleSummaryChatModel struct {
	response string
	calls    atomic.Int32
}

func (m *simpleSummaryChatModel) Chat(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (*ai.ChatResponse, error) {
	m.calls.Add(1)
	return &ai.ChatResponse{
		Content:      m.response,
		FinishReason: "stop",
	}, nil
}

func (m *simpleSummaryChatModel) ChatStream(_ context.Context, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	ch := make(chan ai.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (m *simpleSummaryChatModel) GetProviderName() string { return "mock" }

func (m *simpleSummaryChatModel) CallCount() int { return int(m.calls.Load()) }

func TestProgressSummarizer_EmitsProgressEvents(t *testing.T) {
	model := &simpleSummaryChatModel{response: "Analyzing main.go"}

	ps := agentic.NewProgressSummarizer(agentic.ProgressSummarizerConfig{
		ChatModel:    model,
		SystemPrompt: "You are a test agent.",
		Model:        "test-model",
		Interval:     50 * time.Millisecond, // fast for testing
	})

	// Seed with some messages.
	ps.UpdateMessages([]ai.Message{
		{Role: ai.RoleUser, Content: "refactor the auth module"},
		{Role: ai.RoleAssistant, Content: "I'll start by reading main.go"},
	})

	ch := make(chan agentic.RunEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go ps.Run(ctx, ch, "subtask-123")

	// Wait for context to expire, then collect events.
	<-ctx.Done()
	time.Sleep(20 * time.Millisecond) // let goroutine drain

	var events []agentic.RunEvent
	for {
		select {
		case ev := <-ch:
			events = append(events, ev)
		default:
			goto done
		}
	}
done:

	require.GreaterOrEqual(t, len(events), 1, "should have emitted at least one progress event")
	assert.Equal(t, agentic.EventSubtaskProgress, events[0].Type)

	var data agentic.SubtaskProgressData
	require.NoError(t, json.Unmarshal(events[0].Data, &data))
	assert.Equal(t, "subtask-123", data.ID)
	assert.Equal(t, "Analyzing main.go", data.Summary)
}

func TestProgressSummarizer_RedactsSecretsInGeneratedSummary(t *testing.T) {
	const secret = "progress-summary-secret-value"
	const internalURL = "http://agenthub-skill-runtime:8080/v1/execute"
	model := &simpleSummaryChatModel{
		response: "Authorization: Bearer " + secret + "\nupstream=" + internalURL,
	}
	ps := agentic.NewProgressSummarizer(agentic.ProgressSummarizerConfig{
		ChatModel: model,
		Model:     "test-model",
		Interval:  20 * time.Millisecond,
	})
	ps.UpdateMessages([]ai.Message{{Role: ai.RoleUser, Content: "investigate the error"}})

	ch := make(chan agentic.RunEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	go ps.Run(ctx, ch, "subtask-redaction")
	<-ctx.Done()
	time.Sleep(20 * time.Millisecond)

	select {
	case event := <-ch:
		var data agentic.SubtaskProgressData
		require.NoError(t, json.Unmarshal(event.Data, &data))
		assert.NotContains(t, data.Summary, secret)
		assert.NotContains(t, data.Summary, internalURL)
		assert.Contains(t, data.Summary, "[REDACTED]")
		assert.Contains(t, data.Summary, "<upstream>")
	default:
		t.Fatal("expected a progress event")
	}
}

func TestProgressSummarizer_SkipsWhenNoMessages(t *testing.T) {
	model := &simpleSummaryChatModel{response: "Something"}

	ps := agentic.NewProgressSummarizer(agentic.ProgressSummarizerConfig{
		ChatModel: model,
		Model:     "test-model",
		Interval:  50 * time.Millisecond,
	})

	// Don't update messages — leave empty.
	ch := make(chan agentic.RunEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	go ps.Run(ctx, ch, "subtask-456")

	<-ctx.Done()
	time.Sleep(20 * time.Millisecond)

	// Should not have called the model or emitted events.
	assert.Equal(t, 0, model.CallCount())
	assert.Equal(t, 0, len(ch))
}

func TestProgressSummarizer_UpdateMessages(t *testing.T) {
	model := &simpleSummaryChatModel{response: "Reading config"}

	ps := agentic.NewProgressSummarizer(agentic.ProgressSummarizerConfig{
		ChatModel: model,
		Model:     "test-model",
		Interval:  50 * time.Millisecond,
	})

	// Initial empty — no events.
	ps.UpdateMessages(nil)

	// Update with actual messages.
	ps.UpdateMessages([]ai.Message{
		{Role: ai.RoleUser, Content: "check config"},
	})

	ch := make(chan agentic.RunEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	go ps.Run(ctx, ch, "test")

	<-ctx.Done()
	time.Sleep(20 * time.Millisecond)

	require.GreaterOrEqual(t, model.CallCount(), 1)
}
