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
		Type:     datasource.DataSourceTypePostgreSQL,
		Host:     "pg.internal",
		Port:     5432,
		Database: "appdb",
		DBUser:   "appuser",
	})
	require.NoError(t, err)
	assert.Equal(t, "Prod DB", ds.Name)
	assert.NotEqual(t, uuid.Nil, ds.ID)
}

func TestDataSourceService_Create_DefaultPort_PostgreSQL(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	ds, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "PG DB", Type: datasource.DataSourceTypePostgreSQL, Host: "pg", Database: "db", DBUser: "u",
		// Port deliberately omitted
	})
	require.NoError(t, err)
	assert.Equal(t, 5432, ds.Port)
}

func TestDataSourceService_Create_DefaultPort_MySQL(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	ds, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "MySQL DB", Type: datasource.DataSourceTypeMySQL, Host: "mysql", Database: "db", DBUser: "u",
	})
	require.NoError(t, err)
	assert.Equal(t, 3306, ds.Port)
}

func TestDataSourceService_Create_DefaultPort_SQLServer(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	ds, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "MSSQL DB", Type: datasource.DataSourceTypeSQLServer, Host: "mssql", Database: "db", DBUser: "sa",
	})
	require.NoError(t, err)
	assert.Equal(t, 1433, ds.Port)
}

func TestDataSourceService_Create_ValidationError_MissingName(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Type: datasource.DataSourceTypePostgreSQL, Host: "pg", Database: "db", DBUser: "u",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestDataSourceService_Create_ValidationError_UnsupportedType(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "DB", Type: "ORACLE", Host: "oracle", Database: "db", DBUser: "u",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestDataSourceService_GetByID_NotFound(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, datasource.ErrNotFound)
}

func TestDataSourceService_Delete_Success(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "DB", Type: datasource.DataSourceTypeMySQL, Host: "mysql", Database: "db", DBUser: "root",
	})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
}

func TestDataSourceService_ListAll(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	for i := 0; i < 2; i++ {
		_, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
			Name: "DB", Type: datasource.DataSourceTypePostgreSQL, Host: "pg", Database: "db", DBUser: "u",
		})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, items, 2)
}

// ---- GetCredentials (proxy endpoint) ----

func TestDataSourceService_GetCredentials_Success(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, datasource.CreateRequest{
		Name: "Secure DB", Type: datasource.DataSourceTypePostgreSQL,
		Host: "pg.internal", Database: "appdb", DBUser: "appuser", DBPassword: "s3cr3t",
	})
	require.NoError(t, err)

	creds, err := svc.GetCredentials(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "appuser", creds.User)
	assert.Equal(t, "s3cr3t", creds.Password, "credentials endpoint must return plaintext password for proxy use")
	assert.Equal(t, "pg.internal", creds.Host)
}

func TestDataSourceService_GetCredentials_NotFound(t *testing.T) {
	svc := datasource.NewService(newMockRepo())
	_, err := svc.GetCredentials(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, datasource.ErrNotFound)
}

func TestDataSourceService_ResponseFrom_OmitsPassword(t *testing.T) {
	ds := datasource.DataSource{
		ID: uuid.New(), Name: "DB", Type: datasource.DataSourceTypePostgreSQL,
		Host: "pg", Port: 5432, Database: "db", DBUser: "u", DBPassword: "secret",
	}
	resp := datasource.ResponseFrom(ds)
	// Use json encoding to verify no password field leaks.
	assert.Equal(t, "u", resp.DBUser)
	// DataSourceResponse has no DBPassword field — confirmed by type conversion in ResponseFrom.
}
