//go:build integration

package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const mcpIntegrationTenant = "mcpredactiontest"

func setupMCPTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + mcpIntegrationTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	dir, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var ups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			ups = append(ups, entry.Name())
		}
	}
	sort.Strings(ups)
	require.NotEmpty(t, ups)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, name := range ups {
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sqlBytes))
		require.NoError(t, err, "apply migration %s", name)
	}

	return pool, tenant.NewContext(ctx, mcpIntegrationTenant)
}

func TestIntegration_MCPResponseRedactsSensitiveEnvValues(t *testing.T) {
	pool, ctx := setupMCPTenantSchema(t)
	repo := mcp.NewRepository(pool)
	svc := mcp.NewService(repo)
	httpBaseURL := "https://mcp.example.com"

	created, err := svc.Create(ctx, mcp.CreateRequest{
		Name:          "github",
		TransportType: "http",
		HTTPBaseURL:   &httpBaseURL,
		Env: map[string]string{
			"GITHUB_TOKEN": "ghp-mcp-env-token",
			"API_KEY":      "mcp-api-key-secret",
			"MCP_SECRET":   "mcp-secret-value",
			"DATABASE_URL": "postgres://mcp:audit-mcp-database-url-secret@db.example.test:5432/agenthub",
			"SERVICE_URL":  "https://service.example.test/v1?api_key=audit-mcp-query-url-secret",
			"SAFE_FLAG":    "enabled",
		},
		Enabled: true,
	})
	require.NoError(t, err)

	persisted, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "ghp-mcp-env-token", persisted.Env["GITHUB_TOKEN"])
	assert.Equal(t, "mcp-api-key-secret", persisted.Env["API_KEY"])
	assert.Equal(t, "mcp-secret-value", persisted.Env["MCP_SECRET"])
	assert.Equal(t, "postgres://mcp:audit-mcp-database-url-secret@db.example.test:5432/agenthub", persisted.Env["DATABASE_URL"])
	assert.Equal(t, "https://service.example.test/v1?api_key=audit-mcp-query-url-secret", persisted.Env["SERVICE_URL"])

	got, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	body, err := json.Marshal(got)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "ghp-mcp-env-token")
	assert.NotContains(t, string(body), "mcp-api-key-secret")
	assert.NotContains(t, string(body), "mcp-secret-value")
	assert.NotContains(t, string(body), "audit-mcp-database-url-secret")
	assert.NotContains(t, string(body), "audit-mcp-query-url-secret")
	assert.Contains(t, string(body), `"GITHUB_TOKEN":"***"`)
	assert.Contains(t, string(body), `"API_KEY":"***"`)
	assert.Contains(t, string(body), `"MCP_SECRET":"***"`)
	assert.Contains(t, got.Env["DATABASE_URL"], "mcp:%2A%2A%2A@")
	assert.Contains(t, got.Env["SERVICE_URL"], "api_key=%2A%2A%2A")
	assert.Contains(t, string(body), `"SAFE_FLAG":"enabled"`)
}
