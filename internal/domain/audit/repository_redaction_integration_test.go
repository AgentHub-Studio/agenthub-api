//go:build integration

package audit_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const auditRedactionIntegrationTenant = "auditredaction"

func TestIntegration_AuditProjectionRedactsPersistedSensitiveValues(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	setupAuditRedactionSchema(t, pool)

	ctx := tenant.NewContext(context.Background(), auditRedactionIntegrationTenant)
	repo := audit.NewRepository(pool)
	stored, err := repo.Record(ctx, auditRedactionIntegrationTenant, audit.AuditLog{
		EntityType: "agent",
		EntityID:   "agent-1",
		Action:     audit.AuditActionUpdate,
		ActorID:    "admin-1",
		ActorEmail: "admin@example.test",
		OldValue:   `{"apiKey":"audit-old-integration-secret","safe":"before"}`,
		NewValue:   `{"Authorization":"Bearer audit-new-integration-secret","safe":"after"}`,
		Metadata:   `{"trace":"audit-integration-trace","nested":{"refresh_token":"audit-refresh-integration-secret"}}`,
		IPAddress:  "127.0.0.1",
	})
	require.NoError(t, err)

	items, total, err := repo.ListAll(ctx, auditRedactionIntegrationTenant, audit.ListFilter{}, pagination.PageRequest{Page: 0, Size: 10})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Contains(t, items[0].OldValue, "audit-old-integration-secret")
	require.Contains(t, items[0].NewValue, "audit-new-integration-secret")
	require.Contains(t, items[0].Metadata, "audit-refresh-integration-secret")

	public := audit.ResponseFrom(items[0])
	encoded, err := json.Marshal(public)
	require.NoError(t, err)
	for _, secret := range []string{
		"audit-old-integration-secret",
		"audit-new-integration-secret",
		"audit-refresh-integration-secret",
		"apiKey",
		"Authorization",
		"refresh_token",
	} {
		assert.NotContains(t, string(encoded), secret)
	}
	assert.Contains(t, public.OldValue, "before")
	assert.Contains(t, public.NewValue, "after")
	assert.Contains(t, public.Metadata, "audit-integration-trace")

	assert.True(t, strings.Contains(stored.OldValue, "audit-old-integration-secret"))
	assert.True(t, strings.Contains(stored.NewValue, "audit-new-integration-secret"))
	assert.True(t, strings.Contains(stored.Metadata, "audit-refresh-integration-secret"))
}

func setupAuditRedactionSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS ah_"+auditRedactionIntegrationTenant)
	testutil.MustExec(t, pool, `
		CREATE TABLE ah_auditredaction.audit_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			entity_type VARCHAR(100) NOT NULL,
			entity_id VARCHAR(255) NOT NULL,
			action VARCHAR(100) NOT NULL,
			actor_id VARCHAR(255),
			actor_email TEXT,
			old_value JSONB,
			new_value JSONB,
			metadata JSONB,
			ip_address VARCHAR(50),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
}
