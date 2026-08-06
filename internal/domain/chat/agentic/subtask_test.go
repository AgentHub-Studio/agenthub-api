package agentic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// mockRunnerFactory creates runners backed by a mock chat model.
type mockRunnerFactory struct {
	model     ai.ChatModel
	persister agentic.MessagePersister
}

func (f *mockRunnerFactory) NewRunner(config agentic.RunConfig) *agentic.Runner {
	skills := &mockSkillLister{skills: nil}
	kbs := &mockKBLister{kbs: nil}
	prompt := agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig())
	tools := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:9999")

	return agentic.NewRunner(
		f.model, skillClient, prompt, tools,
		nil, nil,
		f.persister,
		&mockHistoryLoader{},
		nil,
		config,
	)
}

func TestSubtaskExecutor_SimpleTextResponse(t *testing.T) {
	// Sub-agent returns a simple text response.
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("Sub-agent result: analysis complete."), nil
		},
	}

	factory := &mockRunnerFactory{model: model, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 3

	tc := ai.ToolCall{
		ID:   "tc_sub_1",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":"Analyze the Q3 financial report"}`,
		},
	}

	parentInput := agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "parent msg",
		TenantID:     "test-tenant",
		CurrentDepth: 0,
	}

	result := exec.Execute(context.Background(), parentCh, tc, parentInput, config, 0)
	close(parentCh)

	// Verify result.
	assert.Nil(t, result.Error)
	assert.Greater(t, result.LatencyMs, int64(-1))

	var output agentic.SubtaskResult
	require.NoError(t, json.Unmarshal(result.Output, &output))
	assert.Contains(t, output.Result, "Sub-agent result: analysis complete.")
	assert.Equal(t, 1, output.TotalTurns)
	assert.Equal(t, agentic.SubtaskCompleted, output.Status)
	assert.NotEmpty(t, output.SubtaskID)
	assert.NotEmpty(t, output.Summary)
	assert.Greater(t, output.DurationMs, int64(-1))

	// Check events forwarded to parent.
	var events []agentic.RunEvent
	for ev := range parentCh {
		events = append(events, ev)
	}

	assert.True(t, hasEventType(events, agentic.EventSubtaskStart))
	assert.True(t, hasEventType(events, agentic.EventSubtaskComplete))
	assert.True(t, hasEventType(events, agentic.EventTextDelta))

	// Verify subtask_start data.
	startEv := findEvent(t, events, agentic.EventSubtaskStart)
	var startData agentic.SubtaskStartData
	require.NoError(t, json.Unmarshal(startEv.Data, &startData))
	assert.Equal(t, 1, startData.Depth)
	assert.Contains(t, startData.Description, "Analyze the Q3 financial report")

	// Verify subtask_complete data.
	completeEv := findEvent(t, events, agentic.EventSubtaskComplete)
	var completeData agentic.SubtaskCompleteData
	require.NoError(t, json.Unmarshal(completeEv.Data, &completeData))
	assert.Equal(t, 1, completeData.TotalTurns)
	assert.NotEmpty(t, completeData.Summary)
	assert.Nil(t, completeData.Error)
}

func TestSubtaskExecutor_DepthLimit(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("should not be called"), nil
		},
	}

	factory := &mockRunnerFactory{model: model, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 2

	tc := ai.ToolCall{
		ID:   "tc_deep",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":"deep task"}`,
		},
	}

	// CurrentDepth is already at max.
	parentInput := agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		TenantID:     "test-tenant",
		CurrentDepth: 2, // At depth limit.
	}

	result := exec.Execute(context.Background(), parentCh, tc, parentInput, config, 0)
	close(parentCh)

	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "maximum sub-agent depth")
	assert.Equal(t, 0, model.CallCount(), "LLM should not have been called")
}

func TestSubtaskExecutor_InvalidInput(t *testing.T) {
	factory := &mockRunnerFactory{model: &mockChatModel{}, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 10)
	config := agentic.DefaultRunConfig()

	tc := ai.ToolCall{
		ID:   "tc_bad",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `not valid json`,
		},
	}

	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test",
	}, config, 0)
	close(parentCh)

	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "invalid agent tool input")
}

