package agent_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockAgentRepo struct {
	data map[uuid.UUID]agent.Agent
}

func newMockRepo() *mockAgentRepo {
	return &mockAgentRepo{data: make(map[uuid.UUID]agent.Agent)}
}

func (m *mockAgentRepo) FindAll(_ context.Context, status agent.AgentStatus, req pagination.PageRequest) ([]agent.Agent, int64, error) {
	var out []agent.Agent
	for _, a := range m.data {
		if status == "" || a.Status == status {
			out = append(out, a)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockAgentRepo) FindByID(_ context.Context, id uuid.UUID) (agent.Agent, error) {
	a, ok := m.data[id]
	if !ok {
		return agent.Agent{}, agent.ErrNotFound
	}
	return a, nil
}

func (m *mockAgentRepo) Create(_ context.Context, a agent.Agent) (agent.Agent, error) {
	m.data[a.ID] = a
	return a, nil
}

func (m *mockAgentRepo) Update(_ context.Context, a agent.Agent) (agent.Agent, error) {
	if _, ok := m.data[a.ID]; !ok {
		return agent.Agent{}, agent.ErrNotFound
	}
	m.data[a.ID] = a
	return a, nil
}

func (m *mockAgentRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return agent.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockAgentRepo) UpdateStatus(_ context.Context, id uuid.UUID, status agent.AgentStatus) (agent.Agent, error) {
	a, ok := m.data[id]
	if !ok {
		return agent.Agent{}, agent.ErrNotFound
	}
	a.Status = status
	m.data[id] = a
	return a, nil
}

func TestAgentService_Create_Success(t *testing.T) {
	svc := agent.NewService(newMockRepo())
	resp, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "My Agent"})
	require.NoError(t, err)
	assert.Equal(t, "My Agent", resp.Name)
	assert.Equal(t, "DRAFT", resp.Status)
	assert.NotEqual(t, uuid.Nil, resp.ID)
}

func TestAgentService_Create_MissingName(t *testing.T) {
	svc := agent.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), agent.CreateAgentRequest{})
	require.Error(t, err)
}

func TestAgentService_Create_AutoSlug(t *testing.T) {
	svc := agent.NewService(newMockRepo())
	resp, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "My Test Agent"})
	require.NoError(t, err)
	assert.Equal(t, "my-test-agent", resp.Slug)
}

func TestAgentService_Get_NotFound(t *testing.T) {
	svc := agent.NewService(newMockRepo())
	_, err := svc.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, agent.ErrNotFound)
}

func TestAgentService_Update_Success(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo)
	created, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "Old Name"})
	require.NoError(t, err)

	newName := "New Name"
	updated, err := svc.Update(context.Background(), created.ID, agent.UpdateAgentRequest{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
}

func TestAgentService_Delete_Success(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo)
	created, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "Agent"})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)
}

func TestAgentService_Delete_NotFound(t *testing.T) {
	svc := agent.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, agent.ErrNotFound)
}

func TestAgentService_Publish_Success(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo)
	created, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "Agent"})
	require.NoError(t, err)
	published, err := svc.Publish(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "PUBLISHED", published.Status)
}

func TestAgentService_Archive_Success(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo)
	created, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "Agent"})
	require.NoError(t, err)
	archived, err := svc.Archive(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "ARCHIVED", archived.Status)
}

func TestAgentService_Clone_Success(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo)
	created, err := svc.Create(context.Background(), agent.CreateAgentRequest{Name: "Original"})
	require.NoError(t, err)
	cloned, err := svc.Clone(context.Background(), created.ID, agent.CloneAgentRequest{Name: "Clone"})
	require.NoError(t, err)
	assert.Equal(t, "Clone", cloned.Name)
	assert.NotEqual(t, created.ID, cloned.ID)
}

func TestAgentService_List(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo)
	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), agent.CreateAgentRequest{
			Name: "Agent " + string(rune('A'+i)),
		})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), "", pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}
