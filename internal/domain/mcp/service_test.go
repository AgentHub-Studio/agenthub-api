package mcp_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
)

type mockMCPRepo struct {
	data map[uuid.UUID]mcp.McpServerConfig
}

func newMockRepo() *mockMCPRepo {
	return &mockMCPRepo{data: make(map[uuid.UUID]mcp.McpServerConfig)}
}

func (m *mockMCPRepo) List(_ context.Context) ([]mcp.McpServerConfig, error) {
	out := make([]mcp.McpServerConfig, 0, len(m.data))
	for _, c := range m.data {
		out = append(out, c)
	}
	return out, nil
}

func (m *mockMCPRepo) GetByID(_ context.Context, id uuid.UUID) (mcp.McpServerConfig, error) {
	c, ok := m.data[id]
	if !ok {
		return mcp.McpServerConfig{}, mcp.ErrNotFound
	}
	return c, nil
}

func (m *mockMCPRepo) GetByName(_ context.Context, name string) (mcp.McpServerConfig, error) {
	for _, c := range m.data {
		if c.Name == name {
			return c, nil
		}
	}
	return mcp.McpServerConfig{}, mcp.ErrNotFound
}

func (m *mockMCPRepo) Create(_ context.Context, c mcp.McpServerConfig) (mcp.McpServerConfig, error) {
	c.ID = uuid.New()
	m.data[c.ID] = c
	return c, nil
}

func (m *mockMCPRepo) Update(_ context.Context, c mcp.McpServerConfig) (mcp.McpServerConfig, error) {
	if _, ok := m.data[c.ID]; !ok {
		return mcp.McpServerConfig{}, mcp.ErrNotFound
	}
	m.data[c.ID] = c
	return c, nil
}

func (m *mockMCPRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return mcp.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockMCPRepo) ListAutoStart(_ context.Context) ([]mcp.McpServerConfig, error) {
	var out []mcp.McpServerConfig
	for _, c := range m.data {
		if c.AutoStart {
			out = append(out, c)
		}
	}
	return out, nil
}

func TestMCPService_Create_Success(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	c, err := svc.Create(context.Background(), mcp.CreateRequest{
		Name:          "filesystem",
		TransportType: "http",
	})
	require.NoError(t, err)
	assert.Equal(t, "filesystem", c.Name)
	assert.NotEqual(t, uuid.Nil, c.ID)
}

func TestMCPService_GetByID_NotFound(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

func TestMCPService_Delete_NotFound(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

func TestMCPService_ListAutoStart(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	autoStart := true
	_, err := svc.Create(context.Background(), mcp.CreateRequest{Name: "auto", TransportType: "http", AutoStart: autoStart})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), mcp.CreateRequest{Name: "manual", TransportType: "http"})
	require.NoError(t, err)
	items, err := svc.ListAutoStart(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMCPService_List(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	for _, name := range []string{"fs", "github", "slack"} {
		_, err := svc.Create(context.Background(), mcp.CreateRequest{Name: name, TransportType: "http"})
		require.NoError(t, err)
	}
	items, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 3)
}
