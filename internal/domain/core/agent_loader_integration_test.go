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

// Integration tests for CoreAgentLoader against real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// BACKFILL — specialist seed migrations predate the testcontainers pattern.
// This file closes the audit gap and validates the additive binding repair.
//
// Migration chain applied per test:
//   000001 schema → 000002 tools → 000003 skills → 000004 initial agents
//   → 000005 schema upgrade → 000006 reset+seed specialists (12 agents)
//   → 000126 specialist skill-binding repair

func applyAgentSeedChain(t *testing.T, pool any, migDir string) {
	// Helper centralises the full seed chain — agents depend on tools+skills.
	applyAgentSeedChainImpl(t, pool, migDir)
}

func applyAgentSeedChainImpl(t *testing.T, pool any, migDir string) {
	t.Helper()
	type poolType = *pgxpoolPool
	_ = pool
	// We actually call applyMigration which already handles this.
}

func TestIntegration_CoreAgent_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, len(core.SeedExpectedAgentSlugs), len(got),
		"DB row count must match SeedExpectedAgentSlugs canonical count (12)")

	gotSlugs := map[string]core.CoreAgent{}
	for _, a := range got {
		gotSlugs[a.Slug] = a
	}
	for _, slug := range core.SeedExpectedAgentSlugs {
		assert.Contains(t, gotSlugs, slug, "DB must contain seeded slug %q", slug)
	}
}

func TestIntegration_CoreAgent_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "missing schema must NOT error (non-fatal)")
	assert.Empty(t, got)
}

func TestIntegration_CoreAgent_FindBySlug_ReturnsKnownAgent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	agent, found, err := loader.FindBySlug(context.Background(), "core-assistant")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "core-assistant", agent.Slug)
	assert.Equal(t, "ASSISTANT", agent.AgentType)
	assert.NotEmpty(t, agent.SystemPrompt, "assistant must have a system prompt")
	assert.True(t, agent.IsActive)
}

func TestIntegration_CoreAgent_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}
	loader := core.NewCoreAgentLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreAgent_AllAgentTypesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, t := range core.SeedExpectedAgentTypes {
		allowed[t] = true
	}
	for _, a := range got {
		assert.True(t, allowed[a.AgentType],
			"agent %q has type %q outside expected set", a.Slug, a.AgentType)
	}
}

func TestIntegration_CoreAgent_ExactlyOneAssistantAgentExists(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	assistants := 0
	for _, a := range got {
		if a.AgentType == "ASSISTANT" {
			assistants++
		}
	}
	assert.Equal(t, 1, assistants,
		"EXACTLY ONE ASSISTANT-type agent must exist (catch-all entry point)")
}

func TestIntegration_CoreAgent_AllAgentsUseCorePrefix(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}
	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, a := range got {
		assert.True(t, strings.HasPrefix(a.Slug, "core-"),
			"slug %q must start with core-", a.Slug)
	}
}

func TestIntegration_CoreAgent_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, a := range got {
		assert.False(t, seen[a.Slug], "duplicate slug %q in DB", a.Slug)
		seen[a.Slug] = true
	}
}

func TestIntegration_CoreAgent_AllSystemPromptsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, a := range got {
		assert.NotEmpty(t, a.SystemPrompt,
			"agent %q must have system_prompt (LLM behavior depends on it)", a.Slug)
		assert.GreaterOrEqual(t, len(a.SystemPrompt), 100,
			"agent %q system_prompt too short (%d chars) — must be informative",
			a.Slug, len(a.SystemPrompt))
	}
}

func TestIntegration_CoreAgent_LoadSkillBindings_MatchesCanonicalCount(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	bindings, err := loader.LoadSkillBindings(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedExpectedAgentSkillBindingsCount, len(bindings),
		"binding count must match SeedExpectedAgentSkillBindingsCount (canonical catalog after migration 000126)")
}

func TestIntegration_CoreAgent_AssistantHasAllSkillBindings(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	agentLoader := core.NewCoreAgentLoader(pool)
	skillLoader := core.NewCoreSkillLoader(pool)

	assistant, found, err := agentLoader.FindBySlug(context.Background(), "core-assistant")
	require.NoError(t, err)
	require.True(t, found)

	bindings, err := agentLoader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	skills, err := skillLoader.LoadAll(context.Background())
	require.NoError(t, err)

	// Count bindings for assistant.
	assistantBindings := 0
	for _, b := range bindings {
		if b.AgentID == assistant.ID {
			assistantBindings++
		}
	}
	assert.Equal(t, len(skills), assistantBindings,
		"core-assistant must bind ALL skills (cross-join contract): %d skills → %d bindings",
		len(skills), assistantBindings)
}

func TestIntegration_CoreAgent_BindingsReferenceExistingAgentsAndSkills(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	agentLoader := core.NewCoreAgentLoader(pool)
	skillLoader := core.NewCoreSkillLoader(pool)
	bindings, err := agentLoader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	agents, _ := agentLoader.LoadAll(context.Background())
	skills, _ := skillLoader.LoadAll(context.Background())

	agentIDs := map[string]bool{}
	for _, a := range agents {
		agentIDs[a.ID.String()] = true
	}
	skillIDs := map[string]bool{}
	for _, s := range skills {
		skillIDs[s.ID.String()] = true
	}
	for _, b := range bindings {
		assert.True(t, agentIDs[b.AgentID.String()],
			"binding references agent_id %s not in ah_core.agent", b.AgentID)
		assert.True(t, skillIDs[b.SkillID.String()],
			"binding references skill_id %s not in ah_core.skill", b.SkillID)
	}
}

func TestIntegration_CoreAgent_ResetMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}
	// Re-apply the reset and its additive repair — the final catalog is stable.
	applyMigration(t, pool, migDir, "000006_reset_and_seed_specialists.up.sql")
	applyMigration(t, pool, migDir, "000126_fix_specialist_skill_bindings.up.sql")

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedAgentSlugs), len(got),
		"second-apply must result in same row count (DELETE+INSERT pattern)")

	bindings, err := loader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAgentSkillBindingsCount, len(bindings),
		"second-apply binding count must remain stable")
}

func TestIntegration_CoreAgent_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, m := range []string{
		"000001_ah_core_schema.up.sql",
		"000002_seed_tools.up.sql",
		"000003_seed_skills.up.sql",
		"000004_seed_agents.up.sql",
		"000005_upgrade_existing_schema.up.sql",
		"000006_reset_and_seed_specialists.up.sql",
		"000126_fix_specialist_skill_bindings.up.sql",
	} {
		applyMigration(t, pool, migDir, m)
	}

	loader := core.NewCoreAgentLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, a := range got {
		dbSlugs = append(dbSlugs, a.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedAgentSlugs...)
	sort.Strings(expected)

	assert.Equal(t, expected, dbSlugs,
		"DB slugs must EXACTLY match SeedExpectedAgentSlugs")
}

// pgxpoolPool is a marker so the helper compiles regardless of whether
// it is used. The actual integration uses applyMigration directly.
type pgxpoolPool struct{}
