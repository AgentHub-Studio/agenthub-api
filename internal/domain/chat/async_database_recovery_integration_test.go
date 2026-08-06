//go:build integration

package chat

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const databaseRecoveryTenant = "dbrecovery"

type databaseRecoveryRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *databaseRecoveryRunner) RunSession(ctx context.Context, _ RunInput) (<-chan RunEvent, error) {
	events := make(chan RunEvent, 1)
	close(r.started)
	go func() {
		defer close(events)
		select {
		case <-r.release:
			events <- RunEvent{Type: "run_complete"}
		case <-ctx.Done():
			events <- RunEvent{Type: "error"}
		}
	}()
	return events, nil
}

func databaseRecoveryMigrationFiles(t *testing.T) []string {
	t.Helper()
	dir, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	require.NotEmpty(t, files)
	return files
}

func setupDatabaseRecoveryTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := tenant.NewContext(context.Background(), databaseRecoveryTenant)
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS ah_"+databaseRecoveryTenant)

	conn, release, err := database.AcquireWithTenant(ctx, pool, databaseRecoveryTenant)
	require.NoError(t, err)
	defer release()

	dir, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	for _, name := range databaseRecoveryMigrationFiles(t) {
		sqlBytes, readErr := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, readErr, "read migration %s", name)
		_, execErr := conn.Exec(ctx, string(sqlBytes))
		require.NoError(t, execErr, "apply migration %s", name)
	}

	return pool, ctx
}

func terminatePooledConnections(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	admin, err := pgx.Connect(context.Background(), pool.Config().ConnString())
	require.NoError(t, err)
	defer func() { _ = admin.Close(context.Background()) }()

	result, err := admin.Exec(context.Background(), `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid()`)
	require.NoError(t, err)
	return result.RowsAffected()
}

func TestIntegration_AsyncExecutorCompletesRunAfterDatabaseConnectionLoss(t *testing.T) {
	pool, ctx := setupDatabaseRecoveryTenantSchema(t)
	repo := NewRepository(pool)

	var agentID uuid.UUID
	conn, release, err := database.AcquireWithTenant(ctx, pool, databaseRecoveryTenant)
	require.NoError(t, err)
	err = conn.QueryRow(ctx, "SELECT id FROM agent ORDER BY created_at LIMIT 1").Scan(&agentID)
	release()
	require.NoError(t, err)

	session, err := repo.CreateSession(ctx, ChatSession{
		AgentID: &agentID,
		Title:   "database recovery",
		Status:  StatusActive,
	})
	require.NoError(t, err)
	run, err := repo.CreateRun(ctx, ChatRun{
		SessionID: session.ID,
		TenantID:  databaseRecoveryTenant,
		Status:    ChatRunStatusQueued,
	})
	require.NoError(t, err)

	runner := &databaseRecoveryRunner{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	executor := NewAsyncExecutor(repo, runner, "")
	done := make(chan struct{})
	go func() {
		defer close(done)
		executor.processTask(ChatRunTask{
			RunID:     run.ID,
			SessionID: session.ID,
			TenantID:  databaseRecoveryTenant,
			Message:   "finish after database recovery",
		})
	}()

	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not start")
	}

	terminated := terminatePooledConnections(t, pool)
	require.Positive(t, terminated, "the test must terminate at least one pooled PostgreSQL connection")
	close(runner.release)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("executor did not finish after database connection loss")
	}

	persisted, err := repo.GetRunByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, ChatRunStatusCompleted, persisted.Status)
	require.NotNil(t, persisted.CompletedAt)
}
