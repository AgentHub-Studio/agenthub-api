//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_CoreWorkflowTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedWorkflowTemplateSlugs), len(got))
}

func TestIntegration_CoreWorkflowTemplate_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreWorkflowTemplate_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "code-review")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "review", tmpl.WorkflowKind)
	assert.True(t, tmpl.RequiresHumanCheckpoint, "code-review gates on checkpoint")
	assert.True(t, tmpl.IsRecommended)
	assert.Equal(t, "code-reviewer", tmpl.TargetAgentSlug,
		"code-review uses code-reviewer prompt template (cross-table FK)")
}

func TestIntegration_CoreWorkflowTemplate_LoadByKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadByKind(context.Background(), "extraction")
	require.NoError(t, err)
	assert.Equal(t, 2, len(got), "2 extraction workflows: document-summary + invoice-processing")
}

func TestIntegration_CoreWorkflowTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedWorkflowTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CoreWorkflowTemplate_HumanCheckpointSetMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbCheckpoint := []string{}
	for _, tmpl := range got {
		if tmpl.RequiresHumanCheckpoint {
			dbCheckpoint = append(dbCheckpoint, tmpl.Slug)
		}
	}
	sort.Strings(dbCheckpoint)
	expected := append([]string{}, core.SeedHumanCheckpointWorkflowTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbCheckpoint,
		"DB checkpoint set must EXACTLY match SeedHumanCheckpointWorkflowTemplateSlugs")
}

func TestIntegration_CoreWorkflowTemplate_AllKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedWorkflowTemplateKinds {
		allowed[k] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.WorkflowKind])
	}
}

func TestIntegration_CoreWorkflowTemplate_AllEstimatesArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.Greater(t, tmpl.EstimatedSteps, 0,
			"workflow %q estimated_steps must be > 0", tmpl.Slug)
		assert.GreaterOrEqual(t, tmpl.EstimatedCostUSD, 0.0)
	}
}

func TestIntegration_CoreWorkflowTemplate_TargetAgentSlugIsAlwaysPresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.NotEmpty(t, tmpl.TargetAgentSlug,
			"workflow %q must reference a target agent", tmpl.Slug)
	}
}

func TestIntegration_CoreWorkflowTemplate_AllRequireAtLeastOneSkill(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		skills := tmpl.RequiresSkillsList()
		assert.NotEmpty(t, skills,
			"workflow %q must require at least 1 skill — abstract workflow useless", tmpl.Slug)
	}
}

func TestIntegration_CoreWorkflowTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreWorkflowTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30)
	}
}

func TestIntegration_CoreWorkflowTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreWorkflowTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedWorkflowTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreWorkflowTemplate_ComplianceWorkflowGatesOnCheckpoint(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "compliance-export")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, tmpl.RequiresHumanCheckpoint,
		"compliance-export MUST gate on human checkpoint (audit data export)")
}

func TestIntegration_CoreWorkflowTemplate_RecommendedExcludesCheckpointHeavyWorkflows(t *testing.T) {
	// Recommended = one-click safe; checkpoint-required workflows
	// are NOT one-click (extra step).
	// EXCEPTION: code-review is recommended AND checkpoint-required
	// — explicit accept that good code review is worth the step.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")

	loader := core.NewCoreWorkflowTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	checkpointSet := map[string]bool{}
	for _, s := range core.SeedHumanCheckpointWorkflowTemplateSlugs {
		checkpointSet[s] = true
	}
	for _, tmpl := range got {
		if tmpl.Slug == "code-review" {
			continue // documented exception
		}
		assert.False(t, checkpointSet[tmpl.Slug],
			"recommended %q must be one-click (no checkpoint) — except code-review", tmpl.Slug)
	}
}
