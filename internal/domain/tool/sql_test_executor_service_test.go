package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

type sqlTestDatasourceReader struct {
	credentials tool.DatasourceCreds
	tenantID    string
	datasource  uuid.UUID
	calls       int
}

func (r *sqlTestDatasourceReader) GetDatasourceCreds(_ context.Context, tenantID string, datasourceID uuid.UUID) (tool.DatasourceCreds, error) {
	r.calls++
	r.tenantID = tenantID
	r.datasource = datasourceID
	return r.credentials, nil
}

type capturedSQLTestExecutor struct {
	query     string
	inputs    map[string]any
	operation string
	maxRows   int
	calls     int
	result    string
}

func (e *capturedSQLTestExecutor) Execute(_ context.Context, _ tool.DatasourceCreds, query string, inputs map[string]any, operation string, maxRows int) (string, error) {
	e.calls++
	e.query = query
	e.inputs = inputs
	e.operation = operation
	e.maxRows = maxRows
	return e.result, nil
}

func TestToolService_TestSQLTool_UsesTenantDatasourceAndRedactsResult(t *testing.T) {
	repo := newMockRepo()
	toolID := uuid.New()
	datasourceID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Type: tool.ToolTypeDatabase,
		Config: json.RawMessage(`{
			"datasource_id": "` + datasourceID.String() + `",
			"query": "SELECT customer_email FROM orders WHERE id = $1",
			"operation": "SELECT",
			"max_rows": 25
		}`),
	}
	reader := &sqlTestDatasourceReader{credentials: tool.DatasourceCreds{Type: "POSTGRESQL", Host: "db.example.test", Port: 5432, Database: "orders", User: "readonly", Password: "password"}}
	executor := &capturedSQLTestExecutor{result: `{"rows":[{"secret":"must-not-leak","customer_email":"safe@example.test"}]}`}
	service := tool.NewService(repo).
		WithDatasource(reader, func(context.Context) string { return "tenant-a" }).
		WithSQLTestExecutor(executor)

	output, err := service.TestTool(context.Background(), toolID, map[string]any{"1": 42})

	require.NoError(t, err)
	assert.Equal(t, "tenant-a", reader.tenantID)
	assert.Equal(t, datasourceID, reader.datasource)
	assert.Equal(t, 1, reader.calls)
	assert.Equal(t, "SELECT customer_email FROM orders WHERE id = $1", executor.query)
	assert.Equal(t, map[string]any{"1": 42}, executor.inputs)
	assert.Equal(t, "SELECT", executor.operation)
	assert.Equal(t, 25, executor.maxRows)
	assert.Equal(t, 1, executor.calls)
	assert.Contains(t, output, `"secret":"***"`)
	assert.NotContains(t, output, "must-not-leak")
}

func TestToolService_TestSQLTool_RejectsConflictingDatasourceAliasesBeforeLookup(t *testing.T) {
	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Type: tool.ToolTypeSQL,
		Config: json.RawMessage(`{
			"datasource_id": "` + uuid.NewString() + `",
			"dataSourceId": "` + uuid.NewString() + `",
			"query": "SELECT 1"
		}`),
	}
	reader := &sqlTestDatasourceReader{credentials: tool.DatasourceCreds{Type: "POSTGRESQL"}}
	executor := &capturedSQLTestExecutor{result: `{}`}
	service := tool.NewService(repo).
		WithDatasource(reader, func(context.Context) string { return "tenant-a" }).
		WithSQLTestExecutor(executor)

	_, err := service.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting SQL config aliases")
	assert.Zero(t, reader.calls)
	assert.Zero(t, executor.calls)
}

func TestToolService_TestSQLTool_RequiresDatasourceWiring(t *testing.T) {
	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:     toolID,
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"datasource_id":"` + uuid.NewString() + `","query":"SELECT 1"}`),
	}

	_, err := tool.NewService(repo).TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SQL test feature not configured")
}
