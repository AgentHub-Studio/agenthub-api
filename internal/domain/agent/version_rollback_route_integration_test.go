//go:build integration

package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const versionRollbackRouteTenant = "versionrollbacktest"

func setupVersionRollbackRouteTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	schema := "ah_" + versionRollbackRouteTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	dir, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			migrations = append(migrations, entry.Name())
		}
	}
	sort.Strings(migrations)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, name := range migrations {
		sql, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sql))
		require.NoError(t, err, "apply migration %s", name)
	}

	return pool, tenant.NewContext(ctx, versionRollbackRouteTenant)
}

func TestIntegration_VersionRollbackRouteRestoresPublishedSnapshot(t *testing.T) {
	pool, ctx := setupVersionRollbackRouteTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	versionRepo := agent.NewVersionRepository(pool)
	versionSvc := agent.NewVersionService(agentRepo, versionRepo)

	currentPrompt := "current prompt"
	createdAgent, err := agentRepo.Create(ctx, agent.Agent{
		ID:           uuid.New(),
		Name:         "rollback-route",
		Slug:         "rollback-route",
		Description:  "version rollback HTTP integration",
		Status:       agent.StatusPublished,
		SystemPrompt: &currentPrompt,
		ModelConfig:  json.RawMessage(`{"provider":"openrouter","model":"current-model"}`),
		Config:       json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	targetPrompt := "published version one"
	target, err := versionRepo.Create(ctx, agent.AgentVersion{
		ID:             uuid.New(),
		AgentID:        createdAgent.ID,
		VersionNumber:  1,
		Status:         agent.VersionStatusDraft,
		Description:    "published baseline",
		DefinitionJSON: json.RawMessage(`{"systemPrompt":"published version one"}`),
		ConfigJSON:     json.RawMessage(`{"provider":"openrouter","model":"baseline-model"}`),
	})
	require.NoError(t, err)
	_, err = versionRepo.Publish(ctx, target.ID)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	agent.NewVersionHandler(versionSvc).RegisterVersionRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+createdAgent.ID.String()+"/versions/"+target.ID.String()+"/rollback", nil)
	req = req.WithContext(tenant.NewContext(req.Context(), versionRollbackRouteTenant))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response agent.AgentVersionResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, createdAgent.ID, response.AgentID)
	assert.Equal(t, 2, response.VersionNumber)
	assert.Equal(t, string(agent.VersionStatusPublished), response.Status)
	assert.Equal(t, "Rollback to version 1", response.Description)

	restored, err := agentRepo.FindByID(ctx, createdAgent.ID)
	require.NoError(t, err)
	require.NotNil(t, restored.SystemPrompt)
	assert.Equal(t, targetPrompt, *restored.SystemPrompt)
	assert.JSONEq(t, string(target.ConfigJSON), string(restored.ModelConfig))
	assert.Equal(t, 2, restored.CurrentVersion)

	rollbackVersion, err := versionRepo.FindByID(ctx, response.ID)
	require.NoError(t, err)
	assert.Equal(t, agent.VersionStatusPublished, rollbackVersion.Status)
	assert.JSONEq(t, string(target.DefinitionJSON), string(rollbackVersion.DefinitionJSON))
	assert.JSONEq(t, string(target.ConfigJSON), string(rollbackVersion.ConfigJSON))
}
