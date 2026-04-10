package agentic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
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
		nil,  // factory — never reached because lifecycle check fires first
		nil,  // skillClient
		nil,  // prompt
		nil,  // tools
		nil,  // ctxManager
		nil,  // memory
		nil,  // hookExecutor
		nil,  // repo
		loader,
		nil,  // agentRepo
		nil,  // skillRepo
		nil,  // toolRepo
		nil,  // integRepo
		nil,  // mcpRepo
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
