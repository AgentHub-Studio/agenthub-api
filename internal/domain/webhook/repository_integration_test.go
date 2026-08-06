//go:build integration

package webhook_test

import (
	"bytes"
	"context"
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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/webhook"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const webhookIntegrationTenant = "webhooklimitcontract"

func setupWebhookTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + webhookIntegrationTenant
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

	return pool, tenant.NewContext(ctx, webhookIntegrationTenant)
}

type webhookIntegrationTenantLister struct {
	ids []string
}

func (l webhookIntegrationTenantLister) ListAllIDs(_ context.Context) ([]string, error) {
	return l.ids, nil
}

func TestIntegration_WebhookHTTPRejectsOversizedRawBodyBeforeDelivery(t *testing.T) {
	pool, tenantCtx := setupWebhookTenantSchema(t)
	repo := webhook.NewRepository(pool)
	svc := webhook.NewService(repo).
		WithTenantLister(webhookIntegrationTenantLister{ids: []string{webhookIntegrationTenant}})
	handler := webhook.NewHandler(svc)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	const token = "webhook-limit-token"
	const secret = "webhook-limit-secret"
	created, err := repo.Create(tenantCtx, webhook.WebhookConfig{
		Name:       "webhook-limit-integration",
		URL:        upstream.URL,
		Events:     []string{"Push Hook"},
		Secret:     ptr(secret),
		Token:      token,
		Enabled:    true,
		RetryCount: 0,
	})
	require.NoError(t, err)

	body := bytes.Repeat([]byte("x"), 1<<20+1)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/"+token+"/ingest", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Token", secret)
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, res.Code, res.Body.String())
	deliveries, total, err := repo.ListDeliveries(tenantCtx, created.ID, webhook.DeliveryFilter{}, pagination.PageRequest{Size: 10})
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, deliveries)
}

func ptr(value string) *string {
	return &value
}
