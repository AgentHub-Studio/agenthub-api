//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	commonsmigrate "github.com/AgentHub-Studio/agenthub-go-commons/database/migrate"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

var specialistBindingSeedMigrations = []string{
	"000001_ah_core_schema.up.sql",
	"000002_seed_tools.up.sql",
	"000003_seed_skills.up.sql",
	"000004_seed_agents.up.sql",
	"000005_upgrade_existing_schema.up.sql",
	"000006_reset_and_seed_specialists.up.sql",
}

// TestIntegration_CoreAgent_SpecialistsHaveOperationalSkillBindings verifies
// that each non-deprecated specialist is connected to the platform skill set
// required by its declared workflow.
func TestIntegration_CoreAgent_SpecialistsHaveOperationalSkillBindings(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, migration := range specialistBindingSeedMigrations {
		applyMigration(t, pool, migDir, migration)
	}
	applyMigration(t, pool, migDir, "000126_fix_specialist_skill_bindings.up.sql")

	ctx := context.Background()
	agentLoader := core.NewCoreAgentLoader(pool)
	skillLoader := core.NewCoreSkillLoader(pool)
	agents, err := agentLoader.LoadAll(ctx)
	require.NoError(t, err)
	skills, err := skillLoader.LoadAll(ctx)
	require.NoError(t, err)
	bindings, err := agentLoader.LoadSkillBindings(ctx)
	require.NoError(t, err)

	agentSlugs := make(map[string]string, len(agents))
	for _, agent := range agents {
		agentSlugs[agent.ID.String()] = agent.Slug
	}
	skillSlugs := make(map[string]string, len(skills))
	for _, skill := range skills {
		skillSlugs[skill.ID.String()] = skill.Slug
	}
	actual := make(map[string][]string, len(agents))
	for _, binding := range bindings {
		actual[agentSlugs[binding.AgentID.String()]] = append(actual[agentSlugs[binding.AgentID.String()]], skillSlugs[binding.SkillID.String()])
	}
	for slug := range actual {
		sort.Strings(actual[slug])
	}

	expected := map[string][]string{
		"core-agent-builder":        {"core-agents-management", "core-platform-settings", "core-skills-management"},
		"core-tool-builder":         {"core-skills-management", "core-tools-management"},
		"core-skills-specialist":    {"core-skills-management", "core-tools-management"},
		"core-tools-specialist":     {"core-tools-management"},
		"core-kb-builder":           {"core-agents-management", "core-kb-management"},
		"core-kb-specialist":        {"core-kb-management"},
		"core-mcp-configurator":     {"core-mcp-management"},
		"core-api-importer":         {"core-agents-management", "core-skills-management", "core-tools-management"},
		"core-agents-specialist":    {"core-agents-management"},
		"core-execution-specialist": {"core-execution-management"},
	}
	for agentSlug, expectedSkills := range expected {
		assert.ElementsMatch(t, expectedSkills, actual[agentSlug], "agent %q must receive its workflow skills", agentSlug)
	}

	assert.Empty(t, actual["core-pipeline-specialist"], "deprecated pipeline specialist remains read-only")
	assert.Len(t, actual["core-assistant"], len(skills), "assistant must retain universal skill coverage")
	assert.Len(t, bindings, core.SeedExpectedAgentSkillBindingsCount,
		"assistant and ten active specialists must have the canonical binding count")
}

func TestIntegration_CoreAgent_SpecialistBindingRepairIsIdempotentAndReversible(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	for _, migration := range specialistBindingSeedMigrations {
		applyMigration(t, pool, migDir, migration)
	}

	loader := core.NewCoreAgentLoader(pool)
	bindings, err := loader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	assert.Len(t, bindings, core.SeedExpectedAssistantSkillBindingsCount,
		"the historical seed has only the assistant's universal bindings")

	applyMigration(t, pool, migDir, "000126_fix_specialist_skill_bindings.up.sql")
	applyMigration(t, pool, migDir, "000126_fix_specialist_skill_bindings.up.sql")
	bindings, err = loader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	assert.Len(t, bindings, core.SeedExpectedAgentSkillBindingsCount,
		"reapplying the repair must not duplicate bindings")

	applyMigration(t, pool, migDir, "000126_fix_specialist_skill_bindings.down.sql")
	bindings, err = loader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	assert.Len(t, bindings, core.SeedExpectedAssistantSkillBindingsCount,
		"rollback must remove only the repair bindings")

	applyMigration(t, pool, migDir, "000126_fix_specialist_skill_bindings.up.sql")
	bindings, err = loader.LoadSkillBindings(context.Background())
	require.NoError(t, err)
	assert.Len(t, bindings, core.SeedExpectedAgentSkillBindingsCount,
		"the repair must restore the canonical catalog after rollback")
}

func TestIntegration_CoreAgent_SpecialistBindingRepairIsAppliedByMigrationDriver(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	testutil.MustExec(t, pool, "CREATE SCHEMA ah_core")

	ctx := context.Background()
	migDir := ah_coreMigrationsDir(t)
	require.NoError(t, commonsmigrate.Up(ctx, pool, "ah_core", migDir, "ah_core_seed_migrations"))
	require.NoError(t, commonsmigrate.Up(ctx, pool, "ah_core", migDir, "ah_core_seed_migrations"),
		"a second startup must observe the recorded version and remain a no-op")

	// The production chain also seeds capability agents after the management
	// catalog. Scope the count to the canonical platform agents so those
	// independent bindings do not change the specialist repair contract.
	var canonicalBindingCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM ah_core.agent_skill binding
		  JOIN ah_core.agent agent ON agent.id = binding.agent_id
		 WHERE agent.slug = ANY($1::text[])
	`, core.SeedExpectedAgentSlugs).Scan(&canonicalBindingCount))
	assert.Equal(t, core.SeedExpectedAgentSkillBindingsCount, canonicalBindingCount)

	var version int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT version FROM ah_core.ah_core_seed_migrations").Scan(&version))
	assert.Equal(t, 126, version, "the production migration driver must record the repair")
}
