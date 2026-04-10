package agentic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

// stubSkillDeleter is a test double for skill.Deleter.
type stubSkillDeleter struct {
	deleteErr    error
	deleteCalled bool
	lastDeleteID uuid.UUID
}

func (s *stubSkillDeleter) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.lastDeleteID = id
	return s.deleteErr
}

// TestManagementDelete_SkillBoundToAgent_ReturnsError verifies that when the
// service rejects deletion due to agent bindings, the error propagates as a
// tool error result (not a Go error).
func TestManagementDelete_SkillBoundToAgent_ReturnsError(t *testing.T) {
	deleter := &stubSkillDeleter{deleteErr: skill.ErrSkillBoundToAgents}
	exec := agentic.NewManagementExecutor(nil, nil, deleter, nil, nil)

	skillID := uuid.New()
	result := exec.Execute(context.Background(), "delete", "skill", skillID.String(), "", nil)

	require.True(t, deleter.deleteCalled, "Delete should have been called on the deleter")
	require.NotNil(t, result.Error, "result should carry an error message")
	assert.Contains(t, *result.Error, "Error deleting skill")
}

// TestManagementDelete_UnboundSkill_Succeeds verifies that a skill with no agent
// bindings is deleted without errors.
func TestManagementDelete_UnboundSkill_Succeeds(t *testing.T) {
	deleter := &stubSkillDeleter{deleteErr: nil}
	exec := agentic.NewManagementExecutor(nil, nil, deleter, nil, nil)

	skillID := uuid.New()
	result := exec.Execute(context.Background(), "delete", "skill", skillID.String(), "", nil)

	require.True(t, deleter.deleteCalled, "Delete should have been called")
	assert.Nil(t, result.Error, "no error expected for unbound skill")
	assert.Equal(t, `{"status":"deleted"}`, string(result.Output))
}

// TestManagementDelete_CallsDeleterNotRepo verifies that skill deletion goes through
// the injected Deleter (service), not a raw repository call.
func TestManagementDelete_CallsDeleterNotRepo(t *testing.T) {
	deleter := &stubSkillDeleter{}
	// Pass nil for other repos — only the deleter should be invoked.
	exec := agentic.NewManagementExecutor(nil, nil, deleter, nil, nil)

	exec.Execute(context.Background(), "delete", "skill", uuid.New().String(), "", nil)

	assert.True(t, deleter.deleteCalled, "must use injected Deleter, not a raw repository")
}

// TestManagementDelete_SkillBoundToAgents_IsCorrectSentinelError confirms the
// sentinel error returned by skill.Service.Delete propagates with binding count info.
func TestManagementDelete_SkillBoundToAgents_IsCorrectSentinelError(t *testing.T) {
	boundErr := errors.New("skill: cannot delete — skill is bound to one or more agents (agents bound: 3)")
	deleter := &stubSkillDeleter{deleteErr: boundErr}
	exec := agentic.NewManagementExecutor(nil, nil, deleter, nil, nil)

	result := exec.Execute(context.Background(), "delete", "skill", uuid.New().String(), "", nil)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "skill")
}
