//go:build integration

package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const triggerRunsHistoryTenant = "triggerrunshistory"

func TestIntegration_TriggerRunsHistoryIsBoundToRouteAgent(t *testing.T) {
	pool, ctx := setupTriggerRunsHistorySchema(t)
	agentID := uuid.New()
	otherAgentID := uuid.New()
	triggerID := uuid.New()
	olderRunID := uuid.New()
	newerRunID := uuid.New()

	seedTriggerRunsHistory(t, pool, agentID, otherAgentID, triggerID, olderRunID, newerRunID)

	router := chi.NewRouter()
	NewHandler(NewService(NewRepository(pool), nil)).RegisterRoutes(router)
	adminCtx := middleware.ContextWithRoles(ctx, "admin")

	request := httptest.NewRequest(http.MethodGet,
		"/api/agents/"+agentID.String()+"/triggers/"+triggerID.String()+"/runs?page=0&size=1", nil).
		WithContext(adminCtx)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var page pagination.Page[AgentTriggerRun]
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	assert.Equal(t, int64(2), page.TotalElements)
	assert.Equal(t, 1, page.Size)
	assert.True(t, page.First)
	assert.False(t, page.Last)
	require.Len(t, page.Content, 1)
	assert.Equal(t, newerRunID, page.Content[0].ID)
	assert.Equal(t, RunStatusCompleted, page.Content[0].Status)

	wrongParentRequest := httptest.NewRequest(http.MethodGet,
		"/api/agents/"+otherAgentID.String()+"/triggers/"+triggerID.String()+"/runs", nil).
		WithContext(adminCtx)
	wrongParentResponse := httptest.NewRecorder()
	router.ServeHTTP(wrongParentResponse, wrongParentRequest)

	assert.Equal(t, http.StatusNotFound, wrongParentResponse.Code)
}

func setupTriggerRunsHistorySchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	schema := "ah_" + triggerRunsHistoryTenant

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS pgcrypto")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "SET search_path TO "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "CREATE TABLE agent (id UUID PRIMARY KEY)")
	require.NoError(t, err)

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations", "schemas", "000061_ensure_agent_trigger.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), string(migration))
	require.NoError(t, err)

	return pool, tenant.NewContext(context.Background(), triggerRunsHistoryTenant)
}

func seedTriggerRunsHistory(t *testing.T, pool *pgxpool.Pool, agentID, otherAgentID, triggerID, olderRunID, newerRunID uuid.UUID) {
	t.Helper()
	schema := "ah_" + triggerRunsHistoryTenant
	ctx := context.Background()

	_, err := pool.Exec(ctx, fmt.Sprintf("INSERT INTO %s.agent (id) VALUES ($1), ($2)", schema), agentID, otherAgentID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.agent_trigger (id, agent_id, name, cron_expression, enabled, input_template)
		VALUES ($1, $2, 'daily-report', '0 9 * * *', true, '{}'::jsonb)
	`, schema), triggerID, agentID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.agent_trigger_run (id, trigger_id, session_id, status, started_at, completed_at, total_turns, total_tokens)
		VALUES
			($1, $3, $4, 'failed', $5, $5, 1, 8),
			($2, $3, $6, 'completed', $7, $7, 2, 16)
	`, schema),
		olderRunID,
		newerRunID,
		triggerID,
		uuid.New(),
		time.Now().Add(-time.Hour),
		uuid.New(),
		time.Now())
	require.NoError(t, err)
}
