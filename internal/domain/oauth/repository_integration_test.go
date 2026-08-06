//go:build integration

package oauth_test

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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const oauthIntegrationTenant = "oauthredactiontest"

func setupOAuthTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + oauthIntegrationTenant
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

	return pool, tenant.NewContext(ctx, oauthIntegrationTenant)
}

func TestIntegration_OAuthResolveRouteRedactsSecretValue(t *testing.T) {
	pool, ctx := setupOAuthTenantSchema(t)
	svc := oauth.NewServiceWithEncryption(oauth.NewRepository(pool), aesKey)

	created, err := svc.Create(ctx, oauthIntegrationTenant, oauth.CreateRequest{
		Name:        "bearer-redaction",
		AuthType:    oauth.AuthTypeBearerToken,
		BearerToken: strPtr("oauth-resolve-secret"),
	})
	require.NoError(t, err)

	resolved, err := svc.ResolveAuthHeader(ctx, oauthIntegrationTenant, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bearer oauth-resolve-secret", resolved.Value)

	h := oauth.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			reqCtx := tenant.NewContext(req.Context(), oauthIntegrationTenant)
			reqCtx = middleware.ContextWithRoles(reqCtx, "admin")
			next.ServeHTTP(w, req.WithContext(reqCtx))
		})
	})
	r.Mount("/api/oauth", h.Routes())

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+created.ID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "oauth-resolve-secret")
	var resp oauth.ResolveResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Authorization", resp.Header)
	assert.Equal(t, "***", resp.Value)
}
