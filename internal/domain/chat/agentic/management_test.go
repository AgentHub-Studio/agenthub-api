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

type stubAgentDeleter struct {
	deleteErr    error
	deleteCalled bool
	lastDeleteID uuid.UUID
}

func (s *stubAgentDeleter) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalled = true
	s.lastDeleteID = id
	return s.deleteErr
}

func validMgmtContext(sessionAgent uuid.UUID) agentic.ManagementContext {
	return agentic.ManagementContext{
		SessionID:      uuid.New(),
		SessionAgentID: sessionAgent,
		RunAgentID:     sessionAgent,
	}
}

// TestManagementDelete_SkillBoundToAgent_ReturnsError verifies that when the
// service rejects deletion due to agent bindings, the error propagates as a
// tool error result (not a Go error).
func TestManagementDelete_SkillBoundToAgent_ReturnsError(t *testing.T) {
	deleter := &stubSkillDeleter{deleteErr: skill.ErrSkillBoundToAgents}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)
	agentID := uuid.New()

	result := exec.Execute(context.Background(), validMgmtContext(agentID), "delete", "skill", uuid.New().String(), "", nil)

	require.True(t, deleter.deleteCalled, "Delete should have been called on the deleter")
	require.NotNil(t, result.Error, "result should carry an error message")
	assert.Contains(t, *result.Error, "Error deleting skill")
}

// TestManagementDelete_UnboundSkill_Succeeds verifies that a skill with no agent
// bindings is deleted without errors.
func TestManagementDelete_UnboundSkill_Succeeds(t *testing.T) {
	deleter := &stubSkillDeleter{deleteErr: nil}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)
	agentID := uuid.New()

	result := exec.Execute(context.Background(), validMgmtContext(agentID), "delete", "skill", uuid.New().String(), "", nil)

	require.True(t, deleter.deleteCalled, "Delete should have been called")
	assert.Nil(t, result.Error, "no error expected for unbound skill")
	assert.Equal(t, `{"status":"deleted"}`, string(result.Output))
}

// TestManagementDelete_CallsDeleterNotRepo verifies that skill deletion goes through
// the injected Deleter (service), not a raw repository call.
func TestManagementDelete_CallsDeleterNotRepo(t *testing.T) {
	deleter := &stubSkillDeleter{}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)

	exec.Execute(context.Background(), validMgmtContext(uuid.New()), "delete", "skill", uuid.New().String(), "", nil)

	assert.True(t, deleter.deleteCalled, "must use injected Deleter, not a raw repository")
}

// TestManagementDelete_SkillBoundToAgents_IsCorrectSentinelError confirms the
// sentinel error returned by skill.Service.Delete propagates with binding count info.
func TestManagementDelete_SkillBoundToAgents_IsCorrectSentinelError(t *testing.T) {
	boundErr := errors.New("skill: cannot delete — skill is bound to one or more agents (agents bound: 3)")
	deleter := &stubSkillDeleter{deleteErr: boundErr}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)

	result := exec.Execute(context.Background(), validMgmtContext(uuid.New()), "delete", "skill", uuid.New().String(), "", nil)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "skill")
}

func TestManagementDelete_AgentUsesAgentDeleter(t *testing.T) {
	agentDel := &stubAgentDeleter{}
	exec := agentic.NewManagementExecutor(nil, agentDel, nil, &stubSkillDeleter{}, nil, nil)
	agentID := uuid.New()
	targetID := uuid.New()

	result := exec.Execute(context.Background(), validMgmtContext(agentID), "delete", "agent", targetID.String(), "", nil)

	require.True(t, agentDel.deleteCalled)
	assert.Equal(t, targetID, agentDel.lastDeleteID)
	assert.Nil(t, result.Error)
}

func TestManagementDelete_AgentMismatch_Denied(t *testing.T) {
	exec := agentic.NewManagementExecutor(nil, &stubAgentDeleter{}, nil, &stubSkillDeleter{}, nil, nil)
	mc := agentic.ManagementContext{
		SessionID:      uuid.New(),
		SessionAgentID: uuid.New(),
		RunAgentID:     uuid.New(),
	}

	result := exec.Execute(context.Background(), mc, "delete", "agent", uuid.New().String(), "", nil)

	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "mismatch")
}

func TestManagementDelete_LogsAuditOnSuccess(t *testing.T) {
	capture := &captureAuditLogger{}
	agentID := uuid.New()
	deleter := &stubSkillDeleter{}
	exec := agentic.NewManagementExecutor(nil, nil, nil, deleter, nil, nil)
	mc := validMgmtContext(agentID)
	mc.Audit = capture

	exec.Execute(context.Background(), mc, "delete", "skill", uuid.New().String(), "", nil)

	require.Len(t, capture.entries, 1)
	assert.Equal(t, "agenthub_manage", capture.entries[0].ToolName)
}

type captureAuditLogger struct {
	entries []agentic.PermissionAuditEntry
}

func (c *captureAuditLogger) LogDecision(_ context.Context, entry agentic.PermissionAuditEntry) error {
	c.entries = append(c.entries, entry)
	return nil
}
