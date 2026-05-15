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

func TestIntegration_CoreNotifRoute_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedNotificationRouteTemplateSlugs), len(got))
}

func TestIntegration_CoreNotifRoute_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreNotifRoute_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "runs-to-slack")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "run_complete", tmpl.TriggerEvent)
	assert.Equal(t, "slack-incoming-webhook", tmpl.TargetWebhookTemplateSlug)
	assert.Equal(t, 60, tmpl.AggregationWindowSeconds, "60s aggregation reduces spam")
	assert.True(t, tmpl.IsRecommended)
}

func TestIntegration_CoreNotifRoute_LoadByTriggerEvent_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadByTriggerEvent(context.Background(), "checkpoint_pending")
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
}

func TestIntegration_CoreNotifRoute_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedNotificationRouteTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CoreNotifRoute_AllTriggersAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, e := range core.SeedExpectedNotificationRouteTriggerEvents {
		allowed[e] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.TriggerEvent])
	}
}

func TestIntegration_CoreNotifRoute_AllSeveritiesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedNotificationRouteSeverities {
		allowed[s] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.MinSeverity], "severity %q outside set", tmpl.MinSeverity)
	}
}

func TestIntegration_CoreNotifRoute_PagerDutyRoutesUseCriticalOrWarnSeverity(t *testing.T) {
	// PagerDuty pages oncall — only critical (or warn for timeouts).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		if tmpl.TargetWebhookTemplateSlug != "pagerduty-events-v2" {
			continue
		}
		assert.True(t,
			tmpl.MinSeverity == "critical" || tmpl.MinSeverity == "warn",
			"PagerDuty route %q must filter to critical/warn (got %q)",
			tmpl.Slug, tmpl.MinSeverity)
	}
}

func TestIntegration_CoreNotifRoute_TargetWebhookTemplateExistsInWebhookCatalog(t *testing.T) {
	// Cross-table integrity: every target_webhook_template_slug must
	// reference an existing ah_core.webhook_endpoint_template.slug.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000021_seed_webhook_endpoint_templates.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	routeLoader := core.NewCoreNotificationRouteTemplateLoader(pool)
	webhookLoader := core.NewCoreWebhookEndpointTemplateLoader(pool)

	routes, _ := routeLoader.LoadAll(context.Background())
	webhooks, _ := webhookLoader.LoadAll(context.Background())

	webhookSlugs := map[string]bool{}
	for _, w := range webhooks {
		webhookSlugs[w.Slug] = true
	}
	for _, r := range routes {
		assert.True(t, webhookSlugs[r.TargetWebhookTemplateSlug],
			"route %q references webhook %q that does not exist",
			r.Slug, r.TargetWebhookTemplateSlug)
	}
}

func TestIntegration_CoreNotifRoute_RouteNamesFollowSourceToDestinationPattern(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.True(t, strings.Contains(tmpl.Slug, "-to-"),
			"route %q must follow source-to-destination pattern", tmpl.Slug)
	}
}

func TestIntegration_CoreNotifRoute_AuditEventsHaveNoRateLimit(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "audit-events-to-webhook")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 0, tmpl.MaxPerHour,
		"audit must capture every event — no rate limit (compliance contract)")
}

func TestIntegration_CoreNotifRoute_AggregationRequiresPositiveWindow(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, tmpl.AggregationWindowSeconds, 0,
			"aggregation window must be ≥0 (0=disabled)")
	}
}

func TestIntegration_CoreNotifRoute_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreNotifRoute_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30)
	}
}

func TestIntegration_CoreNotifRoute_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreNotifRoute_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000022_seed_notification_route_templates.up.sql")

	loader := core.NewCoreNotificationRouteTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedNotificationRouteTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
