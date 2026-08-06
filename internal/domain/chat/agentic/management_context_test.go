package agentic_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestManagementContext_validateDestructive_Match(t *testing.T) {
	id := uuid.New()
	mc := agentic.ManagementContext{SessionAgentID: id, RunAgentID: id}
	require.NoError(t, mc.ValidateDestructive())
}

func TestManagementContext_validateDestructive_Mismatch(t *testing.T) {
	mc := agentic.ManagementContext{
		SessionAgentID: uuid.New(),
		RunAgentID:     uuid.New(),
	}
	err := mc.ValidateDestructive()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestManagementContext_validateDestructive_MissingAgent(t *testing.T) {
	err := agentic.ManagementContext{}.ValidateDestructive()
	require.Error(t, err)
}
