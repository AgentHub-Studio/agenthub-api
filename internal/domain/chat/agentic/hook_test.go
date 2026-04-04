package agentic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesToolName_EmptyMatcher(t *testing.T) {
	assert.True(t, matchesToolName("", "execute-sql"))
}

func TestMatchesToolName_ExactMatch(t *testing.T) {
	assert.True(t, matchesToolName("execute-sql", "execute-sql"))
	assert.False(t, matchesToolName("execute-sql", "document-search"))
}

func TestMatchesToolName_GlobPattern(t *testing.T) {
	assert.True(t, matchesToolName("document_*", "document_search"))
	assert.True(t, matchesToolName("document_*", "document_upload"))
	assert.False(t, matchesToolName("document_*", "execute-sql"))
}

func TestMatchesToolName_WildcardAll(t *testing.T) {
	assert.True(t, matchesToolName("*", "anything"))
}

func TestMatchesToolName_EmptyToolName(t *testing.T) {
	assert.False(t, matchesToolName("execute-sql", ""))
	assert.True(t, matchesToolName("", ""))
}

func TestMatchesToolName_QuestionMark(t *testing.T) {
	assert.True(t, matchesToolName("sql-?", "sql-1"))
	assert.False(t, matchesToolName("sql-?", "sql-12"))
}

func TestHookExecutor_NilRepo(t *testing.T) {
	// Execute with nil agent ID should return nil.
	exec := NewHookExecutor(nil)
	results := exec.Execute(context.Background(), HookPayload{
		Event:   HookPreToolUse,
		AgentID: "not-a-uuid",
	})
	assert.Nil(t, results)
}

func TestHookExecutor_NoHooks(t *testing.T) {
	repo := &stubHookRepo{hooks: nil}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPreToolUse,
		AgentID:  "00000000-0000-0000-0000-000000000001",
		ToolName: "execute-sql",
	})
	assert.Empty(t, results)
}

func TestHookExecutor_PromptHook(t *testing.T) {
	hookID := uuid.New()
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       hookID,
				AgentID:  agentID,
				Event:    HookPreToolUse,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"Verify before executing SQL"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPreToolUse,
		AgentID:  agentID.String(),
		ToolName: "execute-sql",
	})
	require.Len(t, results, 1)
	assert.Equal(t, "Verify before executing SQL", results[0].Inject)
	assert.Nil(t, results[0].Error)
}

func TestHookExecutor_MatcherFilters(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookPreToolUse,
				Matcher:  "sql-*",
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"SQL hook"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	// Matching tool.
	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPreToolUse,
		AgentID:  agentID.String(),
		ToolName: "sql-query",
	})
	assert.Len(t, results, 1)

	// Non-matching tool.
	results = exec.Execute(context.Background(), HookPayload{
		Event:    HookPreToolUse,
		AgentID:  agentID.String(),
		ToolName: "document-search",
	})
	assert.Empty(t, results)
}

func TestHookExecutor_HTTPHookInvalidURL(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookPostToolUse,
				HookType: HookTypeHTTP,
				Config:   json.RawMessage(`{"url":"http://localhost:1/hook","timeoutMs":500}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPostToolUse,
		AgentID:  agentID.String(),
		ToolName: "execute-sql",
	})
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error) // should fail — unreachable
}

func TestHookExecutor_UnknownHookType(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookPreToolUse,
				HookType: HookType("unknown"),
				Config:   json.RawMessage(`{}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:   HookPreToolUse,
		AgentID: agentID.String(),
	})
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error)
	assert.Contains(t, *results[0].Error, "unknown hook type")
}

func TestHookExecutor_PromptHookInvalidConfig(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookPreToolUse,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`invalid-json`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:   HookPreToolUse,
		AgentID: agentID.String(),
	})
	require.Len(t, results, 1)
	assert.NotNil(t, results[0].Error)
}

// --- Turn-End Hook Tests ---

func TestHookExecutor_ExecuteTurnEnd_PersistedHook(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookTurnEnd,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"turn ended"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	// Should not panic and execute persisted hook.
	exec.ExecuteTurnEnd(context.Background(), TurnEndPayload{
		Event:     HookTurnEnd,
		AgentID:   agentID.String(),
		SessionID: uuid.New().String(),
		TurnIndex: 0,
	}, nil)
}

func TestHookExecutor_ExecuteTurnEnd_InMemoryHandler(t *testing.T) {
	repo := &stubHookRepo{}
	exec := NewHookExecutor(repo)

	handler := &stubTurnEndHandler{}
	exec.ExecuteTurnEnd(context.Background(), TurnEndPayload{
		Event:            HookTurnEnd,
		AgentID:          uuid.New().String(),
		SessionID:        uuid.New().String(),
		TurnIndex:        2,
		AssistantContent: "Hello",
		ToolCalls:        []ToolCallInfo{{ID: "tc_1", Name: "search"}},
	}, []TurnEndHandler{handler})

	assert.Equal(t, 1, handler.callCount)
	assert.Equal(t, 2, handler.lastPayload.TurnIndex)
	assert.Equal(t, "Hello", handler.lastPayload.AssistantContent)
	require.Len(t, handler.lastPayload.ToolCalls, 1)
	assert.Equal(t, "search", handler.lastPayload.ToolCalls[0].Name)
}

