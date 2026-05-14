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

const ecpMigration = "000067_seed_extension_context_cost_policy_default_templates.up.sql"
const ecpMigrationDown = "000067_seed_extension_context_cost_policy_default_templates.down.sql"

func TestIntegration_CoreECP_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedECPTemplateRowCount, len(got))
}

func TestIntegration_CoreECP_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreECP_FindBySlug_MicroShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "micro-readonly-lookup")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "micro", tmpl.Category)
	assert.Equal(t, "readonly_lookup", tmpl.ExtensionKind)
	assert.LessOrEqual(t, tmpl.PerTurnTokens, core.SeedEXT010MicroMaxPerTurn)
}

func TestIntegration_CoreECP_FindByCategory_RoundTrip(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	for _, c := range core.SeedExpectedECPTemplateCategories {
		tmpl, found, err := loader.FindByCategory(context.Background(), c)
		require.NoError(t, err)
		require.True(t, found, "category %q must be seeded", c)
		assert.Equal(t, c, tmpl.Category)
	}
}

func TestIntegration_CoreECP_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreECP_AllCategoriesInEXT010Enum(t *testing.T) {
	// Cross-feature invariant: every seeded category must exist in the
	// EXT-010 ExtensionContextCostCategory enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedECPTemplateCategories {
		allowed[c] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Category],
			"%s category %q outside EXT-010 enum", t2.Slug, t2.Category)
	}
}

func TestIntegration_CoreECP_AllRowsRespectBandInvariant(t *testing.T) {
	// Cross-feature invariant: per_turn_tokens must classify to declared
	// category band (matches EXT-010 ClassifyByPerTurnTokens). DB CHECK
	// enforces but we double-check via app-level check.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		switch t2.Category {
		case "micro":
			assert.LessOrEqual(t, t2.PerTurnTokens, core.SeedEXT010MicroMaxPerTurn,
				"%s micro band violation", t2.Slug)
		case "small":
			assert.Greater(t, t2.PerTurnTokens, core.SeedEXT010MicroMaxPerTurn, "%s small lower", t2.Slug)
			assert.LessOrEqual(t, t2.PerTurnTokens, core.SeedEXT010SmallMaxPerTurn, "%s small upper", t2.Slug)
		case "medium":
			assert.Greater(t, t2.PerTurnTokens, core.SeedEXT010SmallMaxPerTurn, "%s medium lower", t2.Slug)
			assert.LessOrEqual(t, t2.PerTurnTokens, core.SeedEXT010MediumMaxPerTurn, "%s medium upper", t2.Slug)
		case "large":
			assert.Greater(t, t2.PerTurnTokens, core.SeedEXT010MediumMaxPerTurn, "%s large lower", t2.Slug)
			assert.LessOrEqual(t, t2.PerTurnTokens, core.SeedEXT010LargeMaxPerTurn, "%s large upper", t2.Slug)
		case "heavy":
			assert.Greater(t, t2.PerTurnTokens, core.SeedEXT010LargeMaxPerTurn, "%s heavy lower", t2.Slug)
		default:
			t.Fatalf("unexpected category %q", t2.Category)
		}
	}
}

func TestIntegration_CoreECP_DBCheckRejectsCategoryDrift(t *testing.T) {
	// DB CHECK chk_ext_cost_band_matches must fire when category claims
	// "micro" but per_turn_tokens=999.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.extension_context_cost_policy_default_template
		    (id, slug, category, extension_kind,
		     static_header_tokens, per_turn_tokens, per_tool_call_tokens,
		     max_budget_per_session, description, example_use_case, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000010', 'bad-drift', 'micro',
		     'readonly_lookup', 0, 999, 0, 100, 'bad', 'bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject category=micro with per_turn=999")
}

func TestIntegration_CoreECP_DBCheckRejectsNegativeTokens(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.extension_context_cost_policy_default_template
		    (id, slug, category, extension_kind,
		     static_header_tokens, per_turn_tokens, per_tool_call_tokens,
		     max_budget_per_session, description, example_use_case, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000011', 'bad-negative', 'micro',
		     'readonly_lookup', -1, 50, 10, 100, 'bad', 'bad', 999)
	`)
	require.Error(t, err, "DB CHECK must reject negative static_header_tokens")
}

func TestIntegration_CoreECP_AllExtensionKindsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedECPTemplateExtensionKinds {
		allowed[k] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.ExtensionKind],
			"%s extension_kind %q outside closed set", t2.Slug, t2.ExtensionKind)
	}
}

func TestIntegration_CoreECP_AllCategoriesUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Category], "duplicate category %q", t2.Category)
		seen[t2.Category] = true
	}
}

func TestIntegration_CoreECP_DescriptionsAndExampleUseCasesPresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
		assert.GreaterOrEqual(t, len(t2.ExampleUseCase), 20, "%s example_use_case", t2.Slug)
	}
}

func TestIntegration_CoreECP_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "micro-readonly-lookup", all[0].Slug)
}

func TestIntegration_CoreECP_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedECPTemplateRowCount, len(got))
}

func TestIntegration_CoreECP_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)
	applyMigration(t, pool, migDir, ecpMigrationDown)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreECP_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ecpMigration)

	loader := core.NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedECPTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