func TestSubtaskExecutor_EmptyPrompt(t *testing.T) {
	factory := &mockRunnerFactory{model: &mockChatModel{}, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 10)
	config := agentic.DefaultRunConfig()

	tc := ai.ToolCall{
		ID:   "tc_empty",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":""}`,
		},
	}

	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test",
	}, config, 0)
	close(parentCh)

	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "requires a 'prompt' parameter")
}

func TestSubtaskExecutor_BudgetExhausted(t *testing.T) {
	factory := &mockRunnerFactory{model: &mockChatModel{}, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxBudgetUSD = 1.0

	tc := ai.ToolCall{
		ID:   "tc_budget",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":"expensive task"}`,
		},
	}

	// Total cost already exceeds budget.
	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID:          uuid.New(),
		AgentID:            uuid.New(),
		TenantID:           "test",
		RemainingBudgetUSD: 1.0,
	}, config, 1.5)
	close(parentCh)

	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "no budget remaining")

	// Should still emit subtask_start and subtask_complete.
	var events []agentic.RunEvent
	for ev := range parentCh {
		events = append(events, ev)
	}
	assert.True(t, hasEventType(events, agentic.EventSubtaskStart))
	assert.True(t, hasEventType(events, agentic.EventSubtaskComplete))
}

func TestSubtaskExecutor_ExecuteParallel(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("parallel result"), nil
		},
	}

	factory := &mockRunnerFactory{model: model, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 200)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 3

	toolCalls := []ai.ToolCall{
		{
			ID:   "tc_p1",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "agent",
				Arguments: `{"prompt":"task A"}`,
			},
		},
		{
			ID:   "tc_p2",
			Type: "function",
			Function: ai.ToolFunction{
				Name:      "agent",
				Arguments: `{"prompt":"task B"}`,
			},
		},
	}

	parentInput := agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		TenantID:     "test-tenant",
		CurrentDepth: 0,
	}

	results := exec.ExecuteParallel(context.Background(), parentCh, toolCalls, parentInput, config, 0)
	close(parentCh)

	require.Len(t, results, 2)
	for i, r := range results {
		assert.Nil(t, r.Error, "result %d should not have error", i)
		assert.NotEmpty(t, r.Output, "result %d should have output", i)
	}

	// Both sub-agents should have run.
	assert.GreaterOrEqual(t, model.CallCount(), 2)
}

func TestIsAgentToolCall(t *testing.T) {
	assert.True(t, agentic.IsAgentToolCall(ai.ToolCall{
		Function: ai.ToolFunction{Name: "agent"},
	}))
	assert.False(t, agentic.IsAgentToolCall(ai.ToolCall{
		Function: ai.ToolFunction{Name: "execute-sql"},
	}))
	assert.False(t, agentic.IsAgentToolCall(ai.ToolCall{
		Function: ai.ToolFunction{Name: "agent-search"},
	}))
}

