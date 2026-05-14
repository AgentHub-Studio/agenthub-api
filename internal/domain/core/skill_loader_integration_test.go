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

// Integration tests for CoreSkillLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// BACKFILL — migration 000003_seed_skills predates the testcontainers
// pattern; this file closes the audit gap.

func TestIntegration_CoreSkill_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedSkillSlugs), len(got),
		"DB row count must match SeedExpectedSkillSlugs canonical count")

	gotSlugs := map[string]core.CoreSkill{}
	for _, s := range got {
		gotSlugs[s.Slug] = s
	}
	for _, slug := range core.SeedExpectedSkillSlugs {
		assert.Contains(t, gotSlugs, slug, "DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreSkill_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got)
}

func TestIntegration_CoreSkill_FindBySlug_ReturnsKnownSkill(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	skill, found, err := loader.FindBySlug(context.Background(), "core-agents-management")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "core-agents-management", skill.Slug)
	assert.Equal(t, "platform", skill.Category)
	assert.Equal(t, "inline", skill.ContextMode)
	assert.NotEmpty(t, skill.Description)
}

func TestIntegration_CoreSkill_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSkill_AllSkillsUsePlatformCategory(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.Equal(t, "platform", s.Category,
			"DB skill %q must be category=platform (got %q) — seed contract",
			s.Slug, s.Category)
	}
}

func TestIntegration_CoreSkill_AllSkillsUseInlineContextMode(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.Equal(t, "inline", s.ContextMode,
			"skill %q must use inline context mode", s.Slug)
	}
}

func TestIntegration_CoreSkill_AllSkillsUseCorePrefix(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.True(t, strings.HasPrefix(s.Slug, "core-"),
			"slug %q must start with core-", s.Slug)
	}
}

func TestIntegration_CoreSkill_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, s := range got {
		assert.False(t, seen[s.Slug], "duplicate slug %q in DB", s.Slug)
		seen[s.Slug] = true
	}
}

func TestIntegration_CoreSkill_AllInstructionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.NotEmpty(t, s.Instructions,
			"skill %q must have non-empty instructions (LLM uses these)", s.Slug)
		assert.GreaterOrEqual(t, len(s.Instructions), 30,
			"skill %q instructions too short (%d chars)", s.Slug, len(s.Instructions))
	}
}

func TestIntegration_CoreSkill_DisableModelInvocationDefaultIsFalse(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, s := range got {
		assert.False(t, s.DisableModelInvocation,
			"skill %q must NOT disable model invocation — platform skills are LLM-callable", s.Slug)
	}
}

func TestIntegration_CoreSkill_LoadToolBindings_ReturnsExpectedCount(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	bindings, err := loader.LoadToolBindings(context.Background())
	require.NoError(t, err)

	// Count must match canonical constant. If migration changed, update
	// the constant; do NOT silently let the count drift.
	assert.Equal(t, core.SeedExpectedSkillBindingsCount, len(bindings),
		"skill→tool binding count must match SeedExpectedSkillBindingsCount")
}

func TestIntegration_CoreSkill_EverySkillHasAtLeastOneToolBinding(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	skills, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	bindings, err := loader.LoadToolBindings(context.Background())
	require.NoError(t, err)

	bindingsBySkillID := map[string]int{}
	for _, b := range bindings {
		bindingsBySkillID[b.SkillID.String()]++
	}
	for _, s := range skills {
		count := bindingsBySkillID[s.ID.String()]
		assert.Greater(t, count, 0,
			"skill %q has 0 tool bindings — abstract skill cannot do work", s.Slug)
	}
}

func TestIntegration_CoreSkill_ToolBindingsReferenceExistingTools(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	skillLoader := core.NewCoreSkillLoader(pool)
	toolLoader := core.NewCoreToolLoader(pool)

	bindings, err := skillLoader.LoadToolBindings(context.Background())
	require.NoError(t, err)
	tools, err := toolLoader.LoadAll(context.Background())
	require.NoError(t, err)

	toolIDs := map[string]bool{}
	for _, tl := range tools {
		toolIDs[tl.ID.String()] = true
	}
	for _, b := range bindings {
		assert.True(t, toolIDs[b.ToolID.String()],
			"binding references tool_id %s that does not exist in ah_core.tool", b.ToolID)
	}
}

func TestIntegration_CoreSkill_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")
	// Re-apply must NOT duplicate.
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedSkillSlugs), len(got),
		"second-apply must not duplicate rows")

	bindings, err := loader.LoadToolBindings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSkillBindingsCount, len(bindings),
		"second-apply must not duplicate bindings (ON CONFLICT DO NOTHING)")
}

func TestIntegration_CoreSkill_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, s := range got {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedSkillSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedSkillSlugs")
}

func TestIntegration_CoreSkill_BindingsAreOrderedByPriorityWithinSkill(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000002_seed_tools.up.sql")
	applyMigration(t, pool, migDir, "000003_seed_skills.up.sql")

	loader := core.NewCoreSkillLoader(pool)
	bindings, err := loader.LoadToolBindings(context.Background())
	require.NoError(t, err)

	// Group by skill_id and verify priority is non-decreasing within group.
	bySkill := map[string][]core.CoreSkillToolBinding{}
	for _, b := range bindings {
		bySkill[b.SkillID.String()] = append(bySkill[b.SkillID.String()], b)
	}
	for skillID, bs := range bySkill {
		for i := 1; i < len(bs); i++ {
			assert.LessOrEqual(t, bs[i-1].Priority, bs[i].Priority,
				"skill %s: priority must be non-decreasing within group", skillID)
		}
	}
}
