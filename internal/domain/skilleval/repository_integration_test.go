//go:build integration

package skilleval_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skilleval"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_SkillEvalRepositoryUsesTenantSchemaForLifecycle(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	setupSkillEvalSchema(t, pool, "skillevala")
	setupSkillEvalSchema(t, pool, "skillevalb")

	ctx := tenant.NewContext(context.Background(), "skillevala")
	repo := skilleval.NewRepository(pool)
	suite, err := repo.CreateSuite(ctx, skilleval.EvalSuite{
		SkillID: uuid.New(),
		Name:    "tenant isolated suite",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, suite.ID)

	exists, err := repo.SuiteExistsByName(ctx, suite.SkillID, suite.Name)
	require.NoError(t, err)
	assert.True(t, exists)
	suites, err := repo.ListSuites(ctx, &suite.SkillID)
	require.NoError(t, err)
	require.Len(t, suites, 1)
	foundSuite, err := repo.GetSuiteByID(ctx, suite.ID)
	require.NoError(t, err)
	assert.Equal(t, suite.ID, foundSuite.ID)

	evalCase, err := repo.CreateCase(ctx, skilleval.EvalCase{
		SuiteID:        suite.ID,
		InputText:      "evaluate this",
		ExpectedOutput: "expected",
		GraderType:     skilleval.GraderContains,
		ShouldTrigger:  true,
	})
	require.NoError(t, err)
	cases, err := repo.ListCases(ctx, suite.ID)
	require.NoError(t, err)
	require.Len(t, cases, 1)
	_, err = repo.GetCaseByID(ctx, evalCase.ID)
	require.NoError(t, err)

	run, err := repo.CreateRun(ctx, skilleval.EvalRun{SuiteID: suite.ID, Status: skilleval.RunStatusRunning, TotalCases: 1})
	require.NoError(t, err)
	finishedAt := time.Now()
	run.Status = skilleval.RunStatusPassed
	run.PassedCases = 1
	run.FinishedAt = &finishedAt
	run, err = repo.UpdateRun(ctx, run)
	require.NoError(t, err)
	assert.Equal(t, skilleval.RunStatusPassed, run.Status)
	runs, err := repo.ListRuns(ctx, suite.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	_, err = repo.GetRunByID(ctx, run.ID)
	require.NoError(t, err)

	result, err := repo.CreateCaseResult(ctx, skilleval.CaseResult{RunID: run.ID, CaseID: evalCase.ID, Passed: true, ActualOutput: "expected"})
	require.NoError(t, err)
	results, err := repo.ListCaseResults(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, result.ID, results[0].ID)

	otherTenantCtx := tenant.NewContext(context.Background(), "skillevalb")
	_, err = repo.GetSuiteByID(otherTenantCtx, suite.ID)
	assert.ErrorIs(t, err, skilleval.ErrSuiteNotFound)

	require.NoError(t, repo.DeleteSuite(ctx, suite.ID))
	_, err = repo.GetSuiteByID(ctx, suite.ID)
	assert.ErrorIs(t, err, skilleval.ErrSuiteNotFound)
}

func setupSkillEvalSchema(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	ctx := context.Background()
	testutil.MustExec(t, pool, "CREATE SCHEMA ah_"+tenantID)
	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	t.Cleanup(release)

	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "schemas", "000055_skill_eval.up.sql"))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(sql))
	require.NoError(t, err)
}
