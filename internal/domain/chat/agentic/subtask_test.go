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

	var output map[string]any
	require.NoError(t, json.Unmarshal(result.Output, &output))
	assert.Contains(t, output["result"], "Sub-agent result: analysis complete.")
	assert.Equal(t, float64(1), output["turns"])

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
