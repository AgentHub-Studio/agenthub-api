//go:build integration

package abtest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/abtest"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const (
	abTestMigrationTenant = "abteststatus"
	abTestRepositoryA     = "abtestrepoa"
	abTestRepositoryB     = "abtestrepob"
)

func TestIntegration_ABTestStatusMigrationRepairsLegacyRowsAndEnforcesLifecycle(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	conn := setupABTestMigrationSchema(t, pool, abTestMigrationTenant)

	ctx := context.Background()
	agentID := uuid.New()
	versionID := uuid.New()
	legacyTestID := uuid.New()
	_, err := conn.Exec(ctx, `INSERT INTO agent (id) VALUES ($1)`, agentID)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO agent_version (id, agent_id, version_number) VALUES ($1, $2, 1)`, versionID, agentID)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `
INSERT INTO agent_ab_test (id, agent_id, name, variant_version_id, status)
VALUES ($1, $2, 'legacy undefined state', $3, 'DRAFT')`, legacyTestID, agentID, versionID)
	require.NoError(t, err)

	execABTestMigration(t, conn, "000084_agent_ab_test_status_constraint.up.sql")
	execABTestMigration(t, conn, "000085_agent_ab_test_single_active.up.sql")

	var status string
	require.NoError(t, conn.QueryRow(ctx, `SELECT status FROM agent_ab_test WHERE id = $1`, legacyTestID).Scan(&status))
	assert.Equal(t, "PAUSED", status)

	_, err = conn.Exec(ctx, `
INSERT INTO agent_ab_test (id, agent_id, name, variant_version_id, status)
VALUES ($1, $2, 'invalid after migration', $3, 'DRAFT')`, uuid.New(), agentID, versionID)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23514", pgErr.Code)
	assert.Equal(t, "agent_ab_test_status_check", pgErr.ConstraintName)

	for _, allowed := range []string{"ACTIVE", "PAUSED", "CONCLUDED"} {
		_, err = conn.Exec(ctx, `
INSERT INTO agent_ab_test (id, agent_id, name, variant_version_id, status)
VALUES ($1, $2, $3, $4, $5)`, uuid.New(), agentID, "allowed "+allowed, versionID, allowed)
		require.NoError(t, err, allowed)
	}
}

func TestIntegration_ABTestSingleActiveMigrationRepairsLegacyRowsAndRejectsDuplicates(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	conn := setupABTestMigrationSchema(t, pool, "abtestsingleactive")
	execABTestMigration(t, conn, "000084_agent_ab_test_status_constraint.up.sql")

	ctx := context.Background()
	agentID, versionID := seedABTestAgentAndVersion(t, conn)
	olderID := uuid.New()
	newerID := uuid.New()
	olderStartedAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	newerStartedAt := olderStartedAt.Add(time.Hour)
	for _, test := range []struct {
		id        uuid.UUID
		name      string
		startedAt time.Time
	}{
		{id: olderID, name: "older active", startedAt: olderStartedAt},
		{id: newerID, name: "newer active", startedAt: newerStartedAt},
	} {
		_, err := conn.Exec(ctx, `
INSERT INTO agent_ab_test (id, agent_id, name, variant_version_id, status, started_at)
VALUES ($1, $2, $3, $4, 'ACTIVE', $5)`, test.id, agentID, test.name, versionID, test.startedAt)
		require.NoError(t, err)
	}

	execABTestMigration(t, conn, "000085_agent_ab_test_single_active.up.sql")

	var olderStatus, newerStatus string
	require.NoError(t, conn.QueryRow(ctx, `SELECT status FROM agent_ab_test WHERE id = $1`, olderID).Scan(&olderStatus))
	require.NoError(t, conn.QueryRow(ctx, `SELECT status FROM agent_ab_test WHERE id = $1`, newerID).Scan(&newerStatus))
	assert.Equal(t, "PAUSED", olderStatus)
	assert.Equal(t, "ACTIVE", newerStatus)

	_, err := conn.Exec(ctx, `
INSERT INTO agent_ab_test (id, agent_id, name, variant_version_id, status)
VALUES ($1, $2, 'third active', $3, 'ACTIVE')`, uuid.New(), agentID, versionID)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	assert.Equal(t, "23505", pgErr.Code)
	assert.Equal(t, "uq_agent_ab_test_active_per_agent", pgErr.ConstraintName)
}

func TestIntegration_ABTestRepositoryUsesTenantSchemaForCRUDAndAssignments(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	connA := setupABTestMigrationSchema(t, pool, abTestRepositoryA)
	connB := setupABTestMigrationSchema(t, pool, abTestRepositoryB)
	execABTestMigration(t, connA, "000084_agent_ab_test_status_constraint.up.sql")
	execABTestMigration(t, connB, "000084_agent_ab_test_status_constraint.up.sql")
	execABTestMigration(t, connA, "000085_agent_ab_test_single_active.up.sql")
	execABTestMigration(t, connB, "000085_agent_ab_test_single_active.up.sql")

	ctx := context.Background()
	agentID, versionID := seedABTestAgentAndVersion(t, connA)
	repo := abtest.NewRepository(pool)
	tenantAContext := tenant.NewContext(ctx, abTestRepositoryA)
	tenantBContext := tenant.NewContext(ctx, abTestRepositoryB)

	created, err := repo.Create(tenantAContext, abtest.ABTest{
		AgentID:          agentID,
		Name:             "tenant A experiment",
		VariantVersionID: versionID,
		TrafficPercent:   40,
		Status:           abtest.TestStatusActive,
		StartedAt:        time.Now(),
	})
	require.NoError(t, err)

	_, err = repo.Create(tenantAContext, abtest.ABTest{
		AgentID:          agentID,
		Name:             "second active experiment",
		VariantVersionID: versionID,
		TrafficPercent:   20,
		Status:           abtest.TestStatusActive,
		StartedAt:        time.Now(),
	})
	assert.ErrorIs(t, err, abtest.ErrActiveTestConflict)

	paused, err := repo.Create(tenantAContext, abtest.ABTest{
		AgentID:          agentID,
		Name:             "paused experiment",
		VariantVersionID: versionID,
		TrafficPercent:   20,
		Status:           abtest.TestStatusPaused,
		StartedAt:        time.Now(),
	})
	require.NoError(t, err)
	paused.Status = abtest.TestStatusActive
	_, err = repo.Update(tenantAContext, paused)
	assert.ErrorIs(t, err, abtest.ErrActiveTestConflict)

	page, err := repo.List(tenantAContext, agentID, pagination.PageRequest{Page: 0, Size: 10})
	require.NoError(t, err)
	require.Len(t, page.Content, 2)

	found, err := repo.GetByID(tenantAContext, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "tenant A experiment", found.Name)

	found.Name = "tenant A updated"
	found.TrafficPercent = 55
	found.Status = abtest.TestStatusPaused
	updated, err := repo.Update(tenantAContext, found)
	require.NoError(t, err)
	assert.Equal(t, "tenant A updated", updated.Name)
	assert.Equal(t, 55, updated.TrafficPercent)
	assert.Equal(t, abtest.TestStatusPaused, updated.Status)

	found.Status = abtest.TestStatusActive
	_, err = repo.Update(tenantAContext, found)
	require.NoError(t, err)
	active, err := repo.GetActiveByAgent(tenantAContext, agentID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, active.ID)

	require.NoError(t, repo.RecordAssignment(tenantAContext, abtest.Assignment{
		TestID:    created.ID,
		SessionID: uuid.New(),
		Variant:   abtest.VariantVariant,
	}))
	var assignments int
	require.NoError(t, connA.QueryRow(ctx, `SELECT COUNT(*) FROM agent_ab_assignment WHERE test_id = $1`, created.ID).Scan(&assignments))
	assert.Equal(t, 1, assignments)

	_, err = repo.GetByID(tenantBContext, created.ID)
	assert.ErrorIs(t, err, abtest.ErrNotFound)

	require.NoError(t, repo.Delete(tenantAContext, created.ID))
	_, err = repo.GetByID(tenantAContext, created.ID)
	assert.ErrorIs(t, err, abtest.ErrNotFound)
}

func TestIntegration_ABTestHandlerUsesTenantRepositoryAndNestedParentScope(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	conn := setupABTestMigrationSchema(t, pool, abTestRepositoryA)
	execABTestMigration(t, conn, "000084_agent_ab_test_status_constraint.up.sql")
	execABTestMigration(t, conn, "000085_agent_ab_test_single_active.up.sql")
	agentID, versionID := seedABTestAgentAndVersion(t, conn)
	otherAgentID, _ := seedABTestAgentAndVersion(t, conn)

	h := abtest.NewHandler(abtest.NewService(abtest.NewRepository(pool)))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), abTestRepositoryA)
			ctx = middleware.ContextWithRoles(ctx, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)

	body := `{"name":"real handler","variantVersionId":"` + versionID.String() + `","trafficPercent":25}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/ab-tests", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	r.ServeHTTP(createRes, createReq)
	require.Equal(t, http.StatusCreated, createRes.Code, createRes.Body.String())

	var created abtest.ABTestResponse
	require.NoError(t, json.Unmarshal(createRes.Body.Bytes(), &created))
	assert.Equal(t, agentID, created.AgentID)

	secondCreateReq := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/ab-tests", bytes.NewBufferString(`{"name":"second real handler","variantVersionId":"`+versionID.String()+`","trafficPercent":25}`))
	secondCreateReq.Header.Set("Content-Type", "application/json")
	secondCreateRes := httptest.NewRecorder()
	r.ServeHTTP(secondCreateRes, secondCreateReq)
	assert.Equal(t, http.StatusConflict, secondCreateRes.Code, secondCreateRes.Body.String())

	listReq := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/ab-tests", nil)
	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, listReq)
	require.Equal(t, http.StatusOK, listRes.Code, listRes.Body.String())

	wrongParentReq := httptest.NewRequest(http.MethodGet, "/api/agents/"+otherAgentID.String()+"/ab-tests/"+created.ID.String(), nil)
	wrongParentRes := httptest.NewRecorder()
	r.ServeHTTP(wrongParentRes, wrongParentReq)
	assert.Equal(t, http.StatusNotFound, wrongParentRes.Code, wrongParentRes.Body.String())
}

func setupABTestMigrationSchema(t *testing.T, pool *pgxpool.Pool, tenantID string) *pgxpool.Conn {
	t.Helper()
	ctx := context.Background()
	testutil.MustExec(t, pool, "CREATE SCHEMA ah_"+tenantID)
	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	t.Cleanup(release)
	_, err = conn.Exec(ctx, `CREATE TABLE agent (id UUID PRIMARY KEY)`)
	require.NoError(t, err)
	execABTestMigration(t, conn, "000056_agent_ab_test.up.sql")
	return conn
}

func seedABTestAgentAndVersion(t *testing.T, conn *pgxpool.Conn) (uuid.UUID, uuid.UUID) {
	t.Helper()
	agentID := uuid.New()
	versionID := uuid.New()
	_, err := conn.Exec(context.Background(), `INSERT INTO agent (id) VALUES ($1)`, agentID)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), `INSERT INTO agent_version (id, agent_id, version_number) VALUES ($1, $2, 1)`, versionID, agentID)
	require.NoError(t, err)
	return agentID, versionID
}

func execABTestMigration(t *testing.T, conn *pgxpool.Conn, name string) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "migrations", "schemas", name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), string(sql))
	require.NoError(t, err, name)
}
