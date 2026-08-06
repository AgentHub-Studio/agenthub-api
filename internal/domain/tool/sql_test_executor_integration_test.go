//go:build integration

package tool_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sqlToolTestIntegrationTenant = "sqltooltest"

type postgresSQLTestDatasourceAdapter struct {
	svc *datasource.Service
}

func (a *postgresSQLTestDatasourceAdapter) GetDatasourceCreds(ctx context.Context, tenantID string, id uuid.UUID) (tool.DatasourceCreds, error) {
	creds, err := a.svc.GetCredentials(ctx, tenantID, id)
	if err != nil {
		return tool.DatasourceCreds{}, err
	}
	return tool.DatasourceCreds{
		Type:     string(creds.Type),
		Host:     creds.Host,
		Port:     creds.Port,
		Database: creds.Database,
		User:     creds.User,
		Password: creds.Password,
	}, nil
}

func TestIntegration_ToolServiceTestDatabaseExecutesParameterizedPostgresQuery(t *testing.T) {
	if os.Getenv("AGENTHUB_TEST_POSTGRES_DSN") == "" {
		t.Skip("requires ./build.sh test-integration shared PostgreSQL fixture")
	}

	pool := testutil.NewPostgresContainer(t)
	ctx := tenant.NewContext(context.Background(), sqlToolTestIntegrationTenant)
	setupSQLToolTestIntegrationSchema(t, pool)
	testutil.MustExec(t, pool, `
		CREATE TABLE query_probe (id INTEGER PRIMARY KEY, email TEXT NOT NULL, api_key TEXT NOT NULL);
		INSERT INTO query_probe (id, email, api_key) VALUES (7, 'ada@example.test', 'database-secret');
	`)

	datasourceSvc := datasource.NewService(datasource.NewRepository(pool))
	datasourceHost := sqlToolTestIntegrationHost(t)
	dataSource, err := datasourceSvc.Create(ctx, sqlToolTestIntegrationTenant, datasource.CreateRequest{
		Name:       "Integration PostgreSQL",
		Type:       datasource.DataSourceTypePostgreSQL,
		Host:       datasourceHost,
		Port:       5432,
		Database:   pool.Config().ConnConfig.Database,
		DBUser:     "testuser",
		DBPassword: "testpass",
	})
	require.NoError(t, err)

	service := tool.NewService(tool.NewRepository(pool)).WithDatasource(
		&postgresSQLTestDatasourceAdapter{svc: datasourceSvc},
		tenant.FromContext,
	)
	created, err := service.Create(ctx, tool.CreateRequest{
		Name: "Lookup database probe",
		Type: tool.ToolTypeDatabase,
		Config: json.RawMessage(`{
			"datasource_id": "` + dataSource.ID.String() + `",
			"query": "SELECT id, email, api_key FROM query_probe WHERE id = {{input.id}}",
			"operation": "SELECT",
			"max_rows": 1
		}`),
	})
	require.NoError(t, err)

	result, err := service.TestTool(ctx, created.ID, map[string]any{"id": 7})
	require.NoError(t, err)
	assert.Contains(t, result, "ada@example.test")
	assert.Contains(t, result, `"api_key":"***"`)
	assert.NotContains(t, result, "database-secret")
}

func setupSQLToolTestIntegrationSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE SCHEMA IF NOT EXISTS ah_sqltooltest;
		CREATE TABLE IF NOT EXISTS ah_sqltooltest.data_source (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			host VARCHAR(255) NOT NULL,
			port INTEGER NOT NULL,
			database VARCHAR(255) NOT NULL,
			db_user VARCHAR(255) NOT NULL,
			db_password TEXT NOT NULL DEFAULT '',
			vpn_resource_id UUID NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS ah_sqltooltest.tool (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			config JSONB NOT NULL DEFAULT '{}',
			input_schema JSONB,
			description TEXT NOT NULL DEFAULT '',
			labels TEXT[] NOT NULL DEFAULT '{}',
			read_only BOOLEAN NOT NULL DEFAULT FALSE,
			should_defer BOOLEAN NOT NULL DEFAULT FALSE,
			is_destructive BOOLEAN NOT NULL DEFAULT FALSE,
			search_hint TEXT,
			always_load BOOLEAN NOT NULL DEFAULT FALSE,
			concurrency_safe BOOLEAN,
			max_result_chars INTEGER,
			interrupt_behavior TEXT,
			is_search_or_read BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	require.NoError(t, err)
}

func sqlToolTestIntegrationHost(t *testing.T) string {
	t.Helper()
	interfaces, err := net.Interfaces()
	require.NoError(t, err)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		require.NoError(t, err)
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return ip.String()
			}
		}
	}
	t.Fatal("no non-loopback IPv4 address available for PostgreSQL integration test")
	return ""
}
