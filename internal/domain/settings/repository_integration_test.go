//go:build integration

package settings_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const settingsIntegrationTenant = "settingsredactiontest"

func setupSettingsTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + settingsIntegrationTenant
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

	return pool, tenant.NewContext(ctx, settingsIntegrationTenant)
}

func TestIntegration_SettingsResponseRedactsNestedSensitiveValues(t *testing.T) {
	pool, ctx := setupSettingsTenantSchema(t)
	repo := settings.NewRepository(pool)
	svc := settings.NewService(repo)

	raw := json.RawMessage(`{
		"provider":"custom",
		"credentials":{
			"apiKey":"nested-api-key-secret",
			"clientSecret":"nested-client-secret",
			"headers":{"Authorization":"Bearer nested-authorization-token"}
		},
		"fallbacks":[{"refreshToken":"nested-refresh-token"}]
	}`)
	created, err := svc.Upsert(ctx, "provider.bundle", settings.UpdateSettingRequest{Value: raw})
	require.NoError(t, err)
	assert.NotContains(t, string(created.Value), "nested-api-key-secret")

	persisted, err := repo.FindByKey(ctx, "provider.bundle")
	require.NoError(t, err)
	assert.Contains(t, string(persisted.Value), "nested-api-key-secret")
	assert.Contains(t, string(persisted.Value), "nested-client-secret")

	got, err := svc.Get(ctx, "provider.bundle")
	require.NoError(t, err)
	body := string(got.Value)
	assert.NotContains(t, body, "nested-api-key-secret")
	assert.NotContains(t, body, "nested-client-secret")
	assert.NotContains(t, body, "nested-authorization-token")
	assert.NotContains(t, body, "nested-refresh-token")
	assert.Contains(t, body, `"apiKey":"***"`)
	assert.Contains(t, body, `"clientSecret":"***"`)
	assert.Contains(t, body, `"Authorization":"***"`)
	assert.Contains(t, body, `"refreshToken":"***"`)
}