func TestHookExecutor_ExecuteTurnEnd_MultipleHandlers(t *testing.T) {
	exec := NewHookExecutor(&stubHookRepo{})

	h1 := &stubTurnEndHandler{}
	h2 := &stubTurnEndHandler{}
	exec.ExecuteTurnEnd(context.Background(), TurnEndPayload{
		Event:     HookTurnEnd,
		AgentID:   uuid.New().String(),
		SessionID: uuid.New().String(),
		TurnIndex: 0,
	}, []TurnEndHandler{h1, h2})

	assert.Equal(t, 1, h1.callCount)
	assert.Equal(t, 1, h2.callCount)
}

func TestHookExecutor_ExecuteRunEnd_PersistedHook(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookRunEnd,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"run ended"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	exec.ExecuteRunEnd(context.Background(), RunEndPayload{
		Event:       HookRunEnd,
		AgentID:     agentID.String(),
		SessionID:   uuid.New().String(),
		TotalTurns:  3,
		TotalTokens: 1500,
		TotalCost:   0.05,
	}, nil)
}

func TestHookExecutor_ExecuteRunEnd_InMemoryHandler(t *testing.T) {
	exec := NewHookExecutor(&stubHookRepo{})

	handler := &stubRunEndHandler{}
	exec.ExecuteRunEnd(context.Background(), RunEndPayload{
		Event:       HookRunEnd,
		AgentID:     uuid.New().String(),
		SessionID:   uuid.New().String(),
		TotalTurns:  5,
		TotalTokens: 3000,
		TotalCost:   0.10,
	}, []RunEndHandler{handler})

	assert.Equal(t, 1, handler.callCount)
	assert.Equal(t, 5, handler.lastPayload.TotalTurns)
	assert.Equal(t, 3000, handler.lastPayload.TotalTokens)
}

func TestHookExecutor_ExecuteTurnEnd_NilRepo(t *testing.T) {
	exec := NewHookExecutor(nil)

	handler := &stubTurnEndHandler{}
	// Should not panic with nil repo, handlers still execute.
	exec.ExecuteTurnEnd(context.Background(), TurnEndPayload{
		Event:     HookTurnEnd,
		AgentID:   uuid.New().String(),
		SessionID: uuid.New().String(),
	}, []TurnEndHandler{handler})

	assert.Equal(t, 1, handler.callCount)
}

func TestMemoryTurnEndHandler_HandleTurnEnd(t *testing.T) {
	// The MemoryTurnEndHandler is a TurnEndHandler that wraps MaybeStore.
	handler := NewMemoryTurnEndHandler(nil)

	// With nil memory, should return nil error.
	err := handler.HandleTurnEnd(context.Background(), TurnEndPayload{
		Event:     HookTurnEnd,
		AgentID:   uuid.New().String(),
		SessionID: uuid.New().String(),
	})
	assert.NoError(t, err)
}

func TestMemoryTurnEndHandler_InvalidAgentID(t *testing.T) {
	handler := NewMemoryTurnEndHandler(&MemoryBridge{})
	err := handler.HandleTurnEnd(context.Background(), TurnEndPayload{
		Event:            HookTurnEnd,
		AgentID:          "not-a-uuid",
		AssistantContent: "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid agent ID")
}

func TestHookEventConstants(t *testing.T) {
	// Verify the new hook events are defined.
	assert.Equal(t, HookEvent("turn_end"), HookTurnEnd)
	assert.Equal(t, HookEvent("run_end"), HookRunEnd)
}

// --- stubs ---

// stubHookRepo is an in-memory hook repository for testing.
type stubHookRepo struct {
	hooks []AgentHook
}

func (r *stubHookRepo) FindByAgentAndEvent(_ context.Context, agentID uuid.UUID, event HookEvent) ([]AgentHook, error) {
	var result []AgentHook
	for _, h := range r.hooks {
		if h.AgentID == agentID && h.Event == event {
			result = append(result, h)
		}
	}
	return result, nil
}

func (r *stubHookRepo) DisableHook(_ context.Context, hookID uuid.UUID) error {
	for i, h := range r.hooks {
		if h.ID == hookID {
			r.hooks[i].Enabled = false
			return nil
		}
	}
	return nil
}

type stubTurnEndHandler struct {
	callCount   int
	lastPayload TurnEndPayload
}

func (h *stubTurnEndHandler) HandleTurnEnd(_ context.Context, payload TurnEndPayload) error {
	h.callCount++
	h.lastPayload = payload
	return nil
}

type stubRunEndHandler struct {
	callCount   int
	lastPayload RunEndPayload
}

func (h *stubRunEndHandler) HandleRunEnd(_ context.Context, payload RunEndPayload) error {
	h.callCount++
	h.lastPayload = payload
	return nil
}