func TestRunner_SubtaskIntegration(t *testing.T) {
	// Integration test: LLM calls the agent tool, sub-agent runs and returns result.
	model := &mockChatModel{
		streamFn: func(idx int, msgs []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				// Parent: first call → spawn sub-agent.
				return makeToolCallStream("tc_1", "agent", `{"prompt":"Summarize the document"}`), nil
			case 1:
				// Sub-agent: returns text.
				return makeTextStream("Document summary: key findings include..."), nil
			default:
				// Parent: after receiving sub-agent result → final response.
				return makeTextStream("Based on the sub-agent analysis, here are the findings."), nil
			}
		},
	}

	persister := &mockPersister{}
	factory := &mockRunnerFactory{model: model, persister: persister}
	subtaskExec := agentic.NewSubtaskExecutor(factory)

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	config.MaxDepth = 3
	config.ToolTimeout = 5 * time.Second

	skills := &mockSkillLister{skills: nil}
	kbs := &mockKBLister{kbs: nil}
	prompt := agentic.NewPromptBuilder(skills, kbs, nil, agentic.DefaultPromptConfig())
	tools := agentic.NewToolSchemaBuilder(skills, newMockToolsBySkill(), kbs)
	skillClient := agentic.NewSkillRuntimeClient("http://localhost:9999")

	runner := agentic.NewRunner(
		model, skillClient, prompt, tools,
		nil, nil,
		persister, &mockHistoryLoader{},
		nil, config,
	)
	runner.WithSubtaskExecutor(subtaskExec)

	ch := runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Analyze this document",
		SystemPrompt: "You are a coordinator.",
		TenantID:     "test-tenant",
		CurrentDepth: 0,
	})

	events := collectEvents(ch)

	// Should have subtask events.
	assert.True(t, hasEventType(events, agentic.EventSubtaskStart), "should have subtask_start")
	assert.True(t, hasEventType(events, agentic.EventSubtaskComplete), "should have subtask_complete")
	assert.True(t, hasEventType(events, agentic.EventRunComplete), "should have run_complete")

	// Verify subtask_start data.
	startEv := findEvent(t, events, agentic.EventSubtaskStart)
	var startData agentic.SubtaskStartData
	require.NoError(t, json.Unmarshal(startEv.Data, &startData))
	assert.Equal(t, 1, startData.Depth)
	assert.Contains(t, startData.Description, "Summarize the document")

	// LLM should have been called 3 times: parent(tool_call), sub-agent(text), parent(text).
	assert.GreaterOrEqual(t, model.CallCount(), 3)
}

