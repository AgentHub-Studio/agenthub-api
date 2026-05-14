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

const cadMigration = "000062_seed_custom_agent_definition_default_templates.up.sql"
const cadMigrationDown = "000062_seed_custom_agent_definition_default_templates.down.sql"

func TestIntegration_CoreCAD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCADTemplateRowCount, len(got))
}

func TestIntegration_CoreCAD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCAD_FindBySlug_BareGreenfieldShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "bare-greenfield")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "bare", tmpl.ShapeKind)
	assert.Equal(t, "", tmpl.DerivedFromBuiltinSlug,
		"bare-greenfield must have no SUB-002 lineage")
	assert.Equal(t, "documentation-readonly-allowlist", tmpl.ToolsetPolicySlug)
	assert.Equal(t, "restrict-to-readonly", tmpl.InheritanceModeSlug)
	assert.Equal(t, "structured-findings", tmpl.SummaryShapeSlug)
	assert.Equal(t, "private", tmpl.SuggestedVisibility)
}

func TestIntegration_CoreCAD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCAD_LoadByShapeKind_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	for _, kind := range core.SeedExpectedCADTemplateShapeKinds {
		matched, err := loader.LoadByShapeKind(context.Background(), kind)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched),
			"shape_kind %q must have exactly 1 starter (1:1)", kind)
	}
}

func TestIntegration_CoreCAD_LoadDerivedFromBuiltin_ResearcherLineage(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	matched, err := loader.LoadDerivedFromBuiltin(context.Background(), "researcher-baseline")
	require.NoError(t, err)
	require.Equal(t, 1, len(matched))
	assert.Equal(t, "researcher-derived", matched[0].Slug)
}

func TestIntegration_CoreCAD_LoadGreenfieldOnlyBare(t *testing.T) {
	// Cross-feature invariant: only bare-greenfield has no SUB-002 lineage.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	greenfield, err := loader.LoadGreenfield(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range greenfield {
		got = append(got, t2.Slug)
	}
	assert.Equal(t, []string{"bare-greenfield"}, got)
}

func TestIntegration_CoreCAD_AllShapeKindsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCADTemplateShapeKinds {
		allowed[k] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.ShapeKind],
			"%s shape_kind %q outside closed set", t2.Slug, t2.ShapeKind)
	}
}

func TestIntegration_CoreCAD_ToolsetRefsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedCADTemplateToolsetSlugs {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.ToolsetPolicySlug],
			"%s toolset slug %q outside SUB-005 closed set",
			t2.Slug, t2.ToolsetPolicySlug)
	}
}

func TestIntegration_CoreCAD_InheritanceRefsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedCADTemplateInheritanceSlugs {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.InheritanceModeSlug],
			"%s inheritance slug %q outside SUB-006 closed set",
			t2.Slug, t2.InheritanceModeSlug)
	}
}

func TestIntegration_CoreCAD_SummaryRefsInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedCADTemplateSummarySlugs {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.SummaryShapeSlug],
			"%s summary slug %q outside SUB-010 closed set",
			t2.Slug, t2.SummaryShapeSlug)
	}
}

func TestIntegration_CoreCAD_LineageRefsInSUB002ClosedSet(t *testing.T) {
	// Cross-feature invariant: non-empty derived_from_builtin_slug must
	// be one of the SUB-002 builtin slugs.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedCADTemplateBuiltinLineageSlugs {
		allowed[s] = true
	}
	for _, t2 := range all {
		if t2.DerivedFromBuiltinSlug == "" {
			continue
		}
		assert.True(t, allowed[t2.DerivedFromBuiltinSlug],
			"%s lineage slug %q outside SUB-002 closed set",
			t2.Slug, t2.DerivedFromBuiltinSlug)
	}
}

func TestIntegration_CoreCAD_DualLoopPlannerUsesPlanOnlySummary(t *testing.T) {
	// Cross-feature invariant: dual-loop-planner is the SUB-010 plan-only
	// half of the dual-loop pattern.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "dual-loop-planner")
	require.NoError(t, err)
	assert.Equal(t, "plan-only", tmpl.SummaryShapeSlug)
	assert.Equal(t, "restrict-to-readonly", tmpl.InheritanceModeSlug)
	assert.Equal(t, "planner-baseline", tmpl.DerivedFromBuiltinSlug)
}

func TestIntegration_CoreCAD_AllSystemPromptsAndDescriptionsPresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.SystemPromptTemplate), 40, "%s prompt", t2.Slug)
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreCAD_AllSlugsUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreCAD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
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
	assert.Equal(t, "bare-greenfield", all[0].Slug)
}

func TestIntegration_CoreCAD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCADTemplateRowCount, len(got))
}

func TestIntegration_CoreCAD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)
	applyMigration(t, pool, migDir, cadMigrationDown)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCAD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cadMigration)

	loader := core.NewCoreCustomAgentDefinitionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedCADTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
