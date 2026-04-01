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

// mockExecRepo implements ExecutionRepository for unit tests.
type mockExecRepo struct {
	data      map[uuid.UUID]execution.AgentExecution
	nodes     map[uuid.UUID][]execution.AgentExecutionNode
	tools     map[uuid.UUID][]execution.ToolExecution
	transErr  error
}

func newMockRepo() *mockExecRepo {
	return &mockExecRepo{
		data:  make(map[uuid.UUID]execution.AgentExecution),
		nodes: make(map[uuid.UUID][]execution.AgentExecutionNode),
		tools: make(map[uuid.UUID][]execution.ToolExecution),
	}
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
	if e.Status == "" {
		e.Status = execution.StatusRunning
	}
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

func (m *mockExecRepo) GetDetails(_ context.Context, id uuid.UUID) (execution.ExecutionDetails, error) {
	e, ok := m.data[id]
	if !ok {
		return execution.ExecutionDetails{}, execution.ErrNotFound
	}
	nodelist := m.nodes[id]
	nodeDetails := make([]execution.NodeDetails, len(nodelist))
	for i, n := range nodelist {
		nodeDetails[i] = execution.NodeDetails{
			AgentExecutionNode: n,
			Tools:              m.tools[n.ID],
		}
	}
	return execution.ExecutionDetails{AgentExecution: e, Nodes: nodeDetails}, nil
}

func (m *mockExecRepo) Transition(_ context.Context, id uuid.UUID, from, to string, output []byte, errMsg *string) error {
	if m.transErr != nil {
		return m.transErr
	}
	e, ok := m.data[id]
	if !ok {
		return execution.ErrNotFound
	}
	if e.Status != from {
		return execution.ErrNotFound
	}
	e.Status = to
	if output != nil {
		e.Output = output
	}
	if errMsg != nil {
		e.ErrorMessage = errMsg
	}
	m.data[id] = e
	return nil
}

func (m *mockExecRepo) Cancel(_ context.Context, id uuid.UUID) error {
	e, ok := m.data[id]
	if !ok {
		return execution.ErrNotFound
	}
	e.Status = execution.StatusCancelled
	m.data[id] = e
	return nil
}

func (m *mockExecRepo) ListNodes(_ context.Context, execID uuid.UUID) ([]execution.AgentExecutionNode, error) {
	return m.nodes[execID], nil
}

func (m *mockExecRepo) ListToolExecutions(_ context.Context, nodeID uuid.UUID) ([]execution.ToolExecution, error) {
	return m.tools[nodeID], nil
}

// helpers

func startExecution(t *testing.T, svc *execution.Service, agentID uuid.UUID) execution.AgentExecution {
	t.Helper()
	e, err := svc.Start(context.Background(), execution.StartExecutionRequest{AgentID: agentID.String()})
	require.NoError(t, err)
	return e
}

// ------ tests ------

func TestExecutionService_Start_Success(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	agentID := uuid.New()
	e, err := svc.Start(context.Background(), execution.StartExecutionRequest{AgentID: agentID.String()})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.Equal(t, execution.StatusRunning, e.Status)
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
	for i := 0; i < 2; i++ {
		_, err := svc.Start(context.Background(), execution.StartExecutionRequest{AgentID: agentID.String()})
		require.NoError(t, err)
	}
	// execution from different agent — should not appear
	_, err := svc.Start(context.Background(), execution.StartExecutionRequest{AgentID: uuid.New().String()})
	require.NoError(t, err)

	page, err := svc.List(context.Background(), &agentID, nil, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.TotalElements)
}

// state machine tests

func TestExecutionService_Complete_ValidTransition(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	e := startExecution(t, svc, uuid.New())

	err := svc.Complete(context.Background(), e.ID, []byte(`{"result":"ok"}`))
	require.NoError(t, err)
}

func TestExecutionService_Fail_ValidTransition(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	e := startExecution(t, svc, uuid.New())

	err := svc.Fail(context.Background(), e.ID, "timeout exceeded")
	require.NoError(t, err)
}

func TestExecutionService_Complete_AlreadyCompleted_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	svc := execution.NewService(repo)
	e := startExecution(t, svc, uuid.New())

	// Complete once — valid
	require.NoError(t, svc.Complete(context.Background(), e.ID, nil))

	// Try to Complete again — COMPLETED→COMPLETED is invalid
	err := svc.Complete(context.Background(), e.ID, nil)
	require.ErrorIs(t, err, execution.ErrInvalidTransition)
}

func TestExecutionService_Fail_AlreadyCancelled_InvalidTransition(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	e := startExecution(t, svc, uuid.New())

	require.NoError(t, svc.Cancel(context.Background(), e.ID))

	err := svc.Fail(context.Background(), e.ID, "error")
	require.ErrorIs(t, err, execution.ErrInvalidTransition)
}

func TestExecutionService_CanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{execution.StatusRunning, execution.StatusCompleted, true},
		{execution.StatusRunning, execution.StatusFailed, true},
		{execution.StatusRunning, execution.StatusCancelled, true},
		{execution.StatusPending, execution.StatusRunning, true},
		{execution.StatusCompleted, execution.StatusRunning, false},
		{execution.StatusFailed, execution.StatusCompleted, false},
		{execution.StatusCancelled, execution.StatusRunning, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.ok, execution.CanTransition(tc.from, tc.to),
			"%s → %s", tc.from, tc.to)
	}
}

// GetDetails hierarchy test

func TestExecutionService_GetDetails_ReturnsHierarchy(t *testing.T) {
	repo := newMockRepo()
	svc := execution.NewService(repo)

	agentID := uuid.New()
	e := startExecution(t, svc, agentID)

	// Seed nodes and tools directly in mock repo
	nodeID := uuid.New()
	toolID := uuid.New()
	repo.nodes[e.ID] = []execution.AgentExecutionNode{
		{ID: nodeID, ExecutionID: e.ID, NodeType: "LLM", Status: execution.StatusCompleted},
	}
	repo.tools[nodeID] = []execution.ToolExecution{
		{ID: toolID, ToolID: uuid.New(), Status: execution.StatusCompleted},
	}

	details, err := svc.GetDetails(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.ID, details.ID)
	require.Len(t, details.Nodes, 1)
	assert.Equal(t, nodeID, details.Nodes[0].ID)
	require.Len(t, details.Nodes[0].Tools, 1)
	assert.Equal(t, toolID, details.Nodes[0].Tools[0].ID)
}

func TestExecutionService_GetDetails_NotFound(t *testing.T) {
	svc := execution.NewService(newMockRepo())
	_, err := svc.GetDetails(context.Background(), uuid.New())
	require.ErrorIs(t, err, execution.ErrNotFound)
}