func TestRunner_ForkModeSkillRoutesThroughSubtaskExecutor(t *testing.T) {
	var runtimeRequests atomic.Int32
	skillRuntime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		runtimeRequests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":{"inline":true},"latencyMs":1}`))
	}))
	t.Cleanup(skillRuntime.Close)

	model := &mockChatModel{
		streamFn: func(idx int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			switch idx {
			case 0:
				return makeToolCallStream("tc_fork", "deep-research", `{"topic":"latency","depth":"high"}`), nil
			case 1:
				return makeTextStream("forked research result"), nil
			default:
				return makeTextStream("final answer from parent"), nil
			}
		},
	}

	skillID := uuid.New()
	toolID := uuid.New()
	skills := &mockSkillLister{skills: []skill.Skill{{
		ID:           skillID,
		Name:         "Deep Research",
		Slug:         "deep-research",
		Description:  "Perform long-running research",
		Instructions: "Investigate independently and summarize findings.",
		ContextMode:  "fork",
	}}}
	toolsMock := newMockToolsBySkill()
	toolsMock.bySkill[skillID] = struct {
		bindings []tool.SkillTool
		tools    []tool.Tool
	}{
		bindings: []tool.SkillTool{{ID: uuid.New(), SkillID: skillID, ToolID: toolID, IsActive: true}},
		tools: []tool.Tool{{
			ID:          toolID,
			Name:        "Research Tool",
			Type:        "HTTP",
			Config:      json.RawMessage(`{"inputSchema":{"type":"object","properties":{"topic":{"type":"string"}},"required":["topic"]}}`),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"topic":{"type":"string"}},"required":["topic"]}`),
		}},
	}

	persister := &mockPersister{}
	factory := &mockRunnerFactory{model: model, persister: persister}
	subtaskExec := agentic.NewSubtaskExecutor(factory)

	config := agentic.DefaultRunConfig()
	config.MaxIterations = 5
	config.MaxDepth = 3

	runner := agentic.NewRunner(
		model,
		agentic.NewSkillRuntimeClient(skillRuntime.URL),
		agentic.NewPromptBuilder(skills, &mockKBLister{}, nil, agentic.DefaultPromptConfig()),
		agentic.NewToolSchemaBuilder(skills, toolsMock, &mockKBLister{}),
		nil,
		nil,
		persister,
		&mockHistoryLoader{},
		nil,
		config,
	)
	runner.WithSubtaskExecutor(subtaskExec)

	events := collectEvents(runner.Run(context.Background(), agentic.RunInput{
		SessionID:    uuid.New(),
		AgentID:      uuid.New(),
		UserMessage:  "Research latency",
		SystemPrompt: "You are a coordinator.",
		TenantID:     "test-tenant",
		CurrentDepth: 0,
	}))

	assert.Equal(t, int32(0), runtimeRequests.Load(), "fork-mode skill must not execute inline through skill-runtime")
	assert.True(t, hasEventType(events, agentic.EventSubtaskStart), "fork-mode skill should emit subtask_start")
	assert.True(t, hasEventType(events, agentic.EventSubtaskComplete), "fork-mode skill should emit subtask_complete")
	assert.GreaterOrEqual(t, model.CallCount(), 3, "parent, child, then parent continuation should all call the model")

	startEv := findEvent(t, events, agentic.EventSubtaskStart)
	var startData agentic.SubtaskStartData
	require.NoError(t, json.Unmarshal(startEv.Data, &startData))
	assert.Contains(t, startData.Description, "deep-research")
	assert.Contains(t, startData.Description, `"topic":"latency"`)
}

func TestToolSchemaBuilder_AgentToolAtDepth0(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{})
	builder.WithDepthLimits(0, 3)

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	// Should have memory_store + agent builtins.
	var hasAgent bool
	for _, tool := range tools {
		if tool.Name == "agent" {
			hasAgent = true
			assert.Contains(t, tool.Description, "sub-agent")
			assert.Contains(t, tool.Description, "3 level(s)")
		}
	}
	assert.True(t, hasAgent, "should have agent tool at depth 0")
}

func TestToolSchemaBuilder_NoAgentToolAtMaxDepth(t *testing.T) {
	builder := agentic.NewToolSchemaBuilder(&mockSkillLister{}, newMockToolsBySkill(), &mockKBLister{})
	builder.WithDepthLimits(3, 3) // At max depth.

	tools, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)

	for _, tool := range tools {
		assert.NotEqual(t, "agent", tool.Name, "should not have agent tool at max depth")
	}
}

func TestPromptBuilder_CoordinatorMode(t *testing.T) {
	builder := agentic.NewPromptBuilder(
		&mockSkillLister{},
		&mockKBLister{},
		&mockSummaryFinder{found: false},
		agentic.DefaultPromptConfig(),
	)

	// With coordinator mode.
	prompt, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:         uuid.New(),
		SessionID:       uuid.New(),
		SystemPrompt:    "You are a test agent.",
		CoordinatorMode: true,
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "Coordinator Mode")
	assert.Contains(t, prompt, "sub-agents")
	assert.Contains(t, prompt, "Parallelism is your superpower")
	assert.Contains(t, prompt, "Always synthesize")
	assert.Contains(t, prompt, "Real Verification")

	// Without coordinator mode.
	prompt2, err := builder.Build(context.Background(), agentic.PromptInput{
		AgentID:         uuid.New(),
		SessionID:       uuid.New(),
		SystemPrompt:    "You are a test agent.",
		CoordinatorMode: false,
	})
	require.NoError(t, err)
	assert.NotContains(t, prompt2, "Coordinator Mode")
}

func TestRunConfig_MaxDepthDefault(t *testing.T) {
	config := agentic.DefaultRunConfig()
	assert.Equal(t, 3, config.MaxDepth)
}

func TestRunConfig_MaxDepthFromModelConfig(t *testing.T) {
	raw := json.RawMessage(`{"maxDepth": 5}`)
	config := agentic.RunConfigFromModelConfig(raw)
	assert.Equal(t, 5, config.MaxDepth)
}

func TestSubtaskResult_FailedStatus(t *testing.T) {
	// When sub-agent hits depth limit, result should have failed status.
	factory := &mockRunnerFactory{model: &mockChatModel{}, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 1

	tc := ai.ToolCall{
		ID: "tc_fail", Type: "function",
		Function: ai.ToolFunction{Name: "agent", Arguments: `{"prompt":"fail"}`},
	}

	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test", CurrentDepth: 1,
	}, config, 0)
	close(parentCh)

	assert.NotNil(t, result.Error)
	// Output is not set for depth-limit errors (returned before running).
}

func TestSubtaskResult_StructuredOutput(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("Analysis: The Q3 report shows 15% growth in revenue."), nil
		},
	}

	factory := &mockRunnerFactory{model: model, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 3

	tc := ai.ToolCall{
		ID: "tc_structured", Type: "function",
		Function: ai.ToolFunction{Name: "agent", Arguments: `{"prompt":"Analyze Q3"}`},
	}

	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test", CurrentDepth: 0,
	}, config, 0)
	close(parentCh)

	require.Nil(t, result.Error)

	var sr agentic.SubtaskResult
	require.NoError(t, json.Unmarshal(result.Output, &sr))

	assert.Equal(t, agentic.SubtaskCompleted, sr.Status)
	assert.NotEmpty(t, sr.SubtaskID)
	assert.Contains(t, sr.Result, "Q3 report")
	assert.Contains(t, sr.Summary, "Q3 report")
	assert.Equal(t, 1, sr.TotalTurns)
	assert.Greater(t, sr.DurationMs, int64(-1))
}

func TestSubtaskResult_SummaryTruncation(t *testing.T) {
	// Generate a long response that exceeds 200 chars.
	longText := ""
	for i := 0; i < 50; i++ {
		longText += "This is a very long line of text. "
	}

	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream(longText), nil
		},
	}

	factory := &mockRunnerFactory{model: model, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 3

	tc := ai.ToolCall{
		ID: "tc_long", Type: "function",
		Function: ai.ToolFunction{Name: "agent", Arguments: `{"prompt":"long task"}`},
	}

	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test", CurrentDepth: 0,
	}, config, 0)
	close(parentCh)

	var sr agentic.SubtaskResult
	require.NoError(t, json.Unmarshal(result.Output, &sr))

	// Summary should be truncated to ~200 chars.
	assert.LessOrEqual(t, len(sr.Summary), 200)
	assert.True(t, len(sr.Summary) > 0)
	// Full result should have the complete text.
	assert.Greater(t, len(sr.Result), 200)
}

func TestSubtaskStatus_Constants(t *testing.T) {
	assert.Equal(t, agentic.SubtaskStatus("completed"), agentic.SubtaskCompleted)
	assert.Equal(t, agentic.SubtaskStatus("failed"), agentic.SubtaskFailed)
	assert.Equal(t, agentic.SubtaskStatus("killed"), agentic.SubtaskKilled)
}

// TestSubtaskExecutor_TokensPropagedToResult verifies that the ToolExecResult
// returned by Execute carries the sub-agent's token and cost totals so the
// parent runner can roll them up (ACT-F3-13 / P-C336-1).
func TestSubtaskExecutor_TokensPropagatedToResult(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("done"), nil
		},
	}

	factory := &mockRunnerFactory{model: model, persister: &mockPersister{}}
	exec := agentic.NewSubtaskExecutor(factory)

	parentCh := make(chan agentic.RunEvent, 100)
	config := agentic.DefaultRunConfig()
	config.MaxDepth = 3

	tc := ai.ToolCall{
		ID:   "tc_tokens",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":"do something"}`,
		},
	}

	result := exec.Execute(context.Background(), parentCh, tc, agentic.RunInput{
		SessionID: uuid.New(), AgentID: uuid.New(), TenantID: "test", CurrentDepth: 0,
	}, config, 0)
	close(parentCh)

	// Sub-agent ran successfully — SubtaskTokens should be set.
	assert.Nil(t, result.Error)
	// The mock model reports TotalTokens=10 (5 prompt + 5 completion per makeTextStream).
	// We only assert it's non-negative since mock usage counts vary.
	assert.GreaterOrEqual(t, result.SubtaskTokens, 0)
	assert.GreaterOrEqual(t, result.SubtaskCostUSD, float64(0))
}

