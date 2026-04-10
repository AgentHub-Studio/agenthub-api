package agentic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// mockMetadataPersister captures the last persisted metadata payload.
type mockMetadataPersister struct {
	called   bool
	lastID   uuid.UUID
	lastMeta []byte
}

func (m *mockMetadataPersister) UpdateRunMetadata(_ context.Context, id uuid.UUID, meta json.RawMessage) error {
	m.called = true
	m.lastID = id
	m.lastMeta = meta
	return nil
}

// TestRunMetadata_PersistedOnNormalCompletion verifies that UpdateRunMetadata is called
// when the run finishes normally (finish_reason = "stop"). P-C325-2.
func TestRunMetadata_PersistedOnNormalCompletion(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("hello from the LLM"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	mp := &mockMetadataPersister{}

	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 0
	config.MaxIterations = 1

	runID := uuid.New()
	runner := newTestRunner(model, persister, history, config).
		WithMetadataPersister(mp)

	ch := runner.Run(context.Background(), agentic.RunInput{
		RunID:        runID,
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hi",
		SystemPrompt: "test",
		TenantID:     "t",
	})
	collectEvents(ch)

	require.True(t, mp.called, "UpdateRunMetadata should have been called")
	assert.Equal(t, runID, mp.lastID)

	var meta agentic.RunMetadata
	require.NoError(t, json.Unmarshal(mp.lastMeta, &meta))
	assert.Equal(t, "stop", meta.FinishReason)
	assert.GreaterOrEqual(t, meta.TotalTurns, 0)
	assert.GreaterOrEqual(t, meta.DurationMs, int64(0))
	assert.False(t, meta.HadToolFailures)
}

// TestRunMetadata_NotPersistedWithoutPersister verifies that the runner completes
// without panic when no RunMetadataPersister is configured.
func TestRunMetadata_NotPersistedWithoutPersister(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("hello"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}

	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 0
	config.MaxIterations = 1

	runner := newTestRunner(model, persister, history, config)
	// No WithMetadataPersister — should complete without panic.

	ch := runner.Run(context.Background(), agentic.RunInput{
		RunID:        uuid.New(),
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hi",
		SystemPrompt: "test",
		TenantID:     "t",
	})
	events := collectEvents(ch)
	assert.False(t, hasEventType(events, agentic.EventError))
}

// TestRunMetadata_NotPersistedWhenRunIDIsNil verifies that metadata is skipped
// when RunID is uuid.Nil (e.g. sub-runner invocations without a run record).
func TestRunMetadata_NotPersistedWhenRunIDIsNil(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("hello"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	mp := &mockMetadataPersister{}

	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 0
	config.MaxIterations = 1

	runner := newTestRunner(model, persister, history, config).
		WithMetadataPersister(mp)

	ch := runner.Run(context.Background(), agentic.RunInput{
		RunID:        uuid.Nil, // nil UUID — no DB run record
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hi",
		SystemPrompt: "test",
		TenantID:     "t",
	})
	collectEvents(ch)

	assert.False(t, mp.called, "UpdateRunMetadata should NOT be called when RunID is nil")
}

// TestRunMetadata_CapturesModelAndProvider verifies that model/provider are
// included in the persisted payload (taken from RunConfig, not RunInput). P-C325-2.
func TestRunMetadata_CapturesModelAndProvider(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("hello"), nil
		},
	}

	persister := &mockPersister{}
	history := &mockHistoryLoader{}
	mp := &mockMetadataPersister{}

	config := agentic.DefaultRunConfig()
	config.LLMCallTimeout = 0
	config.MaxIterations = 1
	config.Model = "gpt-4o"
	config.Provider = "openai"

	runner := newTestRunner(model, persister, history, config).
		WithMetadataPersister(mp)

	ch := runner.Run(context.Background(), agentic.RunInput{
		RunID:        uuid.New(),
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "hi",
		SystemPrompt: "test",
		TenantID:     "t",
	})
	collectEvents(ch)

	require.True(t, mp.called)
	var meta agentic.RunMetadata
	require.NoError(t, json.Unmarshal(mp.lastMeta, &meta))
	assert.Equal(t, "gpt-4o", meta.ModelUsed)
	assert.Equal(t, "openai", meta.ProviderUsed)
}
