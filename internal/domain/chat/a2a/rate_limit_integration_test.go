//go:build integration

package a2a

import (
	"context"
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
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const a2aRateLimitIntegrationTenant = "a2alimittest"

func a2aSchemaMigrationsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../../../migrations/schemas")
	require.NoError(t, err)
	require.DirExists(t, abs)
	return abs
}

func a2aSchemaMigrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(a2aSchemaMigrationsDir(t))
	require.NoError(t, err)
	var ups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			ups = append(ups, entry.Name())
		}
	}
	sort.Strings(ups)
	require.NotEmpty(t, ups)
	return ups
}

func setupA2ATenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	schema := "ah_" + a2aRateLimitIntegrationTenant

	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, name := range a2aSchemaMigrationFiles(t) {
		sqlBytes, err := os.ReadFile(filepath.Join(a2aSchemaMigrationsDir(t), name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sqlBytes))
		require.NoError(t, err, "apply migration %s", name)
	}

	return pool, tenant.NewContext(ctx, a2aRateLimitIntegrationTenant)
}

func TestIntegration_DistributedRateLimitSharedAcrossServiceInstances(t *testing.T) {
	pool, ctx := setupA2ATenantSchema(t)
	repo := NewRepository(pool)

	conn, release, err := database.AcquireWithTenant(ctx, pool, a2aRateLimitIntegrationTenant)
	require.NoError(t, err)
	defer release()

	var agentID uuid.UUID
	var agentName string
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT id, name FROM agent ORDER BY created_at LIMIT 1`,
	).Scan(&agentID, &agentName))

	agents := newFakeAgentReader()
	agents.add(a2aRateLimitIntegrationTenant, AgentRef{ID: agentID, Name: agentName})

	_, err = repo.UpsertGrant(ctx, a2aRateLimitIntegrationTenant, Grant{
		SubjectTenant: "tenant-a",
		AgentID:       agentID,
		Actions:       []string{ActionInvoke},
	})
	require.NoError(t, err)

	executor := newFakeSessionExecutor()
	first := NewService(repo, agents, executor).WithPairLimit(2)
	second := NewService(repo, agents, executor).WithPairLimit(2)

	for i := 0; i < 2; i++ {
		resp, err := first.Invoke(ctx, "tenant-a", InvokeRequest{
			TargetTenant: a2aRateLimitIntegrationTenant,
			AgentID:      agentID.String(),
			Input:        "ping",
		})
		require.NoError(t, err)
		resp.Cancel()
	}

	_, err = second.Invoke(ctx, "tenant-a", InvokeRequest{
		TargetTenant: a2aRateLimitIntegrationTenant,
		AgentID:      agentID.String(),
		Input:        "ping",
	})
	require.ErrorIs(t, err, ErrRateLimited)

	var count int
	require.NoError(t, conn.QueryRow(ctx,
		`SELECT request_count FROM a2a_rate_limit WHERE source_tenant = $1`,
		"tenant-a",
	).Scan(&count))
	assert.Equal(t, 3, count)
}
