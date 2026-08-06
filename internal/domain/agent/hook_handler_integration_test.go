//go:build integration

package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

func TestIntegration_HookHandlerRejectsConflictingPromptAliasesOnCreateAndUpdate(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	createdAgent := createIntegrationAgent(t, ctx, agentRepo, "prompt-hook-alias-contract")

	router := hookHandlerRouter(pool)
	path := "/api/agents/" + createdAgent.ID.String() + "/hooks"
	createReq := requestWithContext(t, http.MethodPost, path, `{
		"event":"before_run",
		"hookType":"prompt",
		"config":{"template":"Keep the initial instruction"}
	}`, ctx)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())

	var created struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	updateReq := requestWithContext(t, http.MethodPut, path+"/"+created.ID.String(), `{
		"config":{"template":"Prefer this","inject":"Use this instead"}
	}`, ctx)
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	assert.Equal(t, http.StatusUnprocessableEntity, updateRec.Code, updateRec.Body.String())
	assert.Contains(t, updateRec.Body.String(), "template")
	assert.Contains(t, updateRec.Body.String(), "inject")

	getReq := httptest.NewRequest(http.MethodGet, path+"/"+created.ID.String(), nil).WithContext(ctx)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code, getRec.Body.String())
	assert.Contains(t, getRec.Body.String(), "Keep the initial instruction")
	assert.NotContains(t, getRec.Body.String(), "Use this instead")
}

func TestIntegration_HookHandlerRedactsPersistedSensitiveConfig(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	createdAgent := createIntegrationAgent(t, ctx, agentRepo, "hook-config-redaction")

	router := hookHandlerRouter(pool)
	path := "/api/agents/" + createdAgent.ID.String() + "/hooks"
	createReq := requestWithContext(t, http.MethodPost, path, `{
		"event":"before_run",
		"hookType":"http",
		"config":{"url":"https://hook-user:hook-url-password@hooks.example.test/run?api_key=hook-url-api-key&safe=kept","headers":{"Authorization":"Bearer hook-http-secret"},"nested":{"api_key":"hook-api-key-secret"},"safe":"kept"}
	}`, ctx)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	assert.NotContains(t, createRec.Body.String(), "hook-http-secret")
	assert.NotContains(t, createRec.Body.String(), "hook-api-key-secret")
	assert.NotContains(t, createRec.Body.String(), "hook-url-password")
	assert.NotContains(t, createRec.Body.String(), "hook-url-api-key")
	assert.Contains(t, createRec.Body.String(), "kept")

	var created struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	conn, release, err := database.AcquireWithTenant(ctx, pool, tenant.FromContext(ctx))
	require.NoError(t, err)
	defer release()
	var persisted string
	require.NoError(t, conn.QueryRow(ctx, `SELECT config::text FROM agent_hook WHERE id = $1`, created.ID).Scan(&persisted))
	assert.Contains(t, persisted, "hook-http-secret")
	assert.Contains(t, persisted, "hook-api-key-secret")
	assert.Contains(t, persisted, "hook-url-password")
	assert.Contains(t, persisted, "hook-url-api-key")

	updateReq := requestWithContext(t, http.MethodPut, path+"/"+created.ID.String(), `{
		"config":{"url":"https://hook-user:hook-updated-url-password@hooks.example.test/run?api_key=hook-updated-url-api-key&safe=updated","headers":{"X-API-Key":"hook-updated-api-key-secret"},"nested":{"client_secret":"hook-client-secret"},"safe":"updated"}
	}`, ctx)
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	require.Equal(t, http.StatusOK, updateRec.Code, updateRec.Body.String())
	assert.NotContains(t, updateRec.Body.String(), "hook-updated-api-key-secret")
	assert.NotContains(t, updateRec.Body.String(), "hook-client-secret")
	assert.NotContains(t, updateRec.Body.String(), "hook-updated-url-password")
	assert.NotContains(t, updateRec.Body.String(), "hook-updated-url-api-key")
	assert.Contains(t, updateRec.Body.String(), "updated")

	require.NoError(t, conn.QueryRow(ctx, `SELECT config::text FROM agent_hook WHERE id = $1`, created.ID).Scan(&persisted))
	assert.Contains(t, persisted, "hook-updated-api-key-secret")
	assert.Contains(t, persisted, "hook-client-secret")
	assert.Contains(t, persisted, "hook-updated-url-password")
	assert.Contains(t, persisted, "hook-updated-url-api-key")

	for _, requestPath := range []string{path, path + "/" + created.ID.String()} {
		req := httptest.NewRequest(http.MethodGet, requestPath, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "hook-updated-api-key-secret")
		assert.NotContains(t, rec.Body.String(), "hook-client-secret")
		assert.NotContains(t, rec.Body.String(), "hook-updated-url-password")
		assert.NotContains(t, rec.Body.String(), "hook-updated-url-api-key")
		assert.Contains(t, rec.Body.String(), "updated")
	}
}

func hookHandlerRouter(pool *pgxpool.Pool) chi.Router {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	agent.NewHookHandler(pool).RegisterRoutes(router)
	return router
}

func requestWithContext(t *testing.T, method, path, body string, ctx context.Context) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	return req
}