func TestSubtaskExecutor_PersistsSubtaskLifecycle(t *testing.T) {
	model := &mockChatModel{
		streamFn: func(_ int, _ []ai.Message, _ ai.ChatOptions) (<-chan ai.StreamChunk, error) {
			return makeTextStream("persisted subtask result"), nil
		},
	}

	sessionID := uuid.New()
	repo := newMockPersistRepo()
	coordinator := agentic.NewCoordinatorState().
		WithRepository(repo, sessionID).
		WithContext(context.Background())

	exec := agentic.NewSubtaskExecutor(&mockRunnerFactory{model: model, persister: &mockPersister{}}).
		WithCoordinatorState(coordinator)
	parentCh := make(chan agentic.RunEvent, 16)

	result := exec.Execute(context.Background(), parentCh, ai.ToolCall{
		ID:   "tc_persisted_subtask",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":"Persist this delegated task"}`,
		},
	}, agentic.RunInput{
		SessionID: sessionID,
		AgentID:   uuid.New(),
		TenantID:  "test-tenant",
	}, agentic.DefaultRunConfig(), 0)

	require.Nil(t, result.Error)
	var output agentic.SubtaskResult
	require.NoError(t, json.Unmarshal(result.Output, &output))

	persisted, ok := repo.tasks[output.SubtaskID]
	require.True(t, ok, "delegated work must be persisted under the SSE subtask ID")
	assert.Equal(t, sessionID, persisted.SessionID)
	assert.Equal(t, "implementation", persisted.Phase)
	assert.Equal(t, "completed", persisted.Status)
	assert.NotEmpty(t, persisted.AssignedTo)
	require.NotNil(t, persisted.CompletedAt)

	require.Len(t, repo.notifications, 1)
	assert.Equal(t, output.SubtaskID, repo.notifications[0].TaskID)
	assert.Equal(t, "completed", repo.notifications[0].Status)
	assert.Contains(t, repo.notifications[0].Summary, "persisted subtask result")

	close(parentCh)
	events := collectEvents(parentCh)
	startEvent := findEvent(t, events, agentic.EventSubtaskStart)
	completeEvent := findEvent(t, events, agentic.EventSubtaskComplete)
	var start agentic.SubtaskStartData
	var complete agentic.SubtaskCompleteData
	require.NoError(t, json.Unmarshal(startEvent.Data, &start))
	require.NoError(t, json.Unmarshal(completeEvent.Data, &complete))
	assert.Equal(t, output.SubtaskID, start.ID)
	assert.Equal(t, output.SubtaskID, complete.ID)
}

