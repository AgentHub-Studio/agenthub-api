package datasource_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockDSRepo struct {
	data map[uuid.UUID]datasource.DataSource
}

func newMockRepo() *mockDSRepo {
	return &mockDSRepo{data: make(map[uuid.UUID]datasource.DataSource)}
}

func (m *mockDSRepo) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]datasource.DataSource, int, error) {
	out := make([]datasource.DataSource, 0, len(m.data))
	for _, d := range m.data {
		out = append(out, d)
	}
	return out, len(out), nil
}

func (m *mockDSRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (datasource.DataSource, error) {
	d, ok := m.data[id]
	if !ok {
		return datasource.DataSource{}, datasource.ErrNotFound
	}
	return d, nil
}

func (m *mockDSRepo) Create(_ context.Context, _ string, d datasource.DataSource) (datasource.DataSource, error) {
	d.ID = uuid.New()
	m.data[d.ID] = d
	return d, nil
}

func (m *mockDSRepo) Update(_ context.Context, _ string, id uuid.UUID, d datasource.DataSource) (datasource.DataSource, error) {
	if _, ok := m.data[id]; !ok {
		return datasource.DataSource{}, datasource.ErrNotFound
	}
	d.ID = id
	m.data[id] = d
	return d, nil
}

func (m *mockDSRepo) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return datasource.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

const tenantID = "test-tenant"

func TestDataSourceService_Create_Success(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	ds, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name:     "Prod DB",
		Type:     datasource.TypePostgreSQL,
		Host:     "pg.internal",
		Port:     5432,
		Database: "appdb",
		DbUser:   "appuser",
	})
	require.NoError(t, err)
	assert.Equal(t, "Prod DB", ds.Name)
	assert.NotEqual(t, uuid.Nil, ds.ID)
}

func TestDataSourceService_GetByID_NotFound(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, datasource.ErrNotFound)
}

func TestDataSourceService_Delete_Success(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "DB", Type: datasource.TypeMySQL, Host: "mysql", Port: 3306, Database: "db", DbUser: "root",
	})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
}

func TestDataSourceService_ListAll(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	for i := 0; i < 2; i++ {
		_, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
			Name: "DB", Type: datasource.TypePostgreSQL, Host: "pg", Port: 5432, Database: "db", DbUser: "u",
		})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, items, 2)
}
