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

const capabilityMigration = "000090_seed_capability_skill_tool_templates.up.sql"
const capabilityMigrationDown = "000090_seed_capability_skill_tool_templates.down.sql"

func TestIntegration_Capability_LoadCapabilityTools_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	got, err := loader.LoadCapabilityTools(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedCapabilityToolCount, len(got))
}

func TestIntegration_Capability_LoadCapabilitySkills_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	got, err := loader.LoadCapabilitySkills(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedCapabilitySkillCount, len(got))
}

func TestIntegration_Capability_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreCapabilitySkillToolLoader(pool)

	tools, err := loader.LoadCapabilityTools(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools)

	skills, err := loader.LoadCapabilitySkills(context.Background())
	require.NoError(t, err)
	assert.Empty(t, skills)
}

func TestIntegration_Capability_DocSearchIsDocumentSearchType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	tools, err := loader.LoadCapabilityTools(context.Background())
	require.NoError(t, err)

	var docSearch *core.CoreTool
	for i := range tools {
		if tools[i].Slug == core.SeedDocumentSearchToolSlug {
			docSearch = &tools[i]
			break
		}
	}
	require.NotNil(t, docSearch, "core-doc-search must be present after seed")
	assert.Equal(t, "DOCUMENT_SEARCH", docSearch.Type)
}

func TestIntegration_Capability_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityMigration)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	tools, err := loader.LoadCapabilityTools(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedCapabilityToolCount, len(tools))

	skills, err := loader.LoadCapabilitySkills(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedCapabilitySkillCount, len(skills))
}

func TestIntegration_Capability_DownMigrationRemovesCapabilityRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)
	applyMigration(t, pool, migDir, capabilityMigrationDown)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	tools, err := loader.LoadCapabilityTools(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools)

	skills, err := loader.LoadCapabilitySkills(context.Background())
	require.NoError(t, err)
	assert.Empty(t, skills)
}

func TestIntegration_Capability_AllToolSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	tools, err := loader.LoadCapabilityTools(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(tools))
	for _, t2 := range tools {
		dbSlugs = append(dbSlugs, t2.Slug)
	}
	for _, expected := range core.SeedCapabilityToolSlugs {
		assert.Contains(t, dbSlugs, expected)
	}
}

func TestIntegration_Capability_WebResearchSkillHasInlineContextMode(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityMigration)

	loader := core.NewCoreCapabilitySkillToolLoader(pool)
	skills, err := loader.LoadCapabilitySkills(context.Background())
	require.NoError(t, err)

	for _, s := range skills {
		assert.Equal(t, "capability", s.Category, "skill %q has wrong category", s.Slug)
		assert.Equal(t, "inline", s.ContextMode, "skill %q has wrong context_mode", s.Slug)
	}
}
