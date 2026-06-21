package agentic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestHookExecutor_WebhookHookParsesStructuredResult(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"continue":true,"modified":{"query":"select 1"},"inject":"webhook noted"}`))
	}))
	defer ts.Close()

	repo := &stubHookRepo{hooks: []AgentHook{{
		ID:       uuid.New(),
		AgentID:  agentID,
		Event:    HookPreToolUse,
		HookType: HookTypeWebhook,
		Config:   json.RawMessage(`{"url":"` + ts.URL + `"}`),
		Enabled:  true,
	}}}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPreToolUse,
		AgentID:  agentID.String(),
		ToolName: "execute-sql",
	})

	require.Len(t, results, 1)
	assert.True(t, results[0].Continue)
	assert.JSONEq(t, `{"query":"select 1"}`, string(results[0].Modified))
	assert.Equal(t, "webhook noted", results[0].Inject)
}

func TestHookExecutor_TransformHookReturnsModifiedPayload(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{hooks: []AgentHook{{
		ID:       uuid.New(),
		AgentID:  agentID,
		Event:    HookPreToolUse,
		HookType: HookTypeTransform,
		Config:   json.RawMessage(`{"toolInput":{"query":"select safe"}}`),
		Enabled:  true,
	}}}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:     HookPreToolUse,
		AgentID:   agentID.String(),
		ToolName:  "execute-sql",
		ToolInput: json.RawMessage(`{"query":"drop table users"}`),
	})

	require.Len(t, results, 1)
	assert.True(t, results[0].Continue)
	assert.JSONEq(t, `{"query":"select safe"}`, string(results[0].Modified))
}

func TestHookExecutor_ScriptHookCanBlockExecution(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{hooks: []AgentHook{{
		ID:       uuid.New(),
		AgentID:  agentID,
		Event:    HookPreToolUse,
		HookType: HookTypeScript,
		Config:   json.RawMessage(`{"continue":false,"reason":"SQL denied by policy"}`),
		Enabled:  true,
	}}}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPreToolUse,
		AgentID:  agentID.String(),
		ToolName: "execute-sql",
	})

	require.Len(t, results, 1)
	assert.False(t, results[0].Continue)
	require.NotNil(t, results[0].Error)
	assert.Equal(t, "SQL denied by policy", *results[0].Error)
}

func TestApplyPreToolHookResults_ModifiesAndBlocks(t *testing.T) {
	errMsg := "blocked by hook"
	next, blocked := applyPreToolHookResults([]HookResult{
		{Continue: true, Modified: json.RawMessage(`{"query":"select 1"}`)},
		{Continue: false, Error: &errMsg},
	}, json.RawMessage(`{"query":"drop table users"}`))

	assert.JSONEq(t, `{"query":"select 1"}`, string(next))
	require.NotNil(t, blocked)
	assert.Equal(t, "blocked by hook", *blocked)
}

func TestApplyPostToolHookResults_ModifiesOutputAndInjects(t *testing.T) {
	result := ToolExecResult{Output: json.RawMessage(`{"ok":false}`)}

	applyPostToolHookResults([]HookResult{
		{Continue: true, Modified: json.RawMessage(`{"ok":true}`), Inject: "first"},
		{Continue: true, Inject: "second"},
	}, &result)

	assert.JSONEq(t, `{"ok":true}`, string(result.Output))
	assert.Equal(t, "first\nsecond", result.InjectText)
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
	assert.Equal(t, HookEvent("pre_llm_call"), HookPreLLMCall)
	assert.Equal(t, HookEvent("post_llm_call"), HookPostLLMCall)
	assert.Equal(t, HookEvent("on_error"), HookOnError)
	assert.Equal(t, HookEvent("on_complete"), HookOnComplete)
}

// TestPromptHook_TemplateSubstitution verifies that {{.ToolName}}, {{.AgentID}},
// etc. are actually substituted in the template. BUG-HOOK-TEMPLATE: previously
// the template was returned as a raw string without execution.
func TestPromptHook_TemplateSubstitution(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookPostToolUse,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"Tool {{.ToolName}} was called by agent {{.AgentID}}"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPostToolUse,
		AgentID:  agentID.String(),
		ToolName: "memory_store",
	})
	require.Len(t, results, 1)
	assert.Equal(t, "Tool memory_store was called by agent 00000000-0000-0000-0000-000000000001", results[0].Inject)
	assert.Nil(t, results[0].Error)
}

// TestPromptHook_StaticTemplate_NoSubstitution verifies that templates without
// {{...}} actions are returned unchanged (backward-compatible).
func TestPromptHook_StaticTemplate_NoSubstitution(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookPostToolUse,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"COMPLIANCE HOOK FIRED: Tool execution logged."}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	results := exec.Execute(context.Background(), HookPayload{
		Event:    HookPostToolUse,
		AgentID:  agentID.String(),
		ToolName: "memory_store",
	})
	require.Len(t, results, 1)
	assert.Equal(t, "COMPLIANCE HOOK FIRED: Tool execution logged.", results[0].Inject)
}

// TestExecuteTurnEnd_ReturnsInjectTexts verifies that turn-end hooks surface their
// inject texts via the return value. BUG-HOOK-TURNEND-INJECT: previously the
// inject field was discarded; only Error was checked.
func TestExecuteTurnEnd_ReturnsInjectTexts(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookTurnEnd,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"TURN-END: turn {{.TurnIndex}} completed"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	injects := exec.ExecuteTurnEnd(context.Background(), TurnEndPayload{
		Event:     HookTurnEnd,
		AgentID:   agentID.String(),
		SessionID: uuid.New().String(),
		TurnIndex: 3,
	}, nil)

	require.Len(t, injects, 1)
	assert.Equal(t, "TURN-END: turn 3 completed", injects[0])
}

// TestExecuteTurnEnd_EmptyInjectWhenNoHooks verifies no inject texts are returned
// when no turn-end hooks are registered.
func TestExecuteTurnEnd_EmptyInjectWhenNoHooks(t *testing.T) {
	exec := NewHookExecutor(&stubHookRepo{})
	injects := exec.ExecuteTurnEnd(context.Background(), TurnEndPayload{
		Event:     HookTurnEnd,
		AgentID:   uuid.New().String(),
		SessionID: uuid.New().String(),
	}, nil)
	assert.Empty(t, injects)
}

// TestExecuteRunEnd_ReturnsInjectTexts verifies that run-end hook inject texts
// are returned so the runner can persist them as audit notes.
// Previously ExecuteRunEnd was void and discarded all inject content.
func TestExecuteRunEnd_ReturnsInjectTexts(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{
		hooks: []AgentHook{
			{
				ID:       uuid.New(),
				AgentID:  agentID,
				Event:    HookRunEnd,
				HookType: HookTypePrompt,
				Config:   json.RawMessage(`{"template":"RUN-END: {{.TotalTurns}} turns, {{.TotalTokens}} tokens, ${{.TotalCostUSD}}"}`),
				Enabled:  true,
			},
		},
	}
	exec := NewHookExecutor(repo)

	injects := exec.ExecuteRunEnd(context.Background(), RunEndPayload{
		Event:       HookRunEnd,
		AgentID:     agentID.String(),
		SessionID:   uuid.New().String(),
		TotalTurns:  5,
		TotalTokens: 2500,
		TotalCost:   0.025,
	}, nil)

	require.Len(t, injects, 1)
	assert.Contains(t, injects[0], "5 turns")
	assert.Contains(t, injects[0], "2500 tokens")
}

func TestExecuteRunEnd_RunsOnCompleteAlias(t *testing.T) {
	agentID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo := &stubHookRepo{hooks: []AgentHook{{
		ID:       uuid.New(),
		AgentID:  agentID,
		Event:    HookOnComplete,
		HookType: HookTypePrompt,
		Config:   json.RawMessage(`{"template":"ON-COMPLETE: {{.TotalTurns}}"}`),
		Enabled:  true,
	}}}
	exec := NewHookExecutor(repo)

	injects := exec.ExecuteRunEnd(context.Background(), RunEndPayload{
		Event:      HookRunEnd,
		AgentID:    agentID.String(),
		SessionID:  uuid.New().String(),
		TotalTurns: 7,
	}, nil)

	require.Len(t, injects, 1)
	assert.Equal(t, "ON-COMPLETE: 7", injects[0])
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
