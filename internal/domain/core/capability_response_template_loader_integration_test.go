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

// Integration tests for CoreCapabilityResponseTemplateLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000116 creates capability_response_template table + 9 rows
//
// The ah_core.capability_response_template table is created by migration 000116
// itself (no prior migration defines it).

const capabilityResponseTemplateMigration = "000116_seed_capability_response_templates.up.sql"
const capabilityResponseTemplateMigrationDown = "000116_seed_capability_response_templates.down.sql"

func TestIntegration_ResponseTemplate_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigration)

	loader := core.NewCoreCapabilityResponseTemplateLoader(pool)
	got, err := loader.LoadCapabilityResponseTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedResponseTemplateCount, len(got),
		"DB row count must match SeedResponseTemplateCount (9) after migration 000116")
}

func TestIntegration_ResponseTemplate_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityResponseTemplateLoader(pool)
	got, err := loader.LoadCapabilityResponseTemplates(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_ResponseTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigration)
	// Apply 000116 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigration)

	loader := core.NewCoreCapabilityResponseTemplateLoader(pool)
	got, err := loader.LoadCapabilityResponseTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedResponseTemplateCount, len(got),
		"double-apply of migration 000116 must still produce exactly 9 response template rows (idempotent)")
}

func TestIntegration_ResponseTemplate_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigration)

	loader := core.NewCoreCapabilityResponseTemplateLoader(pool)
	before, err := loader.LoadCapabilityResponseTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed must produce response template rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigrationDown)

	after, err := loader.LoadCapabilityResponseTemplates(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 response template rows and drop the table")
}

func TestIntegration_ResponseTemplate_GetGreetingForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigration)

	loader := core.NewCoreCapabilityResponseTemplateLoader(pool)
	tmpl, err := loader.GetResponseTemplate(context.Background(), "core-researcher", core.SeedTemplateKeyGreeting)
	require.NoError(t, err)

	require.NotNil(t, tmpl,
		"GetResponseTemplate must return non-nil for core-researcher greeting after migration 000116")
	assert.Equal(t, "core-researcher", tmpl.AgentSlug,
		"returned template must belong to core-researcher")
	assert.Equal(t, core.SeedTemplateKeyGreeting, tmpl.TemplateKey,
		"returned template must have key 'greeting'")
	assert.NotEmpty(t, tmpl.TemplateBody,
		"greeting template body must not be empty")
	assert.False(t, tmpl.HasPlaceholders,
		"greeting template must have has_placeholders=FALSE (rendered verbatim)")
}

func TestIntegration_ResponseTemplate_TemplatesWithPlaceholdersHaveBraces(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityResponseTemplateMigration)

	loader := core.NewCoreCapabilityResponseTemplateLoader(pool)
	all, err := loader.LoadCapabilityResponseTemplates(context.Background())
	require.NoError(t, err)
	require.Len(t, all, core.SeedResponseTemplateCount,
		"precondition: all 9 templates must be loaded")

	placeholderCount := 0
	for _, tmpl := range all {
		if tmpl.HasPlaceholders {
			placeholderCount++
			assert.Contains(t, tmpl.TemplateBody, "{",
				"template %q/%q has has_placeholders=TRUE but body contains no '{' token",
				tmpl.AgentSlug, tmpl.TemplateKey)
		}
	}
	assert.Equal(t, core.SeedResponseTemplatesWithPlaceholdersCount, placeholderCount,
		"exactly SeedResponseTemplatesWithPlaceholdersCount (%d) templates must have has_placeholders=TRUE",
		core.SeedResponseTemplatesWithPlaceholdersCount)
}
