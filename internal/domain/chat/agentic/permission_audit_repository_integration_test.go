//go:build integration

package agentic

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_PermissionAuditRepositoryUsesTenantSchema(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	setupPermissionAuditSchema(t, pool, "permissionaudita")
	setupPermissionAuditSchema(t, pool, "permissionauditb")

	ctx := tenant.NewContext(context.Background(), "permissionaudita")
	repo := NewPermissionAuditRepository(pool)
	sessionID := uuid.New()
	require.NoError(t, repo.LogDecision(ctx, PermissionAuditEntry{
		SessionID: sessionID,
		ToolName:  "Read",
		Decision:  AuditDecisionAllow,
	}))
	entries, err := repo.ListBySession(ctx, sessionID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	otherEntries, err := repo.ListBySession(tenant.NewContext(context.Background(), "permissionauditb"), sessionID, 10)
	require.NoError(t, err)
	require.Empty(t, otherEntries)
}

func setupPermissionAuditSchema(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	ctx := context.Background()
	testutil.MustExec(t, pool, "CREATE SCHEMA ah_"+tenantID)
	conn, release, err := database.AcquireWithTenant(ctx, pool, tenantID)
	require.NoError(t, err)
	t.Cleanup(release)
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "schemas", "000052_permission_audit_log.up.sql"))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(sql))
	require.NoError(t, err)
}
