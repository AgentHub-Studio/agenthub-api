package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

// --- TR-01-TASK-16: session snapshot (P-C115-1, P-C330-1) ---

func strPtr(s string) *string { return &s }

func TestResolveSystemPrompt_UsesSnapshotWhenSet(t *testing.T) {
	in := chat.RunInput{
		SystemPromptSnapshot: strPtr("You are ARIA."),
	}
	agentCfg := &chat.AgentRunConfig{SystemPrompt: "You are ZEUS."}

	result := resolveSystemPrompt(in, agentCfg)

	assert.Equal(t, "You are ARIA.", result, "snapshot must take precedence over current agent prompt")
	assert.NotContains(t, result, "ZEUS")
}

func TestResolveSystemPrompt_NilSnapshot_FallsBackToCurrentPrompt(t *testing.T) {
	in := chat.RunInput{SystemPromptSnapshot: nil}
	agentCfg := &chat.AgentRunConfig{SystemPrompt: "Current prompt"}

	result := resolveSystemPrompt(in, agentCfg)

	assert.Equal(t, "Current prompt", result)
}

func TestResolveSystemPrompt_EmptySnapshot_FallsBackToCurrentPrompt(t *testing.T) {
	in := chat.RunInput{SystemPromptSnapshot: strPtr("")}
	agentCfg := &chat.AgentRunConfig{SystemPrompt: "Current prompt"}

	result := resolveSystemPrompt(in, agentCfg)

	assert.Equal(t, "Current prompt", result)
}

func TestResolveSystemPrompt_AfterAgentUpdate_SnapshotUnchanged(t *testing.T) {
	original := "Original prompt"
	in := chat.RunInput{SystemPromptSnapshot: &original}
	updatedAgent := &chat.AgentRunConfig{SystemPrompt: "Updated prompt"}

	result := resolveSystemPrompt(in, updatedAgent)

	assert.Equal(t, "Original prompt", result)
}

func TestResolveModelConfig_UsesSnapshotWhenSet(t *testing.T) {
	snapshot := json.RawMessage(`{"provider":"openai","model":"gpt-4o"}`)
	in := chat.RunInput{ModelConfigSnapshot: snapshot}
	agentCfg := &chat.AgentRunConfig{ModelConfig: json.RawMessage(`{"provider":"anthropic","model":"claude-3"}`)}

	result := resolveModelConfig(in, agentCfg)

	assert.Contains(t, string(result), "openai")
}

func TestResolveModelConfig_NilSnapshot_FallsBackToAgentConfig(t *testing.T) {
	in := chat.RunInput{ModelConfigSnapshot: nil}
	agentCfg := &chat.AgentRunConfig{ModelConfig: json.RawMessage(`{"provider":"anthropic"}`)}

	result := resolveModelConfig(in, agentCfg)

	assert.Contains(t, string(result), "anthropic")
}
