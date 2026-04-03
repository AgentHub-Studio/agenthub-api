package agentic_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestStreamingToolExecutor_EmitsProgressEvents(t *testing.T) {
	// Start a local HTTP server that responds to skill execution.
	// For unit test, we use a skill client pointing to a non-existent server.
	// The tool will fail, but we can verify the event sequence.
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:1") // will fail

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 1 * time.Second
	config.ConcurrentReadTools = 2

	executor := agentic.NewStreamingToolExecutor(skillClient, nil, config)

	ch := make(chan agentic.RunEvent, 100)

	toolCalls := []ai.ToolCall{
		{
			ID:   "tc_1",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "execute-sql",
				Arguments: `{"query":"SELECT 1"}`,
			},
		},
	}

	results := executor.ExecuteAll(context.Background(), ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	})
	close(ch)

	// Should have 1 result (with error since server is unreachable).
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error)

	// Collect events.
	var events []agentic.RunEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Should have: tool_call_start, tool_progress(queued), tool_progress(executing), tool_progress(completed)
	types := make([]agentic.RunEventType, len(events))
	for i, ev := range events {
		types[i] = ev.Type
	}

	assert.Contains(t, types, agentic.EventToolCallStart)
	assert.Contains(t, types, agentic.EventToolProgress)

	// Verify progress states.
	var states []agentic.ToolState
	for _, ev := range events {
		if ev.Type == agentic.EventToolProgress {
			var pd agentic.ToolProgressData
			require.NoError(t, json.Unmarshal(ev.Data, &pd))
			states = append(states, pd.State)
		}
	}
	assert.Contains(t, states, agentic.ToolStateQueued)
	assert.Contains(t, states, agentic.ToolStateExecuting)
	// Should have completed (even with error, we emit completed not aborted).
	assert.Contains(t, states, agentic.ToolStateCompleted)
}

func TestStreamingToolExecutor_ContextCancelled(t *testing.T) {
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:1")
	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 1 * time.Second

	executor := agentic.NewStreamingToolExecutor(skillClient, nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch := make(chan agentic.RunEvent, 100)
	toolCalls := []ai.ToolCall{
		{ID: "tc_1", Type: "function", Function: ai.ToolFunction{Name: "test", Arguments: "{}"}},
	}

	results := executor.ExecuteAll(ctx, ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	})
	close(ch)

	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error)
}

func TestStreamingToolExecutor_MultipleTools(t *testing.T) {
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:1")

	config := agentic.DefaultRunConfig()
	config.ToolTimeout = 1 * time.Second
	config.ConcurrentReadTools = 3

	executor := agentic.NewStreamingToolExecutor(skillClient, nil, config)

	ch := make(chan agentic.RunEvent, 100)

	toolCalls := []ai.ToolCall{
		{ID: "tc_1", Type: "function", Function: ai.ToolFunction{Name: "tool-a", Arguments: "{}"}},
		{ID: "tc_2", Type: "function", Function: ai.ToolFunction{Name: "tool-b", Arguments: "{}"}},
		{ID: "tc_3", Type: "function", Function: ai.ToolFunction{Name: "tool-c", Arguments: "{}"}},
	}

	results := executor.ExecuteAll(context.Background(), ch, toolCalls, agentic.RunInput{
		SessionID: uuid.New(),
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	})
	close(ch)

	// All tools should have results (all failed since server is unreachable).
	// First error triggers abort cascade, so some may be aborted.
	require.Len(t, results, 3)
	for _, r := range results {
		assert.NotNil(t, r.Error, "each result should have an error")
	}
}
