package execution_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockExecRepo struct {
	data map[uuid.UUID]execution.AgentExecution
}

func newMockRepo() *mockExecRepo {
	return &mockExecRepo{data: make(map[uuid.UUID]execution.AgentExecution)}
}

func (m *mockExecRepo) List(_ context.Context, agentID *uuid.UUID, status *string, _ pagination.PageRequest) ([]execution.AgentExecution, int64, error) {
	var out []execution.AgentExecution
	for _, e := range m.data {
		if agentID != nil && e.AgentID != *agentID {
			continue
		}
		if status != nil && e.Status != *status {
			continue
		}
		out = append(out, e)
	}
	return out, int64(len(out)), nil
}

func (m *mockExecRepo) Create(_ context.Context, e execution.AgentExecution) (execution.AgentExecution, error) {
	e.ID = uuid.New()
	e.Status = "RUNNING"
	m.data[e.ID] = e
	return e, nil
}

func (m *mockExecRepo) GetByID(_ context.Context, id uuid.UUID) (execution.AgentExecution, error) {
	e, ok := m.data[id]
	if !ok {
		return execution.AgentExecution{}, execution.ErrNotFound
	}
	return e, nil
}

func (m *mockExecRepo) Cancel(_ context.Context, id uuid.UUID) error {
	e, ok := m.data[id]
	if !ok {
		return execution.ErrNotFound
	}
	e.Status = "CANCELLED"
	m.data[id] = e
	return nil
}

func (m *mockExecRepo) ListNodes(_ context.Context, _ uuid.UUID) ([]execution.AgentExecutionNode, error) {
	return nil, nil
}

func (m *mockExecRepo) ListToolExecutions(_ context.Context, _ uuid.UUID) ([]execution.ToolExecution, error) {
	return nil, nil
}

func TestExecutionService_Start_Success(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	agentID := uuid.New()
	agentStr := agentID.String()
	e, err := svc.Start(context.Background(), execution.StartExecutionRequest{
		AgentID: agentStr,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.Equal(t, "RUNNING", e.Status)
}

func TestExecutionService_GetByID_NotFound(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, execution.ErrNotFound)
}

func TestExecutionService_Cancel_NotFound(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	err := svc.Cancel(context.Background(), uuid.New())
	require.ErrorIs(t, err, execution.ErrNotFound)
}

func TestExecutionService_List_FilterByAgent(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	agentID := uuid.New()
	agentStr := agentID.String()
	for i := 0; i < 2; i++ {
		_, err := svc.Start(context.Background(), execution.StartExecutionRequest{AgentID: agentStr})
		require.NoError(t, err)
	}
	// execution from different agent — should not appear
	_, err := svc.Start(context.Background(), execution.StartExecutionRequest{AgentID: uuid.New().String()})
	require.NoError(t, err)

	page, err := svc.List(context.Background(), &agentID, nil, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.TotalElements)
}