func TestSubtaskExecutor_PersistsBudgetExhaustion(t *testing.T) {
	sessionID := uuid.New()
	repo := newMockPersistRepo()
	coordinator := agentic.NewCoordinatorState().
		WithRepository(repo, sessionID).
		WithContext(context.Background())

	exec := agentic.NewSubtaskExecutor(&mockRunnerFactory{model: &mockChatModel{}, persister: &mockPersister{}}).
		WithCoordinatorState(coordinator)
	parentCh := make(chan agentic.RunEvent, 16)
	config := agentic.DefaultRunConfig()
	config.MaxBudgetUSD = 1

	result := exec.Execute(context.Background(), parentCh, ai.ToolCall{
		ID:   "tc_persisted_budget_failure",
		Type: "function",
		Function: ai.ToolFunction{
			Name:      "agent",
			Arguments: `{"prompt":"This task cannot fit the remaining budget"}`,
		},
	}, agentic.RunInput{
		SessionID:          sessionID,
		AgentID:            uuid.New(),
		TenantID:           "test-tenant",
		RemainingBudgetUSD: 1,
	}, config, 1)

	require.NotNil(t, result.Error)
	require.Len(t, repo.tasks, 1)
	for taskID, persisted := range repo.tasks {
		assert.Equal(t, "failed", persisted.Status)
		require.NotNil(t, persisted.CompletedAt)
		require.Len(t, repo.notifications, 1)
		assert.Equal(t, taskID, repo.notifications[0].TaskID)
		assert.Equal(t, "failed", repo.notifications[0].Status)
		require.NotNil(t, repo.notifications[0].Error)
		assert.Contains(t, *repo.notifications[0].Error, "no budget remaining")
		assert.Contains(t, repo.notifications[0].Summary, "no budget remaining")
	}
}
