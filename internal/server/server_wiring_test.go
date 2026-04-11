package server

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- agentConfigAdapter tests ---

type stubAgentRepo struct {
	agents map[uuid.UUID]agent.Agent
	err    error
}

func (s *stubAgentRepo) FindAll(_ context.Context, _ agent.AgentStatus, _ string, _ pagination.PageRequest) ([]agent.Agent, int64, error) {
	return nil, 0, nil
}

func (s *stubAgentRepo) FindByID(_ context.Context, id uuid.UUID) (agent.Agent, error) {
	if s.err != nil {
		return agent.Agent{}, s.err
	}
	a, ok := s.agents[id]
	if !ok {
		return agent.Agent{}, agent.ErrNotFound
	}
	return a, nil
}

func (s *stubAgentRepo) Create(_ context.Context, a agent.Agent) (agent.Agent, error) {
	return a, nil
}

func (s *stubAgentRepo) Update(_ context.Context, a agent.Agent) (agent.Agent, error) {
	return a, nil
}

func (s *stubAgentRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func (s *stubAgentRepo) UpdateStatus(_ context.Context, id uuid.UUID, _ agent.AgentStatus) (agent.Agent, error) {
	return s.agents[id], nil
}

func (s *stubAgentRepo) CountPublishedWithoutProvider(_ context.Context) (int64, error) {
	return 0, nil
}

func TestAgentConfigAdapter_GetAgentForRun_Success(t *testing.T) {
	agentID := uuid.New()
	systemPrompt := "You are a helpful assistant."
	modelCfg := json.RawMessage(`{"provider":"anthropic","model":"claude-sonnet-4-20250514"}`)

	repo := &stubAgentRepo{
		agents: map[uuid.UUID]agent.Agent{
			agentID: {
				ID:           agentID,
				Name:         "Test Agent",
				SystemPrompt: &systemPrompt,
				ModelConfig:  modelCfg,
			},
		},
	}

	adapter := &agentConfigAdapter{repo: repo}
	cfg, err := adapter.GetAgentForRun(context.Background(), agentID)

	require.NoError(t, err)
	assert.Equal(t, agentID, cfg.ID)
	assert.Equal(t, systemPrompt, cfg.SystemPrompt)
	assert.Equal(t, modelCfg, cfg.ModelConfig)
}

func TestAgentConfigAdapter_GetAgentForRun_NilSystemPrompt(t *testing.T) {
	agentID := uuid.New()
	repo := &stubAgentRepo{
		agents: map[uuid.UUID]agent.Agent{
			agentID: {ID: agentID, Name: "No Prompt"},
		},
	}

	adapter := &agentConfigAdapter{repo: repo}
	cfg, err := adapter.GetAgentForRun(context.Background(), agentID)

	require.NoError(t, err)
	assert.Equal(t, "", cfg.SystemPrompt)
}

func TestAgentConfigAdapter_GetAgentForRun_NotFound(t *testing.T) {
	adapter := &agentConfigAdapter{repo: &stubAgentRepo{agents: map[uuid.UUID]agent.Agent{}}}
	_, err := adapter.GetAgentForRun(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent config")
}

// --- buildDefaultChatModel tests ---

func TestBuildDefaultChatModel_NoProviders(t *testing.T) {
	// Unset all provider env vars.
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(key, "")
	}
	// Reset Ollama to default (which is skipped).
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")

	model := buildDefaultChatModel()
	assert.Nil(t, model)
}

func TestBuildDefaultChatModel_Anthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "anthropic", model.GetProviderName())
}

func TestBuildDefaultChatModel_OpenAI(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("OPENROUTER_API_KEY", "")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "openai", model.GetProviderName())
}

func TestBuildDefaultChatModel_Ollama(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OLLAMA_BASE_URL", "http://custom-ollama:11434")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "ollama", model.GetProviderName())
}

func TestBuildDefaultChatModel_OpenRouter(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")
	t.Setenv("OPENROUTER_API_KEY", "test-or-key")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "openrouter", model.GetProviderName())
}

func TestBuildDefaultChatModel_PriorityOrder(t *testing.T) {
	// When multiple providers are set, Anthropic takes priority.
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("OPENROUTER_API_KEY", "or-key")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "anthropic", model.GetProviderName())
}

// --- buildAgenticRunner tests ---

func TestBuildAgenticRunner_ReturnsRunnerWithoutEnvProvider(t *testing.T) {
	// Ensure no providers are configured.
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")

	// The runner is still constructed; provider resolution may happen later via settings.
	runner := buildAgenticRunner(
		&config.Config{SkillRuntimeURL: "http://localhost:8083"},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	assert.NotNil(t, runner)
}

func TestBuildAgenticRunner_ReturnsRunnerWhenProviderConfigured(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	// Pass nil repos — buildAgenticRunner only needs the chatModel check to pass;
	// the repos are wrapped in adapters but not called at construction time.
	runner := buildAgenticRunner(
		&config.Config{SkillRuntimeURL: "http://localhost:8083"},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	assert.NotNil(t, runner)
}

// Prevent "imported and not used" for os.
var _ = os.Getenv
