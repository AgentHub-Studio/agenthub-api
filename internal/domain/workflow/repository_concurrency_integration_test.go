//go:build integration

package workflow_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/workflow"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const workflowResumeIntegrationTenant = "workflowresume"

func TestIntegration_WorkflowResumeIsSingleWinnerAndPersistent(t *testing.T) {
	pool, ctx := setupWorkflowResumeSchema(t)
	service := workflow.NewService(workflow.NewRepository(pool))

	created, err := service.Create(ctx, workflow.CreateRequest{
		Name: "Approval path",
		Steps: []workflow.Step{
			{ID: "prepare", Kind: workflow.StepKindAgent},
			{ID: "approval", Kind: workflow.StepKindBranch},
			{ID: "finalize", Kind: workflow.StepKindTool},
		},
	})
	require.NoError(t, err)

	execution, err := service.Execute(ctx, created.Slug, workflow.ExecuteRequest{Input: map[string]any{"request": "approval"}})
	require.NoError(t, err)
	require.Equal(t, workflow.ExecutionStateSuspended, execution.State)
	require.NotNil(t, execution.SuspendedAt)

	// A single-connection pool makes any nested acquire in ResumeExecution
	// observable as a deadline instead of letting a wider pool mask it.
	resumePool := singleConnectionPool(t, pool)
	resumeService := workflow.NewService(workflow.NewRepository(resumePool))
	resumeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = resumeService.Resume(resumeCtx, uuid.New(), workflow.ResumeRequest{"decision": "approve"})
	require.ErrorIs(t, err, workflow.ErrNotFound)

	const attempts = 64
	start := make(chan struct{})
	errs := make(chan error, attempts)
	var group sync.WaitGroup
	for attempt := 0; attempt < attempts; attempt++ {
		group.Add(1)
		go func(attempt int) {
			defer group.Done()
			<-start
			_, err := resumeService.Resume(resumeCtx, execution.ID, workflow.ResumeRequest{
				"decision": fmt.Sprintf("approve-%d", attempt),
			})
			errs <- err
		}(attempt)
	}
	close(start)
	group.Wait()
	close(errs)

	successes := 0
	alreadyResolved := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, workflow.ErrAlreadyResolved) {
			alreadyResolved++
			continue
		}
		t.Fatalf("unexpected concurrent resume error: %v", err)
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, attempts-1, alreadyResolved)

	// Closing and recreating the constrained pool models recovery after the
	// process that served the resume has released its database connections.
	resumePool.Close()
	reopenedPool := singleConnectionPool(t, pool)
	reloaded, err := workflow.NewService(workflow.NewRepository(reopenedPool)).GetExecution(resumeCtx, execution.ID)
	require.NoError(t, err)
	assert.Equal(t, workflow.ExecutionStateCompleted, reloaded.State)
	require.NotNil(t, reloaded.ResumedAt)
	decision, ok := reloaded.ResumeData["decision"].(string)
	require.True(t, ok)
	assert.Contains(t, decision, "approve-")
}

func singleConnectionPool(t *testing.T, source *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(source.Config().ConnString())
	require.NoError(t, err)
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func setupWorkflowResumeSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	schema := "ah_" + workflowResumeIntegrationTenant

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "SET search_path TO "+schema+", public")
	require.NoError(t, err)

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations", "schemas", "000075_workflows.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(migration))
	require.NoError(t, err)

	return pool, tenant.NewContext(ctx, workflowResumeIntegrationTenant)
}
