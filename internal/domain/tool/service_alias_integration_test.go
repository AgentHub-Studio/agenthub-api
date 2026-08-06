//go:build integration

package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const sqlDatasourceAliasTenant = "sqlalias"

func TestIntegration_ToolServiceRejectsConflictingSQLDatasourceAliasesWithoutPersisting(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := tenant.NewContext(context.Background(), sqlDatasourceAliasTenant)
	setupSQLDatasourceAliasSchema(t, pool)
	svc := tool.NewService(tool.NewRepository(pool))

	_, err := svc.Create(ctx, tool.CreateRequest{
		Name: "conflicting-sql-datasource",
		Type: tool.ToolTypeSQL,
		Config: json.RawMessage(`{
			"datasource_id":"00000000-0000-0000-0000-000000000001",
			"dataSourceId":"00000000-0000-0000-0000-000000000002",
			"query":"SELECT 1"
		}`),
	})
	require.ErrorIs(t, err, tool.ErrValidation)

	var count int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM ah_sqlalias.tool").Scan(&count))
	assert.Zero(t, count)

	created, err := svc.Create(ctx, tool.CreateRequest{
		Name:   "stable-sql-datasource",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"datasource_id":"00000000-0000-0000-0000-000000000001","query":"SELECT 1"}`),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, created.ID, tool.UpdateRequest{Config: json.RawMessage(`{
		"datasource_id":"00000000-0000-0000-0000-000000000001",
		"dataSourceId":"00000000-0000-0000-0000-000000000002",
		"query":"SELECT 1"
	}`)})
	require.ErrorIs(t, err, tool.ErrValidation)

	persisted, err := tool.NewRepository(pool).GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"datasource_id":"00000000-0000-0000-0000-000000000001","query":"SELECT 1"}`, string(persisted.Config))
}

func TestIntegration_ToolServiceRejectsConflictingHTTPURLAliasesWithoutPersisting(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := tenant.NewContext(context.Background(), sqlDatasourceAliasTenant)
	setupSQLDatasourceAliasSchema(t, pool)
	svc := tool.NewService(tool.NewRepository(pool)).WithHTTPURLValidator(func(string) error { return nil })

	_, err := svc.Create(ctx, tool.CreateRequest{
		Name: "conflicting-http-url",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(`{
			"url":"https://api.example.com/primary",
			"urlTemplate":"https://api.example.com/legacy"
		}`),
	})
	require.ErrorIs(t, err, tool.ErrValidation)

	var count int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM ah_sqlalias.tool").Scan(&count))
	assert.Zero(t, count)

	created, err := svc.Create(ctx, tool.CreateRequest{
		Name:   "stable-http-url",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/primary"}`),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, created.ID, tool.UpdateRequest{Config: json.RawMessage(`{
		"url":"https://api.example.com/primary",
		"urlTemplate":"https://api.example.com/legacy"
	}`)})
	require.ErrorIs(t, err, tool.ErrValidation)

	persisted, err := tool.NewRepository(pool).GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"url":"https://api.example.com/primary"}`, string(persisted.Config))
}

func setupSQLDatasourceAliasSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Kept local and minimal: it mirrors the columns read and written by
	// tool.Repository while exercising tenant search_path against real Postgres.
	_, err := pool.Exec(context.Background(), `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE SCHEMA IF NOT EXISTS ah_sqlalias;
		CREATE TABLE IF NOT EXISTS ah_sqlalias.tool (
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
		)
	`)
	require.NoError(t, err)
}
