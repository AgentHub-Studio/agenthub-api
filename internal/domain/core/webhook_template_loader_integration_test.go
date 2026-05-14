//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_CoreWebhookTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedWebhookTemplateSlugs), len(got))
}

func TestIntegration_CoreWebhookTemplate_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreWebhookTemplate_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "slack-incoming-webhook")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "slack", tmpl.TargetKind)
	assert.Equal(t, "POST", tmpl.Method)
	assert.True(t, tmpl.IsRecommended)
}

func TestIntegration_CoreWebhookTemplate_LoadByTargetKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadByTargetKind(context.Background(), "slack")
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
}

func TestIntegration_CoreWebhookTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedWebhookTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CoreWebhookTemplate_AllTargetKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedWebhookTemplateTargetKinds {
		allowed[k] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.TargetKind])
	}
}

func TestIntegration_CoreWebhookTemplate_AllRetryPoliciesAreParseable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		retries := tmpl.RetryPolicyMs()
		assert.GreaterOrEqual(t, len(retries), 1,
			"template %q must have ≥1 retry attempt", tmpl.Slug)
		// Verify ascending (exponential backoff convention).
		for i := 1; i < len(retries); i++ {
			assert.GreaterOrEqual(t, retries[i], retries[i-1],
				"%q retry %d must be ≥ retry %d (backoff)", tmpl.Slug, i, i-1)
		}
	}
}

func TestIntegration_CoreWebhookTemplate_SMSEmailHaveLongerBackoffs(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		if tmpl.TargetKind != "sms" && tmpl.TargetKind != "email" {
			continue
		}
		retries := tmpl.RetryPolicyMs()
		require.NotEmpty(t, retries)
		assert.GreaterOrEqual(t, retries[0], 5000,
			"%q first retry must be ≥5s (cost-sensitive channel)", tmpl.Slug)
	}
}

func TestIntegration_CoreWebhookTemplate_AllPayloadTemplatesReferencePlaceholders(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.Contains(t, tmpl.PayloadTemplate, "{{",
			"template %q payload must use {{...}} placeholders", tmpl.Slug)
	}
}

func TestIntegration_CoreWebhookTemplate_AllUseHTTPSOrSMTPInProduction(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		// URL templates with {{...}} placeholders are tenant-supplied;
		// hardcoded URLs (PagerDuty, Opsgenie) must be HTTPS.
		if !strings.Contains(tmpl.URLTemplate, "{{") {
			assert.True(t,
				strings.HasPrefix(tmpl.URLTemplate, "https://") ||
					strings.HasPrefix(tmpl.URLTemplate, "smtp://"),
				"hardcoded URL for %q must be HTTPS or SMTP (got %q)", tmpl.Slug, tmpl.URLTemplate)
		}
	}
}

func TestIntegration_CoreWebhookTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreWebhookTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30)
	}
}

func TestIntegration_CoreWebhookTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreWebhookTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")

	loader := core.NewCoreWebhookEndpointTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedWebhookTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
