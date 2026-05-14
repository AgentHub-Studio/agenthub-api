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

const modelCapMigration = "000073_seed_model_capability_default_templates.up.sql"
const modelCapMigrationDown = "000073_seed_model_capability_default_templates.down.sql"

func TestIntegration_CoreModelCap_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedModelCapRowCount, len(got))
}

func TestIntegration_CoreModelCap_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreModelCap_FindByModelID_SonnetShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindByModelID(context.Background(), "claude-sonnet-4-6")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Claude Sonnet 4.6", tmpl.Label)
	assert.Equal(t, "sonnet", tmpl.Family)
	assert.Equal(t, 200_000, tmpl.ContextWindow)
	assert.Equal(t, 8192, tmpl.MaxOutputTokens)
	assert.NotEmpty(t, tmpl.RecommendedFor)
}

func TestIntegration_CoreModelCap_FindByModelID_OpusHasLargeOutput(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindByModelID(context.Background(), "claude-opus-4-7")
	require.NoError(t, err)
	require.True(t, found)
	assert.Greater(t, tmpl.MaxOutputTokens, 8192,
		"opus has a larger max_output_tokens than haiku/sonnet")
}

func TestIntegration_CoreModelCap_FindByModelID_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	_, found, err := loader.FindByModelID(context.Background(), "gpt-4-turbo")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreModelCap_LoadByFamily_Haiku(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	got, err := loader.LoadByFamily(context.Background(), "haiku")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "haiku", got[0].Family)
}

func TestIntegration_CoreModelCap_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedModelCapRowCount, len(got))
}

func TestIntegration_CoreModelCap_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)
	applyMigration(t, pool, migDir, modelCapMigrationDown)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreModelCap_SeedModelIDsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.ModelID)
	}
	expected := append([]string{}, core.SeedExpectedModelCapModelIDs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreModelCap_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].ModelID, all[i].ModelID)
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder)
		}
	}
	// haiku should be first (sort_order=10)
	assert.Equal(t, "haiku", all[0].Family)
}

func TestIntegration_CoreModelCap_AllContextWindowsEqual(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, modelCapMigration)

	loader := core.NewCoreModelCapabilityDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.Equal(t, core.SeedModelCapMaxContextWindow, t2.ContextWindow,
			"%s context window must be %d", t2.ModelID, core.SeedModelCapMaxContextWindow)
	}
}
