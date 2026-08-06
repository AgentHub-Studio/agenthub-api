//go:build integration

package agent_test

import (
	"bytes"
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

const versionHandlerIntegrationTenant = "agentversiontest"

func versionHandlerSchemasDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	require.DirExists(t, abs, "migrations/schemas dir must exist")
	return abs
}

func setupVersionHandlerTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + versionHandlerIntegrationTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	dir := versionHandlerSchemasDir(t)
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

	return pool, tenant.NewContext(ctx, versionHandlerIntegrationTenant)
}

func TestIntegration_VersionHandlerRejectsCrossAgentVersionOperations(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	versionRepo := agent.NewVersionRepository(pool)
	versionSvc := agent.NewVersionService(agentRepo, versionRepo)

	owner := createIntegrationAgent(t, ctx, agentRepo, "version-owner")
	other := createIntegrationAgent(t, ctx, agentRepo, "version-other")
	versionID := uuid.New()
	createdVersion, err := versionRepo.Create(ctx, agent.AgentVersion{
		ID:             versionID,
		AgentID:        owner.ID,
		VersionNumber:  1,
		Status:         agent.VersionStatusDraft,
		Description:    "owned draft",
		DefinitionJSON: json.RawMessage(`{"systemPrompt":"owned prompt"}`),
		ConfigJSON:     json.RawMessage(`{"provider":"openrouter","model":"owned-model"}`),
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	agent.NewVersionHandler(versionSvc).RegisterVersionRoutes(r)

	getReq := httptest.NewRequest(http.MethodGet, "/api/agents/"+other.ID.String()+"/versions/by-id/"+createdVersion.ID.String(), nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	assert.Equal(t, http.StatusNotFound, getRec.Code)

	updateBody, err := json.Marshal(map[string]any{"description": "cross-agent mutation"})
	require.NoError(t, err)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/agents/"+other.ID.String()+"/versions/by-id/"+createdVersion.ID.String(), bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	r.ServeHTTP(updateRec, updateReq)
	assert.Equal(t, http.StatusNotFound, updateRec.Code)

	publishReq := httptest.NewRequest(http.MethodPost, "/api/agents/"+other.ID.String()+"/versions/"+createdVersion.ID.String()+"/publish", nil)
	publishRec := httptest.NewRecorder()
	r.ServeHTTP(publishRec, publishReq)
	assert.Equal(t, http.StatusNotFound, publishRec.Code)

	got, err := versionRepo.FindByID(ctx, createdVersion.ID)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, got.AgentID)
	assert.Equal(t, agent.VersionStatusDraft, got.Status)
	assert.Equal(t, "owned draft", got.Description)
}

func TestIntegration_VersionServiceCreateDraftDefaultsToCurrentAgentSnapshot(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	versionRepo := agent.NewVersionRepository(pool)
	versionSvc := agent.NewVersionService(agentRepo, versionRepo)

	prompt := "current integration prompt"
	createdAgent, err := agentRepo.Create(ctx, agent.Agent{
		ID:           uuid.New(),
		Name:         "version-snapshot",
		Slug:         "version-snapshot",
		Description:  "integration agent",
		Status:       agent.StatusPublished,
		SystemPrompt: &prompt,
		ModelConfig:  json.RawMessage(`{"provider":"openrouter","model":"openai/gpt-oss-120b"}`),
		Config:       json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	resp, err := versionSvc.CreateDraft(ctx, createdAgent.ID, agent.CreateAgentVersionRequest{Description: "snapshot"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"systemPrompt":"current integration prompt"}`, string(resp.DefinitionJSON))
	assert.JSONEq(t, `{"provider":"openrouter","model":"openai/gpt-oss-120b"}`, string(resp.ConfigJSON))

	persisted, err := versionRepo.FindByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"systemPrompt":"current integration prompt"}`, string(persisted.DefinitionJSON))
	assert.JSONEq(t, `{"provider":"openrouter","model":"openai/gpt-oss-120b"}`, string(persisted.ConfigJSON))
}

func TestIntegration_VersionServiceCreateDraftRedactsResponseConfigSecrets(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	versionRepo := agent.NewVersionRepository(pool)
	versionSvc := agent.NewVersionService(agentRepo, versionRepo)

	createdAgent, err := agentRepo.Create(ctx, agent.Agent{
		ID:          uuid.New(),
		Name:        "version-redaction",
		Slug:        "version-redaction",
		Description: "integration agent",
		Status:      agent.StatusPublished,
		ModelConfig: json.RawMessage(`{
			"provider":"openrouter",
			"model":"openai/gpt-oss-120b",
			"apiKey":"sk-version-secret",
			"api_secret":"legacy-version-secret"
		}`),
		Config: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	resp, err := versionSvc.CreateDraft(ctx, createdAgent.ID, agent.CreateAgentVersionRequest{Description: "redacted snapshot"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"provider":"openrouter","model":"openai/gpt-oss-120b"}`, string(resp.ConfigJSON))
	assert.NotContains(t, string(resp.ConfigJSON), "apiKey")
	assert.NotContains(t, string(resp.ConfigJSON), "api_secret")
	assert.NotContains(t, string(resp.ConfigJSON), "sk-version-secret")
	assert.NotContains(t, string(resp.ConfigJSON), "legacy-version-secret")

	reloaded, err := versionSvc.GetVersionByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.JSONEq(t, string(resp.ConfigJSON), string(reloaded.ConfigJSON))
	assert.NotContains(t, string(reloaded.ConfigJSON), "apiKey")
	assert.NotContains(t, string(reloaded.ConfigJSON), "api_secret")

	persisted, err := versionRepo.FindByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.Contains(t, string(persisted.ConfigJSON), "apiKey")
	assert.Contains(t, string(persisted.ConfigJSON), "api_secret")
}

func TestIntegration_VersionServiceCreateDraftRedactsNestedResponseConfigSecrets(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	versionRepo := agent.NewVersionRepository(pool)
	versionSvc := agent.NewVersionService(agentRepo, versionRepo)

	createdAgent, err := agentRepo.Create(ctx, agent.Agent{
		ID:          uuid.New(),
		Name:        "version-nested-redaction",
		Slug:        "version-nested-redaction",
		Description: "integration agent",
		Status:      agent.StatusPublished,
		ModelConfig: json.RawMessage(`{
			"provider":"openrouter",
			"model":"openai/gpt-oss-120b",
			"credentials":{
				"apiKey":"nested-version-secret",
				"api_secret":"nested-legacy-secret",
				"safe":"kept"
			},
			"fallbacks":[
				{"model":"backup","clientSecret":"nested-client-secret"}
			],
			"voice":{
				"enabled":true,
				"ttsVoice":"nova"
			}
		}`),
		Config: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	resp, err := versionSvc.CreateDraft(ctx, createdAgent.ID, agent.CreateAgentVersionRequest{Description: "nested redacted snapshot"})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"provider":"openrouter",
		"model":"openai/gpt-oss-120b",
		"credentials":{"safe":"kept"},
		"fallbacks":[{"model":"backup"}],
		"voice":{"enabled":true,"ttsVoice":"nova"}
	}`, string(resp.ConfigJSON))
	assert.NotContains(t, string(resp.ConfigJSON), "apiKey")
	assert.NotContains(t, string(resp.ConfigJSON), "api_secret")
	assert.NotContains(t, string(resp.ConfigJSON), "clientSecret")
	assert.NotContains(t, string(resp.ConfigJSON), "nested-version-secret")
	assert.NotContains(t, string(resp.ConfigJSON), "nested-legacy-secret")
	assert.NotContains(t, string(resp.ConfigJSON), "nested-client-secret")

	reloaded, err := versionSvc.GetVersionByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.JSONEq(t, string(resp.ConfigJSON), string(reloaded.ConfigJSON))
	assert.NotContains(t, string(reloaded.ConfigJSON), "apiKey")
	assert.NotContains(t, string(reloaded.ConfigJSON), "nested-client-secret")

	persisted, err := versionRepo.FindByID(ctx, resp.ID)
	require.NoError(t, err)
	assert.Contains(t, string(persisted.ConfigJSON), "nested-version-secret")
	assert.Contains(t, string(persisted.ConfigJSON), "nested-client-secret")
}

func createIntegrationAgent(t *testing.T, ctx context.Context, repo agent.Repository, slug string) agent.Agent {
	t.Helper()
	prompt := "integration prompt"
	created, err := repo.Create(ctx, agent.Agent{
		ID:           uuid.New(),
		Name:         slug,
		Slug:         slug,
		Description:  "integration agent",
		Status:       agent.StatusPublished,
		SystemPrompt: &prompt,
		ModelConfig:  json.RawMessage(`{"provider":"openrouter","model":"mistralai/mistral-nemo"}`),
		Config:       json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	return created
}
