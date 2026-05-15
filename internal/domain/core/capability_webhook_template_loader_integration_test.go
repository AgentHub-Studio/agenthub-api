//go:build integration

package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreCapabilityWebhookTemplateLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000097 creates webhook_notification_template table + 3 capability rows
//
// The ah_core.webhook_notification_template table is created by migration 000097 itself
// (no prior migration defines it). The table is distinct from webhook_endpoint_template
// (migration 000021) which catalogs outbound delivery destinations.

const capabilityWebhookMigration = "000097_seed_capability_webhook_templates.up.sql"
const capabilityWebhookMigrationDown = "000097_seed_capability_webhook_templates.down.sql"

func TestIntegration_Capability_LoadWebhookTemplates_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityWebhookMigration)

	loader := core.NewCoreCapabilityWebhookTemplateLoader(pool)
	got, err := loader.LoadCapabilityWebhookTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityWebhookTemplateCount, len(got),
		"DB row count must match SeedCapabilityWebhookTemplateCount (3) after migration 000097")
}

func TestIntegration_Capability_WebhookTemplateMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityWebhookMigration)
	// Apply 000097 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityWebhookMigration)

	loader := core.NewCoreCapabilityWebhookTemplateLoader(pool)
	got, err := loader.LoadCapabilityWebhookTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityWebhookTemplateCount, len(got),
		"double-apply of migration 000097 must still produce exactly 3 capability webhook templates (idempotent)")
}

func TestIntegration_Capability_WebhookTemplateDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityWebhookMigration)

	loader := core.NewCoreCapabilityWebhookTemplateLoader(pool)
	before, err := loader.LoadCapabilityWebhookTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityWebhookMigrationDown)

	after, err := loader.LoadCapabilityWebhookTemplates(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 3 capability webhook templates and drop the table")
}

func TestIntegration_Capability_WebhookTemplateNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityWebhookTemplateLoader(pool)
	got, err := loader.LoadCapabilityWebhookTemplates(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_AllWebhookTemplateSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityWebhookMigration)

	loader := core.NewCoreCapabilityWebhookTemplateLoader(pool)
	got, err := loader.LoadCapabilityWebhookTemplates(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3, "precondition: 3 capability webhook templates loaded")

	// Verify every loaded slug appears in the canonical Go list.
	canonicalSet := map[string]bool{}
	for _, s := range core.SeedCapabilityWebhookTemplateSlugs {
		canonicalSet[s] = true
	}
	for _, tmpl := range got {
		assert.True(t, canonicalSet[tmpl.Slug],
			"DB slug %q must be in SeedCapabilityWebhookTemplateSlugs (Go constant list)",
			tmpl.Slug)
	}
}
