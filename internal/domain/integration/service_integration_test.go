//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const integrationServiceTenant = "integrationservicetest"

type integrationToolDatasourceAdapter struct {
	svc *datasource.Service
}

func (a *integrationToolDatasourceAdapter) GetDatasourceCreds(ctx context.Context, tenantID string, id uuid.UUID) (tool.DatasourceCreds, error) {
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

func setupIntegrationServiceTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + integrationServiceTenant
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

	return pool, tenant.NewContext(ctx, integrationServiceTenant)
}

func TestIntegration_DatabaseUpdateRefreshesGeneratedSkillInstructions(t *testing.T) {
	pool, ctx := setupIntegrationServiceTenantSchema(t)
	dataSourceSvc := datasource.NewService(datasource.NewRepository(pool))
	skillSvc := skill.NewService(skill.NewRepository(pool))
	toolSvc := tool.NewService(tool.NewRepository(pool)).
		WithDatasource(&integrationToolDatasourceAdapter{svc: dataSourceSvc}, tenant.FromContext)
	svc := integration.NewService(toolSvc, dataSourceSvc, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(skillSvc, toolSvc, integration.NewRepository(pool))

	created, err := svc.CreateDatabase(ctx, integration.DatabaseCreateRequest{
		Name:        "Orders DB",
		Description: "Read stale orders",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg.internal",
		Port:        5432,
		Database:    "orders",
		DBUser:      "orders_user",
		DBPassword:  "secret",
		Query:       "SELECT stale FROM orders",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateDatabase(ctx, created.ID, integration.DatabaseCreateRequest{
		Name:        "Orders DB",
		Description: "Sync fresh orders",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg.internal",
		Port:        5432,
		Database:    "orders",
		DBUser:      "orders_user",
		Query:       "UPDATE orders SET synced = true",
		AllowWrite:  true,
	})
	require.NoError(t, err)
	assert.True(t, updated.AllowWrite)
	assert.Equal(t, "UPDATE orders SET synced = true", updated.Query)

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenant.FromContext(ctx))
	require.NoError(t, err)
	defer release()

	var toolConfig, instructions string
	err = conn.QueryRow(ctx,
		`SELECT t.config::text, s.instructions
		 FROM tool t
		 INNER JOIN skill_tool st ON st.tool_id = t.id
		 INNER JOIN skill s ON s.id = st.skill_id
		 WHERE t.config->>'dataSourceId' = $1 OR t.config->>'datasource_id' = $1`,
		created.ID.String(),
	).Scan(&toolConfig, &instructions)
	require.NoError(t, err)

	assert.NotContains(t, toolConfig, "SELECT stale FROM orders")
	assert.Contains(t, toolConfig, "UPDATE orders SET synced = true")
	assert.Contains(t, toolConfig, `"operation": "EXEC"`)
	assert.NotContains(t, instructions, "SELECT stale FROM orders")
	assert.Contains(t, instructions, "UPDATE orders SET synced = true")
	assert.Contains(t, instructions, "execute")
}

func TestIntegration_DatabaseUpdateClearsVPNResourceWhenOmitted(t *testing.T) {
	pool, ctx := setupIntegrationServiceTenantSchema(t)
	tenantID := tenant.FromContext(ctx)
	dataSourceSvc := datasource.NewService(datasource.NewRepository(pool))
	vpnSvc := vpnresource.NewService(vpnresource.NewRepository(pool))
	skillSvc := skill.NewService(skill.NewRepository(pool))
	toolSvc := tool.NewService(tool.NewRepository(pool)).
		WithDatasource(&integrationToolDatasourceAdapter{svc: dataSourceSvc}, tenant.FromContext)
	svc := integration.NewService(toolSvc, dataSourceSvc, &stubMCPCatalog{}, vpnSvc).
		WithHTTPManagement(skillSvc, toolSvc, integration.NewRepository(pool))

	vpn, err := vpnSvc.Create(ctx, tenantID, vpnresource.CreateRequest{
		Name:        "Corp VPN",
		Description: "Optional tunnel for database integration",
		Enabled:     true,
	})
	require.NoError(t, err)

	created, err := svc.CreateDatabase(ctx, integration.DatabaseCreateRequest{
		Name:          "Orders DB VPN",
		Description:   "Read orders behind VPN",
		Type:          datasource.DataSourceTypePostgreSQL,
		Host:          "pg.internal",
		Port:          5432,
		Database:      "orders",
		DBUser:        "orders_user",
		DBPassword:    "secret",
		VpnResourceID: &vpn.ID,
		Query:         "SELECT * FROM orders",
	})
	require.NoError(t, err)
	require.NotNil(t, created.VpnResourceID)
	assert.Equal(t, vpn.ID, *created.VpnResourceID)

	updated, err := svc.UpdateDatabase(ctx, created.ID, integration.DatabaseCreateRequest{
		Name:        "Orders DB VPN",
		Description: "Read orders without VPN",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg.internal",
		Port:        5432,
		Database:    "orders",
		DBUser:      "orders_user",
		Query:       "SELECT * FROM orders WHERE active = true",
	})
	require.NoError(t, err)
	assert.Nil(t, updated.VpnResourceID)

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	defer release()

	var vpnResourceID *uuid.UUID
	err = conn.QueryRow(ctx, `SELECT vpn_resource_id FROM data_source WHERE id=$1`, created.ID).Scan(&vpnResourceID)
	require.NoError(t, err)
	assert.Nil(t, vpnResourceID)
}

func TestIntegration_DatabaseUpdateRejectsBlankQueryWithoutSideEffects(t *testing.T) {
	pool, ctx := setupIntegrationServiceTenantSchema(t)
	dataSourceSvc := datasource.NewService(datasource.NewRepository(pool))
	skillSvc := skill.NewService(skill.NewRepository(pool))
	toolSvc := tool.NewService(tool.NewRepository(pool)).
		WithDatasource(&integrationToolDatasourceAdapter{svc: dataSourceSvc}, tenant.FromContext)
	svc := integration.NewService(toolSvc, dataSourceSvc, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(skillSvc, toolSvc, integration.NewRepository(pool))

	created, err := svc.CreateDatabase(ctx, integration.DatabaseCreateRequest{
		Name:        "Orders DB",
		Description: "Read orders",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg.internal",
		Port:        5432,
		Database:    "orders",
		DBUser:      "orders_user",
		DBPassword:  "secret",
		Query:       "SELECT * FROM orders",
	})
	require.NoError(t, err)

	_, err = svc.UpdateDatabase(ctx, created.ID, integration.DatabaseCreateRequest{
		Name:        "Orders DB Broken",
		Description: "Blank query must fail",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg-broken.internal",
		Port:        5432,
		Database:    "orders_broken",
		DBUser:      "orders_broken",
		Query:       "   ",
		AllowWrite:  true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenant.FromContext(ctx))
	require.NoError(t, err)
	defer release()

	var datasourceName, datasourceHost, databaseName, dbUser, toolConfig, instructions string
	err = conn.QueryRow(ctx,
		`SELECT d.name, d.host, d.database, d.db_user, t.config::text, s.instructions
		 FROM data_source d
		 INNER JOIN tool t ON t.config->>'dataSourceId' = d.id::text OR t.config->>'datasource_id' = d.id::text
		 INNER JOIN skill_tool st ON st.tool_id = t.id
		 INNER JOIN skill s ON s.id = st.skill_id
		 WHERE d.id = $1`,
		created.ID,
	).Scan(&datasourceName, &datasourceHost, &databaseName, &dbUser, &toolConfig, &instructions)
	require.NoError(t, err)

	assert.Equal(t, "Orders DB", datasourceName)
	assert.Equal(t, "pg.internal", datasourceHost)
	assert.Equal(t, "orders", databaseName)
	assert.Equal(t, "orders_user", dbUser)
	assert.Contains(t, toolConfig, "SELECT * FROM orders")
	assert.NotContains(t, toolConfig, "pg-broken")
	assert.Contains(t, instructions, "SELECT * FROM orders")
	assert.NotContains(t, instructions, "Blank query must fail")
}

func TestIntegration_DatabaseUpdateDeleteRejectMultipleLinkedToolsWithoutSideEffects(t *testing.T) {
	pool, ctx := setupIntegrationServiceTenantSchema(t)
	dataSourceSvc := datasource.NewService(datasource.NewRepository(pool))
	skillSvc := skill.NewService(skill.NewRepository(pool))
	toolSvc := tool.NewService(tool.NewRepository(pool)).
		WithDatasource(&integrationToolDatasourceAdapter{svc: dataSourceSvc}, tenant.FromContext)
	svc := integration.NewService(toolSvc, dataSourceSvc, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(skillSvc, toolSvc, integration.NewRepository(pool))

	created, err := svc.CreateDatabase(ctx, integration.DatabaseCreateRequest{
		Name:        "Orders DB",
		Description: "Read orders",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg.internal",
		Port:        5432,
		Database:    "orders",
		DBUser:      "orders_user",
		DBPassword:  "secret",
		Query:       "SELECT * FROM orders",
	})
	require.NoError(t, err)

	duplicateConfig, err := json.Marshal(map[string]any{
		"datasource_id": created.ID.String(),
		"query":         "SELECT id FROM orders",
		"operation":     "SELECT",
	})
	require.NoError(t, err)
	_, err = toolSvc.Create(ctx, tool.CreateRequest{
		Name:        "Orders Duplicate Query",
		Type:        tool.ToolTypeSQL,
		Config:      duplicateConfig,
		Description: "Duplicate SQL companion",
	})
	require.NoError(t, err)

	_, err = svc.UpdateDatabase(ctx, created.ID, integration.DatabaseCreateRequest{
		Name:        "Orders DB Updated",
		Description: "Should not update",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg2.internal",
		Port:        5432,
		Database:    "orders_updated",
		DBUser:      "orders_updated",
		Query:       "UPDATE orders SET synced = true",
		AllowWrite:  true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple database tools")

	err = svc.DeleteDatabase(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple database tools")

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenant.FromContext(ctx))
	require.NoError(t, err)
	defer release()

	var datasourceName, datasourceHost, databaseName, dbUser string
	err = conn.QueryRow(ctx,
		`SELECT name, host, database, db_user FROM data_source WHERE id = $1`,
		created.ID,
	).Scan(&datasourceName, &datasourceHost, &databaseName, &dbUser)
	require.NoError(t, err)
	assert.Equal(t, "Orders DB", datasourceName)
	assert.Equal(t, "pg.internal", datasourceHost)
	assert.Equal(t, "orders", databaseName)
	assert.Equal(t, "orders_user", dbUser)

	var linkedToolCount int
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM tool
		 WHERE config->>'dataSourceId' = $1 OR config->>'datasource_id' = $1`,
		created.ID.String(),
	).Scan(&linkedToolCount)
	require.NoError(t, err)
	assert.Equal(t, 2, linkedToolCount)
}

func TestIntegration_DatabaseUpdateDeleteRejectMissingLinkedToolWithoutSideEffects(t *testing.T) {
	pool, ctx := setupIntegrationServiceTenantSchema(t)
	dataSourceSvc := datasource.NewService(datasource.NewRepository(pool))
	skillSvc := skill.NewService(skill.NewRepository(pool))
	toolSvc := tool.NewService(tool.NewRepository(pool)).
		WithDatasource(&integrationToolDatasourceAdapter{svc: dataSourceSvc}, tenant.FromContext)
	svc := integration.NewService(toolSvc, dataSourceSvc, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(skillSvc, toolSvc, integration.NewRepository(pool))

	tenantID := tenant.FromContext(ctx)
	created, err := dataSourceSvc.Create(ctx, tenantID, datasource.CreateRequest{
		Name:       "Orders DB",
		Type:       datasource.DataSourceTypePostgreSQL,
		Host:       "pg.internal",
		Port:       5432,
		Database:   "orders",
		DBUser:     "orders_user",
		DBPassword: "secret",
	})
	require.NoError(t, err)

	_, err = svc.UpdateDatabase(ctx, created.ID, integration.DatabaseCreateRequest{
		Name:        "Orders DB Updated",
		Description: "Should not update",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg2.internal",
		Port:        5432,
		Database:    "orders_updated",
		DBUser:      "orders_updated",
		Query:       "UPDATE orders SET synced = true",
		AllowWrite:  true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no linked database tool")

	err = svc.DeleteDatabase(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no linked database tool")

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	defer release()

	var datasourceName, datasourceHost, databaseName, dbUser string
	err = conn.QueryRow(ctx,
		`SELECT name, host, database, db_user FROM data_source WHERE id = $1`,
		created.ID,
	).Scan(&datasourceName, &datasourceHost, &databaseName, &dbUser)
	require.NoError(t, err)
	assert.Equal(t, "Orders DB", datasourceName)
	assert.Equal(t, "pg.internal", datasourceHost)
	assert.Equal(t, "orders", databaseName)
	assert.Equal(t, "orders_user", dbUser)

	var linkedToolCount int
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM tool
		 WHERE config->>'dataSourceId' = $1 OR config->>'datasource_id' = $1`,
		created.ID.String(),
	).Scan(&linkedToolCount)
	require.NoError(t, err)
	assert.Equal(t, 0, linkedToolCount)
}

func TestIntegration_DatabaseRejectsManualSkillBindingWithoutSideEffects(t *testing.T) {
	pool, ctx := setupIntegrationServiceTenantSchema(t)
	dataSourceSvc := datasource.NewService(datasource.NewRepository(pool))
	skillSvc := skill.NewService(skill.NewRepository(pool))
	toolSvc := tool.NewService(tool.NewRepository(pool)).
		WithDatasource(&integrationToolDatasourceAdapter{svc: dataSourceSvc}, tenant.FromContext)
	svc := integration.NewService(toolSvc, dataSourceSvc, &stubMCPCatalog{}, &stubVPNCatalog{}).
		WithHTTPManagement(skillSvc, toolSvc, integration.NewRepository(pool))

	created, err := svc.CreateDatabase(ctx, integration.DatabaseCreateRequest{
		Name:        "Orders DB",
		Description: "Read orders",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg.internal",
		Port:        5432,
		Database:    "orders",
		DBUser:      "orders_user",
		DBPassword:  "secret",
		Query:       "SELECT * FROM orders",
	})
	require.NoError(t, err)

	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	defer release()

	var toolID, skillID uuid.UUID
	err = conn.QueryRow(ctx,
		`SELECT t.id, s.id
		 FROM tool t
		 INNER JOIN skill_tool st ON st.tool_id = t.id
		 INNER JOIN skill s ON s.id = st.skill_id
		 WHERE t.config->>'dataSourceId' = $1 OR t.config->>'datasource_id' = $1`,
		created.ID.String(),
	).Scan(&toolID, &skillID)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `UPDATE skill SET category = 'CUSTOM_DATABASE' WHERE id = $1`, skillID)
	require.NoError(t, err)

	page, err := svc.List(ctx, pagination.PageRequest{Page: 0, Size: 1000}, integration.ListFilters{})
	require.NoError(t, err)
	var catalogItem integration.Response
	for _, item := range page.Content {
		if item.ID == created.ID {
			catalogItem = item
			break
		}
	}
	require.NotEqual(t, uuid.Nil, catalogItem.ID)
	assert.True(t, catalogItem.Advanced)
	assert.Contains(t, catalogItem.Summary, "tool:Orders DB")

	_, err = svc.GetDatabase(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated database skill")

	_, err = svc.UpdateDatabase(ctx, created.ID, integration.DatabaseCreateRequest{
		Name:        "Orders DB Updated",
		Description: "Should not update",
		Type:        datasource.DataSourceTypePostgreSQL,
		Host:        "pg2.internal",
		Port:        5432,
		Database:    "orders_updated",
		DBUser:      "orders_updated",
		Query:       "UPDATE orders SET synced = true",
		AllowWrite:  true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated database skill")

	err = svc.DeleteDatabase(ctx, created.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated database skill")

	var datasourceName, datasourceHost, databaseName, dbUser, toolConfig, skillCategory string
	var remainingBindings int
	err = conn.QueryRow(ctx,
		`SELECT d.name, d.host, d.database, d.db_user, t.config::text, s.category, COUNT(st.*)
		 FROM data_source d
		 INNER JOIN tool t ON t.config->>'dataSourceId' = d.id::text OR t.config->>'datasource_id' = d.id::text
		 INNER JOIN skill_tool st ON st.tool_id = t.id
		 INNER JOIN skill s ON s.id = st.skill_id
		 WHERE d.id = $1 AND t.id = $2 AND s.id = $3
		 GROUP BY d.name, d.host, d.database, d.db_user, t.config::text, s.category`,
		created.ID,
		toolID,
		skillID,
	).Scan(&datasourceName, &datasourceHost, &databaseName, &dbUser, &toolConfig, &skillCategory, &remainingBindings)
	require.NoError(t, err)
	assert.Equal(t, "Orders DB", datasourceName)
	assert.Equal(t, "pg.internal", datasourceHost)
	assert.Equal(t, "orders", databaseName)
	assert.Equal(t, "orders_user", dbUser)
	assert.Contains(t, toolConfig, "SELECT * FROM orders")
	assert.NotContains(t, toolConfig, "UPDATE orders SET synced = true")
	assert.Equal(t, "CUSTOM_DATABASE", skillCategory)
	assert.Equal(t, 1, remainingBindings)
}
