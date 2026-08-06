package agentic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// mockAgentConfigLoader is a test double for agentic.AgentConfigLoader.
type mockAgentConfigLoader struct {
	cfg *chat.AgentRunConfig
	err error
}

func (m *mockAgentConfigLoader) GetAgentForRun(_ context.Context, _ uuid.UUID) (*chat.AgentRunConfig, error) {
	return m.cfg, m.err
}

// newMinimalAdapter creates an adapter with only the agentLoader wired.
// Only suitable for testing the early lifecycle check before the runner starts.
func newMinimalAdapter(loader agentic.AgentConfigLoader) *agentic.SessionRunnerAdapter {
	return agentic.NewSessionRunnerAdapterWithFactory(
		nil, // factory — never reached because lifecycle check fires first
		nil, // skillClient
		nil, // prompt
		nil, // tools
		nil, // ctxManager
		nil, // memory
		nil, // hookExecutor
		nil, // repo
		loader,
		nil, // agentRepo
		nil, // skillRepo
		nil, // toolRepo
		nil, // integRepo
		nil, // mcpRepo
	)
}

// --- TR-01-TASK-11: Agent lifecycle check before run (P-C178-1) ---

// TestRunSession_DraftAgent_ReturnsErrAgentNotPublished verifies that starting a run
// for a DRAFT agent returns ErrAgentNotPublished.
func TestRunSession_DraftAgent_ReturnsErrAgentNotPublished(t *testing.T) {
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		Status: "DRAFT",
		ID:     uuid.New(),
	}}
	adapter := newMinimalAdapter(loader)

	_, err := adapter.RunSession(context.Background(), chat.RunInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
		TenantID:  "test",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, chat.ErrAgentNotPublished), "expected ErrAgentNotPublished, got: %v", err)
}

// TestRunSession_ArchivedAgent_ReturnsErrAgentArchived verifies that starting a run
// for an ARCHIVED agent returns ErrAgentArchived.
func TestRunSession_ArchivedAgent_ReturnsErrAgentArchived(t *testing.T) {
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		Status: "ARCHIVED",
		ID:     uuid.New(),
	}}
	adapter := newMinimalAdapter(loader)

	_, err := adapter.RunSession(context.Background(), chat.RunInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
		TenantID:  "test",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, chat.ErrAgentArchived), "expected ErrAgentArchived, got: %v", err)
}

func TestRunSession_UsesProvidedAgentConfig(t *testing.T) {
	agentID := uuid.New()
	loader := &mockAgentConfigLoader{err: errors.New("loader must not be called")}
	adapter := newMinimalAdapter(loader)

	_, err := adapter.RunSession(context.Background(), chat.RunInput{
		AgentID:   agentID,
		SessionID: uuid.New(),
		TenantID:  "test",
		AgentConfig: &chat.AgentRunConfig{
			ID:     agentID,
			Status: "DRAFT",
		},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, chat.ErrAgentNotPublished), "expected provided config lifecycle validation, got: %v", err)
}

// --- OOB: model build failure surfaces a provider-tagged error ---

// buildErrorFactory is a ChatModelFactory whose Build always fails — simulating
// a tenant that has not configured the provider's API key yet.
type buildErrorFactory struct{}

func (buildErrorFactory) Build(context.Context, string, string) (ai.ChatModel, error) {
	return nil, errors.New("chat model: openrouter.apiKey not configured in settings")
}
func (buildErrorFactory) ResolveModel(context.Context, string) string   { return "" }
func (buildErrorFactory) ResolveDefaultProvider(context.Context) string { return "" }

// TestRunSession_BuildError_WrapsProvider verifies that when the model factory
// cannot build a ChatModel (e.g. missing API key), RunSession returns an error
// that names the provider — friendlyStartupError relies on that tag to point
// the user at the right settings row.
func TestRunSession_BuildError_WrapsProvider(t *testing.T) {
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		Status:      "PUBLISHED",
		ID:          uuid.New(),
		ModelConfig: json.RawMessage(`{"provider":"openrouter","model":"mistralai/mistral-nemo"}`),
	}}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		buildErrorFactory{}, // factory — reached because the agent is PUBLISHED
		nil,                 // skillClient
		nil,                 // prompt
		nil,                 // tools
		nil,                 // ctxManager
		nil,                 // memory
		nil,                 // hookExecutor
		nil,                 // repo
		loader,
		nil, // agentRepo
		nil, // skillRepo
		nil, // toolRepo
		nil, // integRepo
		nil, // mcpRepo
	)

	_, err := adapter.RunSession(context.Background(), chat.RunInput{
		AgentID:   uuid.New(),
		SessionID: uuid.New(),
		TenantID:  "test",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build model for provider")
	assert.Contains(t, err.Error(), "openrouter")
}

func TestEffectivePrompt_UsesSessionSnapshotAndRequestIdentity(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	snapshotPrompt := "Olá {{user.email}} em {{tenant.name}} {{user.unknown_field}}"
	repo := &adapterRepoStub{session: chat.ChatSession{
		ID:                   sessionID,
		AgentID:              &agentID,
		Status:               chat.StatusActive,
		SystemPromptSnapshot: &snapshotPrompt,
	}}
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		ID:           agentID,
		Status:       "PUBLISHED",
		SystemPrompt: "current prompt must not be used",
	}}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repo,
		loader,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	resp, err := adapter.EffectivePrompt(context.Background(), sessionID, chat.PromptIdentity{
		UserEmail:  "ana@example.test",
		TenantID:   "test",
		TenantName: "Test Tenant",
	})

	require.NoError(t, err)
	assert.Equal(t, sessionID, resp.SessionID)
	require.NotNil(t, resp.AgentID)
	assert.Equal(t, agentID, *resp.AgentID)
	assert.Equal(t, "Olá ana@example.test em Test Tenant ", resp.SystemPrompt)
	assert.Equal(t, []string{"unresolved_placeholder:user.unknown_field"}, resp.Warnings)
}

func TestEffectiveTools_EmptyMCPBindingSetBlocksAllMCPTools(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	repo := &adapterRepoStub{session: chat.ChatSession{
		ID:      sessionID,
		AgentID: &agentID,
		Status:  chat.StatusActive,
	}}
	loader := &mockAgentConfigLoader{cfg: &chat.AgentRunConfig{
		ID:             agentID,
		Status:         "PUBLISHED",
		MCPServerNames: []string{},
	}}
	toolBuilder := agentic.NewToolSchemaBuilder(
		&mockSkillLister{},
		newMockToolsBySkill(),
		&mockKBLister{},
	)
	mcpClient := &mockMCPClient{tools: []agentic.MCPToolInfo{{
		ServerName: "slack",
		Name:       "send_message",
	}}}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		nil,
		nil,
		nil,
		toolBuilder,
		nil,
		nil,
		nil,
		repo,
		loader,
		nil,
		nil,
		nil,
		nil,
		nil,
	).WithMCPClient(mcpClient)

	response, err := adapter.EffectiveTools(context.Background(), sessionID, chat.PromptIdentity{TenantID: "test"})

	require.NoError(t, err)
	for _, effectiveTool := range response.Tools {
		assert.NotEqual(t, "mcp__slack__send_message", effectiveTool.Name)
	}
}
