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

const relMilestoneMigration = "000027_seed_relationship_milestone_templates.up.sql"
const relMilestoneMigrationDown = "000027_seed_relationship_milestone_templates.down.sql"

func TestIntegration_CoreRelMilestoneTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedRelMilestoneTemplateRowCount, len(got),
		"expected 8 milestone templates after seed migration")
}

func TestIntegration_CoreRelMilestoneTemplate_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreRelMilestoneTemplate_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "established-after-ten-positive")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "established", tmpl.TargetTrustLevel)
	assert.Equal(t, 10, tmpl.MinInteractionCount)
	assert.InDelta(t, 0.0, tmpl.MinRapportScore, 1e-9)
	assert.Equal(t, "elevate_communication_style", tmpl.OnReachAction)
	assert.True(t, tmpl.IsRecommended)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreRelMilestoneTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreRelMilestoneTemplate_LoadByTargetTrustLevel_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	// 1 promotion + 4 record-* templates → 5 probationary.
	prob, err := loader.LoadByTargetTrustLevel(context.Background(), "probationary")
	require.NoError(t, err)
	assert.Equal(t, 5, len(prob), "1 promotion + 4 recording templates target probationary")

	est, err := loader.LoadByTargetTrustLevel(context.Background(), "established")
	require.NoError(t, err)
	assert.Equal(t, 1, len(est))

	tr, err := loader.LoadByTargetTrustLevel(context.Background(), "trusted")
	require.NoError(t, err)
	assert.Equal(t, 1, len(tr))

	mt, err := loader.LoadByTargetTrustLevel(context.Background(), "mistrusted")
	require.NoError(t, err)
	assert.Equal(t, 1, len(mt))
}

func TestIntegration_CoreRelMilestoneTemplate_LoadByOnReachAction_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	// no_action: first-interaction-probationary, record-positive,
	// record-conflict-resolutions = 3.
	noaction, err := loader.LoadByOnReachAction(context.Background(), "no_action")
	require.NoError(t, err)
	assert.Equal(t, 3, len(noaction))

	notif, err := loader.LoadByOnReachAction(context.Background(), "send_admin_notification")
	require.NoError(t, err)
	assert.Equal(t, 2, len(notif),
		"trusted-after-fifty + record-negative-and-alert")

	tickets, err := loader.LoadByOnReachAction(context.Background(), "open_review_ticket")
	require.NoError(t, err)
	assert.Equal(t, 2, len(tickets),
		"mistrusted + record-escalation-and-open-ticket")
}

func TestIntegration_CoreRelMilestoneTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	got := []string{}
	for _, t := range rec {
		got = append(got, t.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedRelMilestoneTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got,
		"DB recommended set must equal Go SeedRecommendedRelMilestoneTemplateSlugs")
}

func TestIntegration_CoreRelMilestoneTemplate_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	got := []string{}
	for _, t := range all {
		if t.RequiresAdminReview {
			got = append(got, t.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewRelMilestoneTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreRelMilestoneTemplate_AllTrustLevelsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, l := range core.SeedExpectedRelMilestoneTemplateTrustLevels {
		allowed[l] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.TargetTrustLevel],
			"template %q has level %q outside expected set",
			tmpl.Slug, tmpl.TargetTrustLevel)
	}
}

func TestIntegration_CoreRelMilestoneTemplate_AllActionsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, a := range core.SeedExpectedRelMilestoneTemplateActions {
		allowed[a] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.OnReachAction],
			"template %q has action %q outside expected set",
			tmpl.Slug, tmpl.OnReachAction)
	}
}

func TestIntegration_CoreRelMilestoneTemplate_AllTriggerEventKindsAreInExpectedSet(t *testing.T) {
	// Cross-row invariant: every event kind ANY template references must
	// be in the FUTURE-002 RapportEvent enum (no orphan event labels).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedRelMilestoneTemplateEventKinds {
		allowed[k] = true
	}
	for _, tmpl := range all {
		for _, k := range tmpl.TriggersEventKindsList() {
			assert.True(t, allowed[k],
				"template %q references event_kind %q outside FUTURE-002 enum",
				tmpl.Slug, k)
		}
	}
}

func TestIntegration_CoreRelMilestoneTemplate_PromotionsAlignWithFutureTwoThresholds(t *testing.T) {
	// FUTURE-002 documented: 1+ event = probationary, 10+ rapport≥0
	// = established, 50+ rapport≥0.5 = trusted. Ensure the seed
	// encodes those exact values (no drift between docs and seed).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)

	prob, _, _ := loader.FindBySlug(context.Background(), "first-interaction-probationary")
	assert.Equal(t, 1, prob.MinInteractionCount, "1+ event")

	est, _, _ := loader.FindBySlug(context.Background(), "established-after-ten-positive")
	assert.Equal(t, 10, est.MinInteractionCount, "10+ interactions")
	assert.InDelta(t, 0.0, est.MinRapportScore, 1e-9, "rapport ≥0")

	tr, _, _ := loader.FindBySlug(context.Background(), "trusted-after-fifty-strong-rapport")
	assert.Equal(t, 50, tr.MinInteractionCount, "50+ interactions")
	assert.InDelta(t, 0.5, tr.MinRapportScore, 1e-9, "rapport ≥0.5")
}

func TestIntegration_CoreRelMilestoneTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range all {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreRelMilestoneTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30,
			"template %q description must be informative (≥30 chars)", tmpl.Slug)
	}
}

func TestIntegration_CoreRelMilestoneTemplate_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].Slug, all[i].Slug)
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder)
		}
	}
	assert.Equal(t, "first-interaction-probationary", all[0].Slug,
		"first by sort_order=10 must be first-interaction-probationary")
}

func TestIntegration_CoreRelMilestoneTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedRelMilestoneTemplateRowCount, len(got))
}

func TestIntegration_CoreRelMilestoneTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)
	applyMigration(t, pool, migDir, relMilestoneMigrationDown)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "loader must remain non-fatal after table drop")
	assert.Empty(t, got)
}

func TestIntegration_CoreRelMilestoneTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, tmpl := range all {
		got = append(got, tmpl.Slug)
	}
	expected := append([]string{}, core.SeedExpectedRelMilestoneTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got,
		"DB slug set must equal SeedExpectedRelMilestoneTemplateSlugs")
}

func TestIntegration_CoreRelMilestoneTemplate_RecordingTemplatesUseProbationaryDefault(t *testing.T) {
	// Record-only templates do not promote — they stay at probationary
	// as a sentinel target so dispatcher knows "do not promote", just
	// record the event.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	for _, slug := range []string{
		"record-positive-acknowledgements",
		"record-conflict-resolutions",
		"record-negative-and-alert",
		"record-escalation-and-open-ticket",
	} {
		tmpl, found, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		require.True(t, found, "missing %s", slug)
		assert.Equal(t, "probationary", tmpl.TargetTrustLevel,
			"%s must use probationary sentinel for recording-only", slug)
		assert.Equal(t, 0, tmpl.MinInteractionCount,
			"%s must allow recording at any interaction count", slug)
	}
}

func TestIntegration_CoreRelMilestoneTemplate_TriggerEventLabelsLowerSnakeCase(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, relMilestoneMigration)

	loader := core.NewCoreRelationshipMilestoneTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		for _, k := range tmpl.TriggersEventKindsList() {
			assert.Equal(t, strings.ToLower(k), k,
				"event %q in %s must be lowercase", k, tmpl.Slug)
			assert.NotContains(t, k, "-",
				"event %q in %s must use snake_case (no hyphens)", k, tmpl.Slug)
		}
	}
}
